package main

// Escritura del EPUB 3: texto fluido (capítulos en XHTML) o páginas fijas (una imagen por
// página). Lleva también el índice para lectores viejos (toc.ncx) y los datos de serie que
// entienden Calibre y la mayoría de los lectores.

import (
	"archive/zip"
	"crypto/rand"
	"fmt"
	"html"
	"io"
	"os"
	"regexp"
	"strings"
	"time"
)

type Metadatos struct {
	Titulo      string `json:"titulo"`
	Autor       string `json:"autor"`
	Idioma      string `json:"idioma"`
	Editorial   string `json:"editorial"`
	Fecha       string `json:"fecha"`
	Descripcion string `json:"descripcion"`
	Serie       string `json:"serie"`
	Numero      string `json:"numero"`
	ISBN        string `json:"isbn"`
	Etiquetas   string `json:"etiquetas"`
}

// CapituloFinal es un capítulo tal como quedó en el editor. Desde es el primer bloque (texto
// fluido) y Pagina la primera página (páginas fijas), empezando en 0.
type CapituloFinal struct {
	Titulo  string `json:"titulo"`
	Desde   int    `json:"desde"`
	Pagina  int    `json:"pagina"`
	Incluir bool   `json:"incluir"`
}

// Imagen es un recurso ya guardado en el disco.
type Imagen struct {
	Ruta  string
	Tipo  string // "image/jpeg" o "image/png"
	Ancho int
	Alto  int
}

func (im Imagen) extension() string {
	if im.Tipo == "image/png" {
		return "png"
	}
	return "jpg"
}

// Libro es todo lo que hace falta para escribir el EPUB.
type Libro struct {
	Datos     Metadatos
	Fijo      bool
	Portada   *Imagen
	Capitulos []CapituloFinal
	Bloques   []Bloque          // texto fluido
	Paginas   []Imagen          // páginas fijas, una por página del PDF
	Imagenes  map[string]Imagen // imágenes de los bloques, por id
	Estilo    string            // CSS del texto fluido
}

var reFecha = regexp.MustCompile(`^\d{4}(-\d{2}(-\d{2})?)?$`)

// esc escapa un texto para XML y quita lo que no puede ir en él.
func esc(s string) string { return html.EscapeString(limpiarXML(strings.TrimSpace(s))) }

func uuid4() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	b[6] = b[6]&0x0f | 0x40
	b[8] = b[8]&0x3f | 0x80
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

// autores separa varios autores escritos con «;» o «&».
func autores(s string) []string {
	var res []string
	for _, a := range regexp.MustCompile(`\s*[;&]\s*`).Split(s, -1) {
		if a = strings.TrimSpace(a); a != "" {
			res = append(res, a)
		}
	}
	return res
}

type archivoEPUB struct {
	ruta, id, tipo, propiedades string
	enLomo                      bool // va en el spine (orden de lectura)
	lineal                      bool
	titulo                      string
}

type entradaNav struct {
	titulo, ruta string
	hijos        []entradaNav
}

