package version

// Version is the application build version. Override at build time with:
//
//	go build -ldflags "-X github.com/14mdzk/goscratch/internal/platform/version.Version=v1.3.0"
//
// Untracked local builds default to "dev".
var Version = "dev"
