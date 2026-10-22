# Historial de versiones

Todas las versiones siguen [Versionado Semántico](https://semver.org/lang/es/). Cada una tiene su
etiqueta `vX.Y.Z` y su página en [Releases](../../releases).

## [1.1.0] - 2026-10-22

Primera versión publicada.

### Añadido
- Conversión de PDF a EPUB 3 en dos modos: **texto fluido** (párrafos, títulos y capítulos rearmados, sin encabezados ni números de página) y **páginas fijas** (cada página idéntica al PDF).
- Capítulos en un pliego: la página original del PDF a la izquierda y cómo se leerá a la derecha. Se pueden renombrar, unir, excluir y dividir.
- Editor de portada: una página del PDF, una imagen propia o un color, con plantillas de título y autor.
- Datos del libro: título, autores, idioma, fecha, editorial, ISBN, serie, etiquetas y descripción.
- PDF con contraseña.
- Skin «Pliego» y logo «Cosido», de la familia de Codex. Instalador NSIS, para instalarla y actualizarla desde Codex.

### Seguridad
- pdf.js 6.4.299 sin ejecutar los scripts de los PDF. La API local rechaza otros orígenes y limita el tamaño, las medidas y las subidas simultáneas de imágenes.
