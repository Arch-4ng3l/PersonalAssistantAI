package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/plutov/paypal/v4"
	"github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/customer"
	"github.com/stripe/stripe-go/v80/subscription"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
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




func UpdateSubscriptionStatus(userEmail, newStatus , newPlan string) error {
    update := map[string]string{
        "subscription_status": newStatus,
        "subscription_plan": newPlan,
    }
    return db.Model(&User{}).Where("email = ?", userEmail).Updates(update).Error
}


func HandleAuthentication(c *gin.Context) {
    fmt.Println("AUHTRHTHT:ADJ:ASLDKJASL")
    token, err := c.Cookie("token")
    log.Println("AUTHENTICATION")
    if err != nil {
        log.Println(err.Error())
        return }

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
        log.Println("REDIRECT")
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
        t := &oauth2.Token{}
        if err := json.Unmarshal(user.CalenderToken, t); err != nil {
            return nil
        }
        switch user.Provider {
        case Microsoft: 
            service = NewMicrosoftCalendar(t)
        case Google:
            service = NewGoogleCalendar(googleOAuthConf, *user)
        default:
            return nil
        }
        calendarCache[token] = service
    }
    return service
}

func CreateEvent(c *gin.Context) {
    token, _ := c.Cookie("token")

    service := getServiceFromToken(token)
    event := Event{}
    if err := c.ShouldBindBodyWithJSON(&event); err != nil {
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



func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
    return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}
