package entities_test

import (
	"testing"

	"github.com/smirnoffmg/deeper/internal/entities"
)

func TestNewTrace(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  entities.TraceType
	}{
		{
			name:  "Email",
			value: "test@test.com",
			want:  entities.Email,
		},
		{
			name:  "Phone",
			value: "123-456-7890",
			want:  entities.Phone,
		},
		{
			name:  "Address",
			value: "123 Main St",
			want:  entities.Address,
		},
		{
			name:  "IpAddr",
			value: "192.168.0.1",
			want:  entities.IpAddr,
		},
		{
			name:  "Domain",
			value: "example.com",
			want:  entities.Domain,
		},
		{
			name:  "Url",
			value: "http://example.com",
			want:  entities.Url,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := entities.NewTrace(tt.value)
			if got.Type != tt.want {
				t.Errorf("NewTrace() = %v, want %v", got.Type, tt.want)
			}
		})
	}
}
