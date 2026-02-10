package server

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strconv"

	"github.com/retronian/romu/internal/db"
)

//go:embed static
var staticFiles embed.FS

type Server struct {
	db   *db.DB
	port int
}

func New(database *db.DB, port int) *Server {
	return &Server{db: database, port: port}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// API
	mux.HandleFunc("/api/roms", s.handleRoms)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/platforms", s.handlePlatforms)

	// Static files
	staticFS, _ := fs.Sub(staticFiles, "static")
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	addr := fmt.Sprintf(":%d", s.port)
	fmt.Printf("🎮 romu server running at http://localhost%s\n", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) handleRoms(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query().Get("q")
	platform := r.URL.Query().Get("platform")
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	perPage, _ := strconv.Atoi(r.URL.Query().Get("per_page"))
	if page == 0 {
		page = 1
	}
	if perPage == 0 {
		perPage = 50
	}

	files, total, err := s.db.SearchRoms(q, platform, page, perPage)
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}

	type romJSON struct {
		Platform string  `json:"platform"`
		Filename string  `json:"filename"`
		Size     int64   `json:"size"`
		CRC32    string  `json:"crc32"`
		Title    string  `json:"title"`
		TitleEN  *string `json:"title_en"`
		TitleJA  *string `json:"title_ja"`
	}

	roms := make([]romJSON, 0, len(files))
	for _, f := range files {
		title := "-"
		if f.TitleJA != nil {
			title = *f.TitleJA
		} else if f.TitleEN != nil {
			title = *f.TitleEN
		}
		roms = append(roms, romJSON{
			Platform: f.Platform, Filename: f.Filename, Size: f.Size,
			CRC32: f.HashCRC32, Title: title, TitleEN: f.TitleEN, TitleJA: f.TitleJA,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"roms": roms, "total": total, "page": page, "per_page": perPage,
	})
}

func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := s.db.GetStats()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (s *Server) handlePlatforms(w http.ResponseWriter, r *http.Request) {
	platforms, err := s.db.GetPlatforms()
	if err != nil {
		http.Error(w, err.Error(), 500)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(platforms)
}
