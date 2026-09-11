package gateway

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestDuplicateUpstreamDefinitionsLeaveHealthyCatalogAvailable(t *testing.T) {
	s := gatewayDB(t)
	bad := boundaryUpstream(t, "ping", "duplicate")
	original := bad.Config.Handler
	bad.Config.Handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		recorded := httptest.NewRecorder()
		original.ServeHTTP(recorded, r)
		var payload struct {
			Result struct {
				Tools []json.RawMessage `json:"tools"`
			} `json:"result"`
		}
		body := recorded.Body.Bytes()
		if json.Unmarshal(body, &payload) == nil && len(payload.Result.Tools) > 0 {
			var envelope map[string]any
			if err := json.Unmarshal(body, &envelope); err != nil {
				t.Error(err)
				return
			}
			result := envelope["result"].(map[string]any)
			tools := result["tools"].([]any)
			result["tools"] = append(tools, tools[0])
			var err error
			body, err = json.Marshal(envelope)
			if err != nil {
				t.Error(err)
				return
			}
		}
		for name, values := range recorded.Header() {
			w.Header()[name] = values
		}
		w.WriteHeader(recorded.Code)
		_, _ = w.Write(body)
	})
	healthy := boundaryUpstream(t, "ping", "healthy")
	key := bytes.Repeat([]byte{9}, 32)
	g := New(s, Options{EncryptionKey: key, AllowPrivate: true})
	t.Cleanup(g.Close)
	for _, fixture := range []struct{ key, url string }{{"duplicate", bad.URL}, {"healthy", healthy.URL}} {
		sealed, err := SealConfig(key, []byte(`{"url":"`+fixture.url+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		if _, err = s.Pool.Exec(t.Context(), "INSERT INTO tools(key,name,kind,config) VALUES($1,$1,'http',$2)", fixture.key, sealed); err != nil {
			t.Fatal(err)
		}
	}
	bindings, err := g.tools(t.Context())
	if err != nil {
		t.Fatalf("one invalid upstream made the entire catalog unavailable: %v", err)
	}
	names := map[string]bool{}
	for _, binding := range bindings {
		names[binding.definition.Name] = true
	}
	if len(bindings) != 6 || !names["echo"] || !names["time_now"] || !names["healthy__ping"] {
		t.Fatalf("healthy tools lost or duplicate provider exposed: names=%v", names)
	}
}
