# Audio Switcher

A tiny Windows tray app for switching your sound output (speakers, headphones, monitor, TV…) with a click or a keyboard shortcut.

- Lives in the system tray, with no window and no installer
- Click the icon to see all your output devices and pick one
- **Ctrl+Alt+F12** steps to the next device (you can change this)
- Optional hotkeys that jump straight to a specific device
- Middle-click the tray icon to step to the next device too
- Small notification showing which device you switched to
- Follows light/dark taskbar and dark context menus
- "Start with Windows" toggle in the menu
- Single ~2 MB `.exe` with no dependencies (Windows 10/11, 64-bit)

## Getting started

1. Put `AudioSwitcher.exe` somewhere permanent, e.g. `C:\Tools\AudioSwitcher\`.
2. Double-click it. A speaker icon appears in the tray (you may need to click the **^** arrow; drag the icon onto the taskbar to keep it visible).
3. Press **Ctrl+Alt+F12** to switch outputs, or click the icon to choose one.
4. Turn on **Start with Windows** from the tray menu if you want it to start with Windows.

> The exe isn't code-signed, so the first time you run it Windows SmartScreen may say *"Windows protected your PC"*. Click **More info → Run anyway**.

## Settings & hotkeys

Choose **Edit hotkeys & settings…** from the tray menu. This opens `%APPDATA%\AudioSwitcher\settings.json` in Notepad. Save the file and the changes apply within a few seconds; no restart is needed.

```json
{
  "cycle_hotkey": "Ctrl+Alt+F12",
  "cycle_devices": ["Speakers", "Headphones"],
  "device_hotkeys": {
    "Headphones": "Ctrl+Alt+1",
    "Speakers": "Ctrl+Alt+2"
  },
  "show_notifications": true,
  "set_communications_device": true
}
```

| Setting | What it does |
|---|---|
| `cycle_hotkey` | Steps to the next output. `""` turns it off. |
| `cycle_devices` | Which outputs the cycle steps through, in order. Use the device name or just part of it (`"Headphones"` matches `Headphones (Arctis Nova 7)`). Leave `[]` to cycle through all active devices. |
| `device_hotkeys` | A hotkey per device that jumps straight to it. |
| `show_notifications` | Show a notification when a hotkey switches the output. |
| `set_communications_device` | Also switch the default device for calls (Teams, Discord, Zoom…). Set to `false` if you keep a headset fixed for calls. |

**Hotkey format:** modifiers `Ctrl`, `Alt`, `Shift`, `Win` plus one key, joined with `+`. Keys can be `A`–`Z`, `0`–`9`, `F1`–`F24`, `Num0`–`Num9`, `Space`, `Insert`, `Delete`, `Home`, `End`, `PageUp`, `PageDown`, `Up`, `Down`, `Left`, `Right`, `Pause`, `ScrollLock`, `MediaNext`, `MediaPrev`, `MediaPlayPause`, and punctuation such as `-`, `=`, `[`, `]`, `;`, `'`, `,`, `.`, `/`, `` ` ``.

If a hotkey is already taken by another app, you'll get a notification. Pick a different combination.

The exact device names are listed in the tray menu. They're the same names shown in Windows **Sound settings**, where you can rename them under *Properties*.

## Tray menu

| | |
|---|---|
| Device list | Click one to make it the output; the current one has a dot. Its hotkey, if any, is shown on the right. |
| Switch to next device | Same as the cycle hotkey |
| Start with Windows | Adds or removes it from your startup apps (current user only) |
| Edit hotkeys & settings… | Opens `settings.json` |
| Windows sound settings… | Opens the classic Playback devices panel |
| Exit | Closes the app |

## Building from source

You only need [Go](https://go.dev/dl/) 1.22 or newer. There are no third-party modules. It builds on Windows, macOS or Linux:

```sh
GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-H windowsgui -s -w" -o AudioSwitcher.exe .
```

On Windows PowerShell:

```powershell
$env:GOOS="windows"; $env:GOARCH="amd64"
go build -trimpath -ldflags "-H windowsgui -s -w" -o AudioSwitcher.exe .
```

`rsrc_windows_amd64.syso` (exe icon, DPI manifest and version info) is committed and linked automatically. To regenerate it after changing `app.ico`, `app.manifest` or the version, run `./build.sh`, which uses `tools/mkres.py` (Python 3, standard library only).

### How it works

- Devices are listed through the Windows Core Audio **MMDevice API**.
- The default device is changed through **IPolicyConfig**. This is the same undocumented COM interface the Windows Sound panel and tools like SoundSwitch use.
- Hotkeys use `RegisterHotKey`, and the tray icon is plain `Shell_NotifyIcon`. Everything is pure Go calling Win32 directly.

## Files

| File | Purpose |
|---|---|
| `main.go` | Tray icon, menu, hotkeys, notifications, settings reload |
| `audio.go` | Listing devices and switching the default output |
| `hotkey.go` | Parses hotkey strings like `Ctrl+Alt+F12` |
| `config.go` | `settings.json` loading and defaults |
| `winapi.go` | Win32 bindings |
| `icons/` | Tray icons (light/dark taskbar) |
