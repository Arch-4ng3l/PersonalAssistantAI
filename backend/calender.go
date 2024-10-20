package main

type Event interface  {
    GetTitle() string
    GetStartTime() string
    GetEndTime() string
    GetID() string
}

type Calendar interface {
    CreateEvent(title, startTime, endTime string) error
    RemoveEvent(title, startTime, endTime string) error
    GetEvent(title, startTime, endTime string) error
}
