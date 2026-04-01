package entities

import (
	"time"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Request struct {
    ID          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
    RequestID   string             `bson:"request_id" json:"request_id"`
    Hash        string             `bson:"hash" json:"hash"`
    Status      string             `bson:"status" json:"status"` // IN_PROGRESS, READY, ERROR
    Words       []string           `bson:"words,omitempty" json:"words,omitempty"`
    PartCount   int                `bson:"part_count" json:"part_count"`
    ReceivedParts []int            `bson:"received_parts" json:"received_parts"`
    CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
    CompletedAt *time.Time         `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
}