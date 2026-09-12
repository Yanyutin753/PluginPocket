package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

const billingRolesPath = "/api/v1/admin/billing-roles"

func roleListResponse(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	if w.Code != http.StatusOK {
		t.Fatalf("billing roles response %d: %s", w.Code, w.Body)
	}
	var out struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &out); err != nil {
		t.Fatal(err)
	}
	return out.Items
}

func TestBillingRolesAdminListUpdateAssignAndUsageDisplay(t *testing.T) {
	s, h := setup(t)
	admin := register(t, h, "roleadmin")
	user := register(t, h, "roleuser")
	if _, err := s.Pool.Exec(t.Context(), "UPDATE users SET role='admin' WHERE username='roleadmin'"); err != nil {
		t.Fatal(err)
	}

	// 非管理员不可访问
	if w := request(h, http.MethodGet, billingRolesPath, "", user); w.Code == http.StatusOK {
		t.Fatal("non-admin must not list billing roles")
	}

	// 种子三档
	items := roleListResponse(t, request(h, http.MethodGet, billingRolesPath, "", admin))
	if len(items) != 3 {
		t.Fatalf("seed roles: %v", items)
	}

	// 修改 vip 倍率与描述
	w := request(h, http.MethodPatch, billingRolesPath+"/vip", `{"multiplier_bp":6000,"description":"六折"}`, admin)
	if w.Code != http.StatusOK {
		t.Fatalf("patch vip %d: %s", w.Code, w.Body)
	}
	items = roleListResponse(t, request(h, http.MethodGet, billingRolesPath, "", admin))
	for _, item := range items {
		if item["name"] == "vip" && (item["multiplier_bp"] != float64(6000) || item["description"] != "六折") {
			t.Fatalf("vip not updated: %v", item)
		}
	}

	// 边界：未知角色 404、非法倍率 400
	if w = request(h, http.MethodPatch, billingRolesPath+"/ghost", `{"multiplier_bp":1}`, admin); w.Code != http.StatusNotFound {
		t.Fatalf("unknown role patch: %d %s", w.Code, w.Body)
	}
	if w = request(h, http.MethodPatch, billingRolesPath+"/vip", `{"multiplier_bp":1000001}`, admin); w.Code != http.StatusBadRequest {
		t.Fatalf("invalid multiplier: %d %s", w.Code, w.Body)
	}
	if w = request(h, http.MethodPatch, billingRolesPath+"/vip", `{"multiplier_bp":-1}`, admin); w.Code != http.StatusBadRequest {
		t.Fatalf("negative multiplier: %d %s", w.Code, w.Body)
	}

	// 指派用户计费角色：PATCH 响应带 billing_role
	var userID int64
	if err := s.Pool.QueryRow(t.Context(), "SELECT id FROM users WHERE username='roleuser'").Scan(&userID); err != nil {
		t.Fatal(err)
	}
	userPath := "/api/v1/admin/users/" + strconv.FormatInt(userID, 10)
	w = request(h, http.MethodPatch, userPath, `{"billing_role":"vip"}`, admin)
	if w.Code != http.StatusOK {
		t.Fatalf("assign role %d: %s", w.Code, w.Body)
	}
	var updated struct {
		User struct {
			BillingRole string `json:"billing_role"`
		} `json:"user"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &updated); err != nil {
		t.Fatal(err)
	}
	if updated.User.BillingRole != "vip" {
		t.Fatalf("assigned billing_role=%q", updated.User.BillingRole)
	}
	// 未知角色赋值被拒
	if w = request(h, http.MethodPatch, userPath, `{"billing_role":"ghost"}`, admin); w.Code != http.StatusBadRequest {
		t.Fatalf("unknown role assignment: %d %s", w.Code, w.Body)
	}

	// 管理员用户列表透出 billing_role
	w = request(h, http.MethodGet, "/api/v1/admin/users", "", admin)
	if w.Code != http.StatusOK {
		t.Fatalf("users list %d: %s", w.Code, w.Body)
	}
	var users struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &users); err != nil {
		t.Fatal(err)
	}
	found := false
	for _, item := range users.Items {
		if item["username"] == "roleuser" {
			found = true
			if item["billing_role"] != "vip" {
				t.Fatalf("users list billing_role=%v", item["billing_role"])
			}
		}
	}
	if !found {
		t.Fatal("roleuser missing from admin users list")
	}

	// 用量行与详情透出生效角色和倍率（先建令牌，用量行挂在令牌上）
	if w = request(h, http.MethodPost, "/api/v1/account/tokens", `{"name":"roles"}`, user); w.Code != http.StatusCreated {
		t.Fatalf("create token %d: %s", w.Code, w.Body)
	}
	if _, err := s.Pool.Exec(t.Context(), `INSERT INTO usage_logs(user_id,token_id,wallet_id,tool,cost,requested_cost,status,request_key,billing_role,multiplier_bp)
 SELECT t.user_id,t.id,t.wallet_id,'echo',3,3,'ok','role-api-test','vip',6000 FROM tokens t WHERE t.user_id=$1 AND t.revoked_at IS NULL LIMIT 1`, userID); err != nil {
		t.Fatal(err)
	}
	w = request(h, http.MethodGet, "/api/v1/account/usage", "", user)
	if w.Code != http.StatusOK {
		t.Fatalf("usage list %d: %s", w.Code, w.Body)
	}
	var usage struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &usage); err != nil {
		t.Fatal(err)
	}
	if len(usage.Items) == 0 {
		t.Fatal("usage list empty")
	}
	if usage.Items[0]["billing_role"] != "vip" || usage.Items[0]["multiplier_bp"] != float64(6000) {
		t.Fatalf("usage item role fields: %v", usage.Items[0])
	}
	callID := strconv.FormatInt(int64(usage.Items[0]["id"].(float64)), 10)
	w = request(h, http.MethodGet, "/api/v1/account/usage/"+callID, "", user)
	if w.Code != http.StatusOK {
		t.Fatalf("usage detail %d: %s", w.Code, w.Body)
	}
	var detail struct {
		Item map[string]any `json:"item"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &detail); err != nil {
		t.Fatal(err)
	}
	if detail.Item["billing_role"] != "vip" || detail.Item["multiplier_bp"] != float64(6000) {
		t.Fatalf("usage detail role fields: %v", detail.Item)
	}
}

