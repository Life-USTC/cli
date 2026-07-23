package community

import (
	"github.com/spf13/cobra"

	"github.com/Life-USTC/CLI/internal/cmd/comment"
	"github.com/Life-USTC/CLI/internal/cmd/description"
	"github.com/Life-USTC/CLI/internal/cmd/homework"
)

func NewCmdCommunity() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "community <command>",
		Short: "Manage shared campus content",
		Args:  cobra.NoArgs,
	}
	sectionHomework := homework.NewCmdSectionHomework()
	sectionHomework.Use = "section-homework <command>"
	cmd.AddCommand(
		comment.NewCmdComment(),
		description.NewCmdDescription(),
		sectionHomework,
	)
	return cmd
}
