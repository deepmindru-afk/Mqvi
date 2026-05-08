package services

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/akinalp/mqvi/database"
	"github.com/akinalp/mqvi/models"
	"github.com/akinalp/mqvi/pkg"
	"github.com/akinalp/mqvi/pkg/crypto"
	"github.com/akinalp/mqvi/repository"
	"github.com/akinalp/mqvi/ws"
)

// LiveKitSettings exposes non-secret LiveKit info for the settings UI.
type LiveKitSettings struct {
	URL               string `json:"url"`
	IsPlatformManaged bool   `json:"is_platform_managed"`
}

type ServerService interface {
	CreateServer(ctx context.Context, ownerID string, req *models.CreateServerRequest) (*models.Server, error)
	GetServer(ctx context.Context, serverID string) (*models.Server, error)
	// GetServerRaw returns the server without signing file URLs. Used for internal
	// operations like file deletion where the raw DB path is needed.
	GetServerRaw(ctx context.Context, serverID string) (*models.Server, error)
	GetUserServers(ctx context.Context, userID string) ([]models.ServerListItem, error)
	UpdateServer(ctx context.Context, serverID string, req *models.UpdateServerRequest) (*models.Server, error)
	UpdateIcon(ctx context.Context, serverID, iconURL string) (*models.Server, error)
	// DeleteServer soft-deletes the server. Files and LiveKit instance are preserved.
	// Use HardDeleteServer to permanently remove (skip 30-day TTL).
	DeleteServer(ctx context.Context, serverID, userID string) error
	// RestoreServer un-soft-deletes the server. Owner can only restore servers they soft-deleted
	// themselves; admin-deleted servers can only be restored by an admin.
	RestoreServer(ctx context.Context, serverID, userID string) error
	// HardDeleteServer permanently deletes a soft-deleted server (skip 30-day TTL).
	// Files cleaned, LiveKit instance released, DB cascade.
	HardDeleteServer(ctx context.Context, serverID, userID string) error
	// GetDeletedServers returns soft-deleted servers owned by this user (for restore UI).
	GetDeletedServers(ctx context.Context, userID string) ([]models.DeletedServerInfo, error)
	JoinServer(ctx context.Context, userID, inviteCode string) (*models.Server, error)
	LeaveServer(ctx context.Context, serverID, userID string) error
	GetLiveKitSettings(ctx context.Context, serverID string) (*LiveKitSettings, error)
	// ReorderServers updates the user's personal server list order. No WS broadcast.
	ReorderServers(ctx context.Context, userID string, req *models.ReorderServersRequest) ([]models.ServerListItem, error)
}

type serverService struct {
	db            *sql.DB // for WithTx in CreateServer
	serverRepo    repository.ServerRepository
	livekitRepo   repository.LiveKitRepository
	roleRepo      repository.RoleRepository
	channelRepo   repository.ChannelRepository
	categoryRepo  repository.CategoryRepository
	userRepo      repository.UserRepository
	inviteService InviteService
	hub           ws.BroadcastAndManage
	encryptionKey []byte // AES-256-GCM for LiveKit credentials
	urlSigner     FileURLSigner
	fileCleanup   FileCleanupService
}

func NewServerService(
	db *sql.DB,
	serverRepo repository.ServerRepository,
	livekitRepo repository.LiveKitRepository,
	roleRepo repository.RoleRepository,
	channelRepo repository.ChannelRepository,
	categoryRepo repository.CategoryRepository,
	userRepo repository.UserRepository,
	inviteService InviteService,
	hub ws.BroadcastAndManage,
	encryptionKey []byte,
	urlSigner FileURLSigner,
	fileCleanup FileCleanupService,
) ServerService {
	return &serverService{
		db:            db,
		serverRepo:    serverRepo,
		livekitRepo:   livekitRepo,
		roleRepo:      roleRepo,
		channelRepo:   channelRepo,
		categoryRepo:  categoryRepo,
		userRepo:      userRepo,
		inviteService: inviteService,
		hub:           hub,
		encryptionKey: encryptionKey,
		urlSigner:     urlSigner,
		fileCleanup:   fileCleanup,
	}
}

