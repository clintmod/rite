package version

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	t.Parallel()

	const embedded = "v0.1.0"

	tests := []struct {
		name        string
		embedded    string
		info        *debug.BuildInfo
		ok          bool
		wantVersion string
		wantCommit  string
		wantDirty   bool
	}{
		{
			name:        "go install @tag uses Main.Version",
			embedded:    embedded,
			ok:          true,
			info:        &debug.BuildInfo{Main: debug.Module{Version: "v1.0.0"}},
			wantVersion: "v1.0.0",
		},
		{
			name:        "go install @latest uses resolved tag",
			embedded:    embedded,
			ok:          true,
			info:        &debug.BuildInfo{Main: debug.Module{Version: "v1.2.3"}},
			wantVersion: "v1.2.3",
		},
		{
			name:     "local go build falls back and decorates with commit+dirty",
			embedded: embedded,
			ok:       true,
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdef1234567"},
					{Key: "vcs.modified", Value: "true"},
				},
			},
			wantVersion: embedded,
			wantCommit:  "abcdef1",
			wantDirty:   true,
		},
		{
			name:     "local go build clean checkout",
			embedded: embedded,
			ok:       true,
			info: &debug.BuildInfo{
				Main: debug.Module{Version: "(devel)"},
				Settings: []debug.BuildSetting{
					{Key: "vcs.revision", Value: "abcdef1234567"},
					{Key: "vcs.modified", Value: "false"},
				},
			},
			wantVersion: embedded,
			wantCommit:  "abcdef1",
		},
		{
			name:        "no build info available",
			embedded:    embedded,
			ok:          false,
			wantVersion: embedded,
		},
		{
			name:        "ldflags path — Main.Version empty, treated like fallback",
			embedded:    "v9.9.9",
			ok:          true,
			info:        &debug.BuildInfo{Main: debug.Module{Version: ""}},
			wantVersion: "v9.9.9",
		},
		{
			name:        "embedded value is trimmed",
			embedded:    "  v0.1.0\n",
			ok:          false,
			wantVersion: "v0.1.0",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			gotVersion, gotCommit, gotDirty := resolveVersion(tc.embedded, tc.info, tc.ok)
			if gotVersion != tc.wantVersion {
				t.Errorf("version: got %q, want %q", gotVersion, tc.wantVersion)
			}
			if gotCommit != tc.wantCommit {
				t.Errorf("commit: got %q, want %q", gotCommit, tc.wantCommit)
			}
			if gotDirty != tc.wantDirty {
				t.Errorf("dirty: got %v, want %v", gotDirty, tc.wantDirty)
			}
		})
	}
}