// EscribirEPUB escribe el libro en w.
func EscribirEPUB(w io.Writer, lb *Libro) error {
	z := zip.NewWriter(w)
	// «mimetype» va primero y sin comprimir: así reconocen el archivo los lectores.
	f, err := z.CreateHeader(&zip.FileHeader{Name: "mimetype", Method: zip.Store})
	if err != nil {
		return err
	}
	if _, err := io.WriteString(f, "application/epub+zip"); err != nil {
		return err
	}
	escribir := func(nombre, contenido string) error {
		f, err := z.Create(nombre)
		if err != nil {
			return err
		}
		_, err = io.WriteString(f, contenido)
		return err
	}
	copiar := func(nombre, origen string) error {
		f, err := z.CreateHeader(&zip.FileHeader{Name: nombre, Method: zip.Store}) // ya están comprimidas
		if err != nil {
			return err
		}
		src, err := os.Open(origen)
		if err != nil {
			return err
		}
		defer src.Close()
		_, err = io.Copy(f, src)
		return err
	}

	if err := escribir("META-INF/container.xml", `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="OEBPS/content.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`); err != nil {
		return err
	}

	idioma := strings.TrimSpace(lb.Datos.Idioma)
	if idioma == "" {
		idioma = "und"
	}
	titulo := strings.TrimSpace(lb.Datos.Titulo)
	if titulo == "" {
		titulo = "Sin título"
	}

	var archivos []archivoEPUB
	var nav []entradaNav
	agregar := func(a archivoEPUB) { archivos = append(archivos, a) }

	// Portada
	if lb.Portada != nil {
		ruta := "imagenes/portada." + lb.Portada.extension()
		if err := copiar("OEBPS/"+ruta, lb.Portada.Ruta); err != nil {
			return err
		}
		agregar(archivoEPUB{ruta: ruta, id: "portada-img", tipo: lb.Portada.Tipo, propiedades: "cover-image"})
		if err := escribir("OEBPS/portada.xhtml", xhtmlPortada(lb, idioma, titulo, ruta)); err != nil {
			return err
		}
		agregar(archivoEPUB{ruta: "portada.xhtml", id: "portada", tipo: "application/xhtml+xml", enLomo: true, lineal: true, propiedades: portadaSVG(lb.Fijo)})
	}

	if lb.Fijo {
		if err := escribirFijo(lb, idioma, copiar, escribir, agregar, &nav); err != nil {
			return err
		}
	} else {
		if err := escribir("OEBPS/estilo.css", lb.Estilo); err != nil {
			return err
		}
		agregar(archivoEPUB{ruta: "estilo.css", id: "estilo", tipo: "text/css"})
		if err := escribirFluido(lb, idioma, copiar, escribir, agregar, &nav); err != nil {
			return err
		}
	}

	if err := escribir("OEBPS/nav.xhtml", xhtmlNav(lb, idioma, titulo, nav)); err != nil {
		return err
	}
	agregar(archivoEPUB{ruta: "nav.xhtml", id: "nav", tipo: "application/xhtml+xml", propiedades: "nav"})
	uid := "urn:uuid:" + uuid4()
	if err := escribir("OEBPS/toc.ncx", ncx(uid, titulo, nav)); err != nil {
		return err
	}
	agregar(archivoEPUB{ruta: "toc.ncx", id: "ncx", tipo: "application/x-dtbncx+xml"})

	if err := escribir("OEBPS/content.opf", opf(lb, uid, idioma, titulo, archivos)); err != nil {
		return err
	}
	return z.Close()
}

// portadaSVG: en texto fluido la portada encaja la imagen con un SVG, y EPUB 3 pide declararlo.
func portadaSVG(fijo bool) string {
	if fijo {
		return ""
	}
	return "svg"
}

func cabeceraXHTML(idioma, titulo, extra string) string {
	return `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE html>
<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` + esc(idioma) + `" lang="` + esc(idioma) + `">
<head>
<meta charset="UTF-8"/>
<title>` + esc(titulo) + `</title>
` + extra + `</head>
`
}

func xhtmlPortada(lb *Libro, idioma, titulo, img string) string {
	if lb.Fijo {
		w, h := lb.Portada.Ancho, lb.Portada.Alto
		return cabeceraXHTML(idioma, titulo, fmt.Sprintf(`<meta name="viewport" content="width=%d, height=%d"/>
<style>html, body { margin: 0; padding: 0; width: %dpx; height: %dpx; } img { display: block; width: %dpx; height: %dpx; }</style>
`, w, h, w, h, w, h)) + `<body epub:type="cover"><img src="` + img + `" alt="` + esc(titulo) + `"/></body>
</html>
`
	}
	// La imagen ocupa toda la pantalla sin deformarse: el SVG la encaja como hacen las editoriales.
	w, h := lb.Portada.Ancho, lb.Portada.Alto
	return cabeceraXHTML(idioma, titulo, `<style>html, body { margin: 0; padding: 0; height: 100%; text-align: center; } svg { display: block; width: 100%; height: 100%; }</style>
`) + fmt.Sprintf(`<body epub:type="cover">
<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" viewBox="0 0 %d %d" preserveAspectRatio="xMidYMid meet">
<image width="%d" height="%d" xlink:href="%s"/>
</svg>
</body>
</html>
`, w, h, w, h, img)
}

type fnCopiar func(nombre, origen string) error
type fnEscribir func(nombre, contenido string) error