// CreateServer creates a new server atomically (server + membership + roles + channels in one tx).
func (s *serverService) CreateServer(ctx context.Context, ownerID string, req *models.CreateServerRequest) (*models.Server, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", pkg.ErrBadRequest, err)
	}

	// Non-admin users can own at most 1 mqvi-hosted server
	if req.HostType == "mqvi_hosted" {
		user, err := s.userRepo.GetByID(ctx, ownerID)
		if err != nil {
			return nil, fmt.Errorf("failed to get user: %w", err)
		}
		if !user.IsPlatformAdmin {
			count, err := s.serverRepo.CountOwnedMqviHostedServers(ctx, ownerID)
			if err != nil {
				return nil, fmt.Errorf("failed to count owned servers: %w", err)
			}
			if count >= 1 {
				return nil, fmt.Errorf("%w: you can only own 1 mqvi-hosted server, you can create unlimited self-hosted servers", pkg.ErrBadRequest)
			}
		}
	}

	// ─── LiveKit instance setup (outside transaction) ───
	var livekitInstanceID *string

	switch req.HostType {
	case "self_hosted":
		if req.LiveKitURL == "" || req.LiveKitKey == "" || req.LiveKitSecret == "" {
			return nil, fmt.Errorf("%w: livekit_url, livekit_key, and livekit_secret are required for self-hosted", pkg.ErrBadRequest)
		}

		encKey, err := crypto.Encrypt(req.LiveKitKey, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt livekit key: %w", err)
		}
		encSecret, err := crypto.Encrypt(req.LiveKitSecret, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt livekit secret: %w", err)
		}

		instance := &models.LiveKitInstance{
			URL:               req.LiveKitURL,
			APIKey:            encKey,
			APISecret:         encSecret,
			IsPlatformManaged: false,
			ServerCount:       1,
		}

		if err := s.livekitRepo.Create(ctx, instance); err != nil {
			return nil, fmt.Errorf("failed to create livekit instance: %w", err)
		}

		livekitInstanceID = &instance.ID

	case "mqvi_hosted":
		instance, err := s.livekitRepo.GetLeastLoadedPlatformInstance(ctx)
		if err != nil {
			log.Printf("[server] no platform livekit instance available, creating server without voice: %v", err)
		} else {
			livekitInstanceID = &instance.ID
			if err := s.livekitRepo.IncrementServerCount(ctx, instance.ID); err != nil {
				return nil, fmt.Errorf("failed to increment server count: %w", err)
			}
		}

	default:
		// No voice support
	}

	// ─── Atomic transaction: server + membership + roles + channels ───
	server := &models.Server{
		Name:              req.Name,
		OwnerID:           ownerID,
		InviteRequired:    false,
		LiveKitInstanceID: livekitInstanceID,
	}

	err := database.WithTx(ctx, s.db, func(tx *sql.Tx) error {
		txServerRepo := repository.NewSQLiteServerRepo(tx)
		txRoleRepo := repository.NewSQLiteRoleRepo(tx)
		txChannelRepo := repository.NewSQLiteChannelRepo(tx)
		txCategoryRepo := repository.NewSQLiteCategoryRepo(tx)

		if err := txServerRepo.Create(ctx, server); err != nil {
			return fmt.Errorf("failed to create server: %w", err)
		}

		if err := txServerRepo.AddMember(ctx, server.ID, ownerID); err != nil {
			return fmt.Errorf("failed to add owner as member: %w", err)
		}

		// Default "everyone" role
		defaultPerms := models.PermViewChannel | models.PermReadMessages | models.PermSendMessages |
			models.PermConnectVoice | models.PermSpeak | models.PermUseSoundboard

		defaultRole := &models.Role{
			ServerID:    server.ID,
			Name:        "everyone",
			Color:       "#99AAB5",
			Position:    1,
			Permissions: defaultPerms,
			IsDefault:   true,
			Mentionable: true,
		}
		if err := txRoleRepo.Create(ctx, defaultRole); err != nil {
			return fmt.Errorf("failed to create default role: %w", err)
		}

		// Owner role — highest position, full permissions
		ownerRole := &models.Role{
			ServerID:    server.ID,
			Name:        "Owner",
			Color:       "#E74C3C",
			Position:    100,
			Permissions: models.PermAll,
			IsOwner:     true,
		}
		if err := txRoleRepo.Create(ctx, ownerRole); err != nil {
			return fmt.Errorf("failed to create owner role: %w", err)
		}

		if err := txRoleRepo.AssignToUser(ctx, ownerID, defaultRole.ID, server.ID); err != nil {
			return fmt.Errorf("failed to assign default role to owner: %w", err)
		}
		if err := txRoleRepo.AssignToUser(ctx, ownerID, ownerRole.ID, server.ID); err != nil {
			return fmt.Errorf("failed to assign owner role: %w", err)
		}

		// Default categories + channels
		textCategory := &models.Category{
			ServerID: server.ID,
			Name:     "Text Channels",
			Position: 0,
		}
		if err := txCategoryRepo.Create(ctx, textCategory); err != nil {
			return fmt.Errorf("failed to create text category: %w", err)
		}

		voiceCategory := &models.Category{
			ServerID: server.ID,
			Name:     "Voice Channels",
			Position: 1,
		}
		if err := txCategoryRepo.Create(ctx, voiceCategory); err != nil {
			return fmt.Errorf("failed to create voice category: %w", err)
		}

		textChannel := &models.Channel{
			ServerID:   server.ID,
			Name:       "general",
			Type:       models.ChannelTypeText,
			CategoryID: &textCategory.ID,
			Position:   0,
		}
		if err := txChannelRepo.Create(ctx, textChannel); err != nil {
			return fmt.Errorf("failed to create default text channel: %w", err)
		}

		voiceChannel := &models.Channel{
			ServerID:   server.ID,
			Name:       "General",
			Type:       models.ChannelTypeVoice,
			CategoryID: &voiceCategory.ID,
			Position:   0,
			Bitrate:    64000,
		}
		if err := txChannelRepo.Create(ctx, voiceChannel); err != nil {
			return fmt.Errorf("failed to create default voice channel: %w", err)
		}

		return nil
	})

	if err != nil {
		return nil, fmt.Errorf("failed to create server (transaction): %w", err)
	}

	// WS broadcast (after commit)
	s.hub.AddClientServerID(ownerID, server.ID)
	s.hub.BroadcastToUser(ownerID, ws.Event{
		Op: ws.OpServerCreate,
		Data: models.ServerListItem{
			ID:      server.ID,
			Name:    server.Name,
			IconURL: s.urlSigner.SignURLPtr(server.IconURL),
		},
	})

	log.Printf("[server] created server %s (name=%s, owner=%s, host=%s)",
		server.ID, server.Name, ownerID, req.HostType)

	return server, nil
}

