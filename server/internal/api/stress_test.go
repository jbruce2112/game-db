package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"game-db/internal/barcode"
	"game-db/internal/export"
	"game-db/internal/model"
	"game-db/internal/store"
)

func stressScale() int {
	switch strings.ToLower(strings.TrimSpace(os.Getenv("STRESS"))) {
	case "1", "true", "yes", "extreme":
		return 10
	default:
		return 1
	}
}

// TestExtremeStress hammers the HTTP API the way several phones plus the web
// UI would: concurrent CRUD, last-write-wins sync from many devices, a large
// CSV round-trip, wipe-and-replace import, and noisy barcode lookups.
//
// Default scale finishes in a few seconds. For the full run:
//
//	STRESS=1 go test -count=1 -timeout 20m -run TestExtremeStress ./internal/api
func TestExtremeStress(t *testing.T) {
	scale := stressScale()
	nItems := 250 * scale
	nWorkers := 16
	nDevices := 5
	if scale > 1 {
		nWorkers = 32
		nDevices = 8
	}
	nBarcode := 40 * scale
	syncRounds := 8
	if scale > 1 {
		syncRounds = 16
	}

	t.Logf("scale=%d items=%d workers=%d devices=%d barcodes=%d rounds=%d",
		scale, nItems, nWorkers, nDevices, nBarcode, syncRounds)

	h := testHandler(t)
	h.productLookup = func(_ context.Context, codes []string) (barcode.Product, error) {
		code := "unknown"
		if len(codes) > 0 {
			code = codes[0]
		}
		return barcode.Product{
			Title:  fmt.Sprintf("Stress Title %s  Atlus  PlayStation 4  [Physical]  %s", code, code),
			Source: "test",
		}, nil
	}
	srv := httptest.NewServer(h.Router())
	t.Cleanup(srv.Close)
	client := srv.Client()
	client.Timeout = 90 * time.Second

	token := loginToken(t, client, srv.URL)
	start := time.Now()

	t.Run("login_storm", func(t *testing.T) {
		var bad, good atomic.Int64
		parallel(t, nWorkers*2, func(i int) error {
			pw := "secret"
			if i%3 == 0 {
				pw = "nope"
			}
			tok, err := tryLogin(client, srv.URL, pw)
			if pw == "secret" {
				if err != nil || tok == "" {
					return fmt.Errorf("good login failed: %v", err)
				}
				good.Add(1)
			} else {
				if err == nil {
					return fmt.Errorf("bad login accepted")
				}
				bad.Add(1)
			}
			return nil
		})
		if good.Load() == 0 || bad.Load() == 0 {
			t.Fatalf("good=%d bad=%d", good.Load(), bad.Load())
		}
	})

	ids := make([]string, nItems)
	t.Run("concurrent_create", func(t *testing.T) {
		var mu sync.Mutex
		parallel(t, nWorkers, func(i int) error {
			for n := i; n < nItems; n += nWorkers {
				body := map[string]any{
					"title":        stressTitle(n),
					"platform":     stressPlatform(n),
					"completeness": []string{"unknown", "loose", "cib", "new"}[n%4],
					"notes":        fmt.Sprintf("note %d, \"quoted\"", n),
					"region":       []string{"us", "eu", "jp", "au", "other"}[n%5],
					"barcode":      stressBarcode(n),
				}
				var item model.Item
				if err := doJSON(client, token, http.MethodPost, srv.URL+"/v1/library", body, http.StatusCreated, &item); err != nil {
					return err
				}
				if item.ID == "" || item.Title == "" {
					return fmt.Errorf("empty create %d", n)
				}
				mu.Lock()
				ids[n] = item.ID
				mu.Unlock()
			}
			return nil
		})
	})

	t.Run("list_and_filter", func(t *testing.T) {
		var list struct {
			Items []model.Item `json:"items"`
		}
		if err := doJSON(client, token, http.MethodGet, srv.URL+"/v1/library", nil, 200, &list); err != nil {
			t.Fatal(err)
		}
		if len(list.Items) != nItems {
			t.Fatalf("list %d want %d", len(list.Items), nItems)
		}
		seen := map[string]struct{}{}
		for _, it := range list.Items {
			if _, ok := seen[it.ID]; ok {
				t.Fatalf("duplicate id %s", it.ID)
			}
			seen[it.ID] = struct{}{}
			if it.Title == "" || it.Platform == "" {
				t.Fatalf("blank row %+v", it)
			}
			if it.DeletedAt != nil {
				t.Fatalf("list leaked tombstone %s", it.ID)
			}
		}
		parallel(t, nWorkers, func(i int) error {
			q := srv.URL + "/v1/library?q=Stress&sort=title"
			if i%2 == 0 {
				q += "&platform=Nintendo%20Switch"
			}
			var got struct {
				Items []model.Item `json:"items"`
			}
			return doJSON(client, token, http.MethodGet, q, nil, 200, &got)
		})
	})

	t.Run("concurrent_patch_get_delete", func(t *testing.T) {
		parallel(t, nWorkers, func(i int) error {
			for n := i; n < nItems; n += nWorkers {
				id := ids[n]
				if id == "" {
					return fmt.Errorf("missing id %d", n)
				}
				if n%11 == 0 {
					var got model.Item
					if err := doJSON(client, token, http.MethodGet, srv.URL+"/v1/library/"+id, nil, 200, &got); err != nil {
						return err
					}
					continue
				}
				if n%17 == 0 {
					var got model.Item
					if err := doJSON(client, token, http.MethodDelete, srv.URL+"/v1/library/"+id, nil, 200, &got); err != nil {
						return err
					}
					if got.DeletedAt == nil {
						return fmt.Errorf("delete %s no tombstone", id)
					}
					continue
				}
				patch := map[string]any{"notes": fmt.Sprintf("patched-%d", n)}
				var got model.Item
				if err := doJSON(client, token, http.MethodPatch, srv.URL+"/v1/library/"+id, patch, 200, &got); err != nil {
					return err
				}
			}
			return nil
		})
	})

	t.Run("csv_roundtrip", func(t *testing.T) {
		req, _ := http.NewRequest(http.MethodGet, srv.URL+"/v1/library.csv", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		res, err := client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		raw, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("csv %d %s", res.StatusCode, raw)
		}
		parsed, err := export.ParseLibraryCSV(raw)
		if err != nil {
			t.Fatal(err)
		}
		if len(parsed) == 0 {
			t.Fatal("empty csv")
		}
		for _, it := range parsed {
			if strings.Contains(it.Title, "Stress") && strings.Contains(it.Title, ",") && !strings.Contains(it.Title, `"`) {
				// quoted titles survive parse without extra quotes
			}
			if it.Title == "" || it.Platform == "" {
				t.Fatalf("csv row %+v", it)
			}
		}
		// Wipe-replace with a smaller CSV, then restore via re-import of the export.
		small := "title,platform,barcode\nOnly One,\"PC (Microsoft Windows)\",012345678905\n"
		req, _ = http.NewRequest(http.MethodPost, srv.URL+"/v1/library/import", strings.NewReader(small))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "text/csv")
		res, err = client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("import small %d %s", res.StatusCode, body)
		}
		var list struct {
			Items []model.Item `json:"items"`
		}
		if err := doJSON(client, token, http.MethodGet, srv.URL+"/v1/library", nil, 200, &list); err != nil {
			t.Fatal(err)
		}
		if len(list.Items) != 1 || list.Items[0].Title != "Only One" {
			t.Fatalf("after small import %+v", list.Items)
		}
		req, _ = http.NewRequest(http.MethodPost, srv.URL+"/v1/library/import", bytes.NewReader(raw))
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "text/csv")
		res, err = client.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		body, _ = io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode != 200 {
			t.Fatalf("import restore %d %s", res.StatusCode, body)
		}
		if err := doJSON(client, token, http.MethodGet, srv.URL+"/v1/library", nil, 200, &list); err != nil {
			t.Fatal(err)
		}
		if len(list.Items) != len(parsed) {
			t.Fatalf("restored %d want %d", len(list.Items), len(parsed))
		}
	})

	t.Run("barcode_storm", func(t *testing.T) {
		parallel(t, nWorkers, func(i int) error {
			for n := i; n < nBarcode; n += nWorkers {
				code := stressBarcode(10_000 + n)
				var got struct {
					Barcode      string `json:"barcode"`
					ProductTitle string `json:"product_title"`
					Query        string `json:"query"`
					Platform     string `json:"platform"`
					Games        []any  `json:"games"`
					Owned        []any  `json:"owned"`
				}
				u := srv.URL + "/v1/search/barcode?q=" + code
				if err := doJSON(client, token, http.MethodGet, u, nil, 200, &got); err != nil {
					return err
				}
				if got.Barcode == "" {
					return fmt.Errorf("no barcode")
				}
				if strings.Contains(got.Query, "[Physical]") || strings.Contains(got.Query, code) {
					return fmt.Errorf("query still dirty %q", got.Query)
				}
				if got.Platform != "PlayStation 4" {
					return fmt.Errorf("platform %q", got.Platform)
				}
				if n%5 == 0 {
					body := map[string]any{
						"title":    got.Query,
						"platform": got.Platform,
						"barcode":  code,
					}
					var item model.Item
					if err := doJSON(client, token, http.MethodPost, srv.URL+"/v1/library", body, http.StatusCreated, &item); err != nil {
						return err
					}
					if err := doJSON(client, token, http.MethodGet, u, nil, 200, &got); err != nil {
						return err
					}
					if len(got.Owned) < 1 {
						return fmt.Errorf("expected owned copy for %s", code)
					}
				}
			}
			return nil
		})
	})

	t.Run("multi_device_sync", func(t *testing.T) {
		now := time.Now().UTC().Truncate(time.Second)
		type replica struct {
			mu     sync.Mutex
			cursor int64
			items  map[string]model.Item
			dirty  []model.Item
		}
		devices := make([]*replica, nDevices)
		for i := range devices {
			devices[i] = &replica{items: map[string]model.Item{}}
		}

		// Each device inserts a disjoint batch, plus they all fight over a shared id.
		shared := "ffffffff-aaaa-4aaa-8aaa-000000000001"
		parallel(t, nDevices, func(d int) error {
			dev := devices[d]
			for k := 0; k < 20*scale; k++ {
				id := fmt.Sprintf("%08x-bbbb-4bbb-8bbb-%012x", d, k)
				it := model.Item{
					ID:           id,
					Title:        fmt.Sprintf("Device %d game %d", d, k),
					Platform:     "PC (Microsoft Windows)",
					Completeness: "cib",
					CreatedAt:    now.Add(time.Duration(d) * time.Second),
					UpdatedAt:    now.Add(time.Duration(d)*time.Second + time.Duration(k)*time.Millisecond),
				}
				dev.mu.Lock()
				dev.dirty = append(dev.dirty, it)
				dev.mu.Unlock()
			}
			winner := model.Item{
				ID:           shared,
				Title:        fmt.Sprintf("shared-from-%d", d),
				Platform:     "Nintendo Switch",
				Completeness: "new",
				CreatedAt:    now,
				UpdatedAt:    now.Add(time.Duration(d) * time.Second),
			}
			dev.mu.Lock()
			dev.dirty = append(dev.dirty, winner)
			dev.mu.Unlock()
			return nil
		})

		syncOnce := func(d int) error {
			dev := devices[d]
			dev.mu.Lock()
			payload := map[string]any{"cursor": dev.cursor, "changes": dev.dirty}
			dev.dirty = nil
			dev.mu.Unlock()
			var res store.SyncResult
			if err := doJSON(client, token, http.MethodPost, srv.URL+"/v1/sync", payload, 200, &res); err != nil {
				return err
			}
			dev.mu.Lock()
			if dev.items == nil {
				dev.items = map[string]model.Item{}
			}
			for _, ch := range res.Changes {
				dev.items[ch.ID] = ch
			}
			dev.cursor = res.Cursor
			dev.mu.Unlock()
			return nil
		}

		for round := 0; round < syncRounds; round++ {
			parallel(t, nDevices, syncOnce)
			// Mid-flight edits and a tombstone from device 0.
			if round == syncRounds/2 {
				dev := devices[0]
				dev.mu.Lock()
				var victim string
				for id, it := range dev.items {
					if it.DeletedAt == nil && id != shared {
						victim = id
						break
					}
				}
				if victim != "" {
					it := dev.items[victim]
					del := now.Add(2 * time.Minute)
					it.DeletedAt = &del
					it.UpdatedAt = del
					dev.dirty = append(dev.dirty, it)
				}
				dev.mu.Unlock()
			}
		}
		// Drain until idle.
		for pass := 0; pass < 4; pass++ {
			parallel(t, nDevices, syncOnce)
		}

		var server struct {
			Items []model.Item `json:"items"`
		}
		if err := doJSON(client, token, http.MethodGet, srv.URL+"/v1/library", nil, 200, &server); err != nil {
			t.Fatal(err)
		}
		serverLive := map[string]model.Item{}
		for _, it := range server.Items {
			serverLive[it.ID] = it
		}
		for d, dev := range devices {
			dev.mu.Lock()
			live := 0
			for id, it := range dev.items {
				if it.DeletedAt != nil {
					if _, ok := serverLive[id]; ok {
						dev.mu.Unlock()
						t.Fatalf("device %d still has deleted %s on server list", d, id)
					}
					continue
				}
				live++
				got, ok := serverLive[id]
				if !ok {
					dev.mu.Unlock()
					t.Fatalf("device %d has %s missing on server", d, id)
				}
				if got.Title != it.Title {
					dev.mu.Unlock()
					t.Fatalf("device %d title drift %s %q vs %q", d, id, it.Title, got.Title)
				}
			}
			dev.mu.Unlock()
			if live == 0 {
				t.Fatalf("device %d empty", d)
			}
		}
		if _, ok := serverLive[shared]; !ok {
			t.Fatal("shared conflict row missing")
		}
		if !strings.HasPrefix(serverLive[shared].Title, "shared-from-") {
			t.Fatalf("shared title %q", serverLive[shared].Title)
		}
	})

	t.Run("future_clock_clamp", func(t *testing.T) {
		future := time.Now().UTC().Add(2 * time.Hour)
		it := model.Item{
			ID:           "cccccccc-cccc-4ccc-8ccc-cccccccccccc",
			Title:        "Clock Cheat",
			Platform:     "PC (Microsoft Windows)",
			Completeness: "loose",
			CreatedAt:    future,
			UpdatedAt:    future,
		}
		var res store.SyncResult
		if err := doJSON(client, token, http.MethodPost, srv.URL+"/v1/sync", map[string]any{
			"cursor":  0,
			"changes": []model.Item{it},
		}, 200, &res); err != nil {
			t.Fatal(err)
		}
		var found *model.Item
		for i := range res.Changes {
			if res.Changes[i].ID == it.ID {
				found = &res.Changes[i]
				break
			}
		}
		if found == nil {
			t.Fatal("clamped row missing")
		}
		if found.UpdatedAt.After(time.Now().UTC().Add(6 * time.Minute)) {
			t.Fatalf("timestamp not clamped %s", found.UpdatedAt)
		}
	})

	t.Logf("extreme stress finished in %s", time.Since(start).Truncate(time.Millisecond))
}

