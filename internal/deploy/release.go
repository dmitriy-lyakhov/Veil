package deploy

import (
	"fmt"
	"regexp"
	"strings"
)

// Release — откуда сервер сам скачивает свои файлы, если у приложения их нет рядом (Android):
// файлы релиза <имя>-<версия>-linux-<arch> и SHA256SUMS.txt по адресу BaseURL. Скачивает сам сервер,
// поэтому через устройство трафик не идёт, а контрольные суммы проверяются на сервере.
type Release struct {
	BaseURL string // например https://github.com/<владелец>/Veil/releases/download/v1.0.1
	Version string // например 1.0.1
}

var releaseVersionRe = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)

// GitHubRelease — релиз версии version в репозитории GitHub owner/repo.
func GitHubRelease(repo, version string) *Release {
	version = strings.TrimPrefix(strings.TrimSpace(version), "v")
	version, _, _ = strings.Cut(version, "-") // 1.0.1-debug → 1.0.1
	return &Release{BaseURL: "https://github.com/" + repo + "/releases/download/v" + version, Version: version}
}

func (rel *Release) asset(base, arch string) string {
	return base + "-" + rel.Version + "-linux-" + arch
}

func (rel *Release) valid() error {
	if !releaseVersionRe.MatchString(rel.Version) {
		return fmt.Errorf("некорректная версия релиза %q", rel.Version)
	}
	if !strings.HasPrefix(rel.BaseURL, "https://") || strings.ContainsAny(rel.BaseURL, " '\"`$\\;&|<>") {
		return fmt.Errorf("некорректный адрес релиза %q", rel.BaseURL)
	}
	return nil
}

// fetchRelease скачивает на сервере файлы base (veil-server, wireguard-go, …) для arch, проверяет их по
// SHA256SUMS.txt того же релиза и кладёт в dst(base) с правами 0755. Нужен curl или wget.
func (r *remote) fetchRelease(bases []string, arch string, dst func(base string) string, logf Logf) error {
	rel := r.rel
	if err := rel.valid(); err != nil {
		return err
	}
	var files []string
	var b strings.Builder
	b.WriteString(`set -e
t=$(mktemp -d); trap 'rm -rf "$t"' EXIT; cd "$t"
get() {
  if command -v curl >/dev/null 2>&1; then curl -fsSL --retry 3 --connect-timeout 20 --max-time 600 -o "$2" "$1"
  elif command -v wget >/dev/null 2>&1; then wget -q -T 30 -t 3 -O "$2" "$1"
  else echo "на сервере нет ни curl, ни wget" >&2; exit 3; fi
}
get '` + rel.BaseURL + `/SHA256SUMS.txt' SHA256SUMS.txt || { echo "не удалось скачать SHA256SUMS.txt из ` + rel.BaseURL + `" >&2; exit 4; }
`)
	for _, base := range bases {
		name := rel.asset(base, arch)
		files = append(files, name)
		fmt.Fprintf(&b, `get '%s/%s' '%s' || { echo "не удалось скачать %s" >&2; exit 5; }
want=$(sed -n 's/^\([0-9a-f]\{64\}\) [ *]%s$/\1/p' SHA256SUMS.txt)
[ -n "$want" ] || { echo "в SHA256SUMS.txt нет %s" >&2; exit 6; }
got=$(sha256sum '%s' | cut -d' ' -f1)
[ "$want" = "$got" ] || { echo "контрольная сумма %s не совпала" >&2; exit 7; }
mkdir -p "$(dirname %s)" && install -m 0755 '%s' %s
`, rel.BaseURL, name, name, name, regexp.QuoteMeta(name), name, name, name, shellQuote(dst(base)), name, shellQuote(dst(base)))
	}
	what := strings.Join(files, ", ")
	logf("Сервер скачивает %s из %s…", what, rel.BaseURL)
	r.step("download", "Загрузка файлов Veil "+rel.Version, "сервер скачивает из релиза: "+what, "active")
	if out, err := r.in("").exec("sh -c "+shellQuote(b.String())+" 2>&1", nil, true); err != nil {
		msg := strings.TrimSpace(tail(out, 6))
		if msg == "" {
			msg = err.Error()
		}
		r.step("download", "Загрузка файлов Veil "+rel.Version, msg, "fail")
		return fmt.Errorf("сервер не смог скачать файлы Veil %s: %s", rel.Version, msg)
	}
	r.step("download", "Загрузка файлов Veil "+rel.Version, what+": контрольные суммы совпали", "done")
	return nil
}
