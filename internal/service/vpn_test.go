package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
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
		&models.ServerRegistry{},
		&models.ServerInbound{},
		&models.NodeStateSnapshot{},
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
		{ServerID: "direct-1", NodeID: "direct-1", EndpointGroup: jobsvc.EndpointGroupDirect, ExpectedProtocol: jobsvc.VPNProfileVLESS, ReportedProtocol: jobsvc.VPNProfileVLESS, Protocol: jobsvc.VPNProfileVLESS, Status: models.ServerStatusOnline, Enabled: true},
		{ServerID: "direct-2", NodeID: "direct-2", EndpointGroup: jobsvc.EndpointGroupDirect, ExpectedProtocol: jobsvc.VPNProfileVLESS, ReportedProtocol: jobsvc.VPNProfileVLESS, Protocol: jobsvc.VPNProfileVLESS, Status: models.ServerStatusOnline, Enabled: true},
		{ServerID: "ru-1", NodeID: "ru-1", EndpointGroup: jobsvc.EndpointGroupRU, ExpectedProtocol: jobsvc.VPNProfileTrojan, ReportedProtocol: jobsvc.VPNProfileTrojan, Protocol: jobsvc.VPNProfileTrojan, Status: models.ServerStatusOnline, Enabled: true},
	}
	for _, node := range nodes {
		if err := db.Create(&node).Error; err != nil {
			t.Fatalf("create node %s: %v", node.NodeID, err)
		}
	}

	registries := []models.ServerRegistry{
		{ServerID: "direct-1", DisplayName: "direct-1", EndpointGroup: jobsvc.EndpointGroupDirect, ExpectedProtocol: jobsvc.VPNProfileVLESS, Source: models.NodeSourceDiscovered, Enabled: true},
		{ServerID: "direct-2", DisplayName: "direct-2", EndpointGroup: jobsvc.EndpointGroupDirect, ExpectedProtocol: jobsvc.VPNProfileVLESS, Source: models.NodeSourceDiscovered, Enabled: true},
		{ServerID: "ru-1", DisplayName: "ru-1", EndpointGroup: jobsvc.EndpointGroupRU, ExpectedProtocol: jobsvc.VPNProfileTrojan, Source: models.NodeSourceDiscovered, Enabled: true},
	}
	for _, registry := range registries {
		if err := db.Create(&registry).Error; err != nil {
			t.Fatalf("create server registry %s: %v", registry.ServerID, err)
		}
	}

	return telegram
}

func newTestVPNService(db *gorm.DB, publisher jobsvc.JobPublisher) *VPNService {
	jobs := jobsvc.NewService(repository.NewJobsRepo(db), repository.NewServerRepo(db), nil, publisher)
	return NewVPNService(repository.NewVpnRepo(db), repository.NewTelegramRepo(db), jobs, nil)
}

func TestRequestCreateVPNVLESSCreatesProfileAndPayload(t *testing.T) {
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
	if result.BatchID == 0 || result.JobID == 0 || result.JobsCount != 2 {
		t.Fatalf("queued result = %#v, want non-zero batch/job and 2 jobs", result)
	}
	var batch models.JobBatch
	if err := db.Take(&batch, result.BatchID).Error; err != nil {
		t.Fatalf("load batch: %v", err)
	}
	if batch.Type != "create_vpn" {
		t.Fatalf("batch type = %q, want create_vpn", batch.Type)
	}

	var client models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&client).Error; err != nil {
		t.Fatalf("vpn client not created: %v", err)
	}
	if client.ClientCode == "" || client.Email == "" || client.VlessUUID == "" || client.TrojanPassword == "" {
		t.Fatalf("vpn client credentials are incomplete: %#v", client)
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
	if payload.ServerID == "" || payload.TargetServerID == "" || payload.ServerID != payload.TargetServerID {
		t.Fatalf("unexpected payload server identity: %#v", payload)
	}
	var job models.Job
	if err := db.Take(&job, payload.JobID).Error; err != nil {
		t.Fatalf("load job: %v", err)
	}
	if job.TargetServerID != payload.ServerID || job.ServerID != nil || job.Protocol != jobsvc.VPNProfileVLESS || job.Action != jobsvc.ActionCreateClient || job.ProfileID != profile.ID {
		t.Fatalf("unexpected job row: %#v", job)
	}
	if payload.Credentials.VLESS.ID != client.VlessUUID || payload.Credentials.Trojan.Password != client.TrojanPassword {
		t.Fatalf("payload does not carry vpn client credentials")
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("marshal payload: %v", err)
	}
	payloadString := string(payloadJSON)
	for _, forbidden := range []string{"node_id", "target_node_id", "public_host", "public_domain", "final_link", "subscription_link", "raven.net.ru", "br.raven.net.ru"} {
		if strings.Contains(payloadString, forbidden) {
			t.Fatalf("payload contains forbidden %q: %s", forbidden, payloadString)
		}
	}
}

