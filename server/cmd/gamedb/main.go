package main

import (
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"game-db/internal/api"
	"game-db/internal/config"
	"game-db/internal/igdb"
	"game-db/internal/store"
)

//go:embed all:frontend
var embeddedFrontend embed.FS

func main() {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if files, err := config.LoadDotEnv(); err != nil {
		log.Error("dotenv", "err", err)
		os.Exit(1)
	} else if len(files) > 0 {
		log.Info("loaded env file", "path", files)
	}
	cfg, err := config.FromEnv()
	if err != nil {
		log.Error("config", "err", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(cfg.DataDir, 0o755); err != nil {
		log.Error("data dir", "err", err)
		os.Exit(1)
	}
	st, err := store.Open(cfg.DataDir)
	if err != nil {
		log.Error("store", "err", err)
		os.Exit(1)
	}
	defer st.Close()

	var ig *igdb.Client
	if cfg.IGDBConfigured() {
		ig = igdb.New(cfg.IGDBClientID, cfg.IGDBClientSecret)
	}

	h := api.New(cfg, st, ig, log, loadFrontend(log))
	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           h.Router(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Info("listen", "addr", cfg.HTTPAddr, "data", cfg.DataDir, "igdb", cfg.IGDBConfigured())
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("http", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop
	_ = srv.Close()
}

func loadFrontend(log *slog.Logger) fs.FS {
	candidates := []string{
		"web/dist",
		"../web/dist",
		filepath.Join("cmd", "gamedb", "frontend"),
	}
	for _, dir := range candidates {
		if _, err := os.Stat(filepath.Join(dir, "index.html")); err == nil {
			log.Info("serving spa", "dir", dir)
			return os.DirFS(dir)
		}
	}
	sub, err := fs.Sub(embeddedFrontend, "frontend")
	if err != nil {
		log.Warn("embedded frontend missing")
		return nil
	}
	if _, err := sub.Open("index.html"); err != nil {
		log.Warn("embedded frontend empty")
		return sub
	}
	log.Info("serving spa", "dir", "embedded")
	return sub
}
