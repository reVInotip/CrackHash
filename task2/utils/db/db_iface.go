package db

import ent "TaskOneUtils/db/entities"

type Database interface {
	CreateRequest(req *ent.Request) error
	CreateRequestWithoutConnRetry(req *ent.Request) error
	GetRequestByID(requestID string) (*ent.Request, error)
	DeleteRequest(id string) error
	UpdateRequestReceivedPartsAndWords(requestID string, receivedPart int, words []string) error
	GetAllRequests() ([]ent.Request, error)
	Close() error
}