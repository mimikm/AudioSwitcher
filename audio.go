package main

// Audio endpoint enumeration (MMDevice API) and default-device switching
// (IPolicyConfig, the same undocumented interface the Windows Sound panel uses).

import (
	"fmt"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"unsafe"
)

type Device struct {
	ID   string
	Name string
}

var (
	clsidMMDeviceEnumerator = syscall.GUID{Data1: 0xBCDE0395, Data2: 0xE52F, Data3: 0x467C, Data4: [8]byte{0x8E, 0x3D, 0xC4, 0x57, 0x92, 0x91, 0x69, 0x2E}}
	iidIMMDeviceEnumerator  = syscall.GUID{Data1: 0xA95664D2, Data2: 0x9614, Data3: 0x4F35, Data4: [8]byte{0xA7, 0x46, 0xDE, 0x8D, 0xB6, 0x36, 0x17, 0xE6}}
	clsidPolicyConfigClient = syscall.GUID{Data1: 0x870AF99C, Data2: 0x171D, Data3: 0x4F9E, Data4: [8]byte{0xAF, 0x0D, 0xE6, 0x3D, 0xF4, 0x0C, 0x2B, 0xC9}}
	iidIPolicyConfig        = syscall.GUID{Data1: 0xF8679F50, Data2: 0x850A, Data3: 0x41CF, Data4: [8]byte{0x9C, 0x72, 0x43, 0x0F, 0x29, 0x02, 0x90, 0xC8}}
	iidIPolicyConfigVista   = syscall.GUID{Data1: 0x568B9108, Data2: 0x44BF, Data3: 0x40B4, Data4: [8]byte{0x90, 0x06, 0x86, 0xAF, 0xE5, 0xB5, 0xA6, 0x20}}

	pkeyDeviceFriendlyName = propertyKey{
		FmtID: syscall.GUID{Data1: 0xA45C254E, Data2: 0xDF1C, Data3: 0x4EFD, Data4: [8]byte{0x80, 0x20, 0x67, 0xD1, 0x46, 0xA8, 0x50, 0xE0}},
		PID:   14,
	}
)

const (
	clsctxAll         = 0x17
	eRender           = 0
	eConsole          = 0
	eMultimedia       = 1
	eCommunications   = 2
	deviceStateActive = 1
	stgmRead          = 0
	vtLPWSTR          = 31
)

type propertyKey struct {
	FmtID syscall.GUID
	PID   uint32
}

// PROPVARIANT is 24 bytes on 64-bit Windows; we only read the LPWSTR case.
type propVariant struct {
	VT  uint16
	_   [3]uint16
	Val uintptr
	_   uintptr
}

// comCall invokes method #idx of a COM object's vtable.
func comCall(obj uintptr, idx int, args ...uintptr) int32 {
	vtbl := *(*unsafe.Pointer)(unsafe.Pointer(obj))
	fn := *(*uintptr)(unsafe.Add(vtbl, idx*int(unsafe.Sizeof(uintptr(0)))))
	r, _, _ := syscall.SyscallN(fn, append([]uintptr{obj}, args...)...)
	return int32(r)
}

func release(obj uintptr) {
	if obj != 0 {
		comCall(obj, 2)
	}
}

func coCreate(clsid, iid *syscall.GUID) (uintptr, int32) {
	var obj uintptr
	r, _, _ := pCoCreateInstance.Call(uintptr(unsafe.Pointer(clsid)), 0, clsctxAll,
		uintptr(unsafe.Pointer(iid)), uintptr(unsafe.Pointer(&obj)))
	return obj, int32(r)
}

func hrErr(what string, hr int32) error {
	return fmt.Errorf("%s failed (0x%08X)", what, uint32(hr))
}

func newEnumerator() (uintptr, error) {
	en, hr := coCreate(&clsidMMDeviceEnumerator, &iidIMMDeviceEnumerator)
	if hr < 0 {
		return 0, hrErr("Creating the device enumerator", hr)
	}
	return en, nil
}

