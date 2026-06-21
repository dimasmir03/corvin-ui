package service

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type captureJobPublisher struct {
	messages []broker.JobTask
}

func (p *captureJobPublisher) PublishJob(msg broker.JobTask) error {
	p.messages = append(p.messages, msg)
	return nil
}

func newVPNServiceTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dbName := strings.NewReplacer("/", "_", " ", "_").Replace(t.Name())
	db, err := gorm.Open(sqlite.Open("file:"+dbName+"?mode=memory&cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open sqlite: %v", err)
	}
	if err := db.AutoMigrate(
		&models.User{},
		&models.Telegram{},
		&models.Vpn{},
		&models.NodeState{},
		&models.Server{},
		&models.EndpointGroup{},
		&models.VPNClient{},
		&models.VPNProfile{},
		&models.VPNProfileNode{},
		&models.JobBatch{},
		&models.Job{},
		&models.AuditLog{},
	); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return db
}

func seedVPNServiceCreateData(t *testing.T, db *gorm.DB) models.Telegram {
	t.Helper()
	user := models.User{Username: "tg-user", Status: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	telegram := models.Telegram{TgID: 123456789, Username: "tg", Firstname: "T", Lastname: "G", UserID: user.ID}
	if err := db.Create(&telegram).Error; err != nil {
		t.Fatalf("create telegram: %v", err)
	}

	nodes := []models.NodeState{
		{NodeID: "direct-1", EndpointGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.ServerStatusOnline, Enabled: true},
		{NodeID: "direct-2", EndpointGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.ServerStatusOnline, Enabled: true},
		{NodeID: "ru-1", EndpointGroup: jobsvc.EndpointGroupRU, Protocol: jobsvc.VPNProfileTrojan, Status: models.ServerStatusOnline, Enabled: true},
	}
	for _, node := range nodes {
		if err := db.Create(&node).Error; err != nil {
			t.Fatalf("create node %s: %v", node.NodeID, err)
		}
	}

	servers := []models.Server{
		{Name: "direct-1", IP: "10.0.0.1", Port: 443, ApiKey: "x", Type: jobsvc.VPNProfileVLESS, NodeRole: jobsvc.EndpointGroupDirect, Enabled: true, Status: models.ServerStatusOnline, ManagementMode: models.ServerManagementModeAgent},
		{Name: "direct-2", IP: "10.0.0.2", Port: 443, ApiKey: "x", Type: jobsvc.VPNProfileVLESS, NodeRole: jobsvc.EndpointGroupDirect, Enabled: true, Status: models.ServerStatusOnline, ManagementMode: models.ServerManagementModeAgent},
		{Name: "ru-1", IP: "10.0.1.1", Port: 443, ApiKey: "x", Type: jobsvc.VPNProfileTrojan, NodeRole: jobsvc.EndpointGroupRU, Enabled: true, Status: models.ServerStatusOnline, ManagementMode: models.ServerManagementModeAgent},
	}
	for _, server := range servers {
		if err := db.Create(&server).Error; err != nil {
			t.Fatalf("create server %s: %v", server.Name, err)
		}
	}

	return telegram
}

func newTestVPNService(db *gorm.DB, publisher jobsvc.JobPublisher) *VPNService {
	jobs := jobsvc.NewService(repository.NewJobsRepo(db), repository.NewServerRepo(db), nil, publisher)
	return NewVPNService(repository.NewVpnRepo(db), repository.NewTelegramRepo(db), jobs, nil)
}

func TestRequestCreateVPNVLESSCreatesCanonicalProfileAndPayload(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	result, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS})
	if err != nil {
		t.Fatalf("RequestCreateVPN: %v", err)
	}
	if result.Protocol != jobsvc.VPNProfileVLESS {
		t.Fatalf("protocol = %q", result.Protocol)
	}

	var client models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&client).Error; err != nil {
		t.Fatalf("vpn client not created: %v", err)
	}
	if client.ClientCode == "" || client.Email == "" || client.VlessUUID == "" || client.TrojanPassword == "" {
		t.Fatalf("canonical credentials are incomplete: %#v", client)
	}

	var profile models.VPNProfile
	if err := db.Preload("Nodes").Where("vpn_client_id = ? AND profile = ?", client.ID, jobsvc.VPNProfileVLESS).Take(&profile).Error; err != nil {
		t.Fatalf("vless profile not created: %v", err)
	}
	if profile.EndpointGroup != jobsvc.EndpointGroupDirect || profile.Status != models.VPNProfileStatusPending {
		t.Fatalf("unexpected profile group/status: %s/%s", profile.EndpointGroup, profile.Status)
	}
	if len(profile.Nodes) != 2 {
		t.Fatalf("profile nodes = %d, want 2", len(profile.Nodes))
	}

	if len(publisher.messages) != 2 {
		t.Fatalf("published messages = %d, want 2", len(publisher.messages))
	}
	payload := publisher.messages[0]
	if payload.ProfileID != profile.ID || payload.Profile != jobsvc.VPNProfileVLESS || payload.TargetGroup != jobsvc.EndpointGroupDirect {
		t.Fatalf("unexpected payload profile fields: %#v", payload)
	}
	if payload.Credentials.VLESS.ID != client.VlessUUID || payload.Credentials.Trojan.Password != client.TrojanPassword {
		t.Fatalf("payload does not carry canonical credentials")
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payloadString := string(payloadJSON)
	for _, forbidden := range []string{"public_host", "public_domain", "final_link", "subscription_link", "raven.net.ru", "br.raven.net.ru"} {
		if strings.Contains(payloadString, forbidden) {
			t.Fatalf("payload contains forbidden %q: %s", forbidden, payloadString)
		}
	}
}

