package main

import (
	"encoding/gob"
	"net/http"

	"github.com/Arch-4ng3l/StartupFramework/backend/config"
	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/cookie"
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
	store := cookie.NewStore([]byte(cfg.JWTSecret))
	gob.Register(StateToken{})
	r.Use(sessions.Sessions("session", store))

	r.Static("/static", "../frontend/static")
	r.LoadHTMLGlob("../frontend/templates/*")

	api := r.Group("/api")
	{
		api.POST("/register", HandleError(Register))
		api.POST("/login", HandleError(Login))
		api.GET("/payment", HandleError(Payment))
		api.POST("/paypal", HandleError(CreateSubscriptionHandler))
		api.POST("/calendar-create", HandleError(CreateEvent))
		api.POST("/calendar-remove", HandleError(RemoveEvent))
		api.GET("/calendar-load", HandleError(FetchCalenderData))
		api.POST("/ai-chat", HandleError(AIChat))
		api.GET("/paypal-check", HandleError(PayPalReturnURL))
		api.GET("/email", HandleError(GetEmail))
		api.POST("/paypal-webhook", HandleError(webhookHandler))
		api.POST("/paypal-cancel", HandleError(CancelSubscriptionHandler))
		api.POST("/paypal-activate", HandleError(ActivateSubscriptionHandler))
		api.POST("/paypal-suspend", HandleError(SuspendSubscriptionHandler))
		api.GET("/subscription-status", HandleError(GetSubscriptionDetails))
	}

	// OAuth routes
	r.GET("/auth/google/login", HandleError(GoogleLogin))
	r.GET("/auth/google/callback", HandleError(GoogleCallback))

	r.GET("/auth/microsoft/login", HandleError(MicrosoftLogin))
	r.GET("/auth/microsoft/callback", HandleError(MicrosoftCallback))

	// Serve frontend pages
	r.GET("/", func(c *gin.Context) {
		c.HTML(http.StatusOK, "index.html", nil)
	})

	r.GET("/chat", HandleError(HandleAuthentication))

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
