package client

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"veil/internal/config"
)

func withHosts(t *testing.T, content string) string {
	t.Helper()
	old := hostsFile
	p := filepath.Join(t.TempDir(), "hosts")
	if content != "\x00" {
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	hostsFile = p
	t.Cleanup(func() { hostsFile = old })
	return p
}

func readFile(t *testing.T, p string) string {
	t.Helper()
	var b []byte
	var err error
	for i := 0; i < 20; i++ { // антивирус на Windows может кратко держать только что записанный файл
		if b, err = os.ReadFile(p); err == nil {
			return string(b)
		}
		time.Sleep(25 * time.Millisecond)
	}
	t.Fatal(err)
	return ""
}

func TestRewriteHostsAdd(t *testing.T) {
	p := withHosts(t, "127.0.0.1 localhost\n")
	if err := rewriteHosts("10.0.0.5"); err != nil {
		t.Fatal(err)
	}
	want := "127.0.0.1 localhost\n10.0.0.5 admin " + hostsMarker + "\n"
	if got := readFile(t, p); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
}

func TestRewriteHostsReplaceKeepsOthers(t *testing.T) {
	p := withHosts(t, "127.0.0.1 localhost\n1.1.1.1 admin "+hostsMarker+"\n::1 localhost\n")
	if err := rewriteHosts("2.2.2.2"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, p)
	if strings.Count(got, hostsMarker) != 1 || !strings.Contains(got, "2.2.2.2 admin") || strings.Contains(got, "1.1.1.1") {
		t.Fatalf("bad replace: %q", got)
	}
	if !strings.Contains(got, "127.0.0.1 localhost\n") || !strings.Contains(got, "::1 localhost\n") {
		t.Fatalf("other lines lost: %q", got)
	}
}

func TestRewriteHostsRemove(t *testing.T) {
	p := withHosts(t, "127.0.0.1 localhost\n3.3.3.3 admin "+hostsMarker+"\n")
	if err := rewriteHosts(""); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, p); got != "127.0.0.1 localhost\n" {
		t.Fatalf("got %q", got)
	}
}

func TestRewriteHostsCRLFPreserved(t *testing.T) {
	p := withHosts(t, "127.0.0.1 localhost\r\n::1 localhost\r\n")
	if err := rewriteHosts("4.4.4.4"); err != nil {
		t.Fatal(err)
	}
	got := readFile(t, p)
	want := "127.0.0.1 localhost\r\n::1 localhost\r\n4.4.4.4 admin " + hostsMarker + "\r\n"
	if got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if err := rewriteHosts(""); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, p); got != "127.0.0.1 localhost\r\n::1 localhost\r\n" {
		t.Fatalf("after remove: %q", got)
	}
}

func TestRewriteHostsLFPreserved(t *testing.T) {
	p := withHosts(t, "a\nb\n")
	_ = rewriteHosts("5.5.5.5")
	if strings.Contains(readFile(t, p), "\r") {
		t.Fatal("CR appeared in an LF file")
	}
}

func TestRewriteHostsIdempotent(t *testing.T) {
	p := withHosts(t, "127.0.0.1 localhost\n")
	_ = rewriteHosts("6.6.6.6")
	first := readFile(t, p)
	_ = rewriteHosts("6.6.6.6")
	if readFile(t, p) != first {
		t.Fatal("repeat call changed the file")
	}
}

func TestRewriteHostsNoFile(t *testing.T) {
	withHosts(t, "\x00")
	if err := rewriteHosts("1.2.3.4"); err == nil {
		t.Fatal("expected error for a missing hosts file")
	}
}

func TestRewriteHostsReadOnly(t *testing.T) {
	p := withHosts(t, "127.0.0.1 localhost\n")
	if err := os.Chmod(p, 0o444); err != nil {
		t.Skip("chmod is not supported")
	}
	t.Cleanup(func() { _ = os.Chmod(p, 0o644) })
	if f, err := os.OpenFile(p, os.O_WRONLY, 0); err == nil {
		f.Close()
		t.Skip("file stays writable (running as admin)")
	}
	if err := rewriteHosts("1.2.3.4"); err == nil {
		t.Fatal("expected a write error")
	}
}

func TestInstallAdminAliasStatuses(t *testing.T) {
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	prof := &config.Profile{AdminURL: "https://10.1.2.3:8443"}

	p := withHosts(t, "127.0.0.1 localhost\n")
	u := &undoStack{}
	if got := installAdminAlias(prof, u, log); got != AdminHostsOK {
		t.Fatalf("status %q", got)
	}
	if !strings.Contains(readFile(t, p), "10.1.2.3 admin") {
		t.Fatal("alias not written")
	}
	u.run()
	if strings.Contains(readFile(t, p), hostsMarker) {
		t.Fatal("alias not removed by undo")
	}

	withHosts(t, "\x00")
	u = &undoStack{}
	if got := installAdminAlias(prof, u, log); got != AdminHostsFailed {
		t.Fatalf("no access: status %q, want %q", got, AdminHostsFailed)
	}
	if len(u.fns) != 0 {
		t.Fatal("undo registered after a failure")
	}

	if got := installAdminAlias(&config.Profile{AdminURL: "https://example.com"}, &undoStack{}, log); got != "" {
		t.Fatalf("hostname admin url: %q", got)
	}
}
