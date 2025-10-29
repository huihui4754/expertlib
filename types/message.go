package types

type PossibleIntentions struct {
	IntentName        string  `json:"intent_name"`
	IntentDescription string  `json:"intent_description"`
	Probability       float64 `json:"probability"`
}

type Attachment struct {
	Type   string `json:"type"`
	Name   string `json:"name,omitempty"`
	FileID string `json:"file_id,omitempty"`
	Option any    `json:"option,omitempty"`
}

type TotalMessage struct {
	EventType          int                  `json:"event_type"`
	DialogID           string               `json:"dialog_id"`
	UserId             string               `json:"user_id"`
	MessageID          string               `json:"message_id,omitempty"`
	Intention          string               `json:"intention,omitempty"` // 专家告诉程序库匹配的意图
	PossibleIntentions []PossibleIntentions `json:"possible_intentions,omitempty"`
	Messages           struct {
		Content     string       `json:"content"`
		Attachments []Attachment `json:"attachments"`
		History     []string     `json:"history,omitempty"`
	} `json:"messages,omitzero"`
}

// ClientMessage  客户端发送给专家的消息
type ClientMessage struct {
	EventType int    `json:"event_type"`
	DialogID  string `json:"dialog_id"`
	UserId    string `json:"user_id"`
	MessageID string `json:"message_id"`
	Messages  struct {
		Content     string       `json:"content"`
		Attachments []Attachment `json:"attachments"`
	} `json:"messages"`
}

// ExpertToProgramMessage 专家发送给程序库的消息
type ExpertToProgramMessage struct {
	EventType int    `json:"event_type"`
	DialogID  string `json:"dialog_id"`
	UserId    string `json:"user_id"`
	MessageID string `json:"message_id"`
	Intention string `json:"intention"` // 专家告诉程序库匹配的意图
	Messages  struct {
		Content     string       `json:"content"`
		Attachments []Attachment `json:"attachments"`
	} `json:"messages"`
}

// ExpertToProgramMessage 专家发送给多轮对话的消息
type ExpertToProgramChatMessage struct {
	EventType          int                  `json:"event_type"`
	DialogID           string               `json:"dialog_id"`
	MessageID          string               `json:"message_id"`
	UserId             string               `json:"user_id"`
	PossibleIntentions []PossibleIntentions `json:"possible_intentions"`
	Messages           struct {
		Content     string       `json:"content"`
		Attachments []Attachment `json:"attachments"`
		History     []string     `json:"history,omitempty"`
	} `json:"messages"`
}
