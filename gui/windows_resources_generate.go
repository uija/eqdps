package main

// Regenerate the Windows icon resources after changing img/eqdps-icon.svg
// and its derived ICO file. The architecture suffixes keep these resources
// out of Linux and macOS builds.
//
//go:generate go run github.com/tc-hib/go-winres@v0.3.3 simply --arch amd64,arm64 --out rsrc --manifest none --icon ../img/icons/eqdps.ico
