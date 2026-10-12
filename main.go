package main

import (
	"embed"
	"fmt"
	"os"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	appName    = "Audio Switcher"
	appVersion = "1.0.0"
)

//go:embed icons/*.png
var iconFS embed.FS

// Menu command IDs
const (
	cmdCycle     = 9
	cmdAutostart = 10
	cmdSettings  = 11
	cmdSoundCpl  = 12
	cmdExit      = 13
	cmdDevice0   = 1000

	hkCycle   = 1
	hkDevice0 = 100

	timerID = 1
)

type hotkeyAction struct {
	cycle  bool
	device string // pattern from settings
}

type app struct {
	hwnd           uintptr
	trayIcon       uintptr
	balloonIcon    uintptr
	nid            notifyIconData
	trayAdded      bool
	taskbarCreated uint32

	cfgPath string
	cfg     Config
	cfgMod  time.Time

	hotkeys     map[int]hotkeyAction
	hotkeyText  map[string]string // device pattern -> "Ctrl+Alt+1"
	cycleText   string
	lastDefault string
}

var theApp *app

func init() {
	// Window messages, hotkeys and COM all belong to one OS thread.
	runtime.LockOSThread()
}

func main() {
	// Single instance
	r, _, err := pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(utf16p(`Local\AudioSwitcher.mimikm`))))
	if r != 0 && err == syscall.ERROR_ALREADY_EXISTS {
		msgBox("Audio Switcher is already running.\n\nLook for the speaker icon in the system tray (you may need to click the ^ arrow).", mbIconInfo)
		return
	}

	pCoInitializeEx.Call(0, 0x2) // COINIT_APARTMENTTHREADED
	enableDarkMenus()

	a := &app{cfgPath: configPath(), hotkeys: map[int]hotkeyAction{}, hotkeyText: map[string]string{}}
	theApp = a

	cfg, created, mod, cfgErr := loadConfig(a.cfgPath)
	a.cfg, a.cfgMod = cfg, mod

	if err := a.createWindow(); err != nil {
		msgBox("Couldn't start: "+err.Error(), mbIconError)
		return
	}
	a.loadIcons()
	a.addTray()
	problems := a.registerHotkeys()

	switch {
	case cfgErr != nil:
		a.notify("Settings file has an error", cfgErr.Error()+"\nUsing defaults until it's fixed.", true)
	case len(problems) > 0:
		a.notify("Some hotkeys didn't register", strings.Join(problems, "\n"), true)
	case created:
		hint := "Click the tray icon to pick an output."
		if a.cycleText != "" {
			hint = "Press " + a.cycleText + " to switch output, or click the tray icon."
		}
		a.notify(appName+" is running", hint, false)
	}

	pSetTimer.Call(a.hwnd, timerID, 3000, 0)

	var m msgT
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	os.Exit(0)
}

// ---------- window & tray ----------

func (a *app) createWindow() error {
	hinst, _, _ := pGetModuleHandleW.Call(0)
	className := utf16p("AudioSwitcherWnd")
	wc := wndClassEx{
		WndProc:   syscall.NewCallback(wndProc),
		Instance:  hinst,
		ClassName: className,
	}
	wc.Size = uint32(unsafe.Sizeof(wc))
	if r, _, err := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassEx: %v", err)
	}
	// A hidden top-level window (not message-only) so it also receives
	// TaskbarCreated and WM_SETTINGCHANGE broadcasts.
	hwnd, _, err := pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(className)), uintptr(unsafe.Pointer(utf16p(appName))),
		0, 0, 0, 0, 0, 0, 0, hinst, 0)
	if hwnd == 0 {
		return fmt.Errorf("CreateWindowEx: %v", err)
	}
	a.hwnd = hwnd
	tc, _, _ := pRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(utf16p("TaskbarCreated"))))
	a.taskbarCreated = uint32(tc)
	return nil
}

