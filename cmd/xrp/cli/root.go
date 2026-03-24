package cli

import (
	"fmt"
	"os"

	"github.com/N3M1K/xrp/internal/config"
	"github.com/spf13/cobra"
)

var cfg *config.Config

var rootCmd = &cobra.Command{
	Use:   "xrp",
	Short: "XRP is a local development reverse proxy with magically automatic TLS",
	Long:  `XRP automatically scans your system for running development servers and magically proxies them to .local domains with trusted HTTPS.`,
}

// Execute adds all child commands to the root command and sets flags appropriately.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	cobra.OnInitialize(initConfig)
}

func initConfig() {
	var err error
	cfg, err = config.LoadConfig()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading config: %v\n", err)
		os.Exit(1)
	}
}
