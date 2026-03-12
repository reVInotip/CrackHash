package models

type CrackRequest struct {
	Hash      string `json:"hash"`
	MaxLength int    `json:"maxLength"`
}

type CrackResponse struct {
	RequestID string `json:"requestId"`
}

type StatusResponse struct {
	Status string   `json:"status"`
	Data   []string `json:"data"` // nil when status != READY
}