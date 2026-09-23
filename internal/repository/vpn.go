package repository

import (
	"crypto/sha1"
	"errors"
	"fmt"
	"strings"
	"time"
	"vpnpanel/internal/models"

	"github.com/google/uuid"

	"gorm.io/gorm"
)

type VpnRepo struct {
	DB *gorm.DB
}

func NewVpnRepo(db *gorm.DB) *VpnRepo {
	return &VpnRepo{DB: db}
}

func (r *VpnRepo) GetByUserID(userID uint) (models.Vpn, error) {
	var vpn models.Vpn
	err := r.DB.Where("user_id = ?", userID).First(&vpn).Error
	return vpn, err
}

func (r *VpnRepo) Create(vpn models.Vpn) (models.Vpn, error) {
	if err := r.DB.Create(&vpn).Error; err != nil {
		return models.Vpn{}, err
	}
	return vpn, nil
}

func (r *VpnRepo) Save(vpn models.Vpn) (models.Vpn, error) {
	if err := r.DB.Save(&vpn).Error; err != nil {
		return models.Vpn{}, err
	}
	return vpn, nil
}

func (r *VpnRepo) UpsertLinkByUserID(userID uint, protocol string, link string) (models.Vpn, error) {
	vpn, err := r.GetByUserID(userID)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.Vpn{}, err
	}
	if errors.Is(err, gorm.ErrRecordNotFound) {
		vpn = models.Vpn{
			UUID:   fmt.Sprintf("agent-user-%d", userID),
			UserID: userID,
			Status: "active",
			Link:   link,
		}
	}

	switch protocol {
	case "vless":
		vpn.VlessLink = link
	case "trojan":
		vpn.TrojanLink = link
	}
	if vpn.Link == "" {
		vpn.Link = link
	}

	if errors.Is(err, gorm.ErrRecordNotFound) {
		return r.Create(vpn)
	}
	return r.Save(vpn)
}

func (r *VpnRepo) GetVPNClientByUserID(userID uint) (models.VPNClient, error) {
	var client models.VPNClient
	err := r.DB.Where("user_id = ?", userID).Order("CASE WHEN device_id IS NULL THEN 0 ELSE 1 END, id ASC").Take(&client).Error
	return client, err
}

func (r *VpnRepo) ListVPNClientsByUserID(userID uint) ([]models.VPNClient, error) {
	var clients []models.VPNClient
	err := r.DB.Where("user_id = ?", userID).Order("CASE WHEN device_id IS NULL THEN 0 ELSE 1 END, id ASC").Find(&clients).Error
	return clients, err
}

func (r *VpnRepo) ListProfilesByClientID(clientID uint) ([]models.VPNProfile, error) {
	var profiles []models.VPNProfile
	err := r.DB.Preload("Nodes", func(db *gorm.DB) *gorm.DB {
		return db.Order("server_id ASC, node_id ASC")
	}).Where("vpn_client_id = ?", clientID).Order("profile ASC").Find(&profiles).Error
	return profiles, err
}

func (r *VpnRepo) GetOrCreateVPNClient(userID uint, telegramID int64) (models.VPNClient, bool, error) {
	return r.GetOrCreateVPNClientForDevice(userID, telegramID, nil)
}

func (r *VpnRepo) GetOrCreateVPNClientForDevice(userID uint, telegramID int64, deviceID *string) (models.VPNClient, bool, error) {
	var client models.VPNClient
	query := r.DB.Where("user_id = ?", userID)
	if deviceID == nil || strings.TrimSpace(*deviceID) == "" {
		query = query.Where("device_id IS NULL")
		deviceID = nil
	} else {
		cleanDeviceID := strings.TrimSpace(*deviceID)
		deviceID = &cleanDeviceID
		query = query.Where("device_id = ?", cleanDeviceID)
	}
	err := query.Take(&client).Error
	if err == nil {
		return client, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.VPNClient{}, false, err
	}

	code := generateClientCode()
	client = models.VPNClient{
		UserID:         userID,
		DeviceID:       deviceID,
		TelegramID:     telegramID,
		ClientCode:     code,
		Email:          code,
		VlessUUID:      uuid.NewString(),
		TrojanPassword: uuid.NewString(),
	}
	if err := r.DB.Create(&client).Error; err != nil {
		return models.VPNClient{}, false, err
	}
	return client, true, nil
}

