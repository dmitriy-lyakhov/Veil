package gui

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// assetPrefix — файлы релиза скачиваются только из релизов этого репозитория (даже если API вернёт другой адрес).
var assetPrefix = repoURL + "/releases/download/"

// release — последний релиз в GitHub: тег, страница и файлы (имя → адрес загрузки).
type release struct {
	Tag    string
	HTML   string
	Assets map[string]string
}

// fetchLatestRelease возвращает последний релиз в GitHub.
func fetchLatestRelease(ctx context.Context) (*release, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releasesAPI, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "Veil-update-check")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New(L("no releases yet", "релизов пока нет"))
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf(L("GitHub responded %s", "GitHub ответил %s"), resp.Status)
	}
	var r struct {
		Tag    string `json:"tag_name"`
		HTML   string `json:"html_url"`
		Assets []struct {
			Name string `json:"name"`
			URL  string `json:"browser_download_url"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 4<<20)).Decode(&r); err != nil {
		return nil, err
	}
	rel := &release{Tag: r.Tag, HTML: r.HTML, Assets: map[string]string{}}
	for _, a := range r.Assets {
		rel.Assets[a.Name] = a.URL
	}
	return rel, nil
}

// installerAsset — имя файла установщика в релизе для этой системы; пусто — автоматической установки нет (откроется страница загрузки).
func installerAsset(tag string) string {
	if runtime.GOOS == "windows" {
		return "Veil-" + strings.TrimPrefix(tag, "v") + "-windows-setup.exe"
	}
	return ""
}

// canSelfInstall — приложение установлено установщиком (рядом с exe есть деинсталлятор): тогда новая версия ставится поверх.
// Портативная версия обновляется вручную: установщик превратил бы её в установленную.
func canSelfInstall() bool {
	if runtime.GOOS != "windows" {
		return false
	}
	exe, err := os.Executable()
	if err != nil {
		return false
	}
	dir := filepath.Dir(exe)
	for _, n := range []string{"unins000.exe", "uninstall.cmd"} {
		if _, err := os.Stat(filepath.Join(dir, n)); err == nil {
			return true
		}
	}
	return false
}

// parseSums находит в SHA256SUMS.txt контрольную сумму файла (формат sha256sum: «<hex> *<имя>»).
func parseSums(text, name string) string {
	for _, line := range strings.Split(text, "\n") {
		f := strings.Fields(strings.TrimSpace(line))
		if len(f) == 2 && strings.TrimPrefix(f[1], "*") == name && len(f[0]) == sha256.Size*2 {
			return strings.ToLower(f[0])
		}
	}
	return ""
}

func get(ctx context.Context, url string) (*http.Response, error) {
	if !strings.HasPrefix(url, assetPrefix) {
		return nil, fmt.Errorf("неожиданный адрес загрузки: %s", url)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Veil-update")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, fmt.Errorf(L("download failed: %s", "загрузка не удалась: %s"), resp.Status)
	}
	return resp, nil
}

// downloadAsset скачивает файл релиза во временную папку и проверяет его SHA-256 по SHA256SUMS.txt из того же релиза.
// progress получает (скачано, всего); всего может быть 0, если размер неизвестен. Возвращает путь к файлу.
func downloadAsset(ctx context.Context, rel *release, name string, progress func(done, total int64)) (string, error) {
	url, ok := rel.Assets[name]
	if !ok {
		return "", fmt.Errorf(L("the release has no file %s", "в релизе нет файла %s"), name)
	}
	sumsURL, ok := rel.Assets["SHA256SUMS.txt"]
	if !ok {
		return "", errors.New(L("the release has no SHA256SUMS.txt: refusing to install an unverified file", "в релизе нет SHA256SUMS.txt: непроверенный файл ставить нельзя"))
	}
	sr, err := get(ctx, sumsURL)
	if err != nil {
		return "", err
	}
	sums, err := io.ReadAll(io.LimitReader(sr.Body, 64<<10))
	sr.Body.Close()
	if err != nil {
		return "", err
	}
	want := parseSums(string(sums), name)
	if want == "" {
		return "", fmt.Errorf(L("no checksum for %s in SHA256SUMS.txt", "для %s нет контрольной суммы в SHA256SUMS.txt"), name)
	}

	resp, err := get(ctx, url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	dir, err := os.MkdirTemp("", "veil-update-")
	if err != nil {
		return "", err
	}
	dst := filepath.Join(dir, name)
	f, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o700)
	if err != nil {
		return "", err
	}
	h := sha256.New()
	buf := make([]byte, 64<<10)
	var done int64
	for {
		n, rerr := resp.Body.Read(buf)
		if n > 0 {
			if _, werr := f.Write(buf[:n]); werr != nil {
				f.Close()
				os.RemoveAll(dir)
				return "", werr
			}
			h.Write(buf[:n])
			done += int64(n)
			if progress != nil {
				progress(done, resp.ContentLength)
			}
		}
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			f.Close()
			os.RemoveAll(dir)
			return "", rerr
		}
	}
	if err := f.Close(); err != nil {
		os.RemoveAll(dir)
		return "", err
	}
	if got := hex.EncodeToString(h.Sum(nil)); got != want {
		os.RemoveAll(dir)
		return "", errors.New(L("checksum mismatch: the file was corrupted or replaced", "контрольная сумма не совпала: файл повреждён или подменён"))
	}
	return dst, nil
}
