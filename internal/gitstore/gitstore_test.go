package gitstore

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"wardnet/cubit/internal/model"
)

func TestAppendReadLatest(t *testing.T) {
	repo := initRepo(t)
	s := New(repo, "")

	// Empty store: no index, no latest, no record.
	if metas, err := s.Index(); err != nil || len(metas) != 0 {
		t.Fatalf("empty Index() = %v, %v; want empty, nil", metas, err)
	}
	if _, ok, err := s.Latest(""); err != nil || ok {
		t.Fatalf("empty Latest() ok=%v err=%v; want false, nil", ok, err)
	}

	run1 := model.Run{
		Schema: 1, Tool: "criterion", Commit: "aaaaaaaaaaaaaaaa", Branch: "main",
		Timestamp: "2026-07-18T00:00:00Z",
		Measurements: []model.Measurement{
			{ID: "dns_cache/get", Unit: "ns", Value: 45.2},
		},
	}
	if err := s.Append(run1); err != nil {
		t.Fatalf("Append(run1): %v", err)
	}

	got, ok, err := s.Read("aaaaaaaaaaaaaaaa")
	if err != nil || !ok {
		t.Fatalf("Read(run1) ok=%v err=%v", ok, err)
	}
	if len(got.Measurements) != 1 || got.Measurements[0].Value != 45.2 {
		t.Fatalf("Read(run1) = %+v; want the recorded measurement", got.Measurements)
	}

	// A newer run on main supersedes as "latest"; branch filter is honored.
	run2 := run1
	run2.Commit = "bbbbbbbbbbbbbbbb"
	run2.Timestamp = "2026-07-19T00:00:00Z"
	run2.Measurements = []model.Measurement{{ID: "dns_cache/get", Unit: "ns", Value: 44.0}}
	if err := s.Append(run2); err != nil {
		t.Fatalf("Append(run2): %v", err)
	}

	metas, err := s.Index()
	if err != nil || len(metas) != 2 {
		t.Fatalf("Index() = %v (%d), %v; want 2 entries", metas, len(metas), err)
	}
	if metas[0].Commit != run1.Commit || metas[1].Commit != run2.Commit {
		t.Fatalf("Index() not sorted oldest-first: %+v", metas)
	}

	latest, ok, err := s.Latest("main")
	if err != nil || !ok || latest.Commit != run2.Commit {
		t.Fatalf("Latest(main) = %+v ok=%v err=%v; want run2", latest, ok, err)
	}
	if _, ok, _ := s.Latest("nonexistent-branch"); ok {
		t.Fatalf("Latest(nonexistent-branch) should be not-found")
	}

	// The orphan-branch writes must never touch the working tree.
	if out := gitOut(t, repo, "status", "--porcelain"); out != "" {
		t.Fatalf("working tree dirtied by Append: %q", out)
	}
	if _, err := os.Stat(filepath.Join(repo, "runs")); !os.IsNotExist(err) {
		t.Fatalf("runs/ leaked into the working tree")
	}
}

func TestAppendRejectsEmptyCommit(t *testing.T) {
	s := New(initRepo(t), "")
	if err := s.Append(model.Run{Branch: "main"}); err == nil {
		t.Fatal("Append with empty commit should fail")
	}
}

// initRepo creates a git repo with one commit on the default branch, so the
// orphan cubit-state branch has a repository to live in.
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, "README"), []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "-m", "init")
	return dir
}

func gitOut(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
