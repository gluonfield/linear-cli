package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/gluonfield/linear-cli/api"
)

var teamsCmd = &cobra.Command{
	Use:   "teams",
	Short: "List teams",
	RunE: func(cmd *cobra.Command, args []string) error {
		q := `query { teams { nodes { id name key } } }`
		var result struct {
			Teams struct {
				Nodes []struct {
					ID   string `json:"id"`
					Name string `json:"name"`
					Key  string `json:"key"`
				} `json:"nodes"`
			} `json:"teams"`
		}
		if err := api.Query(q, &result); err != nil {
			return err
		}

		nodes := result.Teams.Nodes

		switch effectiveFormat() {
		case "json":
			return writeJSON(nodes)
		case "id-only":
			for _, t := range nodes {
				fmt.Println(t.Key)
			}
			return nil
		}

		if optQuiet {
			for _, t := range nodes {
				fmt.Printf("%s\t%s\n", t.Key, t.Name)
			}
			return nil
		}

		for _, t := range nodes {
			fmt.Printf("%s  %s  %s\n", t.Key, t.Name, t.ID)
		}
		return nil
	},
}

func init() {
	rootCmd.AddCommand(teamsCmd)
}
