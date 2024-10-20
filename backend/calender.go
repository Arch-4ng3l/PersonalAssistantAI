package main

import "time"

type Event struct {
    Title string `json:"title"`
    StartTime string `json:"startTime"`
    EndTime string `json:"endTime"`
    ID string `json:"id"`
}

type Calendar interface {
    CreateEvent(Event) error
    RemoveEvent(Event) error
    GetEvents(startTime, endTime time.Time) ([]*Event, error)
}