func escribirFluido(lb *Libro, idioma string, copiar fnCopiar, escribir fnEscribir, agregar func(archivoEPUB), nav *[]entradaNav) error {
	usadas := map[string]string{} // id del recurso → ruta dentro del EPUB
	num := 0
	for i, c := range lb.Capitulos {
		if !c.Incluir {
			continue
		}
		fin := len(lb.Bloques)
		if i+1 < len(lb.Capitulos) {
			fin = lb.Capitulos[i+1].Desde
		}
		if c.Desde < 0 || c.Desde >= fin || fin > len(lb.Bloques) {
			continue
		}
		num++
		ruta := fmt.Sprintf("texto/c%03d.xhtml", num)
		tituloCap := strings.TrimSpace(c.Titulo)
		if tituloCap == "" {
			tituloCap = fmt.Sprint(num)
		}
		entrada := entradaNav{titulo: tituloCap, ruta: ruta}
		var b strings.Builder
		b.WriteString(cabeceraXHTML(idioma, tituloCap, `<link rel="stylesheet" type="text/css" href="../estilo.css"/>
`))
		b.WriteString("<body>\n<section epub:type=\"chapter\" role=\"doc-chapter\">\n")
		for k := c.Desde; k < fin; k++ {
			bl := lb.Bloques[k]
			switch bl.Tipo {
			case "h":
				nivel := min(max(bl.Nivel, 1), 3)
				id := ""
				if k > c.Desde {
					// Los subtítulos del capítulo entran en el índice, un nivel más adentro.
					id = fmt.Sprintf(` id="t%d"`, k)
					entrada.hijos = append(entrada.hijos, entradaNav{titulo: bl.Texto, ruta: fmt.Sprintf("%s#t%d", ruta, k)})
				}
				fmt.Fprintf(&b, "<h%d%s>%s</h%d>\n", nivel, id, bl.HTML, nivel)
			case "img":
				im, ok := lb.Imagenes[bl.Imagen]
				if !ok {
					continue
				}
				destino, ya := usadas[bl.Imagen]
				if !ya {
					destino = fmt.Sprintf("imagenes/i%04d.%s", len(usadas)+1, im.extension())
					if err := copiar("OEBPS/"+destino, im.Ruta); err != nil {
						return err
					}
					usadas[bl.Imagen] = destino
					agregar(archivoEPUB{ruta: destino, id: fmt.Sprintf("i%04d", len(usadas)), tipo: im.Tipo})
				}
				fmt.Fprintf(&b, "<figure class=\"imagen\"><img src=\"../%s\" alt=\"\"/></figure>\n", destino)
			default:
				clase := ""
				if bl.Centro {
					clase = ` class="centro"`
				}
				fmt.Fprintf(&b, "<p%s>%s</p>\n", clase, bl.HTML)
			}
		}
		b.WriteString("</section>\n</body>\n</html>\n")
		if err := escribir("OEBPS/"+ruta, b.String()); err != nil {
			return err
		}
		agregar(archivoEPUB{ruta: ruta, id: fmt.Sprintf("c%03d", num), tipo: "application/xhtml+xml", enLomo: true, lineal: true, titulo: tituloCap})
		*nav = append(*nav, entrada)
	}
	return nil
}

func escribirFijo(lb *Libro, idioma string, copiar fnCopiar, escribir fnEscribir, agregar func(archivoEPUB), nav *[]entradaNav) error {
	// Qué páginas van: las de los capítulos incluidos.
	incluida := make([]bool, len(lb.Paginas))
	inicioCap := map[int]string{}
	for i, c := range lb.Capitulos {
		fin := len(lb.Paginas)
		if i+1 < len(lb.Capitulos) {
			fin = lb.Capitulos[i+1].Pagina
		}
		if !c.Incluir || c.Pagina < 0 || c.Pagina >= fin || fin > len(lb.Paginas) {
			continue
		}
		for p := c.Pagina; p < fin; p++ {
			incluida[p] = true
		}
		inicioCap[c.Pagina] = strings.TrimSpace(c.Titulo)
	}
	if len(lb.Capitulos) == 0 {
		for i := range incluida {
			incluida[i] = true
		}
	}
	num := 0
	for p, im := range lb.Paginas {
		if !incluida[p] {
			continue
		}
		num++
		img := fmt.Sprintf("imagenes/p%04d.%s", num, im.extension())
		if err := copiar("OEBPS/"+img, im.Ruta); err != nil {
			return err
		}
		agregar(archivoEPUB{ruta: img, id: fmt.Sprintf("pi%04d", num), tipo: im.Tipo})
		ruta := fmt.Sprintf("paginas/p%04d.xhtml", num)
		titulo := fmt.Sprint(p + 1)
		contenido := cabeceraXHTML(idioma, titulo, fmt.Sprintf(`<meta name="viewport" content="width=%d, height=%d"/>
<style>html, body { margin: 0; padding: 0; width: %dpx; height: %dpx; } img { display: block; width: %dpx; height: %dpx; }</style>
`, im.Ancho, im.Alto, im.Ancho, im.Alto, im.Ancho, im.Alto)) + `<body><img src="../` + img + `" alt=""/></body>
</html>
`
		if err := escribir("OEBPS/"+ruta, contenido); err != nil {
			return err
		}
		agregar(archivoEPUB{ruta: ruta, id: fmt.Sprintf("p%04d", num), tipo: "application/xhtml+xml", enLomo: true, lineal: true})
		if t, ok := inicioCap[p]; ok {
			if t == "" {
				t = titulo
			}
			*nav = append(*nav, entradaNav{titulo: t, ruta: ruta})
		}
	}
	if len(*nav) == 0 && num > 0 {
		*nav = append(*nav, entradaNav{titulo: "1", ruta: "paginas/p0001.xhtml"})
	}
	return nil
}

