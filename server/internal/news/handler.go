// SPDX-License-Identifier: AGPL-3.0-or-later

package news

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
)

// Register adds the news routes to mux.
func (s *Service) Register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/news/{uid}", func(w http.ResponseWriter, r *http.Request) {
		uid, err := strconv.Atoi(r.PathValue("uid"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "bad region"})
			return
		}
		items, err := s.List(r.Context(), uid, 30)
		switch {
		case errors.Is(err, ErrUnsupported):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "unsupported"})
		case err != nil:
			s.log.Error("news list", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		default:
			writeJSON(w, http.StatusOK, map[string][]Item{"items": items})
		}
	})
	// Post IDs contain a slash ("channel/123"), so the item endpoint takes a query parameter.
	mux.HandleFunc("GET /api/news-item", func(w http.ResponseWriter, r *http.Request) {
		item, _, err := s.Post(r.Context(), r.URL.Query().Get("id"))
		switch {
		case errors.Is(err, ErrNotFound):
			writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		case err != nil:
			s.log.Error("news item", "err", err)
			writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "internal"})
		default:
			writeJSON(w, http.StatusOK, item)
		}
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
