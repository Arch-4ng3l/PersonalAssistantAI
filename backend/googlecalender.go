package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"

	"golang.org/x/oauth2"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/option"
)


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
