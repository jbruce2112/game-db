package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"game-db/internal/config"
	"game-db/internal/store"
)

func testServer(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := config.Config{AppPassword: "secret"}
	h := New(cfg, st, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
	return h.Router()
}

func TestHealth(t *testing.T) {
	srv := httptest.NewServer(testServer(t))
	t.Cleanup(srv.Close)
	res, err := http.Get(srv.URL + "/health")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d", res.StatusCode)
	}
}

func TestLoginAndCRUD(t *testing.T) {
	srv := httptest.NewServer(testServer(t))
	t.Cleanup(srv.Close)
	client := srv.Client()

	bad, _ := json.Marshal(map[string]string{"password": "nope"})
	res, err := client.Post(srv.URL+"/v1/auth/login", "application/json", bytes.NewReader(bad))
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 401 {
		t.Fatalf("want 401 got %d", res.StatusCode)
	}

	ok, _ := json.Marshal(map[string]string{"password": "secret"})
	res, err = client.Post(srv.URL+"/v1/auth/login", "application/json", bytes.NewReader(ok))
	if err != nil {
		t.Fatal(err)
	}
	var login struct{ Token string }
	if err := json.NewDecoder(res.Body).Decode(&login); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if login.Token == "" {
		t.Fatal("empty token")
	}

	create, _ := json.Marshal(map[string]string{
		"title":    "Ocarina of Time",
		"platform": "Nintendo 64",
		"completeness": "cib",
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/library", bytes.NewReader(create))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+login.Token)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 201 {
		b, _ := io.ReadAll(res.Body)
		t.Fatalf("create %d %s", res.StatusCode, b)
	}
	var item map[string]any
	if err := json.NewDecoder(res.Body).Decode(&item); err != nil {
		t.Fatal(err)
	}
	if item["title"] != "Ocarina of Time" {
		t.Fatalf("%v", item)
	}
}

func TestSyncJSONShape(t *testing.T) {
	srv := httptest.NewServer(testServer(t))
	t.Cleanup(srv.Close)
	client := srv.Client()
	ok, _ := json.Marshal(map[string]string{"password": "secret"})
	res, err := client.Post(srv.URL+"/v1/auth/login", "application/json", bytes.NewReader(ok))
	if err != nil {
		t.Fatal(err)
	}
	var login struct{ Token string }
	if err := json.NewDecoder(res.Body).Decode(&login); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()

	body, _ := json.Marshal(map[string]any{
		"cursor":  0,
		"changes": []any{},
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/sync", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+login.Token)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("sync %d %s", res.StatusCode, raw)
	}
	var parsed map[string]any
	if err := json.Unmarshal(raw, &parsed); err != nil {
		t.Fatal(err)
	}
	if _, ok := parsed["cursor"]; !ok {
		t.Fatalf("missing cursor: %s", raw)
	}
	if _, ok := parsed["changes"]; !ok {
		t.Fatalf("missing changes: %s", raw)
	}
}
