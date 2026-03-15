package cli_test

import (
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestUsageShowsFinalCommandNames(t *testing.T) {
	cmd := exec.Command("go", "run", "./cmd/echo-fade-memory")
	cmd.Dir = filepath.Join("..", "..", "..")

	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("go run without args succeeded, want usage failure")
	}

	output := string(out)
	if !strings.Contains(output, "echo-fade-memory remember <content>") {
		t.Fatalf("usage missing remember command:\n%s", output)
	}
	if strings.Contains(output, "echo-fade-memory store <content>") {
		t.Fatalf("usage still contains legacy store command:\n%s", output)
	}
}
