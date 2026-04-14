package version

import (
	_ "embed"
	"runtime/debug"
	"strings"
)

var (
	//go:embed version.txt
	version string
	commit  string
	dirty   bool
)

func init() {
	info, ok := debug.ReadBuildInfo()
	version, commit, dirty = resolveVersion(version, info, ok)
}

// resolveVersion picks the best version string available and, when we fall
// back to the embedded value, pulls commit/dirty metadata off the build info.
//
// Precedence:
//  1. `go install …@vX.Y.Z` / `…@latest` — info.Main.Version is the real tag.
//     Use it directly; no commit/dirty suffix (the tag is authoritative).
//  2. Local `go build` from a checkout — Main.Version == "(devel)". Keep the
//     embedded version.txt fallback and decorate with commit/dirty.
//  3. No build info at all — just the embedded value.
//
// The goreleaser path is unaffected: its -ldflags overwrite of `version`
// runs before init, and Main.Version is empty for ldflags-injected binaries.
func resolveVersion(embedded string, info *debug.BuildInfo, ok bool) (string, string, bool) {
	v := strings.TrimSpace(embedded)
	if !ok {
		return v, "", false
	}
	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version, "", false
	}
	return v, getCommit(info), getDirty(info)
}

func getDirty(info *debug.BuildInfo) bool {
	for _, setting := range info.Settings {
		if setting.Key == "vcs.modified" {
			return setting.Value == "true"
		}
	}
	return false
}

func getCommit(info *debug.BuildInfo) string {
	for _, setting := range info.Settings {
		if setting.Key == "vcs.revision" {
			return setting.Value[:7]
		}
	}
	return ""
}

// GetVersion returns the version of Task. By default, this is retrieved from
// the embedded version.txt file which is kept up-to-date by our release script.
// However, it can also be overridden at build time using:
// -ldflags="-X 'github.com/clintmod/rite/internal/version.version=vX.X.X'".
func GetVersion() string {
	return version
}

// GetVersionWithBuildInfo is the same as [GetVersion], but it also includes
// the commit hash and dirty status if available. This will only work when built
// within inside of a Git checkout.
func GetVersionWithBuildInfo() string {
	var buildMetadata []string
	if commit != "" {
		buildMetadata = append(buildMetadata, commit)
	}
	if dirty {
		buildMetadata = append(buildMetadata, "dirty")
	}
	if len(buildMetadata) > 0 {
		return version + "+" + strings.Join(buildMetadata, ".")
	}
	return version
}
