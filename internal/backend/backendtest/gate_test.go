package backendtest

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestIntegrationRequired(t *testing.T) {
	tests := []struct {
		name string
		env  string
		set  bool
		want bool
	}{
		{name: "unset", set: false, want: false},
		{name: "enabled", env: "1", set: true, want: true},
		{name: "other value", env: "true", set: true, want: false},
		{name: "empty", env: "", set: true, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("REQUIRE_INTEGRATION", "sentinel")

			if tt.set {
				t.Setenv("REQUIRE_INTEGRATION", tt.env)
			} else {
				require.NoError(t, os.Unsetenv("REQUIRE_INTEGRATION"))
			}

			require.Equal(t, tt.want, integrationRequired())
		})
	}
}
