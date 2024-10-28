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

type SubscriptionPlan struct {
	SubscriptionID string
	ApprovalURL    string
}

var paypalClient *paypal.Client
var subscriptionPlans map[string]*SubscriptionPlan
var subscriptionIDs map[string]string

type SubscriptionConfig struct {
	Price              string                 `json:"price"`
	Currency           string                 `json:"currency"`
	ProductName        string                 `json:"productName"`
	ProductDescription string                 `json:"productDescription"`
	ProductType        paypal.ProductType     `json:"productType"`
	ProductCategory    paypal.ProductCategory `json:"productCategory"`

	PlanName        string `json:"planName"`
	PlanDescription string `json:"planDescription"`

	BrandName string              `json:"brandName"`
	Interval  paypal.IntervalUnit `json:"interval"`
}

func InitPayPal(config config.Config) {
	client, err := paypal.NewClient(config.PayPalClientID, config.PayPalSecret, paypal.APIBaseSandBox)
	if err != nil {
		log.Fatalf("Paypal Init Error: %s\n", err.Error())
	}
	//client.SetLog(os.Stdout)

	paypalClient = client
	subscriptionPlans = make(map[string]*SubscriptionPlan)
	subscriptionIDs = make(map[string]string)
	readPayPalJson()
}

func CreateProduct(config SubscriptionConfig) (string, error) {
	product := paypal.Product{
		Name:        config.ProductName,
		Description: config.ProductDescription,
		Type:        config.ProductType,
		//Category: config.ProductCategory,
	}
	createdProduct, err := paypalClient.CreateProduct(context.Background(), product)

	if err != nil {
		log.Fatalf("Creating Product Error: %s", err.Error())
		return "", err
	}
	return createdProduct.ID, nil
}

func CreateNewSubscription(config SubscriptionConfig) (string, string, error) {
	productID, err := CreateProduct(config)
	if err != nil {
		log.Println(err.Error())
	}
	planID, err := CreatePlan(productID, config)
	if err != nil {
		log.Println(err.Error())
	}
	subscriptionID, approvalURL, err := CreateSubscription(planID, config)
	if err != nil {
		log.Println(err.Error())
	}
	return subscriptionID, approvalURL, nil
}