func TestRequestCreateVPNTrojanReusesVPNClient(t *testing.T) {
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
		t.Fatalf("vpn client credentials changed: before=%#v after=%#v", before, after)
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

func TestRequestCreateVPNPendingProfileCreatesFreshJobsAndSupersedesOldJobs(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	first, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if first.BatchID == 0 || first.JobID == 0 || first.JobsCount != 2 {
		t.Fatalf("first result = %#v, want queued jobs", first)
	}
	var clientBefore models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&clientBefore).Error; err != nil {
		t.Fatalf("load client before: %v", err)
	}

	second, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS})
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	if second.BatchID == 0 || second.JobID == 0 || second.JobsCount != 2 || second.BatchID == first.BatchID {
		t.Fatalf("second result = %#v, want fresh non-zero batch/jobs", second)
	}
	if len(publisher.messages) != 4 {
		t.Fatalf("published messages = %d, want 4", len(publisher.messages))
	}

	var superseded int64
	if err := db.Model(&models.Job{}).Where("profile_id = ? AND batch_id = ? AND status = ?", loadTestProfile(t, db, telegram.UserID, jobsvc.VPNProfileVLESS).ID, first.BatchID, models.JobStatusSuperseded).Count(&superseded).Error; err != nil {
		t.Fatalf("count superseded jobs: %v", err)
	}
	if superseded != 2 {
		t.Fatalf("superseded jobs = %d, want 2", superseded)
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
	var profileNodes int64
	profile := loadTestProfile(t, db, telegram.UserID, jobsvc.VPNProfileVLESS)
	if err := db.Model(&models.VPNProfileNode{}).Where("vpn_profile_id = ? AND status = ?", profile.ID, models.VPNProfileNodeStatusPending).Count(&profileNodes).Error; err != nil {
		t.Fatalf("count profile nodes: %v", err)
	}
	if profileNodes != 2 {
		t.Fatalf("pending profile nodes = %d, want 2", profileNodes)
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

func TestRequestCreateVPNActiveProfileReturnsExistingLinkWithoutJobs(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	client := models.VPNClient{UserID: telegram.UserID, TelegramID: telegram.TgID, ClientCode: "cvn_active", Email: "cvn_active", VlessUUID: "vless-secret", TrojanPassword: "trojan-secret"}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("create vpn client: %v", err)
	}
	profile := models.VPNProfile{VPNClientID: client.ID, Profile: jobsvc.VPNProfileVLESS, EndpointGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.VPNProfileStatusActive, FinalLink: "vless://existing-link"}
	if err := db.Create(&profile).Error; err != nil {
		t.Fatalf("create active profile: %v", err)
	}
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	result, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS})
	if err != nil {
		t.Fatalf("RequestCreateVPN active profile: %v", err)
	}
	if result.JobsCount != 0 || result.BatchID != 0 || result.JobID != 0 || result.FinalLink != "vless://existing-link" || result.Status != models.VPNProfileStatusActive {
		t.Fatalf("result = %#v, want existing active link without jobs", result)
	}
	if len(publisher.messages) != 0 {
		t.Fatalf("published messages = %d, want 0", len(publisher.messages))
	}
	var jobs int64
	if err := db.Model(&models.Job{}).Count(&jobs).Error; err != nil {
		t.Fatalf("count jobs: %v", err)
	}
	if jobs != 0 {
		t.Fatalf("jobs = %d, want 0", jobs)
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

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileTrojan}); !errors.Is(err, ErrNoMatchingServers) {
		t.Fatalf("RequestCreateVPN trojan err = %v, want ErrNoMatchingServers", err)
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

func TestRequestCreateVPNPendingProfileWithoutJobsRequeuesJobs(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	first, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS})
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	if first.BatchID == 0 || first.JobID == 0 || first.JobsCount != 2 {
		t.Fatalf("first result = %#v, want queued jobs", first)
	}

	var client models.VPNClient
	if err := db.Where("user_id = ?", telegram.UserID).Take(&client).Error; err != nil {
		t.Fatalf("load client: %v", err)
	}
	var profile models.VPNProfile
	if err := db.Where("vpn_client_id = ? AND profile = ?", client.ID, jobsvc.VPNProfileVLESS).Take(&profile).Error; err != nil {
		t.Fatalf("load profile: %v", err)
	}
	if err := db.Where("profile_id = ?", profile.ID).Delete(&models.Job{}).Error; err != nil {
		t.Fatalf("delete jobs: %v", err)
	}
	publisher.messages = nil

	second, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS})
	if err != nil {
		t.Fatalf("requeue pending profile without jobs: %v", err)
	}
	if second.BatchID == 0 || second.JobID == 0 || second.JobsCount != 2 {
		t.Fatalf("second result = %#v, want requeued jobs", second)
	}
	if len(publisher.messages) != 2 {
		t.Fatalf("published messages = %d, want 2", len(publisher.messages))
	}
}

