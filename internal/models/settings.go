package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Settings struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	UserID          string             `bson:"user_id" json:"user_id"`
	Windows         map[string]int     `bson:"windows" json:"windows"`
	TTL             map[string]int     `bson:"ttl" json:"ttl"`
	PollInterval    string             `bson:"poll_interval" json:"poll_interval"`
	Timezone        string             `bson:"timezone" json:"timezone"`
	InitialLookback string             `bson:"initial_lookback" json:"initial_lookback"`
}

func DefaultSettings(userID string) *Settings {
	return &Settings{
		UserID: userID,
		Windows: map[string]int{
			"birthday":     7,
			"subscription": 3,
			"payment":      5,
			"task":         1,
			"event":        2,
		},
		TTL: map[string]int{
			"birthday":     2,
			"subscription": 7,
			"payment":      7,
			"task":         14,
			"event":        1,
		},
		PollInterval:    "15m",
		Timezone:        "Asia/Kolkata",
		InitialLookback: "3m",
	}
}
