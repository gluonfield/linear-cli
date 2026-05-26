package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/gluonfield/linear-cli/cmd"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "version" {
		fmt.Println(version)
		os.Exit(0)
	}
	if err := cmd.Execute(); err != nil {
		if cmd.IsCompact() {
			fmt.Fprintf(os.Stderr, "ERR: %s\n", err.Error())
		} else {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			if strings.Contains(err.Error(), "not authenticated") || strings.Contains(err.Error(), "LINEAR_API_KEY") {
				fmt.Fprintln(os.Stderr, "\nOptions:")
				fmt.Fprintln(os.Stderr, "  - export LINEAR_API_KEY=lin_api_...   (personal API key)")
				fmt.Fprintln(os.Stderr, "  - linear oauth setup && linear oauth login   (OAuth flow)")
				fmt.Fprintln(os.Stderr, "  - psst --global LINEAR_API_KEY -- linear ... (psst-managed key)")
			}
		}
		os.Exit(1)
	}
}
