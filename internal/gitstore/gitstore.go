// Package gitstore persists benchmark runs to an orphan git branch (default
// "cubit-state") and reads them back, without ever checking that branch out.
//
// Records live at runs/<commit-sha>.json, with an index.json manifest listing
// every recorded run for the dashboard and for resolving "the latest baseline".
// Only default-branch runs are persisted (main-only history), so the branch is
// a small, append-mostly ledger — a good fit for a git branch rather than a
// database.
//
// Writes use git plumbing (hash-object → update-index against a temp index →
// write-tree → commit-tree → update-ref with compare-and-swap), so they never
// touch the caller's working tree and are safe to run mid-build in CI.
package gitstore

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strings"

	"wardnet/cubit/internal/model"
)

// DefaultBranch is the orphan branch runs are stored on.
const DefaultBranch = "cubit-state"

const (
	runsDir   = "runs"
	indexPath = "index.json"
)

// Store reads and writes runs on an orphan branch within a git repository.
type Store struct {
	RepoDir string // path to the git repository (worktree or bare)
	Branch  string // orphan branch name; empty means DefaultBranch
}

// New returns a Store for repoDir. An empty branch defaults to DefaultBranch.
func New(repoDir, branch string) *Store {
	if branch == "" {
		branch = DefaultBranch
	}
	return &Store{RepoDir: repoDir, Branch: branch}
}

// RunMeta is one entry in the index manifest.
type RunMeta struct {
	Commit    string `json:"commit"`
	Branch    string `json:"branch"`
	Timestamp string `json:"timestamp"`
}

// Index returns the manifest of recorded runs, sorted oldest-first by
// timestamp. It returns an empty slice (no error) when the branch or manifest
// does not exist yet.
func (s *Store) Index() ([]RunMeta, error) {
	ref, ok, err := s.resolveRef()
	if err != nil || !ok {
		return nil, err
	}
	data, ok, err := s.showFile(ref, indexPath)
	if err != nil || !ok {
		return nil, err
	}
	var metas []RunMeta
	if err := json.Unmarshal(data, &metas); err != nil {
		return nil, fmt.Errorf("parse %s: %w", indexPath, err)
	}
	sort.SliceStable(metas, func(i, j int) bool { return metas[i].Timestamp < metas[j].Timestamp })
	return metas, nil
}

// Read returns the run recorded for commit sha. The bool is false (with no
// error) when the branch or that record does not exist.
func (s *Store) Read(sha string) (model.Run, bool, error) {
	ref, ok, err := s.resolveRef()
	if err != nil || !ok {
		return model.Run{}, false, err
	}
	data, ok, err := s.showFile(ref, runPath(sha))
	if err != nil || !ok {
		return model.Run{}, false, err
	}
	var run model.Run
	if err := json.Unmarshal(data, &run); err != nil {
		return model.Run{}, false, fmt.Errorf("parse %s: %w", runPath(sha), err)
	}
	return run, true, nil
}

// Latest returns the most recently recorded run (by index timestamp), matching
// only the given branch when branchFilter is non-empty. The bool is false (no
// error) when nothing has been recorded yet.
func (s *Store) Latest(branchFilter string) (model.Run, bool, error) {
	metas, err := s.Index()
	if err != nil {
		return model.Run{}, false, err
	}
	for i := len(metas) - 1; i >= 0; i-- {
		if branchFilter != "" && metas[i].Branch != branchFilter {
			continue
		}
		run, ok, err := s.Read(metas[i].Commit)
		if err != nil {
			return model.Run{}, false, err
		}
		if ok {
			return run, true, nil
		}
		// The manifest lists this commit but its run blob is gone — fall back to
		// the next-older matching run rather than reporting "no baseline".
	}
	return model.Run{}, false, nil
}

