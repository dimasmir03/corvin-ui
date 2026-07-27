package repository

import (
	"crypto/sha1"
	"errors"
	"fmt"
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
	err := r.DB.Where("user_id = ?", userID).Take(&client).Error
	return client, err
}

func (r *VpnRepo) ListProfilesByClientID(clientID uint) ([]models.VPNProfile, error) {
	var profiles []models.VPNProfile
	err := r.DB.Preload("Nodes", func(db *gorm.DB) *gorm.DB {
		return db.Order("server_id ASC, node_id ASC")
	}).Where("vpn_client_id = ?", clientID).Order("profile ASC").Find(&profiles).Error
	return profiles, err
}

func (r *VpnRepo) GetOrCreateVPNClient(userID uint, telegramID int64) (models.VPNClient, bool, error) {
	var client models.VPNClient
	err := r.DB.Where("user_id = ?", userID).Take(&client).Error
	if err == nil {
		return client, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return models.VPNClient{}, false, err
	}

	code := generateClientCode()
	client = models.VPNClient{
		UserID:         userID,
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
					"status":     models.VPNProfileNodeStatusPending,
					"last_error": "",
					"applied_at": nil,
					"updated_at": time.Now(),
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
	node.Status = status
	node.InboundID = inboundID
	node.LastError = lastError
	node.AppliedAt = &appliedAt
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
