package response

type SuggestReplyResponse struct {
	Suggestion string `json:"suggestion"`
}

type ConversationSummaryResponse struct {
	Summary string `json:"summary"`
}
