package runtime

import (
	"encoding/json"
	"strings"
	"time"

	applicationruntime "agent-desk/internal/ai/application/runtime"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/i18nx"
	svc "agent-desk/internal/services"
)

type interruptMessagePreview struct {
	Message string `json:"message"`
}

func buildConversationInterrupt(conversation models.Conversation, message models.Message, aiAgent models.AIAgent, summary *applicationruntime.RunResult) *models.ConversationInterrupt {
	if summary == nil {
		return nil
	}
	now := time.Now()
	item := svc.ConversationInterruptService.GetByCheckPointID(summary.CheckPointID)
	if item == nil {
		item = &models.ConversationInterrupt{
			CheckPointID: summary.CheckPointID,
			CreatedAt:    now,
		}
	}
	item.ConversationID = conversation.ID
	item.AIAgentID = aiAgent.ID
	item.AgentRunID = summary.AgentRunID
	item.SourceMessageID = message.ID
	item.InterruptID = firstInterruptID(summary)
	item.InterruptType = firstInterruptType(summary)
	item.WorkflowRunID = summary.WorkflowRunID
	item.WorkflowNodeID = firstInterruptID(summary)
	item.Status = "pending"
	locale := svc.ConversationHumanDispatchService.ResolveConversationLocale(conversation.ID)
	item.PromptText = resolveInterruptPrompt(summary, locale)
	item.RequestData = strings.TrimSpace(summary.CheckPointData)
	item.UpdatedAt = now
	return item
}

func resolveInterruptPrompt(summary *applicationruntime.RunResult, locale ...string) string {
	targetLocale := i18nx.DefaultLocale
	if len(locale) > 0 && strings.TrimSpace(locale[0]) != "" {
		targetLocale = locale[0]
	}
	if summary == nil || len(summary.Interrupts) == 0 {
		return i18nx.Getf(targetLocale, "conversation.interrupt.defaultPrompt")
	}
	interrupt := summary.Interrupts[0]
	if prompt := strings.TrimSpace(interrupt.PromptText); prompt != "" {
		return prompt
	}
	if prompt := extractInterruptMessage(interrupt.InfoPreview); prompt != "" {
		return prompt
	}
	if prompt := strings.TrimSpace(summary.ReplyText); prompt != "" {
		return prompt
	}
	if interrupt.Type != "tool_confirmation" {
		if prompt := strings.TrimSpace(interrupt.InfoPreview); prompt != "" {
			return prompt
		}
	}
	if displayName := strings.TrimSpace(interrupt.DisplayName); displayName != "" {
		switch targetLocale {
		case "ru-RU":
			return "Готовится выполнение «" + displayName + "». Подтверждаете продолжение?"
		case "en-US":
			return "About to execute \"" + displayName + "\". Confirm to continue?"
		default:
			return "即将执行“" + displayName + "”，是否确认继续？"
		}
	}
	return i18nx.Getf(targetLocale, "conversation.interrupt.defaultPrompt")
}

func extractInterruptMessage(infoPreview string) string {
	infoPreview = strings.TrimSpace(infoPreview)
	if infoPreview == "" {
		return ""
	}
	var payload interruptMessagePreview
	if err := json.Unmarshal([]byte(infoPreview), &payload); err != nil {
		return ""
	}
	return strings.TrimSpace(payload.Message)
}

func firstInterruptID(summary *applicationruntime.RunResult) string {
	if summary == nil || len(summary.Interrupts) == 0 {
		return ""
	}
	return strings.TrimSpace(summary.Interrupts[0].ID)
}

func firstInterruptType(summary *applicationruntime.RunResult) string {
	if summary == nil || len(summary.Interrupts) == 0 {
		return ""
	}
	return strings.TrimSpace(summary.Interrupts[0].Type)
}

func isCheckpointMissingError(err error) bool {
	if err == nil {
		return false
	}
	message := strings.ToLower(strings.TrimSpace(err.Error()))
	return strings.Contains(message, "failed to load from checkpoint") && strings.Contains(message, "not exist")
}
