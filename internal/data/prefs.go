package data

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// PrefsDir holds our own settings, outside the install folder so an
// in-place upgrade never removes them and they never touch the shared
// config.db the original PortProxyGUI reads.
func PrefsDir() (string, error) {
	if p := os.Getenv("PORTPROXY_PREFS_DIR"); p != "" {
		return p, nil
	}
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "PortProxy"), nil
}

// LoadLanguagePref returns "" to follow the system, else a BCP 47 tag such
// as "en" or "zh-Hans".
func LoadLanguagePref() string {
	dir, err := PrefsDir()
	if err != nil {
		return ""
	}
	b, err := os.ReadFile(filepath.Join(dir, "language.txt"))
	if err != nil {
		return ""
	}
	tag := strings.TrimSpace(string(b))
	if tag == "" || tag == "auto" {
		return ""
	}
	return tag
}

func SaveLanguagePref(tag string) error {
	dir, err := PrefsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if tag == "" {
		tag = "auto"
	}
	return os.WriteFile(filepath.Join(dir, "language.txt"), []byte(tag+"\n"), 0o644)
}

func LoadPageSize() int {
	dir, err := PrefsDir()
	if err != nil {
		return 20
	}
	b, err := os.ReadFile(filepath.Join(dir, "page-size.txt"))
	if err != nil {
		return 20
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil || n <= 0 {
		return 20
	}
	return n
}

func SavePageSize(n int) error {
	dir, err := PrefsDir()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "page-size.txt"),
		[]byte(strconv.Itoa(n)+"\n"), 0o644)
}
