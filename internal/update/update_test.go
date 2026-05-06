package update

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestInfo_Available(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		current   string
		latest    string
		available bool
	}{
		{"older version", "0.10.0", "0.11.0", true},
		{"same version", "0.10.0", "0.10.0", false},
		{"current is beta, latest is stable", "0.11.0-beta.1", "0.11.0", true},
		{"current is stable, latest is beta", "0.10.0", "0.11.0-beta.1", false},
		{"both beta, different", "0.11.0-beta.1", "0.11.0-beta.2", true},
		{"both beta, same", "0.11.0-beta.1", "0.11.0-beta.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := Info{Current: tt.current, Latest: tt.latest}
			require.Equal(t, tt.available, info.Available())
		})
	}
}

func TestInfo_IsDevelopment(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		current string
		isDev   bool
	}{
		{"devel", "devel", true},
		{"unknown", "unknown", true},
		{"dirty", "0.6.7-abc1234-dirty", true},
		{"go install format", "v0.0.0-0.20251231235959-06c807842604", true},
		{"release", "0.6.7", false},
		{"beta release", "0.6.7-beta.1", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			info := Info{Current: tt.current}
			require.Equal(t, tt.isDev, info.IsDevelopment())
		})
	}
}