func wndProc(hwnd, msg, wp, lp uintptr) uintptr {
	a := theApp
	switch uint32(msg) {
	case wmTray:
		switch uint32(lp) & 0xFFFF {
		case wmLButtonUp, wmRButtonUp:
			a.showMenu()
		case wmMButtonUp:
			a.cycle()
		}
		return 0
	case wmHotkey:
		a.onHotkey(int(wp))
		return 0
	case wmTimer:
		a.onTimer()
		return 0
	case wmSettingChange:
		if lp != 0 && ptrToString((*uint16)(unsafe.Pointer(lp))) == "ImmersiveColorSet" {
			refreshMenuTheme()
			a.loadIcons()
			if a.trayAdded {
				a.nid.Flags = nifIcon
				a.nid.Icon = a.trayIcon
				pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&a.nid)))
			}
		}
	case wmDestroy:
		a.removeTray()
		pPostQuitMessage.Call(0)
		return 0
	}
	if a != nil && a.taskbarCreated != 0 && uint32(msg) == a.taskbarCreated {
		// Explorer restarted: put the icon back.
		a.trayAdded = false
		a.addTray()
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wp, lp)
	return r
}

func (a *app) addTray() {
	a.nid = notifyIconData{}
	a.nid.Size = uint32(unsafe.Sizeof(a.nid))
	a.nid.Wnd = a.hwnd
	a.nid.ID = 1
	a.nid.Flags = nifMessage | nifIcon | nifTip | nifShowTip
	a.nid.CallbackMessage = wmTray
	a.nid.Icon = a.trayIcon
	copyUTF16(a.nid.Tip[:], a.tooltip())
	r, _, _ := pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&a.nid)))
	a.trayAdded = r != 0
}

func (a *app) removeTray() {
	if a.trayAdded {
		a.nid.Flags = 0
		pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&a.nid)))
		a.trayAdded = false
	}
}

func (a *app) tooltip() string {
	a.lastDefault = DefaultOutputID()
	name := "no output device"
	if devs, err := ListOutputs(); err == nil {
		for _, d := range devs {
			if d.ID == a.lastDefault {
				name = d.Name
			}
		}
	}
	return appName + "\n" + name
}

func (a *app) updateTooltip() {
	if !a.trayAdded {
		return
	}
	a.nid.Flags = nifTip | nifShowTip
	copyUTF16(a.nid.Tip[:], a.tooltip())
	pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&a.nid)))
}

func (a *app) notify(title, text string, warning bool) {
	if !a.trayAdded {
		return
	}
	a.nid.Flags = nifInfo
	copyUTF16(a.nid.InfoTitle[:], title)
	copyUTF16(a.nid.Info[:], text)
	if warning {
		a.nid.InfoFlags = niifWarning | niifNoSound
		a.nid.BalloonIcon = 0
	} else {
		a.nid.InfoFlags = niifUser | niifNoSound
		a.nid.BalloonIcon = a.balloonIcon
		if a.balloonIcon != 0 {
			a.nid.InfoFlags |= niifLarge
		}
	}
	pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&a.nid)))
}

// loadIcons picks a tray glyph that contrasts with the taskbar colour.
func (a *app) loadIcons() {
	variant := "dark" // white glyph for a dark taskbar
	if regDWORD(themeKey, "SystemUsesLightTheme", 0) == 1 {
		variant = "light"
	}
	size, _, _ := pGetSystemMetrics.Call(smCxSmIcon)
	if size == 0 {
		size = 16
	}
	best := 64
	for _, s := range []int{16, 20, 24, 32, 40, 48, 64} {
		if s >= int(size) {
			best = s
			break
		}
	}
	old := a.trayIcon
	a.trayIcon = iconFromPNG(fmt.Sprintf("icons/tray-%s-%d.png", variant, best), int(size))
	if a.trayIcon == 0 {
		a.trayIcon, _, _ = pLoadIconW.Call(0, 32512) // IDI_APPLICATION
	} else if old != 0 {
		pDestroyIcon.Call(old)
	}
	if a.balloonIcon == 0 {
		big, _, _ := pGetSystemMetrics.Call(smCxIcon)
		if big == 0 {
			big = 32
		}
		a.balloonIcon = iconFromPNG("icons/app-256.png", int(big)*2)
	}
}

func iconFromPNG(name string, size int) uintptr {
	data, err := iconFS.ReadFile(name)
	if err != nil || len(data) == 0 {
		return 0
	}
	h, _, _ := pCreateIconFromResourceEx.Call(uintptr(unsafe.Pointer(&data[0])), uintptr(len(data)), 1, 0x00030000,
		uintptr(size), uintptr(size), 0)
	runtime.KeepAlive(data)
	return h
}

// ---------- menu ----------

