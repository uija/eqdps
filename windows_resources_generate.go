package main

// Generate the executable icon before a Windows amd64 build. The platform and
// architecture suffix keeps this resource out of Linux and other builds.
// Run on the build host before setting GOOS for cross-compilation.
//go:generate go run github.com/akavel/rsrc@v0.10.2 -arch amd64 -ico img/icons/eqdps.ico -o rsrc_windows_amd64.syso
