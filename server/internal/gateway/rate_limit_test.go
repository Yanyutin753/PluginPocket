package gateway

import (
	"context"
	"testing"

	"github.com/Yanyutin753/loadout/server/internal/store"
)

func TestAdmitUsesConfiguredLimits(t *testing.T) {
	s := gatewayDB(t)
	g := New(s, Options{TokenPerMinute: 1000000, UserPerDay: 100000000})
	t.Cleanup(g.Close)
	principal := store.Principal{UserID: 11, TokenID: 12}
	for i := 0; i < 70; i++ {
		if !g.admit(context.Background(), principal) {
			t.Fatalf("relaxed token limit denied request %d", i+1)
		}
	}
	strict := New(s, Options{})
	t.Cleanup(strict.Close)
	allowed := 0
	for i := 0; i < 70; i++ {
		if strict.admit(context.Background(), store.Principal{UserID: 21, TokenID: 22}) {
			allowed++
		}
	}
	if allowed != 60 {
		t.Fatalf("default token limit admits %d, want 60", allowed)
	}
}
