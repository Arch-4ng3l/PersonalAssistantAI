package main

import "github.com/jinzhu/gorm"

type User struct {
    gorm.Model
    Email string `gorm:"unique;not null"`
    Password string
    GoogleID string
    SubscriptionStatus string
}



