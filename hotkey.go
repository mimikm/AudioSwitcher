package main

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	modAlt      = 0x0001
	modControl  = 0x0002
	modShift    = 0x0004
	modWin      = 0x0008
	modNoRepeat = 0x4000
)

type hotkey struct {
	Mods uint32
	VK   uint32
	Text string // normalised, e.g. "Ctrl+Alt+F12"
}

var namedKeys = map[string]struct {
	vk   uint32
	name string
}{
	"space": {0x20, "Space"}, "enter": {0x0D, "Enter"}, "return": {0x0D, "Enter"}, "tab": {0x09, "Tab"},
	"esc": {0x1B, "Esc"}, "escape": {0x1B, "Esc"}, "backspace": {0x08, "Backspace"},
	"insert": {0x2D, "Insert"}, "ins": {0x2D, "Insert"}, "delete": {0x2E, "Delete"}, "del": {0x2E, "Delete"},
	"home": {0x24, "Home"}, "end": {0x23, "End"}, "pageup": {0x21, "PageUp"}, "pgup": {0x21, "PageUp"},
	"pagedown": {0x22, "PageDown"}, "pgdn": {0x22, "PageDown"},
	"up": {0x26, "Up"}, "down": {0x28, "Down"}, "left": {0x25, "Left"}, "right": {0x27, "Right"},
	"pause": {0x13, "Pause"}, "scrolllock": {0x91, "ScrollLock"}, "printscreen": {0x2C, "PrintScreen"},
	"medianext": {0xB0, "MediaNext"}, "mediaprev": {0xB1, "MediaPrev"}, "mediaplaypause": {0xB3, "MediaPlayPause"},
	"plus": {0xBB, "="}, "=": {0xBB, "="}, "-": {0xBD, "-"}, "minus": {0xBD, "-"}, ",": {0xBC, ","}, ".": {0xBE, "."},
	"/": {0xBF, "/"}, ";": {0xBA, ";"}, "'": {0xDE, "'"}, "[": {0xDB, "["}, "]": {0xDD, "]"}, "\\": {0xDC, "\\"}, "`": {0xC0, "`"},
	"numadd": {0x6B, "NumAdd"}, "numsubtract": {0x6D, "NumSubtract"}, "nummultiply": {0x6A, "NumMultiply"}, "numdivide": {0x6F, "NumDivide"},
}

func parseHotkey(s string) (hotkey, error) {
	var hk hotkey
	parts := strings.Split(s, "+")
	var modNames []string
	keyName := ""
	for _, raw := range parts {
		p := strings.ToLower(strings.TrimSpace(raw))
		switch p {
		case "":
			return hk, fmt.Errorf("%q isn't a valid hotkey", s)
		case "ctrl", "control":
			hk.Mods |= modControl
		case "alt":
			hk.Mods |= modAlt
		case "shift":
			hk.Mods |= modShift
		case "win", "windows", "super":
			hk.Mods |= modWin
		default:
			if hk.VK != 0 {
				return hk, fmt.Errorf("%q has more than one key", s)
			}
			vk, name, ok := keyFromName(p)
			if !ok {
				return hk, fmt.Errorf("%q: unknown key %q", s, strings.TrimSpace(raw))
			}
			hk.VK, keyName = vk, name
		}
	}
	if hk.VK == 0 {
		return hk, fmt.Errorf("%q needs a key as well as modifiers", s)
	}
	if hk.Mods&modControl != 0 {
		modNames = append(modNames, "Ctrl")
	}
	if hk.Mods&modAlt != 0 {
		modNames = append(modNames, "Alt")
	}
	if hk.Mods&modShift != 0 {
		modNames = append(modNames, "Shift")
	}
	if hk.Mods&modWin != 0 {
		modNames = append(modNames, "Win")
	}
	hk.Text = strings.Join(append(modNames, keyName), "+")
	return hk, nil
}

func keyFromName(p string) (uint32, string, bool) {
	if len(p) == 1 {
		c := p[0]
		if c >= 'a' && c <= 'z' {
			return uint32(c - 'a' + 'A'), strings.ToUpper(p), true
		}
		if c >= '0' && c <= '9' {
			return uint32(c), p, true
		}
	}
	if strings.HasPrefix(p, "f") {
		if n, err := strconv.Atoi(p[1:]); err == nil && n >= 1 && n <= 24 {
			return uint32(0x70 + n - 1), "F" + strconv.Itoa(n), true
		}
	}
	for _, pre := range []string{"numpad", "num"} {
		if strings.HasPrefix(p, pre) {
			if n, err := strconv.Atoi(p[len(pre):]); err == nil && n >= 0 && n <= 9 {
				return uint32(0x60 + n), "Num" + strconv.Itoa(n), true
			}
		}
	}
	if k, ok := namedKeys[p]; ok {
		return k.vk, k.name, true
	}
	return 0, "", false
}