func (r *VpnRepo) GetOrCreateSubscription(userID uint) (models.UserSubscription, error) {
	var subscription models.UserSubscription
	err := r.DB.Where("user_id = ?", userID).Take(&subscription).Error
	if err == nil {
		return subscription, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.UserSubscription{}, err
	}
	subscription = models.UserSubscription{UserID: userID, Status: models.SubscriptionStatusActive, TariffName: "default", DeviceLimit: 1}
	if err := r.DB.Create(&subscription).Error; err != nil {
		return models.UserSubscription{}, err
	}
	return subscription, nil
}

func (r *VpnRepo) UpdateSubscription(subscription models.UserSubscription) (models.UserSubscription, error) {
	updates := map[string]any{
		"status":       subscription.Status,
		"tariff_name":  subscription.TariffName,
		"expires_at":   subscription.ExpiresAt,
		"device_limit": subscription.DeviceLimit,
		"updated_at":   time.Now(),
	}
	if err := r.DB.Model(&models.UserSubscription{}).Where("user_id = ?", subscription.UserID).Updates(updates).Error; err != nil {
		return models.UserSubscription{}, err
	}
	return r.GetOrCreateSubscription(subscription.UserID)
}

func (r *VpnRepo) GetOrCreateRoutingSettings() (models.VPNRoutingSettings, error) {
	var settings models.VPNRoutingSettings
	err := r.DB.Where("id = ?", 1).Take(&settings).Error
	if err == nil {
		return settings, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.VPNRoutingSettings{}, err
	}
	settings = models.VPNRoutingSettings{ID: 1, AutoMode: models.AutoServerModeAutomatic}
	if err := r.DB.Create(&settings).Error; err != nil {
		return models.VPNRoutingSettings{}, err
	}
	return settings, nil
}

func (r *VpnRepo) UpdateRoutingSettings(autoMode, autoServerID string) (models.VPNRoutingSettings, error) {
	if _, err := r.GetOrCreateRoutingSettings(); err != nil {
		return models.VPNRoutingSettings{}, err
	}
	if err := r.DB.Model(&models.VPNRoutingSettings{}).Where("id = ?", 1).Updates(map[string]any{
		"auto_mode":      autoMode,
		"auto_server_id": autoServerID,
		"updated_at":     time.Now(),
	}).Error; err != nil {
		return models.VPNRoutingSettings{}, err
	}
	return r.GetOrCreateRoutingSettings()
}

func (r *VpnRepo) ListDevicesByUserID(userID uint) ([]models.MobileDevice, error) {
	var devices []models.MobileDevice
	err := r.DB.Where("user_id = ?", userID).Order("created_at ASC").Find(&devices).Error
	return devices, err
}

func (r *VpnRepo) ListRegisteredServers() ([]models.ServerRegistry, error) {
	var servers []models.ServerRegistry
	err := r.DB.Where("archived_at IS NULL").Order("display_name ASC, server_id ASC").Find(&servers).Error
	return servers, err
}

func (r *VpnRepo) ListServerAccessByUserID(userID uint) ([]models.UserServerAccess, error) {
	var access []models.UserServerAccess
	err := r.DB.Where("user_id = ?", userID).Order("server_id ASC").Find(&access).Error
	return access, err
}

func (r *VpnRepo) SetServerAccess(access models.UserServerAccess) (models.UserServerAccess, error) {
	var stored models.UserServerAccess
	err := r.DB.Where("user_id = ? AND server_id = ?", access.UserID, access.ServerID).Take(&stored).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		access.ServerID = strings.TrimSpace(access.ServerID)
		if err := r.DB.Create(&access).Error; err != nil {
			return models.UserServerAccess{}, err
		}
		return access, nil
	}
	if err != nil {
		return models.UserServerAccess{}, err
	}
	if err := r.DB.Model(&stored).Updates(map[string]any{
		"enabled":      access.Enabled,
		"allow_vless":  access.AllowVLESS,
		"allow_trojan": access.AllowTrojan,
		"valid_until":  access.ValidUntil,
		"updated_at":   time.Now(),
	}).Error; err != nil {
		return models.UserServerAccess{}, err
	}
	if err := r.DB.Where("id = ?", stored.ID).Take(&stored).Error; err != nil {
		return models.UserServerAccess{}, err
	}
	return stored, nil
}