func TestToolAllowedRolesRoundTrip(t *testing.T) {
	s, h := setup(t)
	admin := register(t, h, "tooladmin")
	if _, err := s.Pool.Exec(t.Context(), "UPDATE users SET role='admin' WHERE username='tooladmin'"); err != nil {
		t.Fatal(err)
	}
	body := `{"key":"echo","name":"Echo","description":"","kind":"builtin","enabled":true,"units_per_call":1,"input_schema":{"type":"object"},"allowed_roles":["vip"]}`
	w := request(h, http.MethodPatch, "/api/v1/admin/tools/1", body, admin)
	if w.Code != http.StatusOK {
		t.Fatalf("patch gated tool %d: %s", w.Code, w.Body)
	}
	w = request(h, http.MethodGet, "/api/v1/admin/tools", "", admin)
	if w.Code != http.StatusOK {
		t.Fatalf("tools list %d: %s", w.Code, w.Body)
	}
	var tools struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &tools); err != nil {
		t.Fatal(err)
	}
	for _, item := range tools.Items {
		if item["key"] == "echo" {
			roles, ok := item["allowed_roles"].([]any)
			if !ok || len(roles) != 1 || roles[0] != "vip" {
				t.Fatalf("echo allowed_roles=%v", item["allowed_roles"])
			}
			return
		}
	}
	t.Fatal("echo missing from tools list")
}
