package app

import (
	"context"
	"errors"
	"net/http"

	"github.com/Yanyutin753/PluginPocket/server/internal/settings"
)

type runtimeSettingsView struct {
	settings.Snapshot
	GitHubClientSecretSet bool `json:"github_client_secret_set"`
	SMTPPasswordSet       bool `json:"smtp_password_set"`
}

func (a *application) settingsResponse(w http.ResponseWriter, snapshot settings.Snapshot) {
	respond(w, http.StatusOK, map[string]any{
		"item":                    runtimeSettingsView{Snapshot: snapshot, GitHubClientSecretSet: snapshot.GitHubClientSecret != "", SMTPPasswordSet: snapshot.SMTPPassword != ""},
		"secret_writes_available": len(a.options.Runtime.Key) == 32,
	})
}
func settingsFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, settings.ErrConflict):
		fail(w, http.StatusConflict, "settings_conflict")
	case errors.Is(err, settings.ErrInvalid):
		fail(w, http.StatusBadRequest, "invalid_request")
	case errors.Is(err, settings.ErrEncryptionUnavailable):
		fail(w, http.StatusServiceUnavailable, "settings_encryption_unavailable")
	default:
		fail(w, http.StatusServiceUnavailable, "temporarily_unavailable")
	}
}
func (a *application) getSettings(w http.ResponseWriter, r *http.Request) {
	if _, ok := a.currentUser(w, r, true); !ok {
		return
	}
	if a.options.Runtime == nil {
		fail(w, http.StatusServiceUnavailable, "settings_unavailable")
		return
	}
	snapshot, err := a.options.Runtime.Read(r.Context(), nil)
	if err != nil {
		settingsFailure(w, err)
		return
	}
	a.settingsResponse(w, snapshot)
}
func (a *application) saveSettings(w http.ResponseWriter, r *http.Request) {
	actor, ok := a.currentUser(w, r, true)
	if !ok {
		return
	}
	runtime := a.options.Runtime
	if runtime == nil {
		fail(w, http.StatusServiceUnavailable, "settings_unavailable")
		return
	}
	var in struct {
		AccessTokenSeconds  *int    `json:"access_token_seconds"`
		RefreshTokenSeconds *int    `json:"refresh_token_seconds"`
		Revision            *int64  `json:"revision"`
		InitialCredits      *int64  `json:"initial_credits"`
		GitHubEnabled       *bool   `json:"github_enabled"`
		GitHubClientID      *string `json:"github_client_id"`
		GitHubOrg           *string `json:"github_org"`
		SMTPEnabled         *bool   `json:"smtp_enabled"`
		SMTPAddress         *string `json:"smtp_address"`
		SMTPFrom            *string `json:"smtp_from"`
		SMTPUsername        *string `json:"smtp_username"`
		GitHubClientSecret  *string `json:"github_client_secret"`
		SMTPPassword        *string `json:"smtp_password"`
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Revision == nil || *in.Revision < 0 || in.InitialCredits == nil || in.GitHubEnabled == nil || in.GitHubClientID == nil || in.GitHubOrg == nil || in.SMTPEnabled == nil || in.SMTPAddress == nil || in.SMTPFrom == nil || in.SMTPUsername == nil {
		fail(w, http.StatusBadRequest, "invalid_request")
		return
	}
	tx, err := a.s.Pool.Begin(r.Context())
	if err != nil {
		settingsFailure(w, err)
		return
	}
	defer func() { _ = tx.Rollback(context.Background()) }()
	if _, err = tx.Exec(r.Context(), "SELECT pg_advisory_xact_lock($1)", settings.LockID); err != nil {
		settingsFailure(w, err)
		return
	}
	current, err := runtime.Read(r.Context(), tx)
	if err != nil {
		settingsFailure(w, err)
		return
	}
	if current.Revision != *in.Revision {
		settingsFailure(w, settings.ErrConflict)
		return
	}
	values := settings.Values{InitialCredits: *in.InitialCredits, GitHubEnabled: *in.GitHubEnabled, GitHubClientID: *in.GitHubClientID, GitHubOrg: *in.GitHubOrg, SMTPEnabled: *in.SMTPEnabled, SMTPAddress: *in.SMTPAddress, SMTPFrom: *in.SMTPFrom, SMTPUsername: *in.SMTPUsername, GitHubClientSecret: current.GitHubClientSecret, SMTPPassword: current.SMTPPassword}
	values.AccessTokenSeconds = current.AccessTokenSeconds
	values.RefreshTokenSeconds = current.RefreshTokenSeconds
	if in.AccessTokenSeconds != nil {
		if *in.AccessTokenSeconds <= 0 {
			fail(w, 400, "invalid_request")
			return
		}
		values.AccessTokenSeconds = *in.AccessTokenSeconds
	}
	if in.RefreshTokenSeconds != nil {
		if *in.RefreshTokenSeconds <= 0 {
			fail(w, 400, "invalid_request")
			return
		}
		values.RefreshTokenSeconds = *in.RefreshTokenSeconds
	}
	if in.GitHubClientSecret != nil {
		values.GitHubClientSecret = *in.GitHubClientSecret
	}
	if in.SMTPPassword != nil {
		values.SMTPPassword = *in.SMTPPassword
	}
	if err = values.Validate(runtime.Origin); err != nil {
		settingsFailure(w, err)
		return
	}
	if err = runtime.Write(r.Context(), tx, values, *in.Revision, actor.ID); err != nil {
		settingsFailure(w, err)
		return
	}
	// Write may wait on updated_by's user foreign key; authorize after it finishes.
	if _, ok = currentUserQuery(w, r, true, tx); !ok {
		return
	}
	if err = tx.Commit(r.Context()); err != nil {
		settingsFailure(w, err)
		return
	}
	a.settingsResponse(w, settings.Snapshot{Values: values, Revision: *in.Revision + 1})
}
