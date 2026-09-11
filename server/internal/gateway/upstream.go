package gateway

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type UpstreamConfig struct {
	URL     string            `json:"url"`
	Headers map[string]string `json:"headers"`
	Command string            `json:"command"`
	Args    []string          `json:"args"`
	Env     map[string]string `json:"env"`
}
type upstreamSession struct {
	session *mcp.ClientSession
	pool    *httpSessionPool
	ctx     context.Context
	cancel  context.CancelFunc
}

func (s *upstreamSession) release(cause error) {
	if s.pool != nil {
		s.pool.release(s, cause)
	}
}

func (s *upstreamSession) close() {
	_ = s.session.Close()
}
func sessionKey(row toolRow) string { return fmt.Sprintf("%d:%x", row.ID, sha256.Sum256(row.Config)) }
func (g *Gateway) dropSession(ctx context.Context, row toolRow, cause error) {
	// A canceled request or an RPC error response does not break its connection.
	// Closing it would interrupt other requests still using the same session.
	var rpcError *jsonrpc.Error
	if ctx.Err() != nil || errors.As(cause, &rpcError) {
		return
	}
	g.mu.Lock()
	key := sessionKey(row)
	session := g.sessions[key]
	delete(g.sessions, key)
	g.mu.Unlock()
	if session != nil {
		session.close()
	}
}

func (g *Gateway) ValidateConfig(kind string, raw []byte) error {
	var cfg UpstreamConfig
	if json.Unmarshal(raw, &cfg) != nil {
		return errors.New("invalid upstream config")
	}
	switch kind {
	case "http":
		client, err := upstreamClient(cfg.URL, cfg.Headers, g.options.AllowPrivate)
		if err == nil {
			client.CloseIdleConnections()
		}
		return err
	case "stdio":
		if g.options.StdioCommands[cfg.Command] == "" {
			return errors.New("stdio command not allowlisted")
		}
		return nil
	default:
		return errors.New("unknown upstream kind")
	}
}
func (g *Gateway) upstream(ctx context.Context, row toolRow) (*upstreamSession, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := g.ctx.Err(); err != nil {
		return nil, err
	}
	if row.Kind == "http" {
		pool, err := g.httpPool(row)
		if err != nil {
			return nil, err
		}
		return pool.lease(ctx)
	}
	key := sessionKey(row)
	g.mu.Lock()
	existing := g.sessions[key]
	g.mu.Unlock()
	if existing != nil {
		return existing, nil
	}
	completed := g.flight.DoChan(key, func() (any, error) {
		g.mu.Lock()
		existing := g.sessions[key]
		g.mu.Unlock()
		if existing != nil {
			return existing, nil
		}
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()
		stop := context.AfterFunc(g.ctx, cancel)
		defer stop()
		raw, err := OpenConfig(g.options.EncryptionKey, row.Config)
		if err != nil {
			return nil, err
		}
		var cfg UpstreamConfig
		if err = json.Unmarshal(raw, &cfg); err != nil {
			return nil, err
		}
		client := mcp.NewClient(&mcp.Implementation{Name: "pluginpocket-upstream", Version: "0.2.0"}, nil)
		var transport mcp.Transport
		switch row.Kind {
		case "stdio":
			path := g.options.StdioCommands[cfg.Command]
			if path == "" {
				return nil, errors.New("stdio disabled")
			}
			cmd := exec.Command(path, cfg.Args...)
			cmd.Env = []string{"PATH=/usr/local/bin:/usr/bin:/bin"}
			for k, v := range cfg.Env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			transport = &mcp.CommandTransport{Command: cmd}
		default:
			return nil, errors.New("unsupported upstream")
		}
		session, err := client.Connect(ctx, transport, nil)
		if err != nil {
			return nil, err
		}
		result := &upstreamSession{session: session}
		var retired []*upstreamSession
		g.mu.Lock()
		if err := g.ctx.Err(); err != nil {
			g.mu.Unlock()
			result.close()
			return nil, err
		}
		prefix := fmt.Sprintf("%d:", row.ID)
		for oldKey, old := range g.sessions {
			if oldKey != key && strings.HasPrefix(oldKey, prefix) {
				retired = append(retired, old)
				delete(g.sessions, oldKey)
			}
		}
		g.sessions[key] = result
		g.mu.Unlock()
		for _, old := range retired {
			old.close()
		}
		return result, nil
	})
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-g.ctx.Done():
		return nil, g.ctx.Err()
	case result := <-completed:
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Val.(*upstreamSession), nil
	}
}
