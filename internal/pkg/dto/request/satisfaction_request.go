package request

type SubmitSatisfactionRequest struct {
	ConversationID int64  `json:"conversationId"`
	Score          int    `json:"score"`
	Feedback       string `json:"feedback"`
}
