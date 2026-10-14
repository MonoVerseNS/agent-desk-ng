package response

type ConversationSatisfactionResponse struct {
	ID             int64  `json:"id"`
	ConversationID int64  `json:"conversationId"`
	Score          int    `json:"score"`
	Feedback       string `json:"feedback"`
	CreatedAt      string `json:"createdAt"`
}
