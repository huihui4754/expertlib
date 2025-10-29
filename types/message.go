package types

type Attachment struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	FileID string `json:"file_id"`
	Option any    `json:"option"`
}

// ClientMessage  客户端发送给专家的消息
type ClientMessage struct {
	EventType int    `json:"event_type"`
	DialogID  string `json:"dialog_id"`
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
	MessageID string `json:"message_id"`
	Intention string `json:"intention"` // 专家告诉程序库匹配的意图
	Messages  struct {
		Content     string       `json:"content"`
		Attachments []Attachment `json:"attachments"`
	} `json:"messages"`
}
