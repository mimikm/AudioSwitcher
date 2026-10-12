package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"time"
)

type Config struct {
	Help                    []string          `json:"_help,omitempty"`
	CycleHotkey             string            `json:"cycle_hotkey"`
	CycleDevices            []string          `json:"cycle_devices"`
	DeviceHotkeys           map[string]string `json:"device_hotkeys"`
	ShowNotifications       bool              `json:"show_notifications"`
	SetCommunicationsDevice bool              `json:"set_communications_device"`
}

func defaultConfig() Config {
	return Config{
		Help: []string{
			"Hotkeys look like Ctrl+Alt+F12, Ctrl+Shift+1 or Win+Alt+A. Keys: A-Z, 0-9, F1-F24, Num0-Num9, Space, Insert, Delete, Home, End, PageUp, PageDown, Up, Down, Left, Right, Pause, MediaNext, MediaPrev, MediaPlayPause.",
			"cycle_hotkey: steps to the next output device. Set it to \"\" to turn it off.",
			"cycle_devices: names (or part of a name) of the outputs to step through, in order, e.g. [\"Speakers\", \"Headphones\"]. Leave it empty to step through all of them.",
			"device_hotkeys: a hotkey that jumps straight to one device, e.g. {\"Headphones\": \"Ctrl+Alt+1\", \"Speakers\": \"Ctrl+Alt+2\"}.",
			"set_communications_device: also make the device the default for calls (Teams, Discord, etc.).",
			"Changes are picked up automatically a few seconds after you save this file.",
		},
		CycleHotkey:             "Ctrl+Alt+F12",
		CycleDevices:            []string{},
		DeviceHotkeys:           map[string]string{},
		ShowNotifications:       true,
		SetCommunicationsDevice: true,
	}
}

func configPath() string {
	dir := os.Getenv("APPDATA")
	if dir == "" {
		dir = filepath.Dir(exePath())
	}
	return filepath.Join(dir, "AudioSwitcher", "settings.json")
}

// loadConfig reads settings, creating the file with defaults on first run.
func loadConfig(path string) (cfg Config, created bool, mod time.Time, err error) {
	cfg = defaultConfig()
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return cfg, false, mod, err
		}
		out, _ := json.MarshalIndent(cfg, "", "  ")
		if err = os.WriteFile(path, out, 0o644); err != nil {
			return cfg, false, mod, err
		}
		if st, e := os.Stat(path); e == nil {
			mod = st.ModTime()
		}
		return cfg, true, mod, nil
	}
	if err != nil {
		return cfg, false, mod, err
	}
	if st, e := os.Stat(path); e == nil {
		mod = st.ModTime()
	}
	if err = json.Unmarshal(data, &cfg); err != nil {
		return defaultConfig(), false, mod, err
	}
	if cfg.DeviceHotkeys == nil {
		cfg.DeviceHotkeys = map[string]string{}
	}
	return cfg, false, mod, nil
}

// sortedDeviceHotkeys gives a stable order so hotkey IDs don't shuffle.
func (c Config) sortedDeviceHotkeys() []string {
	keys := make([]string, 0, len(c.DeviceHotkeys))
	for k := range c.DeviceHotkeys {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