func generateClientCode() string {
	sum := sha1.Sum([]byte(uuid.NewString()))
	return fmt.Sprintf("cvn_%x", sum[:4])
}

func (r *VpnRepo) GetOrCreateEndpointGroup(code string) (models.EndpointGroup, error) {
	var group models.EndpointGroup
	if err := r.DB.Where("code = ?", code).Take(&group).Error; err == nil {
		return group, nil
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.EndpointGroup{}, err
	}

	switch code {
	case "direct":
		group = models.EndpointGroup{Code: "direct", Name: "Direct", Protocol: "vless", PublicHost: "raven.net.ru", PublicPort: 443, Security: "reality", Network: "tcp", SNI: "yahoo.com", Flow: "xtls-rprx-vision", Enabled: true}
	case "ru":
		group = models.EndpointGroup{Code: "ru", Name: "RU", Protocol: "trojan", PublicHost: "br.raven.net.ru", PublicPort: 443, Security: "tls", Network: "tcp", SNI: "lofin.raven.net.ru", Enabled: true}
	default:
		return models.EndpointGroup{}, fmt.Errorf("unsupported endpoint group %q", code)
	}
	if err := r.DB.Create(&group).Error; err != nil {
		return models.EndpointGroup{}, err
	}
	return group, nil
}

func (r *VpnRepo) EnabledNodesByGroup(group string) ([]models.NodeState, error) {
	var nodes []models.NodeState
	err := r.DB.Table("node_states").
		Select("node_states.*").
		Joins("JOIN server_registry ON server_registry.server_id = node_states.server_id").
		Where("node_states.endpoint_group = ? AND node_states.expected_protocol = ? AND node_states.enabled = ?", group, protocolForEndpointGroup(group), true).
		Where("server_registry.endpoint_group = ? AND server_registry.expected_protocol = ?", group, protocolForEndpointGroup(group)).
		Where("node_states.server_id <> ''").
		Where("server_registry.enabled = ? AND server_registry.archived_at IS NULL", true).
		Order("node_states.server_id ASC, node_states.node_id ASC").
		Find(&nodes).Error
	if err != nil {
		return nil, err
	}
	return nodes, nil
}

// EnabledNodesByGroupForUser applies explicit per-user server grants. Users
// without any grants retain the legacy behaviour and target every eligible
// server in the endpoint group. Once the first grant exists, the list becomes
// an allow-list, including disabled rows.
func (r *VpnRepo) EnabledNodesByGroupForUser(userID uint, group, protocol, onlyServerID string) ([]models.NodeState, error) {
	nodes, err := r.EnabledNodesByGroup(group)
	if err != nil {
		return nil, err
	}
	access, err := r.ListServerAccessByUserID(userID)
	if err != nil {
		return nil, err
	}
	onlyServerID = strings.TrimSpace(onlyServerID)
	if len(access) == 0 {
		if onlyServerID == "" {
			return nodes, nil
		}
		return filterNodeStatesByServerIDs(nodes, map[string]struct{}{onlyServerID: {}}), nil
	}

	now := time.Now()
	allowed := make(map[string]struct{}, len(access))
	for _, item := range access {
		if !item.Enabled || (item.ValidUntil != nil && !item.ValidUntil.After(now)) {
			continue
		}
		if strings.EqualFold(protocol, "vless") && !item.AllowVLESS {
			continue
		}
		if strings.EqualFold(protocol, "trojan") && !item.AllowTrojan {
			continue
		}
		if onlyServerID != "" && item.ServerID != onlyServerID {
			continue
		}
		allowed[item.ServerID] = struct{}{}
	}
	return filterNodeStatesByServerIDs(nodes, allowed), nil
}

