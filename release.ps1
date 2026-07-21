New-Item -ItemType Directory -Force dist

go test ./gui/...
go vet ./gui/...

go build `
  -trimpath `
  -ldflags="-s -w -H=windowsgui" `
  -o dist\eqdps-windows-amd64.exe `
  ./gui

Get-FileHash dist\eqdps-windows-amd64.exe -Algorithm SHA256
