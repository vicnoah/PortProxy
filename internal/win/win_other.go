//go:build !windows

package win

import (
	"os"
)

func GetProxies() ([]Proxy, error) { return nil, nil }

func AddOrUpdateProxy(Proxy) error { return ErrUnsupported }

func DeleteProxy(Proxy) error { return ErrUnsupported }

func IsServiceRunning() (bool, error) { return true, nil }

func StartService() error { return ErrUnsupported }

func ParamChange() error { return ErrUnsupported }

func FlushDNSCache() error { return ErrUnsupported }

func DocumentsDir() (string, error) {
	if p := os.Getenv("PORTPROXY_DOCUMENTS"); p != "" {
		return p, nil
	}
	return os.UserHomeDir()
}

func IsElevated() bool { return true }

func RelaunchElevated() error { return ErrUnsupported }

func AcquireSingleInstance() bool { return true }

func FocusExistingWindow() {}
