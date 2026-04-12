package testutil

import (
	"bytes"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"time"
)

func StartAPI(repoRoot string, reposDir string) (*exec.Cmd, string, error) {
	const (
		candidateCount   = 12
		candidateStart   = 18080
		candidateSpan    = 2000
		startupProbeWait = 4 * time.Second
	)

	basePort := candidateStart + int(time.Now().UnixNano()%candidateSpan)
	baseEnv := append(os.Environ(),
		"DB_HOST=127.0.0.1",
		"DB_PORT="+PostgresTestHostPort(),
		"DB_USER=gitfed_test",
		"DB_PASSWORD=gitfed_test",
		"DB_NAME=gitfed_test",
		"DB_SSLMODE=disable",
		"REDIS_HOST=127.0.0.1",
		"REDIS_PORT="+RedisTestHostPort(),
		"REDIS_PASSWORD=",
		"REDIS_DB=0",
		"SECURE_COOKIES=false",
	)

	var lastErr error
	for i := range candidateCount {
		port := candidateStart + ((basePort - candidateStart + i) % candidateSpan)
		httpAddr := net.JoinHostPort("127.0.0.1", strconv.Itoa(port))

		cmd := exec.Command("go", "run", "./cmd/http-api", reposDir)
		cmd.Dir = repoRoot
		cmd.Env = append(baseEnv, "HTTP_ADDR="+httpAddr)

		if err := cmd.Start(); err != nil {
			lastErr = fmt.Errorf("start api on %s: %w", httpAddr, err)
			continue
		}

		if err := WaitForHTTP(httpAddr, startupProbeWait); err == nil {
			return cmd, httpAddr, nil
		} else {
			lastErr = fmt.Errorf("api readiness on %s: %w", httpAddr, err)
			_ = cmd.Process.Kill()
			_, _ = cmd.Process.Wait()
		}
	}

	if lastErr == nil {
		lastErr = errors.New("no startup attempts made")
	}
	return nil, "", fmt.Errorf("start api after %d port attempts: %w", candidateCount, lastErr)
}

func StopAPI(cmd *exec.Cmd) error {
	if cmd == nil || cmd.Process == nil {
		return nil
	}

	if err := cmd.Process.Kill(); err != nil {
		if !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("kill api: %w", err)
		}
	}

	if _, err := cmd.Process.Wait(); err != nil {
		if !errors.Is(err, os.ErrProcessDone) {
			return fmt.Errorf("wait api process: %w", err)
		}
	}

	return nil
}

func WaitForHTTP(httpAddr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 1 * time.Second}
	url := "http://" + httpAddr + "/auth/register"

	for {
		req, err := http.NewRequest(http.MethodPost, url, bytes.NewBufferString("{"))
		if err != nil {
			return fmt.Errorf("build readiness request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusBadRequest {
				return nil
			}
		}

		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for api http readiness at %s", url)
		}
		time.Sleep(250 * time.Millisecond)
	}
}

func WaitForTCP(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		conn, err := net.DialTimeout("tcp", addr, 500*time.Millisecond)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timeout waiting for tcp %s", addr)
		}
		time.Sleep(250 * time.Millisecond)
	}
}