func TestRequestCreateVPNTrojanReusesCanonicalClient(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); err != nil {
		t.Fatalf("create vless: %v", err)
	}
	var before models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&before).Error; err != nil {
		t.Fatalf("load client before: %v", err)
	}

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileTrojan}); err != nil {
		t.Fatalf("create trojan: %v", err)
	}
	var after models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&after).Error; err != nil {
		t.Fatalf("load client after: %v", err)
	}
	if before.ID != after.ID || before.ClientCode != after.ClientCode || before.VlessUUID != after.VlessUUID || before.TrojanPassword != after.TrojanPassword {
		t.Fatalf("canonical credentials changed: before=%#v after=%#v", before, after)
	}

	var profile models.VPNProfile
	if err := db.Preload("Nodes").Where("vpn_client_id = ? AND profile = ?", after.ID, jobsvc.VPNProfileTrojan).Take(&profile).Error; err != nil {
		t.Fatalf("trojan profile not created: %v", err)
	}
	if profile.EndpointGroup != jobsvc.EndpointGroupRU || len(profile.Nodes) != 1 {
		t.Fatalf("unexpected trojan profile: group=%s nodes=%d", profile.EndpointGroup, len(profile.Nodes))
	}
	last := publisher.messages[len(publisher.messages)-1]
	if last.TargetGroup != jobsvc.EndpointGroupRU || last.Profile != jobsvc.VPNProfileTrojan {
		t.Fatalf("unexpected trojan routing payload: %#v", last)
	}
}

func TestRequestCreateVPNRepeatedCreateDoesNotDuplicate(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); err != nil {
		t.Fatalf("first create: %v", err)
	}
	published := len(publisher.messages)
	var clientBefore models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&clientBefore).Error; err != nil {
		t.Fatalf("load client before: %v", err)
	}

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); err != nil {
		t.Fatalf("repeated create: %v", err)
	}
	if len(publisher.messages) != published {
		t.Fatalf("repeated create published new messages: got %d want %d", len(publisher.messages), published)
	}

	var clients int64
	if err := db.Model(&models.VPNClient{}).Where("user_id = ?", telegram.UserID).Count(&clients).Error; err != nil {
		t.Fatalf("count clients: %v", err)
	}
	if clients != 1 {
		t.Fatalf("clients = %d, want 1", clients)
	}
	var profiles int64
	if err := db.Model(&models.VPNProfile{}).Where("vpn_client_id = ? AND profile = ?", clientBefore.ID, jobsvc.VPNProfileVLESS).Count(&profiles).Error; err != nil {
		t.Fatalf("count profiles: %v", err)
	}
	if profiles != 1 {
		t.Fatalf("profiles = %d, want 1", profiles)
	}
	var profile models.VPNProfile
	if err := db.Where("vpn_client_id = ? AND profile = ?", clientBefore.ID, jobsvc.VPNProfileVLESS).Take(&profile).Error; err != nil {
		t.Fatalf("load profile: %v", err)
	}
	var profileNodes int64
	if err := db.Model(&models.VPNProfileNode{}).Where("vpn_profile_id = ?", profile.ID).Count(&profileNodes).Error; err != nil {
		t.Fatalf("count profile nodes: %v", err)
	}
	if profileNodes != 2 {
		t.Fatalf("profile nodes = %d, want 2", profileNodes)
	}

	var clientAfter models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&clientAfter).Error; err != nil {
		t.Fatalf("load client after: %v", err)
	}
	if clientBefore.ClientCode != clientAfter.ClientCode || clientBefore.VlessUUID != clientAfter.VlessUUID || clientBefore.TrojanPassword != clientAfter.TrojanPassword {
		t.Fatalf("credentials changed on repeated create")
	}
}

