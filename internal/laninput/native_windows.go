//go:build windows

package laninput

import (
	"fmt"
	"log"
	"sync"
	"syscall"
	"unsafe"

	"github.com/scribble-rs/scribble.rs/internal/game"
	"golang.org/x/sys/windows"
)

const (
	hidUsagePageGeneric = 0x01
	hidUsageKeyboard    = 0x06

	ridInput       = 0x10000003
	ridevInputSink = 0x00000100
	ridiDeviceName = 0x20000007

	rimTypeKeyboard = 1

	wmInput      = 0x00FF
	wmKeyDown    = 0x0100
	wmKeyUp      = 0x0101
	wmSysKeyDown = 0x0104
	wmSysKeyUp   = 0x0105

	vkBack = 0x08
	vkTab  = 0x09
	vkRet  = 0x0D
	vkEsc  = 0x1B
	vkSpc  = 0x20
)

var (
	user32                     = windows.NewLazySystemDLL("user32.dll")
	kernel32                   = windows.NewLazySystemDLL("kernel32.dll")
	procRegisterClassExW       = user32.NewProc("RegisterClassExW")
	procCreateWindowExW        = user32.NewProc("CreateWindowExW")
	procDefWindowProcW         = user32.NewProc("DefWindowProcW")
	procGetMessageW            = user32.NewProc("GetMessageW")
	procTranslateMessage       = user32.NewProc("TranslateMessage")
	procDispatchMessageW       = user32.NewProc("DispatchMessageW")
	procRegisterRawInputDevice = user32.NewProc("RegisterRawInputDevices")
	procGetRawInputData        = user32.NewProc("GetRawInputData")
	procGetRawInputDeviceInfoW = user32.NewProc("GetRawInputDeviceInfoW")
	procGetKeyboardState       = user32.NewProc("GetKeyboardState")
	procToUnicode              = user32.NewProc("ToUnicode")
	procGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")

	activeHandler   func(game.LanKeyboardInput)
	listOnlyMode    bool
	deviceNameCache sync.Map
)

type rawInputDevice struct {
	usagePage uint16
	usage     uint16
	flags     uint32
	hwnd      windows.Handle
}

type rawInputHeader struct {
	dwType  uint32
	dwSize  uint32
	hDevice windows.Handle
	wParam  uintptr
}

type rawKeyboard struct {
	makeCode         uint16
	flags            uint16
	reserved         uint16
	vKey             uint16
	message          uint32
	extraInformation uint32
}

type rawInputKeyboard struct {
	header   rawInputHeader
	keyboard rawKeyboard
}

type wndClassEx struct {
	size       uint32
	style      uint32
	wndProc    uintptr
	clsExtra   int32
	wndExtra   int32
	instance   windows.Handle
	icon       windows.Handle
	cursor     windows.Handle
	background windows.Handle
	menuName   *uint16
	className  *uint16
	iconSm     windows.Handle
}

type point struct {
	x int32
	y int32
}

type msg struct {
	hwnd    windows.Handle
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

func captureNative(handleInput func(game.LanKeyboardInput), listOnly bool) error {
	activeHandler = handleInput
	listOnlyMode = listOnly

	window, err := createRawInputWindow()
	if err != nil {
		return err
	}
	if err := registerKeyboardRawInput(window); err != nil {
		return err
	}

	log.Println("capturing Windows Raw Input keyboard events")
	log.Println("keyboard IDs are Raw Input device names; use them in LAN assignment")
	return messageLoop()
}

func createRawInputWindow() (windows.Handle, error) {
	className, err := windows.UTF16PtrFromString("ScribbleRsLanInput")
	if err != nil {
		return 0, err
	}
	instance, _, err := procGetModuleHandleW.Call(0)
	if instance == 0 {
		return 0, fmt.Errorf("GetModuleHandleW failed: %w", err)
	}

	class := wndClassEx{
		size:      uint32(unsafe.Sizeof(wndClassEx{})),
		wndProc:   windows.NewCallback(windowProc),
		instance:  windows.Handle(instance),
		className: className,
	}
	if atom, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&class))); atom == 0 {
		return 0, fmt.Errorf("RegisterClassExW failed: %w", err)
	}

	hwnd, _, err := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(className)),
		0,
		0, 0, 0, 0,
		0,
		0,
		instance,
		0,
	)
	if hwnd == 0 {
		return 0, fmt.Errorf("CreateWindowExW failed: %w", err)
	}
	return windows.Handle(hwnd), nil
}

func registerKeyboardRawInput(hwnd windows.Handle) error {
	device := rawInputDevice{
		usagePage: hidUsagePageGeneric,
		usage:     hidUsageKeyboard,
		flags:     ridevInputSink,
		hwnd:      hwnd,
	}
	ok, _, err := procRegisterRawInputDevice.Call(
		uintptr(unsafe.Pointer(&device)),
		1,
		unsafe.Sizeof(device),
	)
	if ok == 0 {
		return fmt.Errorf("RegisterRawInputDevices failed: %w", err)
	}
	return nil
}

