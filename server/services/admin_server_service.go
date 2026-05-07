// Package services — AdminServerService: platform admin server management.
//
// Allows platform admin to delete any server (unlike owner-only ServerService.DeleteServer).
//
// Deletion order:
// 1. Collect file refs (read-only)
// 2. server_delete broadcast (BEFORE DB delete — member list is needed for broadcast)
// 3. DB delete (CASCADE removes channels, messages, members, etc.)
// 4. Side effects: file cleanup, LiveKit cleanup, email notification
package services

import (
	"context"
	"fmt"
	"log"

	"github.com/akinalp/mqvi/pkg/email"
	"github.com/akinalp/mqvi/repository"
	"github.com/akinalp/mqvi/ws"
)

// AdminServerService handles platform admin server deletion.
type AdminServerService interface {
	DeleteServer(ctx context.Context, adminUserID, serverID, reason string) error
}

type adminServerService struct {
	serverRepo  repository.ServerRepository
	userRepo    repository.UserRepository
	livekitRepo repository.LiveKitRepository
	hub         ws.EventPublisher
	emailSender email.EmailSender // optional, nil = no emails
	fileCleanup FileCleanupService
}

func NewAdminServerService(
	serverRepo repository.ServerRepository,
	userRepo repository.UserRepository,
	livekitRepo repository.LiveKitRepository,
	hub ws.EventPublisher,
	emailSender email.EmailSender,
	fileCleanup FileCleanupService,
) AdminServerService {
	return &adminServerService{
		serverRepo:  serverRepo,
		userRepo:    userRepo,
		livekitRepo: livekitRepo,
		hub:         hub,
		emailSender: emailSender,
		fileCleanup: fileCleanup,
	}
}

func (s *adminServerService) DeleteServer(ctx context.Context, adminUserID, serverID, reason string) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return fmt.Errorf("server not found: %w", err)
	}

	// Phase 1: collect file refs (read-only, no side effects)
	plan, collectErr := s.fileCleanup.CollectServerFiles(ctx, serverID)
	if collectErr != nil {
		return fmt.Errorf("failed to collect server files: %w", collectErr)
	}

	// Broadcast BEFORE delete (member list is lost after)
	s.hub.BroadcastToServer(serverID, ws.Event{
		Op:   ws.OpServerDelete,
		Data: map[string]string{"id": serverID},
	})

	// Phase 2: DB delete (CASCADE removes channels, messages, etc.)
	if err := s.serverRepo.Delete(ctx, serverID); err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	// Phase 3: side effects AFTER successful DB delete (files, quota, LiveKit)
	s.fileCleanup.Execute(plan)

	// Best-effort email to server owner
	if reason != "" && s.emailSender != nil {
		owner, ownerErr := s.userRepo.GetByID(ctx, server.OwnerID)
		if ownerErr == nil && owner.Email != nil {
			if emailErr := s.emailSender.SendServerDeleteNotification(ctx, *owner.Email, server.Name, reason); emailErr != nil {
				log.Printf("[admin-server] failed to send server delete notification to owner %s: %v", server.OwnerID, emailErr)
			}
		}
	}

	log.Printf("[admin-server] admin %s deleted server %s (%s)", adminUserID, serverID, server.Name)
	return nil
}
