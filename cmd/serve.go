package cmd

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"runtime"

	"ninefingers/internal/server"
	"ninefingers/internal/store"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start the ninefingers web UI",
	RunE: func(cmd *cobra.Command, args []string) error {
		port, _ := cmd.Flags().GetInt("port")
		noBrowser, _ := cmd.Flags().GetBool("no-browser")

		st, err := store.New()
		if err != nil {
			return fmt.Errorf("failed to open database: %w", err)
		}
		defer st.Close()

		srv := server.New(st)

		// Serve SvelteKit static build from web/build
		fs := http.FileServer(http.Dir("web/build"))
		srv.SetStaticHandler(fs)

		addr := fmt.Sprintf(":%d", port)
		url := fmt.Sprintf("http://localhost:%d", port)

		fmt.Printf("🌐 Ninefingers running at %s\n", url)

		if !noBrowser {
			openBrowser(url)
		}

		return http.ListenAndServe(addr, srv)
	},
}

func init() {
	rootCmd.AddCommand(serveCmd)
	serveCmd.Flags().IntP("port", "P", 8080, "Port to listen on")
	serveCmd.Flags().Bool("no-browser", false, "Don't open browser automatically")
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", url)
	case "linux":
		cmd = exec.Command("xdg-open", url)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	}
	if cmd != nil {
		if err := cmd.Start(); err != nil {
			log.Printf("failed to open browser: %v", err)
		}
	}
}