func TestRequestCreateVPNDoesNotUseLegacyServersTable(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	if db.Migrator().HasTable(&models.Server{}) {
		t.Fatal("legacy servers table must not be part of VPN create test schema")
	}
	if err := db.Where("endpoint_group = ?", jobsvc.EndpointGroupDirect).Delete(&models.ServerRegistry{}).Error; err != nil {
		t.Fatalf("delete server registry: %v", err)
	}

	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)
	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); !errors.Is(err, ErrNoMatchingServers) {
		t.Fatalf("RequestCreateVPN err = %v, want ErrNoMatchingServers", err)
	}
	if len(publisher.messages) != 0 {
		t.Fatalf("published messages = %d, want 0", len(publisher.messages))
	}
}

func TestRequestCreateVPNExcludesDisabledAndArchivedServerRegistry(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	archivedAt := time.Now().UTC()
	if err := db.Model(&models.ServerRegistry{}).Where("server_id = ?", "direct-1").Updates(map[string]any{"archived_at": &archivedAt, "archived_reason": "manual"}).Error; err != nil {
		t.Fatalf("archive direct-1: %v", err)
	}
	if err := db.Model(&models.ServerRegistry{}).Where("server_id = ?", "direct-2").Update("enabled", false).Error; err != nil {
		t.Fatalf("disable direct-2: %v", err)
	}

	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)
	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); !errors.Is(err, ErrNoMatchingServers) {
		t.Fatalf("RequestCreateVPN err = %v, want ErrNoMatchingServers", err)
	}
	if len(publisher.messages) != 0 {
		t.Fatalf("published messages = %d, want 0", len(publisher.messages))
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

func seedVPNLinkTestUser(t *testing.T, db *gorm.DB) models.Telegram {
	t.Helper()
	user := models.User{Username: "link-user", Status: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	telegram := models.Telegram{TgID: 99887766, Username: "link", Firstname: "Link", Lastname: "User", UserID: user.ID}
	if err := db.Create(&telegram).Error; err != nil {
		t.Fatalf("create telegram: %v", err)
	}
	return telegram
}

func seedCanonicalLinkProfiles(t *testing.T, db *gorm.DB, telegram models.Telegram, vlessLink string, trojanLink string) models.VPNClient {
	t.Helper()
	client := models.VPNClient{UserID: telegram.UserID, TelegramID: telegram.TgID, ClientCode: "cvn_test", Email: "cvn_test", VlessUUID: "vless-secret", TrojanPassword: "trojan-secret"}
	if err := db.Create(&client).Error; err != nil {
		t.Fatalf("create canonical client: %v", err)
	}
	profiles := []models.VPNProfile{
		{VPNClientID: client.ID, Profile: jobsvc.VPNProfileVLESS, EndpointGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.VPNProfileStatusActive, FinalLink: vlessLink},
		{VPNClientID: client.ID, Profile: jobsvc.VPNProfileTrojan, EndpointGroup: jobsvc.EndpointGroupRU, Protocol: jobsvc.VPNProfileTrojan, Status: models.VPNProfileStatusActive, FinalLink: trojanLink},
	}
	for _, profile := range profiles {
		if err := db.Create(&profile).Error; err != nil {
			t.Fatalf("create canonical profile %s: %v", profile.Profile, err)
		}
	}
	return client
}

func TestGetLinkOverviewUsesCanonicalProfilesWhenUsable(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNLinkTestUser(t, db)
	seedCanonicalLinkProfiles(t, db, telegram, "canonical-vless", "canonical-trojan")
	if err := db.Create(&models.Vpn{UserID: telegram.UserID, VlessLink: "legacy-vless", TrojanLink: "legacy-trojan"}).Error; err != nil {
		t.Fatalf("create legacy vpn: %v", err)
	}
	svc := newTestVPNService(db, &captureJobPublisher{})

	overview, err := svc.GetLinkOverview(telegram.TgID)
	if err != nil {
		t.Fatalf("GetLinkOverview: %v", err)
	}
	if overview.Reason != "both_links_available" {
		t.Fatalf("reason = %q", overview.Reason)
	}
	if got := overview.Profiles[jobsvc.VPNProfileVLESS]; got.FinalLink != "canonical-vless" || got.Source != "canonical" {
		t.Fatalf("vless profile = %#v", got)
	}
	if got := overview.Profiles[jobsvc.VPNProfileTrojan]; got.FinalLink != "canonical-trojan" || got.Source != "canonical" {
		t.Fatalf("trojan profile = %#v", got)
	}
}

func TestGetLinkOverviewFallsBackToLegacyWhenCanonicalMissing(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNLinkTestUser(t, db)
	if err := db.Create(&models.Vpn{UserID: telegram.UserID, VlessLink: "legacy-vless", TrojanLink: "legacy-trojan"}).Error; err != nil {
		t.Fatalf("create legacy vpn: %v", err)
	}
	svc := newTestVPNService(db, &captureJobPublisher{})

	overview, err := svc.GetLinkOverview(telegram.TgID)
	if err != nil {
		t.Fatalf("GetLinkOverview: %v", err)
	}
	if overview.Reason != "both_links_available" {
		t.Fatalf("reason = %q", overview.Reason)
	}
	if got := overview.Profiles[jobsvc.VPNProfileVLESS]; got.FinalLink != "legacy-vless" || got.Source != "legacy" {
		t.Fatalf("vless profile = %#v", got)
	}
	if got := overview.Profiles[jobsvc.VPNProfileTrojan]; got.FinalLink != "legacy-trojan" || got.Source != "legacy" {
		t.Fatalf("trojan profile = %#v", got)
	}

	vless, err := svc.GetProtocolLink(telegram.TgID, jobsvc.VPNProfileVLESS)
	if err != nil {
		t.Fatalf("GetProtocolLink vless: %v", err)
	}
	if vless.Link != "legacy-vless" || vless.Reason != "protocol_link_found" {
		t.Fatalf("vless result = %#v", vless)
	}
	trojan, err := svc.GetProtocolLink(telegram.TgID, jobsvc.VPNProfileTrojan)
	if err != nil {
		t.Fatalf("GetProtocolLink trojan: %v", err)
	}
	if trojan.Link != "legacy-trojan" || trojan.Reason != "protocol_link_found" {
		t.Fatalf("trojan result = %#v", trojan)
	}
}

func TestGetLinkOverviewFallsBackToLegacyWhenCanonicalFinalLinkEmpty(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNLinkTestUser(t, db)
	seedCanonicalLinkProfiles(t, db, telegram, "", "")
	if err := db.Create(&models.Vpn{UserID: telegram.UserID, VlessLink: "legacy-vless", TrojanLink: "legacy-trojan"}).Error; err != nil {
		t.Fatalf("create legacy vpn: %v", err)
	}
	svc := newTestVPNService(db, &captureJobPublisher{})

	overview, err := svc.GetLinkOverview(telegram.TgID)
	if err != nil {
		t.Fatalf("GetLinkOverview: %v", err)
	}
	if overview.Reason != "both_links_available" {
		t.Fatalf("reason = %q", overview.Reason)
	}
	if got := overview.Profiles[jobsvc.VPNProfileVLESS]; got.FinalLink != "legacy-vless" || got.Source != "legacy" {
		t.Fatalf("vless profile = %#v", got)
	}
	if got := overview.Profiles[jobsvc.VPNProfileTrojan]; got.FinalLink != "legacy-trojan" || got.Source != "legacy" {
		t.Fatalf("trojan profile = %#v", got)
	}
}

func TestGetLinkOverviewWithoutCanonicalAndLegacyReturnsNotConfigured(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNLinkTestUser(t, db)
	svc := newTestVPNService(db, &captureJobPublisher{})

	overview, err := svc.GetLinkOverview(telegram.TgID)
	if err != nil {
		t.Fatalf("GetLinkOverview: %v", err)
	}
	if overview.Reason != "vpn_not_configured" {
		t.Fatalf("reason = %q", overview.Reason)
	}
	if linkProfileAvailableForTest(overview.Profiles[jobsvc.VPNProfileVLESS]) || linkProfileAvailableForTest(overview.Profiles[jobsvc.VPNProfileTrojan]) {
		t.Fatalf("profiles should not be usable: %#v", overview.Profiles)
	}
}

func TestGetLinkOverviewLegacyOnlyVLESS(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNLinkTestUser(t, db)
	if err := db.Create(&models.Vpn{UserID: telegram.UserID, VlessLink: "legacy-vless"}).Error; err != nil {
		t.Fatalf("create legacy vpn: %v", err)
	}
	svc := newTestVPNService(db, &captureJobPublisher{})

	overview, err := svc.GetLinkOverview(telegram.TgID)
	if err != nil {
		t.Fatalf("GetLinkOverview: %v", err)
	}
	if overview.Reason != "single_link_available" {
		t.Fatalf("reason = %q", overview.Reason)
	}
	if !linkProfileAvailableForTest(overview.Profiles[jobsvc.VPNProfileVLESS]) || linkProfileAvailableForTest(overview.Profiles[jobsvc.VPNProfileTrojan]) {
		t.Fatalf("unexpected profiles: %#v", overview.Profiles)
	}
}

func TestGetLinkOverviewLegacyOnlyTrojan(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNLinkTestUser(t, db)
	if err := db.Create(&models.Vpn{UserID: telegram.UserID, TrojanLink: "legacy-trojan"}).Error; err != nil {
		t.Fatalf("create legacy vpn: %v", err)
	}
	svc := newTestVPNService(db, &captureJobPublisher{})

	overview, err := svc.GetLinkOverview(telegram.TgID)
	if err != nil {
		t.Fatalf("GetLinkOverview: %v", err)
	}
	if overview.Reason != "single_link_available" {
		t.Fatalf("reason = %q", overview.Reason)
	}
	if linkProfileAvailableForTest(overview.Profiles[jobsvc.VPNProfileVLESS]) || !linkProfileAvailableForTest(overview.Profiles[jobsvc.VPNProfileTrojan]) {
		t.Fatalf("unexpected profiles: %#v", overview.Profiles)
	}
}

func linkProfileAvailableForTest(profile LinkProfileView) bool {
	return profile.Usable && strings.TrimSpace(profile.FinalLink) != ""
}

func TestGetUserVPNDetailsWithoutClientShowsMissingProfiles(t *testing.T) {
	db := newVPNServiceTestDB(t)
	user := models.User{Username: "no-vpn", Status: true}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	svc := newTestVPNService(db, &captureJobPublisher{})

	details, err := svc.GetUserVPNDetails(user.ID)
	if err != nil {
		t.Fatalf("GetUserVPNDetails: %v", err)
	}
	if details.Client != nil {
		t.Fatalf("client = %#v, want nil", details.Client)
	}
	if len(details.Profiles) != 2 {
		t.Fatalf("profiles = %d, want 2", len(details.Profiles))
	}
	for _, profile := range details.Profiles {
		if profile.Exists {
			t.Fatalf("profile %s exists, want missing", profile.Profile)
		}
	}
}

func TestGetUserVPNDetailsReturnsProfilesAndNodesWithoutCredentials(t *testing.T) {
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	publisher := &captureJobPublisher{}
	svc := newTestVPNService(db, publisher)

	if _, err := svc.RequestCreateVPN(RequestCreateVPNInput{TgID: telegram.TgID, Protocol: jobsvc.VPNProfileVLESS}); err != nil {
		t.Fatalf("create vless: %v", err)
	}
	profile := loadTestProfile(t, db, telegram.UserID, jobsvc.VPNProfileVLESS)
	if _, err := svc.ApplyJobResult(context.Background(), broker.JobResultEvent{ProfileID: profile.ID, NodeID: "direct-1", Profile: jobsvc.VPNProfileVLESS, TargetGroup: jobsvc.EndpointGroupDirect, Protocol: jobsvc.VPNProfileVLESS, Status: models.VPNProfileNodeStatusSuccess, ClientCode: profile.VPNClient.ClientCode}); err != nil {
		t.Fatalf("apply result: %v", err)
	}

	details, err := svc.GetUserVPNDetails(telegram.UserID)
	if err != nil {
		t.Fatalf("GetUserVPNDetails: %v", err)
	}
	if details.Client == nil || details.Client.ClientCode == "" || details.Client.Email == "" {
		t.Fatalf("client view is incomplete: %#v", details.Client)
	}
	encoded, err := json.Marshal(details)
	if err != nil {
		t.Fatalf("marshal details: %v", err)
	}
	payload := string(encoded)
	if strings.Contains(payload, "vless_uuid") || strings.Contains(payload, "trojan_password") || strings.Contains(payload, profile.VPNClient.TrojanPassword) {
		t.Fatalf("details leaked credentials: %s", payload)
	}
	var foundVLESS bool
	for _, item := range details.Profiles {
		if item.Profile == jobsvc.VPNProfileVLESS {
			foundVLESS = true
			if !item.Exists || item.Status != models.VPNProfileStatusPartial || item.FinalLink == "" || len(item.Nodes) != 2 {
				t.Fatalf("unexpected vless profile details: %#v", item)
			}
		}
	}
	if !foundVLESS {
		t.Fatalf("vless profile missing from details: %#v", details.Profiles)
	}
}
