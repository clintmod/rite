package main

import "testing"

func TestMigrateSubcommand(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		args     []string
		wantPath string
		wantOk   bool
	}{
		{"empty args", nil, "", false},
		{"task named migrate-but-not-equal", []string{"migration"}, "", false},
		{"bare subcommand → autodetect", []string{"migrate"}, "", true},
		{"subcommand with path", []string{"migrate", "Taskfile.yml"}, "Taskfile.yml", true},
		{"extra args ignored", []string{"migrate", "a.yml", "ignored"}, "a.yml", true},
		{"migrate not first → not subcommand", []string{"other", "migrate"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			gotPath, gotOk := migrateSubcommand(tt.args)
			if gotOk != tt.wantOk {
				t.Errorf("ok = %v, want %v", gotOk, tt.wantOk)
			}
			if gotPath != tt.wantPath {
				t.Errorf("path = %q, want %q", gotPath, tt.wantPath)
			}
		})
	}
}
