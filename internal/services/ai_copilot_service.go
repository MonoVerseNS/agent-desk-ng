package services

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"agent-desk/internal/ai"
	"agent-desk/internal/models"
	"agent-desk/internal/pkg/enums"
	"agent-desk/internal/pkg/errorsx"
	"agent-desk/internal/pkg/i18nx"
	"agent-desk/internal/pkg/utils"
	"agent-desk/internal/repositories"

	"github.com/mlogclub/simple/sqls"
)

var AICopilotService = newAICopilotService()

func newAICopilotService() *aiCopilotService {
	return &aiCopilotService{
		chatFn: ai.LLM.Chat,
	}
}

type aiCopilotService struct {
	chatFn func(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error)
}

// SetChatFnForTest allows tests outside package services to override LLM chat logic.
func (s *aiCopilotService) SetChatFnForTest(fn func(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error)) func() {
	prev := s.chatFn
	s.chatFn = fn
	return func() {
		s.chatFn = prev
	}
}

// SuggestReply analyzes recent conversation messages and produces a suggested draft reply for human operators.
func (s *aiCopilotService) SuggestReply(ctx context.Context, conversationID int64, locale string) (string, error) {
	if conversationID <= 0 {
		return "", errorsx.InvalidParamI18n("error.e0064")
	}
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return "", errorsx.NotFoundI18n("error.e0116")
	}

	messages := repositories.MessageRepository.Find(
		sqls.DB(),
		sqls.NewCnd().Eq("conversation_id", conversationID).Desc("id").Limit(12),
	)
	if len(messages) == 0 {
		return "", errorsx.InvalidParamI18n("error.e0245")
	}
	slices.Reverse(messages)

	transcript := s.buildConversationTranscript(messages)
	if strings.TrimSpace(transcript) == "" {
		return "", errorsx.InvalidParamI18n("error.e0245")
	}

	systemPrompt, userPrompt := s.buildSuggestReplyPrompts(transcript, locale)
	result, err := s.chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}
	if result == nil || strings.TrimSpace(result.Content) == "" {
		return "", fmt.Errorf("empty reply suggestion from model")
	}

	return result.Content, nil
}

// SummarizeConversation produces a structured summary of the conversation for handoffs or resolution records.
func (s *aiCopilotService) SummarizeConversation(ctx context.Context, conversationID int64, locale string) (string, error) {
	if conversationID <= 0 {
		return "", errorsx.InvalidParamI18n("error.e0064")
	}
	conversation := ConversationService.Get(conversationID)
	if conversation == nil {
		return "", errorsx.NotFoundI18n("error.e0116")
	}

	messages := repositories.MessageRepository.Find(
		sqls.DB(),
		sqls.NewCnd().Eq("conversation_id", conversationID).Desc("id").Limit(20),
	)
	if len(messages) == 0 {
		return "", errorsx.InvalidParamI18n("error.e0245")
	}
	slices.Reverse(messages)

	transcript := s.buildConversationTranscript(messages)
	if strings.TrimSpace(transcript) == "" {
		return "", errorsx.InvalidParamI18n("error.e0245")
	}

	systemPrompt, userPrompt := s.buildSummarizePrompts(transcript, locale)
	result, err := s.chat(ctx, systemPrompt, userPrompt)
	if err != nil {
		return "", err
	}
	if result == nil || strings.TrimSpace(result.Content) == "" {
		return "", fmt.Errorf("empty summary from model")
	}

	return result.Content, nil
}

func (s *aiCopilotService) chat(ctx context.Context, systemPrompt, userPrompt string) (*ai.ChatCompletionResult, error) {
	if s.chatFn != nil {
		return s.chatFn(ctx, systemPrompt, userPrompt)
	}
	return ai.LLM.Chat(ctx, systemPrompt, userPrompt)
}

func (s *aiCopilotService) buildConversationTranscript(messages []models.Message) string {
	var lines []string
	for _, msg := range messages {
		if msg.RecalledAt != nil || msg.SendStatus == enums.IMMessageStatusRecalled {
			continue
		}
		text := s.extractMessageText(msg)
		if text == "" {
			continue
		}
		switch msg.SenderType {
		case enums.IMSenderTypeCustomer:
			lines = append(lines, "Customer: "+text)
		case enums.IMSenderTypeAgent, enums.IMSenderTypeAI:
			lines = append(lines, "Agent: "+text)
		case enums.IMSenderTypeSystem:
			// System notifications are intentionally excluded from operator transcript
		}
	}
	return strings.Join(lines, "\n")
}

