package api

import (
	"agent-desk/internal/pkg/dto/response"
	"agent-desk/internal/pkg/enums"
	"reflect"
	"testing"
)

func TestChannelConfigPromptStarters(t *testing.T) {
	rawConfig := `{
		"title": "Support Desk",
		"subtitle": "How can we help?",
		"themeColor": "#2563eb",
		"position": "right",
		"width": "380px",
		"promptStarters": [
			"How do I reset my password?",
			"What are your business hours?",
			"Can I talk to an agent?"
		]
	}`

	cfg, err := resolveWidgetConfig(enums.ChannelTypeWeb, rawConfig)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil {
		t.Fatalf("expected non-nil config")
	}

	expectedStarters := []string{
		"How do I reset my password?",
		"What are your business hours?",
		"Can I talk to an agent?",
	}

	if !reflect.DeepEqual(cfg.PromptStarters, expectedStarters) {
		t.Fatalf("expected prompt starters %v, got %v", expectedStarters, cfg.PromptStarters)
	}

	resp := response.WidgetConfigResponse{
		ChannelID:      "test-channel",
		ChannelType:    enums.ChannelTypeWeb,
		Title:          cfg.Title,
		Subtitle:       cfg.Subtitle,
		ThemeColor:     cfg.ThemeColor,
		Position:       cfg.Position,
		Width:          cfg.Width,
		PromptStarters: cfg.PromptStarters,
	}

	if !reflect.DeepEqual(resp.PromptStarters, expectedStarters) {
		t.Fatalf("expected response prompt starters %v, got %v", expectedStarters, resp.PromptStarters)
	}
}
