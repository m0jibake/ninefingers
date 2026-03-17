package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "ninefingers [youtube-url]",
	Short: "ninefingers — summarize any YouTube video from its captions",
	Long: `ninefingers fetches captions from a YouTube video using yt-dlp,
then sends them to an LLM to produce a clean summary.

Example:
  ninefingers "https://www.youtube.com/watch?v=dQw4w9WgXcQ"
  ninefingers "https://www.youtube.com/watch?v=dQw4w9WgXcQ" --model "moonshotai/kimi-k2-instruct"
  ninefingers "https://www.youtube.com/watch?v=dQw4w9WgXcQ" --language es
  ninefingers "https://www.youtube.com/watch?v=dQw4w9WgXcQ" --prompt "list the key takeaways as bullet points"`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		url := args[0]
		model, _ := cmd.Flags().GetString("model")
		language, _ := cmd.Flags().GetString("language")
		prompt, _ := cmd.Flags().GetString("prompt")
		verbose, _ := cmd.Flags().GetBool("verbose")

		return runSummarize(url, model, language, prompt, verbose)
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	rootCmd.Flags().StringP("model", "m", "z-ai/glm4.7", "LLM model to use for summarization")
	rootCmd.Flags().StringP("language", "l", "en", "Caption language code (e.g. en, es, fr)")
	rootCmd.Flags().StringP("prompt", "p", "Give me a thorough summary of this YouTube video based on its captions.", "Custom instruction to send to the LLM alongside the captions")
	rootCmd.Flags().BoolP("verbose", "v", false, "Logs and warnings verbosity in terminal.")
}