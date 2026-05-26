package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/gluonfield/linear-cli/api"
	"github.com/spf13/cobra"
)

var usersCmd = &cobra.Command{
	Use:   "users",
	Short: "List workspace users",
	RunE: func(cmd *cobra.Command, args []string) error {
		q := `query { users(first: 100) { nodes { id name email active admin guest lastSeen } } }`

		var result struct {
			Users struct {
				Nodes []struct {
					ID       string  `json:"id"`
					Name     string  `json:"name"`
					Email    string  `json:"email"`
					Active   bool    `json:"active"`
					Admin    bool    `json:"admin"`
					Guest    bool    `json:"guest"`
					LastSeen *string `json:"lastSeen"`
				} `json:"nodes"`
			} `json:"users"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		nodes := result.Users.Nodes

		switch effectiveFormat() {
		case "json":
			return writeJSON(nodes)
		case "id-only":
			for _, u := range nodes {
				fmt.Println(u.ID)
			}
			return nil
		}

		if optQuiet {
			for _, u := range nodes {
				fmt.Printf("%s\t%s\n", u.Name, u.Email)
			}
			return nil
		}

		w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
		fmt.Fprintln(w, "NAME\tEMAIL\tROLE\tACTIVE")
		for _, u := range nodes {
			role := "member"
			if u.Admin {
				role = "admin"
			}
			if u.Guest {
				role = "guest"
			}
			active := "yes"
			if !u.Active {
				active = "no"
			}
			fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", u.Name, u.Email, role, active)
		}
		w.Flush()
		return nil
	},
}

func init() {
	rootCmd.AddCommand(usersCmd)
}
