package worker

import (
	"TaskOneUtils/models"
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"log"

	br "TaskOneUtils/message_broker"
)

type worker struct {
	msgBroker	br.MessageBroker
}

func NewWorker(msgBroker br.MessageBroker) *worker {
	return &worker{
		msgBroker: msgBroker,
	}
}

func (w *worker) HandleTask(body []byte) {
	var task models.WorkerTask
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&task); err != nil {
		log.Println("Invalid JSON")
		return
	}

	w.processTask(task)
}

func (w *worker) processTask(task models.WorkerTask) {
	log.Printf("Processing task %s part %d/%d", task.RequestID, task.PartNumber, task.PartCount)
	words := w.crack(task.Hash, task.Alphabet, task.MaxLength, task.PartNumber, task.PartCount)
	if len(words) == 0 {
		log.Printf("No words found for task %s part %d", task.RequestID, task.PartNumber)
	}
	// Send response to manager
	resp := models.WorkerResponse{
		RequestID:  task.RequestID,
		PartNumber: task.PartNumber,
		Words:      words,
	}
	data, err := xml.Marshal(resp)
	if err != nil {
		log.Printf("Failed to marshal response: %v", err)
		return
	}

	err = w.msgBroker.SendMessage("worker_responses", "application/xml", data)
	if err != nil {
		log.Printf("Can not send message to manager: %s", err)
	}
}

func (w *worker) crack(targetHash, alphabet string, maxLength, partNumber, partCount int) []string {
	a := uint64(len(alphabet))
	// Precompute cumulative counts for each length
	cum := make([]uint64, maxLength+1)
	cum[0] = 0
	for l := 1; l <= maxLength; l++ {
		// a^l
		pow := uint64(1)
		for i := 0; i < l; i++ {
			pow *= a
		}
		cum[l] = cum[l-1] + pow
	}
	total := cum[maxLength]
	// Determine range for this worker (0-based indices)
	start := total * uint64(partNumber-1) / uint64(partCount)
	end := total * uint64(partNumber) / uint64(partCount) - 1
	if end >= total {
		end = total - 1
	}
	if start > end {
		return nil
	}

	targetBytes, err := hex.DecodeString(targetHash)
	if err != nil {
		log.Printf("Invalid hash: %v", err)
		return nil
	}

	var found []string
	for idx := start; idx <= end; idx++ {
		word := indexToWord(idx, alphabet, maxLength, cum)
		hash := md5.Sum([]byte(word))
		if bytes.Equal(hash[:], targetBytes) {
			found = append(found, word)
		}
	}
	return found
}

// indexToWord converts a linear index (0-based) into a word of length up to maxLength.
// cum is cumulative counts per length: cum[l] = sum_{i=1..l} a^i.
func indexToWord(idx uint64, alphabet string, maxLength int, cum []uint64) string {
	// Find length
	length := 0
	for l := 1; l <= maxLength; l++ {
		if idx < cum[l] {
			length = l
			break
		}
	}
	if length == 0 {
		return "" // should not happen
	}
	offset := idx - cum[length-1] // offset within this length
	a := uint64(len(alphabet))
	// Build word from least significant digit
	word := make([]byte, length)
	for i := length - 1; i >= 0; i-- {
		word[i] = alphabet[offset%a]
		offset /= a
	}
	return string(word)
}