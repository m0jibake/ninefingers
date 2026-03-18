package summarize

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ── LLM types ────────────────────────────────────────────────────────────────

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ChatRequest struct {
	Model       string    `json:"model"`
	Temperature float64   `json:"temperature"`
	TopP        float64   `json:"top_p"`
	MaxTokens   int       `json:"max_tokens"`
	Stream      bool      `json:"stream"`
	Messages    []Message `json:"messages"`
}

type Delta struct {
	Content string `json:"content"`
}

type Choice struct {
	Delta        Delta  `json:"delta"`
	FinishReason string `json:"finish_reason"`
}

type StreamChunk struct {
	Choices []Choice `json:"choices"`
}

// ── Caption fetching ──────────────────────────────────────────────────────────

// FetchCaptions downloads captions for a YouTube video using yt-dlp and returns plain text.
func FetchCaptions(videoURL, language string, verbose bool) (string, error) {
	videoURL = strings.ReplaceAll(videoURL, "\\", "")

	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", fmt.Errorf("yt-dlp not found — install it with: pip install yt-dlp")
	}

	tmpDir, err := os.MkdirTemp("", "ninefingers-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	outputTemplate := filepath.Join(tmpDir, "%(id)s")

	args := []string{
		"--no-check-cert",
		"--write-subs",
		"--write-subs",
		"--write-auto-subs",
		"--sub-langs", language,
		"--sub-format", "vtt",
		"--skip-download",
		"--output", outputTemplate,
		videoURL,
	}

	cmd := exec.Command("yt-dlp", args...)

	if verbose {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w", err)
	}

	vttFile, err := findVTTFile(tmpDir)
	if err != nil {
		return "", err
	}

	return parseVTT(vttFile)
}

// FetchVideoTitle retrieves the video title using yt-dlp.
func FetchVideoTitle(videoURL string) (string, error) {
	videoURL = strings.ReplaceAll(videoURL, "\\", "")

	cmd := exec.Command("yt-dlp", "--no-check-cert", "--print", "title", "--skip-download", videoURL)
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("failed to fetch video title: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

func findVTTFile(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".vtt") {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("no .vtt caption file found — the video may not have captions in language requested")
}

func parseVTT(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var lines []string
	seen := map[string]bool{}
	scanner := bufio.NewScanner(f)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" ||
			line == "WEBVTT" ||
			strings.Contains(line, "-->") ||
			strings.HasPrefix(line, "Kind:") ||
			strings.HasPrefix(line, "Language:") {
			continue
		}

		line = stripVTTTags(line)
		if line == "" {
			continue
		}

		if !seen[line] {
			seen[line] = true
			lines = append(lines, line)
		}
	}

	return strings.Join(lines, " "), scanner.Err()
}

func stripVTTTags(s string) string {
	var result strings.Builder
	inTag := false
	for _, r := range s {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			result.WriteRune(r)
		}
	}
	return strings.TrimSpace(result.String())
}

// ── LLM streaming ─────────────────────────────────────────────────────────────

// StreamSummary sends captions to the NVIDIA LLM and calls onToken for each streamed token.
// This allows callers to write to stdout (CLI) or send SSE events (web server).
func StreamSummary(apiKey, model, userPrompt, captions string, onToken func(token string) error) error {
	content := fmt.Sprintf("%s\n\n---CAPTIONS START---\n%s\n---CAPTIONS END---", userPrompt, captions)

	payload := ChatRequest{
		Model:       model,
		Temperature: 1,
		TopP:        1,
		MaxTokens:   16384,
		Stream:      true,
		Messages: []Message{
			{Role: "user", Content: content},
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", "https://integrate.api.nvidia.com/v1/chat/completions", bytes.NewBuffer(body))
	if err != nil {
		return err
	}

	req.Header.Set("accept", "application/json")
	req.Header.Set("authorization", "Bearer "+apiKey)
	req.Header.Set("content-type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("API returned %s: %s", resp.Status, string(b))
	}

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}

		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk StreamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				if err := onToken(choice.Delta.Content); err != nil {
					return err
				}
			}
		}
	}

	return scanner.Err()
}
