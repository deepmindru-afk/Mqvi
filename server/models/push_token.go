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
	DeviceLabel *string   `json:"device_label,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// RegisterPushTokenRequest is the payload a client sends to register/refresh its token.
type RegisterPushTokenRequest struct {
	Token       string `json:"token"`
	Platform    string `json:"platform"`
	DeviceLabel string `json:"device_label,omitempty"`
}

func (r *RegisterPushTokenRequest) Validate() error {
	if r.Token == "" {
		return errors.New("token is required")
	}
	switch r.Platform {
	case "android", "ios", "web":
		return nil
	default:
		return errors.New("platform must be one of: android, ios, web")
	}
}
