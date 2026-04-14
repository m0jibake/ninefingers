package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
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
	s.mux.HandleFunc("POST /api/summaries/{id}/chat", s.handleChat)
	s.mux.HandleFunc("GET /api/summaries/{id}/messages", s.handleListMessages)
	s.mux.HandleFunc("POST /api/export-to-blog", s.handleExportToBlog)
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

type exportRequest struct {
	SummaryText string `json:"summaryText"`
	VideoTitle string `json:"videoTitle"`
	VideoURL string `json:"videoURL"`
}

type githubCreateFileRequest struct {
	Message string `json:"message"`
	Content string `json:"content"`
	Branch  string `json:"branch"`
}

type githubCreateFileResponse struct {
	Content struct {
		Path    string `json:"path"`
		HTMLURL string `json:"html_url"`
	} `json:"content"`
	Commit struct {
		SHA string `json:"sha"`
	} `json:"commit"`
	Message string `json:"message"`
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
		// SSE spec: multi-line data must use separate "data:" lines
		lines := strings.Split(data, "\n")
		fmt.Fprintf(w, "event: %s\n", event)
		for _, line := range lines {
			fmt.Fprintf(w, "data: %s\n", line)
		}
		fmt.Fprintf(w, "\n")
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
		ID:           summaryID,
		VideoURL:     req.URL,
		VideoTitle:   videoTitle,
		Model:        req.Model,
		Language:     req.Language,
		Prompt:       req.Prompt,
		CaptionsText: captions,
		CreatedAt:    time.Now(),
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

	// Seed the conversation history with the initial assistant response
	initMsg := &store.ChatMessage{
		ID:        uuid.New().String(),
		SummaryID: summaryID,
		Role:      "assistant",
		Content:   fullText.String(),
		CreatedAt: time.Now(),
	}
	if err := s.store.SaveMessage(initMsg); err != nil {
		log.Printf("failed to seed initial message: %v", err)
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

type chatRequest struct {
	Message string `json:"message"`
}

func (s *Server) handleChat(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")

	sum, err := s.store.GetSummary(id)
	if err != nil || sum == nil {
		http.Error(w, "summary not found", http.StatusNotFound)
		return
	}

	var req chatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Message == "" {
		http.Error(w, "message is required", http.StatusBadRequest)
		return
	}

	_ = godotenv.Load()
	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		http.Error(w, "NVIDIA_API_KEY is not set", http.StatusInternalServerError)
		return
	}

	history, err := s.store.ListMessages(id)
	if err != nil {
		http.Error(w, "failed to load history", http.StatusInternalServerError)
		return
	}

	// Save the user message before streaming
	userMsg := &store.ChatMessage{
		ID:        uuid.New().String(),
		SummaryID: id,
		Role:      "user",
		Content:   req.Message,
		CreatedAt: time.Now(),
	}
	if err := s.store.SaveMessage(userMsg); err != nil {
		log.Printf("failed to save user message: %v", err)
	}

	// Build message history for the LLM
	var systemContent string
	if sum.CaptionsText != "" {
		systemContent = fmt.Sprintf(
			"You are a helpful assistant. The user watched a YouTube video titled %q.\n\n"+
				"Here are the original captions from the video:\n\n%s\n\n"+
				"Here is an AI-generated summary of the video:\n\n%s\n\n"+
				"Answer the user's follow-up questions using the captions and summary above.",
			sum.VideoTitle, sum.CaptionsText, sum.SummaryText,
		)
	} else {
		systemContent = fmt.Sprintf(
			"You are a helpful assistant. The user watched a YouTube video titled %q. "+
				"Here is an AI-generated summary of the video:\n\n%s\n\n"+
				"Answer the user's follow-up questions based on this summary.",
			sum.VideoTitle, sum.SummaryText,
		)
	}
	messages := []summarize.Message{{Role: "system", Content: systemContent}}
	// Add all prior messages except the seeded initial assistant message (index 0)
	for _, m := range history {
		messages = append(messages, summarize.Message{Role: m.Role, Content: m.Content})
	}
	messages = append(messages, summarize.Message{Role: "user", Content: req.Message})

	// Set up SSE
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	sendSSE := func(event, data string) {
		lines := strings.Split(data, "\n")
		fmt.Fprintf(w, "event: %s\n", event)
		for _, line := range lines {
			fmt.Fprintf(w, "data: %s\n", line)
		}
		fmt.Fprintf(w, "\n")
		flusher.Flush()
	}

	var fullReply strings.Builder
	err = summarize.StreamChat(apiKey, sum.Model, messages, func(token string) error {
		fullReply.WriteString(token)
		sendSSE("token", token)
		return nil
	})
	if err != nil {
		b, _ := json.Marshal(map[string]string{"message": err.Error()})
		sendSSE("error", string(b))
		return
	}

	// Persist the assistant reply
	assistantMsg := &store.ChatMessage{
		ID:        uuid.New().String(),
		SummaryID: id,
		Role:      "assistant",
		Content:   fullReply.String(),
		CreatedAt: time.Now(),
	}
	if err := s.store.SaveMessage(assistantMsg); err != nil {
		log.Printf("failed to save assistant message: %v", err)
	}

	sendSSE("done", "")
}

func (s *Server) handleListMessages(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	msgs, err := s.store.ListMessages(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if msgs == nil {
		msgs = []store.ChatMessage{}
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(msgs)
}

func (s *Server) handleExportToBlog(w http.ResponseWriter, r *http.Request) {

	githubToken := os.Getenv("GITHUB_TOKEN")
	if githubToken == "" {
		http.Error(w, "GITHUB_TOKEN is not set", http.StatusBadGateway)
		return
	}
	blogRepo := os.Getenv("BLOG_REPO")
	if blogRepo == "" {
		http.Error(w, "BLOG_REPO is not set", http.StatusBadGateway)
		return
	}
	splitBlogRepo := strings.Split(blogRepo,`/`)
	if len(splitBlogRepo) != 2 {
		http.Error(w, "blog repo requires format owner/repo", http.StatusBadGateway)
		return	
	}
	owner := splitBlogRepo[0]
	repo := splitBlogRepo[1] 
	today := time.Now().Format("2006-01-02")
	


	var req exportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.SummaryText == "" {
		http.Error(w, "SummaryText is required", http.StatusBadRequest)
		return
	}
	if req.VideoTitle == "" {
		http.Error(w, "VideoTitle is required", http.StatusBadRequest)
		return
	}
	if req.VideoURL == "" {
		http.Error(w, "VideoURL is required", http.StatusBadRequest)
		return
	}
	blogBranch := "main"

	safeTitle := strings.ToLower(req.VideoTitle)
	safeTitle = strings.ReplaceAll(safeTitle, " ", "-")
	safeTitle = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			return r
		}
		return -1
	}, safeTitle)
	safeTitle = strings.Trim(safeTitle, "-")
	safeTitle = strings.ReplaceAll(safeTitle, "--", "-")

	blogFileName := fmt.Sprintf("%s-things-i-learned-from-%s.md", today, safeTitle)

	markdownContent := fmt.Sprintf("---\ntitle: Things I learned from - %s\ndate: %s\nsource: %s\nai_generated: true\nauthor: m0jibake\n---\n\n%s\n\n---\n*This summary was generated by AI from the YouTube video captions.*\n",
		req.VideoTitle, today, req.VideoURL, req.SummaryText)

	markdownContentB64 := base64.StdEncoding.EncodeToString([]byte(markdownContent))

	payload := githubCreateFileRequest{
		Message: fmt.Sprintf("Publish AI summary: %s", req.VideoTitle),
		Content: markdownContentB64,
		Branch:  blogBranch,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to encode github payload: %v", err), http.StatusInternalServerError)
		return
	}

	apiURL := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/_posts/%s",
		owner,
		repo,
		url.PathEscape(blogFileName),
	)

	ghReq, err := http.NewRequest(http.MethodPut, apiURL, bytes.NewReader(payloadBytes))
	if err != nil {
		http.Error(w, fmt.Sprintf("failed to create github request: %v", err), http.StatusInternalServerError)
		return
	}

	ghReq.Header.Set("Authorization", "Bearer "+githubToken)
	ghReq.Header.Set("Accept", "application/vnd.github+json")
	ghReq.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(ghReq)
	if err != nil {
		http.Error(w, fmt.Sprintf("github request failed: %v", err), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		http.Error(
			w,
			fmt.Sprintf("github api error (%d): %s", resp.StatusCode, string(respBody)),
			http.StatusBadGateway,
		)
		return
	}

	var ghResp githubCreateFileResponse
	if err := json.Unmarshal(respBody, &ghResp); err != nil {
		http.Error(w, fmt.Sprintf("failed to parse github response: %v", err), http.StatusBadGateway)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"path":      ghResp.Content.Path,
		"html_url":  ghResp.Content.HTMLURL,
		"commit_sha": ghResp.Commit.SHA,
	})

}
