package i18nx

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

var localeVerbPattern = regexp.MustCompile(`%[a-zA-Z]`)
var routeKeyPattern = regexp.MustCompile(`^[a-z][A-Za-z0-9]*(\.[^.\s]+)+$`)

func readLocaleFile(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("locales", name+".yml"))
	if err != nil {
		t.Fatalf("read locale %s: %v", name, err)
	}
	return string(data)
}

// Locale files must stay key-for-key identical so no user-visible string is
// silently served in the wrong language for one locale only.
func TestLocaleFilesShareTheSameKeys(t *testing.T) {
	t.Parallel()

	base := readLocaleFile(t, "en-US")
	baseKeys := localeKeysFromYAML(t, base)

	for _, name := range []string{"zh-CN", "ru-RU"} {
		keys := localeKeysFromYAML(t, readLocaleFile(t, name))
		for key := range baseKeys {
			if _, ok := keys[key]; !ok {
				t.Errorf("%s.yml is missing key %q", name, key)
			}
		}
		for key := range keys {
			if _, ok := baseKeys[key]; !ok {
				t.Errorf("%s.yml has extra key %q", name, key)
			}
		}
	}
}

// Every locale must format arguments the same way, otherwise a message renders
// with %!s(MISSING) or a wrong value once the locale changes.
func TestLocaleFilesShareTheSameVerbs(t *testing.T) {
	t.Parallel()

	base := localeEntriesFromYAML(t, readLocaleFile(t, "en-US"))

	for _, name := range []string{"zh-CN", "ru-RU"} {
		values := localeEntriesFromYAML(t, readLocaleFile(t, name))
		for key, want := range base {
			got, ok := values[key]
			if !ok {
				continue
			}
			if verbs := localeVerbPattern.FindAllString(want, -1); !sameVerbs(verbs, localeVerbPattern.FindAllString(got, -1)) {
				t.Errorf("%s.yml key %q verbs = %v, want %v", name, key, verbs, localeVerbPattern.FindAllString(got, -1))
			}
		}
	}
}

func TestRussianLocaleTranslatesRepresentativeKeys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		key  string
		args []any
		want string
	}{
		{name: "expired session", key: "error.auth.expired", want: "Сессия истекла. Войдите снова."},
		{name: "directory not found", key: "error.e0273", want: "Каталог не найден."},
		{
			name: "knowledge base references",
			key:  "error.knowledgeBase.referencedByAgent",
			args: []any{"SalesBot"},
			want: "На эту базу знаний ссылается ИИ-агент «SalesBot». Сначала удалите привязку.",
		},
		{
			name: "schedule batch limit",
			key:  "error.agentTeamSchedule.batchLimit",
			args: []any{31},
			want: "За один раз можно создать не более 31 записей расписания.",
		},
		{
			name: "mcp list failure",
			key:  "error.mcp.listToolsFailed",
			args: []any{"timeout"},
			want: "Не удалось получить список инструментов MCP: timeout",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := Getf(LocaleRuRU, tt.key, tt.args...); got != tt.want {
				t.Fatalf("Getf(%q) = %q, want %q", LocaleRuRU, got, tt.want)
			}
		})
	}
}

func TestNormalizeLocaleAcceptsRussianAliases(t *testing.T) {
	previous := DefaultLocale
	DefaultLocale = LocaleZhCN
	t.Cleanup(func() { DefaultLocale = previous })

	for _, in := range []string{"ru-RU", "ru_ru", "RU", " ru-ru "} {
		if got := NormalizeLocale(in); got != LocaleRuRU {
			t.Fatalf("NormalizeLocale(%q) = %q, want %q", in, got, LocaleRuRU)
		}
	}
}

func localeEntriesFromYAML(t *testing.T, raw string) map[string]string {
	t.Helper()
	values := map[string]string{}
	if err := yaml.Unmarshal([]byte(raw), &values); err != nil {
		t.Fatalf("parse locale: %v", err)
	}
	if len(values) == 0 {
		t.Fatal("locale file parsed to zero entries")
	}
	return values
}

// localeKeysFromYAML keeps only real message keys so commented or empty lines
// do not influence the parity comparison.
func localeKeysFromYAML(t *testing.T, raw string) map[string]string {
	t.Helper()
	entries := localeEntriesFromYAML(t, raw)
	keys := map[string]string{}
	for key, value := range entries {
		if strings.TrimSpace(value) == "" {
			continue
		}
		if !routeKeyPattern.MatchString(key) {
			t.Fatalf("unexpected locale key shape %q", key)
		}
		keys[key] = value
	}
	return keys
}

func sameVerbs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
