package request

type SuggestReplyRequest struct {
	ConversationID int64 `json:"conversationId"`
}

type SummarizeConversationRequest struct {
	ConversationID int64 `json:"conversationId"`
}
