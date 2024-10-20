package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Azure/azure-sdk-for-go/sdk/azcore"
	"github.com/Azure/azure-sdk-for-go/sdk/azcore/policy"
	"github.com/coreos/go-oidc"
	abstractions "github.com/microsoft/kiota-abstractions-go"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	msgraphsdk "github.com/microsoftgraph/msgraph-sdk-go"
	"github.com/microsoftgraph/msgraph-sdk-go/models/odataerrors"
	"github.com/plutov/paypal/v4"
	"github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/customer"
	"github.com/stripe/stripe-go/v80/subscription"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"golang.org/x/oauth2/microsoft"
	"google.golang.org/api/calendar/v3"
)

var (
    db *gorm.DB
    googleOAuthConf *oauth2.Config
    microsoftOAuthConf *oauth2.Config
    microsoftProvider *oidc.Provider

    calendarCache map[string]Calendar

    geminiClient *genai.Client
    conversationsCache map[string]*genai.ChatSession
)

func InitDB(config config.Config) {
    var err error
    dbURI := fmt.Sprintf("host=%s user=%s dbname=%s password=%s port=%s sslmode=disable",
            config.DBHost, config.DBUser, config.DBName, config.DBPassword, config.DBPort,
        )
    db, err = gorm.Open("postgres", dbURI)
    if err != nil {
        panic("failed to connect to databse")
    }
    db.AutoMigrate(&User{})
    calendarCache = make(map[string]Calendar)
    conversationsCache = make(map[string]*genai.ChatSession)
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
            "Calendars.Read",
        },
        Endpoint: microsoft.AzureADEndpoint("common"),
    }
}


