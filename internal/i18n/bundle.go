package i18n

import (
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Bundle struct {
	translations map[string]map[string]string
	available    []string
}

func Load(dir string) *Bundle {
	b := &Bundle{
		translations: map[string]map[string]string{},
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return b
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".toml") {
			continue
		}
		lang := strings.TrimSuffix(entry.Name(), ".toml")
		path := filepath.Join(dir, entry.Name())
		raw, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		b.translations[lang] = parseSimpleTOML(string(raw))
	}

	for lang := range b.translations {
		b.available = append(b.available, lang)
	}
	sort.Strings(b.available)
	return b
}

func (b *Bundle) AvailableLanguages() []string {
	out := make([]string, 0, len(b.available)+1)
	out = append(out, "system")
	out = append(out, b.available...)
	return out
}

func (b *Bundle) ResolveLanguage(selected string) string {
	if selected != "" && selected != "system" {
		if _, ok := b.translations[selected]; ok {
			return selected
		}
	}

	if env := os.Getenv("LANG"); env != "" {
		env = strings.ToLower(env)
		env = strings.SplitN(env, ".", 2)[0]
		env = strings.ReplaceAll(env, "-", "_")
		if _, ok := b.translations[env]; ok {
			return env
		}
		if base := strings.SplitN(env, "_", 2)[0]; base != "" {
			if _, ok := b.translations[base]; ok {
				return base
			}
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
