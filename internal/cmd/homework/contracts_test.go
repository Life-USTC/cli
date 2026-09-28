package homework

import (
	"context"
	"github.com/Life-USTC/CLI/internal/specification"
	"github.com/spf13/cobra"
	"testing"
)

func TestSpecHomeworkSectionList(t *testing.T) {
	t.Run("homework.cli-section-pagination", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecHomeworkFilteredList(t *testing.T) {
	t.Run("homework.cli-filter-after-pages", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecHomeworkPicker(t *testing.T) {
	t.Run("homework.cli-picker-complete-input", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecHomeworkSubscribedSections(t *testing.T) {
	t.Run("homework.cli-subscription-fallback", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecHomeworkCreatedIdentity(t *testing.T) {
	t.Run("homework.cli-created-identity", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func acceptanceAdapter(t *testing.T, in specification.Input) specification.Observation {
	if in.Action == "homework-section" || in.Action == "homework-create" {
		return specification.Execute(t, NewCmdSectionHomework(), in)
	}
	if in.Action == "homework-list" {
		return specification.Execute(t, NewCmdMyHomework(), in)
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.PersistentFlags().String("server", in.ServerURL, "")
	var rows []map[string]any
	var err error
	if in.Action == "homework-picker" {
		opts := myHomeworkListOpts{}
		addMyHomeworkListFlags(cmd, &opts)
		if err := cmd.ParseFlags(in.Arguments); err != nil {
			return specification.Observation{Err: err}
		}
		rows, err = fetchHomeworkPickList(cmd, opts)
	} else {
		rows, err = loadSubscribedSections(cmd)
	}
	return specification.Observation{Data: rows, Err: err}
}
