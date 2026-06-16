package version

import (
	"runtime/debug"
	"testing"
)

func TestVersionFromBuildInfoPrefersModuleVersion(t *testing.T) {
	got := VersionFromBuildInfo(&debug.BuildInfo{
		Main: debug.Module{
			Path:    "cardsheet-pdf",
			Version: "v1.2.3",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "1234567890abcdef"},
		},
	})

	if got != "v1.2.3" {
		t.Fatalf("VersionFromBuildInfo() = %q, want %q", got, "v1.2.3")
	}
}

func TestVersionFromBuildInfoUsesShortDirtyRevision(t *testing.T) {
	got := VersionFromBuildInfo(&debug.BuildInfo{
		Main: debug.Module{
			Path:    "cardsheet-pdf",
			Version: "(devel)",
		},
		Settings: []debug.BuildSetting{
			{Key: "vcs.revision", Value: "1234567890abcdef"},
			{Key: "vcs.modified", Value: "true"},
		},
	})

	if got != "1234567890ab+dirty" {
		t.Fatalf("VersionFromBuildInfo() = %q, want %q", got, "1234567890ab+dirty")
	}
}

func TestVersionFromBuildInfoFallsBackToLocal(t *testing.T) {
	got := VersionFromBuildInfo(&debug.BuildInfo{
		Main: debug.Module{
			Path:    "cardsheet-pdf",
			Version: "(devel)",
		},
	})

	if got != "local" {
		t.Fatalf("VersionFromBuildInfo() = %q, want %q", got, "local")
	}
}

func TestDepVersionFromBuildInfoUsesExactModulePath(t *testing.T) {
	info := &debug.BuildInfo{
		Deps: []*debug.Module{
			{Path: "github.com/pdfcpu/pdfcpu-extra", Version: "v9.9.9"},
			{Path: "github.com/pdfcpu/pdfcpu", Version: "v0.13.0"},
		},
	}

	got := DepVersionFromBuildInfo(info, "github.com/pdfcpu/pdfcpu")
	if got != "v0.13.0" {
		t.Fatalf("DepVersionFromBuildInfo() = %q, want %q", got, "v0.13.0")
	}
}

func TestDepVersionFromBuildInfoUsesReplacement(t *testing.T) {
	info := &debug.BuildInfo{
		Deps: []*debug.Module{
			{
				Path:    "github.com/pdfcpu/pdfcpu",
				Version: "v0.13.0",
				Replace: &debug.Module{
					Path: "D:/src/pdfcpu",
				},
			},
		},
	}

	got := DepVersionFromBuildInfo(info, "github.com/pdfcpu/pdfcpu")
	if got != "D:/src/pdfcpu@local" {
		t.Fatalf("DepVersionFromBuildInfo() = %q, want %q", got, "D:/src/pdfcpu@local")
	}
}
