package metadata

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/api"
	"github.com/Life-USTC/CLI/internal/cmd/cmdutil"
	"github.com/Life-USTC/CLI/internal/output"
)

type category struct {
	key  string
	name string
	cols []output.Column
}

var categories = []category{
	{"educationLevels", "Education Levels", []output.Column{{Header: "Name", Key: "nameCn"}, {Header: "ID", Key: "id"}}},
	{"campuses", "Campuses", []output.Column{{Header: "Name", Key: "nameCn"}, {Header: "ID", Key: "id"}}},
	{"courseCategories", "Categories", []output.Column{{Header: "Name", Key: "nameCn"}, {Header: "ID", Key: "id"}}},
	{"classTypes", "Class Types", []output.Column{{Header: "Name", Key: "nameCn"}, {Header: "ID", Key: "id"}}},
	{"courseGradations", "Gradations", []output.Column{{Header: "Name", Key: "nameCn"}, {Header: "ID", Key: "id"}}},
	{"courseTypes", "Course Types", []output.Column{{Header: "Name", Key: "nameCn"}, {Header: "ID", Key: "id"}}},
	{"examModes", "Exam Modes", []output.Column{{Header: "Name", Key: "nameCn"}, {Header: "ID", Key: "id"}}},
	{"teachLanguages", "Teaching Languages", []output.Column{{Header: "Name", Key: "nameCn"}, {Header: "ID", Key: "id"}}},
	{"courseClassifies", "Classifies", []output.Column{{Header: "Name", Key: "nameCn"}, {Header: "ID", Key: "id"}}},
}

func NewCmdMetadata() *cobra.Command {
	return &cobra.Command{
		Use:   "metadata",
		Short: "Show platform metadata dictionaries (campuses, categories, ...)",
		Long: `Show platform metadata dictionaries used throughout the system.

Displays reference data such as campuses, education levels, course categories,
class types, gradations, course types, exam modes, teaching languages, and
course classifies. These values are used as filters and identifiers in other
commands.`,
		Example: `  # Show all metadata tables
  life-ustc catalog metadata

  # Output as JSON for scripting
  life-ustc catalog metadata --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			c, err := api.NewTypedClient(cmdutil.ServerFromCmd(cmd), false)
			if err != nil {
				return err
			}
			data, err := api.ParseResponseRaw(c.GetMetadata(api.Ctx()))
			if err != nil {
				return err
			}
			if output.IsJSON() {
				return output.JSON(data)
			}

			m, ok := data.(map[string]any)
			if !ok {
				return output.JSON(data)
			}

			for _, cat := range categories {
				items, ok := m[cat.key].([]any)
				if !ok || len(items) == 0 {
					continue
				}
				fmt.Println()
				output.Bold(fmt.Sprintf("  %s  (%d)", cat.name, len(items)))
				rows := cmdutil.RowsFromAny(items)
				output.Table(rows, cat.cols)
			}
			return nil
		},
	}
}
