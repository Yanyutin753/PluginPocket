package gateway

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// HTTP requests own an SDK session until completion. One RPC/HTTP error must
// never close a session carrying another request; the HTTP client stays shared.
type httpProvider struct {
	slots chan struct{}
	pools int // guarded by Gateway.mu; includes retired pools with active leases
}

type httpSessionPool struct {
	owner     *Gateway
	rowID     int64
	key       string
	url       string
	client    *http.Client
	ctx       context.Context
	cancel    context.CancelFunc
	mu        sync.Mutex
	retired   bool
	forgotten bool
	leases    int // includes waiting and unfinished handshakes
	provider  *httpProvider
	idle      []*mcp.ClientSession
	sessions  map[*mcp.ClientSession]context.CancelFunc
}

func (g *Gateway) httpPool(row toolRow) (*httpSessionPool, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if err := g.ctx.Err(); err != nil {
		return nil, err
	}
	key := sessionKey(row)
	if pool := g.httpPools[key]; pool != nil {
		pool.mu.Lock()
		retired := pool.retired
		pool.mu.Unlock()
		if !retired {
			return pool, nil
		}
	}
	// A request holding an old binding cannot recreate a retired configuration
	// after a newer catalog has been published.
	if time.Now().Before(g.catalogUntil) {
		active := false
		for _, binding := range g.catalog {
			if sessionKey(binding.row) == key {
				active = true
				break
			}
		}
		if !active {
			return nil, errors.New("upstream configuration is no longer active")
		}
	}
	raw, err := OpenConfig(g.options.EncryptionKey, row.Config)
	if err != nil {
		return nil, err
	}
	var config UpstreamConfig
	if err := json.Unmarshal(raw, &config); err != nil {
		return nil, err
	}
	client, err := upstreamClient(config.URL, config.Headers, g.options.AllowPrivate)
	if err != nil {
		return nil, err
	}
	transport := client.Transport.(headerTransport)
	transport.life = g.ctx
	client.Transport = transport
	ctx, cancel := context.WithCancel(g.ctx)
	if g.httpProviders == nil {
		g.httpProviders = make(map[int64]*httpProvider)
	}
	provider := g.httpProviders[row.ID]
	if provider == nil {
		provider = &httpProvider{slots: make(chan struct{}, 32)}
		g.httpProviders[row.ID] = provider
	}
	provider.pools++
	pool := &httpSessionPool{owner: g, rowID: row.ID, key: key, url: config.URL, client: client, ctx: ctx, cancel: cancel, provider: provider, sessions: make(map[*mcp.ClientSession]context.CancelFunc)}
	g.httpPools[key] = pool
	return pool, nil
}

func (p *httpSessionPool) lease(ctx context.Context) (*upstreamSession, error) {
	type result struct {
		session *upstreamSession
		err     error
	}
	completed := make(chan result)
	go func() {
		session, err := p.acquire(ctx)
		select {
		case completed <- result{session, err}:
			return
		case <-ctx.Done():
		case <-p.owner.ctx.Done():
		}
		if session != nil {
			session.release(context.Canceled)
		}
	}()
	select {
	case got := <-completed:
		return got.session, got.err
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-p.owner.ctx.Done():
		return nil, p.owner.ctx.Err()
	}
}

func (p *httpSessionPool) acquire(parent context.Context) (*upstreamSession, error) {
	p.mu.Lock()
	if p.retired {
		p.mu.Unlock()
		return nil, context.Canceled
	}
	p.leases++
	p.mu.Unlock()
	select {
	case p.provider.slots <- struct{}{}:
	case <-parent.Done():
		p.finishLease()
		return nil, parent.Err()
	case <-p.ctx.Done():
		p.finishLease()
		return nil, p.ctx.Err()
	}
	p.owner.mu.Lock()
	if err := p.owner.ctx.Err(); err != nil {
		p.owner.mu.Unlock()
		<-p.provider.slots
		p.finishLease()
		return nil, err
	}
	p.owner.httpLeases.Add(1)
	p.owner.mu.Unlock()
	ctx, cancel := context.WithCancel(parent)
	stop := context.AfterFunc(p.owner.ctx, cancel)
	cleanup := func() { stop(); cancel() }
	// Retiring a pool stops unfinished handshakes, while established leases
	// keep their request deadline and are canceled only by Gateway.Close.
	stopConnect := context.AfterFunc(p.ctx, cancel)
	defer stopConnect()
	p.mu.Lock()
	var session *mcp.ClientSession
	if n := len(p.idle); n > 0 {
		session = p.idle[n-1]
		p.idle = p.idle[:n-1]
	}
	retired := p.retired
	p.mu.Unlock()
	var err error
	if retired {
		err = context.Canceled
	} else if session == nil {
		client := mcp.NewClient(&mcp.Implementation{Name: "loadout-upstream", Version: "0.2.0"}, nil)
		session, err = client.Connect(ctx, &mcp.StreamableClientTransport{Endpoint: p.url, HTTPClient: p.client, MaxRetries: -1, DisableStandaloneSSE: true}, nil)
	}
	p.mu.Lock()
	if err == nil && (p.retired || ctx.Err() != nil) {
		err = context.Canceled
	}
	if err == nil {
		p.sessions[session] = cleanup
	} else if session != nil {
		delete(p.sessions, session)
	}
	p.mu.Unlock()
	if err != nil {
		cleanup()
		if session != nil {
			_ = session.Close()
		}
		<-p.provider.slots
		p.finishLease()
		p.owner.httpLeases.Done()
		return nil, err
	}
	return &upstreamSession{session: session, pool: p, ctx: ctx, cancel: cleanup}, nil
}

func (p *httpSessionPool) release(lease *upstreamSession, cause error) {
	lease.cancel()
	p.mu.Lock()
	retired := p.retired
	discard := cause != nil || retired
	if discard {
		delete(p.sessions, lease.session)
	} else {
		p.sessions[lease.session] = nil
		p.idle = append(p.idle, lease.session)
	}
	p.mu.Unlock()
	if discard {
		_ = lease.session.Close()
	}
	if retired {
		p.client.CloseIdleConnections()
	}
	<-p.provider.slots
	p.finishLease()
	p.owner.httpLeases.Done()
}

func (p *httpSessionPool) close(force bool) {
	p.cancel()
	p.mu.Lock()
	p.retired = true
	closing := p.idle
	p.idle = nil
	if force {
		for _, cancel := range p.sessions {
			if cancel != nil {
				cancel()
			}
		}
	}
	for _, session := range closing {
		delete(p.sessions, session)
	}
	p.mu.Unlock()
	for _, session := range closing {
		_ = session.Close()
	}
	p.client.CloseIdleConnections()
	p.forgetIfUnused()
}

func (p *httpSessionPool) finishLease() {
	p.mu.Lock()
	p.leases--
	p.mu.Unlock()
	p.forgetIfUnused()
}

func (p *httpSessionPool) forgetIfUnused() {
	p.mu.Lock()
	done := p.retired && p.leases == 0 && len(p.sessions) == 0 && !p.forgotten
	if done {
		p.forgotten = true
	}
	p.mu.Unlock()
	if done {
		p.owner.mu.Lock()
		if p.owner.httpPools[p.key] == p {
			delete(p.owner.httpPools, p.key)
		}
		p.provider.pools--
		if p.provider.pools == 0 && p.owner.httpProviders[p.rowID] == p.provider {
			delete(p.owner.httpProviders, p.rowID)
		}
		p.owner.mu.Unlock()
	}
}
