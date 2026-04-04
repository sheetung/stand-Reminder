package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"net/http"

	"stand-reminder/internal/app"
	"stand-reminder/internal/config"
)

//go:embed index.html
var assets embed.FS

type Server struct {
	app *app.App
}

type actionRequest struct {
	Action string `json:"action"`
}

func NewServer(application *app.App) *Server {
	return &Server{app: application}
}

func (s *Server) ListenAndServe(addr string) error {
	mux := http.NewServeMux()
	mux.HandleFunc("/", s.handleIndex)
	mux.HandleFunc("/api/status", s.handleStatus)
	mux.HandleFunc("/api/config", s.handleConfig)
	mux.HandleFunc("/api/test-notification", s.handleTestNotification)
	mux.HandleFunc("/api/action", s.handleAction)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleIndex(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path != "/" {
		http.NotFound(w, r)
		return
	}

	data, err := assets.ReadFile("index.html")
	if err != nil {
		http.Error(w, "failed to load ui", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write(data)
}

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, s.app.Snapshot())
}

func (s *Server) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, s.app.Snapshot())
	case http.MethodPost:
		var cfg config.Config
		if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
			return
		}
		if err := s.app.UpdateConfig(cfg); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "saved"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Server) handleTestNotification(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := s.app.TestNotification(); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "notification sent"})
}

func (s *Server) handleAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req actionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid json"})
		return
	}

	switch req.Action {
	case "pause":
		writeJSON(w, http.StatusOK, s.app.Pause())
	case "resume":
		writeJSON(w, http.StatusOK, s.app.Resume())
	case "break":
		writeJSON(w, http.StatusOK, s.app.StartBreak())
	default:
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "unknown action"})
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil {
		http.Error(w, fmt.Sprintf("json encode failed: %v", err), http.StatusInternalServerError)
	}
}
