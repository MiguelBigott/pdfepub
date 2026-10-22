//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"
)

const crearSinVentana = 0x08000000 // CREATE_NO_WINDOW

func ocultar(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: crearSinVentana}
}

func carpetaDatos() string {
	if d := os.Getenv("LOCALAPPDATA"); d != "" {
		return filepath.Join(d, "PdfEpub")
	}
	d, _ := os.UserConfigDir()
	return filepath.Join(d, "PdfEpub")
}

// carpetaSalidaPorDefecto devuelve "PdfEpub" dentro de la carpeta Documentos
// del usuario (respeta si la movió a otro disco).
func carpetaSalidaPorDefecto() string {
	if v := carpetaDocumentos(); v != "" {
		return filepath.Join(v, "PdfEpub")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "Documents", "PdfEpub")
}

// FOLDERID_Documents {FDD39AD0-238F-46AF-ADB4-6C85480369C7}
var folderIDDocumentos = syscall.GUID{Data1: 0xFDD39AD0, Data2: 0x238F, Data3: 0x46AF,
	Data4: [8]byte{0xAD, 0xB4, 0x6C, 0x85, 0x48, 0x03, 0x69, 0xC7}}

func carpetaDocumentos() string {
	var p *uint16
	r, _, _ := syscall.NewLazyDLL("shell32.dll").NewProc("SHGetKnownFolderPath").Call(
		uintptr(unsafe.Pointer(&folderIDDocumentos)), 0, 0, uintptr(unsafe.Pointer(&p)))
	if p == nil {
		return ""
	}
	defer syscall.NewLazyDLL("ole32.dll").NewProc("CoTaskMemFree").Call(uintptr(unsafe.Pointer(p)))
	if r != 0 {
		return ""
	}
	return syscall.UTF16ToString(unsafe.Slice(p, 32768))
}

func powershell(script string) (string, error) {
	cmd := exec.Command("powershell", "-NoProfile", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script)
	ocultar(cmd)
	out, err := cmd.Output()
	return strings.TrimSpace(string(out)), err
}

func comillasPS(s string) string { return "'" + strings.ReplaceAll(s, "'", "''") + "'" }

func elegirCarpeta(titulo, inicial string) (string, error) {
	script := `[Console]::OutputEncoding=[Text.Encoding]::UTF8
Add-Type -AssemblyName System.Windows.Forms
[System.Windows.Forms.Application]::EnableVisualStyles()
$f = New-Object System.Windows.Forms.Form -Property @{TopMost=$true; ShowInTaskbar=$false; Width=0; Height=0; StartPosition='CenterScreen'}
$d = New-Object System.Windows.Forms.FolderBrowserDialog
$d.Description = ` + comillasPS(titulo) + `
$d.ShowNewFolderButton = $true
$d.SelectedPath = ` + comillasPS(inicial) + `
if ($d.ShowDialog($f) -eq 'OK') { $d.SelectedPath }`
	return powershell(script)
}

func abrirCarpeta(ruta string) {
	_ = exec.Command("explorer.exe", ruta).Start()
}

func mostrarArchivo(ruta string) {
	c := exec.Command("explorer.exe")
	c.SysProcAttr = &syscall.SysProcAttr{CmdLine: `explorer.exe /select,"` + ruta + `"`}
	_ = c.Start()
}
