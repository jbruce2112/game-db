package ebay

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"game-db/internal/barcode"
)

const (
	apiBase    = "https://api.ebay.com"
	tokenURL   = "https://api.ebay.com/identity/v1/oauth2/token"
	browsePath = "/buy/browse/v1/item_summary/search"
	videoGames = "139973"
	scope      = "https://api.ebay.com/oauth/api_scope"
	defaultMkt = "EBAY_US"
)

type Client struct {
	ClientID     string
	ClientSecret string
	Marketplace  string
	HTTP         *http.Client
	BaseURL      string
	TokenURL     string

	mu       sync.Mutex
	token    string
	tokenExp time.Time
	lastReq  time.Time
}

func New(clientID, secret, marketplace string) *Client {
	if marketplace == "" {
		marketplace = defaultMkt
	}
	return &Client{
		ClientID:     strings.TrimSpace(clientID),
		ClientSecret: strings.TrimSpace(secret),
		Marketplace:  marketplace,
		HTTP:         &http.Client{Timeout: 20 * time.Second},
		BaseURL:      apiBase,
		TokenURL:     tokenURL,
	}
}

type Quote struct {
	Name     string
	Console  string
	URL      string
	Loose    *int
	CIB      *int
	New      *int
	Listings int
}

func (c *Client) Quote(ctx context.Context, title, platform, code string) (Quote, error) {
	var listings []listing
	var err error
	searchURL := SearchURL(title, platform, code)
	if code != "" {
		for _, gtin := range barcode.Variants(code) {
			listings, err = c.search(ctx, "", gtin)
			if err != nil {
				return Quote{}, err
			}
			if len(listings) > 0 {
				break
			}
		}
	}
	if len(listings) == 0 {
		q := strings.TrimSpace(title + " " + platform)
		listings, err = c.search(ctx, q, "")
		if err != nil {
			return Quote{}, err
		}
	}
	matched := filterListings(listings, title, platform)
	if len(matched) == 0 {
		return Quote{}, nil
	}
	loose, cib, neu := bucketPrices(matched)
	name := matched[0].Title
	if score := barcode.NameScore(name, title, nil, ""); score < 50 {
		name = title
	}
	return Quote{
		Name:     name,
		Console:  platform,
		URL:      searchURL,
		Loose:    medianPtr(loose),
		CIB:      medianPtr(cib),
		New:      medianPtr(neu),
		Listings: len(matched),
	}, nil
}

func SearchURL(title, platform, code string) string {
	q := strings.TrimSpace(code)
	if q == "" {
		q = strings.TrimSpace(title + " " + platform)
	}
	return "https://www.ebay.com/sch/" + videoGames + "/i.html?_nkw=" + url.QueryEscape(q)
}

type listing struct {
	Title       string
	Cents       int
	Currency    string
	Condition   string
	ConditionID string
}

