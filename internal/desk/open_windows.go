//go:build windows

package desk

import (
	"sync"
	"sync/atomic"
	"unsafe"

	webview "github.com/jchv/go-webview2"
	"golang.org/x/sys/windows"
)

var (
	user32            = windows.NewLazySystemDLL("user32.dll")
	dwmapi            = windows.NewLazySystemDLL("dwmapi.dll")
	setWindowLong     = user32.NewProc("SetWindowLongPtrW")
	setWindowPos      = user32.NewProc("SetWindowPos")
	releaseCapture    = user32.NewProc("ReleaseCapture")
	sendMessageW      = user32.NewProc("SendMessageW")
	postMessageW      = user32.NewProc("PostMessageW")
	showWindow        = user32.NewProc("ShowWindow")
	isZoomed          = user32.NewProc("IsZoomed")
	callWindowProc    = user32.NewProc("CallWindowProcW")
	defWindowProc     = user32.NewProc("DefWindowProcW")
	monitorFromWindow = user32.NewProc("MonitorFromWindow")
	getMonitorInfo    = user32.NewProc("GetMonitorInfoW")
	getSystemMetrics  = user32.NewProc("GetSystemMetrics")
	setForeground     = user32.NewProc("SetForegroundWindow")
	dwmAttribute      = dwmapi.NewProc("DwmSetWindowAttribute")
	frameProcCB       = windows.NewCallback(frameProc)
	stay              atomic.Bool
	liveHWND          uintptr
)

type frameWindow struct {
	proc       uintptr
	fullscreen bool
	zoomed     bool
}

var (
	frameMu sync.Mutex
	frames  = map[uintptr]*frameWindow{}
)

func open(title, url string) bool {
	window := webview.NewWithOptions(webview.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview.WindowOptions{
			Title:  title,
			Width:  1120,
			Height: 760,
			IconId: 1,
			Center: true,
		},
	})
	if window == nil {
		browser(url)
		return false
	}
	defer window.Destroy()
	hwnd := uintptr(window.Window())
	liveHWND = hwnd
	ownFrame(hwnd)
	_ = window.Bind("bnFrame", func(action string) {
		frameAction(hwnd, action)
	})
	window.SetTitle(title)
	window.Navigate(url)
	window.Run()
	return true
}

func ownFrame(hwnd uintptr) {
	if hwnd == 0 {
		return
	}
	const style = uintptr(0x80000000 | 0x00040000 | 0x00020000 | 0x00010000 | 0x00080000 | 0x10000000 | 0x02000000)
	_, _, _ = setWindowLong.Call(hwnd, ^uintptr(15), style)
	_, _, _ = setWindowPos.Call(hwnd, 0, 0, 0, 0, 0, 0x0020|0x0002|0x0001|0x0004)
	installFrameProc(hwnd)
	var round int32 = 2
	_, _, _ = dwmAttribute.Call(hwnd, 33, uintptr(unsafe.Pointer(&round)), 4)
	var dark int32 = 1
	_, _, _ = dwmAttribute.Call(hwnd, 20, uintptr(unsafe.Pointer(&dark)), 4)
	var border uint32 = 0x002C3A31
	_, _, _ = dwmAttribute.Call(hwnd, 34, uintptr(unsafe.Pointer(&border)), 4)
}

func frameAction(hwnd uintptr, action string) {
	if hwnd == 0 {
		return
	}
	switch action {
	case "drag":
		_, _, _ = releaseCapture.Call()
		_, _, _ = sendMessageW.Call(hwnd, 0x00A1, 2, 0)
	case "min":
		_, _, _ = showWindow.Call(hwnd, 6)
	case "max":
		if leaveFullscreen(hwnd) {
			return
		}
		zoomed, _, _ := isZoomed.Call(hwnd)
		if zoomed != 0 {
			_, _, _ = showWindow.Call(hwnd, 9)
			return
		}
		_, _, _ = showWindow.Call(hwnd, 3)
	case "full":
		toggleFullscreen(hwnd)
	case "close":
		_, _, _ = postMessageW.Call(hwnd, 0x0010, 0, 0)
	}
}

