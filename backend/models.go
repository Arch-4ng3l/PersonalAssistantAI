package main

import (
	"encoding/json"

	"github.com/jinzhu/gorm"
)

const (
    Premium string = "PREMIUM"
    Basic string = "BASIC"
    Microsoft string = "MICROSOFT"
    Google string = "GOOGLE"
)

type User struct {
    gorm.Model
    Email string `gorm:"unique;not null"`
    Password string
    Provider string
    ProviderID string
    SubscriptionStatus string
    SubscriptionPlan string
    CalenderToken json.RawMessage `gorm:"type:jsonb"`

}



