package models

import (
	"errors"
	"time"
)

// PushToken is a device's FCM registration token used to deliver push notifications.
type PushToken struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Token       string    `json:"token"`
	Platform    string    `json:"platform"`
	TokenType   string    `json:"token_type"` // "fcm" | "apns_voip"
	DeviceLabel *string   `json:"device_label,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// Push token types.
const (
	PushTokenTypeFCM      = "fcm"       // FCM registration token (Android + iOS messages)
	PushTokenTypeAPNsVoIP = "apns_voip" // iOS PushKit VoIP token (calls, via direct APNs)
)

// RegisterPushTokenRequest is the payload a client sends to register/refresh its token.
type RegisterPushTokenRequest struct {
	Token       string `json:"token"`
	Platform    string `json:"platform"`
	TokenType   string `json:"token_type,omitempty"` // defaults to "fcm"
	DeviceLabel string `json:"device_label,omitempty"`
}

func (r *RegisterPushTokenRequest) Validate() error {
	if r.Token == "" {
		return errors.New("token is required")
	}
	switch r.Platform {
	case "android", "ios", "web":
	default:
		return errors.New("platform must be one of: android, ios, web")
	}
	switch r.TokenType {
	case "", PushTokenTypeFCM, PushTokenTypeAPNsVoIP:
		return nil
	default:
		return errors.New("token_type must be fcm or apns_voip")
	}
}
