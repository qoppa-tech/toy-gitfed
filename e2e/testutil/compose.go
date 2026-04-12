package testutil

import (
	"fmt"
	"os/exec"
)

const composeFile = "docker-compose.test.yaml"

func ComposeUp(repoRoot string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "up", "-d")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose up: %w: %s", err, string(out))
	}
	return nil
}

func ComposeDown(repoRoot string) error {
	cmd := exec.Command("docker", "compose", "-f", composeFile, "down", "-v")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("compose down: %w: %s", err, string(out))
	}
	return nil
}