func filterNodeStatesByServerIDs(nodes []models.NodeState, allowed map[string]struct{}) []models.NodeState {
	filtered := make([]models.NodeState, 0, len(nodes))
	for _, node := range nodes {
		serverID := strings.TrimSpace(node.ServerID)
		if serverID == "" {
			serverID = strings.TrimSpace(node.NodeID)
		}
		if _, ok := allowed[serverID]; ok {
			filtered = append(filtered, node)
		}
	}
	return filtered
}

func protocolForEndpointGroup(group string) string {
	switch group {
	case "ru":
		return "trojan"
	default:
		return "vless"
	}
}

func (r *VpnRepo) GetProfile(clientID uint, profile string) (models.VPNProfile, error) {
	var vpnProfile models.VPNProfile
	err := r.DB.Preload("Nodes").Where("vpn_client_id = ? AND profile = ?", clientID, profile).Take(&vpnProfile).Error
	return vpnProfile, err
}

func (r *VpnRepo) CreateProfileWithNodes(profile models.VPNProfile, nodes []models.NodeState) (models.VPNProfile, error) {
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&profile).Error; err != nil {
			return err
		}
		for _, node := range nodes {
			nodeID := node.ServerID
			if nodeID == "" {
				nodeID = node.NodeID
			}
			profileNode := models.VPNProfileNode{VPNProfileID: profile.ID, ServerID: nodeID, NodeID: nodeID, Protocol: profile.Protocol, Status: models.VPNProfileNodeStatusPending}
			if err := tx.Create(&profileNode).Error; err != nil {
				return err
			}
			profile.Nodes = append(profile.Nodes, profileNode)
		}
		return nil
	})
	if err != nil {
		return models.VPNProfile{}, err
	}
	return profile, nil
}

func (r *VpnRepo) EnsureProfileNodes(profile models.VPNProfile, nodes []models.NodeState) (models.VPNProfile, []models.VPNProfileNode, error) {
	created := []models.VPNProfileNode{}
	err := r.DB.Transaction(func(tx *gorm.DB) error {
		for _, node := range nodes {
			serverID := node.ServerID
			if serverID == "" {
				serverID = node.NodeID
			}
			if serverID == "" {
				continue
			}
			var existing models.VPNProfileNode
			err := tx.Where("vpn_profile_id = ? AND (server_id = ? OR node_id = ?)", profile.ID, serverID, serverID).Take(&existing).Error
			if err == nil {
				updates := map[string]any{
					"server_id":  serverID,
					"node_id":    serverID,
					"protocol":   profile.Protocol,
					"updated_at": time.Now(),
				}
				// A successful deployment is durable domain state. Rebuilding a
				// provisioning plan must not turn it back into pending.
				if existing.Status != models.VPNProfileNodeStatusSuccess {
					updates["status"] = models.VPNProfileNodeStatusPending
					updates["last_error"] = ""
					updates["applied_at"] = nil
					updates["next_attempt_at"] = nil
				} else {
					updates["next_attempt_at"] = nil
				}
				if err := tx.Model(&existing).Updates(updates).Error; err != nil {
					return err
				}
				continue
			}
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			profileNode := models.VPNProfileNode{VPNProfileID: profile.ID, ServerID: serverID, NodeID: serverID, Protocol: profile.Protocol, Status: models.VPNProfileNodeStatusPending}
			if err := tx.Create(&profileNode).Error; err != nil {
				return err
			}
			created = append(created, profileNode)
		}
		return nil
	})
	if err != nil {
		return models.VPNProfile{}, nil, err
	}
	profile, err = r.GetProfileByID(profile.ID)
	if err != nil {
		return models.VPNProfile{}, nil, err
	}
	return profile, created, nil
}

