package httpapi

import (
	"encoding/json"
	"net/http"
	"sort"
)

// Fail writes the standardized error envelope {"error": code}. It is the only
// sanctioned way to fail an API request; codes are pinned in the registry
// below so clients (web/src/i18n/errors.ts, docs/API.md, openapi.json) can
// map them without parsing prose.
func Fail(w http.ResponseWriter, status int, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

// registry maps every wire error code to the HTTP statuses it may use. Keep
// sorted; the contract golden file and docs are locked to this table by
// codes_test.go.
var registry = map[string][]int{
	"already_installed":               {409},
	"already_member":                  {409},
	"already_redeemed":                {409},
	"auth_busy":                       {429},
	"authorization_pending":           {400},
	"cannot_disable_self":             {409},
	"config_required":                 {400},
	"directory_unavailable":           {503},
	"email_unavailable":               {409, 503}, // 409: address taken; 503: SMTP down
	"expired_token":                   {400},
	"forbidden":                       {403},
	"forbidden_origin":                {403},
	"gateway_unavailable":             {503},
	"github_failed":                   {502},
	"github_unavailable":              {503},
	"idempotency_conflict":            {409},
	"insufficient_balance":            {409},
	"internal_error":                  {500},
	"invalid_credentials":             {401},
	"invalid_device_code":             {409},
	"invalid_grant":                   {400},
	"invalid_icon":                    {400},
	"invalid_request":                 {400},
	"invalid_settlement":              {400},
	"invalid_state":                   {400},
	"invalid_token":                   {400},
	"invite_expired":                  {410},
	"invite_used":                     {409},
	"key_taken":                       {409},
	"last_admin":                      {409},
	"last_owner":                      {409},
	"marketplace_unavailable":         {503},
	"method_not_allowed":              {405},
	"not_found":                       {404},
	"not_installed":                   {409},
	"organization_required":           {403},
	"payment_unavailable":             {503},
	"rate_limited":                    {429},
	"rate_limit_unavailable":          {503},
	"registration_unavailable":        {503},
	"seats_in_use":                    {409},
	"settings_conflict":               {409},
	"settings_encryption_unavailable": {503},
	"settings_unavailable":            {503},
	"slow_down":                       {429},
	"team_full":                       {409},
	"temporarily_unavailable":         {503},
	"tool_disabled":                   {409},
	"transport_not_supported":         {400},
	"transport_required":              {400},
	"unauthorized":                    {401},
	"upstream_unavailable":            {502, 503},
	"username_taken":                  {409},
}

// Codes returns every registered error code, sorted.
func Codes() []string {
	codes := make([]string, 0, len(registry))
	for code := range registry {
		codes = append(codes, code)
	}
	sort.Strings(codes)
	return codes
}

func statusesFor(code string) []int {
	return registry[code]
}

// AllowedStatus reports whether status is one of the registered HTTP statuses
// for code.
func AllowedStatus(code string, status int) bool {
	for _, s := range registry[code] {
		if s == status {
			return true
		}
	}
	return false
}
