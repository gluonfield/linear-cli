package cmd

import (
	"fmt"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/gluonfield/linear-cli/api"
	"github.com/spf13/cobra"
)

var initiativesCmd = &cobra.Command{
	Use:   "initiatives",
	Short: "List initiatives",
	RunE: func(cmd *cobra.Command, args []string) error {
		q := `query { initiatives(first: 50) { nodes { id name description status health targetDate startedAt projects { nodes { name slugId } } } } }`

		var result struct {
			Initiatives struct {
				Nodes []struct {
					ID          string `json:"id"`
					Name        string `json:"name"`
					Description string `json:"description"`
					Status      string `json:"status"`
					Health      string `json:"health"`
					TargetDate  string `json:"targetDate"`
					StartsAt    string `json:"startedAt"`
					Projects    *struct {
						Nodes []struct {
							Name   string `json:"name"`
							SlugID string `json:"slugId"`
						} `json:"nodes"`
					} `json:"projects"`
				} `json:"nodes"`
			} `json:"initiatives"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		nodes := result.Initiatives.Nodes
		if len(nodes) == 0 {
			if effectiveFormat() == "json" {
				return writeJSON([]any{})
			}
			fmt.Println("No initiatives found.")
			return nil
		}

		return outputListItems(toAnySlice(nodes), func(item any) string {
			if n, ok := item.(struct {
				Name   string `json:"name"`
				Status string `json:"status"`
			}); ok {
				return n.Name + "\t" + n.Status
			}
			return ""
		}, []string{"name", "status", "health", "targetDate"}, func() {
			w := tabwriter.NewWriter(os.Stdout, 0, 2, 2, ' ', 0)
			fmt.Fprintln(w, "NAME\tSTATUS\tHEALTH\tTARGET\tPROJECTS")
			for _, i := range nodes {
				projects := "-"
				if i.Projects != nil && len(i.Projects.Nodes) > 0 {
					names := make([]string, len(i.Projects.Nodes))
					for j, p := range i.Projects.Nodes {
						names[j] = p.SlugID
					}
					projects = strings.Join(names, ", ")
				}
				health := i.Health
				if health == "" {
					health = "-"
				}
				target := "-"
				if len(i.TargetDate) >= 10 {
					target = i.TargetDate[:10]
				}
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n", i.Name, i.Status, health, target, projects)
			}
			w.Flush()
		})
	},
}

var (
	initName   string
	initDesc   string
	initTeam   string
	initTarget string
	initStart  string
)

var initCreateCmd = &cobra.Command{
	Use:   "init-create",
	Short: "Create an initiative",
	RunE: func(cmd *cobra.Command, args []string) error {
		if initName == "" {
			return fmt.Errorf("--name is required")
		}

		input := fmt.Sprintf(`name: "%s"`, escapeGraphQL(initName))
		if initDesc != "" {
			input += fmt.Sprintf(`, description: "%s"`, escapeGraphQL(initDesc))
		}
		if initTeam != "" {
			input += fmt.Sprintf(`, teamIds: ["%s"]`, initTeam)
		}
		if initTarget != "" {
			input += fmt.Sprintf(`, targetDate: "%s"`, initTarget)
		}
		if initStart != "" {
			input += fmt.Sprintf(`, startsAt: "%s"`, initStart)
		}

		q := fmt.Sprintf(`mutation { initiativeCreate(input: { %s }) { initiative { id name } } }`, input)

		var result struct {
			InitiativeCreate struct {
				Initiative struct {
					ID   string `json:"id"`
					Name string `json:"name"`
				} `json:"initiative"`
			} `json:"initiativeCreate"`
		}

		if err := api.Query(q, &result); err != nil {
			return err
		}

		init := result.InitiativeCreate.Initiative

		switch effectiveFormat() {
		case "json":
			return writeJSON(init)
		case "id-only":
			fmt.Println(init.ID)
			return nil
		}
		if optQuiet {
			fmt.Printf("%s\t%s\n", init.Name, init.ID)
			return nil
		}

		fmt.Printf("Created initiative: %s (%s)\n", init.Name, init.ID)
		return nil
	},
}

func init() {
	initCreateCmd.Flags().StringVarP(&initName, "name", "n", "", "initiative name (required)")
	initCreateCmd.Flags().StringVarP(&initDesc, "desc", "d", "", "description")
	initCreateCmd.Flags().StringVarP(&initTeam, "team", "t", "", "team key")
	initCreateCmd.Flags().StringVar(&initTarget, "target", "", "target date (ISO 8601)")
	initCreateCmd.Flags().StringVar(&initStart, "start", "", "start date (ISO 8601)")

	rootCmd.AddCommand(initiativesCmd)
	rootCmd.AddCommand(initCreateCmd)
}
