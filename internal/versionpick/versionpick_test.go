package versionpick_test

import (
	"testing"

	"github.com/hugoh/hrd/internal/versionpick"
	"github.com/stretchr/testify/require"
)

func TestLatestMinors(t *testing.T) {
	tests := []struct {
		name     string
		versions []string
		n        int
		want     []string
		wantErr  bool
	}{
		{
			"sorted list",
			[]string{"0.37.0", "0.42.0", "0.43.0", "0.44.0"},
			3,
			[]string{"0.42", "0.43", "0.44"},
			false,
		},
		{
			"unsorted input",
			[]string{"0.44.0", "0.42.1", "0.43.0", "0.42.0"},
			2,
			[]string{"0.43", "0.44"},
			false,
		},
		{
			"patches collapse to minor",
			[]string{"1.2.0", "1.2.1", "1.2.9", "1.3.0"},
			3,
			[]string{"1.2", "1.3"},
			false,
		},
		{
			"v prefix and prerelease",
			[]string{"v0.42.0", "0.43.0-rc.1", "0.43.0", "0.44.0"},
			3,
			[]string{"0.42", "0.43", "0.44"},
			false,
		},
		{
			"garbage lines ignored",
			[]string{"", "nope", "0.43.0", "  ", "0.44.0"},
			3,
			[]string{"0.43", "0.44"},
			false,
		},
		{"n larger than available", []string{"2.0.0", "2.1.0"}, 5, []string{"2.0", "2.1"}, false},
		{"empty input", nil, 3, nil, false},
		{"non-positive n", []string{"1.0.0"}, 0, nil, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := versionpick.LatestMinors(tt.versions, tt.n)
			if tt.wantErr {
				require.Error(t, err)

				return
			}

			require.NoError(t, err)
			require.Equal(t, tt.want, got)
		})
	}
}
