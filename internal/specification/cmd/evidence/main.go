package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/Life-USTC/CLI/internal/specification"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	out := flag.String("out", "artifacts/specifications", "native evidence directory")
	requireClean := flag.Bool("require-clean", false, "require commit-clean execution evidence")
	flag.Parse()
	root, err := specification.Root()
	if err != nil {
		return err
	}
	repo, err := specification.Load(root)
	if err != nil {
		return err
	}
	if *requireClean && !repo.Provenance.WorkingTreeClean {
		return fmt.Errorf("CI native evidence requires a clean committed checkout")
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		return err
	}
	raw, err := os.Create(filepath.Join(*out, "native-go.jsonl"))
	if err != nil {
		return err
	}
	defer func() { _ = raw.Close() }()
	nonce := make([]byte, 16)
	if _, err := rand.Read(nonce); err != nil {
		return err
	}
	runID := hex.EncodeToString(nonce)
	cmd := exec.Command("go", "test", "-race", "-count=1", "-json", "./...")
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "SPEC_RUN_ID="+runID)
	cmd.Stdout = raw
	cmd.Stderr = os.Stderr
	testErr := cmd.Run()
	current, err := specification.Load(root)
	if err != nil {
		return err
	}
	if current.Provenance != repo.Provenance {
		return fmt.Errorf("repository changed during native execution")
	}
	if _, err := raw.Seek(0, 0); err != nil {
		return err
	}
	report, gateErr := specification.Collect(repo, raw, runID)
	encoded, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(*out, "coverage.json"), append(encoded, '\n'), 0o644); err != nil {
		return err
	}
	if testErr != nil {
		return fmt.Errorf("native Go suite failed: %w (evidence: %v)", testErr, gateErr)
	}
	if gateErr != nil {
		return gateErr
	}
	fmt.Printf("Specifications: %d canonical requirements, %d/%d native cases passed; source and field consumption verified.\n", report.Requirements, report.Passed, report.Cases)
	return nil
}
