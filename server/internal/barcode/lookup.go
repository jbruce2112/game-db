package barcode

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

var (
	UPCItemDBURL    = "https://api.upcitemdb.com/prod/trial/lookup"
	OpenProductsURL = "https://world.openproductsfacts.org/api/v2/product/"
	GoUPCURL        = "https://go-upc.com/search"
)

type Product struct {
	Title  string
	Brand  string
	Source string
}

func HTTPClient() *http.Client {
	return &http.Client{Timeout: 8 * time.Second}
}

// Lookup tries upcitemdb, Open Products Facts, then Go-UPC for any code variant.
// Grocery-oriented catalogs often miss Japanese JAN / Asian game SKUs; Go-UPC covers those better.
func Lookup(ctx context.Context, client *http.Client, codes []string) (Product, error) {
	if client == nil {
		client = HTTPClient()
	}
	var last error
	for _, code := range codes {
		p, err := lookupUPCitemdb(ctx, client, code)
		if err == nil && p.Title != "" {
			return p, nil
		}
		if err != nil {
			last = err
		}
		p, err = lookupOpenProducts(ctx, client, code)
		if err == nil && p.Title != "" {
			return p, nil
		}
		if err != nil {
			last = err
		}
		p, err = lookupGoUPC(ctx, client, code)
		if err == nil && p.Title != "" {
			return p, nil
		}
		if err != nil {
			last = err
		}
	}
	if last != nil {
		return Product{}, last
	}
	return Product{}, fmt.Errorf("no product found")
}

func lookupUPCitemdb(ctx context.Context, client *http.Client, code string) (Product, error) {
	u := UPCItemDBURL + "?upc=" + url.QueryEscape(code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Product{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "game-db/0.1 (self-hosted library)")
	res, err := client.Do(req)
	if err != nil {
		return Product{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusNotFound {
		return Product{}, nil
	}
	if res.StatusCode == http.StatusTooManyRequests {
		return Product{}, fmt.Errorf("barcode catalog rate-limited")
	}
	if res.StatusCode >= 400 {
		return Product{}, fmt.Errorf("upcitemdb: HTTP %d", res.StatusCode)
	}
	var parsed struct {
		Code  string `json:"code"`
		Items []struct {
			Title string `json:"title"`
			Brand string `json:"brand"`
		} `json:"items"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Product{}, err
	}
	if len(parsed.Items) == 0 || strings.TrimSpace(parsed.Items[0].Title) == "" {
		return Product{}, nil
	}
	return Product{
		Title:  strings.TrimSpace(parsed.Items[0].Title),
		Brand:  strings.TrimSpace(parsed.Items[0].Brand),
		Source: "upcitemdb",
	}, nil
}

func lookupOpenProducts(ctx context.Context, client *http.Client, code string) (Product, error) {
	u := strings.TrimRight(OpenProductsURL, "/") + "/" + url.PathEscape(code) + ".json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Product{}, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "game-db/0.1 (self-hosted library)")
	res, err := client.Do(req)
	if err != nil {
		return Product{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusNotFound {
		return Product{}, nil
	}
	if res.StatusCode >= 400 {
		return Product{}, fmt.Errorf("open products facts: HTTP %d", res.StatusCode)
	}
	var parsed struct {
		Status  int `json:"status"`
		Product struct {
			ProductName   string `json:"product_name"`
			ProductNameEN string `json:"product_name_en"`
			Brands        string `json:"brands"`
		} `json:"product"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return Product{}, err
	}
	title := strings.TrimSpace(parsed.Product.ProductNameEN)
	if title == "" {
		title = strings.TrimSpace(parsed.Product.ProductName)
	}
	if parsed.Status != 1 || title == "" {
		return Product{}, nil
	}
	return Product{Title: title, Brand: strings.TrimSpace(parsed.Product.Brands), Source: "openproductsfacts"}, nil
}

func lookupGoUPC(ctx context.Context, client *http.Client, code string) (Product, error) {
	u := GoUPCURL + "?q=" + url.QueryEscape(code)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return Product{}, err
	}
	req.Header.Set("Accept", "text/html")
	req.Header.Set("User-Agent", "game-db/0.1 (self-hosted library)")
	res, err := client.Do(req)
	if err != nil {
		return Product{}, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusBadRequest {
		return Product{}, nil
	}
	if res.StatusCode >= 400 {
		return Product{}, fmt.Errorf("go-upc: HTTP %d", res.StatusCode)
	}
	title := parseGoUPC(string(body))
	if title == "" {
		return Product{}, nil
	}
	return Product{Title: title, Source: "goupc"}, nil
}

func parseGoUPC(page string) string {
	if i := strings.Index(page, `class="product-name"`); i >= 0 {
		rest := page[i:]
		if gt := strings.Index(rest, ">"); gt >= 0 {
			rest = rest[gt+1:]
			if end := strings.Index(rest, "<"); end >= 0 {
				if title := cleanGoUPCTitle(html.UnescapeString(rest[:end])); title != "" {
					return title
				}
			}
		}
	}
	if i := strings.Index(strings.ToLower(page), "<title>"); i >= 0 {
		rest := page[i+7:]
		if end := strings.Index(strings.ToLower(rest), "</title>"); end >= 0 {
			return cleanGoUPCTitle(html.UnescapeString(rest[:end]))
		}
	}
	return ""
}

func cleanGoUPCTitle(s string) string {
	s = strings.TrimSpace(s)
	for _, sep := range []string{" — EAN", " — UPC", " — JAN", " — Go-UPC", " - EAN", " - UPC"} {
		if i := strings.Index(s, sep); i > 0 {
			s = strings.TrimSpace(s[:i])
		}
	}
	switch strings.ToLower(s) {
	case "", "go-upc", "invalid value", "not found":
		return ""
	}
	return s
}
