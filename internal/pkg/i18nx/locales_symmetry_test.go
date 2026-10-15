package i18nx

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func flattenYamlMap(prefix string, in map[string]any, out map[string]string) {
	for k, v := range in {
		fullKey := k
		if prefix != "" {
			fullKey = prefix + "." + k
		}
		switch val := v.(type) {
		case map[string]any:
			flattenYamlMap(fullKey, val, out)
		case string:
			out[fullKey] = val
		}
	}
}

func loadLocaleYaml(t *testing.T, filename string) map[string]string {
	t.Helper()
	data, err := os.ReadFile("locales/" + filename)
	if err != nil {
		t.Fatalf("failed to read %s: %v", filename, err)
	}
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		t.Fatalf("failed to unmarshal %s: %v", filename, err)
	}
	res := make(map[string]string)
	flattenYamlMap("", raw, res)
	return res
}

func TestLocalesSymmetry(t *testing.T) {
	zh := loadLocaleYaml(t, "zh-CN.yml")
	en := loadLocaleYaml(t, "en-US.yml")
	ru := loadLocaleYaml(t, "ru-RU.yml")

	t.Logf("zh-CN keys: %d, en-US keys: %d, ru-RU keys: %d", len(zh), len(en), len(ru))

	var missingInEn []string
	var missingInRu []string

	for k := range zh {
		if _, ok := en[k]; !ok {
			missingInEn = append(missingInEn, k)
		}
		if _, ok := ru[k]; !ok {
			missingInRu = append(missingInRu, k)
		}
	}

	if len(missingInEn) > 0 {
		t.Errorf("Missing in en-US.yml (%d): %s", len(missingInEn), strings.Join(missingInEn, ", "))
	}
	if len(missingInRu) > 0 {
		t.Errorf("Missing in ru-RU.yml (%d): %s", len(missingInRu), strings.Join(missingInRu, ", "))
	}
}
