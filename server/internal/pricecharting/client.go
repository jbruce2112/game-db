package pricecharting

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const apiBase = "https://www.pricecharting.com"

type Client struct {
	Token   string
	HTTP    *http.Client
	BaseURL string
	mu      sync.Mutex
	lastReq time.Time
}

func New(token string) *Client {
	return &Client{
		Token:   strings.TrimSpace(token),
		HTTP:    &http.Client{Timeout: 15 * time.Second},
		BaseURL: apiBase,
	}
}

type Product struct {
	ID      string
	Name    string
	Console string
	Loose   *int
	CIB     *int
	New     *int
}

func (p Product) URL() string {
	if p.ID == "" {
		return ""
	}
	if p.Console != "" && p.Name != "" {
		return apiBase + "/game/" + slug(p.Console) + "/" + slug(p.Name)
	}
	return apiBase + "/search-products?type=videogames&q=" + url.QueryEscape(p.ID)
}

func (c *Client) ProductByUPC(ctx context.Context, upc string) (Product, error) {
	return c.product(ctx, url.Values{"upc": {upc}})
}

func (c *Client) Search(ctx context.Context, q string) ([]Product, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, fmt.Errorf("query required")
	}
	raw, err := c.get(ctx, "/api/products", url.Values{"q": {q}})
	if err != nil {
		return nil, err
	}
	var parsed struct {
		Status   string           `json:"status"`
		Error    string           `json:"error-message"`
		Products []productPayload `json:"products"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, err
	}
	if parsed.Status == "error" {
		return nil, fmt.Errorf("pricecharting: %s", parsed.Error)
	}
	out := make([]Product, 0, len(parsed.Products))
	for _, row := range parsed.Products {
		if p := row.toProduct(); p.ID != "" {
			out = append(out, p)
		}
	}
	return out, nil
}

func (c *Client) product(ctx context.Context, extra url.Values) (Product, error) {
	raw, err := c.get(ctx, "/api/product", extra)
	if err != nil {
		return Product{}, err
	}
	var parsed productPayload
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return Product{}, err
	}
	if parsed.Status == "error" {
		return Product{}, fmt.Errorf("pricecharting: %s", parsed.Error)
	}
	return parsed.toProduct(), nil
}

type productPayload struct {
	Status  string          `json:"status"`
	Error   string          `json:"error-message"`
	ID      json.RawMessage `json:"id"`
	Name    string          `json:"product-name"`
	Console string          `json:"console-name"`
	Loose   *int            `json:"loose-price"`
	CIB     *int            `json:"cib-price"`
	New     *int            `json:"new-price"`
}

func (p productPayload) toProduct() Product {
	id := parseID(p.ID)
	if id == "" || p.Name == "" {
		return Product{}
	}
	return Product{
		ID:      id,
		Name:    p.Name,
		Console: p.Console,
		Loose:   p.Loose,
		CIB:     p.CIB,
		New:     p.New,
	}
}

func parseID(raw json.RawMessage) string {
	s := strings.TrimSpace(string(raw))
	if s == "" || s == "null" {
		return ""
	}
	if len(s) >= 2 && s[0] == '"' {
		var str string
		if err := json.Unmarshal(raw, &str); err == nil {
			return strings.TrimSpace(str)
		}
	}
	var n json.Number
	if err := json.Unmarshal(raw, &n); err == nil {
		return n.String()
	}
	return strings.Trim(s, `"`)
}

func (c *Client) get(ctx context.Context, path string, extra url.Values) ([]byte, error) {
	if c.Token == "" {
		return nil, fmt.Errorf("pricecharting token not set")
	}
	c.throttle()
	vals := url.Values{}
	vals.Set("t", c.Token)
	for k, vs := range extra {
		for _, v := range vs {
			vals.Add(k, v)
		}
	}
	base := c.BaseURL
	if base == "" {
		base = apiBase
	}
	u := strings.TrimRight(base, "/") + path + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "game-db/0.1 (self-hosted library)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("pricecharting: HTTP %d %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	return body, nil
}

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	wait := time.Second - time.Since(c.lastReq)
	if wait > 0 {
		time.Sleep(wait)
	}
	c.lastReq = time.Now()
}

func slug(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	var b strings.Builder
	lastDash := false
	for _, r := range s {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
			continue
		}
		if !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	return strings.Trim(b.String(), "-")
}
