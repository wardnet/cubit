package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The staged file has to come out executable. cubit replaces its own binary
// with it, and a downloaded update that cannot be executed leaves the user
// with a cubit that no longer runs — a failure that only shows up after the
// swap, when the old binary is already gone.
func TestDownloadBinaryStagesAnExecutable(t *testing.T) {
	const payload = "#!/bin/sh\necho cubit\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "cubit")
	if err := downloadBinary(context.Background(), srv.URL, dst); err != nil {
		t.Fatalf("downloadBinary() error = %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read staged binary: %v", err)
	}
	if string(got) != payload {
		t.Errorf("staged %q, want %q", got, payload)
	}

	info, err := os.Stat(dst)
	if err != nil {
		t.Fatalf("stat staged binary: %v", err)
	}
	// Windows has no execute bit; everywhere cubit ships, it must be set.
	if runtime.GOOS != "windows" && info.Mode().Perm()&0o111 == 0 {
		t.Errorf("staged binary mode = %v, want the execute bit set", info.Mode().Perm())
	}
}

// A missing asset must fail loudly, naming the platform. Writing the 404 body
// to disk and swapping it in would replace cubit with an error page.
func TestDownloadBinaryRejectsANon200(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	dst := filepath.Join(t.TempDir(), "cubit")
	err := downloadBinary(context.Background(), srv.URL, dst)
	if err == nil {
		t.Fatal("downloadBinary() = nil, want an error for a 404")
	}
	if !strings.Contains(err.Error(), runtime.GOOS) || !strings.Contains(err.Error(), runtime.GOARCH) {
		t.Errorf("error %q does not name the platform the asset was missing for", err)
	}
	if _, err := os.Stat(dst); !os.IsNotExist(err) {
		t.Error("a failed download left a file behind, which a later swap could install")
	}
}