func (s *serverService) GetServer(ctx context.Context, serverID string) (*models.Server, error) {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, err
	}
	server.IconURL = s.urlSigner.SignURLPtr(server.IconURL)
	return server, nil
}

func (s *serverService) GetServerRaw(ctx context.Context, serverID string) (*models.Server, error) {
	return s.serverRepo.GetByID(ctx, serverID)
}

func (s *serverService) GetUserServers(ctx context.Context, userID string) ([]models.ServerListItem, error) {
	servers, err := s.serverRepo.GetUserServers(ctx, userID)
	if err != nil {
		return nil, err
	}
	for i := range servers {
		servers[i].IconURL = s.urlSigner.SignURLPtr(servers[i].IconURL)
	}
	return servers, nil
}

func (s *serverService) UpdateServer(ctx context.Context, serverID string, req *models.UpdateServerRequest) (*models.Server, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %v", pkg.ErrBadRequest, err)
	}

	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, err
	}

	if req.Name != nil {
		server.Name = *req.Name
	}
	if req.InviteRequired != nil {
		server.InviteRequired = *req.InviteRequired
	}
	if req.E2EEEnabled != nil {
		server.E2EEEnabled = *req.E2EEEnabled
	}
	if req.AFKTimeoutMinutes != nil {
		server.AFKTimeoutMinutes = *req.AFKTimeoutMinutes
	}

	if err := s.serverRepo.Update(ctx, server); err != nil {
		return nil, fmt.Errorf("failed to update server: %w", err)
	}

	// LiveKit credential update (self-hosted only)
	if req.HasLiveKitUpdate() {
		if server.LiveKitInstanceID == nil {
			return nil, fmt.Errorf("%w: this server has no LiveKit instance", pkg.ErrBadRequest)
		}

		instance, err := s.livekitRepo.GetByID(ctx, *server.LiveKitInstanceID)
		if err != nil {
			return nil, fmt.Errorf("failed to get livekit instance: %w", err)
		}
		if instance.IsPlatformManaged {
			return nil, fmt.Errorf("%w: cannot modify platform-managed LiveKit instance", pkg.ErrForbidden)
		}

		encKey, err := crypto.Encrypt(*req.LiveKitKey, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt livekit key: %w", err)
		}
		encSecret, err := crypto.Encrypt(*req.LiveKitSecret, s.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt livekit secret: %w", err)
		}

		instance.URL = *req.LiveKitURL
		instance.APIKey = encKey
		instance.APISecret = encSecret

		if err := s.livekitRepo.Update(ctx, instance); err != nil {
			return nil, fmt.Errorf("failed to update livekit instance: %w", err)
		}

		log.Printf("[server] livekit credentials updated for server %s", serverID)
	}

	server.IconURL = s.urlSigner.SignURLPtr(server.IconURL)
	s.hub.BroadcastToServer(serverID, ws.Event{
		Op:   ws.OpServerUpdate,
		Data: server,
	})

	return server, nil
}

