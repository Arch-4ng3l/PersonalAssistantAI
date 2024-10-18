package main

import (
	"encoding/json"

	"github.com/jinzhu/gorm"
)

type User struct {
    gorm.Model
    Email string `gorm:"unique;not null"`
    Password string
    GoogleID string
    SubscriptionStatus string
    CalenderToken json.RawMessage `gorm:"type:jsonb"`
}



