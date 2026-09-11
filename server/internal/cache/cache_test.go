package cache

import (
	"context"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestOpenValidatesWithoutConnectingAndHidesCredentials(t *testing.T) {
	for _, raw := range []string{"http://name:TOP_SECRET@localhost:6379", "redis://name:TOP_SECRET@localhost:bad", "redis://name:TOP_SECRET@localhost:6379/?unknown_option=1"} {
		c, err := Open(raw, "test")
		if c != nil {
			_ = c.Close()
		}
		if err == nil {
			t.Errorf("invalid redis URL accepted")
		} else if strings.Contains(err.Error(), "TOP_SECRET") {
			t.Errorf("credentials leaked: %v", err)
		}
	}
	if c, err := Open("redis://127.0.0.1:1/0", "bad:namespace"); err == nil {
		_ = c.Close()
		t.Error("ambiguous namespace accepted")
	}
	c, err := Open("redis://name:TOP_SECRET@127.0.0.1:1/0", "valid")
	if err != nil {
		t.Fatalf("Open must not require availability: %v", err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	started := time.Now()
	err = c.Ping(context.Background())
	if err == nil {
		t.Fatal("unavailable Redis returned success")
	}
	if strings.Contains(err.Error(), "TOP_SECRET") {
		t.Fatal("credentials leaked")
	}
	if time.Since(started) > time.Second {
		t.Fatal("cache outage blocked request")
	}
}

func redisFixture(t *testing.T, namespace string) *Client {
	t.Helper()
	raw := os.Getenv("PLUGINPOCKET_TEST_REDIS_URL")
	if raw == "" {
		t.Skip("PLUGINPOCKET_TEST_REDIS_URL required for real Redis integration")
	}
	c, err := Open(raw, namespace)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = c.Close() })
	if err = c.Ping(t.Context()); err != nil {
		t.Fatal(err)
	}
	return c
}
func namespace(t *testing.T) string { return fmt.Sprintf("test_%d", time.Now().UnixNano()) }

func TestSharedCacheNamespaceAndTTL(t *testing.T) {
	ns := namespace(t)
	first := redisFixture(t, ns)
	second := redisFixture(t, ns)
	other := redisFixture(t, ns+"_other")
	if err := first.Set(t.Context(), "tools", []byte(`{"tools":["ping"]}`), 80*time.Millisecond); err != nil {
		t.Fatal(err)
	}
	got, err := second.Get(t.Context(), "tools")
	if err != nil || string(got) != `{"tools":["ping"]}` {
		t.Fatalf("shared value=%q err=%v", got, err)
	}
	if got, err := other.Get(t.Context(), "tools"); err != nil || got != nil {
		t.Fatalf("namespace leak: %q %v", got, err)
	}
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		got, err = second.Get(t.Context(), "tools")
		if err != nil {
			t.Fatal(err)
		}
		if got == nil {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("cache value did not expire")
}

func TestInvalidationIsSharedNamespacedAndCancelable(t *testing.T) {
	ns := namespace(t)
	first := redisFixture(t, ns)
	second := redisFixture(t, ns)
	other := redisFixture(t, ns+"_other")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	messages := make(chan string, 10)
	done := make(chan struct{})
	go func() { second.Watch(ctx, func(source string) { messages <- source }); close(done) }()
	deadline := time.Now().Add(time.Second)
	received := false
	for time.Now().Before(deadline) && !received {
		if err := first.PublishInvalidation(t.Context(), "node-a"); err != nil {
			t.Fatal(err)
		}
		select {
		case value := <-messages:
			if value != "node-a" {
				t.Fatal(value)
			}
			received = true
		case <-time.After(20 * time.Millisecond):
		}
	}
	if !received {
		t.Fatal("other instance did not receive invalidation")
	}
	if err := other.PublishInvalidation(t.Context(), "wrong-namespace"); err != nil {
		t.Fatal(err)
	}
	isolationDeadline := time.NewTimer(50 * time.Millisecond)
	defer isolationDeadline.Stop()
checkIsolation:
	for {
		select {
		case got := <-messages:
			if got == "wrong-namespace" {
				t.Fatal("namespace channel leak")
			}
		case <-isolationDeadline.C:
			break checkIsolation
		}
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("watch did not stop on cancellation")
	}
}

func TestCacheCommandHonorsContextWithUnresponsiveServer(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := listener.Close(); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	done := make(chan struct{})
	defer close(done)
	go func() {
		conn, err := listener.Accept()
		if err == nil {
			defer func() {
				if err := conn.Close(); err != nil {
					t.Errorf("fixture cleanup failed: %v", err)
				}
			}()
			<-done
		}
	}()
	c, err := Open("redis://"+listener.Addr().String(), "test")
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := c.Close(); err != nil {
			t.Errorf("fixture cleanup failed: %v", err)
		}
	}()
	ctx, cancel := context.WithTimeout(t.Context(), 25*time.Millisecond)
	defer cancel()
	started := time.Now()
	_, err = c.Get(ctx, "metadata")
	if err == nil {
		t.Fatal("unresponsive cache returned success")
	}
	if time.Since(started) > 500*time.Millisecond {
		t.Fatal("context deadline ignored")
	}
}

func TestWatchReconnectsAfterItsRedisConnectionIsLost(t *testing.T) {
	ns := namespace(t)
	publisher := redisFixture(t, ns)
	u, err := url.Parse(os.Getenv("PLUGINPOCKET_TEST_REDIS_URL"))
	if err != nil {
		t.Fatal(err)
	}
	query := u.Query()
	query.Set("client_name", ns)
	u.RawQuery = query.Encode()
	watcher, err := Open(u.String(), ns)
	if err != nil {
		t.Fatal(err)
	}
	// Fallback cleanup also runs after the explicit Close assertion below.
	// A second close may report an already-closed connection.
	defer func() { _ = watcher.Close() }()
	messages := make(chan string, 8)
	done := make(chan struct{})
	go func() { watcher.Watch(t.Context(), func(source string) { messages <- source }); close(done) }()
	waitMessage := func(want string) {
		t.Helper()
		deadline := time.Now().Add(3 * time.Second)
		for time.Now().Before(deadline) {
			if err := publisher.PublishInvalidation(t.Context(), want); err != nil {
				t.Fatal(err)
			}
			select {
			case got := <-messages:
				if got == want {
					return
				}
			case <-time.After(20 * time.Millisecond):
			}
		}
		t.Fatalf("did not receive %s", want)
	}
	waitMessage("before")
	clients, err := publisher.redis.ClientList(t.Context()).Result()
	if err != nil {
		t.Fatal(err)
	}
	id := ""
	for _, line := range strings.Split(clients, "\n") {
		fields := map[string]string{}
		for _, field := range strings.Fields(line) {
			key, value, _ := strings.Cut(field, "=")
			fields[key] = value
		}
		if fields["name"] == ns && strings.Contains(fields["flags"], "P") {
			id = fields["id"]
		}
	}
	if id == "" {
		t.Fatal("test subscription connection not found")
	}
	// Disconnect only this test's named connection, never any other namespace.
	if err := publisher.redis.ClientKillByFilter(t.Context(), "ID", id).Err(); err != nil {
		t.Fatal(err)
	}
	waitMessage("after")
	if err := watcher.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Close left Watch blocked")
	}
}
