package gateway

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/netip"
	"net/url"
	"strings"
	"time"
)

func unsafeAddress(ip netip.Addr) bool {
	ip = ip.Unmap()
	return !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || netip.MustParsePrefix("100.64.0.0/10").Contains(ip)
}

func upstreamClient(address string, headers map[string]string, allowPrivate bool) (*http.Client, error) {
	u, err := url.Parse(address)
	if err != nil || u.Hostname() == "" || u.User != nil || u.Fragment != "" || (u.Scheme != "https" && (!allowPrivate || u.Scheme != "http")) {
		return nil, errors.New("invalid upstream URL")
	}
	if !allowPrivate {
		host := strings.ToLower(strings.TrimSuffix(u.Hostname(), "."))
		if host == "localhost" || strings.HasSuffix(host, ".localhost") {
			return nil, errors.New("private upstream address")
		}
		if ip, err := netip.ParseAddr(host); err == nil && unsafeAddress(ip) {
			return nil, errors.New("private upstream address")
		}
	}
	for name, value := range headers {
		if strings.ContainsAny(name+value, "\r\n") || strings.EqualFold(name, "Host") {
			return nil, errors.New("invalid upstream header")
		}
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.MaxConnsPerHost = 32
	transport.MaxIdleConnsPerHost = 8
	transport.ResponseHeaderTimeout = 10 * time.Second
	transport.IdleConnTimeout = 90 * time.Second
	if !allowPrivate {
		transport.DialContext = publicDial
	}
	return &http.Client{Transport: headerTransport{base: transport, headers: headers}, Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}, nil
}

type headerTransport struct {
	base    *http.Transport
	headers map[string]string
	life    context.Context
}

func (h headerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// SDK v1.7.0 still emits this legacy notification for stateless HTTP.
	// Reject it before sending: the canceled HTTP request already stops its
	// handler, and an upstream 400 would poison unrelated calls on the session.
	if r.Header.Get("Mcp-Protocol-Version") >= "2026-07-28" && r.Header.Get("Mcp-Session-Id") == "" && r.Header.Get("Mcp-Method") == "notifications/cancelled" {
		if r.Body != nil {
			_ = r.Body.Close()
		}
		return nil, errors.New("stateless MCP HTTP uses request cancellation")
	}
	request := r.Clone(r.Context())
	cleanup := func() {}
	if h.life != nil {
		ctx, cancel := context.WithCancel(r.Context())
		stop := context.AfterFunc(h.life, cancel)
		cleanup = func() { stop(); cancel() }
		request = r.Clone(ctx)
	}
	for key, value := range h.headers {
		request.Header.Set(key, value)
	}
	response, err := h.base.RoundTrip(request)
	if err != nil {
		cleanup()
	} else if h.life != nil {
		response.Body = &cancelBody{ReadCloser: response.Body, cancel: cleanup}
	}
	return response, err
}
func (h headerTransport) CloseIdleConnections() { h.base.CloseIdleConnections() }

type cancelBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *cancelBody) Close() error {
	err := b.ReadCloser.Close()
	b.cancel()
	return err
}

func publicDial(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, err
	}
	ips, err := net.DefaultResolver.LookupNetIP(ctx, "ip", host)
	if err != nil {
		return nil, errors.New("upstream DNS failed")
	}
	// Validate the whole answer and dial the checked IP without a second DNS lookup.
	for _, ip := range ips {
		if unsafeAddress(ip) {
			return nil, errors.New("private upstream address")
		}
	}
	dialer := net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	for _, ip := range ips {
		conn, e := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if e == nil {
			return conn, nil
		}
	}
	return nil, errors.New("upstream connection failed")
}
