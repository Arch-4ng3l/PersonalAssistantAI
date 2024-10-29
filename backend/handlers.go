package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/coreos/go-oidc"

	"strconv"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-gonic/gin"
	"github.com/google/generative-ai-go/genai"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/plutov/paypal/v4"
	"github.com/stripe/stripe-go/v80"
	"github.com/stripe/stripe-go/v80/customer"
	"github.com/stripe/stripe-go/v80/subscription"
	"golang.org/x/net/context"
	"golang.org/x/oauth2"
)

var (
	db                 *gorm.DB
	googleOAuthConf    *oauth2.Config
	microsoftOAuthConf *oauth2.Config
	microsoftProvider  *oidc.Provider

	calendarCache map[string]Calendar

	geminiClient       *genai.Client
	conversationsCache map[string]*genai.ChatSession
)

type HTTPHandlerFunction func(c *gin.Context) error

func HandleError(handler HTTPHandlerFunction) gin.HandlerFunc {
	return func(c *gin.Context) {
		if err := handler(c); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			log.Println(err.Error())
		}
	}
}

func InitDB(config config.Config) {
	var err error
	dbURI := fmt.Sprintf("host=%s user=%s dbname=%s password=%s port=%s sslmode=disable",
		config.DBHost, config.DBUser, config.DBName, config.DBPassword, config.DBPort,
	)
	db, err = gorm.Open("postgres", dbURI)
	if err != nil {
		log.Fatalln("failed to connect to databse")
	}
	db.AutoMigrate(&User{})
	calendarCache = make(map[string]Calendar)
	conversationsCache = make(map[string]*genai.ChatSession)
}

func HandleAuthentication(c *gin.Context) error {
	token, err := c.Cookie("token")
	log.Println("AUTHENTICATION")
	if err != nil {
		return err
	}

	claims, err := ValidateToken(token)
	if err != nil {
		return err
	}
	user := &User{}

	if err := db.Where("email = ?", claims.Email).First(user).Error; err != nil {
		return err
	}

	if user.SubscriptionStatus != string(paypal.SubscriptionStatusActive) && user.SubscriptionStatus != string(stripe.SubscriptionStatusActive) {
		c.Redirect(http.StatusPermanentRedirect, "/payment")
		return nil
	}
	log.Println("NO REDIRECT")

	c.HTML(http.StatusOK, "chat.html", nil)
	return nil

}

func Register(c *gin.Context) error {
	var json struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required,min=6`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return err
	}

	hashedPassword, err := HashPassword(json.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error hashing password"})
		return err
	}

	user := User{Email: json.Email, Password: hashedPassword}
	if err := db.Create(&user).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "User already exists"})
		return err
	}

	token, err := GenerateJWT(user.Email)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
		return err
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
	return nil
}

func Login(c *gin.Context) error {
	var json struct {
		Email    string `json:"email" binding:"required,email"`
		Password string `json:"password" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		return err
	}

	user, err := GetUser(json.Email)
	if err != nil {
		return err
	}

	if !CheckPasswordHash(json.Password, user.Password) {
		return err
	}

	token, err := GenerateJWT(user.Email)
	if err != nil {
		return err
	}

	c.JSON(http.StatusOK, gin.H{"token": token})
	return nil
}

func Payment(c *gin.Context) error {
	token, _ := c.Cookie("token")
	claims, _ := ValidateToken(token)
	var json struct {
		Token  string `json:"token" binding:"required"`
		Amount int64  `json:"amount" binding:"required"`
	}

	if err := c.ShouldBindJSON(&json); err != nil {
		return err
	}

	stripe.Key = os.Getenv("STRIPE_SECRET_KEY")
	customerParams := &stripe.CustomerParams{
		Email: stripe.String(claims.Email),
	}
	customerStripe, err := customer.New(customerParams)

	params := &stripe.SubscriptionParams{
		Currency:    stripe.String(string(stripe.CurrencyEUR)),
		Description: stripe.String("Premium Subscription"),
		Items: []*stripe.SubscriptionItemsParams{
			{
				Price: stripe.String("20.0"),
			},
		},
		Customer: stripe.String(customerStripe.ID),
	}

	sub, err := subscription.New(params)
	if err != nil {
		return err
	}
	if err != UpdateSubscriptionStatus(claims.Email, string(sub.Status), Premium) {
		return err
	}

	c.JSON(http.StatusOK, gin.H{"status": sub.Status})
	return nil
}

