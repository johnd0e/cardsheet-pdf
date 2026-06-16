package version

import (
	"runtime/debug"
	"strings"
)

// TryBuildInfoVersion extracts a concise version string from debug.ReadBuildInfo.
// It prefers module versions, then VCS revisions, and falls back to "local".
func TryBuildInfoVersion() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "local"
	}
	return VersionFromBuildInfo(info)
}

// VersionFromBuildInfo returns a concise version string for build info.
func VersionFromBuildInfo(info *debug.BuildInfo) string {
	if info == nil {
		return "local"
	}

	if info.Main.Version != "" && info.Main.Version != "(devel)" {
		return info.Main.Version
	}

	var rev string
	modified := false
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			modified = s.Value == "true"
		}
	}

	if rev != "" {
		v := shortRevision(rev)
		if modified {
			v += "+dirty"
		}
		return v
	}

	return "local"
}

// FindDepVersion searches build info deps for an exact module path and returns its version or "unknown".
func FindDepVersion(modulePath string) string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}
	return DepVersionFromBuildInfo(info, modulePath)
}

// DepVersionFromBuildInfo returns a dependency version for an exact module path.
func DepVersionFromBuildInfo(info *debug.BuildInfo, modulePath string) string {
	if info == nil {
		return "unknown"
	}

	for _, d := range info.Deps {
		if d.Path != modulePath {
			continue
		}
		if d.Replace != nil {
			return moduleVersion(*d.Replace)
		}
		return moduleVersion(*d)
	}

	return "unknown"
}

func moduleVersion(m debug.Module) string {
	if m.Version != "" {
		return m.Version
	}
	if m.Path != "" {
		return m.Path + "@local"
	}
	return "unknown"
}

func shortRevision(rev string) string {
	rev = strings.TrimSpace(rev)
	if len(rev) > 12 {
		return rev[:12]
	}
	return rev
}