func xhtmlNav(lb *Libro, idioma, titulo string, nav []entradaNav) string {
	var b strings.Builder
	b.WriteString(cabeceraXHTML(idioma, titulo, ""))
	b.WriteString("<body>\n<nav epub:type=\"toc\" id=\"toc\" role=\"doc-toc\">\n<h1>" + esc(titulo) + "</h1>\n<ol>\n")
	for _, e := range nav {
		b.WriteString(`<li><a href="` + e.ruta + `">` + esc(e.titulo) + "</a>")
		if len(e.hijos) > 0 {
			b.WriteString("\n<ol>\n")
			for _, h := range e.hijos {
				b.WriteString(`<li><a href="` + h.ruta + `">` + esc(h.titulo) + "</a></li>\n")
			}
			b.WriteString("</ol>\n")
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</ol>\n</nav>\n")
	b.WriteString("<nav epub:type=\"landmarks\" hidden=\"hidden\">\n<ol>\n")
	if lb.Portada != nil {
		b.WriteString(`<li><a epub:type="cover" href="portada.xhtml">` + esc(titulo) + "</a></li>\n")
	}
	if len(nav) > 0 {
		b.WriteString(`<li><a epub:type="bodymatter" href="` + nav[0].ruta + `">` + esc(nav[0].titulo) + "</a></li>\n")
	}
	b.WriteString("</ol>\n</nav>\n</body>\n</html>\n")
	return b.String()
}

func ncx(uid, titulo string, nav []entradaNav) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>
<ncx xmlns="http://www.daisy.org/z3986/2005/ncx/" version="2005-1">
<head>
<meta name="dtb:uid" content="` + esc(uid) + `"/>
<meta name="dtb:depth" content="2"/>
<meta name="dtb:totalPageCount" content="0"/>
<meta name="dtb:maxPageNumber" content="0"/>
</head>
<docTitle><text>` + esc(titulo) + `</text></docTitle>
<navMap>
`)
	orden := 0
	var punto func(e entradaNav)
	punto = func(e entradaNav) {
		orden++
		fmt.Fprintf(&b, "<navPoint id=\"n%d\" playOrder=\"%d\"><navLabel><text>%s</text></navLabel><content src=\"%s\"/>\n", orden, orden, esc(e.titulo), e.ruta)
		for _, h := range e.hijos {
			punto(h)
		}
		b.WriteString("</navPoint>\n")
	}
	for _, e := range nav {
		punto(e)
	}
	b.WriteString("</navMap>\n</ncx>\n")
	return b.String()
}

func opf(lb *Libro, uid, idioma, titulo string, archivos []archivoEPUB) string {
	d := lb.Datos
	var m strings.Builder
	m.WriteString(`    <dc:identifier id="uid">` + esc(uid) + "</dc:identifier>\n")
	m.WriteString(`    <dc:title id="titulo">` + esc(titulo) + "</dc:title>\n")
	m.WriteString(`    <dc:language>` + esc(idioma) + "</dc:language>\n")
	for i, a := range autores(d.Autor) {
		fmt.Fprintf(&m, "    <dc:creator id=\"autor%d\">%s</dc:creator>\n", i+1, esc(a))
		fmt.Fprintf(&m, "    <meta refines=\"#autor%d\" property=\"role\" scheme=\"marc:relators\">aut</meta>\n", i+1)
	}
	if s := strings.TrimSpace(d.Editorial); s != "" {
		m.WriteString("    <dc:publisher>" + esc(s) + "</dc:publisher>\n")
	}
	if s := strings.TrimSpace(d.Fecha); reFecha.MatchString(s) {
		m.WriteString("    <dc:date>" + s + "</dc:date>\n")
	}
	if s := strings.TrimSpace(d.Descripcion); s != "" {
		m.WriteString("    <dc:description>" + esc(s) + "</dc:description>\n")
	}
	if isbn := limpiarISBN(d.ISBN); isbn != "" {
		m.WriteString("    <dc:identifier id=\"isbn\">urn:isbn:" + isbn + "</dc:identifier>\n")
	}
	for _, e := range strings.Split(d.Etiquetas, ",") {
		if e = strings.TrimSpace(e); e != "" {
			m.WriteString("    <dc:subject>" + esc(e) + "</dc:subject>\n")
		}
	}
	if s := strings.TrimSpace(d.Serie); s != "" {
		m.WriteString("    <meta property=\"belongs-to-collection\" id=\"serie\">" + esc(s) + "</meta>\n")
		m.WriteString("    <meta refines=\"#serie\" property=\"collection-type\">series</meta>\n")
		if n := numeroSerie(d.Numero); n != "" {
			m.WriteString("    <meta refines=\"#serie\" property=\"group-position\">" + n + "</meta>\n")
		}
		// Calibre y muchos lectores leen la serie de estas dos etiquetas de EPUB 2.
		m.WriteString("    <meta name=\"calibre:series\" content=\"" + esc(s) + "\"/>\n")
		if n := numeroSerie(d.Numero); n != "" {
			m.WriteString("    <meta name=\"calibre:series_index\" content=\"" + n + "\"/>\n")
		}
	}
	if lb.Portada != nil {
		m.WriteString("    <meta name=\"cover\" content=\"portada-img\"/>\n")
	}
	if lb.Fijo {
		m.WriteString("    <meta property=\"rendition:layout\">pre-paginated</meta>\n")
		m.WriteString("    <meta property=\"rendition:orientation\">auto</meta>\n")
		m.WriteString("    <meta property=\"rendition:spread\">landscape</meta>\n")
	}
	m.WriteString("    <meta property=\"dcterms:modified\">" + time.Now().UTC().Format("2006-01-02T15:04:05Z") + "</meta>\n")
	m.WriteString("    <meta name=\"generator\" content=\"PdfEpub " + Version + "\"/>\n")

	var man, lomo strings.Builder
	for _, a := range archivos {
		props := ""
		if a.propiedades != "" {
			props = ` properties="` + a.propiedades + `"`
		}
		fmt.Fprintf(&man, "    <item id=\"%s\" href=\"%s\" media-type=\"%s\"%s/>\n", a.id, a.ruta, a.tipo, props)
		if a.enLomo {
			fmt.Fprintf(&lomo, "    <itemref idref=\"%s\"/>\n", a.id)
		}
	}
	prefijo := ""
	if lb.Fijo {
		prefijo = ` prefix="rendition: http://www.idpf.org/vocab/rendition/#"`
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="uid" xml:lang="` + esc(idioma) + `"` + prefijo + `>
  <metadata xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:opf="http://www.idpf.org/2007/opf">
` + m.String() + `  </metadata>
  <manifest>
` + man.String() + `  </manifest>
  <spine toc="ncx">
` + lomo.String() + `  </spine>
</package>
`
}

func limpiarISBN(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= '0' && r <= '9') || r == 'X' || r == 'x' {
			b.WriteRune(r)
		}
	}
	if n := b.Len(); n != 10 && n != 13 {
		return ""
	}
	return strings.ToUpper(b.String())
}

var reNumero = regexp.MustCompile(`^\d+([.,]\d+)?$`)

func numeroSerie(s string) string {
	s = strings.TrimSpace(s)
	if !reNumero.MatchString(s) {
		return ""
	}
	return strings.ReplaceAll(s, ",", ".")
}
