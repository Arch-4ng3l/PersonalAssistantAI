package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/plutov/paypal/v4"
	"github.com/stripe/stripe-go/v72"
	"github.com/stripe/stripe-go/v72/charge"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
    db *gorm.DB
    oauthConf *oauth2.Config
    paypalClient *paypal.Client
)

type PayPalPaymentRequest struct {
    Amount   string `json:"amount" binding:"required"`   // e.g., "10.00"
    Currency string `json:"currency" binding:"required"` // e.g., "USD"
    Description string `json:"description"` // Optional
}

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
}


func InitOAuth(config config.Config) {
    oauthConf = &oauth2.Config{
        RedirectURL:  "http://localhost:8080/auth/google/callback",
        ClientID:     config.GoogleClientID,
        ClientSecret: config.GoogleClientSecret,
        Scopes:       []string{"https://www.googleapis.com/auth/userinfo.email"},
        Endpoint:     google.Endpoint,
    }
}

func InitPayPal(config config.Config) {
    client, err  := paypal.NewClient(config.PayPalClientID,  config.PayPalSecret, paypal.APIBaseSandBox)
    if err != nil {
        log.Fatalf("Paypal Init Error: %s\n", err.Error())
    }
    client.SetLog(os.Stdout)

    paypalClient = client
}

func CreateProduct(config SubscriptionConfig) (string, error) {
    product := paypal.Product{
        Name: config.ProductName,
        Description: config.ProductDescription,
        Type: config.ProductType,
        Category: config.ProductCategory,
    }
    createdProduct, err := paypalClient.CreateProduct(context.Background(), product)
    if err != nil {
        log.Printf("Creating Product Error: %s", err.Error());
        return "", err
    }
    return createdProduct.ID, nil
}

type SubscriptionConfig struct {
    Price string
    Currency string
    ProductName string
    ProductDescription string
    ProductType paypal.ProductType
    ProductCategory paypal.ProductCategory

    PlanName string
    PlanDescription string

    BrandName string
    Interval paypal.IntervalUnit
}

func CreateNewSubscription(config SubscriptionConfig) (string, string, error){
    productID, err := CreateProduct(config)
    if err != nil {
    }
    planID, err := CreatePlan(productID, config)
    if err != nil {
    }
    subscriptionID, approvalURL, err := CreateSubscription(planID, config)
    if err != nil {
    }
    return subscriptionID, approvalURL, nil
}

func CreatePlan(productID string, config SubscriptionConfig) (string, error) {
    plan := paypal.SubscriptionPlan{
        ID: "TEST",
        ProductId: productID,
        Name: config.PlanName,
        Description: config.PlanDescription,
        Status: paypal.SubscriptionPlanStatusActive,
        BillingCycles: []paypal.BillingCycle{
            {
                Frequency: paypal.Frequency{
                    IntervalUnit: config.Interval,
                    IntervalCount: 1,
                },
                TenureType: paypal.TenureTypeRegular,
                Sequence: 1,
                TotalCycles: 0,
                PricingScheme: paypal.PricingScheme{
                    FixedPrice: paypal.Money{
                        Value: config.Price,
                        Currency: config.Currency,
                    },
                },
            },
        },
        PaymentPreferences: &paypal.PaymentPreferences{
            AutoBillOutstanding: true,
            SetupFeeFailureAction: paypal.SetupFeeFailureActionContinue,
            PaymentFailureThreshold: 3,
        },
        Taxes: &paypal.Taxes{
            Percentage: "19.0",
            Inclusive: true,
        },
    }

    createdPlay, err := paypalClient.CreateSubscriptionPlan(context.Background(), plan)
    if err != nil {
        log.Fatalf("Creating Plan Error: %s\n", err.Error())
        return "", nil
    }
    return createdPlay.ID, nil
}


func CreateSubscription(planID string, config SubscriptionConfig) (string, string, error) {
    subscription := paypal.SubscriptionBase{
        PlanID: planID,
        ApplicationContext: &paypal.ApplicationContext{
            BrandName: config.BrandName,
            UserAction: paypal.UserActionSubscribeNow,
            ReturnURL: "http://localhost:8080/dashboard",
            CancelURL: "http://localhost:8080/cancel",
        },
    }
    createdSubscription, err := paypalClient.CreateSubscription(context.Background(), subscription)
    if err != nil {
        log.Fatalf("Create Subscription Error: %s\n", err.Error())
    }

    var approvalURL string
    for _, link := range createdSubscription.Links {
        if link.Rel == "approve" {
            approvalURL = link.Href
            break
        }
    }
    return createdSubscription.ID, approvalURL, nil
}

func PayPalReturnURL(c *gin.Context) {
    subscriptionID := c.Query("subscription_id")
    if subscriptionID == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Subscription ID was not found"})
        return
    }

    subscription, err := paypalClient.GetSubscriptionDetails(context.Background(), subscriptionID)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }
    if subscription.SubscriptionStatus == paypal.SubscriptionStatusActive {
        c.JSON(http.StatusOK, gin.H{"t": "Active"})
    } else {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Subscription Not Active"})
    }

}

func CreateSubscriptionHandler(c *gin.Context) {
    config := SubscriptionConfig{
        Price: "50.00",
        Currency: "EUR",
        ProductName: "SUB",
        PlanDescription: "IM A TEST",
        ProductCategory: paypal.ProductCategorySoftware,
        ProductType: paypal.ProductTypeService,
        
        PlanName: "SUBSCRIPTION",
        ProductDescription: "TEST",

        BrandName: "MORITZ",
        Interval: paypal.IntervalUnitYear,
    }
    subscriptionID, approvalURL, err := CreateNewSubscription(config)
    if err != nil {
        c.JSON(http.StatusOK, gin.H{"error": err.Error()})
    }
    
    c.JSON(http.StatusOK, gin.H{"subscription_id": subscriptionID, "approval_url": approvalURL})
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
        db.Create(&user)
    }

    jwtToken, err := GenerateJWT(user.Email)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Error generating token"})
        return
    }

    // Redirect to frontend with token
    c.Redirect(http.StatusTemporaryRedirect, "http://localhost:8080/dashboard?token="+jwtToken)
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

type PayPalAccessTokenResponse struct {
    Scope       string `json:"scope"`
    AccessToken string `json:"access_token"`
    TokenType   string `json:"token_type"`
    AppID       string `json:"app_id"`
    ExpiresIn   int    `json:"expires_in"`
    Nonce       string `json:"nonce"`
}





func HashPassword(password string) (string, error) {
    bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
    return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
    err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
    return err == nil
}