func appendMenu(m uintptr, flags, id uintptr, text string) {
	var p uintptr
	if text != "" {
		p = uintptr(unsafe.Pointer(utf16p(text)))
	}
	pAppendMenuW.Call(m, flags, id, p)
}

func (a *app) showMenu() {
	devs, err := ListOutputs()
	current := DefaultOutputID()

	m, _, _ := pCreatePopupMenu.Call()
	appendMenu(m, mfString|mfGrayed, 0, "Output device")
	checked := -1
	if err != nil {
		appendMenu(m, mfString|mfGrayed, 0, "  "+err.Error())
	} else if len(devs) == 0 {
		appendMenu(m, mfString|mfGrayed, 0, "  No active output devices")
	}
	for i, d := range devs {
		label := strings.ReplaceAll(d.Name, "&", "&&")
		if hk := a.hotkeyForDevice(d, devs); hk != "" {
			label += "\t" + hk
		}
		appendMenu(m, mfString, uintptr(cmdDevice0+i), label)
		if d.ID == current {
			checked = i
		}
	}
	if checked >= 0 {
		pCheckMenuRadioItem.Call(m, cmdDevice0, uintptr(cmdDevice0+len(devs)-1), uintptr(cmdDevice0+checked), mfByCommand)
	}
	appendMenu(m, mfSeparator, 0, "")
	cycleLabel := "Switch to next device"
	if a.cycleText != "" {
		cycleLabel += "\t" + a.cycleText
	}
	flags := uintptr(mfString)
	if len(devs) < 2 {
		flags |= mfGrayed
	}
	appendMenu(m, flags, cmdCycle, cycleLabel)
	appendMenu(m, mfSeparator, 0, "")
	auto := uintptr(mfString)
	if regValueExists(runKey, runValue) {
		auto |= mfChecked
	}
	appendMenu(m, auto, cmdAutostart, "Start with Windows")
	appendMenu(m, mfString, cmdSettings, "Edit hotkeys && settings…")
	appendMenu(m, mfString, cmdSoundCpl, "Windows sound settings…")
	appendMenu(m, mfSeparator, 0, "")
	appendMenu(m, mfString, cmdExit, "Exit")

	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(a.hwnd) // so the menu closes when you click away
	cmd, _, _ := pTrackPopupMenu.Call(m, tpmReturnCmd|tpmNoNotify|tpmRightButton,
		uintptr(pt.X), uintptr(pt.Y), 0, a.hwnd, 0)
	pPostMessageW.Call(a.hwnd, wmNull, 0, 0)
	pDestroyMenu.Call(m)

	switch c := int(cmd); {
	case c >= cmdDevice0 && c < cmdDevice0+len(devs):
		a.switchTo(devs[c-cmdDevice0], false)
	case c == cmdCycle:
		a.cycle()
	case c == cmdAutostart:
		a.toggleAutostart()
	case c == cmdSettings:
		shellOpen("notepad.exe", `"`+a.cfgPath+`"`)
	case c == cmdSoundCpl:
		shellOpen("control.exe", "mmsys.cpl,,0") // classic Playback tab
	case c == cmdExit:
		pDestroyWindow.Call(a.hwnd)
	}
}

func (a *app) hotkeyForDevice(d Device, devs []Device) string {
	for pattern, text := range a.hotkeyText {
		if match, ok := findDevice(devs, pattern); ok && match.ID == d.ID {
			return text
		}
	}
	return ""
}

func (a *app) toggleAutostart() {
	if regValueExists(runKey, runValue) {
		regDelete(runKey, runValue)
		return
	}
	if !regSetString(runKey, runValue, `"`+exePath()+`"`) {
		a.notify("Couldn't turn on Start with Windows", "Writing to the registry failed.", true)
	}
}

// ---------- switching ----------

func (a *app) switchTo(d Device, fromHotkey bool) {
	if err := SetDefaultOutput(d.ID, a.cfg.SetCommunicationsDevice); err != nil {
		a.notify("Couldn't switch output", err.Error(), true)
		return
	}
	a.updateTooltip()
	if a.cfg.ShowNotifications && fromHotkey {
		a.notify("🔊 "+d.Name, "Now playing through this device", false)
	}
}

