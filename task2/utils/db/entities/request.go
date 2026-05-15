package entities

import (
	"time"
    "go.mongodb.org/mongo-driver/v2/bson"
)

type Request struct {
    ID          bson.ObjectID      `bson:"_id,omitempty" json:"id"`
    RequestID   string             `bson:"request_id" json:"request_id"`
    Hash        string             `bson:"hash" json:"hash"`
	MaxLen		int				   `bson:"max_len" json:"max_len"`
    Words       []string           `bson:"words,omitempty" json:"words,omitempty"`
    PartCount   int                `bson:"part_count" json:"part_count"`
    ReceivedParts map[int]bool     `bson:"received_parts" json:"received_parts"`
    CreatedAt   time.Time          `bson:"created_at" json:"created_at"`
    UpdatedAt   time.Time          `bson:"updated_at" json:"updated_at"`
    CompletedAt *time.Time         `bson:"completed_at,omitempty" json:"completed_at,omitempty"`
}