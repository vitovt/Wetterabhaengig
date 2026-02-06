package i18n

import (
	"embed"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

//go:embed locales/*.toml
var embeddedLocales embed.FS

type Bundle struct {
	translations map[string]map[string]string
	available    []string
	systemLang   string
}

func Load(dir string) *Bundle {
	b := &Bundle{
		translations: map[string]map[string]string{},
	}

	_ = b.loadDir(dir)
	if len(b.translations) == 0 {
		_ = b.loadFS(embeddedLocales, "locales")
	}
	b.refreshAvailable()
	return b
}

func (b *Bundle) AvailableLanguages() []string {
	out := make([]string, 0, len(b.available)+1)
	out = append(out, "system")
	out = append(out, b.available...)
	return out
}

func (b *Bundle) SetSystemLanguage(lang string) {
	b.systemLang = normalizeLanguage(lang)
}

func (b *Bundle) ResolveLanguage(selected string) string {
	if selected != "" && selected != "system" {
		if matched := b.matchLanguage(selected); matched != "" {
			return matched
		}
	}

	if matched := b.matchLanguage(b.systemLang); matched != "" {
		return matched
	}

	if env := os.Getenv("LANG"); env != "" {
		if matched := b.matchLanguage(env); matched != "" {
			return matched
		}
	}

	if _, ok := b.translations["en"]; ok {
		return "en"
	}
	if len(b.available) > 0 {
		return b.available[0]
	}
	return "en"
}

func (b *Bundle) Text(selectedLanguage, key, fallback string) string {
	lang := b.ResolveLanguage(selectedLanguage)
	if values, ok := b.translations[lang]; ok {
		if text, ok := values[key]; ok && text != "" {
			return text
		}
	}
	if values, ok := b.translations["en"]; ok {
		if text, ok := values[key]; ok && text != "" {
			return text
		}
	}
	return fallback
}

func (b *Bundle) loadDir(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		lang := strings.TrimSuffix(entry.Name(), ".toml")
		filePath := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}
		b.translations[lang] = parseSimpleTOML(string(raw))
	}
	return nil
}

func (b *Bundle) loadFS(fsys fs.FS, dir string) error {
	entries, err := fs.ReadDir(fsys, dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		lang := strings.TrimSuffix(entry.Name(), ".toml")
		raw, err := fs.ReadFile(fsys, path.Join(dir, entry.Name()))
		if err != nil {
			continue
		}
		b.translations[lang] = parseSimpleTOML(string(raw))
	}
	return nil
}

func (b *Bundle) refreshAvailable() {
	b.available = b.available[:0]
	for lang := range b.translations {
		b.available = append(b.available, lang)
	}
	sort.Strings(b.available)
}

func (b *Bundle) matchLanguage(candidate string) string {
	candidate = normalizeLanguage(candidate)
	if candidate == "" {
		return ""
	}
	if _, ok := b.translations[candidate]; ok {
		return candidate
	}
	if base := strings.SplitN(candidate, "_", 2)[0]; base != "" {
		if _, ok := b.translations[base]; ok {
			return base
		}
	}
	return ""
}

func normalizeLanguage(raw string) string {
	raw = strings.TrimSpace(strings.ToLower(raw))
	if raw == "" {
		return ""
	}
	raw = strings.SplitN(raw, ".", 2)[0]
	raw = strings.ReplaceAll(raw, "-", "_")
	return raw
}

func parseSimpleTOML(input string) map[string]string {
	out := map[string]string{}
	section := ""

	for _, line := range strings.Split(input, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(line, "["), "]"))
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if unquoted, err := strconv.Unquote(value); err == nil {
			value = unquoted
		}
		fullKey := key
		if section != "" {
			fullKey = section + "." + key
		}
		out[fullKey] = value
	}

	return out
}