// cycleList is the devices the cycle hotkey steps through, in order.
func (a *app) cycleList(devs []Device) []Device {
	if len(a.cfg.CycleDevices) == 0 {
		return devs
	}
	var out []Device
	seen := map[string]bool{}
	for _, p := range a.cfg.CycleDevices {
		if d, ok := findDevice(devs, p); ok && !seen[d.ID] {
			out = append(out, d)
			seen[d.ID] = true
		}
	}
	return out
}

func (a *app) cycle() {
	devs, err := ListOutputs()
	if err != nil {
		a.notify("Couldn't list devices", err.Error(), true)
		return
	}
	list := a.cycleList(devs)
	if len(list) == 0 {
		msg := "No output devices are active."
		if len(a.cfg.CycleDevices) > 0 {
			msg = "None of the devices in cycle_devices are connected right now."
		}
		a.notify("Nothing to switch to", msg, true)
		return
	}
	current := DefaultOutputID()
	next := 0
	for i, d := range list {
		if d.ID == current {
			next = (i + 1) % len(list)
			break
		}
	}
	if list[next].ID == current {
		a.notify("Only one device to switch between", list[next].Name+" is the only one available.", false)
		return
	}
	a.switchTo(list[next], true)
}

func (a *app) onHotkey(id int) {
	act, ok := a.hotkeys[id]
	if !ok {
		return
	}
	if act.cycle {
		a.cycle()
		return
	}
	devs, err := ListOutputs()
	if err != nil {
		a.notify("Couldn't list devices", err.Error(), true)
		return
	}
	d, found := findDevice(devs, act.device)
	if !found {
		a.notify("Device not found", "\""+act.device+"\" isn't connected or enabled right now.", true)
		return
	}
	if d.ID == DefaultOutputID() {
		if a.cfg.ShowNotifications {
			a.notify("🔊 "+d.Name, "Already the output device", false)
		}
		return
	}
	a.switchTo(d, true)
}

// ---------- hotkeys & settings ----------

func (a *app) registerHotkeys() (problems []string) {
	for id := range a.hotkeys {
		pUnregisterHotKey.Call(a.hwnd, uintptr(id))
	}
	a.hotkeys = map[int]hotkeyAction{}
	a.hotkeyText = map[string]string{}
	a.cycleText = ""

	register := func(id int, spec string, act hotkeyAction) (string, bool) {
		hk, err := parseHotkey(spec)
		if err != nil {
			problems = append(problems, err.Error())
			return "", false
		}
		r, _, _ := pRegisterHotKey.Call(a.hwnd, uintptr(id), uintptr(hk.Mods|modNoRepeat), uintptr(hk.VK))
		if r == 0 {
			problems = append(problems, hk.Text+" is already used by another app")
			return "", false
		}
		a.hotkeys[id] = act
		return hk.Text, true
	}

	if strings.TrimSpace(a.cfg.CycleHotkey) != "" {
		if t, ok := register(hkCycle, a.cfg.CycleHotkey, hotkeyAction{cycle: true}); ok {
			a.cycleText = t
		}
	}
	for i, pattern := range a.cfg.sortedDeviceHotkeys() {
		spec := a.cfg.DeviceHotkeys[pattern]
		if strings.TrimSpace(spec) == "" {
			continue
		}
		if t, ok := register(hkDevice0+i, spec, hotkeyAction{device: pattern}); ok {
			a.hotkeyText[pattern] = t
		}
	}
	return problems
}

func (a *app) onTimer() {
	if !a.trayAdded {
		a.addTray() // e.g. started before Explorer was ready
	}
	// Keep the tooltip right if the output was changed elsewhere.
	if cur := DefaultOutputID(); cur != a.lastDefault {
		a.updateTooltip()
	}
	// Reload settings when the file is saved.
	st, err := os.Stat(a.cfgPath)
	if err != nil || st.ModTime().Equal(a.cfgMod) {
		return
	}
	cfg, _, mod, cfgErr := loadConfig(a.cfgPath)
	a.cfgMod = mod
	if cfgErr != nil {
		a.notify("Settings file has an error", cfgErr.Error()+"\nKeeping the previous settings.", true)
		return
	}
	a.cfg = cfg
	if problems := a.registerHotkeys(); len(problems) > 0 {
		a.notify("Some hotkeys didn't register", strings.Join(problems, "\n"), true)
		return
	}
	msg := "Hotkeys updated."
	if a.cycleText != "" {
		msg = "Switch output with " + a.cycleText + "."
	}
	a.notify("Settings reloaded", msg, false)
}