func CreatePlan(productID string, config SubscriptionConfig) (string, error) {
	plan := paypal.SubscriptionPlan{
		ID:          "TEST",
		ProductId:   productID,
		Name:        config.PlanName,
		Description: config.PlanDescription,
		Status:      paypal.SubscriptionPlanStatusActive,
		BillingCycles: []paypal.BillingCycle{
			{
				Frequency: paypal.Frequency{
					IntervalUnit:  config.Interval,
					IntervalCount: 1,
				},
				TenureType:  paypal.TenureTypeRegular,
				Sequence:    1,
				TotalCycles: 0,
				PricingScheme: paypal.PricingScheme{
					FixedPrice: paypal.Money{
						Value:    config.Price,
						Currency: config.Currency,
					},
				},
			},
		},
		PaymentPreferences: &paypal.PaymentPreferences{
			AutoBillOutstanding:     true,
			SetupFeeFailureAction:   paypal.SetupFeeFailureActionContinue,
			PaymentFailureThreshold: 3,
		},
		Taxes: &paypal.Taxes{
			Percentage: "19.0",
			Inclusive:  true,
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
			BrandName:  config.BrandName,
			UserAction: paypal.UserActionSubscribeNow,
			ReturnURL:  "http://localhost:8080/api/paypal-check",
			CancelURL:  "http://localhost:8080/payment",
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
	token, err := c.Cookie("token")
	if err != nil {
		log.Println(err.Error())
	}
	claims, err := ValidateToken(token)

	if err != nil {
		log.Println(err.Error())
		return
	}

	if subscriptionID == "" {
		log.Println("NO SUB ID")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Subscription ID was not found"})
		return
	}

	subscription, err := paypalClient.GetSubscriptionDetails(context.Background(), subscriptionID)
	if err != nil {
		log.Println(err.Error())
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if subscription.SubscriptionStatus == paypal.SubscriptionStatusActive {
		plan := subscriptionIDs[subscriptionID]

		err = UpdateSubscriptionStatus(claims.Email, string(subscription.SubscriptionStatus), plan)

		if err != nil {
			log.Println(err.Error())
			return
		}
		err = UpdateSubscriptionID(claims.Email, subscriptionID)
		if err != nil {
			log.Println(err.Error())
			return
		}
		err = UpdateSubscriptionProvider(claims.Email, Paypal)
		if err != nil {
			log.Println(err.Error())
			return
		}

		c.Redirect(http.StatusPermanentRedirect, "/chat")
	} else {
		log.Println("error")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Subscription Not Active"})
	}

}

func readPayPalJson() {
	file, err := os.Open("paypal.json")
	if err != nil {
	}
	var content struct {
		Items []SubscriptionConfig `json:"items"`
	}

	if err := json.NewDecoder(file).Decode(&content); err != nil {
	}
	for _, config := range content.Items {
		subscriptionID, approvalURL, _ := CreateNewSubscription(config)
		subscriptionPlans[config.PlanName] = &SubscriptionPlan{
			SubscriptionID: subscriptionID,
			ApprovalURL:    approvalURL,
		}
		subscriptionIDs[subscriptionID] = config.PlanName
	}
}

type UserSubscriptionRequest struct {
	ProductName string `json:"productName"`
}

func CreateSubscriptionHandler(c *gin.Context) {
	req := UserSubscriptionRequest{}
	if err := c.ShouldBindBodyWithJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No Valid  Product Was Given"})
	}
	subscriptionPlan, ok := subscriptionPlans[req.ProductName]

	if !ok {
		log.Println("PRODUCT NOT FOUND")
		c.JSON(http.StatusBadRequest, gin.H{"error": "Product Was Not Found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"subscription_id": subscriptionPlan.SubscriptionID, "approval_url": subscriptionPlan.ApprovalURL})
}

func CancelSubscription(subscriptionID, reason string) error {
	return paypalClient.CancelSubscription(context.Background(), subscriptionID, reason)
}

func ActivateSubscription(subscriptionID string) error {
	return paypalClient.ActivateSubscription(context.Background(), subscriptionID, "")
}

func SuspendSubscription(subscriptionID, reason string) error {
	return paypalClient.SuspendSubscription(context.Background(), subscriptionID, reason)
}
func webhookHandler(c *gin.Context) {
	var webhookEvent struct {
		EventType string `json:"event_type"`
		Resource  struct {
			SubscriptionID string `json:"id"`
			Status         string `json:"status"`
		} `json:"resource"`
	}

	if err := c.ShouldBindJSON(&webhookEvent); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid webhook payload"})
		return
	}

	// Verify webhook signature
	verifyResponse, err := paypalClient.VerifyWebhookSignature(context.Background(), c.Request, "")
	if err != nil || verifyResponse.VerificationStatus != "SUCCESS" {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid webhook signature"})
		return
	}

	// Handle different webhook events
	switch webhookEvent.EventType {
	case "BILLING.SUBSCRIPTION.CANCELLED":
		// Handle subscription cancellation
		// Update your database accordingly
	case "BILLING.SUBSCRIPTION.SUSPENDED":
		// Handle subscription suspension
	case "BILLING.SUBSCRIPTION.ACTIVATED":
		// Handle subscription activation
	case "BILLING.SUBSCRIPTION.EXPIRED":
		// Handle subscription expiration
	}

	c.JSON(http.StatusOK, gin.H{"message": "Webhook processed successfully"})
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

func CancelSubscriptionHandler(c *gin.Context) {
	token, err := c.Cookie("token")
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Authentication required"})
		return
	}

	claims, err := ValidateToken(token)

	var user User
	if err := db.Where("email = ?", claims.Email).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
		return
	}

	// Cancel the subscription
	err = CancelSubscription(user.SubscriptionID, "")
	if err != nil {
		log.Println(err.Error())
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// Update user's subscription status in your database
	err = UpdateSubscriptionStatus(claims.Email, string(paypal.SubscriptionStatusCancelled), subscriptionIDs[user.SubscriptionID])
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update subscription status"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription cancelled successfully"})
}

func SuspendSubscriptionHandler(c *gin.Context) {
	var requestData struct {
		SubscriptionID string `json:"subscription_id"`
		Reason         string `json:"reason"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	err := SuspendSubscription(requestData.SubscriptionID, requestData.Reason)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription suspended successfully"})
}

func ActivateSubscriptionHandler(c *gin.Context) {
	var requestData struct {
		SubscriptionID string `json:"subscription_id"`
	}

	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "Invalid request data"})
		return
	}

	err := ActivateSubscription(requestData.SubscriptionID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "Subscription activated successfully"})
}
