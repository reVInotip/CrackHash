package models

import "encoding/xml"

// Task sent from manager to worker (JSON)
type WorkerTask struct {
	RequestID  string `json:"requestId"`
	PartNumber int    `json:"partNumber"`
	PartCount  int    `json:"partCount"`
	Hash       string `json:"hash"`
	MaxLength  int    `json:"maxLength"`
	Alphabet   string `json:"alphabet"`
}

// Response from worker to manager (XML)
type WorkerResponse struct {
	XMLName    xml.Name `xml:"response"`
	RequestID  string   `xml:"requestId"`
	PartNumber int      `xml:"partNumber"`
	Words      []string `xml:"words>word"` // <words><word>...</word>...</words>
}