func (r *VpnRepo) IsServerEligible(serverID string, group string, protocol string) (bool, error) {
	var count int64
	err := r.DB.Table("node_states").
		Joins("JOIN server_registry ON server_registry.server_id = node_states.server_id").
		Where("node_states.server_id = ?", serverID).
		Where("node_states.endpoint_group = ? AND node_states.expected_protocol = ? AND node_states.enabled = ?", group, protocol, true).
		Where("server_registry.endpoint_group = ? AND server_registry.expected_protocol = ?", group, protocol).
		Where("server_registry.enabled = ? AND server_registry.archived_at IS NULL", true).
		Count(&count).Error
	return count > 0, err
}

func (r *VpnRepo) MarkProfileNodePublished(nodeID uint, publishedAt time.Time, nextAttemptAt time.Time) error {
	return r.DB.Model(&models.VPNProfileNode{}).
		Where("id = ? AND status = ?", nodeID, models.VPNProfileNodeStatusPending).
		Updates(map[string]any{
			"attempts":          gorm.Expr("attempts + 1"),
			"last_published_at": &publishedAt,
			"next_attempt_at":   &nextAttemptAt,
			"last_error":        "",
			"updated_at":        publishedAt,
		}).Error
}

func (r *VpnRepo) MarkProfileNodePublishFailed(nodeID uint, errText string, attemptedAt time.Time, nextAttemptAt time.Time) error {
	return r.DB.Model(&models.VPNProfileNode{}).
		Where("id = ? AND status <> ?", nodeID, models.VPNProfileNodeStatusSuccess).
		Updates(map[string]any{
			"status":          models.VPNProfileNodeStatusPending,
			"attempts":        gorm.Expr("attempts + 1"),
			"next_attempt_at": &nextAttemptAt,
			"last_error":      errText,
			"updated_at":      attemptedAt,
		}).Error
}

func (r *VpnRepo) PendingProfileNodes(now time.Time, limit int) ([]models.VPNProfileNode, error) {
	if limit <= 0 {
		limit = 100
	}
	var nodes []models.VPNProfileNode
	err := r.DB.Where("status = ? AND (next_attempt_at IS NULL OR next_attempt_at <= ?)", models.VPNProfileNodeStatusPending, now).
		Order("CASE WHEN next_attempt_at IS NULL THEN 0 ELSE 1 END, next_attempt_at ASC, id ASC").
		Limit(limit).
		Find(&nodes).Error
	return nodes, err
}

func (r *VpnRepo) UpdateProfileStatus(profileID uint, status string, lastError string) error {
	return r.DB.Model(&models.VPNProfile{}).Where("id = ?", profileID).Updates(map[string]any{"status": status, "last_error": lastError, "updated_at": time.Now()}).Error
}

func (r *VpnRepo) TouchProfilePublishError(profileID uint, errText string) error {
	return r.DB.Model(&models.VPNProfile{}).Where("id = ?", profileID).Updates(map[string]any{"last_error": errText, "updated_at": time.Now()}).Error
}

func (r *VpnRepo) GetProfileByID(profileID uint) (models.VPNProfile, error) {
	var profile models.VPNProfile
	err := r.DB.Preload("VPNClient").Preload("Nodes").Where("id = ?", profileID).Take(&profile).Error
	return profile, err
}

func (r *VpnRepo) GetProfileNodeByID(nodeID uint) (models.VPNProfileNode, error) {
	var node models.VPNProfileNode
	err := r.DB.Where("id = ?", nodeID).Take(&node).Error
	return node, err
}

func (r *VpnRepo) PrepareProfileNodeAction(nodeID uint, desiredState, action string) (models.VPNProfileNode, error) {
	now := time.Now().UTC()
	if err := r.DB.Model(&models.VPNProfileNode{}).Where("id = ?", nodeID).Updates(map[string]any{
		"desired_state":   desiredState,
		"pending_action":  action,
		"status":          models.VPNProfileNodeStatusPending,
		"last_error":      "",
		"next_attempt_at": nil,
		"updated_at":      now,
	}).Error; err != nil {
		return models.VPNProfileNode{}, err
	}
	return r.GetProfileNodeByID(nodeID)
}

