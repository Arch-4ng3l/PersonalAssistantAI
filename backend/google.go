package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-contrib/sessions"
	"github.com/gin-gonic/gin"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
	"google.golang.org/api/gmail/v1"
	"google.golang.org/api/option"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/renderer/html"
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
	googleEvent.Start = &calendar.EventDateTime{DateTime: event.StartTime}
	googleEvent.End = &calendar.EventDateTime{DateTime: event.EndTime}

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
		SingleEvents(true). // This expands recurring events into instances
		OrderBy("startTime").
		Do()

	if err != nil {
		return nil, err
	}

	eventArr := make([]*Event, len(events.Items))

	for i, event := range events.Items {
		eventArr[i] = &Event{
			ID:        event.Id,
			Title:     event.Summary,
			StartTime: event.Start.DateTime,
			EndTime:   event.End.DateTime,
		}
	}

	return eventArr, err
}

func InitGoogle(config config.Config) {
	googleOAuthConf = &oauth2.Config{
		RedirectURL:  "http://localhost:8080/auth/google/callback",
		ClientID:     config.GoogleClientID,
		ClientSecret: config.GoogleClientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/userinfo.email",
			calendar.CalendarScope,
			gmail.GmailReadonlyScope,
		},
		Endpoint: google.Endpoint,
	}
}

func GoogleLogin(c *gin.Context) error {
	state, err := generateStateToken()
	if err != nil {
		return err
	}

	session := sessions.Default(c)
	session.Set("oauth_state", StateToken{
		Value:     state,
		ExpiresAt: time.Now().Add(15 * time.Minute),
	})
	if err := session.Save(); err != nil {
		return err
	}

	url := googleOAuthConf.AuthCodeURL(state, oauth2.AccessTypeOffline)
	c.Redirect(http.StatusTemporaryRedirect, url)
	return nil
}

func GoogleCallback(c *gin.Context) error {
	session := sessions.Default(c)
	storedState, ok := session.Get("oauth_state").(StateToken)
	if !ok {
		return fmt.Errorf("Invalid session state")
	}

	session.Delete("oauth_state")
	session.Save()

	if time.Now().After(storedState.ExpiresAt) {
		return fmt.Errorf("State token expired")
	}

	receivedState := c.Query("state")
	if receivedState != storedState.Value {
		return fmt.Errorf("Invalid state parameter")
	}

	code := c.Query("code")
	if code == "" {
		return fmt.Errorf("Code not provided")
	}

	token, err := googleOAuthConf.Exchange(context.Background(), code)
	if err != nil {
		return fmt.Errorf("Failed to Exchange %s \n", err.Error())
	}

	// Fetch user info
	resp, err := http.Get("https://www.googleapis.com/oauth2/v2/userinfo?access_token=" + token.AccessToken)
	if err != nil {
		return fmt.Errorf("Failed to Fetch %s \n", err.Error())
	}

	defer resp.Body.Close()
	var userInfo struct {
		Email string `json:"email"`
		ID    string `json:"id"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&userInfo); err != nil {
		return fmt.Errorf("Failed to parse user info")
	}

	// Check if user exists
	user, err := GetUser(userInfo.Email)
	if err != nil {
		// If not, create
		user = &User{Email: userInfo.Email, ProviderID: userInfo.ID, Provider: Google}

		tokenJSON, _ := json.Marshal(token)
		user.CalenderToken = tokenJSON
		if err := db.Create(&user).Error; err != nil {
			return err
		}
		c.Redirect(http.StatusTemporaryRedirect, "http://localhost:8080/payment")
	}

	jwtToken, err := GenerateJWT(user.Email)
	if err != nil {
		return err
	}
	service := NewGoogleCalendar(googleOAuthConf, *user)
	calendarCache[jwtToken] = service

	// Redirect to frontend with token
	c.SetCookie("token", jwtToken, int(time.Now().Add(time.Hour*24*7).Unix()), "/", "", true, false)

	c.Redirect(http.StatusTemporaryRedirect, "http://localhost:8080/chat")
	return nil
}

type GoogleEmailClient struct {
	service *gmail.Service
}

func NewGoogleMail(token *oauth2.Token) *GoogleEmailClient {
	service, _ := gmail.NewService(
		context.Background(),
		option.WithTokenSource(googleOAuthConf.TokenSource(context.Background(), token)),
	)
	return &GoogleEmailClient{
		service: service,
	}
}

func (c *GoogleEmailClient) GetEmails(startTime, endTime time.Time, userID string) []*Email {
	query := fmt.Sprintf("after:%d before:%d", startTime.Unix(), endTime.Unix())
	resp, err := c.service.Users.Messages.List(userID).Q(query).Do()
	if err != nil {
		log.Println(err.Error())
		return nil
	}
	messages := make([]*Email, len(resp.Messages))
	log.Println(len(resp.Messages))

	wg := sync.WaitGroup{}

	wg.Add(len(resp.Messages))
	for i, m := range resp.Messages {
		go func(userId string, m *gmail.Message, idx int) {
			defer wg.Done()
			msg, err := c.service.Users.Messages.Get(userID, m.Id).Do()
			if err != nil {
				log.Println(err.Error())
				return
			}
			subject := ""
			from := ""
			for _, h := range msg.Payload.Headers {
				switch h.Name {
				case "Subject":
					subject = h.Value
				case "From":
					from = h.Value
				}

			}

			body, err := extractBody(msg.Payload, "text/html")
			if err != nil {
				fmt.Println(err.Error())
			}
			if body == "" {
				body, _ := extractBody(msg.Payload, "text/plain")
				fmt.Println("TEXT")
				messages[i] = &Email{
					Type:    "html",
					Subject: subject + " From: " + from,
					Body:    mdToHTML(body),
				}
			} else {
				messages[i] = &Email{
					Type:    "html",
					Subject: subject + " From: " + from,
					Body:    body,
				}
			}
		}(userID, m, i)
	}
	wg.Wait()

	return messages
}

func mdToHTML(md string) string {
	markdown := goldmark.New(
		goldmark.WithExtensions(extension.GFM),
		goldmark.WithParserOptions(
			parser.WithAutoHeadingID(),
		),
		goldmark.WithRendererOptions(
			html.WithHardWraps(),
			html.WithXHTML(),
		),
	)

	htmlOut := &bytes.Buffer{}

	markdown.Convert([]byte(md), htmlOut)
	return htmlOut.String()

}

func extractBody(part *gmail.MessagePart, mimeType string) (string, error) {
	if part.MimeType == mimeType && (part.Parts == nil || len(part.Parts) == 0) {
		b, _ := base64.URLEncoding.DecodeString(part.Body.Data)
		return string(b), nil
	}

	for _, subPart := range part.Parts {
		body, err := extractBody(subPart, mimeType)
		if body != "" {
			return body, err
		}
	}
	return "", nil
}
