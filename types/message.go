package types

type Attachment struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	FileID string `json:"file_id"`
	Option any    `json:"option"`
}