// Append records run at runs/<commit>.json and updates index.json, committing
// to the orphan branch via plumbing. It refuses a run with no commit SHA (the
// records are SHA-keyed). Concurrent writers are guarded by a compare-and-swap
// on the branch ref: a racing update makes this fail rather than clobber.
func (s *Store) Append(run model.Run) error {
	if strings.TrimSpace(run.Commit) == "" {
		return fmt.Errorf("run has no commit SHA — records are keyed by commit")
	}

	// The commit the new record builds on: the local branch if present, else the
	// remote-tracking ref (the CI fetched-not-checked-out case). Seeding the
	// index AND reading the manifest from this SAME base keeps the persisted tree
	// and the manifest consistent, and setting the commit parent to it makes the
	// new local branch a fast-forward descendant of origin — so the later push is
	// not rejected. (Reading the manifest via a separately-resolved ref was the
	// bug: with only origin present, the tree was seeded empty while the manifest
	// still listed — and thus orphaned — every historical run.)
	base, hasBase, err := s.baseCommit()
	if err != nil {
		return err
	}

	// Stage into a throwaway index seeded from the base tree, so existing records
	// survive the commit. The index path must NOT pre-exist as an empty file —
	// git rejects a zero-byte index — so we hand git a fresh path inside a temp
	// dir and let it create the index itself.
	tmpDir, err := os.MkdirTemp("", "cubit-index-")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(tmpDir) }()
	env := append(os.Environ(), "GIT_INDEX_FILE="+filepath.Join(tmpDir, "index"))

	if hasBase {
		if _, err := s.gitEnv(env, nil, "read-tree", base+"^{tree}"); err != nil {
			return err
		}
	}

	// Merge the new run into the manifest read from the exact base we seeded the
	// tree from — never a separately-resolved ref — so history and tree agree.
	var metas []RunMeta
	if hasBase {
		if data, ok, err := s.showFile(base, indexPath); err != nil {
			return err
		} else if ok {
			if err := json.Unmarshal(data, &metas); err != nil {
				return fmt.Errorf("parse %s at %s: %w", indexPath, short(base), err)
			}
		}
	}
	metas = upsertMeta(metas, RunMeta{Commit: run.Commit, Branch: run.Branch, Timestamp: run.Timestamp})

	runJSON, err := marshal(run)
	if err != nil {
		return err
	}
	indexJSON, err := marshal(metas)
	if err != nil {
		return err
	}

	if err := s.stage(env, runPath(run.Commit), runJSON); err != nil {
		return err
	}
	if err := s.stage(env, indexPath, indexJSON); err != nil {
		return err
	}

	treeOut, err := s.gitEnv(env, nil, "write-tree")
	if err != nil {
		return err
	}
	tree := strings.TrimSpace(treeOut)

	msg := fmt.Sprintf("record %s (%s)", short(run.Commit), run.Branch)
	commitArgs := []string{"commit-tree", tree, "-m", msg}
	if hasBase {
		commitArgs = append(commitArgs, "-p", base)
	}
	commitOut, err := s.gitEnv(env, nil, commitArgs...)
	if err != nil {
		return err
	}
	commit := strings.TrimSpace(commitOut)

	// Compare-and-swap the LOCAL branch. The old value is the local ref's current
	// commit, or the empty string ("must not exist yet") when we are creating it
	// — never the remote-tracking ref, which cannot be CAS'd. When base came from
	// origin, the new commit's parent is origin's tip, so this create yields a
	// clean fast-forward descendant.
	localTip, hasLocal, err := s.tip()
	if err != nil {
		return err
	}
	ref := "refs/heads/" + s.Branch
	if hasLocal {
		_, err = s.git(nil, "update-ref", ref, commit, localTip)
	} else {
		_, err = s.git(nil, "update-ref", ref, commit, "")
	}
	return err
}

// stage hashes content as a blob and adds it to the temp index at repoPath.
func (s *Store) stage(env []string, repoPath string, content []byte) error {
	blobOut, err := s.gitEnv(env, content, "hash-object", "-w", "--stdin")
	if err != nil {
		return err
	}
	blob := strings.TrimSpace(blobOut)
	_, err = s.gitEnv(env, nil, "update-index", "--add", "--cacheinfo", fmt.Sprintf("100644,%s,%s", blob, repoPath))
	return err
}

