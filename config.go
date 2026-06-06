package main

import (
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	ImmichURL      string
	ImmichAPIKey   string
	IPPBaseURL     string
	ShortURLDomain string
	Port           int
	PollInterval   time.Duration
	ShortCodeLen   int
	DBPath         string
	AdminPassword  string
}

func LoadConfig() (*Config, error) {
	cfg := &Config{
		Port:         3100,
		PollInterval: 60 * time.Second,
		ShortCodeLen: 8,
		DBPath:       "/data/links.db",
	}

	cfg.ImmichURL = strings.TrimRight(os.Getenv("IMMICH_URL"), "/")
	if cfg.ImmichURL == "" {
		return nil, fmt.Errorf("IMMICH_URL is required")
	}

	cfg.ImmichAPIKey = os.Getenv("IMMICH_API_KEY")
	if cfg.ImmichAPIKey == "" {
		return nil, fmt.Errorf("IMMICH_API_KEY is required")
	}

	cfg.IPPBaseURL = strings.TrimRight(os.Getenv("IPP_BASE_URL"), "/")
	if cfg.IPPBaseURL == "" {
		return nil, fmt.Errorf("IPP_BASE_URL is required")
	}

	cfg.ShortURLDomain = strings.TrimRight(os.Getenv("SHORT_URL_DOMAIN"), "/")
	if cfg.ShortURLDomain == "" {
		return nil, fmt.Errorf("SHORT_URL_DOMAIN is required")
	}

	if v := os.Getenv("PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("PORT must be a number: %w", err)
		}
		cfg.Port = port
	}

	if v := os.Getenv("POLL_INTERVAL"); v != "" {
		d, err := time.ParseDuration(v)
		if err != nil {
			return nil, fmt.Errorf("POLL_INTERVAL must be a valid duration (e.g. 60s, 5m): %w", err)
		}
		cfg.PollInterval = d
	}

	if v := os.Getenv("SHORT_CODE_LENGTH"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n < 4 || n > 32 {
			return nil, fmt.Errorf("SHORT_CODE_LENGTH must be between 4 and 32")
		}
		cfg.ShortCodeLen = n
	}

	if v := os.Getenv("DB_PATH"); v != "" {
		cfg.DBPath = v
	}

	cfg.AdminPassword = os.Getenv("ADMIN_PASSWORD")

	return cfg, nil
}

func (c *Config) LogConfig() {
	log.Println("Configuration:")
	log.Printf("  IMMICH_URL:       %s", c.ImmichURL)
	log.Printf("  IMMICH_API_KEY:   %s...%s", c.ImmichAPIKey[:4], c.ImmichAPIKey[len(c.ImmichAPIKey)-4:])
	log.Printf("  IPP_BASE_URL:     %s", c.IPPBaseURL)
	log.Printf("  SHORT_URL_DOMAIN: %s", c.ShortURLDomain)
	log.Printf("  PORT:             %d", c.Port)
	log.Printf("  POLL_INTERVAL:    %s", c.PollInterval)
	log.Printf("  SHORT_CODE_LENGTH:%d", c.ShortCodeLen)
	log.Printf("  DB_PATH:          %s", c.DBPath)
	if c.AdminPassword != "" {
		log.Printf("  ADMIN_PASSWORD:   (set)")
	} else {
		log.Printf("  ADMIN_PASSWORD:   (not set - admin UI is open)")
	}
}
