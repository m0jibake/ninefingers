package server

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"ninefingers/internal/store"
	"ninefingers/internal/summarize"

	"github.com/google/uuid"
	"github.com/joho/godotenv"
)

type Server struct {
	store *store.Store
	mux   *http.ServeMux
}

func New(st *store.Store) *Server {
	s := &Server{store: st, mux: http.NewServeMux()}
	s.routes()
	return s
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("POST /api/summarize", s.handleSummarize)
	s.mux.HandleFunc("GET /api/summaries", s.handleListSummaries)
	s.mux.HandleFunc("GET /api/summaries/{id}", s.handleGetSummary)
	s.mux.HandleFunc("DELETE /api/summaries/{id}", s.handleDeleteSummary)
}

// SetStaticHandler registers a file server for the frontend build output.
func (s *Server) SetStaticHandler(fs http.Handler) {
	s.mux.Handle("GET /", fs)
}

type summarizeRequest struct {
	URL      string `json:"url"`
	Model    string `json:"model"`
	Language string `json:"language"`
	Prompt   string `json:"prompt"`
}

func (s *Server) handleSummarize(w http.ResponseWriter, r *http.Request) {
	var req summarizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.URL == "" {
		http.Error(w, "url is required", http.StatusBadRequest)
		return
	}
	if req.Model == "" {
		req.Model = "z-ai/glm4.7"
	}
	if req.Language == "" {
		req.Language = "en"
	}
	if req.Prompt == "" {
		req.Prompt = "Give me a thorough summary of this YouTube video based on its captions."
	}

	_ = godotenv.Load()
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		http.Error(w, "NVIDIA_API_KEY is not set", http.StatusInternalServerError)
		return
	}

	// Set up SSE headers
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sendSSE := func(event, data string) {
		fmt.Fprintf(w, "event: %s\ndata: %s\n\n", event, data)
		flusher.Flush()
	}

	sendJSON := func(event string, v any) {
		b, _ := json.Marshal(v)
		sendSSE(event, string(b))
	}

	// Fetch video title and captions concurrently
	type titleResult struct {
		title string
		err   error
	}
	titleCh := make(chan titleResult, 1)
	go func() {
		t, err := summarize.FetchVideoTitle(req.URL)
		titleCh <- titleResult{t, err}
	}()

	sendSSE("status", "Fetching captions...")
	captions, err := summarize.FetchCaptions(req.URL, req.Language, false)
	if err != nil {
		sendJSON("error", map[string]string{"message": fmt.Sprintf("Failed to fetch captions: %v", err)})
		return
	}

	tr := <-titleCh
	videoTitle := tr.title
	if tr.err != nil {
		videoTitle = "Untitled Video"
	}

	// Create summary record
	summaryID := uuid.New().String()
	sum := &store.Summary{
		ID:         summaryID,
		VideoURL:   req.URL,
		VideoTitle: videoTitle,
		Model:      req.Model,
		Language:    req.Language,
		Prompt:     req.Prompt,
		CreatedAt:  time.Now(),
	}
	if err := s.store.SaveSummary(sum); err != nil {
		log.Printf("failed to save summary: %v", err)
	}

	// Send metadata to client
	sendJSON("meta", map[string]string{
		"id":          summaryID,
		"video_title": videoTitle,
	})

	sendSSE("status", "Summarizing...")

	// Stream LLM tokens
	var fullText strings.Builder
	err = summarize.StreamSummary(apiKey, req.Model, req.Prompt, captions, func(token string) error {
		fullText.WriteString(token)
		sendSSE("token", token)
		return nil
	})

	if err != nil {
		sendJSON("error", map[string]string{"message": fmt.Sprintf("LLM error: %v", err)})
		return
	}

	// Persist the full summary
	if err := s.store.UpdateSummaryText(summaryID, fullText.String()); err != nil {
		log.Printf("failed to update summary text: %v", err)
	}

	sendSSE("done", "")
}

func (s *Server) handleListSummaries(w http.ResponseWriter, r *http.Request) {
	summaries, err := s.store.ListSummaries()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if summaries == nil {
		summaries = []store.Summary{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summaries)
}

func (s *Server) handleGetSummary(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	sum, err := s.store.GetSummary(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if sum == nil {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(sum)
}

func (s *Server) handleDeleteSummary(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if err := s.store.DeleteSummary(id); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
