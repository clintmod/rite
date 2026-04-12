package ast

import (
	"testing"

	"go.yaml.in/yaml/v3"
)

func TestVarExport(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name       string
		yaml       string
		wantValue  any
		wantExport bool // Exported() return — the effective truth
	}{
		{
			name:       "scalar defaults to exported",
			yaml:       "foo",
			wantValue:  "foo",
			wantExport: true,
		},
		{
			name:       "value map defaults to exported",
			yaml:       "{value: bar}",
			wantValue:  "bar",
			wantExport: true,
		},
		{
			name:       "value + export:true",
			yaml:       "{value: bar, export: true}",
			wantValue:  "bar",
			wantExport: true,
		},
		{
			name:       "value + export:false",
			yaml:       "{value: bar, export: false}",
			wantValue:  "bar",
			wantExport: false,
		},
		{
			name:       "sh + export:false",
			yaml:       "{sh: 'echo hi', export: false}",
			wantValue:  nil, // unresolved until compile
			wantExport: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var v Var
			if err := yaml.Unmarshal([]byte(tc.yaml), &v); err != nil {
				t.Fatalf("unmarshal: %v", err)
			}
			if v.Value != tc.wantValue {
				t.Errorf("Value = %v, want %v", v.Value, tc.wantValue)
			}
			if v.Exported() != tc.wantExport {
				t.Errorf("Exported() = %v, want %v", v.Exported(), tc.wantExport)
			}
		})
	}
}
