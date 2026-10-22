package main

// Ventana nativa sin barra de título: el encabezado de la interfaz hace de barra
// (arrastrar, minimizar, maximizar, cerrar), como en la captura de referencia.

import (
	"sync"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

var (
	user32 = windows.NewLazySystemDLL("user32.dll")
	dwmapi = windows.NewLazySystemDLL("dwmapi.dll")

	procSetWindowLongPtrW     = user32.NewProc("SetWindowLongPtrW")
	procGetWindowLongPtrW     = user32.NewProc("GetWindowLongPtrW")
	procCallWindowProcW       = user32.NewProc("CallWindowProcW")
	procSetWindowPos          = user32.NewProc("SetWindowPos")
	procShowWindow            = user32.NewProc("ShowWindow")
	procIsZoomed              = user32.NewProc("IsZoomed")
	procReleaseCapture        = user32.NewProc("ReleaseCapture")
	procSendMessageW          = user32.NewProc("SendMessageW")
	procPostMessageW          = user32.NewProc("PostMessageW")
	procLoadImageW            = user32.NewProc("LoadImageW")
	procGetDpiForWindow       = user32.NewProc("GetDpiForWindow")
	procGetSystemMetricsDpi   = user32.NewProc("GetSystemMetricsForDpi")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procFindWindowW           = user32.NewProc("FindWindowW")
	procSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	procDwmSetWindowAttr      = dwmapi.NewProc("DwmSetWindowAttribute")
)

const (
	gwlpWndProc = ^uintptr(3) // GWLP_WNDPROC (-4)

	wmSetIcon       = 0x0080
	wmNCCalcSize    = 0x0083
	wmNCLButtonDown = 0x00A1
	wmClose         = 0x0010

	htCaption     = 2
	htTop         = 12
	htTopLeft     = 13
	htTopRight    = 14
	swMinimize    = 6
	swMaximize    = 3
	swRestore     = 9
	smCxFrame     = 32
	smCyFrame     = 33
	smCxPadBorder = 92

	swpFrameChanged = 0x0020
	swpNoMove       = 0x0002
	swpNoSize       = 0x0001
	swpNoZOrder     = 0x0004

	tituloVentana = "PdfEpub"

	// idIcono: go-winres guarda el icono del .exe como recurso número 1.
	idIcono = 1
)

type rect struct{ Left, Top, Right, Bottom int32 }

type ventana struct {
	hwnd     uintptr
	original uintptr
	once     sync.Once
}

var vent ventana

// prepararVentana quita la barra de título nativa, pone el icono del .exe y el modo oscuro.
func prepararVentana(hwnd uintptr) {
	vent.hwnd = hwnd
	vent.once.Do(func() {
		cb := syscall.NewCallback(procVentana)
		vent.original, _, _ = procSetWindowLongPtrW.Call(hwnd, gwlpWndProc, cb)
	})
	oscuro := int32(1)
	procDwmSetWindowAttr.Call(hwnd, 20, uintptr(unsafe.Pointer(&oscuro)), 4) // DWMWA_USE_IMMERSIVE_DARK_MODE
	esquinas := int32(2)
	procDwmSetWindowAttr.Call(hwnd, 33, uintptr(unsafe.Pointer(&esquinas)), 4) // DWMWA_WINDOW_CORNER_PREFERENCE: redondas
	ponerIcono(hwnd)
	// Recalcula el marco para aplicar el área de cliente sin barra de título.
	procSetWindowPos.Call(hwnd, 0, 0, 0, 0, 0, swpFrameChanged|swpNoMove|swpNoSize|swpNoZOrder)
}

// procVentana extiende el área de cliente hasta arriba: sin barra de título, pero con los
// bordes invisibles de Windows 11 para redimensionar por los costados y abajo.
func procVentana(hwnd, msg, wp, lp uintptr) uintptr {
	if msg == wmNCCalcSize && wp != 0 {
		// lp apunta a NCCALCSIZE_PARAMS, cuyo primer campo es el rectángulo propuesto.
		r := (*rect)(*(*unsafe.Pointer)(unsafe.Pointer(&lp)))
		arriba := r.Top
		res, _, _ := procCallWindowProcW.Call(vent.original, hwnd, msg, wp, lp)
		r.Top = arriba
		if z, _, _ := procIsZoomed.Call(hwnd); z != 0 {
			// Maximizada, Windows la agranda más allá de la pantalla: compensamos el borde.
			dpi, _, _ := procGetDpiForWindow.Call(hwnd)
			marco, _, _ := procGetSystemMetricsDpi.Call(smCyFrame, dpi)
			relleno, _, _ := procGetSystemMetricsDpi.Call(smCxPadBorder, dpi)
			r.Top += int32(marco + relleno)
		}
		return res
	}
	res, _, _ := procCallWindowProcW.Call(vent.original, hwnd, msg, wp, lp)
	return res
}

func ponerIcono(hwnd uintptr) {
	inst, _, _ := windows.NewLazySystemDLL("kernel32.dll").NewProc("GetModuleHandleW").Call(0)
	dpi, _, _ := procGetDpiForWindow.Call(hwnd)
	if dpi == 0 {
		dpi = 96
	}
	for _, t := range []struct{ tipo, lado uintptr }{{0, 16}, {1, 32}} { // ICON_SMALL, ICON_BIG
		lado := t.lado * dpi / 96
		ico, _, _ := procLoadImageW.Call(inst, idIcono, 1, lado, lado, 0) // IMAGE_ICON
		if ico != 0 {
			procSendMessageW.Call(hwnd, wmSetIcon, t.tipo, ico)
		}
	}
}

// tamanoInicial: una parte del área útil de la pantalla principal (en píxeles reales).
func tamanoInicial() (uint, uint) {
	var area rect
	procSystemParametersInfoW.Call(0x0030, 0, uintptr(unsafe.Pointer(&area)), 0) // SPI_GETWORKAREA
	ancho, alto := uint(area.Right-area.Left), uint(area.Bottom-area.Top)
	if ancho == 0 || alto == 0 {
		return 1440, 900
	}
	// El editor aprovecha la pantalla: 78 % del ancho y 88 % del alto.
	return ancho * 78 / 100, alto * 88 / 100
}

func mensaje(texto string) {
	t, _ := windows.UTF16PtrFromString(texto)
	c, _ := windows.UTF16PtrFromString(tituloVentana)
	windows.MessageBox(0, t, c, windows.MB_OK|windows.MB_ICONWARNING)
}

// ---- Acciones que llama la interfaz (enlazadas con webview.Bind) ----

func ventanaArrastrar() {
	procReleaseCapture.Call()
	procSendMessageW.Call(vent.hwnd, wmNCLButtonDown, htCaption, 0)
}

func ventanaRedimensionar(borde string) {
	ht := map[string]uintptr{"arriba": htTop, "arriba-izq": htTopLeft, "arriba-der": htTopRight}[borde]
	if ht == 0 {
		return
	}
	procReleaseCapture.Call()
	procSendMessageW.Call(vent.hwnd, wmNCLButtonDown, ht, 0)
}

func ventanaMinimizar() { procShowWindow.Call(vent.hwnd, swMinimize) }

func ventanaMaximizar() bool {
	if z, _, _ := procIsZoomed.Call(vent.hwnd); z != 0 {
		procShowWindow.Call(vent.hwnd, swRestore)
		return false
	}
	procShowWindow.Call(vent.hwnd, swMaximize)
	return true
}

func ventanaMaximizada() bool {
	z, _, _ := procIsZoomed.Call(vent.hwnd)
	return z != 0
}

func ventanaCerrar() { procPostMessageW.Call(vent.hwnd, wmClose, 0, 0) }

// traerAlFrente muestra la ventana de una instancia ya abierta. Devuelve false si no hay.
func traerAlFrente() bool {
	clase, _ := windows.UTF16PtrFromString("webview")
	titulo, _ := windows.UTF16PtrFromString(tituloVentana)
	h, _, _ := procFindWindowW.Call(uintptr(unsafe.Pointer(clase)), uintptr(unsafe.Pointer(titulo)))
	if h == 0 {
		return false
	}
	procShowWindow.Call(h, swRestore)
	procSetForegroundWindow.Call(h)
	return true
}