func (c *Client) search(ctx context.Context, q, gtin string) ([]listing, error) {
	token, err := c.accessToken(ctx)
	if err != nil {
		return nil, err
	}
	c.throttle()
	vals := url.Values{}
	vals.Set("limit", "50")
	vals.Set("category_ids", videoGames)
	if gtin != "" {
		vals.Set("gtin", gtin)
	} else {
		vals.Set("q", q)
	}
	base := c.BaseURL
	if base == "" {
		base = apiBase
	}
	u := strings.TrimRight(base, "/") + browsePath + "?" + vals.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	mkt := c.Marketplace
	if mkt == "" {
		mkt = defaultMkt
	}
	req.Header.Set("X-EBAY-C-MARKETPLACE-ID", mkt)
	req.Header.Set("User-Agent", "game-db/0.1 (self-hosted library)")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if res.StatusCode >= 400 {
		return nil, fmt.Errorf("ebay browse: HTTP %d %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var parsed struct {
		ItemSummaries []struct {
			Title       string `json:"title"`
			Condition   string `json:"condition"`
			ConditionID string `json:"conditionId"`
			Price       struct {
				Value    string `json:"value"`
				Currency string `json:"currency"`
			} `json:"price"`
		} `json:"itemSummaries"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, err
	}
	out := make([]listing, 0, len(parsed.ItemSummaries))
	for _, it := range parsed.ItemSummaries {
		cents, ok := parseUSDCents(it.Price.Value, it.Price.Currency)
		if !ok || it.Title == "" {
			continue
		}
		out = append(out, listing{
			Title:       it.Title,
			Cents:       cents,
			Currency:    it.Price.Currency,
			Condition:   it.Condition,
			ConditionID: it.ConditionID,
		})
	}
	return out, nil
}

func filterListings(in []listing, title, platform string) []listing {
	var out []listing
	for _, l := range in {
		low := strings.ToLower(l.Title)
		if strings.Contains(low, "lot of") || strings.Contains(low, "bundle") || strings.Contains(low, "lot:") {
			continue
		}
		score := barcode.NameScore(l.Title, title, nil, "")
		if platform != "" && strings.Contains(low, strings.ToLower(platform)) {
			score += 15
		}
		if score < 40 {
			continue
		}
		out = append(out, l)
	}
	return out
}

func bucketPrices(in []listing) (loose, cib, neu []int) {
	for _, l := range in {
		if isNewCondition(l) {
			neu = append(neu, l.Cents)
			continue
		}
		low := strings.ToLower(l.Title)
		if looksCIB(low) {
			cib = append(cib, l.Cents)
			continue
		}
		if looksLoose(low) {
			loose = append(loose, l.Cents)
			continue
		}
		loose = append(loose, l.Cents)
	}
	return loose, cib, neu
}

func isNewCondition(l listing) bool {
	id := l.ConditionID
	return id == "1000" || id == "1500" || strings.EqualFold(l.Condition, "New")
}

func looksCIB(title string) bool {
	for _, p := range []string{"cib", "complete in box", "complete-in-box", "box and manual", "case and manual", "with manual", "w/ manual", "w/manual"} {
		if strings.Contains(title, p) {
			return true
		}
	}
	return false
}

func looksLoose(title string) bool {
	for _, p := range []string{"disc only", "disk only", "cart only", "cartridge only", "game only", "loose"} {
		if strings.Contains(title, p) {
			return true
		}
	}
	return false
}

func parseUSDCents(value, currency string) (int, bool) {
	if !strings.EqualFold(currency, "USD") {
		return 0, false
	}
	value = strings.TrimSpace(value)
	f, err := strconv.ParseFloat(value, 64)
	if err != nil || f < 0 {
		return 0, false
	}
	return int(f*100 + 0.5), true
}

func medianPtr(vals []int) *int {
	if len(vals) == 0 {
		return nil
	}
	cp := append([]int(nil), vals...)
	for i := 0; i < len(cp); i++ {
		for j := i + 1; j < len(cp); j++ {
			if cp[j] < cp[i] {
				cp[i], cp[j] = cp[j], cp[i]
			}
		}
	}
	v := cp[len(cp)/2]
	return &v
}

func (c *Client) accessToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	if c.token != "" && time.Now().Before(c.tokenExp.Add(-time.Minute)) {
		t := c.token
		c.mu.Unlock()
		return t, nil
	}
	c.mu.Unlock()

	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("scope", scope)
	tu := c.TokenURL
	if tu == "" {
		tu = tokenURL
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tu, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.ClientID+":"+c.ClientSecret)))
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 400 {
		return "", fmt.Errorf("ebay token: HTTP %d %s", res.StatusCode, strings.TrimSpace(string(body)))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", err
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("ebay token: empty")
	}
	exp := tok.ExpiresIn
	if exp <= 0 {
		exp = 7200
	}
	c.mu.Lock()
	c.token = tok.AccessToken
	c.tokenExp = time.Now().Add(time.Duration(exp) * time.Second)
	c.mu.Unlock()
	return tok.AccessToken, nil
}

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	wait := 200*time.Millisecond - time.Since(c.lastReq)
	if wait > 0 {
		time.Sleep(wait)
	}
	c.lastReq = time.Now()
}
