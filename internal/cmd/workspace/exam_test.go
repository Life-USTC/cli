package workspace

import (
	"github.com/Life-USTC/CLI/internal/specification"
	"testing"
)

func TestSpecExamReadsEveryPageAndPreservesTotal(t *testing.T) {
	t.Run("exam.cli-complete-list", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecExamExplicitPaginationAndFilters(t *testing.T) {
	t.Run("exam.cli-explicit-pagination", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecExamLaterPageFailureDoesNotPrintPartialSuccess(t *testing.T) {
	t.Run("exam.cli-page-failure", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecExamRejectsInvalidExplicitPaginationAndSemester(t *testing.T) {
	t.Run("exam.cli-positive-input", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecExamSemesterDisplay(t *testing.T) {
	t.Run("exam.cli-semester-display", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func acceptanceAdapter(t *testing.T, in specification.Input) specification.Observation {
	return specification.Execute(t, newCmdExam(), in)
}
