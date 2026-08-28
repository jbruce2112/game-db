package igdb

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

const (
	tokenURL  = "https://id.twitch.tv/oauth2/token"
	apiBase   = "https://api.igdb.com/v4"
	coverBase = "https://images.igdb.com/igdb/image/upload/t_cover_big/"
)

type Client struct {
	HTTP       *http.Client
	ClientID   string
	Secret     string
	mu         sync.Mutex
	token      string
	tokenExp   time.Time
	lastReq    time.Time
}

func New(clientID, secret string) *Client {
	return &Client{
		HTTP:     &http.Client{Timeout: 20 * time.Second},
		ClientID: clientID,
		Secret:   secret,
	}
}

type Platform struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Slug         *string `json:"slug"`
	Abbreviation *string `json:"abbreviation"`
}

type Game struct {
	ID               int64      `json:"igdb_id"`
	Name             string     `json:"name"`
	Summary          string     `json:"summary"`
	CoverURL         *string    `json:"cover_url"`
	CoverImageID     string     `json:"-"`
	FirstReleaseDate *string    `json:"first_release_date"`
	Platforms        []Platform `json:"platforms"`
}

func CoverURL(imageID string) string {
	if imageID == "" {
		return ""
	}
	return coverBase + imageID + ".jpg"
}

func (c *Client) SearchGames(ctx context.Context, q string, platformID int64) ([]Game, error) {
	q = strings.TrimSpace(q)
	if q == "" {
		return nil, fmt.Errorf("query required")
	}
	body := fmt.Sprintf(
		`search "%s"; fields name,summary,cover.image_id,platforms.id,platforms.name,first_release_date; where platforms != null; limit 25;`,
		escape(q),
	)
	if platformID > 0 {
		body = fmt.Sprintf(
			`search "%s"; fields name,summary,cover.image_id,platforms.id,platforms.name,first_release_date; where platforms = (%d); limit 25;`,
			escape(q), platformID,
		)
	}
	var raw []igdbGame
	if err := c.post(ctx, "/games", body, &raw); err != nil {
		return nil, err
	}
	out := make([]Game, 0, len(raw))
	for _, g := range raw {
		out = append(out, g.toGame())
	}
	return out, nil
}

func (c *Client) Game(ctx context.Context, id int64) (Game, error) {
	var raw []igdbGame
	body := fmt.Sprintf(
		`fields name,summary,cover.image_id,platforms.id,platforms.name,first_release_date; where id = %d;`,
		id,
	)
	if err := c.post(ctx, "/games", body, &raw); err != nil {
		return Game{}, err
	}
	if len(raw) == 0 {
		return Game{}, fmt.Errorf("igdb game %d not found", id)
	}
	return raw[0].toGame(), nil
}

func (c *Client) Platforms(ctx context.Context) ([]Platform, error) {
	typeIDs, err := c.consoleLikePlatformTypeIDs(ctx)
	if err != nil {
		return nil, err
	}
	where := "id != null"
	if len(typeIDs) > 0 {
		parts := make([]string, 0, len(typeIDs))
		for _, id := range typeIDs {
			parts = append(parts, fmt.Sprintf("%d", id))
		}
		where = fmt.Sprintf("platform_type = (%s)", strings.Join(parts, ","))
	}
	var raw []struct {
		ID           int64   `json:"id"`
		Name         string  `json:"name"`
		Slug         *string `json:"slug"`
		Abbreviation *string `json:"abbreviation"`
	}
	body := fmt.Sprintf(`fields id,name,slug,abbreviation; where %s; sort name asc; limit 500;`, where)
	if err := c.post(ctx, "/platforms", body, &raw); err != nil {
		return nil, err
	}
	out := make([]Platform, 0, len(raw))
	for _, p := range raw {
		out = append(out, Platform(p))
	}
	return out, nil
}

func (c *Client) consoleLikePlatformTypeIDs(ctx context.Context) ([]int64, error) {
	var types []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	}
	if err := c.post(ctx, "/platform_types", `fields id,name; limit 50;`, &types); err != nil {
		return nil, err
	}
	want := map[string]struct{}{
		"console":           {},
		"portable console":  {},
		"portable_console":  {},
		"computer":          {},
		"operating system":  {},
		"operating_system":  {},
	}
	var ids []int64
	for _, t := range types {
		if _, ok := want[strings.ToLower(strings.TrimSpace(t.Name))]; ok {
			ids = append(ids, t.ID)
		}
	}
	return ids, nil
}

func (c *Client) DownloadCover(ctx context.Context, imageID string) (contentType string, data []byte, err error) {
	u := CoverURL(imageID)
	if u == "" {
		return "", nil, fmt.Errorf("empty cover")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return "", nil, err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", nil, fmt.Errorf("cover download: %s", resp.Status)
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

func (c *Client) post(ctx context.Context, path, body string, dest any) error {
	token, err := c.accessToken(ctx)
	if err != nil {
		return err
	}
	c.throttle()
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBase+path, strings.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Client-ID", c.ClientID)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/json")
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("igdb %s: %s %s", path, resp.Status, strings.TrimSpace(string(b)))
	}
	return json.Unmarshal(b, dest)
}

func (c *Client) throttle() {
	c.mu.Lock()
	defer c.mu.Unlock()
	wait := 260*time.Millisecond - time.Since(c.lastReq)
	if wait > 0 {
		time.Sleep(wait)
	}
	c.lastReq = time.Now()
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
	form.Set("client_id", c.ClientID)
	form.Set("client_secret", c.Secret)
	form.Set("grant_type", "client_credentials")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, tokenURL+"?"+form.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		return "", fmt.Errorf("twitch token: %s %s", resp.Status, strings.TrimSpace(string(b)))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&tok); err != nil {
		return "", err
	}
	c.mu.Lock()
	c.token = tok.AccessToken
	exp := tok.ExpiresIn
	if exp <= 0 {
		exp = 3600
	}
	c.tokenExp = time.Now().Add(time.Duration(exp) * time.Second)
	c.mu.Unlock()
	return tok.AccessToken, nil
}

type igdbGame struct {
	ID               int64  `json:"id"`
	Name             string `json:"name"`
	Summary          string `json:"summary"`
	FirstReleaseDate int64  `json:"first_release_date"`
	Cover            *struct {
		ImageID string `json:"image_id"`
	} `json:"cover"`
	Platforms []struct {
		ID   int64  `json:"id"`
		Name string `json:"name"`
	} `json:"platforms"`
}

func (g igdbGame) toGame() Game {
	out := Game{
		ID:      g.ID,
		Name:    g.Name,
		Summary: g.Summary,
	}
	if g.Cover != nil {
		out.CoverImageID = g.Cover.ImageID
		u := CoverURL(g.Cover.ImageID)
		out.CoverURL = &u
	}
	if g.FirstReleaseDate > 0 {
		d := time.Unix(g.FirstReleaseDate, 0).UTC().Format("2006-01-02")
		out.FirstReleaseDate = &d
	}
	for _, p := range g.Platforms {
		out.Platforms = append(out.Platforms, Platform{ID: p.ID, Name: p.Name})
	}
	if out.Platforms == nil {
		out.Platforms = []Platform{}
	}
	return out
}

func escape(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	s = strings.ReplaceAll(s, `"`, `\"`)
	return s
}