func stressTitle(n int) string {
	switch n % 7 {
	case 0:
		return fmt.Sprintf("Stress, Game %d", n)
	case 1:
		return fmt.Sprintf(`Stress "Quoted" %d`, n)
	case 2:
		return fmt.Sprintf("ゼルダの伝説 %d", n)
	case 3:
		return fmt.Sprintf("Pokémon Café %d", n)
	default:
		return fmt.Sprintf("Stress Game %d", n)
	}
}

func stressPlatform(n int) string {
	ps := []string{"Nintendo Switch", "PlayStation 4", "PlayStation 5", "PC (Microsoft Windows)", "Xbox Series X"}
	return ps[n%len(ps)]
}

func stressBarcode(n int) string {
	// 12-digit UPC-like, unique per n.
	return fmt.Sprintf("%012d", 400000000000+n)
}

func parallel(t *testing.T, n int, fn func(i int) error) {
	t.Helper()
	errCh := make(chan error, n)
	var wg sync.WaitGroup
	wg.Add(n)
	for i := 0; i < n; i++ {
		go func(i int) {
			defer wg.Done()
			if err := fn(i); err != nil {
				errCh <- err
			}
		}(i)
	}
	wg.Wait()
	close(errCh)
	var errs []string
	for err := range errCh {
		errs = append(errs, err.Error())
		if len(errs) >= 8 {
			break
		}
	}
	if len(errs) > 0 {
		t.Fatal(strings.Join(errs, " | "))
	}
}

func tryLogin(client *http.Client, base, password string) (string, error) {
	raw, _ := json.Marshal(map[string]string{"password": password})
	res, err := client.Post(base+"/v1/auth/login", "application/json", bytes.NewReader(raw))
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return "", fmt.Errorf("login %d %s", res.StatusCode, body)
	}
	var out struct{ Token string }
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.Token, nil
}

func doJSON(client *http.Client, token, method, url string, in any, want int, out any) error {
	var rdr io.Reader
	if in != nil {
		raw, err := json.Marshal(in)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, url, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if in != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != want {
		return fmt.Errorf("%s %s -> %d want %d %s", method, url, res.StatusCode, want, truncate(body, 300))
	}
	if out == nil || res.StatusCode == 204 || len(body) == 0 {
		return nil
	}
	return json.Unmarshal(body, out)
}

func truncate(b []byte, n int) string {
	if len(b) <= n {
		return string(b)
	}
	return string(b[:n]) + "…"
}
