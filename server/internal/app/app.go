package app

import (
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/Yanyutin753/loadout/server/internal/auth"
	"github.com/Yanyutin753/loadout/server/internal/gateway"
	"github.com/Yanyutin753/loadout/server/internal/httpapi"
	"github.com/Yanyutin753/loadout/server/internal/marketplace"
	"github.com/Yanyutin753/loadout/server/internal/settings"
	"github.com/Yanyutin753/loadout/server/internal/store"
)

type Options struct {
	Runtime        *settings.Manager
	InitialCredits *int64
	Gateway        *gateway.Gateway
	EncryptionKey  []byte
	SecureCookies  bool
	Origin         string
	Marketplace    marketplace.Options
}
type application struct {
	s       *store.Store
	options Options
}

func New(s *store.Store, options Options) http.Handler {
	a := &application{s: s, options: options}
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/v1/auth/register", a.register)
	mux.HandleFunc("POST /api/v1/auth/login", a.login)
	mux.HandleFunc("POST /api/v1/auth/logout", a.logout)
	mux.HandleFunc("POST /api/v1/auth/refresh", a.refresh)
	mux.HandleFunc("GET /api/v1/account/me", a.me)
	mux.HandleFunc("GET /api/v1/account/verify", a.verify)
	mux.HandleFunc("GET /api/v1/account/tokens", a.listTokens)
	mux.HandleFunc("POST /api/v1/account/tokens", a.createToken)
	mux.HandleFunc("DELETE /api/v1/account/tokens/{id}", a.revokeToken)
	mux.HandleFunc("GET /api/v1/account/usage", a.usage)
	mux.HandleFunc("GET /api/v1/account/usage/{callID}", a.usageDetail)
	mux.HandleFunc("GET /api/v1/admin/usage/{callID}", a.usageDetail)
	mux.HandleFunc("GET /api/v1/account/teams/{id}/usage/{callID}", a.usageDetail)
	mux.HandleFunc("GET /api/v1/admin/settings", a.getSettings)
	mux.HandleFunc("PATCH /api/v1/admin/settings", a.saveSettings)
	mux.HandleFunc("GET /api/v1/admin/users", a.users)
	mux.HandleFunc("PATCH /api/v1/admin/users/{id}", a.setUserEnabled)
	mux.HandleFunc("GET /api/v1/tools", a.listTools)
	mux.HandleFunc("GET /api/v1/admin/tools", a.listTools)
	mux.HandleFunc("POST /api/v1/admin/tools", a.saveTool)
	mux.HandleFunc("POST /api/v1/admin/tools/settlement-preview", a.previewSettlement)
	mux.HandleFunc("PATCH /api/v1/admin/tools/{id}", a.saveTool)
	mux.HandleFunc("GET /api/v1/plugins", a.pluginDirectory)
	mux.HandleFunc("GET /api/v1/plugins/{slug}", a.pluginDetail)
	mux.HandleFunc("GET /api/v1/admin/marketplace", a.listMarketplace)
	mux.HandleFunc("GET /api/v1/marketplace", a.publicMarketplace)
	mux.HandleFunc("GET /api/v1/marketplace/{slug}/files", a.marketplaceFiles)
	mux.HandleFunc("POST /api/v1/admin/marketplace/tools", a.publishPoolTool)
	mux.HandleFunc("POST /api/v1/admin/marketplace/skills", a.createSkill)
	mux.HandleFunc("POST /api/v1/admin/marketplace/bundles", a.createBundle)
	mux.HandleFunc("POST /api/v1/admin/marketplace/sync", a.syncMarketplace)
	mux.HandleFunc("POST /api/v1/admin/marketplace/install", a.installMarketplace)
	mux.HandleFunc("POST /api/v1/admin/marketplace/uninstall", a.uninstallMarketplace)
	mux.HandleFunc("GET /api/v1/admin/tools/{id}/upstream", a.upstreamTools)
	mux.HandleFunc("PUT /api/v1/admin/tools/{id}/metadata", a.saveMetadata)
	mux.HandleFunc("DELETE /api/v1/admin/tools/{id}/metadata/{name}", a.deleteMetadata)
	mux.HandleFunc("GET /api/v1/admin/usage", a.globalUsage)
	mux.HandleFunc("GET /api/v1/admin/usage/export", a.globalUsage)
	mux.HandleFunc("POST /api/v1/admin/users/{id}/balance", a.adjustBalance)

	mux.HandleFunc("GET /api/v1/plans", a.plans)
	mux.HandleFunc("GET /api/v1/admin/plans", a.plans)
	mux.HandleFunc("POST /api/v1/admin/plans", a.savePlan)
	mux.HandleFunc("PATCH /api/v1/admin/plans/{id}", a.savePlan)
	mux.HandleFunc("GET /api/v1/admin/redemption-codes", a.codes)
	mux.HandleFunc("POST /api/v1/admin/redemption-codes", a.createCode)
	mux.HandleFunc("POST /api/v1/account/redeem", a.redeem)
	mux.HandleFunc("GET /api/v1/account/orders", a.orders)
	mux.HandleFunc("POST /api/v1/account/orders", a.createOrder)
	mux.HandleFunc("GET /api/v1/account/ledger", a.ledger)
	mux.HandleFunc("GET /api/v1/admin/ledger", a.adminLedger)
	mux.HandleFunc("GET /api/v1/account/teams", a.teams)
	mux.HandleFunc("POST /api/v1/account/teams", a.createTeam)
	mux.HandleFunc("GET /api/v1/account/teams/{id}", a.team)
	mux.HandleFunc("PATCH /api/v1/account/teams/{id}", a.updateTeam)
	mux.HandleFunc("GET /api/v1/account/teams/{id}/members", a.members)
	mux.HandleFunc("DELETE /api/v1/account/teams/{id}/members/{user_id}", a.removeMember)
	mux.HandleFunc("POST /api/v1/account/teams/{id}/invites", a.createInvite)
	mux.HandleFunc("POST /api/v1/account/team-invites/accept", a.acceptInvite)
	mux.HandleFunc("POST /api/v1/account/teams/{id}/fund", a.fundTeam)
	mux.HandleFunc("GET /api/v1/account/teams/{id}/usage", a.teamUsage)
	mux.HandleFunc("GET /api/v1/account/teams/{id}/usage/export", a.teamUsage)
	mux.HandleFunc("POST /api/v1/device/authorize", a.deviceAuthorize)
	mux.HandleFunc("POST /api/v1/device/token", a.deviceToken)
	mux.HandleFunc("POST /api/v1/account/devices/approve", a.approveDevice)
	mux.HandleFunc("GET /api/v1/admin/usage/summary", a.globalSummary)
	mux.HandleFunc("GET /api/v1/account/teams/{id}/usage/summary", a.teamSummary)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		if r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS" && !a.sameOrigin(r) && r.URL.Path != "/api/v1/device/authorize" && r.URL.Path != "/api/v1/device/token" {
			fail(w, 403, "forbidden_origin")
			return
		}
		if r.Method == http.MethodPost && (r.URL.Path == "/api/v1/auth/login" || r.URL.Path == "/api/v1/auth/register" || r.URL.Path == "/api/v1/device/authorize" || r.URL.Path == "/api/v1/device/token" || r.URL.Path == "/api/v1/account/devices/approve") {
			allowed, err := auth.AllowPublicRequest(r.Context(), a.s, r, r.URL.Path)
			if err != nil {
				fail(w, 503, "rate_limit_unavailable")
				return
			}
			if !allowed {
				w.Header().Set("Retry-After", "60")
				fail(w, 429, "rate_limited")
				return
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func (a *application) sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return false
	}
	if a.options.Origin != "" {
		return origin == a.options.Origin
	}
	u, e := url.Parse(origin)
	return e == nil && u.Host == r.Host && ((r.TLS != nil && u.Scheme == "https") || (r.TLS == nil && u.Scheme == "http"))
}
func respond(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code string) {
	httpapi.Fail(w, status, code)
}
