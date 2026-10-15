package services

import (
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/i18nx"
	"testing"
)

func TestDashboardTextUsesEnglishLocale(t *testing.T) {
	t.Parallel()

	if got := dashboardText(i18nx.LocaleEnUS, "alert.pendingLongWait.title"); got != "Queued conversations are piling up" {
		t.Fatalf("dashboardText(en-US) = %q", got)
	}
	if got := dashboardText(i18nx.LocaleRuRU, "alert.pendingLongWait.title"); got != "Очередь диалогов переполнена" {
		t.Fatalf("dashboardText(ru-RU) = %q", got)
	}
	if got := dashboardText(i18nx.LocaleZhCN, "alert.pendingLongWait.title"); got != "待接入会话堆积" {
		t.Fatalf("dashboardText(zh-CN) = %q", got)
	}
	if got := dashboardText("fr-FR", "alert.pendingLongWait.title"); got != "Queued conversations are piling up" {
		t.Fatalf("dashboardText(fallback) = %q, want English fallback", got)
	}
}

func TestConversationStatusLabelUsesEnglishLocale(t *testing.T) {
	t.Parallel()

	if got := conversationStatusLabel(enums.IMConversationStatusPending, i18nx.LocaleEnUS); got != "Queued" {
		t.Fatalf("conversationStatusLabel(en-US) = %q", got)
	}
	if got := conversationStatusLabel(enums.IMConversationStatusPending, i18nx.LocaleRuRU); got != "В очереди" {
		t.Fatalf("conversationStatusLabel(ru-RU) = %q", got)
	}
	if got := conversationStatusLabel(enums.IMConversationStatusPending, i18nx.LocaleZhCN); got != "待接入" {
		t.Fatalf("conversationStatusLabel(zh-CN) = %q", got)
	}
	if got := conversationStatusLabel(enums.IMConversationStatusPending, "fr-FR"); got != "Queued" {
		t.Fatalf("conversationStatusLabel(fallback) = %q, want English fallback", got)
	}
}
