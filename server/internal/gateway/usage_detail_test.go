package gateway

import (
	"encoding/json"
	"strings"
	"testing"
	"unicode/utf8"
)

func TestGatewayRecordsOriginalInputOutput(t *testing.T) {
	s := gatewayDB(t)
	p := reviewPrincipal(t, s)
	g := New(s, Options{})
	defer g.Close()
	bindings, err := g.tools(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	var b toolBinding
	for _, v := range bindings {
		if v.row.Key == "echo" {
			b = v
		}
	}
	for _, tc := range []struct {
		name, input string
		truncated   bool
	}{
		{"success", `{"message":"hello","token":"user-visible-secret"}`, false},
		{"error", `{}`, false},
		{"large", `{"message":"` + strings.Repeat("界", 23000) + `"}`, true},
		{"denied", `{"message":"no balance"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if tc.name == "denied" {
				if _, err := s.Pool.Exec(t.Context(), "UPDATE wallets SET balance=0 WHERE id=$1", p.WalletID); err != nil {
					t.Fatal(err)
				}
			}
			result := g.call(t.Context(), p, b, json.RawMessage(tc.input))
			var raw []byte
			if err := s.Pool.QueryRow(t.Context(), "SELECT to_jsonb(l) FROM usage_logs l ORDER BY id DESC LIMIT 1").Scan(&raw); err != nil {
				t.Fatal(err)
			}
			var row struct {
				Input           *string `json:"input_data"`
				Output          *string `json:"output_data"`
				InputTruncated  bool    `json:"input_truncated"`
				OutputTruncated bool    `json:"output_truncated"`
			}
			if err := json.Unmarshal(raw, &row); err != nil {
				t.Fatal(err)
			}
			if row.Input == nil || row.Output == nil {
				t.Fatalf("call input/output not recorded: %s", raw)
			}
			output, err := json.Marshal(result)
			if err != nil {
				t.Fatal(err)
			}
			if !tc.truncated && (*row.Input != tc.input || *row.Output != string(output)) {
				t.Fatalf("original payload changed: %+v", row)
			}
			if row.InputTruncated != tc.truncated || row.OutputTruncated != tc.truncated {
				t.Fatalf("truncation flags: %+v", row)
			}
			for _, value := range []string{*row.Input, *row.Output} {
				if len(value) > 65536 || !utf8.ValidString(value) {
					t.Fatal("payload exceeds byte limit or splits UTF-8")
				}
			}
		})
	}
}
