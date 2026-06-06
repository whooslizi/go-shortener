package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

type ImmichSharedLink struct {
	ID          string  `json:"id"`
	Key         string  `json:"key"`
	Type        string  `json:"type"`
	Description *string `json:"description"`
	ExpiresAt   *string `json:"expiresAt"`
	CreatedAt   string  `json:"createdAt"`
}

type Poller struct {
	cfg    *Config
	db     *DB
	client *http.Client
}

func NewPoller(cfg *Config, db *DB) *Poller {
	return &Poller{
		cfg: cfg,
		db:  db,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (p *Poller) Start() {
	log.Printf("poller started (interval: %s)", p.cfg.PollInterval)

	p.poll()

	ticker := time.NewTicker(p.cfg.PollInterval)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(5 * time.Minute)
	defer cleanupTicker.Stop()

	for {
		select {
		case <-ticker.C:
			p.poll()
		case <-cleanupTicker.C:
			p.cleanup()
		}
	}
}

func (p *Poller) poll() {
	links, err := p.fetchSharedLinks()
	if err != nil {
		log.Printf("fetch error: %v", err)
		return
	}

	log.Printf("got %d shared links from immich", len(links))

	// track which keys still exist so we can remove orphans
	activeKeys := make(map[string]bool)

	for _, link := range links {
		activeKeys[link.Key] = true

		var expiresAt *time.Time
		if link.ExpiresAt != nil {
			t, err := time.Parse(time.RFC3339, *link.ExpiresAt)
			if err != nil {
				t, err = time.Parse(time.RFC3339Nano, *link.ExpiresAt)
				if err != nil {
					log.Printf("bad expiresAt %q for key %s: %v", *link.ExpiresAt, link.Key, err)
					continue
				}
			}
			expiresAt = &t
		}

		// skip already-expired
		if expiresAt != nil && expiresAt.Before(time.Now()) {
			continue
		}

		existing, err := p.db.GetByImmichKey(link.Key)
		if err != nil {
			log.Printf("db lookup error for %s: %v", link.Key, err)
			continue
		}

		if existing != nil {
			// sync expiry if it changed
			oldExp := ""
			if existing.ExpiresAt != nil {
				oldExp = existing.ExpiresAt.UTC().Format(time.RFC3339)
			}
			newExp := ""
			if expiresAt != nil {
				newExp = expiresAt.UTC().Format(time.RFC3339)
			}

			if oldExp != newExp {
				if err := p.db.UpdateExpiry(link.Key, expiresAt); err != nil {
					log.Printf("expiry update failed for %s: %v", link.Key, err)
				} else {
					log.Printf("updated expiry: %s → %s", existing.ShortCode, newExp)
				}
			}
			continue
		}

		shortCode, err := GenerateNanoid(p.cfg.ShortCodeLen)
		if err != nil {
			log.Printf("nanoid error: %v", err)
			continue
		}

		ippURL := fmt.Sprintf("%s/share/%s", p.cfg.IPPBaseURL, link.Key)

		if err := p.db.Insert(shortCode, link.Key, ippURL, expiresAt); err != nil {
			log.Printf("insert failed for %s: %v", link.Key, err)
			continue
		}

		shortURL := fmt.Sprintf("%s/%s", p.cfg.ShortURLDomain, shortCode)
		expiryStr := "never"
		if expiresAt != nil {
			expiryStr = expiresAt.Format(time.RFC3339)
		}
		log.Printf("new: %s → %s (expires: %s)", shortURL, ippURL, expiryStr)
	}

	p.removeOrphans(activeKeys)
}

func (p *Poller) fetchSharedLinks() ([]ImmichSharedLink, error) {
	url := fmt.Sprintf("%s/api/shared-links", p.cfg.ImmichURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("request creation failed: %w", err)
	}
	req.Header.Set("x-api-key", p.cfg.ImmichAPIKey)
	req.Header.Set("Accept", "application/json")

	resp, err := p.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("immich api call failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("immich returned %d: %s", resp.StatusCode, string(body))
	}

	var links []ImmichSharedLink
	if err := json.NewDecoder(resp.Body).Decode(&links); err != nil {
		return nil, fmt.Errorf("decode failed: %w", err)
	}

	return links, nil
}

// removeOrphans deletes short links whose immich share was deleted
func (p *Poller) removeOrphans(activeKeys map[string]bool) {
	allLinks, err := p.db.GetAll()
	if err != nil {
		log.Printf("orphan check failed: %v", err)
		return
	}

	for _, link := range allLinks {
		if !activeKeys[link.ImmichKey] {
			if err := p.db.DeleteByImmichKey(link.ImmichKey); err != nil {
				log.Printf("orphan delete failed %s: %v", link.ShortCode, err)
			} else {
				log.Printf("removed orphan: %s (key %s gone)", link.ShortCode, link.ImmichKey)
			}
		}
	}
}

func (p *Poller) cleanup() {
	deleted, err := p.db.DeleteExpired()
	if err != nil {
		log.Printf("cleanup error: %v", err)
		return
	}
	if deleted > 0 {
		log.Printf("cleaned up %d expired link(s)", deleted)
	}
}
