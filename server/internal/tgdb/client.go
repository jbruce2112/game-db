package tgdb

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"game-db/internal/igdb"
)

const apiBase = "https://api.thegamesdb.net/v1"

var ErrNoFront = errors.New("no box front")

type Client struct {
	APIKey  string
	HTTP    *http.Client
	BaseURL string

	mu      sync.Mutex
	lastReq time.Time
}

func New(apiKey string) *Client {
	return &Client{
		APIKey:  strings.TrimSpace(apiKey),
		HTTP:    &http.Client{Timeout: 25 * time.Second},
		BaseURL: apiBase,
	}
}

type Platform struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Alias string `json:"alias"`
}

type Game struct {
	ID       int64
	Title    string
	Platform int64
	FrontURL string
	SourceID string
}

func (c *Client) Platforms(ctx context.Context) ([]Platform, error) {
	var raw struct {
		Data struct {
			Platforms map[string]Platform `json:"platforms"`
		} `json:"data"`
	}
	if err := c.get(ctx, "/Platforms", nil, &raw); err != nil {
		return nil, err
	}
	out := make([]Platform, 0, len(raw.Data.Platforms))
	for _, p := range raw.Data.Platforms {
		out = append(out, p)
	}
	return out, nil
}

// SearchFront finds a platform-specific box front for title.
func (c *Client) SearchFront(ctx context.Context, title string, platformID int64) (Game, error) {
	title = strings.TrimSpace(title)
	if title == "" || platformID <= 0 {
		return Game{}, ErrNoFront
	}
	queries := igdb.SearchTitles(title)
	if len(queries) > 3 {
		queries = queries[:3]
	}
	for _, q := range queries {
		games, err := c.GamesByName(ctx, q, platformID)
		if err != nil {
			return Game{}, err
		}
		if g := PickGame(title, games); g != nil && g.FrontURL != "" {
			return *g, nil
		}
	}
	return Game{}, ErrNoFront
}

func (c *Client) GamesByName(ctx context.Context, name string, platformID int64) ([]Game, error) {
	q := url.Values{}
	q.Set("name", name)
	q.Set("fields", "platform,alternates")
	q.Set("include", "boxart")
	if platformID > 0 {
		q.Set("filter[platform]", strconv.FormatInt(platformID, 10))
	}
	var raw gamesResponse
	if err := c.get(ctx, "/Games/ByGameName", q, &raw); err != nil {
		return nil, err
	}
	base := raw.Include.Boxart.BaseURL.Large
	if base == "" {
		base = raw.Include.Boxart.BaseURL.Original
	}
	out := make([]Game, 0, len(raw.Data.Games))
	for _, g := range raw.Data.Games {
		game := Game{ID: g.ID, Title: g.GameTitle, Platform: g.Platform}
		imgs := raw.Include.Boxart.Data[strconv.FormatInt(g.ID, 10)]
		if front, src := pickFront(base, imgs); front != "" {
			game.FrontURL = front
			game.SourceID = src
		}
		out = append(out, game)
	}
	return out, nil
}

func (c *Client) Download(ctx context.Context, imageURL string) (contentType string, data []byte, err error) {
	if imageURL == "" {
		return "", nil, fmt.Errorf("empty box art url")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, imageURL, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("box art download: %s", resp.Status)
	}
	data, err = io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return "", nil, err
	}
	ct := resp.Header.Get("Content-Type")
	if ct == "" {
		ct = "image/jpeg"
	}
	return ct, data, nil
}

type gamesResponse struct {
	Data struct {
		Games []rawGame `json:"games"`
	} `json:"data"`
	Include struct {
		Boxart struct {
			BaseURL struct {
				Original string `json:"original"`
				Large    string `json:"large"`
				Medium   string `json:"medium"`
			} `json:"base_url"`
			Data map[string][]boxImage `json:"data"`
		} `json:"boxart"`
	} `json:"include"`
}

type rawGame struct {
	ID         int64    `json:"id"`
	GameTitle  string   `json:"game_title"`
	Platform   int64    `json:"platform"`
	Alternates []string `json:"alternates"`
}

type boxImage struct {
	ID         int64  `json:"id"`
	Type       string `json:"type"`
	Side       string `json:"side"`
	Filename   string `json:"filename"`
	Resolution string `json:"resolution"`
}

func pickFront(base string, imgs []boxImage) (imageURL, sourceID string) {
	var best boxImage
	bestArea := -1
	for _, img := range imgs {
		if !isFront(img) || img.Filename == "" {
			continue
		}
		area := imageArea(img.Resolution)
		if area > bestArea {
			best = img
			bestArea = area
		}
	}
	if best.Filename == "" {
		return "", ""
	}
	if base == "" {
		return best.Filename, "tgdb:" + best.Filename
	}
	return strings.TrimRight(base, "/") + "/" + strings.TrimLeft(best.Filename, "/"), "tgdb:" + best.Filename
}

func isFront(img boxImage) bool {
	if !strings.EqualFold(img.Type, "boxart") {
		return false
	}
	side := strings.ToLower(strings.TrimSpace(img.Side))
	return side == "front" || side == ""
}

func imageArea(res string) int {
	parts := strings.Split(strings.ToLower(strings.TrimSpace(res)), "x")
	if len(parts) != 2 {
		return 0
	}
	w, errW := strconv.Atoi(parts[0])
	h, errH := strconv.Atoi(parts[1])
	if errW != nil || errH != nil {
		return 0
	}
	return w * h
}

func (c *Client) get(ctx context.Context, path string, extra url.Values, dest any) error {
	if strings.TrimSpace(c.APIKey) == "" {
		return fmt.Errorf("thegamesdb api key missing")
	}
	base := c.BaseURL
	if base == "" {
		base = apiBase
	}
	q := url.Values{}
	q.Set("apikey", c.APIKey)
	for k, vs := range extra {
		for _, v := range vs {
			q.Add(k, v)
		}
	}
	u := strings.TrimRight(base, "/") + path + "?" + q.Encode()
	c.throttle()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("thegamesdb %s: %s %s", path, resp.Status, strings.TrimSpace(string(b)))
	}
	return json.Unmarshal(b, dest)
}

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	wait := 1100*time.Millisecond - time.Since(c.lastReq)
	if wait > 0 {
		time.Sleep(wait)
	}
	c.lastReq = time.Now()
}
