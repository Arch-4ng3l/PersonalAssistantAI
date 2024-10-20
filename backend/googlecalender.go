package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)


type GoogleCalendar struct {
    service *calendar.Service
}


func NewGoogleCalendar(config *oauth2.Config, user User) *GoogleCalendar {
    var token oauth2.Token

    if err := json.Unmarshal(user.CalenderToken, &token); err != nil {
        log.Fatal(err)
    }
    client := getClient(config, &token)

    srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))

    if err != nil {
        log.Fatal(err)
    }

    return &GoogleCalendar{
	service: srv,
    }
}

func (c *GoogleCalendar) CreateEvent(event Event) error {
    googleEvent := &calendar.Event{}
    googleEvent.Id = event.ID
    googleEvent.Summary = event.Title
    googleEvent.Start.DateTime = event.StartTime
    googleEvent.End.DateTime= event.EndTime

    _, err := c.service.Events.Insert("primary", googleEvent).Do() 
    return err
}

func (c *GoogleCalendar) RemoveEvent(event Event) error {
    return c.service.Events.Delete("primary", event.ID).Do()
}


func (c *GoogleCalendar) GetEvents(startTime, endTime time.Time) ([]*Event, error) {
    events, err := c.
	service.
	Events.
	List("primary").
	TimeMax(startTime.Format(time.RFC3339)).
	TimeMax(endTime.Format(time.RFC3339)).
	Do()

    eventArr := make([]*Event, len(events.Items))

    for i, event := range events.Items {
	eventArr[i] = &Event{
	    ID: event.Id,
	    Title: event.Summary,
	    StartTime: event.Start.DateTime,
	    EndTime: event.End.DateTime,
	}
    }

    return eventArr, err
}


func getClient(config *oauth2.Config, token *oauth2.Token) *http.Client{
    return config.Client(context.Background(), token)
}

func GetCalender(config *oauth2.Config, user User) *calendar.Service {
    var token oauth2.Token
    if err := json.Unmarshal(user.CalenderToken, &token); err != nil {
        log.Fatal(err)
    }
    client := getClient(config, &token)

    srv, err := calendar.NewService(context.Background(), option.WithHTTPClient(client))

    if err != nil {
        log.Fatal(err)
    }

    return srv
    
}