func (s *aiCopilotService) extractMessageText(msg models.Message) string {
	content := strings.TrimSpace(msg.Content)
	var text string
	switch msg.MessageType {
	case enums.IMMessageTypeHTML:
		summary := utils.BuildHTMLSummary(content)
		if summary != "" {
			text = summary
		} else {
			text = content
		}
	case enums.IMMessageTypeImage:
		if content != "" {
			text = fmt.Sprintf("[Image: %s]", content)
		} else {
			text = "[Image]"
		}
	case enums.IMMessageTypeAttachment:
		if content != "" {
			text = fmt.Sprintf("[Attachment: %s]", content)
		} else {
			text = "[Attachment]"
		}
	default:
		text = content
	}

	runes := []rune(text)
	if len(runes) > 2000 {
		return string(runes[:2000])
	}
	return text
}

func (s *aiCopilotService) buildSuggestReplyPrompts(transcript, locale string) (string, string) {
	normalized := i18nx.NormalizeLocale(locale)
	switch normalized {
	case i18nx.LocaleZhCN:
		systemPrompt := "你是一位专业的智能客服助手。请根据对话历史，为客服人员草拟一条针对客户最新诉求的专业、礼貌、准确且简洁的回复建议。回复语言必须与客户在对话中使用的语言保持一致。"
		userPrompt := "对话记录：\n<conversation_history>\n" + transcript + "\n</conversation_history>\n\n请草拟一条回复建议："
		return systemPrompt, userPrompt
	case i18nx.LocaleRuRU:
		systemPrompt := "Вы — эксперт и ассистент оператора службы поддержки клиентов. На основе истории диалога составьте полезный, профессиональный, вежливый и краткий вариант ответа на последний запрос клиента. Ответ должен быть на том языке, на котором пишет клиент."
		userPrompt := "История диалога:\n<conversation_history>\n" + transcript + "\n</conversation_history>\n\nПредложите вариант ответа:"
		return systemPrompt, userPrompt
	default:
		systemPrompt := "You are an expert customer support co-pilot. Based on the conversation history, draft a helpful, professional, polite, and concise reply to the customer's latest inquiry. Reply in the language used by the customer in the conversation."
		userPrompt := "Conversation history:\n<conversation_history>\n" + transcript + "\n</conversation_history>\n\nDraft a suggested reply:"
		return systemPrompt, userPrompt
	}
}

func (s *aiCopilotService) buildSummarizePrompts(transcript, locale string) (string, string) {
	normalized := i18nx.NormalizeLocale(locale)
	switch normalized {
	case i18nx.LocaleZhCN:
		systemPrompt := "你是一位专业的客服对话总结助手。请提供此客服对话的简明扼要总结，包含：1. 核心问题/诉求，2. 已采取的措施/已提供的解答，3. 当前状态/后续跟进事项。"
		userPrompt := "对话记录：\n<conversation_history>\n" + transcript + "\n</conversation_history>\n\n请总结此对话："
		return systemPrompt, userPrompt
	case i18nx.LocaleRuRU:
		systemPrompt := "Вы — ассистент по подведению итогов диалогов службы поддержки. Предоставьте краткую сводку диалога, включая: 1. Основная проблема / вопрос, 2. Принятые меры / предоставленные ответы, 3. Текущий статус / следующие шаги."
		userPrompt := "История диалога:\n<conversation_history>\n" + transcript + "\n</conversation_history>\n\nПодведите итог диалога:"
		return systemPrompt, userPrompt
	default:
		systemPrompt := "You are a customer support summarizer. Provide a concise summary of this customer support conversation including: 1. Main Issue / Inquiry, 2. Actions taken / Answers provided, 3. Current Status / Next Steps."
		userPrompt := "Conversation history:\n<conversation_history>\n" + transcript + "\n</conversation_history>\n\nSummarize this conversation:"
		return systemPrompt, userPrompt
	}
}