// resolveRef returns a readable ref for the branch, preferring the local branch
// and falling back to origin/<branch> (the CI case, where the branch is
// fetched but not checked out). ok is false when neither exists.
func (s *Store) resolveRef() (string, bool, error) {
	for _, ref := range []string{"refs/heads/" + s.Branch, "refs/remotes/origin/" + s.Branch} {
		out, err := s.git(nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
		if err == nil && strings.TrimSpace(out) != "" {
			return ref, true, nil
		}
	}
	return "", false, nil
}

// tip returns the local branch's commit for compare-and-swap on write. It only
// considers the local branch (a CAS against a remote-tracking ref would be
// meaningless), so first-write-in-CI creates the branch locally then pushes.
func (s *Store) tip() (string, bool, error) {
	out, err := s.git(nil, "rev-parse", "--verify", "--quiet", "refs/heads/"+s.Branch+"^{commit}")
	if err != nil {
		// rev-parse --quiet exits non-zero when the ref is absent; treat as "no tip".
		return "", false, nil
	}
	sha := strings.TrimSpace(out)
	if sha == "" {
		return "", false, nil
	}
	return sha, true, nil
}

// baseCommit resolves the commit a new record should build on: the local branch
// if it exists, else origin/<branch> (the CI fetched-not-checked-out case). ok
// is false when neither exists (the branch is being created from scratch).
func (s *Store) baseCommit() (string, bool, error) {
	for _, ref := range []string{"refs/heads/" + s.Branch, "refs/remotes/origin/" + s.Branch} {
		out, err := s.git(nil, "rev-parse", "--verify", "--quiet", ref+"^{commit}")
		if err == nil {
			if sha := strings.TrimSpace(out); sha != "" {
				return sha, true, nil
			}
		}
	}
	return "", false, nil
}

// History returns every recorded run matching branchFilter (empty = all),
// oldest-first, resolving the state ref ONCE. Entries whose run blob is missing
// are skipped. It replaces one Read-per-entry (each of which re-resolved the
// ref) with a single resolution plus one git show per run.
func (s *Store) History(branchFilter string) ([]model.Run, error) {
	ref, ok, err := s.resolveRef()
	if err != nil || !ok {
		return nil, err
	}
	data, ok, err := s.showFile(ref, indexPath)
	if err != nil || !ok {
		return nil, err
	}
	var metas []RunMeta
	if err := json.Unmarshal(data, &metas); err != nil {
		return nil, fmt.Errorf("parse %s: %w", indexPath, err)
	}
	sort.SliceStable(metas, func(i, j int) bool { return metas[i].Timestamp < metas[j].Timestamp })

	var runs []model.Run
	for _, m := range metas {
		if branchFilter != "" && m.Branch != branchFilter {
			continue
		}
		blob, ok, err := s.showFile(ref, runPath(m.Commit))
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		var run model.Run
		if err := json.Unmarshal(blob, &run); err != nil {
			return nil, fmt.Errorf("parse %s: %w", runPath(m.Commit), err)
		}
		runs = append(runs, run)
	}
	return runs, nil
}

// showFile returns the bytes of repoPath at ref. ok is false (no error) when
// the path does not exist at that ref.
func (s *Store) showFile(ref, repoPath string) ([]byte, bool, error) {
	// The binary is the literal "git" and the arguments are passed as a slice,
	// so no shell parses them and a ref or path cannot become a second command.
	// #nosec G204 -- fixed binary, argv passed directly, no shell involved.
	cmd := exec.Command("git", "-C", s.RepoDir, "show", ref+":"+repoPath)
	// Force a stable C locale: the missing-path detection below matches git's
	// English error text, which localized output (LANG/LC_ALL) would otherwise
	// break — turning an expected "not found" into a hard error.
	cmd.Env = append(os.Environ(), "LC_ALL=C", "LANG=C")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		// A missing path is expected (empty branch / first run) — not an error.
		if strings.Contains(stderr.String(), "does not exist") ||
			strings.Contains(stderr.String(), "exists on disk, but not in") ||
			strings.Contains(stderr.String(), "fatal: path") {
			return nil, false, nil
		}
		return nil, false, fmt.Errorf("git show %s:%s: %s", ref, repoPath, strings.TrimSpace(stderr.String()))
	}
	return stdout.Bytes(), true, nil
}

func (s *Store) git(stdin []byte, args ...string) (string, error) {
	return s.gitEnv(nil, stdin, args...)
}

func (s *Store) gitEnv(env []string, stdin []byte, args ...string) (string, error) {
	// Fixed binary, argv as a slice, no shell. Running git is what a git-backed
	// store does.
	// #nosec G204 -- fixed binary, argv passed directly, no shell involved.
	cmd := exec.Command("git", append([]string{"-C", s.RepoDir}, args...)...)
	base := env
	if base == nil {
		base = os.Environ()
	}
	// Force a stable C locale (see showFile) on every git invocation, including
	// those carrying a GIT_INDEX_FILE. Later duplicate keys win, so this overrides
	// any inherited LANG/LC_ALL.
	cmd.Env = append(base, "LC_ALL=C", "LANG=C")
	if stdin != nil {
		cmd.Stdin = bytes.NewReader(stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("git %s: %s", strings.Join(args, " "), strings.TrimSpace(stderr.String()))
	}
	return stdout.String(), nil
}

func upsertMeta(metas []RunMeta, m RunMeta) []RunMeta {
	for i := range metas {
		if metas[i].Commit == m.Commit {
			metas[i] = m
			return metas
		}
	}
	return append(metas, m)
}

func marshal(v any) ([]byte, error) {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

func runPath(sha string) string { return path.Join(runsDir, sha+".json") }

func short(sha string) string {
	if len(sha) > 8 {
		return sha[:8]
	}
	return sha
}
