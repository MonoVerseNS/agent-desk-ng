package config

import "testing"

func TestLanguageOrDefault(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "blank falls back", in: "", want: "zh-CN"},
		{name: "unsupported falls back", in: "fr-FR", want: "zh-CN"},
		{name: "chinese", in: "zh-CN", want: "zh-CN"},
		{name: "english", in: "en", want: "en-US"},
		{name: "english underscore", in: "EN_us", want: "en-US"},
		{name: "russian", in: "ru-RU", want: "ru-RU"},
		{name: "russian short", in: "ru", want: "ru-RU"},
		{name: "russian underscore", in: "ru_ru", want: "ru-RU"},
		{name: "russian padded", in: "  ru-RU  ", want: "ru-RU"},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			cfg := Config{Language: tt.in}
			if got := cfg.LanguageOrDefault(); got != tt.want {
				t.Fatalf("LanguageOrDefault(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
