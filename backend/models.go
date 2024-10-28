package main

import (
	"encoding/json"

	"github.com/jinzhu/gorm"
)

const (
	Premium   string = "Premium"
	Basic     string = "Basic"
	Microsoft string = "MICROSOFT"
	Google    string = "GOOGLE"
	Stripe    string = "STRIPE"
	Paypal    string = "PAYPAL"
)

type User struct {
	gorm.Model
	Email                string `gorm:"unique;not null"`
	Password             string
	Provider             string
	ProviderID           string
	SubscriptionProvider string
	SubscriptionStatus   string
	SubscriptionID       string
	SubscriptionPlan     string
	CalenderToken        json.RawMessage `gorm:"type:jsonb"`
}
