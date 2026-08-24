//go:build !windows

package native

func SupportWindowOppacity() bool {
	return false
}
