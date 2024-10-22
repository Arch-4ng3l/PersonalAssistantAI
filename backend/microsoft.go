package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/coreos/go-oidc"
	"github.com/gin-gonic/gin"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)


type MicrosoftCalendar struct {
    client *msgraphsdk.GraphServiceClient
}

type TokenCredential struct {
    token *oauth2.Token
}


func (token *TokenCredential) GetToken(ctx context.Context, options policy.TokenRequestOptions) (azcore.AccessToken, error) {
    if token.token.Valid() {
        return azcore.AccessToken{
            Token: token.token.AccessToken,
            ExpiresOn: time.Now().Add(time.Duration(token.token.ExpiresIn)),
        }, nil
    }

    return azcore.AccessToken{}, fmt.Errorf("token not valid")
}

func (c *MicrosoftCalendar) GetEvents(startTime, endTime time.Time) ([]*Event, error) {
    events, err := c.client.Me().Calendar().Events().Get(context.Background(), nil)
    if err != nil {
        return nil, err
    }
    layout := "2006-01-02T15:04:05.0000000"
    var arr []*Event
    for _, event := range events.GetValue() {
        t, _:= time.Parse(layout, *event.GetStart().GetDateTime())
        t2, _ := time.Parse(layout, *event.GetEnd().GetDateTime())
        if t.UTC().Unix() < startTime.UTC().Unix() || t2.UTC().Unix() > endTime.UTC().Unix() {
            continue
        }

        arr = append(arr, &Event{
            ID: *event.GetId(),
            Title: *event.GetSubject(),
            StartTime: *event.GetStart().GetDateTime(),
            EndTime: *event.GetEnd().GetDateTime(),
        })
    }
    return arr, nil

}
func (c *MicrosoftCalendar) CreateEvent(event Event) error {
    microsoftEvent := models.NewEvent()
    microsoftEvent.SetSubject(&event.Title)
    t, _:= time.Parse(time.RFC3339, event.StartTime)
    timeZone, _ := t.Zone()
    startTime := models.NewDateTimeTimeZone()
    startTime.SetDateTime(&event.StartTime)
    startTime.SetTimeZone(&timeZone)

    endTime := models.NewDateTimeTimeZone()
    endTime.SetDateTime(&event.EndTime)
    endTime.SetTimeZone(&timeZone)
    microsoftEvent.SetStart(startTime)
    microsoftEvent.SetEnd(endTime)

    _, err := c.client.
        Me().
        Calendar().
        Events().
        Post(context.Background(), microsoftEvent, nil)

    return err
}
func (c *MicrosoftCalendar) RemoveEvent(event Event) error {
    return c.client.
        Me().
        Calendar().
        Events().
        ByEventId(event.ID).
        Delete(context.Background(), nil)
}

func NewMicrosoftCalendar(token *oauth2.Token) *MicrosoftCalendar{

    cred := TokenCredential{token}

    client, err := msgraphsdk.NewGraphServiceClientWithCredentials(&cred, []string {
        "User.Read",
        "Calendars.ReadWrite",
    })
    if err != nil {
        log.Println(err.Error())
        return  nil
    }
    
    return &MicrosoftCalendar{
        client: client,
    }
}


func InitMicrosoft(config config.Config) {
    microsoftOAuthConf = &oauth2.Config{
        ClientID: config.MicrosoftClientID,
        ClientSecret: config.MicrosoftClientSecret,
        Scopes: []string{
            oidc.ScopeOpenID,
            oidc.ScopeOfflineAccess,
            "User.Read",
            "profile",
            "email",
            "Calendars.ReadWrite",
        },
        Endpoint: microsoft.AzureADEndpoint("common"),
    }
}

func MicrosoftLogin(c *gin.Context) {
    url := microsoftOAuthConf.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
    c.Redirect(http.StatusTemporaryRedirect, url)
}

func MicrosoftCallback(c *gin.Context) {
    code := c.Query("code")    

    token, err := microsoftOAuthConf.Exchange(context.Background(), code)
    if err != nil {
        log.Println("Error1 : ", err.Error())
        return
    }

    user := &User{}

    client, err := msgraphsdk.NewGraphServiceClientWithCredentials(&TokenCredential{token: token}, []string{
        "email",
    })

    if err != nil {
        log.Println("Error2: ", err.Error())
        return
    }
    userResp, err := client.Me().Get(context.Background(), nil)

    if err != nil {
        log.Println(err.Error())
        return 
    }

    user.ProviderID = *userResp.GetId()
    user.Email = *userResp.GetMail()
    user.Provider = Microsoft

    if err := db.Where("provider_id = ?", user.ProviderID).First(&user).Error; err != nil {
        tokenJSON, _ := json.Marshal(token)
        user.CalenderToken = tokenJSON
        if err := db.Create(&user).Error; err != nil {
            log.Println(err.Error())
            return
        }

        c.Redirect(http.StatusTemporaryRedirect, "http://localhost:8080/payment")
    }

    jwtToken, err := GenerateJWT(user.Email)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
        log.Println(err.Error())
        return
    }

    c.SetCookie("token", jwtToken, int(time.Now().Add(time.Hour * 24 * 7).Unix()), "/", "", true, false)

    c.Redirect(http.StatusTemporaryRedirect, "http://localhost:8080/chat")

}