func FetchCalenderData(c *gin.Context) error {
	token, _ := c.Cookie("token")
	service := getServiceFromToken(token)
	events, _ := service.GetEvents(time.Now(), time.Now().Add(time.Hour*24*7))
	c.JSON(http.StatusOK, gin.H{"items": events})
	return nil
}

func CreateEvent(c *gin.Context) error {
	token, _ := c.Cookie("token")

	service := getServiceFromToken(token)
	event := Event{}
	if err := c.ShouldBindBodyWithJSON(&event); err != nil {
		return err
	}

	service.CreateEvent(event)
	return nil

}

func RemoveEvent(c *gin.Context) error {
	token, _ := c.Cookie("token")

	service := getServiceFromToken(token)

	event := Event{}

	if err := c.ShouldBindBodyWithJSON(&event); err != nil {
		return err
	}
	service.RemoveEvent(event)
	return nil

}

func AIChat(c *gin.Context) error {
	token, err := c.Cookie("token")
	service := getServiceFromToken(token)

	session, ok := conversationsCache[token]

	if !ok || session == nil {
		events, _ := service.GetEvents(time.Now(), time.Now().Add(time.Hour*24*7))
		eventStr := ""
		for _, event := range events {
			eventStr += fmt.Sprint(event.Title, " start: ", event.StartTime, "end: ", event.EndTime, ",")
		}
		plan := GetUserPlan(token)
		log.Println(plan)
		session = StartChatSession(geminiClient, eventStr, plan)
		conversationsCache[token] = session
	}

	var message struct {
		Content string `json:"message"`
	}

	if err := c.ShouldBindBodyWithJSON(&message); err != nil {
		return err
	}

	response, err := SendGeminiMessage(session, message.Content)

	if err != nil {
		return err
	}

	log.Println(response)
	c.JSON(http.StatusOK, response)
	return nil
}

func GetEmail(c *gin.Context) error {

	token, _ := c.Cookie("token")

	t, _ := ValidateToken(token)
	user, err := GetUser(t.Email)

	if err != nil {
		return err
	}

	oauthToken := &oauth2.Token{}
	json.Unmarshal(user.CalenderToken, oauthToken)
	service := NewGoogleMail(oauthToken)
	emails := service.GetEmails(time.Now().AddDate(0, 0, -2), time.Now(), user.ProviderID)
	c.JSON(http.StatusOK, gin.H{"items": emails})
	return nil
}

func GetSubscriptionDetails(c *gin.Context) error {
	token, err := c.Cookie("token")
	if err != nil {
		return err
	}

	claims, err := ValidateToken(token)
	if err != nil {
		return err
	}

	user, err := GetUser(claims.Email)
	if err != nil {
		return err
	}

	// Get subscription details from PayPal
	subscription, err := paypalClient.GetSubscriptionDetails(context.Background(), user.SubscriptionID)
	if err != nil {
		return err
	}

	// Parse the next billing date
	nextBillingTime := subscription.BillingInfo.NextBillingTime
	if err != nil {
		nextBillingTime = time.Now() // Fallback to current time if parsing fails
	}

	amount := 0.0
	amount, _ = strconv.ParseFloat(subscription.BillingInfo.LastPayment.Amount.Value, 64)

	details := SubscriptionDetails{
		Status:          string(subscription.SubscriptionStatus),
		NextBillingDate: nextBillingTime,
		Amount:          amount,
		Provider:        "paypal",
		SubscriptionID:  user.SubscriptionID,
	}

	c.JSON(http.StatusOK, details)
	return nil
}
