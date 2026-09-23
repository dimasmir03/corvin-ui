package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/config"
	"vpnpanel/internal/jobsvc"
	projectlogger "vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/google/uuid"
)

func newMobileTestService(t *testing.T, publisher *captureJobPublisher) (*MobileService, *VPNService, models.Telegram) {
	t.Helper()
	db := newVPNServiceTestDB(t)
	telegram := seedVPNServiceCreateData(t, db)
	if err := db.Model(&models.ServerRegistry{}).Where("server_id = ?", "direct-1").Updates(map[string]any{"public_host": "direct.example.com", "public_port": 443, "security": "reality", "network": "tcp", "sni": "example.com", "country_code": "NL", "country_name": "Netherlands", "city": "Amsterdam"}).Error; err != nil {
		t.Fatal(err)
	}
	vpn := newTestVPNService(db, publisher)
	cfg := config.MobileConfig{Enabled: true, JWTSecret: strings.Repeat("s", 64), AccessTTL: 900, RefreshTTLHours: 720, LoginTTLMinutes: 10, TelegramBotUsername: "corvin_test_bot", PublicBaseURL: "https://panel.example.com", MinAppVersion: "1.0.0"}
	mobile := NewMobileService(repository.NewMobileRepo(db), repository.NewVpnRepo(db), vpn, cfg)
	return mobile, vpn, telegram
}

func loginMobile(t *testing.T, mobile *MobileService, telegram models.Telegram, installID string) MobileTokens {
	t.Helper()
	verifier := strings.Repeat("v", 64)
	started, err := mobile.StartTelegramLogin(MobileStartInput{InstallID: installID, Platform: "android", AppVersion: "1.0.0", DeviceName: "Test Phone", CodeChallenge: tokenHash(verifier), CodeChallengeMethod: "S256"})
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	raw := started.TelegramDeeplink[strings.LastIndex(started.TelegramDeeplink, "login_")+len("login_"):]
	if err := mobile.ApproveTelegramLogin(raw, telegram.TgID); err != nil {
		t.Fatalf("approve: %v", err)
	}
	tokens, err := mobile.ExchangeTelegramLogin(started.LoginID, verifier, installID)
	if err != nil {
		t.Fatalf("exchange: %v", err)
	}
	return tokens
}

func TestMobileTelegramLoginRefreshRotationAndReuse(t *testing.T) {
	mobile, _, telegram := newMobileTestService(t, &captureJobPublisher{})
	installID := uuid.NewString()
	tokens := loginMobile(t, mobile, telegram, installID)
	identity, err := mobile.Authenticate(tokens.AccessToken)
	if err != nil || identity.UserID != telegram.UserID {
		t.Fatalf("authenticate: identity=%+v err=%v", identity, err)
	}
	rotated, err := mobile.Refresh(tokens.RefreshToken, installID)
	if err != nil {
		t.Fatalf("refresh: %v", err)
	}
	if rotated.RefreshToken == tokens.RefreshToken {
		t.Fatal("refresh token was not rotated")
	}
	_, err = mobile.Refresh(tokens.RefreshToken, installID)
	var mobileError *MobileError
	if !errors.As(err, &mobileError) || mobileError.Code != "TOKEN_REUSED" {
		t.Fatalf("reuse error=%v", err)
	}
	if _, err := mobile.Authenticate(rotated.AccessToken); err == nil {
		t.Fatal("reused token must revoke the session")
	}
}

func TestMobileLoginExchangeIsOneTime(t *testing.T) {
	mobile, _, telegram := newMobileTestService(t, &captureJobPublisher{})
	verifier := strings.Repeat("x", 64)
	installID := uuid.NewString()
	started, err := mobile.StartTelegramLogin(MobileStartInput{InstallID: installID, Platform: "android", AppVersion: "1", DeviceName: "Phone", CodeChallenge: tokenHash(verifier), CodeChallengeMethod: "S256"})
	if err != nil {
		t.Fatal(err)
	}
	raw := started.TelegramDeeplink[strings.LastIndex(started.TelegramDeeplink, "login_")+6:]
	if err := mobile.ApproveTelegramLogin(raw, telegram.TgID); err != nil {
		t.Fatal(err)
	}
	if _, err := mobile.ExchangeTelegramLogin(started.LoginID, verifier, installID); err != nil {
		t.Fatal(err)
	}
	if _, err := mobile.ExchangeTelegramLogin(started.LoginID, verifier, installID); err == nil {
		t.Fatal("replayed login exchange succeeded")
	}
}

