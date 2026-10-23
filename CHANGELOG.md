# Historial de versiones

Todas las versiones siguen [Versionado Semántico](https://semver.org/lang/es/). Cada una tiene su
etiqueta `vX.Y.Z` y su página en [Releases](../../releases).

## [1.1.2] - 2026-10-23

### Corregido
- La letra capitular (la letra grande con que empieza un capítulo) se perdía y mezclaba los renglones que tenía al lado. Ahora el párrafo empieza con ella y sigue entero.
- No se podía cambiar el nombre de un capítulo: al hacer clic en el título, el campo se borraba antes de poder escribir.

### Cambiado
- Un título que es solo el número del capítulo («4») también separa capítulos.

## [1.1.1] - 2026-10-22

### Corregido
- Los PDF con imágenes no se podían convertir: al leerlos aparecía «Cannot read properties of undefined (reading '0')».
- Después de un error, no se podía abrir otro PDF sin cerrar la app.

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
