package cmd

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/gluonfield/linear-cli/api"
)

var (
	projTeamFilter  string
	projStatusFilter string
)

var projectsCmd = &cobra.Command{
	Use:   "projects",
	Short: "List projects",
	RunE: func(cmd *cobra.Command, args []string) error {
		filter := ""
		parts := []string{}
		if projTeamFilter != "" {
			parts = append(parts, fmt.Sprintf(`lead: { name: { eq: "%s" } }`, projTeamFilter))
		}
		if projStatusFilter != "" {
			parts = append(parts, fmt.Sprintf(`status: { name: { eq: "%s" } }`, projStatusFilter))
		}
		if len(parts) > 0 {
			filter = fmt.Sprintf("filter: { %s }", strings.Join(parts, ", "))
		}

		q := fmt.Sprintf(`query { projects(%s first: 50) { nodes { id name slugId description status { name color } lead { name email } startDate targetDate progress createdAt } } }`, filter)

		var result struct {
			Projects struct {
				Nodes []struct {
					ID          string  `json:"id"`
					Name        string  `json:"name"`
					SlugID      string  `json:"slugId"`
					Description string  `json:"description"`
					State       string  `json:"state"`
					Status      *struct {
						Name  string `json:"name"`
						Color string `json:"color"`
					} `json:"status"`
					Lead *struct {
						Name  string `json:"name"`
						Email string `json:"email"`
					} `json:"lead"`
					StartDate  string  `json:"startDate"`
					TargetDate string  `json:"targetDate"`
					Progress   float64 `json:"progress"`
					CreatedAt  string  `json:"createdAt"`
				} `json:"nodes"`
			} `json:"projects"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		nodes := result.Projects.Nodes
		if len(nodes) == 0 {
			if effectiveFormat() == "json" {
				return writeJSON([]any{})
			}
			fmt.Println("No projects found.")
			return nil
		}

		return outputListItems(toAnySlice(nodes), func(item any) string {
			if n, ok := item.(struct {
				SlugID string `json:"slugId"`
				Name   string `json:"name"`
			}); ok {
				return n.SlugID + "\t" + n.Name
			}
			return ""
		}, []string{"slugId", "name", "status.name", "progress"}, func() {
			w := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "SLUG\tNAME\tSTATUS\tLEAD\tPROGRESS")
			for _, p := range nodes {
				status := p.State
				if p.Status != nil {
					status = p.Status.Name
				}
				lead := "-"
				if p.Lead != nil {
					lead = p.Lead.Name
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.0f%%\n", p.SlugID, p.Name, status, lead, p.Progress*100)
			}
			w.Flush()
		})
	},
}

var (
	projCreateName   string
	projCreateDesc   string
	projCreateTeam   string
	projCreateStatus string
)

var projectCreateCmd = &cobra.Command{
	Use:   "project-create",
	Short: "Create a project",
	RunE: func(cmd *cobra.Command, args []string) error {
		if projCreateName == "" {
			return fmt.Errorf("--name is required")
		}

		input := fmt.Sprintf(`name: "%s"`, escapeGraphQL(projCreateName))
		if projCreateDesc != "" {
			input += fmt.Sprintf(`, description: "%s"`, escapeGraphQL(projCreateDesc))
		}
		if projCreateTeam != "" {
			teamID, err := resolveTeamID(projCreateTeam)
			if err != nil {
				return err
			}
			input += fmt.Sprintf(`, teamId: "%s"`, teamID)
		}

		q := fmt.Sprintf(`mutation { projectCreate(input: { %s }) { project { id name key url } } }`, input)

		var result struct {
			ProjectCreate struct {
				Project struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					SlugID string `json:"slugId"`
					URL    string `json:"url"`
				} `json:"project"`
			} `json:"projectCreate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		p := result.ProjectCreate.Project

		switch effectiveFormat() {
		case "json":
			return writeJSON(p)
		case "id-only":
			fmt.Println(p.SlugID)
			return nil
		}
		if optQuiet {
			fmt.Printf("%s\t%s\n", p.SlugID, p.URL)
			return nil
		}

		fmt.Printf("Created: %s - %s\n", p.SlugID, p.Name)
		fmt.Printf("URL: %s\n", p.URL)
		return nil
	},
}

var projectUpdateCmd = &cobra.Command{
	Use:   "project-update [project-id]",
	Short: "Update a project",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		id := args[0]
		fields := []string{}
		if projCreateName != "" {
			fields = append(fields, fmt.Sprintf(`name: "%s"`, escapeGraphQL(projCreateName)))
		}
		if projCreateDesc != "" {
			fields = append(fields, fmt.Sprintf(`description: "%s"`, escapeGraphQL(projCreateDesc)))
		}
		if projCreateStatus != "" {
			fields = append(fields, fmt.Sprintf(`statusId: "%s"`, projCreateStatus))
		}
		if len(fields) == 0 {
			return fmt.Errorf("no updates specified")
		}

		q := fmt.Sprintf(`mutation { projectUpdate(id: "%s", input: { %s }) { success project { id name key } } }`, id, strings.Join(fields, ", "))

		var result struct {
			ProjectUpdate struct {
				Success bool `json:"success"`
				Project struct {
					ID     string `json:"id"`
					Name   string `json:"name"`
					SlugID string `json:"slugId"`
				} `json:"project"`
			} `json:"projectUpdate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		p := result.ProjectUpdate.Project

		switch effectiveFormat() {
		case "json":
			return writeJSON(p)
		case "id-only":
			fmt.Println(p.SlugID)
			return nil
		}
		if optQuiet {
			fmt.Println(p.SlugID)
			return nil
		}

		fmt.Printf("Updated: %s - %s\n", p.SlugID, p.Name)
		return nil
	},
}

func init() {
	projectsCmd.Flags().StringVarP(&projTeamFilter, "team", "t", "", "filter by team key")
	projectsCmd.Flags().StringVar(&projStatusFilter, "status", "", "filter by status name")

	projectCreateCmd.Flags().StringVarP(&projCreateName, "name", "n", "", "project name (required)")
	projectCreateCmd.Flags().StringVarP(&projCreateDesc, "desc", "d", "", "description")
	projectCreateCmd.Flags().StringVarP(&projCreateTeam, "team", "t", "", "team key")

	projectUpdateCmd.Flags().StringVarP(&projCreateName, "name", "n", "", "new name")
	projectUpdateCmd.Flags().StringVarP(&projCreateDesc, "desc", "d", "", "new description")
	projectUpdateCmd.Flags().StringVar(&projCreateStatus, "status-id", "", "new status ID")

	rootCmd.AddCommand(projectsCmd)
	rootCmd.AddCommand(projectCreateCmd)
	rootCmd.AddCommand(projectUpdateCmd)
}
