package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
    DBUser string
    DBPassword string
    DBName string
    DBHost string
    DBPort string
    JWTSecret string
    GoogleClientID string
    GoogleClientSecret string

    MicrosoftClientID string
    MicrosoftClientSecret string
    MicrosoftTenantID string

    StripeSecretKey string
    StripePublishableKey string

    PayPalClientID string
    PayPalSecret string
    PayPalWebhookID string
    PayPalMode string
    OpenAISecret string
    GeminiAISecret string

}


func LoadConfig() Config {
    err := godotenv.Load()
    if err != nil {
        log.Println("No .env file found")
    }

    return Config{
        DBUser:         os.Getenv("DB_USER"),
        DBPassword:     os.Getenv("DB_PASSWORD"),
        DBName:         os.Getenv("DB_NAME"),
        DBHost:         os.Getenv("DB_HOST"),
        DBPort:         os.Getenv("DB_PORT"),
        JWTSecret:      os.Getenv("JWT_SECRET"),
        GoogleClientID:     os.Getenv("GOOGLE_CLIENT_ID"),
        GoogleClientSecret: os.Getenv("GOOGLE_CLIENT_SECRET"),
        MicrosoftClientID:     os.Getenv("MICROSOFT_CLIENT_ID"),
        MicrosoftClientSecret: os.Getenv("MICROSOFT_CLIENT_SECRET"),
        MicrosoftTenantID: os.Getenv("MICROSOFT_TENANT_ID"),
        StripeSecretKey:    os.Getenv("STRIPE_SECRET_KEY"),
        StripePublishableKey: os.Getenv("STRIPE_PUBLISHABLE_KEY"),
        PayPalClientID:       os.Getenv("PAYPAL_CLIENT_ID"),
        PayPalSecret:         os.Getenv("PAYPAL_SECRET"),
        PayPalMode:           os.Getenv("PAYPAL_MODE"),
        PayPalWebhookID: os.Getenv("PAYPAL_WEBHOOK_ID"),
        OpenAISecret: os.Getenv("OPENAI_SECRET_KEY"),
        GeminiAISecret: os.Getenv("GEMINI_SECRET_KEY"),
    }
}
