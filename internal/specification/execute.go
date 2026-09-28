package specification

import (
	"io"
	"os"
	"testing"

	"github.com/Life-USTC/CLI/internal/output"
	"github.com/spf13/cobra"
)

// Execute captures the actual production command's stdout. It does not inspect
// expectations; all assertions remain in Run's closed family consumers.
func Execute(t *testing.T, cmd *cobra.Command, in Input) Observation {
	t.Helper()
	if cmd.PersistentFlags().Lookup("server") == nil {
		cmd.PersistentFlags().String("server", in.ServerURL, "")
	}
	cmd.SilenceErrors, cmd.SilenceUsage = true, true
	cmd.SetArgs(in.Arguments)
	return Capture(t, in.Format, cmd.Execute)
}
func Capture(t *testing.T, format string, run func() error) Observation {
	t.Helper()
	oldStdout, oldOutput := os.Stdout, output.Current
	output.Current = &output.Opts{Format: format, NoColor: true}
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = writer
	done := make(chan string)
	go func() { data, _ := io.ReadAll(reader); done <- string(data) }()
	err = run()
	_ = writer.Close()
	os.Stdout = oldStdout
	output.Current = oldOutput
	result := <-done
	_ = reader.Close()
	return Observation{Output: result, Err: err}
}
