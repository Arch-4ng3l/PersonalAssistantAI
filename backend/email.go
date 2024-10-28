package main

import "time"



type Email struct {
    Type string `json:"type"`
    Subject string `json:"subject"`
    Body string `json:"body"`
    SendAt time.Time
}

type EmailClient interface {
    GetEmails(startTime time.Time, endTime time.Time, userID string) []*Email
}
