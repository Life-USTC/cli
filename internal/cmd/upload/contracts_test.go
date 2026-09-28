package upload

import (
	"context"
	"github.com/Life-USTC/CLI/internal/specification"
	"github.com/spf13/cobra"
	"os"
	"testing"
)

func TestSpecUploadCompleteList(t *testing.T) {
	t.Run("upload.cli-complete-list", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func TestSpecUploadPicker(t *testing.T) {
	t.Run("upload.cli-picker-complete-input", func(t *testing.T) { specification.Run(t, acceptanceAdapter) })
}
func acceptanceAdapter(t *testing.T, in specification.Input) specification.Observation {
	if in.Action == "upload" {
		return specification.Execute(t, NewCmdUpload(), in)
	}
	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	cmd.PersistentFlags().String("server", in.ServerURL, "")
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	_, _ = writer.WriteString(in.Value + "\n")
	_ = writer.Close()
	old := os.Stdin
	os.Stdin = reader
	defer func() { os.Stdin = old; _ = reader.Close() }()
	var row map[string]any
	result := specification.Capture(t, in.Format, func() error { var err error; row, err = promptUploadPick(cmd, "Select upload"); return err })
	if row != nil {
		result.Data = []map[string]any{row}
	}
	return result
}
