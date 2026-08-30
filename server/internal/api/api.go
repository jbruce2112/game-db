package api

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"game-db/internal/barcode"
	"game-db/internal/config"
	"game-db/internal/igdb"
	"game-db/internal/pricecharting"
	"game-db/internal/store"
)

const cookieName = "game_db_session"

type Handler struct {
	cfg               config.Config
	store             *store.Store
	igdb              *igdb.Client
	log               *slog.Logger
	frontend          fs.FS
	productLookup     func(ctx context.Context, codes []string) (barcode.Product, error)
	coverBackfillBusy atomic.Bool
	coverInflight     sync.Map
	pc                priceSource
	priceBackfillBusy atomic.Bool
}

func New(cfg config.Config, st *store.Store, ig *igdb.Client, log *slog.Logger, frontend fs.FS) *Handler {
	h := &Handler{cfg: cfg, store: st, igdb: ig, log: log, frontend: frontend}
	if cfg.PriceChartingConfigured() {
		h.pc = pricecharting.New(cfg.PriceChartingToken)
	}
	return h
}

func (h *Handler) Router() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.health)

	mux.HandleFunc("POST /v1/auth/login", h.login)
	mux.HandleFunc("POST /v1/auth/logout", h.auth(h.logout))
	mux.HandleFunc("GET /v1/auth/me", h.auth(h.me))

	mux.HandleFunc("GET /v1/library", h.auth(h.listLibrary))
	mux.HandleFunc("GET /v1/library.csv", h.auth(h.exportLibraryCSV))
	mux.HandleFunc("POST /v1/library/import", h.auth(h.importLibraryCSV))
	mux.HandleFunc("POST /v1/library", h.auth(h.createLibrary))
	mux.HandleFunc("GET /v1/library/{id}", h.auth(h.getLibrary))
	mux.HandleFunc("PATCH /v1/library/{id}", h.auth(h.patchLibrary))
	mux.HandleFunc("DELETE /v1/library/{id}", h.auth(h.deleteLibrary))

	mux.HandleFunc("POST /v1/sync", h.auth(h.sync))
	mux.HandleFunc("GET /v1/platforms", h.auth(h.platforms))
	mux.HandleFunc("GET /v1/search/games", h.auth(h.searchGames))
	mux.HandleFunc("GET /v1/search/barcode", h.auth(h.searchBarcode))
	mux.HandleFunc("GET /v1/covers/{id}", h.auth(h.cover))
	mux.HandleFunc("GET /v1/library/{id}/cover", h.auth(h.libraryCover))

	mux.HandleFunc("GET /{path...}", h.spa)

	return withJSONLogs(h.log, mux)
}

func (h *Handler) health(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (h *Handler) login(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Password string `json:"password"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&body); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid json")
		return
	}
	if !passwordMatch(body.Password, h.cfg.AppPassword) {
		writeErr(w, http.StatusUnauthorized, "invalid password")
		return
	}
	tok, err := randomToken()
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "token")
		return
	}
	if err := h.store.CreateToken(r.Context(), tok); err != nil {
		writeErr(w, http.StatusInternalServerError, "token")
		return
	}
	http.SetCookie(w, &http.Cookie{
		Name:     cookieName,
		Value:    tok,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Secure:   h.cfg.CookieSecure,
		MaxAge:   86400 * 365,
	})
	writeJSON(w, http.StatusOK, map[string]string{"token": tok})
}

func (h *Handler) logout(w http.ResponseWriter, r *http.Request) {
	tok := tokenFromRequest(r)
	_ = h.store.DeleteToken(r.Context(), tok)
	http.SetCookie(w, &http.Cookie{Name: cookieName, Value: "", Path: "/", MaxAge: -1, HttpOnly: true})
	w.WriteHeader(http.StatusNoContent)
}

func (h *Handler) me(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{
		"igdb_configured":          h.cfg.IGDBConfigured(),
		"pricecharting_configured": h.cfg.PriceChartingConfigured(),
	})
}

func (h *Handler) auth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tok := tokenFromRequest(r)
		if !h.store.ValidToken(r.Context(), tok) {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next(w, r)
	}
}

func tokenFromRequest(r *http.Request) string {
	if a := r.Header.Get("Authorization"); strings.HasPrefix(strings.ToLower(a), "bearer ") {
		return strings.TrimSpace(a[7:])
	}
	if c, err := r.Cookie(cookieName); err == nil {
		return c.Value
	}
	return ""
}

func (h *Handler) spa(w http.ResponseWriter, r *http.Request) {
	if h.frontend == nil {
		http.NotFound(w, r)
		return
	}
	p := strings.TrimPrefix(r.URL.Path, "/")
	if p == "" {
		p = "index.html"
	}
	f, err := h.frontend.Open(p)
	if err != nil {
		p = "index.html"
		f, err = h.frontend.Open(p)
		if err != nil {
			http.NotFound(w, r)
			return
		}
	}
	_ = f.Close()
	http.ServeFileFS(w, r, h.frontend, p)
}

func passwordMatch(got, want string) bool {
	got = strings.TrimSpace(got)
	want = strings.TrimSpace(want)
	// Hash first so ConstantTimeCompare is valid even when lengths differ.
	gh := sha256.Sum256([]byte(got))
	wh := sha256.Sum256([]byte(want))
	return subtle.ConstantTimeCompare(gh[:], wh[:]) == 1
}

func randomToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	hx := fmt.Sprintf("%x", b)
	return hx[0:8] + "-" + hx[8:12] + "-" + hx[12:16] + "-" + hx[16:20] + "-" + hx[20:32]
}

func parseID(s string) (string, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if len(s) != 36 {
		return "", fmt.Errorf("invalid id")
	}
	return s, nil
}

func withJSONLogs(log *slog.Logger, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		ww := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(ww, r)
		log.Info("http",
			"method", r.Method,
			"path", r.URL.Path,
			"status", ww.status,
			"ms", time.Since(start).Milliseconds(),
		)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(code int) {
	w.status = code
	w.ResponseWriter.WriteHeader(code)
}

func (h *Handler) requireIGDB(w http.ResponseWriter) bool {
	if h.igdb == nil || !h.cfg.IGDBConfigured() {
		writeErr(w, http.StatusServiceUnavailable, "IGDB not configured")
		return false
	}
	return true
}

func saveCoverFile(dir, id string, data []byte) (string, error) {
	name := id + ".img"
	path := filepath.Join(dir, "covers", name)
	return name, os.WriteFile(path, data, 0o644)
}

func intQuery(r *http.Request, name string) int64 {
	v := r.URL.Query().Get(name)
	if v == "" {
		return 0
	}
	n, _ := strconv.ParseInt(v, 10, 64)
	return n
}
