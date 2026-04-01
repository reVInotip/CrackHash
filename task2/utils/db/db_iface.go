package db

import ent "TaskOneUtils/db/entities"

type Database interface {
	CreateRequest(req *ent.Request) error
	GetRequestByID(requestID string) (*ent.Request, error)
	DeleteRequest(id string) error
	Close() error
}