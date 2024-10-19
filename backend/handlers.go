package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/charge"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/api/calendar/v3"
)

var (
    db *gorm.DB
    oauthConf *oauth2.Config
    calendarCache map[string]*calendar.Service
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
    calendarCache = make(map[string]*calendar.Service)
    conversationsCache = make(map[string]*genai.ChatSession)
}


func InitOAuth(config config.Config) {
    oauthConf = &oauth2.Config{
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
    url := oauthConf.AuthCodeURL("state-token", oauth2.AccessTypeOffline)
    c.Redirect(http.StatusTemporaryRedirect, url)
}


func GoogleCallback(c *gin.Context) {
    code := c.Query("code")
    if code == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Code not provided"})
        return
    }

    token, err := oauthConf.Exchange(context.Background(), code)
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
    if err := db.Where("google_id = ?", userInfo.ID).First(&user).Error; err != nil {
        // If not, create
        user = User{Email: userInfo.Email, GoogleID: userInfo.ID}
        tokenJSON, _ := json.Marshal(token)
        user.CalenderToken = tokenJSON
        if err := db.Create(&user).Error; err != nil {
            log.Println(err.Error())
        }
    }



    jwtToken, err := GenerateJWT(user.Email)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
        log.Println(err.Error())
        return
    }
    service := GetCalender(oauthConf, user)
    calendarCache[jwtToken] = service

    // Redirect to frontend with token
    c.SetCookie("token", jwtToken, int(time.Now().Add(time.Hour * 24 * 7).Unix()), "/", "", true, false)
    c.Redirect(http.StatusTemporaryRedirect, "http://localhost:8080/chat")
}

func Payment(c *gin.Context) {
    var json struct {
        Token  string `json:"token" binding:"required"`
        Amount int64  `json:"amount" binding:"required"`
    }

    if err := c.ShouldBindJSON(&json); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    stripe.Key = os.Getenv("STRIPE_SECRET_KEY")

    params := &stripe.ChargeParams{
        Amount:       stripe.Int64(json.Amount),
        Currency:     stripe.String(string(stripe.CurrencyEUR)),
        Description:  stripe.String("Payment from Go App"),
        Source:       &stripe.SourceParams{
            Token: stripe.String(json.Token),
        },
    }

    charge, err := charge.New(params)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    c.JSON(http.StatusOK, gin.H{"status": charge.Status})
}


func FetchCalenderData(c *gin.Context) {
    token, _ := c.Cookie("token")
    service := getServiceFromToken(token)
    evenst, _ := service.Events.List("primary").TimeMin(time.Now().Format(time.RFC3339)).Do()
    c.JSON(http.StatusOK, gin.H{"items": evenst.Items})
}

func getServiceFromToken(token string) *calendar.Service {
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
        service = GetCalender(oauthConf, *user)
        calendarCache[token] = service
    }
    return service
}

func CreateEvent(c *gin.Context) {
    token, _ := c.Cookie("token")

    service := getServiceFromToken(token)
    event := &calendar.Event{}
    if err := c.ShouldBindBodyWithJSON(event); err != nil {
        log.Fatalln(err.Error())
    }
    service.Events.Insert("primary", event).Do()

    session, ok := conversationsCache[token]

    if !ok {
        return
    }
    UpdateSchedule(session, event, "Create")
}

func RemoveEvent(c *gin.Context) {
    token, _ := c.Cookie("token")

    service := getServiceFromToken(token)


    event := &calendar.Event{}

    if err := c.ShouldBindBodyWithJSON(event); err != nil {
        log.Fatal(err)
    }

    if err := service.Events.Delete("primary", event.Id).Do(); err != nil {
        log.Println(err.Error())
    }
    session, ok := conversationsCache[token]

    if !ok {
        return
    }
    UpdateSchedule(session, event, "Remove")
}


func AIChat(c *gin.Context) {
    token, err := c.Cookie("token")
    service := getServiceFromToken(token)

    evenst, _ := service.Events.List("primary").TimeMin(time.Now().Format(time.RFC3339)).Do()
    eventStr := ""
    for _, event := range evenst.Items {
        eventStr += fmt.Sprint(event.Summary, " start: ",event.Start.DateTime, "end: ",  event.End.DateTime, ",")
    }
    fmt.Println(eventStr)

    if err != nil {
        log.Println(err.Error())
        return
    }

    var message struct {
        Content string `json:"message"`
    }

    if err := c.ShouldBindBodyWithJSON(&message); err != nil {
        log.Println(err.Error())
        return
    }

    session, ok := conversationsCache[token]

    if !ok {
        session = StartChatSession(geminiClient, eventStr)
        conversationsCache[token] = session
    }

    response, err := SendGeminiMessage(session, message.Content)

    if err != nil {
        log.Println(err.Error())
        return
    }

    c.JSON(http.StatusOK, gin.H{"reply": response})
}

func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
    return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}



