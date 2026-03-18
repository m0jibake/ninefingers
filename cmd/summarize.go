package cmd

import (
	"fmt"
	"os"
	"strings"

	"ninefingers/internal/summarize"

	"github.com/joho/godotenv"
)

func runSummarize(videoURL, model, language, userPrompt string, verbose bool) error {
	if err := godotenv.Load(); err != nil {
		fmt.Println("⚠️  No .env file found, falling back to environment variables")
	}

	apiKey := os.Getenv("NVIDIA_API_KEY")
	if apiKey == "" {
		return fmt.Errorf("NVIDIA_API_KEY is not set")
	}

	fmt.Println("📥 Fetching captions with yt-dlp...")
	captions, err := summarize.FetchCaptions(videoURL, language, verbose)
	if err != nil {
		return fmt.Errorf("failed to fetch captions: %w", err)
	}
	fmt.Printf("✅ Captions fetched (%d characters)\n\n", len(captions))

	fmt.Println("🤖 Summarizing with", model, "...\n")
	fmt.Println(strings.Repeat("─", 60))
	err = summarize.StreamSummary(apiKey, model, userPrompt, captions, func(token string) error {
		fmt.Print(token)
		return nil
	})
	if err != nil {
		return fmt.Errorf("LLM error: %w", err)
	}
	fmt.Println("\n" + strings.Repeat("─", 60))

	return nil
}