package gui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
)

func TestParseSums(t *testing.T) {
	h := strings.Repeat("ab", 32)
	text := h + " *Veil-0.7.1-windows-setup.exe\n" + strings.Repeat("cd", 32) + " *other\n"
	if got := parseSums(text, "Veil-0.7.1-windows-setup.exe"); got != h {
		t.Fatalf("сумма: %q", got)
	}
	if parseSums(text, "нет-такого") != "" || parseSums("мусор", "x") != "" {
		t.Fatal("для отсутствующего файла сумма должна быть пустой")
	}
}

func TestDownloadAssetVerifies(t *testing.T) {
	body := []byte("содержимое установщика")
	sum := sha256.Sum256(body)
	good := hex.EncodeToString(sum[:]) + " *Veil-0.7.1-windows-setup.exe\n"
	bad := strings.Repeat("0", 64) + " *Veil-0.7.1-windows-setup.exe\n"
	mux := http.NewServeMux()
	mux.HandleFunc("/ok/sums", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(good)) })
	mux.HandleFunc("/bad/sums", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte(bad)) })
	mux.HandleFunc("/file", func(w http.ResponseWriter, _ *http.Request) { w.Write(body) })
	srv := httptest.NewServer(mux)
	defer srv.Close()
	old := assetPrefix
	assetPrefix = srv.URL
	defer func() { assetPrefix = old }()

	rel := func(sums string) *release {
		return &release{Tag: "v0.7.1", Assets: map[string]string{"Veil-0.7.1-windows-setup.exe": srv.URL + "/file", "SHA256SUMS.txt": srv.URL + sums}}
	}
	var last int64
	path, err := downloadAsset(context.Background(), rel("/ok/sums"), "Veil-0.7.1-windows-setup.exe", func(done, _ int64) { last = done })
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(path)
	if b, _ := os.ReadFile(path); string(b) != string(body) || last != int64(len(body)) {
		t.Fatalf("файл или ход загрузки: %q %d", b, last)
	}
	if _, err := downloadAsset(context.Background(), rel("/bad/sums"), "Veil-0.7.1-windows-setup.exe", nil); err == nil || !strings.Contains(err.Error(), "суммa не совпала") && !strings.Contains(err.Error(), "не совпала") && !strings.Contains(err.Error(), "mismatch") {
		t.Fatalf("подменённый файл должен отклоняться: %v", err)
	}
	noSums := &release{Assets: map[string]string{"Veil-0.7.1-windows-setup.exe": srv.URL + "/file"}}
	if _, err := downloadAsset(context.Background(), noSums, "Veil-0.7.1-windows-setup.exe", nil); err == nil {
		t.Fatal("без SHA256SUMS.txt ставить нельзя")
	}
	foreign := &release{Assets: map[string]string{"x": "https://evil.example/x", "SHA256SUMS.txt": srv.URL + "/ok/sums"}}
	if _, err := downloadAsset(context.Background(), foreign, "x", nil); err == nil {
		t.Fatal("чужой адрес загрузки должен отклоняться")
	}
}

func TestInstallerAsset(t *testing.T) {
	if installerAsset("v0.7.1") != "" && !strings.HasSuffix(installerAsset("v0.7.1"), "-windows-setup.exe") {
		t.Fatalf("имя установщика: %q", installerAsset("v0.7.1"))
	}
}
