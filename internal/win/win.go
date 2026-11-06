// Package win holds the Windows-only side of PortProxy: the netsh
// portproxy registry keys, the IP Helper service, DNS flushing, and process
// level helpers. On other platforms it compiles to harmless stubs so the
// data layer and the view can be tested off Windows.
package win

import (
	"errors"
	"os"
	"path/filepath"
)

var ErrUnsupported = errors.New("this operation requires Windows")

const (
	ServiceName         = "iphlpsvc"
	ServiceFriendlyName = "IP Helper"
)

// ProxyTypes are the four netsh interface portproxy types.
var ProxyTypes = []string{"v4tov4", "v4tov6", "v6tov4", "v6tov6"}

// Proxy is one active forwarding entry read from the registry.
type Proxy struct {
	Type        string
	ListenOn    string
	ListenPort  int
	ConnectTo   string
	ConnectPort int
}

func IsProxyType(s string) bool {
	for _, t := range ProxyTypes {
		if t == s {
			return true
		}
	}
	return false
}

// UpdateQuitSignalPath is the file the NSIS installer writes before an
// in-place (overwrite) upgrade. An elevated running instance cannot be
// closed by the non-elevated installer through WM_CLOSE (UIPI), so the app
// polls this file and quits itself, letting the installer replace its files.
func UpdateQuitSignalPath() string {
	return filepath.Join(os.TempDir(), "PortProxy-update.quit")
}

func ClearUpdateQuitSignal() { _ = os.Remove(UpdateQuitSignalPath()) }

func UpdateQuitSignalSet() bool {
	_, err := os.Stat(UpdateQuitSignalPath())
	return err == nil
}
