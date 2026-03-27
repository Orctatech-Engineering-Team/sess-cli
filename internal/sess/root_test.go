package sess

import "testing"

func TestSetVersionInfo(t *testing.T) {
	originalVersion := rootCmd.Version
	originalTemplate := rootCmd.VersionTemplate()
	t.Cleanup(func() {
		rootCmd.Version = originalVersion
		rootCmd.SetVersionTemplate(originalTemplate)
	})

	tests := []struct {
		name              string
		version           string
		commit            string
		wantVersionTmpl   string
		wantStoredVersion string
	}{
		{
			name:              "tagged release remains unchanged",
			version:           "v0.2.0",
			commit:            "abcdef123456",
			wantVersionTmpl:   "SESS v0.2.0\n",
			wantStoredVersion: "v0.2.0",
		},
		{
			name:              "pseudo version uses short commit",
			version:           "v0.0.0-20261208220651-7dba82a8368d",
			commit:            "7dba82a8368d1234",
			wantVersionTmpl:   "SESS dev-7dba82a\n",
			wantStoredVersion: "v0.0.0-20261208220651-7dba82a8368d",
		},
		{
			name:              "dirty pseudo version includes modified suffix",
			version:           "v0.0.0-20261208220651-7dba82a8368d+dirty",
			commit:            "7dba82a8368d1234",
			wantVersionTmpl:   "SESS dev-7dba82a (modified)\n",
			wantStoredVersion: "v0.0.0-20261208220651-7dba82a8368d+dirty",
		},
		{
			name:              "pseudo version with unknown commit falls back to dev",
			version:           "v0.0.0-20261208220651-7dba82a8368d",
			commit:            "unknown",
			wantVersionTmpl:   "SESS dev\n",
			wantStoredVersion: "v0.0.0-20261208220651-7dba82a8368d",
		},
		{
			name:              "devel falls back to dev",
			version:           "(devel)",
			commit:            "abcdef123456",
			wantVersionTmpl:   "SESS dev\n",
			wantStoredVersion: "(devel)",
		},
		{
			name:              "unknown falls back to dev",
			version:           "unknown",
			commit:            "abcdef123456",
			wantVersionTmpl:   "SESS dev\n",
			wantStoredVersion: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetVersionInfo(tt.version, tt.commit, "")

			if rootCmd.Version != tt.wantStoredVersion {
				t.Fatalf("rootCmd.Version = %q, want %q", rootCmd.Version, tt.wantStoredVersion)
			}
			if got := rootCmd.VersionTemplate(); got != tt.wantVersionTmpl {
				t.Fatalf("rootCmd.VersionTemplate() = %q, want %q", got, tt.wantVersionTmpl)
			}
		})
	}
}

func TestIsPseudoVersion(t *testing.T) {
	tests := []struct {
		version string
		want    bool
	}{
		{"v0.0.0-20261208220651-7dba82a8368d", true},
		{"v1.2.4-0.20261208220651-7dba82a8368d", true},
		{"v1.2.3-beta.1.0.20261208220651-7dba82a8368d", true},
		{"v0.0.0-20261208220651-7dba82a8368d+dirty", true},
		{"v0.2.0", false},
		{"(devel)", false},
		{"unknown", false},
		{"v1.2.3-beta.1", false},
	}

	for _, tt := range tests {
		if got := isPseudoVersion(tt.version); got != tt.want {
			t.Fatalf("isPseudoVersion(%q) = %v, want %v", tt.version, got, tt.want)
		}
	}
}
