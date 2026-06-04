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
