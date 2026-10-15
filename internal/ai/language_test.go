package ai

import (
	"testing"
)

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{"Russian greeting", "Здравствуйте, подскажите статус заказа", "ru"},
		{"Russian short", "Привет!", "ru"},
		{"English inquiry", "Hello, can you help me with a refund?", "en"},
		{"Chinese inquiry", "你好，请问怎么办理退款？", "zh"},
		{"Mixed Russian and English", "Привет, check this please", "ru"},
		{"Mixed Chinese and English", "你好 hello world", "zh"},
		{"Numbers only", "12345 67890", "en"},
		{"Empty string", "", "en"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DetectLanguage(tt.input)
			if got != tt.expected {
				t.Errorf("DetectLanguage(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestIsCyrillicAndIsHan(t *testing.T) {
	if !IsCyrillic("Привет") {
		t.Errorf("IsCyrillic('Привет') should be true")
	}
	if IsCyrillic("Hello world") {
		t.Errorf("IsCyrillic('Hello world') should be false")
	}
	if !IsHan("你好世界") {
		t.Errorf("IsHan('你好世界') should be true")
	}
	if IsHan("Hello world") {
		t.Errorf("IsHan('Hello world') should be false")
	}
}