func InitOAuth(config config.Config) {
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

func UpdateSubscriptionStatus(userEmail, newStatus , newPlan string) error {
    update := map[string]string{
        "subscription_status": newStatus,
        "subscription_plan": newPlan,
    }
    return db.Model(&User{}).Where("email = ?", userEmail).Updates(update).Error
}


func HandleAuthentication(c *gin.Context) {
    token, err := c.Cookie("token")
    if err != nil {
        log.Println(err.Error())
        return
    }

    claims, err := ValidateToken(token)
    if err != nil {
        log.Println(err.Error())
        return
    }
    user := &User{}

    if err := db.Where("email = ?", claims.Email).First(user).Error; err != nil {
        log.Println(err.Error())
        return 
    }

    if user.SubscriptionStatus != string(paypal.SubscriptionStatusActive) &&  user.SubscriptionStatus != string(stripe.SubscriptionStatusActive) {
        c.Redirect(http.StatusPermanentRedirect, "/payment")
        return
    }

    c.HTML(http.StatusOK, "chat.html", nil)

}

func Register(c *gin.Context) {
    var json struct {
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required,min=6` 
    }

    if err := c.ShouldBindJSON(&json); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return 
    }

    hashedPassword, err := HashPassword(json.Password)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Error hashing password"})
        return
    }

    user := User{Email: json.Email, Password: hashedPassword}
    if err := db.Create(&user).Error; err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": "User already exists"})
        return
    }

    token, err := GenerateJWT(user.Email) 
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"token": token})
}

func Login(c *gin.Context) {
    var json struct {
        Email    string `json:"email" binding:"required,email"`
        Password string `json:"password" binding:"required"`
    }

    if err := c.ShouldBindJSON(&json); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    var user User
    if err := db.Where("email = ?", json.Email).First(&user).Error; err != nil {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
        return
    }

    if !CheckPasswordHash(json.Password, user.Password) {
        c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid credentials"})
        return
}

    token, err := GenerateJWT(user.Email)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
        return
    }

    c.JSON(http.StatusOK, gin.H{"token": token})
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

func MicrosoftLogin(c *gin.Context) {
    url := microsoftOAuthConf.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
    c.Redirect(http.StatusTemporaryRedirect, url)
}

func MicrosoftCallback(c *gin.Context) {
    log.Println("CALLBACK")
    code := c.Query("code")    

    token, err := microsoftOAuthConf.Exchange(context.Background(), code)
    if err != nil {
        log.Println("Error1 : ", err.Error())
        return
    }


    user := &User{}

    client, err := msgraphsdk.NewGraphServiceClientWithCredentials(&TokenCredential{token: token}, []string{
        "email",
        "Calenders.Read",
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

func Payment(c *gin.Context) {
    token, _ := c.Cookie("token")
    claims, _ := ValidateToken(token)
    var json struct {
        Token  string `json:"token" binding:"required"`
        Amount int64  `json:"amount" binding:"required"`
    }

    if err := c.ShouldBindJSON(&json); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
    customerParams := &stripe.CustomerParams{
        Email: stripe.String(claims.Email),
    }
    customerStripe, err := customer.New(customerParams)

    params := &stripe.SubscriptionParams{
        Currency:     stripe.String(string(stripe.CurrencyEUR)),
        Description:  stripe.String("Premium Subscription"),
        Items: []*stripe.SubscriptionItemsParams{
            {
                Price: stripe.String("20.0"),
            },
        },
        Customer: stripe.String(customerStripe.ID),
    }

    sub, err := subscription.New(params)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if err != UpdateSubscriptionStatus(claims.Email, string(sub.Status), Premium) {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": sub.Status})
}


func FetchCalenderData(c *gin.Context) {
    token, _ := c.Cookie("token")
    service := getServiceFromToken(token)
    events, _ := service.GetEvents(time.Now(), time.Now().Add(time.Hour * 24 * 7))
    c.JSON(http.StatusOK, gin.H{"items": events})
}

func getServiceFromToken(token string) Calendar {
    service, ok := calendarCache[token]
    if !ok {
        claims, err:= ValidateToken(token)
        if err != nil {
            log.Println(err.Error())
        }
        user := &User{}
        if err := db.Where("email = ?", claims.Email).First(user).Error; err != nil {
            log.Fatal(err)
        }
        service = NewGoogleCalendar(googleOAuthConf, *user)
        calendarCache[token] = service
    }
    return service
}

func CreateEvent(c *gin.Context) {
    token, _ := c.Cookie("token")

    service := getServiceFromToken(token)
    event := Event{}
    if err := c.ShouldBindBodyWithJSON(event); err != nil {
        log.Fatalln(err.Error())
    }

    service.CreateEvent(event)

    //session, ok := conversationsCache[token]

    //if !ok {
    //    return
    //}
    //UpdateSchedule(session, event, "Create")
}

func RemoveEvent(c *gin.Context) {
    token, _ := c.Cookie("token")

    service := getServiceFromToken(token)


    event := Event{}

    if err := c.ShouldBindBodyWithJSON(event); err != nil {
        log.Fatal(err)
    }
    service.RemoveEvent(event)

    //session, ok := conversationsCache[token]

    //if !ok {
    //    return
    //}
    //UpdateSchedule(session, event, "Remove")
}

func GetUserPlan(token string) string {
    claims, err := ValidateToken(token)
    if err != nil {
        log.Println(err.Error())
        return ""
    }

    user := &User{}
    err = db.Where("email = ?", claims.Email).First(user).Error

    if err != nil {
        log.Println(err.Error())
        return ""
    }
    return user.SubscriptionPlan
}


func AIChat(c *gin.Context) {
    token, err := c.Cookie("token")
    service := getServiceFromToken(token)

    session, ok := conversationsCache[token]

    if !ok {
        events, _ := service.GetEvents(time.Now(), time.Now().Add(time.Hour * 24 * 7))
        eventStr := ""
        for _, event := range events {
            eventStr += fmt.Sprint(event.Title, " start: ",event.StartTime, "end: ",  event.EndTime, ",")

        }
        plan := GetUserPlan(token)
        session = StartChatSession(geminiClient, eventStr, plan)
        conversationsCache[token] = session
    }

    var message struct {
        Content string `json:"message"`
    }

    if err := c.ShouldBindBodyWithJSON(&message); err != nil {
        log.Println(err.Error())
        return
    }

    response, err := SendGeminiMessage(session, message.Content)

    if err != nil {
        log.Println(err.Error())
        return
    }

    c.JSON(http.StatusOK, gin.H{"reply": response})
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

func GetMicrosoftCalendar(token *oauth2.Token) {

    cred := TokenCredential{token}

    client, err := msgraphsdk.NewGraphServiceClientWithCredentials(&cred, []string {
        "User.Read",
        "Calendars.ReadWrite",
    })
    if err != nil {
        log.Println(err.Error())
        return 
    }

    user, err := client.Me().Get(context.Background(), nil)
    if err != nil {
        log.Println(err.Error())
        return
    }
    log.Println("USER: ", user)

    headers := abstractions.NewRequestHeaders()
    headers.Add("Prefer", "outlook.timezone=\"Pacific Standard Time\"")


    req, err:= client.Me().Calendar().ToGetRequestInformation(context.Background(), nil)
    
    httpClient := http.Client{}
    a, err := client.RequestAdapter.ConvertToNativeRequest(context.Background(), req)
    reqHTTP := a.(*http.Request)
    resp, err := httpClient.Do(reqHTTP)
    log.Println(resp)
    s, _ := io.ReadAll(resp.Body)
    log.Println(string(s))

    
    if err != nil {
        switch err.(type) {
        case *odataerrors.ODataError:
            typed := err.(*odataerrors.ODataError)
            fmt.Println("error: ", typed.Error())
            if terr := typed.GetErrorEscaped(); terr != nil  {
                fmt.Println(*terr.GetCode())
                fmt.Println(*terr.GetMessage())
            }
        default:
            fmt.Printf("%T %s", err, err.Error())
        }
        return
    }


    
    
}

func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
    return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
