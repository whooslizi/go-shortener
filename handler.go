package main

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io/fs"
	"log"
	"net/http"
	"time"
)

//go:embed templates/*
var templateFS embed.FS

type Handler struct {
	cfg *Config
	db  *DB
	tpl *template.Template
}

func NewHandler(cfg *Config, db *DB) (*Handler, error) {
	funcMap := template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format("2006-01-02 15:04")
		},
		"formatExpiry": func(t *time.Time) string {
			if t == nil {
				return "Never"
			}
			if t.Before(time.Now()) {
				return "Expired"
			}
			remaining := time.Until(*t)
			if remaining < time.Hour {
				return remaining.Round(time.Minute).String() + " remaining"
			}
			if remaining < 24*time.Hour {
				hours := int(remaining.Hours())
				if hours == 1 {
					return "1 hour remaining"
				}
				return fmt.Sprintf("%d hours remaining", hours)
			}
			days := int(remaining.Hours() / 24)
			if days == 1 {
				return "1 day remaining"
			}
			return fmt.Sprintf("%d days remaining", days)
		},
		"isExpired": func(t *time.Time) bool {
			if t == nil {
				return false
			}
			return t.Before(time.Now())
		},
		"shortURL": func(code string, domain string) string {
			return domain + "/" + code
		},
	}

	tpl, err := template.New("").Funcs(funcMap).ParseFS(templateFS, "templates/*.html")
	if err != nil {
		return nil, err
	}

	return &Handler{cfg: cfg, db: db, tpl: tpl}, nil
}

func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	staticFS, _ := fs.Sub(templateFS, "templates")
	mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))

	mux.HandleFunc("GET /health", h.handleHealth)
	mux.HandleFunc("GET /admin", h.handleAdmin)
	mux.HandleFunc("GET /api/links", h.handleAPILinks)
	mux.HandleFunc("GET /{shortCode}", h.handleRedirect)
}

func (h *Handler) renderError(w http.ResponseWriter, code int) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	h.tpl.ExecuteTemplate(w, "error.html", map[string]interface{}{"Code": code})
}

func (h *Handler) handleHealth(w http.ResponseWriter, r *http.Request) {
	count, err := h.db.Count()
	if err != nil {
		http.Error(w, "database error", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":     "ok",
		"link_count": count,
		"timestamp":  time.Now().UTC().Format(time.RFC3339),
	})
}

func (h *Handler) handleRedirect(w http.ResponseWriter, r *http.Request) {
	shortCode := r.PathValue("shortCode")

	// root path — nothing to see here
	if shortCode == "" {
		h.renderError(w, http.StatusNoContent)
		return
	}

	if shortCode == "favicon.ico" || shortCode == "robots.txt" {
		http.NotFound(w, r)
		return
	}

	link, err := h.db.GetByShortCode(shortCode)
	if err != nil {
		log.Printf("lookup failed for %s: %v", shortCode, err)
		h.renderError(w, http.StatusInternalServerError)
		return
	}

	if link == nil {
		h.renderError(w, http.StatusNotFound)
		return
	}

	if link.ExpiresAt != nil && link.ExpiresAt.Before(time.Now()) {
		h.renderError(w, http.StatusGone)
		return
	}

	http.Redirect(w, r, link.IPPURL, http.StatusFound)
}

func (h *Handler) handleAdmin(w http.ResponseWriter, r *http.Request) {
	if h.cfg.AdminPassword != "" && !h.checkBasicAuth(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Link Shortener Admin"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	links, err := h.db.GetAll()
	if err != nil {
		log.Printf("failed to fetch links: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Links":          links,
		"ShortURLDomain": h.cfg.ShortURLDomain,
		"Now":            time.Now(),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.tpl.ExecuteTemplate(w, "admin.html", data); err != nil {
		log.Printf("template render error: %v", err)
	}
}

func (h *Handler) handleAPILinks(w http.ResponseWriter, r *http.Request) {
	if h.cfg.AdminPassword != "" && !h.checkBasicAuth(r) {
		w.Header().Set("WWW-Authenticate", `Basic realm="Link Shortener Admin"`)
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
		return
	}

	links, err := h.db.GetAll()
	if err != nil {
		log.Printf("failed to fetch links: %v", err)
		http.Error(w, "Internal server error", http.StatusInternalServerError)
		return
	}

	type LinkResponse struct {
		ShortLink
		ShortURL string `json:"short_url"`
	}

	var response []LinkResponse
	for _, link := range links {
		response = append(response, LinkResponse{
			ShortLink: link,
			ShortURL:  h.cfg.ShortURLDomain + "/" + link.ShortCode,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *Handler) checkBasicAuth(r *http.Request) bool {
	_, password, ok := r.BasicAuth()
	if !ok {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(password), []byte(h.cfg.AdminPassword)) == 1
}
