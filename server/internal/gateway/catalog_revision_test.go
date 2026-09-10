package gateway

import (
	"context"
	"sync"
	"testing"
	"time"
)

func TestCatalogRevisionInvalidatesAnAlreadyPublishedFlight(t *testing.T) {
	s := gatewayDB(t)
	g := New(s, Options{})
	t.Cleanup(g.Close)
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	previous, err := g.tools(ctx)
	if err != nil || len(previous) != 2 {
		t.Fatalf("cannot warm real catalog: count=%d err=%v", len(previous), err)
	}
	// A real catalog flight publishes its snapshot, then retires old SDK
	// sessions before returning. Hold that final stage without a production hook.
	published, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	unblock := func() { once.Do(func() { close(release) }) }
	defer unblock()
	held := g.flight.DoChan("catalog", func() (any, error) {
		close(published)
		select {
		case <-release:
			return previous, nil
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	})
	<-published
	// This is a committed database change, as made by another replica.
	if _, err := s.Pool.Exec(ctx, "UPDATE tools SET enabled=false WHERE key='echo'"); err != nil {
		t.Fatal(err)
	}
	var revision int64
	if err := s.Pool.QueryRow(ctx, "SELECT revision FROM tool_catalog_revision WHERE singleton").Scan(&revision); err != nil {
		t.Fatal(err)
	}
	type result struct {
		bindings []toolBinding
		err      error
	}
	completed := make(chan result, 1)
	go func() {
		bindings, err := g.tools(ctx)
		completed <- result{bindings, err}
	}()
	// Wait for the new caller's authoritative revision read. It must still
	// wait on the held flight; an old snapshot cannot satisfy the new request.
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	for {
		g.mu.Lock()
		observed := g.catalogRevision
		g.mu.Unlock()
		if observed == revision {
			break
		}
		select {
		case <-ticker.C:
		case <-ctx.Done():
			t.Fatal("new catalog request did not observe the committed revision")
		}
	}
	select {
	case <-completed:
		t.Fatal("catalog request unexpectedly completed before its shared flight")
	case <-time.After(25 * time.Millisecond):
	}
	unblock()
	if flight := <-held; !flight.Shared {
		t.Fatal("test did not exercise a caller joining the already-published flight")
	}
	select {
	case got := <-completed:
		if got.err != nil {
			t.Fatal(got.err)
		}
		for _, binding := range got.bindings {
			if binding.definition.Name == "echo" {
				t.Fatal("request begun after committed disable returned the previous flight's stale echo tool")
			}
		}
		if len(got.bindings) != 1 || got.bindings[0].definition.Name != "time_now" {
			t.Fatal("catalog refresh lost the enabled tool")
		}
	case <-ctx.Done():
		t.Fatal("catalog did not rebuild after the invalidated flight completed")
	}
}
