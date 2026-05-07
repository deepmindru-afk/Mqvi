// Package services — AdminUserService: platform-level user management.
//
// Handles platform-wide ban and hard delete (distinct from server-scoped MemberService.BanUser).
// Email notifications are optional — sent if reason is provided and user has an email.
// Email errors do not roll back the action.
package services

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"

	"github.com/akinalp/mqvi/database"
	"github.com/akinalp/mqvi/pkg"
	"github.com/akinalp/mqvi/pkg/email"
	"github.com/akinalp/mqvi/repository"
	"github.com/akinalp/mqvi/ws"
)

// AdminUserService handles platform-level user ban and deletion.
type AdminUserService interface {
	PlatformBanUser(ctx context.Context, adminUserID, targetUserID, reason string, deleteMessages bool) error
	PlatformUnbanUser(ctx context.Context, adminUserID, targetUserID string) error
	HardDeleteUser(ctx context.Context, adminUserID, targetUserID, reason string) error
	SetPlatformAdmin(ctx context.Context, adminUserID, targetUserID string, isAdmin bool) error
}

type adminUserService struct {
	db          *sql.DB
	userRepo    repository.UserRepository
	hub         ws.ClientManager
	voiceKit    VoiceDisconnecter // ISP defined in member_service.go
	emailSender email.EmailSender // optional, nil = no emails
	fileCleanup FileCleanupService
}

func NewAdminUserService(
	db *sql.DB,
	userRepo repository.UserRepository,
	hub ws.ClientManager,
	voiceKit VoiceDisconnecter,
	emailSender email.EmailSender,
	fileCleanup FileCleanupService,
) AdminUserService {
	return &adminUserService{
		db:          db,
		userRepo:    userRepo,
		hub:         hub,
		voiceKit:    voiceKit,
		emailSender: emailSender,
		fileCleanup: fileCleanup,
	}
}

func (s *adminUserService) PlatformBanUser(ctx context.Context, adminUserID, targetUserID, reason string, deleteMessages bool) error {
	if adminUserID == targetUserID {
		return fmt.Errorf("%w: cannot ban yourself", pkg.ErrBadRequest)
	}

	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("target user not found: %w", err)
	}

	if target.IsPlatformAdmin {
		return fmt.Errorf("%w: cannot ban a platform admin", pkg.ErrForbidden)
	}

	if target.IsPlatformBanned {
		return fmt.Errorf("%w: user is already banned", pkg.ErrBadRequest)
	}

	// Collect file refs BEFORE any mutations so a failed collect doesn't leave partial state
	var msgPlan *CleanupPlan
	if deleteMessages {
		var collectErr error
		msgPlan, collectErr = s.fileCleanup.CollectUserMessageFiles(ctx, targetUserID)
		if collectErr != nil {
			return fmt.Errorf("failed to collect user message files: %w", collectErr)
		}
	}

	// Atomic: user flag + durable ban record in one transaction
	var banEmail string
	if target.Email != nil {
		banEmail = *target.Email
	}
	if err := database.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		txRepo := repository.NewSQLiteUserRepo(tx)
		if err := txRepo.PlatformBan(ctx, targetUserID, reason, adminUserID); err != nil {
			return fmt.Errorf("failed to ban user: %w", err)
		}
		if err := txRepo.InsertPlatformBan(ctx, banEmail, target.Username, targetUserID, reason, adminUserID); err != nil {
			return fmt.Errorf("failed to insert platform ban record: %w", err)
		}
		return nil
	}); err != nil {
		return err
	}

	if deleteMessages {
		if err := s.userRepo.DeleteAllMessagesByUser(ctx, targetUserID); err != nil {
			return fmt.Errorf("failed to delete user messages: %w", err)
		}
		s.fileCleanup.Execute(msgPlan)
	}

	s.voiceKit.DisconnectUser(targetUserID)
	s.hub.DisconnectUser(targetUserID)

	// Best-effort email notification
	if reason != "" && target.Email != nil && s.emailSender != nil {
		if emailErr := s.emailSender.SendPlatformBanNotification(ctx, *target.Email, reason); emailErr != nil {
			log.Printf("[admin] failed to send ban notification email to %s: %v", targetUserID, emailErr)
		}
	}

	return nil
}

