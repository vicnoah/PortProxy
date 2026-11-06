//go:build windows

package win

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const registryBase = `SYSTEM\CurrentControlSet\Services\PortProxy`

func keyName(typ string) string { return registryBase + `\` + typ + `\tcp` }

// GetProxies reads HKLM\SYSTEM\CurrentControlSet\Services\PortProxy\{type}\tcp.
// Value names are "listenOn/listenPort", values "connectTo/connectPort".
func GetProxies() ([]Proxy, error) {
	var out []Proxy
	for _, typ := range ProxyTypes {
		k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyName(typ), registry.QUERY_VALUE)
		if err != nil {
			continue
		}
		names, err := k.ReadValueNames(-1)
		if err != nil {
			k.Close()
			continue
		}
		for _, name := range names {
			value, _, err := k.GetStringValue(name)
			if err != nil {
				continue
			}
			listen := strings.Split(name, "/")
			if len(listen) != 2 {
				continue
			}
			listenPort, err := strconv.Atoi(listen[1])
			if err != nil {
				continue
			}
			connect := strings.Split(value, "/")
			if len(connect) != 2 {
				continue
			}
			connectPort, err := strconv.Atoi(connect[1])
			if err != nil {
				continue
			}
			out = append(out, Proxy{
				Type:        typ,
				ListenOn:    listen[0],
				ListenPort:  listenPort,
				ConnectTo:   connect[0],
				ConnectPort: connectPort,
			})
		}
		k.Close()
	}
	return out, nil
}

func AddOrUpdateProxy(p Proxy) error {
	if !IsProxyType(p.Type) {
		return fmt.Errorf("Invalid port proxy type (%s).", p.Type)
	}
	kn := keyName(p.Type)
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, kn, registry.SET_VALUE|registry.QUERY_VALUE)
	if err != nil {
		k, _, err = registry.CreateKey(
			registry.LOCAL_MACHINE,
			kn,
			registry.SET_VALUE|registry.QUERY_VALUE,
		)
		if err != nil {
			return err
		}
	}
	defer k.Close()
	name := p.ListenOn + "/" + strconv.Itoa(p.ListenPort)
	value := p.ConnectTo + "/" + strconv.Itoa(p.ConnectPort)
	return k.SetStringValue(name, value)
}

func DeleteProxy(p Proxy) error {
	if !IsProxyType(p.Type) {
		return fmt.Errorf("Invalid port proxy type (%s).", p.Type)
	}
	k, err := registry.OpenKey(registry.LOCAL_MACHINE, keyName(p.Type), registry.SET_VALUE)
	if err != nil {
		return nil
	}
	defer k.Close()
	_ = k.DeleteValue(p.ListenOn + "/" + strconv.Itoa(p.ListenPort))
	return nil
}

func openService() (*mgr.Mgr, *mgr.Service, error) {
	m, err := mgr.Connect()
	if err != nil {
		return nil, nil, err
	}
	s, err := m.OpenService(ServiceName)
	if err != nil {
		m.Disconnect()
		return nil, nil, err
	}
	return m, s, nil
}

func IsServiceRunning() (bool, error) {
	m, s, err := openService()
	if err != nil {
		return false, err
	}
	defer m.Disconnect()
	defer s.Close()
	st, err := s.Query()
	if err != nil {
		return false, err
	}
	return st.State == svc.Running, nil
}

func StartService() error {
	m, s, err := openService()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	defer s.Close()
	return s.Start()
}

// ParamChange tells the IP Helper service to reload its portproxy config.
// The reference app ignores the result, so do we.
func ParamChange() error {
	m, s, err := openService()
	if err != nil {
		return err
	}
	defer m.Disconnect()
	defer s.Close()
	_, _ = s.Control(svc.ParamChange)
	return nil
}

func FlushDNSCache() error {
	proc := windows.NewLazySystemDLL("dnsapi.dll").NewProc("DnsFlushResolverCache")
	r, _, callErr := proc.Call()
	if r == 0 {
		return fmt.Errorf("Flush DNS Cache failed: %w", callErr)
	}
	return nil
}

func DocumentsDir() (string, error) {
	return windows.KnownFolderPath(windows.FOLDERID_Documents, windows.KF_FLAG_DEFAULT)
}

func IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// RelaunchElevated restarts this program with the "runas" verb, which asks
// UAC for the administrator rights HKLM writes need.
func RelaunchElevated() error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	verb, err := windows.UTF16PtrFromString("runas")
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(exe)
	if err != nil {
		return err
	}
	argv := make([]string, 0, len(os.Args)-1)
	for _, a := range os.Args[1:] {
		if strings.ContainsAny(a, " \t\"") {
			argv = append(argv, `"`+strings.ReplaceAll(a, `"`, `\"`)+`"`)
		} else {
			argv = append(argv, a)
		}
	}
	args, err := windows.UTF16PtrFromString(strings.Join(argv, " "))
	if err != nil {
		return err
	}
	cwd, err := windows.UTF16PtrFromString(filepath.Dir(exe))
	if err != nil {
		return err
	}
	return windows.ShellExecute(0, verb, file, args, cwd, windows.SW_SHOWNORMAL)
}

var singletonHandle windows.Handle

// AcquireSingleInstance is a kernel-mutex fallback for RequestSingleInstance
// Lock (whose AF_UNIX socket needs Windows 10 1803+). One per logon session.
func AcquireSingleInstance() bool {
	name, err := windows.UTF16PtrFromString(`Local\PortProxySingleton`)
	if err != nil {
		return true
	}
	h, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		windows.CloseHandle(h)
		return false
	}
	if err != nil || h == 0 {
		return true
	}
	singletonHandle = h
	return true
}

func FocusExistingWindow() {
	user32 := syscall.NewLazyDLL("user32.dll")
	findWindow := user32.NewProc("FindWindowW")
	showWindow := user32.NewProc("ShowWindow")
	setForegroundWindow := user32.NewProc("SetForegroundWindow")
	title, err := windows.UTF16PtrFromString("PortProxy")
	if err != nil {
		return
	}
	hwnd, _, _ := findWindow.Call(0, uintptr(unsafe.Pointer(title)))
	if hwnd == 0 {
		return
	}
	showWindow.Call(hwnd, uintptr(windows.SW_RESTORE))
	setForegroundWindow.Call(hwnd)
}
