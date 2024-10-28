package main

import (
	"net/http"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-gonic/gin"
)

func main() {
	cfg := config.LoadConfig()
	jwtKey = []byte(cfg.JWTSecret)

	InitDB(cfg)
	InitGoogle(cfg)
	InitPayPal(cfg)
	InitMicrosoft(cfg)
	geminiClient = GetGeminiClient(cfg)

	r := gin.Default()

	r.Static("/static", "../frontend/static")
	r.LoadHTMLGlob("../frontend/templates/*")

	api := r.Group("/api")
	{
		api.POST("/register", Register)
		api.POST("/login", Login)
		api.GET("/payment", Payment)
		api.POST("/paypal", CreateSubscriptionHandler)
		api.POST("/calendar-create", CreateEvent)
		api.POST("/calendar-remove", RemoveEvent)
		api.GET("/calendar-load", FetchCalenderData)
		api.POST("/ai-chat", AIChat)
		api.GET("/paypal-check", PayPalReturnURL)
		api.GET("/email", GetEmail)
		api.POST("/paypal-webhook", webhookHandler)
		api.POST("/paypal-cancel", CancelSubscriptionHandler)
		api.POST("/paypal-activate", ActivateSubscriptionHandler)
		api.POST("/paypal-suspend", SuspendSubscriptionHandler)
		api.GET("/subscription-status", GetSubscriptionDetails)
	}

	// OAuth routes
	r.GET("/auth/google/login", GoogleLogin)
	r.GET("/auth/google/callback", GoogleCallback)

	r.GET("/auth/microsoft/login", MicrosoftLogin)
	r.GET("/auth/microsoft/callback", MicrosoftCallback)

	// Serve frontend pages
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.GET("/chat", HandleAuthentication)

	r.GET("/login", func(c *gin.Context) {
		c.HTML(http.StatusOK, "login.html", nil)
	})
	r.GET("/register", func(c *gin.Context) {
		c.HTML(http.StatusOK, "register.html", nil)
	})
	r.GET("/payment", func(c *gin.Context) {
		c.HTML(http.StatusOK, "payment.html", nil)

	})
	r.GET("/email", func(c *gin.Context) {
		c.HTML(http.StatusOK, "email.html", nil)
	})

	r.Run(":8080") // Listen and serve on 0.0.0.0:8080
}