func installFrameProc(hwnd uintptr) {
	prev, _, _ := setWindowLong.Call(hwnd, ^uintptr(3), frameProcCB)
	if prev == 0 {
		return
	}
	frameMu.Lock()
	frames[hwnd] = &frameWindow{proc: prev}
	frameMu.Unlock()
}

func frameProc(hwnd, msg, wp, lp uintptr) uintptr {
	if msg == 0x0010 && stay.Load() {
		_, _, _ = showWindow.Call(hwnd, 0)
		return 0
	}
	proc, fullscreen, ok := frameSnapshot(hwnd)
	if !ok {
		r, _, _ := defWindowProc.Call(hwnd, msg, wp, lp)
		return r
	}
	r, _, _ := callWindowProc.Call(proc, hwnd, msg, wp, lp)
	if msg == 0x0024 {
		fitWorkArea(hwnd, lp, fullscreen)
	}
	if msg == 0x0002 {
		liveHWND = 0
	}
	return r
}

func frameSnapshot(hwnd uintptr) (uintptr, bool, bool) {
	frameMu.Lock()
	defer frameMu.Unlock()
	st := frames[hwnd]
	if st == nil {
		return 0, false, false
	}
	return st.proc, st.fullscreen, true
}

func fitWorkArea(hwnd, lp uintptr, fullscreen bool) {
	if lp == 0 {
		return
	}
	hmon, _, _ := monitorFromWindow.Call(hwnd, 2)
	if hmon == 0 {
		return
	}
	info := struct {
		cbSize    uint32
		rcMonitor rect
		rcWork    rect
		dwFlags   uint32
	}{cbSize: 40}
	ok, _, _ := getMonitorInfo.Call(hmon, uintptr(unsafe.Pointer(&info)))
	if ok == 0 {
		return
	}
	wide, _, _ := getSystemMetrics.Call(32)
	pad, _, _ := getSystemMetrics.Call(92)
	pos, size := maxPlacement(info.rcWork, info.rcMonitor, int32(wide+pad), fullscreen)
	mmi := (*struct {
		reserved, maxSize, maxPosition, minTrack, maxTrack point
	})(unsafe.Pointer(lp))
	mmi.maxPosition = pos
	mmi.maxSize = size
	mmi.maxTrack = size
}

func leaveFullscreen(hwnd uintptr) bool {
	frameMu.Lock()
	st := frames[hwnd]
	if st == nil || !st.fullscreen {
		frameMu.Unlock()
		return false
	}
	zoomed := st.zoomed
	st.fullscreen = false
	frameMu.Unlock()
	_, _, _ = showWindow.Call(hwnd, 9)
	if zoomed {
		_, _, _ = showWindow.Call(hwnd, 3)
	}
	return true
}

func toggleFullscreen(hwnd uintptr) {
	frameMu.Lock()
	st := frames[hwnd]
	if st == nil {
		frameMu.Unlock()
		return
	}
	if st.fullscreen {
		frameMu.Unlock()
		leaveFullscreen(hwnd)
		return
	}
	frameMu.Unlock()
	zoomed, _, _ := isZoomed.Call(hwnd)
	frameMu.Lock()
	if st = frames[hwnd]; st != nil {
		st.zoomed = zoomed != 0
		st.fullscreen = true
	}
	frameMu.Unlock()
	if zoomed != 0 {
		_, _, _ = showWindow.Call(hwnd, 9)
	}
	_, _, _ = showWindow.Call(hwnd, 3)
}

func setStay(on bool) { stay.Store(on) }

func active() bool { return liveHWND != 0 }

func reveal() bool {
	if liveHWND == 0 {
		return false
	}
	_, _, _ = showWindow.Call(liveHWND, 9)
	_, _, _ = setForeground.Call(liveHWND)
	return true
}

func quit() {
	if liveHWND == 0 {
		return
	}
	_, _, _ = postMessageW.Call(liveHWND, 0x0010, 0, 0)
}
