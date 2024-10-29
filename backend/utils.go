package main

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"log"
	"time"

	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
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

func getServiceFromToken(token string) Calendar {
	service, ok := calendarCache[token]
	if !ok {
		claims, err := ValidateToken(token)
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

func UpdateSubscriptionID(email, subscriptionID string) error {
	return db.Model(&User{}).Where("email = ?", email).Update("subscription_id", subscriptionID).Error
}

func UpdateSubscriptionProvider(email, provider string) error {
	return db.Model(&User{}).Where("email = ?", email).Update("subscription_provider", provider).Error
}

func UpdateSubscriptionStatus(userEmail, newStatus, newPlan string) error {
	update := map[string]string{
		"subscription_status": newStatus,
		"subscription_plan":   newPlan,
	}
	return db.Model(&User{}).Where("email = ?", userEmail).Updates(update).Error
}

func GetUser(email string) (*User, error) {
	user := &User{}
	if err := db.Where("email = ?", email).First(user).Error; err != nil {
		return nil, err
	}
	return user, nil
}

type StateToken struct {
	Value     string
	ExpiresAt time.Time
}

func generateStateToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}
