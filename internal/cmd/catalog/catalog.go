package catalog

import (
	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/cmd/bus"
	"github.com/Life-USTC/CLI/internal/cmd/course"
	"github.com/Life-USTC/CLI/internal/cmd/link"
	"github.com/Life-USTC/CLI/internal/cmd/metadata"
	"github.com/Life-USTC/CLI/internal/cmd/schedule"
	"github.com/Life-USTC/CLI/internal/cmd/section"
	"github.com/Life-USTC/CLI/internal/cmd/semester"
	"github.com/Life-USTC/CLI/internal/cmd/teacher"
)

func NewCmdCatalog() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog <command>",
		Short: "Browse public campus data",
		Args:  cobra.NoArgs,
	}
	cmd.AddCommand(
		metadata.NewCmdMetadata(),
		semester.NewCmdSemester(),
		course.NewCmdCourse(),
		section.NewCmdSection(),
		teacher.NewCmdTeacher(),
		schedule.NewCmdSchedule(),
		bus.NewCmdBus(),
		link.NewCmdCatalogLink(),
	)
	return cmd
}