func (s *serverService) UpdateIcon(ctx context.Context, serverID, iconURL string) (*models.Server, error) {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, err
	}

	server.IconURL = &iconURL

	if err := s.serverRepo.Update(ctx, server); err != nil {
		return nil, fmt.Errorf("failed to update server icon: %w", err)
	}

	server.IconURL = s.urlSigner.SignURLPtr(server.IconURL)
	s.hub.BroadcastToServer(serverID, ws.Event{
		Op:   ws.OpServerUpdate,
		Data: server,
	})

	return server, nil
}

// DeleteServer soft-deletes the server. Files, LiveKit instance, and member roles
// are preserved for restore. Worker hard-deletes after 30-day TTL (Phase 16 P3).
func (s *serverService) DeleteServer(ctx context.Context, serverID, userID string) error {
	server, err := s.serverRepo.GetActiveByID(ctx, serverID)
	if err != nil {
		return err
	}

	if server.OwnerID != userID {
		return fmt.Errorf("%w: only the server owner can delete the server", pkg.ErrForbidden)
	}

	if err := s.serverRepo.SoftDelete(ctx, serverID, userID, false); err != nil {
		return fmt.Errorf("failed to soft delete server: %w", err)
	}

	// Members hide the server in their UI on this event.
	s.hub.BroadcastToServer(serverID, ws.Event{
		Op:   ws.OpServerDelete,
		Data: map[string]string{"id": serverID},
	})

	log.Printf("[server] soft-deleted server %s by owner %s", serverID, userID)
	return nil
}

// RestoreServer un-soft-deletes a server. Owner can only restore servers they soft-deleted
// themselves; admin-deleted servers (deleted_by_admin=1) are not restorable by the owner.
func (s *serverService) RestoreServer(ctx context.Context, serverID, userID string) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return err
	}

	if server.OwnerID != userID {
		return fmt.Errorf("%w: only the server owner can restore the server", pkg.ErrForbidden)
	}

	if server.DeletedAt == nil {
		return fmt.Errorf("%w: server is not deleted", pkg.ErrBadRequest)
	}

	if server.DeletedByAdmin {
		return fmt.Errorf("%w: server was deleted by an admin and cannot be restored by the owner", pkg.ErrForbidden)
	}

	if err := s.serverRepo.Restore(ctx, serverID); err != nil {
		return fmt.Errorf("failed to restore server: %w", err)
	}

	s.broadcastServerRestore(ctx, serverID)

	log.Printf("[server] restored server %s by owner %s", serverID, userID)
	return nil
}

// broadcastServerRestore notifies all server members that a soft-deleted server
// is back. We can't use BroadcastToServer alone because members who reconnected
// while the server was soft-deleted are NOT in hub.serverClients[serverID]
// (their client.serverIDs filter excluded the deleted server on connect).
// Approach: re-subscribe each online member via AddClientServerID, then send
// the event via BroadcastToUser so offline members are silent no-ops.
func (s *serverService) broadcastServerRestore(ctx context.Context, serverID string) {
	restored, err := s.serverRepo.GetActiveByID(ctx, serverID)
	if err != nil || restored == nil {
		return
	}

	memberIDs, err := s.serverRepo.GetMemberUserIDs(ctx, serverID)
	if err != nil {
		log.Printf("[server] failed to list members for restore broadcast %s: %v", serverID, err)
		return
	}

	event := ws.Event{Op: ws.OpServerRestore, Data: restored}
	for _, uid := range memberIDs {
		// Re-subscribe online members to this server's broadcast index.
		// AddClientServerID is a no-op for offline users.
		s.hub.AddClientServerID(uid, serverID)
		s.hub.BroadcastToUser(uid, event)
	}
}