func TestMobileLoginExpires(t *testing.T) {
	mobile, _, telegram := newMobileTestService(t, &captureJobPublisher{})
	base := time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)
	mobile.now = func() time.Time { return base }
	verifier := strings.Repeat("e", 64)
	installID := uuid.NewString()
	started, err := mobile.StartTelegramLogin(MobileStartInput{InstallID: installID, Platform: "android", AppVersion: "1", DeviceName: "Phone", CodeChallenge: tokenHash(verifier), CodeChallengeMethod: "S256"})
	if err != nil {
		t.Fatal(err)
	}
	raw := started.TelegramDeeplink[strings.LastIndex(started.TelegramDeeplink, "login_")+6:]
	if err := mobile.ApproveTelegramLogin(raw, telegram.TgID); err != nil {
		t.Fatal(err)
	}
	mobile.now = func() time.Time { return base.Add(11 * time.Minute) }
	status, err := mobile.TelegramLoginStatus(started.LoginID)
	if err != nil || status != "expired" {
		t.Fatalf("status=%q err=%v", status, err)
	}
	if _, err := mobile.ExchangeTelegramLogin(started.LoginID, verifier, installID); err == nil {
		t.Fatal("expired login exchanged")
	}
}

func TestMobileDeviceLimitAndRevoke(t *testing.T) {
	mobile, _, telegram := newMobileTestService(t, &captureJobPublisher{})
	first := loginMobile(t, mobile, telegram, uuid.NewString())
	_, err := func() (MobileTokens, error) {
		verifier := strings.Repeat("z", 64)
		install := uuid.NewString()
		started, e := mobile.StartTelegramLogin(MobileStartInput{InstallID: install, Platform: "android", AppVersion: "1", DeviceName: "Second", CodeChallenge: tokenHash(verifier), CodeChallengeMethod: "S256"})
		if e != nil {
			return MobileTokens{}, e
		}
		raw := started.TelegramDeeplink[strings.LastIndex(started.TelegramDeeplink, "login_")+6:]
		if e = mobile.ApproveTelegramLogin(raw, telegram.TgID); e != nil {
			return MobileTokens{}, e
		}
		return mobile.ExchangeTelegramLogin(started.LoginID, verifier, install)
	}()
	var mobileError *MobileError
	if !errors.As(err, &mobileError) || mobileError.Code != "DEVICE_LIMIT_REACHED" {
		t.Fatalf("device limit err=%v", err)
	}
	identity, err := mobile.Authenticate(first.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := mobile.RevokeDevice(identity, identity.DeviceID); err != nil {
		t.Fatal(err)
	}
	if _, err := mobile.Authenticate(first.AccessToken); err == nil {
		t.Fatal("revoked device authenticated")
	}
}

func TestMobileConnectIdempotentAndBecomesReady(t *testing.T) {
	publisher := &captureJobPublisher{}
	mobile, vpn, telegram := newMobileTestService(t, publisher)
	tokens := loginMobile(t, mobile, telegram, uuid.NewString())
	identity, err := mobile.Authenticate(tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	input := MobileConnectInput{DeviceID: identity.DeviceID, ServerID: "direct-1", Protocol: "vless", Mode: "manual", ConfigRevision: "0"}
	key := uuid.NewString()
	first, status, err := mobile.Connect(identity, key, input)
	if err != nil || status != 202 || first.OperationID == "" {
		t.Fatalf("connect status=%d view=%+v err=%v", status, first, err)
	}
	again, status, err := mobile.Connect(identity, key, input)
	if err != nil || status != 202 || again.OperationID != first.OperationID || len(publisher.messages) != 1 {
		t.Fatalf("idempotency status=%d view=%+v messages=%d err=%v", status, again, len(publisher.messages), err)
	}
	message := publisher.messages[0]
	configLink := "vless://ignored-secret"
	if _, err := vpn.ApplyCommandResult(context.Background(), broker.JobResultEvent{EventType: "job_result", JobID: message.JobID, ProfileID: message.ProfileID, ServerID: message.ServerID, CommandType: jobsvc.ActionCreateClient, Status: "success", ConfigLink: &configLink}); err != nil {
		t.Fatalf("apply result: %v", err)
	}
	ready, err := mobile.Operation(identity, first.OperationID)
	if err != nil || ready.Status != "success" || ready.Result == nil || !strings.HasPrefix(ready.Result.ConfigURL, "vless://") {
		t.Fatalf("operation=%+v err=%v", ready, err)
	}
}

func TestMobileConnectRejectsInactiveSubscription(t *testing.T) {
	mobile, _, telegram := newMobileTestService(t, &captureJobPublisher{})
	tokens := loginMobile(t, mobile, telegram, uuid.NewString())
	identity, _ := mobile.Authenticate(tokens.AccessToken)
	if _, err := mobile.vpn.UpdateUserSubscription(identity.UserID, UpdateSubscriptionInput{Status: models.SubscriptionStatusBlocked, TariffName: "blocked", DeviceLimit: 1}); err != nil {
		t.Fatal(err)
	}
	_, _, err := mobile.Connect(identity, uuid.NewString(), MobileConnectInput{DeviceID: identity.DeviceID, ServerID: "direct-1", Protocol: "vless", Mode: "manual"})
	var mobileError *MobileError
	if !errors.As(err, &mobileError) || mobileError.Code != "SUBSCRIPTION_INACTIVE" {
		t.Fatalf("error=%v", err)
	}
}

func TestMobileServerCatalogDoesNotExposeSecrets(t *testing.T) {
	mobile, _, telegram := newMobileTestService(t, &captureJobPublisher{})
	tokens := loginMobile(t, mobile, telegram, uuid.NewString())
	identity, _ := mobile.Authenticate(tokens.AccessToken)
	_, servers, err := mobile.Servers(identity.UserID)
	if err != nil {
		t.Fatal(err)
	}
	raw := fmtJSON(t, servers)
	for _, secret := range []string{"public_host", "direct.example.com", "uuid", "password", "endpoint_group"} {
		if strings.Contains(raw, secret) {
			t.Fatalf("catalog leaks %q: %s", secret, raw)
		}
	}
}

func TestMobileConnectDoesNotLogTokensOrVPNCredentials(t *testing.T) {
	var logs bytes.Buffer
	previous := projectlogger.Default()
	projectlogger.Configure(&logs, "info", "text")
	defer projectlogger.SetDefault(previous)
	publisher := &captureJobPublisher{}
	mobile, _, telegram := newMobileTestService(t, publisher)
	tokens := loginMobile(t, mobile, telegram, uuid.NewString())
	identity, err := mobile.Authenticate(tokens.AccessToken)
	if err != nil {
		t.Fatal(err)
	}
	_, _, _ = mobile.Connect(identity, uuid.NewString(), MobileConnectInput{DeviceID: identity.DeviceID, ServerID: "direct-1", Protocol: "vless", Mode: "manual", ConfigRevision: "0"})
	var client models.VPNClient
	if err := mobile.vpnRepo.DB.Where("device_id = ?", identity.DeviceID).Take(&client).Error; err != nil {
		t.Fatal(err)
	}
	output := logs.String()
	for _, secret := range []string{tokens.AccessToken, tokens.RefreshToken, client.VlessUUID, client.TrojanPassword} {
		if secret != "" && strings.Contains(output, secret) {
			t.Fatalf("secret leaked to logs: %s", output)
		}
	}
}

func fmtJSON(t *testing.T, value any) string {
	t.Helper()
	raw, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(raw)
}
