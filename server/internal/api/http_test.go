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
	"time"

	"game-db/internal/barcode"
	"game-db/internal/config"
	"game-db/internal/model"
	"game-db/internal/pricecharting"
	"game-db/internal/store"
	"game-db/internal/tgdb"
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

func TestLibraryCoverOnDemandURLAndCachedFile(t *testing.T) {
	h := testHandler(t)
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	client := srv.Client()
	token := loginToken(t, client, srv.URL)

	now := time.Now().UTC().Truncate(time.Second)
	igdbID := int64(1905)
	item, err := h.store.Insert(context.Background(), model.Item{
		ID:           "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Title:        "Sunshine",
		Platform:     "GameCube",
		Completeness: "cib",
		IGDBGameID:   &igdbID,
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
	if item.CoverURL == nil || *item.CoverURL != "/v1/library/"+item.ID+"/cover" {
		t.Fatalf("want on-demand cover url, got %v", item.CoverURL)
	}

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/library", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	var list struct {
		Items []map[string]any `json:"items"`
	}
	if err := json.NewDecoder(res.Body).Decode(&list); err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if len(list.Items) != 1 || list.Items[0]["cover_url"] != "/v1/library/"+item.ID+"/cover" {
		t.Fatalf("%v", list.Items)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/v1/library/"+item.ID+"/cover", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 without IGDB, got %d", res.StatusCode)
	}

	coverID := "cccccccc-cccc-cccc-cccc-cccccccccccc"
	payload := []byte("fake-cover")
	name, err := saveCoverFile(h.store.DataDir, coverID, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.SaveCover(context.Background(), coverID, "abc", "image/jpeg", name); err != nil {
		t.Fatal(err)
	}
	if err := h.store.SetCoverID(context.Background(), item.ID, coverID); err != nil {
		t.Fatal(err)
	}

	req, _ = http.NewRequest(http.MethodGet, srv.URL+"/v1/library/"+item.ID+"/cover", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err = client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d %s", res.StatusCode, got)
	}
	if string(got) != string(payload) {
		t.Fatalf("body %q", got)
	}
}

func TestClearCache(t *testing.T) {
	h := testHandler(t)
	now := time.Now().UTC().Truncate(time.Second)
	item, err := h.store.Insert(context.Background(), model.Item{
		ID:           "eeeeeeee-eeee-eeee-eeee-eeeeeeeeeeee",
		Title:        "Sunshine",
		Platform:     "Nintendo GameCube",
		Completeness: "cib",
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
	coverID := "ffffffff-ffff-ffff-ffff-ffffffffffff"
	payload := []byte("cover-bytes")
	name, err := saveCoverFile(h.store.DataDir, coverID, payload)
	if err != nil {
		t.Fatal(err)
	}
	if err := h.store.SaveCover(context.Background(), coverID, "img", "image/jpeg", name); err != nil {
		t.Fatal(err)
	}
	if err := h.store.SetCoverID(context.Background(), item.ID, coverID); err != nil {
		t.Fatal(err)
	}
	if err := h.store.UpsertPriceQuote(context.Background(), store.PriceQuote{
		ItemID: item.ID, QueryKey: "k", PCID: "1", ProductName: "Sunshine", Status: "ok", FetchedAt: now,
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.DB.Exec(`INSERT INTO barcode_cache (barcode, product_title, query, source, updated_at) VALUES ('123','t','q','test',?)`, model.FormatTime(now)); err != nil {
		t.Fatal(err)
	}
	if _, err := h.store.DB.Exec(`INSERT INTO igdb_games (id, name, summary, cover_image_id, first_release_date, platforms_json, updated_at) VALUES (1,'Sunshine','','',NULL,'[]',?)`, model.FormatTime(now)); err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	client := srv.Client()
	token := loginToken(t, client, srv.URL)
	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/v1/cache/clear", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		body, _ := io.ReadAll(res.Body)
		t.Fatalf("status %d %s", res.StatusCode, body)
	}
	var cleared store.CacheClearResult
	if err := json.NewDecoder(res.Body).Decode(&cleared); err != nil {
		t.Fatal(err)
	}
	if cleared.Prices < 1 || cleared.Covers < 1 || cleared.Barcodes < 1 || cleared.Games < 1 {
		t.Fatalf("%+v", cleared)
	}
	if h.store.CoverExists(context.Background(), &coverID) {
		t.Fatal("cover file still present")
	}
	got, err := h.store.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.Title != "Sunshine" {
		t.Fatalf("item %+v", got)
	}
	quotes, err := h.store.PriceQuotes(context.Background(), []string{item.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(quotes) != 0 {
		t.Fatalf("quotes %+v", quotes)
	}
}

type fakeBoxArt struct {
	plats   []tgdb.Platform
	game    tgdb.Game
	err     error
	payload []byte
	ct      string
}

func (f fakeBoxArt) Platforms(context.Context) ([]tgdb.Platform, error) {
	return f.plats, nil
}
func (f fakeBoxArt) SearchFront(context.Context, string, int64) (tgdb.Game, error) {
	if f.err != nil {
		return tgdb.Game{}, f.err
	}
	return f.game, nil
}
func (f fakeBoxArt) Download(context.Context, string) (string, []byte, error) {
	ct := f.ct
	if ct == "" {
		ct = "image/jpeg"
	}
	return ct, f.payload, nil
}

func TestLibraryBoxCover(t *testing.T) {
	h := testHandler(t)
	h.cfg.TheGamesDBAPIKey = "test-key"
	h.tgdb = fakeBoxArt{
		plats:   []tgdb.Platform{{ID: 2, Name: "Nintendo GameCube", Alias: "GC"}},
		game:    tgdb.Game{ID: 42, Title: "Super Mario Sunshine", Platform: 2, FrontURL: "http://cdn.example/front.jpg", SourceID: "tgdb:front.jpg"},
		payload: []byte("box-bytes"),
	}
	now := time.Now().UTC().Truncate(time.Second)
	item, err := h.store.Insert(context.Background(), model.Item{
		ID:           "aaaaaaaa-aaaa-aaaa-aaaa-aaaaaaaaaaaa",
		Title:        "Super Mario Sunshine",
		Platform:     "Nintendo GameCube",
		Completeness: "cib",
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatal(err)
	}

	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	client := srv.Client()
	token := loginToken(t, client, srv.URL)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/library/"+item.ID+"/box-cover", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	got, _ := io.ReadAll(res.Body)
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatalf("status %d %s", res.StatusCode, got)
	}
	if string(got) != "box-bytes" {
		t.Fatalf("body %q", got)
	}

	fresh, err := h.store.Get(context.Background(), item.ID)
	if err != nil {
		t.Fatal(err)
	}
	if fresh.BoxCoverID == nil || !h.store.CoverExists(context.Background(), fresh.BoxCoverID) {
		t.Fatalf("expected cached box cover, got %+v", fresh)
	}
	if fresh.BoxCoverURL == nil || *fresh.BoxCoverURL != "/v1/covers/"+*fresh.BoxCoverID {
		t.Fatalf("box url %v", fresh.BoxCoverURL)
	}
}

func TestLibraryBoxCoverMiss(t *testing.T) {
	h := testHandler(t)
	h.cfg.TheGamesDBAPIKey = "test-key"
	h.tgdb = fakeBoxArt{err: tgdb.ErrNoFront, plats: []tgdb.Platform{{ID: 2, Name: "Nintendo GameCube"}}}
	now := time.Now().UTC().Truncate(time.Second)
	item, err := h.store.Insert(context.Background(), model.Item{
		ID:           "bbbbbbbb-bbbb-bbbb-bbbb-bbbbbbbbbbbb",
		Title:        "Homebrew Cart",
		Platform:     "Nintendo GameCube",
		Completeness: "loose",
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	client := srv.Client()
	token := loginToken(t, client, srv.URL)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/library/"+item.ID+"/box-cover", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != http.StatusNotFound {
		t.Fatalf("want 404 got %d", res.StatusCode)
	}
	if !h.store.BoxCoverMissed(context.Background(), item) {
		t.Fatal("expected miss cached")
	}
}

type fakePrices struct {
	upc    pricecharting.Product
	search []pricecharting.Product
}

func (f fakePrices) ProductByUPC(context.Context, string) (pricecharting.Product, error) {
	return f.upc, nil
}
func (f fakePrices) Search(context.Context, string) ([]pricecharting.Product, error) {
	return f.search, nil
}

func TestLibraryAttachesPriceChartingValue(t *testing.T) {
	h := testHandler(t)
	h.cfg.PriceChartingToken = "test-token"
	loose, cib := 2495, 5999
	h.pc = fakePrices{
		search: []pricecharting.Product{{
			ID: "3584", Name: "Super Mario Sunshine", Console: "Gamecube",
			Loose: &loose, CIB: &cib,
		}},
	}
	now := time.Now().UTC().Truncate(time.Second)
	item, err := h.store.Insert(context.Background(), model.Item{
		ID:           "dddddddd-dddd-dddd-dddd-dddddddddddd",
		Title:        "Super Mario Sunshine",
		Platform:     "Nintendo GameCube",
		Completeness: "cib",
		CreatedAt:    now,
		UpdatedAt:    now,
	})
	if err != nil {
		t.Fatal(err)
	}
	h.ensurePrice(context.Background(), &item)
	if item.Value == nil || item.Value.PCID != "3584" || item.Value.CIBCents == nil || *item.Value.CIBCents != 5999 {
		t.Fatalf("value %+v", item.Value)
	}
	if item.Value.Source != "pricecharting" {
		t.Fatalf("source %q", item.Value.Source)
	}

	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	client := srv.Client()
	token := loginToken(t, client, srv.URL)
	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/library", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := client.Do(req)
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
	if len(list.Items) != 1 {
		t.Fatalf("%v", list.Items)
	}
	val, _ := list.Items[0]["value"].(map[string]any)
	if val["pc_id"] != "3584" || val["product_name"] != "Super Mario Sunshine" {
		t.Fatalf("%v", list.Items[0])
	}
}
