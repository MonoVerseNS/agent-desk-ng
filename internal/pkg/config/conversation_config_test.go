package config

import "testing"

func TestConversationMaxOpen(t *testing.T) {
	tests := []struct {
		name   string
		config ConversationConfig
		want   int
	}{
		{name: "unset falls back to a cap", config: ConversationConfig{}, want: 3},
		{name: "explicit zero also means the default", config: ConversationConfig{CustomerMaxOpen: 0}, want: 3},
		{name: "explicit value wins", config: ConversationConfig{CustomerMaxOpen: 7}, want: 7},
		{name: "negative removes the cap", config: ConversationConfig{CustomerMaxOpen: -1}, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.config.MaxOpen(); got != tt.want {
				t.Fatalf("MaxOpen() = %d, want %d", got, tt.want)
			}
		})
	}
}
