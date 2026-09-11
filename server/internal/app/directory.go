package app

import (
	"errors"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"
)

// Public metadata only. Credentials, endpoints and skill source files are not
// exposed by the anonymous catalog; authenticated management stays separate.
type directoryPlugin struct {
	Slug        string `json:"slug"`
	Name        string `json:"name"`
	Description string `json:"description"`
	Kind        string `json:"kind"`
	Version     string `json:"version"`
	Gateway     bool   `json:"gateway"`
}

const publicPluginColumns = "slug,name,description,kind,version,(transport='gateway')"

func (a *application) pluginDirectory(w http.ResponseWriter, r *http.Request) {
	rows, err := a.s.Pool.Query(r.Context(), "SELECT "+publicPluginColumns+" FROM marketplace_items WHERE transport!='stdio' ORDER BY (source='curated') DESC, stars DESC, id")
	if err != nil {
		fail(w, 503, "directory_unavailable")
		return
	}
	defer rows.Close()
	items := []directoryPlugin{}
	for rows.Next() {
		var item directoryPlugin
		if err = rows.Scan(&item.Slug, &item.Name, &item.Description, &item.Kind, &item.Version, &item.Gateway); err != nil {
			fail(w, 503, "directory_unavailable")
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		fail(w, 503, "directory_unavailable")
		return
	}
	respond(w, 200, map[string]any{"items": items, "origin": strings.TrimRight(a.options.Origin, "/")})
}

func (a *application) pluginDetail(w http.ResponseWriter, r *http.Request) {
	var item directoryPlugin
	err := a.s.Pool.QueryRow(r.Context(), "SELECT "+publicPluginColumns+" FROM marketplace_items WHERE slug=$1 AND transport!='stdio'", r.PathValue("slug")).Scan(&item.Slug, &item.Name, &item.Description, &item.Kind, &item.Version, &item.Gateway)
	if errors.Is(err, pgx.ErrNoRows) {
		fail(w, 404, "not_found")
		return
	}
	if err != nil {
		fail(w, 503, "directory_unavailable")
		return
	}
	respond(w, 200, map[string]any{"item": item, "origin": strings.TrimRight(a.options.Origin, "/")})
}