func messageLoop() error {
	var message msg
	for {
		ret, _, err := procGetMessageW.Call(uintptr(unsafe.Pointer(&message)), 0, 0, 0)
		switch int32(ret) {
		case -1:
			return fmt.Errorf("GetMessageW failed: %w", err)
		case 0:
			return nil
		default:
			procTranslateMessage.Call(uintptr(unsafe.Pointer(&message)))
			procDispatchMessageW.Call(uintptr(unsafe.Pointer(&message)))
		}
	}
}

func windowProc(hwnd windows.Handle, message uint32, wParam, lParam uintptr) uintptr {
	if message == wmInput {
		if err := handleRawInput(lParam); err != nil {
			log.Printf("raw input error: %v", err)
		}
	}
	ret, _, _ := procDefWindowProcW.Call(uintptr(hwnd), uintptr(message), wParam, lParam)
	return ret
}

func handleRawInput(rawInputHandle uintptr) error {
	var size uint32
	if ret, _, err := procGetRawInputData.Call(rawInputHandle, ridInput, 0, uintptr(unsafe.Pointer(&size)), unsafe.Sizeof(rawInputHeader{})); ret == ^uintptr(0) {
		return fmt.Errorf("GetRawInputData size failed: %w", err)
	}
	if size < uint32(unsafe.Sizeof(rawInputKeyboard{})) {
		return nil
	}

	buffer := make([]byte, size)
	if ret, _, err := procGetRawInputData.Call(rawInputHandle, ridInput, uintptr(unsafe.Pointer(&buffer[0])), uintptr(unsafe.Pointer(&size)), unsafe.Sizeof(rawInputHeader{})); ret == ^uintptr(0) {
		return fmt.Errorf("GetRawInputData failed: %w", err)
	}

	input := (*rawInputKeyboard)(unsafe.Pointer(&buffer[0]))
	if input.header.dwType != rimTypeKeyboard {
		return nil
	}

	action := rawInputAction(input.keyboard.message)
	if action == "" {
		return nil
	}

	deviceName, err := rawInputDeviceName(input.header.hDevice)
	if err != nil {
		return err
	}
	key := keyFromRawKeyboard(input.keyboard)
	if key == "" {
		return nil
	}

	if listOnlyMode {
		log.Printf("%s key=%s action=%s", deviceName, key, action)
		return nil
	}
	if activeHandler != nil {
		activeHandler(game.LanKeyboardInput{
			KeyboardID: deviceName,
			Key:        key,
			Action:     action,
		})
	}
	return nil
}

func rawInputAction(message uint32) string {
	switch message {
	case wmKeyDown, wmSysKeyDown:
		return "keydown"
	case wmKeyUp, wmSysKeyUp:
		return "keyup"
	default:
		return ""
	}
}

func rawInputDeviceName(device windows.Handle) (string, error) {
	if cached, ok := deviceNameCache.Load(device); ok {
		return cached.(string), nil
	}
	var size uint32
	procGetRawInputDeviceInfoW.Call(uintptr(device), ridiDeviceName, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 {
		name := fmt.Sprintf("raw:%x", uintptr(device))
		deviceNameCache.Store(device, name)
		return name, nil
	}
	buffer := make([]uint16, size)
	ret, _, err := procGetRawInputDeviceInfoW.Call(
		uintptr(device),
		ridiDeviceName,
		uintptr(unsafe.Pointer(&buffer[0])),
		uintptr(unsafe.Pointer(&size)),
	)
	if ret == ^uintptr(0) {
		return "", fmt.Errorf("GetRawInputDeviceInfoW failed: %w", err)
	}
	name := syscall.UTF16ToString(buffer)
	deviceNameCache.Store(device, name)
	return name, nil
}

func keyFromRawKeyboard(keyboard rawKeyboard) string {
	switch keyboard.vKey {
	case vkBack:
		return "Backspace"
	case vkRet:
		return "Enter"
	case vkSpc:
		return " "
	case vkTab, vkEsc:
		return ""
	}

	var keyboardState [256]byte
	if ret, _, _ := procGetKeyboardState.Call(uintptr(unsafe.Pointer(&keyboardState[0]))); ret == 0 {
		return fallbackKeyName(keyboard.vKey)
	}

	var chars [8]uint16
	ret, _, _ := procToUnicode.Call(
		uintptr(keyboard.vKey),
		uintptr(keyboard.makeCode),
		uintptr(unsafe.Pointer(&keyboardState[0])),
		uintptr(unsafe.Pointer(&chars[0])),
		uintptr(len(chars)),
		0,
	)
	if charCount := int32(ret); charCount > 0 {
		return syscall.UTF16ToString(chars[:int(charCount)])
	}

	return fallbackKeyName(keyboard.vKey)
}

func fallbackKeyName(vKey uint16) string {
	if vKey >= 'A' && vKey <= 'Z' {
		return string(rune(vKey + 32))
	}
	if vKey >= '0' && vKey <= '9' {
		return string(rune(vKey))
	}
	return ""
}
