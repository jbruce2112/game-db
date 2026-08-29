package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"game-db/internal/barcode"
	"game-db/internal/config"
	"game-db/internal/store"
)

func testHandler(t *testing.T) *Handler {
	t.Helper()
	st, err := store.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	cfg := config.Config{AppPassword: "secret"}
	return New(cfg, st, nil, slog.New(slog.NewTextHandler(io.Discard, nil)), nil)
}

func testServer(t *testing.T) http.Handler {
	t.Helper()
	return testHandler(t).Router()
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
		"title":        "Ocarina of Time",
		"platform":     "Nintendo 64",
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

func TestImportCSVReplacesLibrary(t *testing.T) {
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
	auth := func(req *http.Request) { req.Header.Set("Authorization", "Bearer "+login.Token) }

	create, _ := json.Marshal(map[string]string{"title": "Old Game", "platform": "NES"})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/library", bytes.NewReader(create))
	req.Header.Set("Content-Type", "application/json")
	auth(req)
	res, _ = client.Do(req)
	res.Body.Close()

	csvBody := "title,platform,region,completeness,notes\nSuper Mario Sunshine,Nintendo GameCube,us,cib,amazing\n"
	req, _ = http.NewRequest(http.MethodPost, srv.URL+"/v1/library/import", strings.NewReader(csvBody))
	req.Header.Set("Content-Type", "text/csv")
	auth(req)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("import %d %s", res.StatusCode, raw)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/v1/library", nil)
	auth(req)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var list struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	if len(list.Items) != 1 || list.Items[0]["title"] != "Super Mario Sunshine" {
		t.Fatalf("%v", list.Items)
	}
}

func TestExportCSV(t *testing.T) {
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

	create, _ := json.Marshal(map[string]string{"title": "Mario, Sunshine", "platform": "GameCube", "notes": `say "hi"`})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/library", bytes.NewReader(create))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+login.Token)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 201 {
		t.Fatalf("create %d", res.StatusCode)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/v1/library.csv", nil)
	req.Header.Set("Authorization", "Bearer "+login.Token)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("csv %d %s", res.StatusCode, body)
	}
	ct := res.Header.Get("Content-Type")
	if !strings.Contains(ct, "text/csv") {
		t.Fatalf("content-type %s", ct)
	}
	s := string(body)
	if !strings.Contains(s, `"Mario, Sunshine"`) {
		t.Fatalf("csv body: %s", s)
	}
}

func loginToken(t *testing.T, client *http.Client, base string) string {
	t.Helper()
	ok, _ := json.Marshal(map[string]string{"password": "secret"})
	res, err := client.Post(base+"/v1/auth/login", "application/json", bytes.NewReader(ok))
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var login struct{ Token string }
	if err := json.NewDecoder(res.Body).Decode(&login); err != nil {
		t.Fatal(err)
	}
	if login.Token == "" {
		t.Fatal("empty token")
	}
	return login.Token
}

func TestSearchBarcode(t *testing.T) {
	h := testHandler(t)
	h.productLookup = func(ctx context.Context, codes []string) (barcode.Product, error) {
		return barcode.Product{Title: "Battlefield Bad Company 2 (PC DVD)", Source: "test"}, nil
	}
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	client := srv.Client()
	token := loginToken(t, client, srv.URL)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/search/barcode?q=014633190366", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		t.Fatalf("status %d %s", res.StatusCode, raw)
	}
	var got map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatal(err)
	}
	if got["barcode"] != "014633190366" {
		t.Fatalf("%s", raw)
	}
	if got["product_title"] != "Battlefield Bad Company 2 (PC DVD)" {
		t.Fatalf("%s", raw)
	}
	if got["query"] != "Battlefield Bad Company 2" {
		t.Fatalf("query %s", raw)
	}
}

func TestCreateStoresBarcode(t *testing.T) {
	h := testHandler(t)
	h.productLookup = func(context.Context, []string) (barcode.Product, error) {
		return barcode.Product{}, nil
	}
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	client := srv.Client()
	token := loginToken(t, client, srv.URL)

	create, _ := json.Marshal(map[string]string{
		"title":    "Sunshine",
		"platform": "GameCube",
		"barcode":  "0 45496 59037 6",
	})
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/library", bytes.NewReader(create))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
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
	if item["barcode"] != "045496590376" {
		t.Fatalf("%v", item)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/v1/search/barcode?q=0045496590376", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var got struct {
		Owned []map[string]any `json:"owned"`
	}
	if err := json.NewDecoder(res.Body).Decode(&got); err != nil {
		t.Fatal(err)
	}
	if len(got.Owned) != 1 || got.Owned[0]["title"] != "Sunshine" {
		t.Fatalf("%v", got.Owned)
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
