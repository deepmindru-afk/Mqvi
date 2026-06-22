package services

import (
	"context"
	"log"
	"time"

	"github.com/akinalp/mqvi/models"
	"github.com/akinalp/mqvi/pkg/i18n"
	"github.com/akinalp/mqvi/pkg/push"
	"github.com/akinalp/mqvi/repository"
	"github.com/akinalp/mqvi/ws"
)

// PushNotifier sends mobile push notifications to a user's offline devices.
// Consumed by DMService and P2PCallService via SetPushNotifier.
type PushNotifier interface {
	NotifyDM(recipientID, senderName, content string, encrypted bool, dmChannelID, senderID string)
	NotifyCall(receiverID, callerName string, callType models.P2PCallType, callID, callerID string)
}

// pushBodyMaxLen caps the notification body so a long message doesn't overflow
// the OS notification.
const pushBodyMaxLen = 140

// pushUserLookup is the minimal user-read interface push needs (recipient language).
type pushUserLookup interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
}

type pushService struct {
	sender    push.Sender
	tokenRepo repository.PushTokenRepository
	users     pushUserLookup
	presence  ws.UserStateProvider
}

func NewPushService(sender push.Sender, tokenRepo repository.PushTokenRepository, users pushUserLookup, presence ws.UserStateProvider) PushNotifier {
	return &pushService{sender: sender, tokenRepo: tokenRepo, users: users, presence: presence}
}

func (s *pushService) NotifyDM(recipientID, senderName, content string, encrypted bool, dmChannelID, senderID string) {
	if !s.sender.Enabled() {
		return
	}
	go func() {
		// Online (any WS connection) -> handled in-app, skip before any DB work.
		if s.isOnline(recipientID) {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		loc := i18n.NewLocalizer(s.recipientLang(ctx, recipientID))

		// Plaintext shows content; E2EE can't be read by the server -> generic.
		body := loc.T("push.newMessage")
		if !encrypted && content != "" {
			body = truncateRunes(content, pushBodyMaxLen)
		}

		s.send(ctx, recipientID, push.Notification{
			Title:    senderName,
			Body:     body,
			Category: push.CategoryMessage,
			Data: map[string]string{
				"type":          "dm",
				"dm_channel_id": dmChannelID,
				"sender_id":     senderID,
			},
		})
	}()
}

func (s *pushService) NotifyCall(receiverID, callerName string, callType models.P2PCallType, callID, callerID string) {
	if !s.sender.Enabled() {
		return
	}
	go func() {
		if s.isOnline(receiverID) {
			return
		}

		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		loc := i18n.NewLocalizer(s.recipientLang(ctx, receiverID))

		bodyKey := "push.incomingVoiceCall"
		if callType == models.P2PCallTypeVideo {
			bodyKey = "push.incomingVideoCall"
		}

		s.send(ctx, receiverID, push.Notification{
			Title:    callerName,
			Body:     loc.T(bodyKey),
			Category: push.CategoryCall,
			Data: map[string]string{
				"type":      "call",
				"call_id":   callID,
				"caller_id": callerID,
				"call_type": string(callType),
			},
		})
	}()
}

// send delivers to a user's tokens, pruning any tokens FCM reports as permanently
// unregistered. Callers gate on presence before invoking.
func (s *pushService) send(ctx context.Context, userID string, n push.Notification) {
	tokens, err := s.tokenRepo.ListByUser(ctx, userID)
	if err != nil {
		log.Printf("[push] list tokens for %s: %v", userID, err)
		return
	}
	if len(tokens) == 0 {
		return
	}

	values := make([]string, len(tokens))
	for i, t := range tokens {
		values[i] = t.Token
	}

	invalid, err := s.sender.Send(ctx, values, n)
	if err != nil {
		log.Printf("[push] send to %s: %v", userID, err)
		return
	}
	if len(invalid) > 0 {
		if delErr := s.tokenRepo.DeleteTokens(ctx, invalid); delErr != nil {
			log.Printf("[push] prune %d invalid tokens: %v", len(invalid), delErr)
		}
	}
}

func (s *pushService) isOnline(userID string) bool {
	for _, id := range s.presence.GetOnlineUserIDs() {
		if id == userID {
			return true
		}
	}
	return false
}

func (s *pushService) recipientLang(ctx context.Context, userID string) string {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil || u == nil {
		return i18n.DefaultLanguage
	}
	return u.Language
}

// pushDisplayName resolves a user's notification title: display name if set,
// otherwise username. Safe on nil.
func pushDisplayName(u *models.User) string {
	if u == nil {
		return ""
	}
	if u.DisplayName != nil && *u.DisplayName != "" {
		return *u.DisplayName
	}
	return u.Username
}

func truncateRunes(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
