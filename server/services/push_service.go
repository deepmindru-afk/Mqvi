package services

import (
	"context"
	"log"
	"time"

	"github.com/akinalp/mqvi/models"
	"github.com/akinalp/mqvi/pkg/i18n"
	"github.com/akinalp/mqvi/pkg/push"
	"github.com/akinalp/mqvi/repository"
)

// PushNotifier sends mobile push notifications to a user's devices. Consumed by
// DMService and P2PCallService via SetPushNotifier.
//
// Delivery is NOT gated on WebSocket presence. The mobile OS shows the notification
// only when the app is backgrounded/killed and delivers it silently to a foregrounded
// app (which already renders the message/call in-app over WS). Gating on "any WS
// connection" wrongly suppressed notifications for a backgrounded mobile app that
// still held its socket — exactly the case the feature exists for.
type PushNotifier interface {
	NotifyDM(recipientID, senderName, content string, encrypted bool, dmChannelID, senderID string)
	NotifyCall(receiverID, callerName string, callType models.P2PCallType, callID, callerID string)
}

const pushBodyMaxLen = 140

// pushUserLookup is the minimal user-read interface push needs (names + language).
type pushUserLookup interface {
	GetByID(ctx context.Context, id string) (*models.User, error)
}

type pushService struct {
	sender    push.Sender
	tokenRepo repository.PushTokenRepository
	users     pushUserLookup
}

func NewPushService(sender push.Sender, tokenRepo repository.PushTokenRepository, users pushUserLookup) PushNotifier {
	return &pushService{sender: sender, tokenRepo: tokenRepo, users: users}
}

func (s *pushService) NotifyDM(recipientID, senderName, content string, encrypted bool, dmChannelID, senderID string) {
	if !s.sender.Enabled() {
		return
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		lang, suppressed := s.recipientPush(ctx, recipientID)
		if suppressed {
			return // recipient is in DND / invisible — honor "Pause notifications"
		}

		// Fallback if the caller couldn't supply a name (message.Author not populated).
		if senderName == "" {
			if u, err := s.users.GetByID(ctx, senderID); err == nil && u != nil {
				senderName = pushDisplayName(u)
			}
		}

		loc := i18n.NewLocalizer(lang)

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
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		lang, suppressed := s.recipientPush(ctx, receiverID)
		if suppressed {
			return // receiver is in DND / invisible
		}

		loc := i18n.NewLocalizer(lang)

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

// send delivers to all of a user's device tokens, pruning any FCM reports as
// permanently unregistered.
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

// recipientPush fetches the recipient once and returns their notification language
// plus whether push must be suppressed. Suppression honors the client contract
// ("DND: You will not receive notifications" / "Pause notifications"): a DND or
// invisible (manual offline) recipient gets no push, matching the in-app
// notification-sound suppression in sounds.ts.
func (s *pushService) recipientPush(ctx context.Context, userID string) (lang string, suppressed bool) {
	u, err := s.users.GetByID(ctx, userID)
	if err != nil || u == nil {
		return i18n.DefaultLanguage, false
	}
	if u.PrefStatus == models.UserStatusDND || u.PrefStatus == models.UserStatusOffline {
		return u.Language, true
	}
	return u.Language, false
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
