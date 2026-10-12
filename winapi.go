package main

// Thin wrappers around the Win32 calls the app needs. Only the standard
// library is used, so the project builds with nothing but a Go toolchain.

import (
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pRegisterClassExW         = user32.NewProc("RegisterClassExW")
	pCreateWindowExW          = user32.NewProc("CreateWindowExW")
	pDefWindowProcW           = user32.NewProc("DefWindowProcW")
	pDestroyWindow            = user32.NewProc("DestroyWindow")
	pGetMessageW              = user32.NewProc("GetMessageW")
	pTranslateMessage         = user32.NewProc("TranslateMessage")
	pDispatchMessageW         = user32.NewProc("DispatchMessageW")
	pPostQuitMessage          = user32.NewProc("PostQuitMessage")
	pPostMessageW             = user32.NewProc("PostMessageW")
	pCreatePopupMenu          = user32.NewProc("CreatePopupMenu")
	pAppendMenuW              = user32.NewProc("AppendMenuW")
	pCheckMenuRadioItem       = user32.NewProc("CheckMenuRadioItem")
	pSetMenuDefaultItem       = user32.NewProc("SetMenuDefaultItem")
	pTrackPopupMenu           = user32.NewProc("TrackPopupMenu")
	pDestroyMenu              = user32.NewProc("DestroyMenu")
	pGetCursorPos             = user32.NewProc("GetCursorPos")
	pSetForegroundWindow      = user32.NewProc("SetForegroundWindow")
	pSetTimer                 = user32.NewProc("SetTimer")
	pRegisterHotKey           = user32.NewProc("RegisterHotKey")
	pUnregisterHotKey         = user32.NewProc("UnregisterHotKey")
	pRegisterWindowMessageW   = user32.NewProc("RegisterWindowMessageW")
	pCreateIconFromResourceEx = user32.NewProc("CreateIconFromResourceEx")
	pDestroyIcon              = user32.NewProc("DestroyIcon")
	pLoadIconW                = user32.NewProc("LoadIconW")
	pGetSystemMetrics         = user32.NewProc("GetSystemMetrics")
	pMessageBoxW              = user32.NewProc("MessageBoxW")

	pShellNotifyIconW = shell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW    = shell32.NewProc("ShellExecuteW")

	pGetModuleHandleW   = kernel32.NewProc("GetModuleHandleW")
	pCreateMutexW       = kernel32.NewProc("CreateMutexW")
	pGetProcAddress     = kernel32.NewProc("GetProcAddress")
	pLoadLibraryExW     = kernel32.NewProc("LoadLibraryExW")
	pGetModuleFileNameW = kernel32.NewProc("GetModuleFileNameW")

	pCoInitializeEx   = ole32.NewProc("CoInitializeEx")
	pCoCreateInstance = ole32.NewProc("CoCreateInstance")
	pCoTaskMemFree    = ole32.NewProc("CoTaskMemFree")
	pPropVariantClear = ole32.NewProc("PropVariantClear")

	pRegSetValueExW  = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValueW = advapi32.NewProc("RegDeleteValueW")
)

const (
	wmNull          = 0x0000
	wmDestroy       = 0x0002
	wmSettingChange = 0x001A
	wmTimer         = 0x0113
	wmHotkey        = 0x0312
	wmLButtonUp     = 0x0202
	wmRButtonUp     = 0x0205
	wmMButtonUp     = 0x0208
	wmApp           = 0x8000
	wmTray          = wmApp + 1

	mfString    = 0x0000
	mfGrayed    = 0x0001
	mfChecked   = 0x0008
	mfSeparator = 0x0800
	mfByCommand = 0x0000

	tpmRightButton = 0x0002
	tpmNoNotify    = 0x0080
	tpmReturnCmd   = 0x0100

	nimAdd    = 0
	nimModify = 1
	nimDelete = 2

	nifMessage  = 0x01
	nifIcon     = 0x02
	nifTip      = 0x04
	nifInfo     = 0x10
	nifShowTip  = 0x80
	niifWarning = 0x02
	niifUser    = 0x04
	niifNoSound = 0x10
	niifLarge   = 0x20

	smCxSmIcon = 49
	smCxIcon   = 11

	mbIconInfo  = 0x40
	mbIconError = 0x10
)

type point struct{ X, Y int32 }

type wndClassEx struct {
	Size       uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type msgT struct {
	Hwnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      point
	Private uint32
}

// NOTIFYICONDATAW (full Vista+ layout). Go's natural alignment matches the C struct.
type notifyIconData struct {
	Size            uint32
	Wnd             uintptr
	ID              uint32
	Flags           uint32
	CallbackMessage uint32
	Icon            uintptr
	Tip             [128]uint16
	State           uint32
	StateMask       uint32
	Info            [256]uint16
	Version         uint32
	InfoTitle       [64]uint16
	InfoFlags       uint32
	GUIDItem        syscall.GUID
	BalloonIcon     uintptr
}

func utf16p(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("?")
	}
	return p
}

