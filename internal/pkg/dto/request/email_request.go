package request

// EmailRequestRequest is the body of the API that opens a support request on a
// visitor's behalf and continues the conversation over email.
type EmailRequestRequest struct {
	// Email is the visitor's address. It becomes their contact identity, so it
	// must be the address their replies will come from.
	Email string `json:"email"`
	// Name is an optional display name.
	Name string `json:"name"`
	// Subject is the request title shown in the visitor's request list.
	Subject string `json:"subject"`
	// Message is the request body. Required.
	Message string `json:"message"`
}