// ListOutputs returns the active playback devices, sorted by name.
func ListOutputs() ([]Device, error) {
	en, err := newEnumerator()
	if err != nil {
		return nil, err
	}
	defer release(en)

	var coll uintptr
	// IMMDeviceEnumerator::EnumAudioEndpoints
	if hr := comCall(en, 3, eRender, deviceStateActive, uintptr(unsafe.Pointer(&coll))); hr < 0 {
		return nil, hrErr("Listing audio devices", hr)
	}
	defer release(coll)

	var count uint32
	comCall(coll, 3, uintptr(unsafe.Pointer(&count))) // IMMDeviceCollection::GetCount

	devs := make([]Device, 0, count)
	for i := uint32(0); i < count; i++ {
		var dev uintptr
		if comCall(coll, 4, uintptr(i), uintptr(unsafe.Pointer(&dev))) < 0 { // ::Item
			continue
		}
		d := Device{ID: deviceID(dev), Name: deviceName(dev)}
		release(dev)
		if d.ID != "" {
			devs = append(devs, d)
		}
	}
	sort.SliceStable(devs, func(i, j int) bool {
		return strings.ToLower(devs[i].Name) < strings.ToLower(devs[j].Name)
	})
	return devs, nil
}

func deviceID(dev uintptr) string {
	var p *uint16
	if comCall(dev, 5, uintptr(unsafe.Pointer(&p))) < 0 || p == nil { // IMMDevice::GetId
		return ""
	}
	defer pCoTaskMemFree.Call(uintptr(unsafe.Pointer(p)))
	return ptrToString(p)
}

func deviceName(dev uintptr) string {
	var store uintptr
	if comCall(dev, 4, stgmRead, uintptr(unsafe.Pointer(&store))) < 0 { // IMMDevice::OpenPropertyStore
		return "Unknown device"
	}
	defer release(store)

	var pv propVariant
	// IPropertyStore::GetValue
	if comCall(store, 5, uintptr(unsafe.Pointer(&pkeyDeviceFriendlyName)), uintptr(unsafe.Pointer(&pv))) < 0 {
		return "Unknown device"
	}
	defer pPropVariantClear.Call(uintptr(unsafe.Pointer(&pv)))
	if pv.VT != vtLPWSTR || pv.Val == 0 {
		return "Unknown device"
	}
	return ptrToString((*uint16)(unsafe.Pointer(pv.Val)))
}

// DefaultOutputID returns the ID of the current default playback device ("" if none).
func DefaultOutputID() string {
	en, err := newEnumerator()
	if err != nil {
		return ""
	}
	defer release(en)
	var dev uintptr
	// IMMDeviceEnumerator::GetDefaultAudioEndpoint
	if comCall(en, 4, eRender, eConsole, uintptr(unsafe.Pointer(&dev))) < 0 {
		return ""
	}
	defer release(dev)
	return deviceID(dev)
}

// SetDefaultOutput makes the device the default for apps and media, and
// optionally for calls (the "default communications device") too.
func SetDefaultOutput(id string, includeCommunications bool) error {
	pc, hr := coCreate(&clsidPolicyConfigClient, &iidIPolicyConfig)
	method := 13 // IPolicyConfig::SetDefaultEndpoint (Windows 7 and later)
	if hr < 0 {
		pc, hr = coCreate(&clsidPolicyConfigClient, &iidIPolicyConfigVista)
		method = 12
	}
	if hr < 0 {
		return hrErr("Opening Windows audio policy", hr)
	}
	defer release(pc)

	wid := utf16p(id)
	roles := []uintptr{eConsole, eMultimedia}
	if includeCommunications {
		roles = append(roles, eCommunications)
	}
	for _, role := range roles {
		if hr := comCall(pc, method, uintptr(unsafe.Pointer(wid)), role); hr < 0 {
			return hrErr("Switching the output device", hr)
		}
	}
	runtime.KeepAlive(wid)
	return nil
}

// findDevice matches a name from settings: exact (case-insensitive) first,
// otherwise the first device whose name contains it.
func findDevice(devs []Device, pattern string) (Device, bool) {
	p := strings.TrimSpace(pattern)
	if p == "" {
		return Device{}, false
	}
	for _, d := range devs {
		if strings.EqualFold(d.Name, p) {
			return d, true
		}
	}
	lp := strings.ToLower(p)
	for _, d := range devs {
		if strings.Contains(strings.ToLower(d.Name), lp) {
			return d, true
		}
	}
	return Device{}, false
}