func ptrToString(p *uint16) string {
	if p == nil {
		return ""
	}
	var buf []uint16
	for ptr := unsafe.Pointer(p); ; ptr = unsafe.Add(ptr, 2) {
		c := *(*uint16)(ptr)
		if c == 0 {
			break
		}
		buf = append(buf, c)
	}
	return string(utf16.Decode(buf))
}

// copyUTF16 writes s into a fixed-size WCHAR buffer, truncating if needed.
func copyUTF16(dst []uint16, s string) {
	u := utf16.Encode([]rune(s))
	if len(u) > len(dst)-1 {
		u = u[:len(dst)-1]
	}
	n := copy(dst, u)
	for i := n; i < len(dst); i++ {
		dst[i] = 0
	}
}

func msgBox(text string, flags uintptr) {
	pMessageBoxW.Call(0, uintptr(unsafe.Pointer(utf16p(text))), uintptr(unsafe.Pointer(utf16p(appName))), flags)
}

func shellOpen(file, args string) {
	var a uintptr
	if args != "" {
		a = uintptr(unsafe.Pointer(utf16p(args)))
	}
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(utf16p("open"))), uintptr(unsafe.Pointer(utf16p(file))), a, 0, 1)
}

func exePath() string {
	buf := make([]uint16, 32768)
	n, _, _ := pGetModuleFileNameW.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	return string(utf16.Decode(buf[:n]))
}

// ---- registry (HKCU only) ----

const (
	runKey      = `Software\Microsoft\Windows\CurrentVersion\Run`
	themeKey    = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`
	runValue    = "AudioSwitcher"
	keyRead     = 0x20019
	keySetValue = 0x0002
	regSz       = 1
)

func regOpen(path string, access uint32) (syscall.Handle, bool) {
	var h syscall.Handle
	if err := syscall.RegOpenKeyEx(syscall.HKEY_CURRENT_USER, utf16p(path), 0, access, &h); err != nil {
		return 0, false
	}
	return h, true
}

func regValueExists(path, name string) bool {
	h, ok := regOpen(path, keyRead)
	if !ok {
		return false
	}
	defer syscall.RegCloseKey(h)
	var typ, n uint32
	return syscall.RegQueryValueEx(h, utf16p(name), nil, &typ, nil, &n) == nil
}

func regDWORD(path, name string, def uint32) uint32 {
	h, ok := regOpen(path, keyRead)
	if !ok {
		return def
	}
	defer syscall.RegCloseKey(h)
	var typ, v uint32
	n := uint32(4)
	if syscall.RegQueryValueEx(h, utf16p(name), nil, &typ, (*byte)(unsafe.Pointer(&v)), &n) != nil || typ != syscall.REG_DWORD {
		return def
	}
	return v
}

func regSetString(path, name, value string) bool {
	h, ok := regOpen(path, keySetValue)
	if !ok {
		return false
	}
	defer syscall.RegCloseKey(h)
	u := utf16.Encode([]rune(value + "\x00"))
	r, _, _ := pRegSetValueExW.Call(uintptr(h), uintptr(unsafe.Pointer(utf16p(name))), 0, regSz,
		uintptr(unsafe.Pointer(&u[0])), uintptr(len(u)*2))
	return r == 0
}

func regDelete(path, name string) {
	h, ok := regOpen(path, keySetValue)
	if !ok {
		return
	}
	defer syscall.RegCloseKey(h)
	pRegDeleteValueW.Call(uintptr(h), uintptr(unsafe.Pointer(utf16p(name))))
}

// ---- dark context menus (Windows 10 1903+), best effort ----

func enableDarkMenus() {
	// LOAD_LIBRARY_SEARCH_SYSTEM32: only ever load uxtheme from System32.
	h, _, _ := pLoadLibraryExW.Call(uintptr(unsafe.Pointer(utf16p("uxtheme.dll"))), 0, 0x800)
	if h == 0 {
		return
	}
	// Ordinal 135 = SetPreferredAppMode, 136 = FlushMenuThemes (undocumented).
	setMode, _, _ := pGetProcAddress.Call(h, 135)
	flush, _, _ := pGetProcAddress.Call(h, 136)
	if setMode != 0 {
		syscall.SyscallN(setMode, 1) // AllowDark: follow the system setting
	}
	if flush != 0 {
		syscall.SyscallN(flush)
	}
	darkFlush = flush
}

var darkFlush uintptr

func refreshMenuTheme() {
	if darkFlush != 0 {
		syscall.SyscallN(darkFlush)
	}
}
