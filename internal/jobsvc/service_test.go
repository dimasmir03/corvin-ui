package jobsvc

import (
	"testing"
	"vpnpanel/internal/models"
)

func TestCalculateBatchStatus(t *testing.T) {
	tests := []struct {
		name string
		jobs []models.Job
		want string
	}{
		{name: "empty", jobs: nil, want: BatchStatusPending},
		{name: "all success", jobs: []models.Job{{Status: JobStatusSuccess}, {Status: JobStatusSuccess}}, want: BatchStatusSuccess},
		{name: "all failed", jobs: []models.Job{{Status: JobStatusFailed}, {Status: JobStatusFailed}}, want: BatchStatusFailed},
		{name: "mixed", jobs: []models.Job{{Status: JobStatusSuccess}, {Status: JobStatusFailed}}, want: BatchStatusPartialSuccess},
		{name: "pending wins", jobs: []models.Job{{Status: JobStatusSuccess}, {Status: JobStatusPending}}, want: BatchStatusProcessing},
		{name: "processing wins", jobs: []models.Job{{Status: JobStatusFailed}, {Status: JobStatusProcessing}}, want: BatchStatusProcessing},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CalculateBatchStatus(tt.jobs); got != tt.want {
				t.Fatalf("CalculateBatchStatus() = %s, want %s", got, tt.want)
			}
		})
	}
}

func TestEndpointGroupForProfile(t *testing.T) {
	tests := []struct {
		profile string
		want    string
	}{
		{profile: VPNProfileVLESS, want: EndpointGroupDirect},
		{profile: VPNProfileTrojan, want: EndpointGroupRU},
		{profile: "", want: EndpointGroupDirect},
	}

	for _, tt := range tests {
		t.Run(tt.profile, func(t *testing.T) {
			if got := endpointGroupForProfile(tt.profile); got != tt.want {
				t.Fatalf("endpointGroupForProfile(%q) = %q, want %q", tt.profile, got, tt.want)
			}
		})
	}
}

func TestServerSupportsEndpointProfile(t *testing.T) {
	directVless := models.Server{Type: VPNProfileVLESS, NodeRole: EndpointGroupDirect}
	if !serverSupportsEndpointProfile(directVless, VPNProfileVLESS, EndpointGroupDirect) {
		t.Fatal("direct vless server should support vless/direct profile")
	}
	if serverSupportsEndpointProfile(directVless, VPNProfileTrojan, EndpointGroupRU) {
		t.Fatal("direct vless server should not support trojan/ru profile")
	}

	ruTrojan := models.Server{Type: VPNProfileTrojan, NodeRole: EndpointGroupRU}
	if !serverSupportsEndpointProfile(ruTrojan, VPNProfileTrojan, EndpointGroupRU) {
		t.Fatal("ru trojan server should support trojan/ru profile")
	}
}

func TestCreateClientJobTaskCarriesCanonicalCredentials(t *testing.T) {
	input := CreateUserConfigInput{
		UserID:         55,
		TelegramID:     123456789,
		ClientCode:     "cvn_8f3a91c2",
		Email:          "cvn_8f3a91c2",
		VlessUUID:      "same-vless-uuid",
		VlessFlow:      "",
		TrojanPassword: "same-trojan-password",
		Enable:         true,
	}

	task := createClientJobTask(7, "direct-1", VPNProfileVLESS, EndpointGroupDirect, input)
	if task.EventType != ActionCreateClient || task.CommandType != ActionCreateClient {
		t.Fatalf("unexpected event/action fields: %#v", task)
	}
	if task.ServerID != "direct-1" || task.TargetServerID != "direct-1" || task.NodeID != "direct-1" {
		t.Fatalf("unexpected server identity fields: %#v", task)
	}
	if task.Profile != VPNProfileVLESS || task.TargetGroup != EndpointGroupDirect {
		t.Fatalf("unexpected profile/group: %s/%s", task.Profile, task.TargetGroup)
	}
	if task.Credentials.VLESS.ID != input.VlessUUID {
		t.Fatalf("vless credential = %q, want %q", task.Credentials.VLESS.ID, input.VlessUUID)
	}
	if task.Credentials.Trojan.Password != input.TrojanPassword {
		t.Fatalf("trojan credential = %q, want %q", task.Credentials.Trojan.Password, input.TrojanPassword)
	}
	if task.ClientCode != input.ClientCode || task.Email != input.Email {
		t.Fatalf("unexpected client identity: %s/%s", task.ClientCode, task.Email)
	}
}