func TestRequestCreateVPNAllCreatesVLESSAndTrojanProfiles(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	result, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: "all"})
	if err != nil {
		t.Fatalf("RequestCreateVPN all: %v", err)
	}
	if result.Protocol != "vless,trojan" {
		t.Fatalf("protocol = %q, want all profiles", result.Protocol)
	}

	var client models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&client).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	for _, profileName := range []string{jobsvc.VPNProfileVLESS, jobsvc.VPNProfileTrojan} {
		var count int64
		if err := db.Model(&models.VPNProfile{}).Where("vpn_client_id = ? AND profile = ?", client.ID, profileName).Count(&count).Error; err != nil {
			t.Fatalf("count %s profile: %v", profileName, err)
		}
		if count != 1 {
			t.Fatalf("%s profiles = %d, want 1", profileName, count)
		}
	}
	if len(publisher.messages) != 3 {
		t.Fatalf("published messages = %d, want 3", len(publisher.messages))
	}
}

func TestRequestCreateVPNNoEnabledNodesCreatesFailedProfileWithoutPublish(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	if err := db.Model(&models.NodeState{}).Where("endpoint_group = ?", jobsvc.EndpointGroupRU).Update("enabled", false).Error; err != nil {
		t.Fatalf("disable ru nodes: %v", err)
	}
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileTrojan}); err != nil {
		t.Fatalf("RequestCreateVPN trojan: %v", err)
	}

	var client models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&client).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	var profile models.VPNProfile
	if err := db.Preload("Nodes").Where("vpn_client_id = ? AND profile = ?", client.ID, jobsvc.VPNProfileTrojan).Take(&profile).Error; err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if profile.Status != models.VPNProfileStatusFailed || profile.LastError == "" {
		t.Fatalf("profile status/error = %s/%q, want failed with error", profile.Status, profile.LastError)
	}
	if len(profile.Nodes) != 0 {
		t.Fatalf("profile nodes = %d, want 0", len(profile.Nodes))
	}
	if len(publisher.messages) != 0 {
		t.Fatalf("published messages = %d, want 0", len(publisher.messages))
	}
}

func TestApplyJobResultAllNodesSuccessActivatesProfile(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); err != nil {
		t.Fatalf("create vless: %v", err)
	}
	profile := loadTestProfile(t, db, telegram.UserID, jobsvc.VPNProfileVLESS)
	for _, nodeID := range []string{"direct-1", "direct-2"} {
		if _, err := svc.ApplyJobResult(context.Background(), broker.JobResultEvent{ProfileID: profile.ID, NodeID: nodeID, Profile: jobsvc.VPNProfileVLESS, TargetGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.VPNProfileNodeStatusSuccess, ClientCode: profile.VPNClient.ClientCode}); err != nil {
			t.Fatalf("apply success for %s: %v", nodeID, err)
		}
	}

	profile = loadTestProfileByID(t, db, profile.ID)
	if profile.Status != models.VPNProfileStatusActive {
		t.Fatalf("profile status = %s, want active", profile.Status)
	}
	for _, node := range profile.Nodes {
		if node.Status != models.VPNProfileNodeStatusSuccess || node.AppliedAt == nil {
			t.Fatalf("unexpected node result: %#v", node)
		}
	}
}

func TestApplyJobResultSuccessAndFailedMakesPartial(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); err != nil {
		t.Fatalf("create vless: %v", err)
	}
	profile := loadTestProfile(t, db, telegram.UserID, jobsvc.VPNProfileVLESS)
	if _, err := svc.ApplyJobResult(context.Background(), broker.JobResultEvent{ProfileID: profile.ID, NodeID: "direct-1", Profile: jobsvc.VPNProfileVLESS, TargetGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.VPNProfileNodeStatusSuccess, ClientCode: profile.VPNClient.ClientCode}); err != nil {
		t.Fatalf("apply success: %v", err)
	}
	errText := "3x-ui create client failed"
	if _, err := svc.ApplyJobResult(context.Background(), broker.JobResultEvent{ProfileID: profile.ID, NodeID: "direct-2", Profile: jobsvc.VPNProfileVLESS, TargetGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.VPNProfileNodeStatusFailed, ClientCode: profile.VPNClient.ClientCode, Error: &errText}); err != nil {
		t.Fatalf("apply failed: %v", err)
	}

	profile = loadTestProfileByID(t, db, profile.ID)
	if profile.Status != models.VPNProfileStatusPartial {
		t.Fatalf("profile status = %s, want partial", profile.Status)
	}
	if !strings.Contains(profile.LastError, "direct-2") {
		t.Fatalf("profile last_error = %q, want failed node", profile.LastError)
	}
	for _, node := range profile.Nodes {
		if node.NodeID == "direct-2" && node.LastError != errText {
			t.Fatalf("failed node last_error = %q", node.LastError)
		}
	}
}

