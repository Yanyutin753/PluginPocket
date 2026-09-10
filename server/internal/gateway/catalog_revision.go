package gateway

import (
	"context"
	"time"
)

// A committed configuration update is visible to every replica before it uses
// its local catalog. Notifications alone would lose invalidations on reconnect.
func (g *Gateway) observeCatalogRevision(ctx context.Context) error {
	var revision int64
	if err := g.store.Pool.QueryRow(ctx, "SELECT revision FROM tool_catalog_revision WHERE singleton").Scan(&revision); err != nil {
		return err
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if revision > g.catalogRevision {
		g.catalogRevision = revision
		g.catalogVersion++
		g.catalogUntil = time.Time{}
	}
	return nil
}