func (s *adminUserService) PlatformUnbanUser(ctx context.Context, adminUserID, targetUserID string) error {
	if adminUserID == targetUserID {
		return fmt.Errorf("%w: cannot unban yourself", pkg.ErrBadRequest)
	}

	// User may be hard-deleted — distinguish "not found" from real DB errors
	target, userErr := s.userRepo.GetByID(ctx, targetUserID)
	userExists := userErr == nil
	if userErr != nil && !errors.Is(userErr, pkg.ErrNotFound) {
		return fmt.Errorf("failed to look up user: %w", userErr)
	}

	if userExists && !target.IsPlatformBanned {
		return fmt.Errorf("%w: user is not banned", pkg.ErrBadRequest)
	}

	if !userExists {
		// Verify a platform_bans record actually exists for this user ID
		banned, checkErr := s.userRepo.IsPlatformBannedByUserID(ctx, targetUserID)
		if checkErr != nil {
			return fmt.Errorf("failed to check platform ban: %w", checkErr)
		}
		if !banned {
			return fmt.Errorf("%w: no ban record found", pkg.ErrNotFound)
		}
	}

	// Atomic: clear user flag (if user exists) + remove durable ban record
	return database.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		txRepo := repository.NewSQLiteUserRepo(tx)
		if userExists {
			if err := txRepo.PlatformUnban(ctx, targetUserID); err != nil {
				return fmt.Errorf("failed to unban user: %w", err)
			}
		}
		if err := txRepo.DeletePlatformBan(ctx, targetUserID); err != nil {
			return fmt.Errorf("failed to delete platform ban record: %w", err)
		}
		return nil
	})
}

// HardDeleteUser permanently deletes a user and all associated data.
// Email notification is sent BEFORE deletion (email address is lost after delete).
func (s *adminUserService) HardDeleteUser(ctx context.Context, adminUserID, targetUserID, reason string) error {
	if adminUserID == targetUserID {
		return fmt.Errorf("%w: cannot delete yourself", pkg.ErrBadRequest)
	}

	target, err := s.userRepo.GetByID(ctx, targetUserID)
	if err != nil {
		return fmt.Errorf("target user not found: %w", err)
	}

	if target.IsPlatformAdmin {
		return fmt.Errorf("%w: cannot delete a platform admin", pkg.ErrForbidden)
	}

	// Send email BEFORE deletion (address is lost after)
	if reason != "" && target.Email != nil && s.emailSender != nil {
		if emailErr := s.emailSender.SendAccountDeleteNotification(ctx, *target.Email, reason); emailErr != nil {
			log.Printf("[admin] failed to send delete notification email to %s: %v", targetUserID, emailErr)
		}
	}

	// Phase 1: collect ALL user file refs (including owned server files)
	plan, collectErr := s.fileCleanup.CollectUserFiles(ctx, targetUserID)
	if collectErr != nil {
		return fmt.Errorf("failed to collect user files: %w", collectErr)
	}

	// Disconnect realtime connections before DB delete to avoid race conditions
	s.voiceKit.DisconnectUser(targetUserID)
	s.hub.DisconnectUser(targetUserID)

	// Phase 2: DB delete (CASCADE removes user data, owned servers, etc.)
	if err := s.userRepo.HardDeleteUser(ctx, targetUserID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	// Phase 3: delete files from disk + release other users' quota
	s.fileCleanup.Execute(plan)

	return nil
}

func (s *adminUserService) SetPlatformAdmin(ctx context.Context, adminUserID, targetUserID string, isAdmin bool) error {
	if adminUserID == targetUserID {
		return fmt.Errorf("%w: cannot modify your own admin status", pkg.ErrBadRequest)
	}

	if _, err := s.userRepo.GetByID(ctx, targetUserID); err != nil {
		return fmt.Errorf("target user not found: %w", err)
	}

	if err := s.userRepo.SetPlatformAdmin(ctx, targetUserID, isAdmin); err != nil {
		return fmt.Errorf("failed to update admin status: %w", err)
	}

	return nil
}