func TestApplyJobResultAllFailedMakesProfileFailed(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); err != nil {
		t.Fatalf("create vless: %v", err)
	}
	profile := loadTestProfile(t, db, telegram.UserID, jobsvc.VPNProfileVLESS)
	for _, nodeID := range []string{"direct-1", "direct-2"} {
		errText := "failed " + nodeID
		if _, err := svc.ApplyJobResult(context.Background(), broker.JobResultEvent{ProfileID: profile.ID, NodeID: nodeID, Profile: jobsvc.VPNProfileVLESS, TargetGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.VPNProfileNodeStatusFailed, ClientCode: profile.VPNClient.ClientCode, Error: &errText}); err != nil {
			t.Fatalf("apply failed for %s: %v", nodeID, err)
		}
	}

	profile = loadTestProfileByID(t, db, profile.ID)
	if profile.Status != models.VPNProfileStatusFailed {
		t.Fatalf("profile status = %s, want failed", profile.Status)
	}
	if profile.NotifiedAt != nil {
		t.Fatalf("failed profile must not be marked notified")
	}
}

func TestApplyJobResultDuplicateDoesNotNotifyTwice(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileTrojan}); err != nil {
		t.Fatalf("create trojan: %v", err)
	}
	profile := loadTestProfile(t, db, telegram.UserID, jobsvc.VPNProfileTrojan)
	event := broker.JobResultEvent{ProfileID: profile.ID, NodeID: "ru-1", Profile: jobsvc.VPNProfileTrojan, TargetGroup: jobsvc.EndpointGroupRU, Protocol: jobsvc.VPNProfileTrojan, Status: models.VPNProfileNodeStatusSuccess, ClientCode: profile.VPNClient.ClientCode}
	first, err := svc.ApplyJobResult(context.Background(), event)
	if err != nil {
		t.Fatalf("first apply: %v", err)
	}
	if first == nil || first.TgID != telegram.TgID || first.Link == "" {
		t.Fatalf("first notification = %#v, want vpn ready", first)
	}
	second, err := svc.ApplyJobResult(context.Background(), event)
	if err != nil {
		t.Fatalf("duplicate apply: %v", err)
	}
	if second != nil {
		t.Fatalf("duplicate notification = %#v, want nil", second)
	}
	var nodes int64
	if err := db.Model(&models.VPNProfileNode{}).Where("vpn_profile_id = ? AND node_id = ?", profile.ID, "ru-1").Count(&nodes).Error; err != nil {
		t.Fatalf("count nodes: %v", err)
	}
	if nodes != 1 {
		t.Fatalf("nodes = %d, want 1", nodes)
	}
}

func TestApplyJobResultGeneratesFinalLinkFromPanelData(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); err != nil {
		t.Fatalf("create vless: %v", err)
	}
	profile := loadTestProfile(t, db, telegram.UserID, jobsvc.VPNProfileVLESS)
	if err := db.Model(&models.VPNProfile{}).Where("id = ?", profile.ID).Update("final_link", "").Error; err != nil {
		t.Fatalf("clear final_link: %v", err)
	}
	notification, err := svc.ApplyJobResult(context.Background(), broker.JobResultEvent{ProfileID: profile.ID, NodeID: "direct-1", Profile: jobsvc.VPNProfileVLESS, TargetGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.VPNProfileNodeStatusSuccess, ClientCode: profile.VPNClient.ClientCode})
	if err != nil {
		t.Fatalf("apply result: %v", err)
	}
	if notification == nil || notification.Link == "" {
		t.Fatalf("notification = %#v, want link", notification)
	}
	profile = loadTestProfileByID(t, db, profile.ID)
	if !strings.Contains(profile.FinalLink, "raven.net.ru") || !strings.Contains(profile.FinalLink, profile.VPNClient.VlessUUID) {
		t.Fatalf("final link was not generated from endpoint group and client credentials: %q", profile.FinalLink)
	}
	if strings.Contains(profile.FinalLink, "10.0.0.1") || strings.Contains(profile.FinalLink, "direct-1") {
		t.Fatalf("final link contains node private data: %q", profile.FinalLink)
	}
}

func loadTestProfile(t *testing.T, db *gorm.DB, userID uint, profileName string) models.VPNProfile {
	t.Helper()
	var client models.VPNClient
	if err := db.Where("user_id = ?", userID).Take(&client).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	var profile models.VPNProfile
	if err := db.Preload("VPNClient").Preload("Nodes").Where("vpn_client_id = ? AND profile = ?", client.ID, profileName).Take(&profile).Error; err != nil {
		t.Fatalf("load %s profile: %v", profileName, err)
	}
	return profile
}

func loadTestProfileByID(t *testing.T, db *gorm.DB, profileID uint) models.VPNProfile {
	t.Helper()
	var profile models.VPNProfile
	if err := db.Preload("VPNClient").Preload("Nodes").Where("id = ?", profileID).Take(&profile).Error; err != nil {
		t.Fatalf("load profile %d: %v", profileID, err)
	}
	return profile
}
