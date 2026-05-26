package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gluonfield/linear-cli/api"
)

var meCmd = &cobra.Command{
	Use:   "me",
	Short: "Show current user info",
	RunE: func(cmd *cobra.Command, args []string) error {
		q := `query { viewer { id name email } }`
		var result struct {
			Viewer struct {
				ID    string `json:"id"`
				Name  string `json:"name"`
				Email string `json:"email"`
			} `json:"viewer"`
		}
		if err := api.Query(q, &result); err != nil {
			return err
		}

		switch effectiveFormat() {
		case "json":
			return writeJSON(result.Viewer)
		case "id-only":
			fmt.Println(result.Viewer.ID)
			return nil
		}
		if optQuiet {
			fmt.Printf("%s\t%s\n", result.Viewer.Name, result.Viewer.Email)
			return nil
		}

		fmt.Printf("%s <%s>\n", result.Viewer.Name, result.Viewer.Email)
		fmt.Printf("ID: %s\n", result.Viewer.ID)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(meCmd)
}
