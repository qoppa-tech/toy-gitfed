package e2e

import (
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (s *E2ESuite) gitInfoRefsUploadPackAdvertisement() {
	repoName := "seeded-advertisement.git"
	s.seedBareRepoWithInitialCommit(repoName)

	status, body := s.get("/"+repoName+"/info/refs?service=git-upload-pack", nil, s.client)
	s.Require().Equal(http.StatusOK, status)

	req, err := http.NewRequest(http.MethodGet, s.baseURL+"/"+repoName+"/info/refs?service=git-upload-pack", nil)
	s.Require().NoError(err)
	resp, err := s.client.Do(req)
	s.Require().NoError(err)
	defer resp.Body.Close()

	s.Require().Equal("application/x-git-upload-pack-advertisement", resp.Header.Get("Content-Type"))
	s.Require().NotEmpty(body)
	s.Require().Contains(string(body), "git-upload-pack")
	s.Require().Contains(string(body), "refs/heads/main")
}

func (s *E2ESuite) gitCloneCommitPushCycle() {
	repoName := "seeded-push-cycle.git"
	s.seedBareRepoWithInitialCommit(repoName)

	remoteURL := s.baseURL + "/" + repoName
	cloneDir := filepath.Join(s.T().TempDir(), "clone")
	s.runGit(s.reposDir, "clone", remoteURL, cloneDir)

	readmePath := filepath.Join(cloneDir, "README.md")
	current, err := os.ReadFile(readmePath)
	s.Require().NoError(err)

	appendLine := "push verification line"
	updated := strings.TrimRight(string(current), "\n") + "\n" + appendLine + "\n"
	err = os.WriteFile(readmePath, []byte(updated), 0644)
	s.Require().NoError(err)

	s.runGit(cloneDir, "add", "README.md")
	s.runGit(cloneDir, "-c", "user.name=E2E Test", "-c", "user.email=e2e@example.com", "commit", "-m", "e2e: push via smart http")
	s.runGit(cloneDir, "push", "origin", "HEAD")

	verifyDir := filepath.Join(s.T().TempDir(), "verify")
	s.runGit(s.reposDir, "clone", remoteURL, verifyDir)

	verifyReadme, err := os.ReadFile(filepath.Join(verifyDir, "README.md"))
	s.Require().NoError(err)
	s.Require().Contains(string(verifyReadme), appendLine)
}

func (s *E2ESuite) seedBareRepoWithInitialCommit(repoName string) {
	bareRepoPath := filepath.Join(s.reposDir, repoName)
	s.runGit(s.reposDir, "init", "--bare", bareRepoPath)
	s.runGit(bareRepoPath, "symbolic-ref", "HEAD", "refs/heads/main")

	workDir := s.T().TempDir()
	s.runGit(workDir, "init")

	readmePath := filepath.Join(workDir, "README.md")
	err := os.WriteFile(readmePath, []byte("seeded repository\n"), 0644)
	s.Require().NoError(err)

	s.runGit(workDir, "add", "README.md")
	s.runGit(workDir, "-c", "user.name=E2E Seed", "-c", "user.email=e2e-seed@example.com", "commit", "-m", "seed repository")
	s.runGit(workDir, "branch", "-M", "main")
	s.runGit(workDir, "remote", "add", "origin", bareRepoPath)
	s.runGit(workDir, "push", "origin", "main")
}

func (s *E2ESuite) runGit(dir string, args ...string) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir

	out, err := cmd.CombinedOutput()
	s.Require().NoError(err, "git %s failed: %s", strings.Join(args, " "), string(out))
}

func (s *E2ESuite) TestGit() {
	s.Run("InfoRefsUploadPackAdvertisement", s.gitInfoRefsUploadPackAdvertisement)
	s.Run("CloneCommitPushCycle", s.gitCloneCommitPushCycle)
}
