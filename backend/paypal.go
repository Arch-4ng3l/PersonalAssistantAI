package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-gonic/gin"
	"github.com/plutov/paypal/v4"
)

var paypalClient *paypal.Client
var subscriptionPlans map[string]*SubscriptionConfig

type SubscriptionConfig struct {
    Price string `json:"price"`
    Currency string `json:"currency"`
    ProductName string  `json:"productName"`
    ProductDescription string `json:"productDescription"`
    ProductType paypal.ProductType `json:"productType"`
    ProductCategory paypal.ProductCategory  `json:"productCategory"`

    PlanName string `json:"planName"`
    PlanDescription string `json:"planDescription"`

    BrandName string `json:"brandName"`
    Interval paypal.IntervalUnit `json:"interval"`
}

func InitPayPal(config config.Config) {
    client, err  := paypal.NewClient(config.PayPalClientID,  config.PayPalSecret, paypal.APIBaseSandBox)
    if err != nil {
        log.Fatalf("Paypal Init Error: %s\n", err.Error())
    }
    client.SetLog(os.Stdout)

    paypalClient = client
    subscriptionPlans = make(map[string]*SubscriptionConfig)
    readPayPalJson()
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

func readPayPalJson() {
    file, err := os.Open("paypal.json")
    if err != nil {
    }
    var content  struct {
        Items []SubscriptionConfig `json:"items"`
    }

    if err := json.NewDecoder(file).Decode(&content); err != nil {
    }
    for _, config := range content.Items {
        subscriptionPlans[config.PlanName] = &config
    }
}

func CreateSubscriptionHandler(c *gin.Context) {
    productName := c.Query("productName")
    if productName == "" {
        c.JSON(http.StatusBadRequest, gin.H{"error": "No Valid  Product Was Given"})
    }
    config, ok := subscriptionPlans[productName]

    if !ok {
        c.JSON(http.StatusBadRequest, gin.H{"error": "Product Was Not Found"})
    }

    subscriptionID, approvalURL, err := CreateNewSubscription(*config)

    if err != nil {
        c.JSON(http.StatusOK, gin.H{"error": err.Error()})
    }
    c.JSON(http.StatusOK, gin.H{"subscription_id": subscriptionID, "approval_url": approvalURL})
}


func CancelSubscription(subscriptionID, reason string) error {
    return paypalClient.CancelSubscription(context.Background(),subscriptionID, reason)
}

func ActivateSubscription(subscriptionID string) error {
    return paypalClient.ActivateSubscription(context.Background(), subscriptionID, "")
}

func SuspendSubscription(subscriptionID, reason string) error {
    return paypalClient.SuspendSubscription(context.Background(), subscriptionID, reason)
}

func webhookHandler(c *gin.Context) {
    _, err := paypalClient.VerifyWebhookSignature(context.Background(), c.Request, "")
    if err != nil {
        return
    }
}

func verifySubscriptionHandler(c *gin.Context) {
    var requestData struct {
        SubscriptionID string `json:"subscription_id"`
    }
    err := c.ShouldBindBodyWithJSON(&requestData)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }


    // Get subscription details
    subscription, err := paypalClient.GetSubscriptionDetails(context.Background(), requestData.SubscriptionID)
    if err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    // Send the subscription status back to the client
    response := map[string]string{
        "status": string(subscription.SubscriptionStatus),
    }

    c.JSON(http.StatusOK, response)
}

