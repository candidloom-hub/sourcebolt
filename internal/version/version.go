// Package version holds the build identity of the sourcebolt binary.
package version

// Name is the binary and product name.
const Name = "sourcebolt"

// Version is set at build time with -ldflags "-X .../internal/version.Version=v1.2.3".
var Version = "dev"

// String returns "<name> <version>".
func String() string { return Name + " " + Version }
