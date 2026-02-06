package i18n

import (
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"testing"
)

func TestI18N_AllLocalesCoverEnglishKeys(t *testing.T) {
	t.Parallel()

	repoRoot := testRepoRoot(t)
	sourceDir := filepath.Join(repoRoot, "i18n")

	enPath := filepath.Join(sourceDir, "en.toml")
	enData := readTOMLMap(t, enPath)
	if len(enData) == 0 {
		t.Fatalf("english locale is empty: %s", enPath)
	}

	files, err := filepath.Glob(filepath.Join(sourceDir, "*.toml"))
	if err != nil {
		t.Fatalf("glob locales: %v", err)
	}
	if len(files) == 0 {
		t.Fatalf("no locale files found in %s", sourceDir)
	}

	for _, file := range files {
		lang := strings.TrimSuffix(filepath.Base(file), ".toml")
		if lang == "en" {
			continue
		}
		t.Run(lang, func(t *testing.T) {
			locale := readTOMLMap(t, file)
			var missing []string
			for key := range enData {
				if _, ok := locale[key]; !ok {
					missing = append(missing, key)
				}
			}
			if len(missing) > 0 {
				sort.Strings(missing)
				show := missing
				if len(show) > 12 {
					show = show[:12]
				}
				t.Fatalf("locale %s misses %d keys; first missing keys: %v", lang, len(missing), show)
			}
		})
	}
}

func TestI18N_EmbeddedLocalesSyncedWithSourceLocales(t *testing.T) {
	t.Parallel()

	repoRoot := testRepoRoot(t)
	sourceDir := filepath.Join(repoRoot, "i18n")
	embeddedDir := filepath.Join(repoRoot, "internal", "i18n", "locales")

	files, err := filepath.Glob(filepath.Join(sourceDir, "*.toml"))
	if err != nil {
		t.Fatalf("glob source locales: %v", err)
	}

	for _, sourcePath := range files {
		langFile := filepath.Base(sourcePath)
		embeddedPath := filepath.Join(embeddedDir, langFile)
		t.Run(langFile, func(t *testing.T) {
			source := readTOMLMap(t, sourcePath)
			embedded := readTOMLMap(t, embeddedPath)

			if len(source) != len(embedded) {
				t.Fatalf("key count mismatch source=%d embedded=%d", len(source), len(embedded))
			}
			for key, sourceValue := range source {
				embeddedValue, ok := embedded[key]
				if !ok {
					t.Fatalf("embedded locale misses key: %s", key)
				}
				if embeddedValue != sourceValue {
					t.Fatalf("value mismatch for key %q: source=%q embedded=%q", key, sourceValue, embeddedValue)
				}
			}
		})
	}
}

func testRepoRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatalf("runtime.Caller failed")
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", ".."))
}

func readTOMLMap(t *testing.T, path string) map[string]string {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return parseSimpleTOML(string(raw))
}
