package school

import (
	"context"
	"github.com/Life-USTC/CLI/internal/api"
	ustcschool "github.com/Life-USTC/CLI/internal/school"
	"github.com/Life-USTC/CLI/internal/specification"
	"github.com/spf13/cobra"
	"strconv"
	"testing"
)

func TestSpecSchoolCompleteSemesters(t *testing.T) {
	t.Run("school.cli-complete-semesters", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecSchoolExistingHomework(t *testing.T) {
	t.Run("school.cli-existing-homework", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func acceptanceAdapter(t *testing.T, in specification.Input) specification.Observation {
	client, err := api.NewTypedClient(in.ServerURL, true)
	if err != nil {
		return specification.Observation{Err: err}
	}
	if in.Action == "school-semesters" {
		data, err := fetchAllLifeSemesters(context.Background(), client)
		if err != nil {
			return specification.Observation{Data: data, Err: err}
		}
		selected, parseErr := strconv.Atoi(in.Value)
		if parseErr != nil {
			return specification.Observation{Err: parseErr}
		}
		id, _, _ := resolveLifeSemester(data, ustcschool.Semester{ID: selected})
		return specification.Observation{Data: data, Err: err, Identity: id}
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	rows, err := fetchLifeHomeworksForSection(cmd, client, "7")
	if err != nil {
		observed := specification.Observation{Err: err}
		if rows != nil {
			observed.Data = rows
		}
		return observed
	}
	result, err := syncLifeHomework(cmd, client, "7", map[string]any{"id": 7}, ustcschool.HomeworkItem{Title: "Assignment " + in.Value, EndAt: "2030-09-30T00:00:00Z", Status: "submitted"}, rows, false)
	id, _ := result.LifeHomework["id"].(string)
	return specification.Observation{Data: result, Err: err, Identity: id, Completed: result.Completion, Action: result.Action, CompletionAction: result.CompletionAction}
}
