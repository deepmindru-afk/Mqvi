package repository

import (
	"context"
	"time"

	"github.com/akinalp/mqvi/models"
)

type FeedbackRepository interface {
	CreateTicket(ctx context.Context, ticket *models.FeedbackTicket) error
	GetTicketByID(ctx context.Context, id string) (*models.FeedbackTicketWithUser, error)
	ListByUser(ctx context.Context, userID string, limit, offset int) ([]models.FeedbackTicketWithUser, int, error)
	ListAll(ctx context.Context, status, ticketType string, limit, offset int) ([]models.FeedbackTicketWithUser, int, error)
	UpdateStatus(ctx context.Context, id string, status models.FeedbackStatus) error

	DeleteTicket(ctx context.Context, id string) error

	CreateReply(ctx context.Context, reply *models.FeedbackReply) error
	GetRepliesByTicketID(ctx context.Context, ticketID string) ([]models.FeedbackReplyWithUser, error)

	CreateAttachment(ctx context.Context, att *models.FeedbackAttachment) error
	GetAttachmentsByTicketID(ctx context.Context, ticketID string) ([]models.FeedbackAttachment, error)

	// LatestCreatedAt returns the newest ticket's created_at, or nil when the
	// table is empty. Drives the admin "new feedback" badge.
	LatestCreatedAt(ctx context.Context) (*time.Time, error)

	// LatestAdminReplyForUser returns the newest admin-reply timestamp on any
	// ticket owned by userID, or nil. Drives the user's own feedback badge.
	LatestAdminReplyForUser(ctx context.Context, userID string) (*time.Time, error)
}
