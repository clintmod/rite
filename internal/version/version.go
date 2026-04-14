package version

import (
	_ "embed"
	"regexp"
	"runtime/debug"
	"strings"
)

// rxPseudoVersion matches Go module pseudo-versions like
// v1.4.5-0.20260414175916-2e9d6e67c209, which Go synthesizes for local
// builds inside a module context from the nearest ancestor tag. The
// signature is a 14-digit timestamp and 12-char commit hash at the end,
// which covers all three pseudo-version forms (vX.0.0-…, vX.Y.Z-0.…,
// vX.Y.Z-pre.0.…). These are not real releases and should fall through
// to the embedded-version + commit/dirty path.
var rxPseudoVersion = regexp.MustCompile(`[-.]\d{14}-[0-9a-f]{12}$`)

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
//  1. `go install …@vX.Y.Z` / `…@latest` — info.Main.Version is a real tag.
//     Use it directly; no commit/dirty suffix (the tag is authoritative).
//  2. Local `go build` / `go install ./cmd/rite` from a checkout —
//     Main.Version is either "(devel)" (outside a module context) or a
//     synthesized pseudo-version like v1.4.5-0.20260414175916-2e9d6e67c209
//     (Go derives this from the nearest ancestor tag, which in our merged
//     upstream history can be a go-task tag). Treat both as fallback: keep
//     the embedded version.txt value and decorate with commit/dirty.
//  3. No build info at all — just the embedded value.
//
// The goreleaser path is unaffected: its -ldflags overwrite of `version`
// runs before init, and Main.Version is empty for ldflags-injected binaries.
func resolveVersion(embedded string, info *debug.BuildInfo, ok bool) (string, string, bool) {
	v := strings.TrimSpace(embedded)
	if !ok {
		return v, "", false
	}
	mv := info.Main.Version
	if mv != "" && mv != "(devel)" && !rxPseudoVersion.MatchString(mv) {
		return mv, "", false
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