// HardDeleteServer permanently deletes a soft-deleted server (skip 30-day TTL).
// Files cleaned, LiveKit instance released, DB cascade removes channels/messages/etc.
func (s *serverService) HardDeleteServer(ctx context.Context, serverID, userID string) error {
	// Use GetByID — server must be soft-deleted to be hard-deletable by owner
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return err
	}

	if server.OwnerID != userID {
		return fmt.Errorf("%w: only the server owner can permanently delete the server", pkg.ErrForbidden)
	}

	if server.DeletedAt == nil {
		return fmt.Errorf("%w: server must be soft-deleted before permanent deletion", pkg.ErrBadRequest)
	}

	if server.DeletedByAdmin {
		return fmt.Errorf("%w: admin-deleted server cannot be permanently deleted by owner", pkg.ErrForbidden)
	}

	// Phase 1: collect file refs (also collects LiveKit instance for cleanup)
	plan, err := s.fileCleanup.CollectServerFiles(ctx, serverID)
	if err != nil {
		return fmt.Errorf("failed to collect server files: %w", err)
	}

	// Phase 2: DB delete (CASCADE removes channels, messages, attachments, etc.)
	if err := s.serverRepo.Delete(ctx, serverID); err != nil {
		return fmt.Errorf("failed to delete server: %w", err)
	}

	// Phase 3: file cleanup + LiveKit cleanup (server_delete WS event already
	// broadcast on the original soft-delete; no need to re-broadcast).
	s.fileCleanup.Execute(plan)

	log.Printf("[server] hard-deleted server %s by owner %s", serverID, userID)
	return nil
}

// GetDeletedServers returns soft-deleted servers owned by this user with countdown info.
func (s *serverService) GetDeletedServers(ctx context.Context, userID string) ([]models.DeletedServerInfo, error) {
	servers, err := s.serverRepo.ListDeletedByOwner(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list deleted servers: %w", err)
	}

	result := make([]models.DeletedServerInfo, 0, len(servers))
	for _, srv := range servers {
		iconURL := s.urlSigner.SignURLPtr(srv.IconURL)
		var deletedAt time.Time
		if srv.DeletedAt != nil {
			deletedAt = *srv.DeletedAt
		}
		result = append(result, models.DeletedServerInfo{
			ID:                srv.ID,
			Name:              srv.Name,
			IconURL:           iconURL,
			DeletedAt:         deletedAt,
			DeletedByAdmin:    srv.DeletedByAdmin,
			PermanentDeleteAt: deletedAt.AddDate(0, 0, models.SoftDeleteTTLDays),
		})
	}
	return result, nil
}

