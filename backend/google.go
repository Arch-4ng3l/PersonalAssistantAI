package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
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

    client := config.Client(context.Background(), &token)


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
	TimeMin(startTime.Format(time.RFC3339)).
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


func InitGoogle(config config.Config) {
    googleOAuthConf = &oauth2.Config{
        RedirectURL:  "http://localhost:8080/auth/google/callback",
        ClientID:     config.GoogleClientID,
        ClientSecret: config.GoogleClientSecret,
        Scopes:       []string{
            "https://www.googleapis.com/auth/userinfo.email",
            calendar.CalendarScope,
        },
        Endpoint:     google.Endpoint,
    }
}


func GoogleLogin(c *gin.Context) {
    url := googleOAuthConf.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
    c.Redirect(http.StatusTemporaryRedirect, url)
}


func GoogleCallback(c *gin.Context) {
    code := c.Query("code")
    if code == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Code not provided"})
        return
    }

    token, err := googleOAuthConf.Exchange(context.Background(), code)
    if err != nil {
        log.Printf("Failed to Exchange %s \n", err.Error())
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to exchange token"})
        return
    }

    // Fetch user info
    resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
    if err != nil {
        log.Printf("Failed to Fetch %s \n", err.Error())
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to get user info"})
        return
    }

    defer resp.Body.Close()
    var userInfo struct {
        Email string `json:"email"`
        ID    string `json:"id"`
    }

    if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Failed to parse user info"})
        return
    }

    // Check if user exists
    var user User
    if err := db.Where("provider_id = ?", userInfo.ID).First(&user).Error; err != nil {
        // If not, create
        user = User{Email: userInfo.Email, ProviderID: userInfo.ID, Provider: Google}

        tokenJSON, _ := json.Marshal(token)
        user.CalenderToken = tokenJSON
        if err := db.Create(&user).Error; err != nil {
            log.Println(err.Error())
        }

        c.Redirect(http.StatusTemporaryRedirect, "http://localhost:8080/payment")
    }

    jwtToken, err := GenerateJWT(user.Email)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
        log.Println(err.Error())
        return
    }
    service := NewGoogleCalendar(googleOAuthConf, user)
    calendarCache[jwtToken] = service

    // Redirect to frontend with token
    c.SetCookie("token", jwtToken, int(time.Now().Add(time.Hour * 24 * 7).Unix()), "/", "", true, false)

    c.Redirect(http.StatusTemporaryRedirect, "http://localhost:8080/chat")
}
