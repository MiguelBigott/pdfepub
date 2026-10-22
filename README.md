<div align="center">

<img src="build/logo.png" alt="Logo de PdfEpub" width="88">

# PdfEpub

> Convierte libros **PDF a EPUB** en tu PC: rearma los párrafos, los títulos y los capítulos, y te deja diseñar la portada y completar los datos del libro antes de crearlo.

[![Versión](https://img.shields.io/badge/versión-v1.1.0-8a6fd1)](CHANGELOG.md)
[![Go](https://img.shields.io/badge/Go-1.27+-00ADD8?logo=go&logoColor=white)](https://go.dev)
[![Windows](https://img.shields.io/badge/Windows-10%20·%2011-0078D4)](#-instalación)
[![Licencia](https://img.shields.io/badge/licencia-MIT-3f9d6e)](LICENSE)
[![Lector](https://img.shields.io/badge/lector-pdf.js%206.4-c9a45c)](https://mozilla.github.io/pdf.js/)
[![Codex](https://img.shields.io/badge/suite-Codex-c9a45c)](https://github.com/MiguelBigott/codex)

[Historial de versiones](CHANGELOG.md) · [Releases](../../releases) · [Descargar el instalador](../../releases/latest)

<img src="https://github.com/MiguelBigott/pdfepub/raw/estadisticas/estadisticas.svg" alt="Actividad del repositorio en los últimos 30 días" width="100%">

<sub>Panel de actividad generado a diario por <a href=".github/workflows/estadisticas.yml">GitHub Actions</a>, sin servicios de terceros.</sub>

</div>

---

## 📖 Índice

- [Funciones](#-funciones)
- [Stack](#-stack)
- [Instalación](#-instalación)
- [Compilar](#️-compilar)
- [Seguridad y privacidad](#-seguridad-y-privacidad)
- [Licencia](#️-licencia)

## ✨ Funciones

- **Dos formas de convertir**:
  - **Texto fluido**: el texto se adapta a la pantalla y a la letra del lector. Une las líneas en párrafos (también los que siguen en otra página), quita los encabezados y los números de página y respeta las cursivas y las negritas. Ideal para novelas, ensayos y apuntes.
  - **Páginas fijas**: cada página queda idéntica al PDF. Ideal para cómics, revistas, libros ilustrados y escaneados.
- **Capítulos en un pliego**: los capítulos se revisan en un libro abierto, con la página original del PDF a la izquierda y cómo se leerá a la derecha. Usa el índice del PDF o sus títulos, y puedes renombrar, unir, excluir o dividir capítulos desde la vista previa.
- **Portada**: una página del PDF, una imagen propia o un color, con encuadre, zoom y oscurecido. Encima, el título y el autor con una de cuatro plantillas (clásica, moderna, banda o marco) y cuatro tipografías.
- **Datos del libro**: título, autores, idioma, fecha, editorial, ISBN, serie y número, etiquetas y descripción. Los que trae el PDF se completan solos.
- **PDF con contraseña**: los abre si escribes la clave.
- **EPUB 3** con su índice (también para lectores viejos), la serie que entienden Calibre y la mayoría de los lectores, y la portada como la ponen las editoriales.
- **Ventana propia**: ventana nativa de Windows (WebView2), parte de [Codex](https://github.com/MiguelBigott/codex), que la instala, la actualiza y la abre.

Los PDF escaneados no traen texto: PdfEpub los ofrece como páginas fijas (no hace OCR). En texto fluido el diseño del PDF se reconstruye, así que algún párrafo o capítulo puede necesitar un ajuste en la vista previa.

## 🧱 Stack

| Capa | Tecnología |
|---|---|
| Aplicación | [Go](https://go.dev): servidor local con la biblioteca estándar; rearma el texto y escribe el EPUB |
| Lectura del PDF | [pdf.js](https://mozilla.github.io/pdf.js/) 6.4 de Mozilla, dentro de la interfaz, con los scripts de los PDF desactivados |
| Ventana | [WebView2](https://developer.microsoft.com/microsoft-edge/webview2/) con [go-webview2](https://github.com/jchv/go-webview2) |
| Interfaz | HTML, CSS y JavaScript embebidos (`go:embed`), skin «Pliego» con Inter y Space Mono |
| Instalador | [NSIS](https://nsis.sourceforge.io) |

## 📦 Instalación

Descarga `PdfEpub-Setup-<versión>.exe` desde [Releases](../../releases/latest), o instálalo desde [Codex](https://github.com/MiguelBigott/codex).

- Se instala en `C:\Program Files\PdfEpub` (desde Codex, en `C:\Program Files\Codex\PdfEpub`).
- Los libros van por defecto a `Documentos\PdfEpub`; se puede cambiar desde la app.
- La configuración se guarda en `%LOCALAPPDATA%\PdfEpub`.
- Al desinstalar se borran el programa y la configuración; **tus libros no se tocan**.

## 🛠️ Compilar

Requisitos: [Go](https://go.dev/dl) 1.27.3 o posterior, [NSIS](https://nsis.sourceforge.io) y go-winres
(`go install github.com/tc-hib/go-winres@latest`).

```powershell
.\build\compilar.ps1
```

Genera `dist/PdfEpub.exe` y `dist/PdfEpub-Setup-<versión>.exe`. Con `-SoloExe` compila solo el ejecutable.

El logo «Cosido» (tres hojas de pergamino cosidas al lomo con puntadas de oro, en el estilo del logo de Codex) se define en `build/generar_logo.py`, que escribe `web/logo.svg`, `build/logo.png`, `build/app.ico` (con una versión reforzada para 16-32 px) y `build/lateral.bmp`:

```powershell
python build\generar_logo.py
```

Para revisar la interfaz en un navegador, sin ventana, ejecuta la app con `PDFEPUB_SIN_VENTANA=1` y abre
`http://127.0.0.1:47834`.

## 🔐 Seguridad y privacidad

- **Todo ocurre en tu PC**: el PDF no sale del equipo; la app no usa internet.
- El servidor de la app escucha **solo en `127.0.0.1`** y la interfaz no se puede incrustar en otras páginas.
- La API rechaza peticiones con un origen distinto al de la propia app y las acciones exigen una cabecera propia, así que una página web abierta en el navegador no puede usarla.
- pdf.js corre sin ejecutar los scripts que traen algunos PDF, y las imágenes recibidas tienen topes de tamaño.

## ⚖️ Licencia

**Creado por [Miguel Bigott](https://github.com/MiguelBigott)** y publicado con licencia [MIT](LICENSE): cualquiera puede usarlo, copiarlo, modificarlo y compartirlo libremente, también en sus propios proyectos, siempre que mantenga el aviso de autor. Se entrega tal cual, sin garantías.

pdf.js, las bibliotecas de Go y las tipografías conservan sus licencias: ver [THIRD-PARTY-NOTICES.md](THIRD-PARTY-NOTICES.md) y el texto completo en [THIRD-PARTY-LICENSES.txt](THIRD-PARTY-LICENSES.txt), que se instalan junto a la app.
