//go:build windows

package install

import (
	"errors"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type State struct {
	Ready    bool   `json:"ready"`
	Startup  bool   `json:"startup"`
	Firewall bool   `json:"firewall"`
	Error    string `json:"error,omitempty"`
}

const (
	runKey   = `Software\Microsoft\Windows\CurrentVersion\Run`
	runName  = "BrightNet Node"
	ruleName = "BrightNet Node"
)

var (
	shell32        = windows.NewLazySystemDLL("shell32.dll")
	shellExecuteEx = shell32.NewProc("ShellExecuteExW")
	user32         = windows.NewLazySystemDLL("user32.dll")
	allowFore      = user32.NewProc("AllowSetForegroundWindow")
	nodeMutex      windows.Handle
)

func Inspect(exe string) State {
	state := State{Startup: startupOK(exe), Firewall: firewallOK(exe)}
	state.Ready = state.Startup && state.Firewall
	return state
}

func Enable(exe string) State {
	var notes []string
	if !startupOK(exe) {
		if err := writeStartup(exe); err != nil {
			notes = append(notes, "Le démarrage avec Windows n’a pas été enregistré.")
		}
	}
	if !firewallOK(exe) {
		if err := elevateFirewall(exe); err != nil {
			notes = append(notes, err.Error())
		}
	}
	state := Inspect(exe)
	state.Error = strings.Join(notes, " ")
	return state
}

func OpenFirewall(exe string) error {
	rule := "name=\"" + ruleName + "\""
	_ = exec.Command("netsh", "advfirewall", "firewall", "delete", "rule", rule).Run()
	cmd := exec.Command("netsh", "advfirewall", "firewall", "add", "rule",
		rule, "dir=in", "action=allow", "program="+exe,
		"enable=yes", "profile=private,domain",
		"description=BrightNet Node sur le réseau privé")
	out, err := cmd.CombinedOutput()
	if err != nil {
		if len(out) > 0 {
			return errors.New(strings.TrimSpace(string(out)))
		}
		return err
	}
	return nil
}

func Claim() bool {
	name, err := windows.UTF16PtrFromString(`Local\BrightNetNode`)
	if err != nil {
		return true
	}
	handle, err := windows.CreateMutex(nil, false, name)
	if errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
		if handle != 0 {
			_ = windows.CloseHandle(handle)
		}
		return false
	}
	if err != nil {
		return true
	}
	nodeMutex = handle
	return true
}

func Release() {
	if nodeMutex != 0 {
		_ = windows.CloseHandle(nodeMutex)
		nodeMutex = 0
	}
}

func Ask(urlFile string) {
	_, _, _ = allowFore.Call(^uintptr(0))
	body, err := os.ReadFile(urlFile)
	if err != nil {
		return
	}
	url := strings.TrimSpace(string(body))
	if url == "" {
		return
	}
	client := &http.Client{Timeout: 2 * time.Second}
	_, _ = client.Post(url+"/api/window", "text/plain", nil)
}

func startupOK(exe string) bool {
	key, err := registry.OpenKey(registry.CURRENT_USER, runKey, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer key.Close()
	value, _, err := key.GetStringValue(runName)
	if err != nil {
		return false
	}
	return SameExecutable(value, exe)
}

func writeStartup(exe string) error {
	key, _, err := registry.CreateKey(registry.CURRENT_USER, runKey, registry.SET_VALUE)
	if err != nil {
		return err
	}
	defer key.Close()
	return key.SetStringValue(runName, StartupValue(exe))
}

func firewallOK(exe string) bool {
	cmd := exec.Command("netsh", "advfirewall", "firewall", "show", "rule", "name=\""+ruleName+"\"")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(string(out)), strings.ToLower(exe))
}

func elevateFirewall(exe string) error {
	verb, _ := windows.UTF16PtrFromString("runas")
	file, _ := windows.UTF16PtrFromString(exe)
	args, _ := windows.UTF16PtrFromString("--setup")
	info := shellExecuteInfo{
		fMask:        0x00000040,
		lpVerb:       uintptr(unsafe.Pointer(verb)),
		lpFile:       uintptr(unsafe.Pointer(file)),
		lpParameters: uintptr(unsafe.Pointer(args)),
		nShow:        0,
	}
	info.cbSize = uint32(unsafe.Sizeof(info))
	ok, _, callErr := shellExecuteEx.Call(uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		if errors.Is(callErr, windows.Errno(1223)) {
			return errors.New("La confirmation Windows a été annulée.")
		}
		if callErr != windows.ERROR_SUCCESS {
			return errors.New("Le pare-feu n’a pas été ouvert.")
		}
		return errors.New("Le pare-feu n’a pas été ouvert.")
	}
	if info.hProcess == 0 {
		return errors.New("Le pare-feu n’a pas été ouvert.")
	}
	handle := windows.Handle(info.hProcess)
	defer windows.CloseHandle(handle)
	_, _ = windows.WaitForSingleObject(handle, 0xFFFFFFFF)
	var code uint32
	if err := windows.GetExitCodeProcess(handle, &code); err != nil || code != 0 {
		return errors.New("Le pare-feu n’a pas été ouvert.")
	}
	return nil
}

type shellExecuteInfo struct {
	cbSize         uint32
	fMask          uint32
	hwnd           uintptr
	lpVerb         uintptr
	lpFile         uintptr
	lpParameters   uintptr
	lpDirectory    uintptr
	nShow          int32
	hInstApp       uintptr
	lpIDList       uintptr
	lpClass        uintptr
	hkeyClass      uintptr
	dwHotKey       uint32
	hIconOrMonitor uintptr
	hProcess       uintptr
}
