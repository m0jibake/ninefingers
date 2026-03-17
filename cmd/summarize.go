package cmd

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/joho/godotenv"
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

// ── Main flow ─────────────────────────────────────────────────────────────────

func runSummarize(videoURL, model, language, userPrompt string, verbose bool) error {
	// Load .env
	if err := godotenv.Load(); err != nil {
		fmt.Println("⚠️  No .env file found, falling back to environment variables")
	}

	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("NVIDIA_API_KEY is not set")
	}

	// Step 1: fetch captions
	fmt.Println("📥 Fetching captions with yt-dlp...")
	captions, err := fetchCaptions(videoURL, language, verbose)
	if err != nil {
		return fmt.Errorf("failed to fetch captions: %w", err)
	}
	fmt.Printf("✅ Captions fetched (%d characters)\n\n", len(captions))

	// Step 2: stream LLM summary
	fmt.Println("🤖 Summarizing with", model, "...\n")
	fmt.Println(strings.Repeat("─", 60))
	if err := streamSummary(apiKey, model, userPrompt, captions); err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}
	fmt.Println("\n" + strings.Repeat("─", 60))

	return nil
}

// ── Caption fetching ──────────────────────────────────────────────────────────

func fetchCaptions(videoURL, language string, verbose bool) (string, error) {

	videoURL = strings.ReplaceAll(videoURL, "\\", "")
	// Check yt-dlp is installed
	if _, err := exec.LookPath("yt-dlp"); err != nil {
		return "", fmt.Errorf("yt-dlp not found — install it with: pip install yt-dlp")
	}

	// Write captions to a temp dir
	tmpDir, err := os.MkdirTemp("", "ninefingers-*")
	if err != nil {
		return "", err
	}
	defer os.RemoveAll(tmpDir)

	outputTemplate := filepath.Join(tmpDir, "%(id)s")

	// Try manual subs first, fall back to auto-generated
	args := []string{
		"--write-subs",
		"--write-auto-subs",
		"--sub-langs", language,
		"--sub-format", "vtt",
		"--skip-download",
		"--output", outputTemplate,
		videoURL,
	}
	args = append([]string{
		//"--ca-certificate", "/Library/Application Support/Netskope/STAgent/data/nscacert_combined.pem",
		"--no-check-cert",
		"--write-subs",
	}, args...)

	cmd := exec.Command("yt-dlp", args...)
	//cmd := exec.Command("yt-dlp --ca-certificate \"/Library/Application\ Support/Netskope/STAgent/data/nscacert.pem\"", args...)

	if verbose == true {
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		cmd.Stdout = io.Discard
		cmd.Stderr = io.Discard
	}

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("yt-dlp failed: %w", err)
	}

	// Find the downloaded .vtt file
	vttFile, err := findVTTFile(tmpDir)
	if err != nil {
		return "", err
	}

	// Parse the VTT into plain text
	return parseVTT(vttFile)
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

// parseVTT strips WebVTT markup and returns clean plain text
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

		// Skip header, timestamps, cue identifiers, and blank lines
		if line == "" ||
			line == "WEBVTT" ||
			strings.Contains(line, "-->") ||
			strings.HasPrefix(line, "Kind:") ||
			strings.HasPrefix(line, "Language:") {
			continue
		}

		// Strip inline VTT tags like <00:00:01.000><c>text</c>
		line = stripVTTTags(line)
		if line == "" {
			continue
		}

		// Deduplicate repeated lines (common in auto-captions)
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

func streamSummary(apiKey, model, userPrompt, captions string) error {
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
			log.Println("Warning: could not parse chunk:", err)
			continue
		}

		for _, choice := range chunk.Choices {
			if choice.Delta.Content != "" {
				fmt.Print(choice.Delta.Content)
			}
		}
	}

	return scanner.Err()
}