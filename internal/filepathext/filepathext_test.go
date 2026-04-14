package filepathext

import "testing"

func TestIsAbsSpecialDirs(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		path string
		want bool
	}{
		{"ritefile_dir", "{{.RITEFILE_DIR}}/foo", true},
		{"taskfile_dir_compat", "{{.TASKFILE_DIR}}/foo", true},
		{"root_dir", "{{.ROOT_DIR}}/foo", true},
		{"user_working_dir", "{{.USER_WORKING_DIR}}/foo", true},
		{"plain_relative", "foo/bar", false},
		{"unknown_var", "{{.SOMETHING_ELSE}}/foo", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := IsAbs(tc.path); got != tc.want {
				t.Errorf("IsAbs(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}
