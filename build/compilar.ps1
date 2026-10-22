# Compila PdfEpub.exe y el instalador en dist\.
# Requiere: Go, NSIS (makensis) y go-winres (go install github.com/tc-hib/go-winres@latest).
# El logo y el icono ya están en git; para cambiarlos: python build\generar_logo.py
param([switch]$SoloExe)

$ErrorActionPreference = 'Stop'
$ProgressPreference = 'SilentlyContinue'
$raiz = Split-Path $PSScriptRoot -Parent
Set-Location $raiz

$version = (Select-String -Path main.go -Pattern 'const Version = "(.+)"').Matches[0].Groups[1].Value
$goExe = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $goExe) { $goExe = 'C:\Program Files\Go\bin\go.exe' }
$winres = Join-Path (& $goExe env GOPATH) 'bin\go-winres.exe'
$makensis = 'C:\Program Files (x86)\NSIS\makensis.exe'

Write-Host "PdfEpub $version"

# ---------- Ejecutable ----------
# Icono, manifiesto y datos de versión dentro del .exe
& $winres simply --arch amd64 --out rsrc --manifest gui `
    --icon build\app.ico `
    --product-name PdfEpub --file-description 'PdfEpub - convierte libros PDF a EPUB' `
    --product-version $version --file-version $version `
    --original-filename PdfEpub.exe --copyright 'Copyright (c) 2026 Miguel Bigott'
if ($LASTEXITCODE) { throw 'go-winres falló' }

New-Item -ItemType Directory -Force dist | Out-Null
& $goExe build -trimpath -ldflags '-H windowsgui -s -w' -o dist\PdfEpub.exe .
if ($LASTEXITCODE) { throw 'go build falló' }

# ---------- Licencias ----------
# LICENSE.txt es la licencia MIT de la app; las de terceros (pdf.js, Go, sus módulos y las
# tipografías) van en THIRD-PARTY-NOTICES.md y THIRD-PARTY-LICENSES.txt, junto a la app.
Copy-Item LICENSE (Join-Path $raiz 'dist\LICENSE.txt')
Copy-Item THIRD-PARTY-NOTICES.md, THIRD-PARTY-LICENSES.txt dist -Force

if ($SoloExe) {
    Write-Host "Ejecutable: $raiz\dist\PdfEpub.exe"
    return
}

# ---------- Instalador ----------
# El .nsi está en UTF-8 sin BOM: sin /INPUTCHARSET, makensis lo lee como ANSI y rompe las tildes.
& $makensis /V2 /INPUTCHARSET UTF8 "/DVERSION=$version" build\instalador.nsi
if ($LASTEXITCODE) { throw 'makensis falló' }

Get-ChildItem dist -File | Format-Table Name, @{n = 'MB'; e = { [math]::Round($_.Length / 1MB, 2) } }