func (r *VpnRepo) ApplyProfileNodeActionResult(nodeID uint, action, resultStatus, lastError string, appliedAt time.Time) (models.VPNProfileNode, error) {
	var node models.VPNProfileNode
	if err := r.DB.Where("id = ?", nodeID).Take(&node).Error; err != nil {
		return models.VPNProfileNode{}, err
	}
	status := models.VPNProfileNodeStatusFailed
	if resultStatus == models.VPNProfileNodeStatusSuccess {
		switch action {
		case "disable_client":
			status = models.VPNProfileNodeStatusDisabled
		case "delete_client":
			status = models.VPNProfileNodeStatusDeleted
		default:
			status = models.VPNProfileNodeStatusSuccess
		}
	}
	if err := r.DB.Model(&node).Updates(map[string]any{
		"status":          status,
		"pending_action":  "",
		"last_error":      lastError,
		"applied_at":      &appliedAt,
		"next_attempt_at": nil,
		"updated_at":      appliedAt,
	}).Error; err != nil {
		return models.VPNProfileNode{}, err
	}
	return r.GetProfileNodeByID(nodeID)
}

func (r *VpnRepo) GetEndpointGroup(code string) (models.EndpointGroup, error) {
	var group models.EndpointGroup
	err := r.DB.Where("code = ?", code).Take(&group).Error
	return group, err
}

func (r *VpnRepo) ApplyProfileNodeResult(profile models.VPNProfile, nodeID string, protocol string, status string, inboundID *int, lastError string, appliedAt time.Time) (models.VPNProfileNode, bool, error) {
	var node models.VPNProfileNode
	err := r.DB.Where("vpn_profile_id = ? AND (server_id = ? OR node_id = ?)", profile.ID, nodeID, nodeID).Take(&node).Error
	created := false
	if errors.Is(err, gorm.ErrRecordNotFound) {
		node = models.VPNProfileNode{VPNProfileID: profile.ID, ServerID: nodeID, NodeID: nodeID}
		created = true
	} else if err != nil {
		return models.VPNProfileNode{}, false, err
	}

	duplicate := !created && node.Status == status && node.Protocol == protocol && node.LastError == lastError && sameIntPtr(node.InboundID, inboundID)
	if node.ServerID == "" {
		node.ServerID = nodeID
	}
	if node.NodeID == "" {
		node.NodeID = nodeID
	}
	node.Protocol = protocol
	// Success is terminal for one profile/server deployment. A delayed failure
	// from an earlier redelivery must not downgrade an already active node.
	if !created && node.Status == models.VPNProfileNodeStatusSuccess && status == models.VPNProfileNodeStatusFailed {
		return node, true, nil
	}
	node.Status = status
	node.InboundID = inboundID
	node.LastError = lastError
	node.AppliedAt = &appliedAt
	node.NextAttemptAt = nil
	if created {
		if err := r.DB.Create(&node).Error; err != nil {
			return models.VPNProfileNode{}, false, err
		}
		return node, false, nil
	}
	if err := r.DB.Save(&node).Error; err != nil {
		return models.VPNProfileNode{}, false, err
	}
	return node, duplicate, nil
}

func sameIntPtr(left, right *int) bool {
	if left == nil || right == nil {
		return left == right
	}
	return *left == *right
}

func (r *VpnRepo) UpdateProfileFinalLink(profileID uint, finalLink string) (models.VPNProfile, error) {
	if err := r.DB.Model(&models.VPNProfile{}).Where("id = ?", profileID).Updates(map[string]any{"final_link": finalLink, "updated_at": time.Now()}).Error; err != nil {
		return models.VPNProfile{}, err
	}
	return r.GetProfileByID(profileID)
}

func (r *VpnRepo) UpdateProfileResult(profileID uint, status string, finalLink string, lastError string, notifiedAt *time.Time) (models.VPNProfile, error) {
	updates := map[string]any{
		"status":     status,
		"final_link": finalLink,
		"last_error": lastError,
		"updated_at": time.Now(),
	}
	if notifiedAt != nil {
		updates["notified_at"] = notifiedAt
	}
	if err := r.DB.Model(&models.VPNProfile{}).Where("id = ?", profileID).Updates(updates).Error; err != nil {
		return models.VPNProfile{}, err
	}
	return r.GetProfileByID(profileID)
}
