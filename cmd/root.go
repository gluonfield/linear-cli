package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "linear",
	Short: "Linear GraphQL API CLI",
	Long:  "Fast CLI for Linear issue tracking via GraphQL API. Works with psst for secret injection.",
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.SilenceErrors = true
	rootCmd.PersistentFlags().BoolVar(&optQuiet, "quiet", false, "minimal output (id + url only)")
	rootCmd.PersistentFlags().BoolVar(&optJSON, "json", false, "output JSON")
	rootCmd.PersistentFlags().BoolVar(&optCompact, "compact", false, "compact errors (machine-friendly, no hints)")
	rootCmd.PersistentFlags().StringVar(&optFormat, "format", "", "output format: json, tsv, id-only")

	rootCmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
		if optCompact {
			cmd.SilenceUsage = true
		}
		if optFormat != "" && optFormat != "json" && optFormat != "tsv" && optFormat != "id-only" {
			return fmt.Errorf("invalid --format %q (valid: json, tsv, id-only)", optFormat)
		}
		// oauth subcommands handle their own auth (login bootstraps it)
		if cmd.Parent() != nil && cmd.Parent().Name() == "oauth" {
			return nil
		}
		if cmd.Name() == "oauth" {
			return nil
		}
		if os.Getenv("LINEAR_API_KEY") != "" {
			return nil
		}
		if hasStoredOAuthToken() {
			return nil
		}
		return fmt.Errorf("not authenticated: set LINEAR_API_KEY or run 'linear oauth login'")
	}
}
