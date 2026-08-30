package config

import (
	"fmt"
	"os"
	"strings"
)

type Config struct {
	AppPassword        string
	DataDir            string
	HTTPAddr           string
	CookieSecure       bool
	IGDBClientID       string
	IGDBClientSecret   string
	PriceChartingToken string
}

func FromEnv() (Config, error) {
	cfg := Config{
		AppPassword:        strings.TrimSpace(os.Getenv("APP_PASSWORD")),
		DataDir:            getenv("DATA_DIR", "./data"),
		HTTPAddr:           getenv("HTTP_ADDR", ":8080"),
		CookieSecure:       truthy(os.Getenv("COOKIE_SECURE")),
		IGDBClientID:       os.Getenv("IGDB_CLIENT_ID"),
		IGDBClientSecret:   os.Getenv("IGDB_CLIENT_SECRET"),
		PriceChartingToken: strings.TrimSpace(os.Getenv("PRICECHARTING_TOKEN")),
	}
	if strings.TrimSpace(cfg.AppPassword) == "" {
		return Config{}, fmt.Errorf("APP_PASSWORD is required")
	}
	return cfg, nil
}

func (c Config) IGDBConfigured() bool {
	return c.IGDBClientID != "" && c.IGDBClientSecret != ""
}

func (c Config) PriceChartingConfigured() bool {
	return c.PriceChartingToken != ""
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func truthy(v string) bool {
	switch strings.ToLower(strings.TrimSpace(v)) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}
