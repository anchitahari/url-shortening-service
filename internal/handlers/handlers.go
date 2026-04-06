package handlers

import (
	"encoding/json"
	"log"
	"net/http"
	"net/url"
	"strings"
	"url-shortening-service/internal/cache"
	"url-shortening-service/internal/store"
)

type Handler struct {
	store *store.Store
	cache *cache.Cache
}

func New(s *store.Store, c *cache.Cache) *Handler {
	return &Handler{store: s, cache: c}
}

func (h *Handler) CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func (h *Handler) Health(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
}

func (h *Handler) Shorten(w http.ResponseWriter, r *http.Request) {
	// extract short code from URL path
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/shorten"), "/")
	shortCode := ""
	if len(parts) > 1 {
		shortCode = parts[1]
	}

	// route based on method and whether code is present
	if shortCode == "" {
		switch r.Method {
		case http.MethodPost:
			h.createUrl(w, r)
		default:
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		}
		return
	}

	// check for stats endpoint
	if len(parts) > 2 && parts[2] == "stats" {
		h.getUrl(w, r, shortCode)
	}

	switch r.Method {
	case http.MethodGet:
		h.getUrl(w, r, shortCode)
	case http.MethodPut:
		h.updateUrl(w, r, shortCode)
	case http.MethodDelete:
		h.deleteUrl(w, r, shortCode)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) createUrl(w http.ResponseWriter, r *http.Request) {
	var requestBody struct {
		URL string `json:"url"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil || requestBody.URL == "" {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	parsed, err := url.ParseRequestURI(requestBody.URL)
	if err != nil || parsed.Host == "" || parsed.Scheme == "" {
		http.Error(w, `"error": "invalid URL"`, http.StatusBadRequest)
		return
	}

	url, err := h.store.Create(requestBody.URL)
	if err != nil {
		log.Printf("store.Create error: %v", err)
		http.Error(w, `"error": `+err.Error(), http.StatusInternalServerError)
		return
	}

	h.cache.Set(r.Context(), url.ShortCode, requestBody.URL)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(url)
}

func (h *Handler) getUrl(w http.ResponseWriter, r *http.Request, shortCode string) {
	// check cache first — cache only stores the original URL string
	// so on a hit we still need to fetch the full object from the DB
	// for the response. On a miss we fetch from DB directly.
	u, err := h.store.GetByShortCode(shortCode)
	if err != nil || u == nil {
		http.Error(w, `{"error":"short URL not found"}`, http.StatusNotFound)
		return
	}

	// cache the URL string for future redirect lookups
	h.cache.Set(r.Context(), shortCode, u.OriginalURL)

	// increment access count asynchronously
	go func() {
		if err := h.store.IncrementAccessCount(shortCode); err != nil {
			log.Printf("IncrementAccess error: %v", err)
		}
	}()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(u)
}

func (h *Handler) updateUrl(w http.ResponseWriter, r *http.Request, shortCode string) {
	var requestBody struct {
		URL string `json:"url"`
	}

	err := json.NewDecoder(r.Body).Decode(&requestBody)
	if err != nil || requestBody.URL == "" {
		http.Error(w, `{"error": "invalid request body"}`, http.StatusBadRequest)
		return
	}

	parsed, err := url.ParseRequestURI(requestBody.URL)
	if err != nil || parsed.Host == "" || parsed.Scheme == "" {
		http.Error(w, `"error": "invalid URL"`, http.StatusBadRequest)
		return
	}

	result, err := h.store.Update(shortCode, requestBody.URL)
	if store.ItemNotFound(err) {
		http.Error(w, `"error": "short URL not found"`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `"error": "internal server error"`, http.StatusInternalServerError)
		return
	}

	h.cache.Delete(r.Context(), shortCode)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

func (h *Handler) deleteUrl(w http.ResponseWriter, r *http.Request, shortCode string) {
	err := h.store.Delete(shortCode)
	if store.ItemNotFound(err) {
		http.Error(w, `"error": "short URL not found"`, http.StatusNotFound)
		return
	} else if err != nil {
		http.Error(w, `"error": "internal server error"`, http.StatusInternalServerError)
		return
	}

	h.cache.Delete(r.Context(), shortCode)
	w.WriteHeader(http.StatusNoContent)
}
