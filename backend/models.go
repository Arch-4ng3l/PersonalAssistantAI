package main

import (
	"encoding/json"
	"time"

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

type SubscriptionDetails struct {
	Plan            string    `json:"plan"`
	Status          string    `json:"status"`
	NextBillingDate time.Time `json:"nextBillingDate"`
	Amount          float64   `json:"amount"`
	Provider        string    `json:"provider"` // "stripe" or "paypal"
	SubscriptionID  string    `json:"subscriptionId"`
}