// JoinServer joins a server via invite code.
func (s *serverService) JoinServer(ctx context.Context, userID, inviteCode string) (*models.Server, error) {
	invite, err := s.inviteService.ValidateAndUse(ctx, inviteCode)
	if err != nil {
		return nil, err
	}

	serverID := invite.ServerID

	// Reject join attempts on soft-deleted servers — invite redeem must not
	// dirty membership/usage counters or restore-time member lists.
	if _, err := s.serverRepo.GetActiveByID(ctx, serverID); err != nil {
		return nil, fmt.Errorf("%w: server is no longer available", pkg.ErrNotFound)
	}

	isMember, err := s.serverRepo.IsMember(ctx, serverID, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check membership: %w", err)
	}
	if isMember {
		return nil, fmt.Errorf("%w: already a member of this server", pkg.ErrBadRequest)
	}

	if err := s.serverRepo.AddMember(ctx, serverID, userID); err != nil {
		return nil, fmt.Errorf("failed to add member: %w", err)
	}

	// Assign default role
	defaultRole, err := s.roleRepo.GetDefaultByServer(ctx, serverID)
	if err != nil {
		log.Printf("[server] failed to get default role for server %s: %v", serverID, err)
	} else {
		if err := s.roleRepo.AssignToUser(ctx, userID, defaultRole.ID, serverID); err != nil {
			log.Printf("[server] failed to assign default role: %v", err)
		}
	}

	server, err := s.serverRepo.GetActiveByID(ctx, serverID)
	if err != nil {
		return nil, fmt.Errorf("failed to get server: %w", err)
	}

	// Add server to user's WS subscription list
	s.hub.AddClientServerID(userID, serverID)

	// Notify user: server added to their list
	s.hub.BroadcastToUser(userID, ws.Event{
		Op: ws.OpServerCreate,
		Data: models.ServerListItem{
			ID:      server.ID,
			Name:    server.Name,
			IconURL: s.urlSigner.SignURLPtr(server.IconURL),
		},
	})

	// Notify server members: new member joined (full MemberWithRoles for frontend)
	user, err := s.userRepo.GetByID(ctx, userID)
	if err != nil {
		log.Printf("[server] failed to get user %s for member_join broadcast: %v", userID, err)
	} else {
		roles, _ := s.roleRepo.GetByUserIDAndServer(ctx, userID, serverID)
		member := models.ToMemberWithRoles(user, roles)
		member.AvatarURL = s.urlSigner.SignURLPtr(member.AvatarURL)
		s.hub.BroadcastToServer(serverID, ws.Event{
			Op:   ws.OpMemberJoin,
			Data: member,
		})
	}

	log.Printf("[server] user %s joined server %s via invite", userID, serverID)
	server.IconURL = s.urlSigner.SignURLPtr(server.IconURL)
	return server, nil
}

func (s *serverService) LeaveServer(ctx context.Context, serverID, userID string) error {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return err
	}

	if server.OwnerID == userID {
		return fmt.Errorf("%w: server owner cannot leave; transfer ownership first", pkg.ErrForbidden)
	}

	if err := s.serverRepo.RemoveMember(ctx, serverID, userID); err != nil {
		return fmt.Errorf("failed to remove member: %w", err)
	}

	// Notify server members (broadcast before removing subscription)
	s.hub.BroadcastToServer(serverID, ws.Event{
		Op: ws.OpMemberLeave,
		Data: map[string]string{
			"server_id": serverID,
			"user_id":   userID,
		},
	})

	// Notify user: server removed from their list
	s.hub.BroadcastToUser(userID, ws.Event{
		Op:   ws.OpServerDelete,
		Data: map[string]string{"id": serverID},
	})

	// Remove from WS subscription list
	s.hub.RemoveClientServerID(userID, serverID)

	log.Printf("[server] user %s left server %s", userID, serverID)
	return nil
}

// GetLiveKitSettings returns non-secret LiveKit info for the settings page.
func (s *serverService) GetLiveKitSettings(ctx context.Context, serverID string) (*LiveKitSettings, error) {
	server, err := s.serverRepo.GetByID(ctx, serverID)
	if err != nil {
		return nil, err
	}

	if server.LiveKitInstanceID == nil {
		return nil, fmt.Errorf("%w: this server has no LiveKit instance", pkg.ErrNotFound)
	}

	instance, err := s.livekitRepo.GetByID(ctx, *server.LiveKitInstanceID)
	if err != nil {
		return nil, fmt.Errorf("failed to get livekit instance: %w", err)
	}

	return &LiveKitSettings{
		URL:               instance.URL,
		IsPlatformManaged: instance.IsPlatformManaged,
	}, nil
}

// ReorderServers updates the user's personal server list order (per-user, no broadcast).
func (s *serverService) ReorderServers(ctx context.Context, userID string, req *models.ReorderServersRequest) ([]models.ServerListItem, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("%w: %s", pkg.ErrBadRequest, err.Error())
	}

	if err := s.serverRepo.UpdateMemberPositions(ctx, userID, req.Items); err != nil {
		return nil, fmt.Errorf("failed to update server positions: %w", err)
	}

	servers, err := s.serverRepo.GetUserServers(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to reload servers after reorder: %w", err)
	}
	for i := range servers {
		servers[i].IconURL = s.urlSigner.SignURLPtr(servers[i].IconURL)
	}
	return servers, nil
}
