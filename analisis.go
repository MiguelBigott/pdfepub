package main

// Análisis del texto del PDF: pdf.js (en la interfaz) manda cada trozo de texto con su posición,
// su tamaño y su tipografía, y aquí se rearma el libro: líneas, columnas, párrafos, títulos y
// capítulos, sin encabezados de página ni números de página.

import (
	"html"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"
)

// ---------------- Lo que manda la interfaz ----------------

// Documento es lo que pdf.js extrae del PDF. Las coordenadas van en puntos, con el origen arriba
// a la izquierda de la página (y crece hacia abajo).
type Documento struct {
	Paginas []PaginaPDF     `json:"paginas"`
	Fuentes []Fuente        `json:"fuentes"`
	Indice  []EntradaIndice `json:"indice"`
}

type PaginaPDF struct {
	Ancho    float64     `json:"ancho"`
	Alto     float64     `json:"alto"`
	Textos   []Trozo     `json:"textos"`
	Imagenes []ImagenPDF `json:"imagenes"`
}

// Trozo es un fragmento de texto: X e Y son el inicio de su línea base, W su ancho, T el tamaño
// de letra y F el número de su tipografía en Documento.Fuentes.
type Trozo struct {
	S string  `json:"s"`
	X float64 `json:"x"`
	Y float64 `json:"y"`
	W float64 `json:"w"`
	T float64 `json:"t"`
	F int     `json:"f"`
}

type Fuente struct {
	Nombre  string `json:"nombre"`
	Negrita bool   `json:"negrita"`
	Cursiva bool   `json:"cursiva"`
}

// ImagenPDF es una imagen ya recortada y guardada como recurso; X, Y es su esquina superior izquierda.
type ImagenPDF struct {
	ID string  `json:"id"`
	X  float64 `json:"x"`
	Y  float64 `json:"y"`
	W  float64 `json:"w"`
	H  float64 `json:"h"`
}

// EntradaIndice es un marcador del PDF (su índice interno). Pagina empieza en 0.
type EntradaIndice struct {
	Titulo string  `json:"titulo"`
	Pagina int     `json:"pagina"`
	Y      float64 `json:"y"`
	Nivel  int     `json:"nivel"`
}

// ---------------- Lo que devuelve ----------------

// Bloque es un párrafo, un título o una imagen del libro, en orden de lectura.
type Bloque struct {
	Tipo   string  `json:"tipo"` // "p", "h" o "img"
	Nivel  int     `json:"nivel,omitempty"`
	HTML   string  `json:"html,omitempty"`
	Texto  string  `json:"texto,omitempty"`
	Centro bool    `json:"centro,omitempty"`
	Imagen string  `json:"imagen,omitempty"`
	Ancho  float64 `json:"ancho,omitempty"`
	Alto   float64 `json:"alto,omitempty"`
	Pagina int     `json:"pagina"`

	y         float64
	capitular bool
}

// CapituloSugerido empieza en el bloque Desde. Sin título, Tipo dice cómo nombrarlo:
// "inicio" (lo que va antes del primer capítulo) o "paginas" (el PDF no tiene capítulos).
type CapituloSugerido struct {
	Titulo string `json:"titulo"`
	Desde  int    `json:"desde"`
	Tipo   string `json:"tipo,omitempty"`
}

type Analisis struct {
	Bloques   []Bloque           `json:"bloques"`
	Capitulos []CapituloSugerido `json:"capitulos"`
	Idioma    string             `json:"idioma"`
	Palabras  int                `json:"palabras"`
	Escaneado bool               `json:"escaneado"`
}

// ---------------- Líneas ----------------

type tramo struct {
	s                string
	negrita, cursiva bool
}

type linea struct {
	x0, x1, y, t float64
	tramos       []tramo
	pagina       int
	imagen       *ImagenPDF
	negrita      bool // toda la línea en negrita
}

func (l *linea) texto() string {
	var b strings.Builder
	for _, t := range l.tramos {
		b.WriteString(t.s)
	}
	return b.String()
}

// metricas del cuerpo del texto: el tamaño de letra más usado y si es negrita o cursiva.
type metricas struct {
	cuerpo           float64
	negrita, cursiva bool
}

func redondear(t float64) float64 { return math.Round(t*2) / 2 }

func medirCuerpo(doc *Documento) metricas {
	porTam := map[float64]int{}
	var negr, curs, total int
	for _, p := range doc.Paginas {
		for _, t := range p.Textos {
			n := utf8.RuneCountInString(strings.TrimSpace(t.S))
			if n == 0 || t.T <= 0 {
				continue
			}
			porTam[redondear(t.T)] += n
		}
	}
	m := metricas{cuerpo: 10}
	mejor := 0
	for t, n := range porTam {
		if n > mejor || (n == mejor && t < m.cuerpo) {
			m.cuerpo, mejor = t, n
		}
	}
	for _, p := range doc.Paginas {
		for _, t := range p.Textos {
			if redondear(t.T) != m.cuerpo {
				continue
			}
			n := utf8.RuneCountInString(t.S)
			total += n
			f := fuenteDe(doc, t.F)
			if f.Negrita {
				negr += n
			}
			if f.Cursiva {
				curs += n
			}
		}
	}
	m.negrita = total > 0 && negr*2 > total
	m.cursiva = total > 0 && curs*2 > total
	return m
}

func fuenteDe(doc *Documento, i int) Fuente {
	if i >= 0 && i < len(doc.Fuentes) {
		return doc.Fuentes[i]
	}
	return Fuente{}
}

// lineasDePagina agrupa los trozos de una página en líneas, en orden de lectura (también con
// dos columnas). Las imágenes entran como líneas especiales en su lugar.
func lineasDePagina(doc *Documento, num int, m metricas) ([]*linea, bool) {
	p := doc.Paginas[num]
	trozos := make([]Trozo, 0, len(p.Textos))
	for _, t := range p.Textos {
		if strings.TrimSpace(t.S) != "" && t.T > 0 {
			trozos = append(trozos, t)
		}
	}
	sort.SliceStable(trozos, func(i, j int) bool {
		if trozos[i].Y != trozos[j].Y {
			return trozos[i].Y < trozos[j].Y
		}
		return trozos[i].X < trozos[j].X
	})

	// Grupos de trozos con la misma línea base (con margen para índices y superíndices).
	var grupos [][]Trozo
	var yRef, tRef float64
	for _, t := range trozos {
		if n := len(grupos); n > 0 && math.Abs(t.Y-yRef) <= 0.45*math.Max(t.T, tRef) {
			grupos[n-1] = append(grupos[n-1], t)
			if t.T > tRef {
				yRef, tRef = t.Y, t.T
			}
			continue
		}
		grupos = append(grupos, []Trozo{t})
		yRef, tRef = t.Y, t.T
	}

	// Cada grupo se corta en segmentos donde hay un hueco grande (el espacio entre columnas).
	var segmentos []*linea
	for _, g := range grupos {
		sort.SliceStable(g, func(i, j int) bool { return g[i].X < g[j].X })
		inicio := 0
		for i := 1; i <= len(g); i++ {
			if i == len(g) || g[i].X-(g[i-1].X+g[i-1].W) > 1.2*math.Max(g[i].T, g[i-1].T) {
				segmentos = append(segmentos, armarLinea(doc, g[inicio:i], num))
				inicio = i
			}
		}
	}
	segmentos = separarCapitulares(segmentos, m)

	// Los segmentos ya están en orden (de arriba abajo y de izquierda a derecha): cada imagen
	// entra antes del primer texto que está más abajo que su borde superior.
	for i := range p.Imagenes {
		im := &p.Imagenes[i]
		l := &linea{x0: im.X, x1: im.X + im.W, y: im.Y, t: m.cuerpo, pagina: num, imagen: im}
		pos := len(segmentos)
		for k, sg := range segmentos {
			if sg.y > im.Y {
				pos = k
				break
			}
		}
		segmentos = append(segmentos[:pos], append([]*linea{l}, segmentos[pos:]...)...)
	}

	if !dosColumnas(segmentos, p.Ancho) {
		return unirSegmentos(segmentos), false
	}
	// Dos columnas: lo que cruza el centro (títulos, imágenes anchas) corta la lectura; entre
	// cortes se lee primero la columna izquierda y después la derecha.
	mitad, tol := p.Ancho/2, p.Ancho*0.03
	var res, izq, der []*linea
	vaciar := func() {
		res = append(res, izq...)
		res = append(res, der...)
		izq, der = nil, nil
	}
	for _, s := range segmentos {
		switch {
		case s.x1 <= mitad+tol:
			izq = append(izq, s)
		case s.x0 >= mitad-tol:
			der = append(der, s)
		default:
			vaciar()
			res = append(res, s)
		}
	}
	vaciar()
	return res, true
}

func dosColumnas(segs []*linea, ancho float64) bool {
	if ancho <= 0 {
		return false
	}
	mitad, tol := ancho/2, ancho*0.03
	var izq, der, total int
	for _, s := range segs {
		if s.imagen != nil || utf8.RuneCountInString(s.texto()) < 5 {
			continue
		}
		total++
		switch {
		case s.x1 <= mitad+tol:
			izq++
		case s.x0 >= mitad-tol:
			der++
		}
	}
	return izq >= 3 && der >= 3 && float64(izq+der) >= 0.6*float64(total)
}

// unirSegmentos vuelve a juntar en una línea los segmentos de una misma línea base (en una página
// de una columna, el hueco es una tabulación, no un cambio de columna).
func unirSegmentos(segs []*linea) []*linea {
	var res []*linea
	for _, s := range segs {
		if n := len(res); n > 0 && s.imagen == nil && res[n-1].imagen == nil &&
			math.Abs(s.y-res[n-1].y) <= 0.45*math.Min(s.t, res[n-1].t) && s.x0 >= res[n-1].x1 {
			u := res[n-1]
			u.tramos = append(u.tramos, tramo{s: " "})
			u.tramos = append(u.tramos, s.tramos...)
			u.x1 = s.x1
			u.t = math.Max(u.t, s.t)
			u.negrita = u.negrita && s.negrita
			continue
		}
		res = append(res, s)
	}
	return res
}

// armarLinea junta los trozos de un segmento: agrega los espacios que el PDF no trae y quita el
// texto repetido (algunos PDF imitan la negrita escribiendo dos veces lo mismo).
func armarLinea(doc *Documento, g []Trozo, pagina int) *linea {
	l := &linea{x0: g[0].X, y: g[0].Y, pagina: pagina, negrita: true}
	pesoTam := map[float64]int{}
	var ant *Trozo
	for i := range g {
		t := &g[i]
		if ant != nil && t.S == ant.S && math.Abs(t.X-ant.X) < 0.2*t.T {
			continue
		}
		f := fuenteDe(doc, t.F)
		if ant != nil {
			hueco := t.X - (ant.X + ant.W)
			if hueco > 0.18*math.Min(t.T, ant.T) && !strings.HasSuffix(ant.S, " ") && !strings.HasPrefix(t.S, " ") {
				l.tramos = append(l.tramos, tramo{s: " "})
			}
		}
		l.tramos = append(l.tramos, tramo{s: t.S, negrita: f.Negrita, cursiva: f.Cursiva})
		n := utf8.RuneCountInString(strings.TrimSpace(t.S))
		pesoTam[redondear(t.T)] += n
		if n > 0 && !f.Negrita {
			l.negrita = false
		}
		l.x1 = math.Max(l.x1, t.X+t.W)
		ant = t
	}
	mejor := -1
	for tam, n := range pesoTam {
		if n > mejor || (n == mejor && tam > l.t) {
			l.t, mejor = tam, n
		}
	}
	// La línea base es la del tamaño principal (no la de un superíndice).
	for _, t := range g {
		if redondear(t.T) == l.t {
			l.y = t.Y
			break
		}
	}
	return l
}

// separarCapitulares saca las letras capitulares (una letra enorme al empezar un párrafo, que
// baja varias líneas) y las pega al principio de la primera línea que acompañan.
func separarCapitulares(segs []*linea, m metricas) []*linea {
	for i, s := range segs {
		texto := strings.TrimSpace(s.texto())
		if s.t < 2*m.cuerpo || utf8.RuneCountInString(texto) > 2 || !esLetra(texto) {
			continue
		}
		var destino *linea
		for _, o := range segs {
			if o == s || o.x0 < s.x0 || o.y > s.y+0.1*s.t || o.y < s.y-1.2*s.t || o.t >= 1.5*m.cuerpo {
				continue
			}
			if destino == nil || o.y < destino.y {
				destino = o
			}
		}
		if destino == nil {
			continue
		}
		destino.tramos = append([]tramo{{s: texto, negrita: s.tramos[0].negrita, cursiva: s.tramos[0].cursiva}}, destino.tramos...)
		destino.x0 = s.x0
		segs[i] = nil
	}
	res := segs[:0]
	for _, s := range segs {
		if s != nil {
			res = append(res, s)
		}
	}
	return res
}

func esLetra(s string) bool {
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return s != ""
}

// ---------------- Encabezados y números de página ----------------

var (
	reNumeroPagina = regexp.MustCompile(`(?i)^[\s\p{P}]*(\d{1,4}|[ivxlcdm]{1,7})[\s\p{P}]*$`)
	reDigitos      = regexp.MustCompile(`\d+`)
)

func claveRepetida(s string) string {
	s = strings.ToLower(strings.Join(strings.Fields(s), " "))
	return reDigitos.ReplaceAllString(s, "#")
}

// quitarEncabezados borra las líneas que se repiten arriba o abajo en muchas páginas (el título
// del libro, el del capítulo) y los números de página.
func quitarEncabezados(paginas [][]*linea, doc *Documento) {
	enBorde := func(l *linea) bool {
		alto := doc.Paginas[l.pagina].Alto
		return alto > 0 && (l.y < alto*0.1+l.t || l.y > alto*0.9)
	}
	candidatas := func(ls []*linea) []*linea {
		var c []*linea
		for i, l := range ls {
			if l.imagen == nil && (i < 2 || i >= len(ls)-2) && enBorde(l) {
				c = append(c, l)
			}
		}
		return c
	}
	cuenta := map[string]int{}
	for _, ls := range paginas {
		vistas := map[string]bool{}
		for _, l := range candidatas(ls) {
			k := claveRepetida(l.texto())
			if !vistas[k] {
				vistas[k] = true
				cuenta[k]++
			}
		}
	}
	minimo := math.Max(3, 0.2*float64(len(paginas)))
	for i, ls := range paginas {
		borrar := map[*linea]bool{}
		for _, l := range candidatas(ls) {
			t := l.texto()
			if reNumeroPagina.MatchString(t) || (len(paginas) >= 6 && float64(cuenta[claveRepetida(t)]) >= minimo) {
				borrar[l] = true
			}
		}
		if len(borrar) == 0 {
			continue
		}
		res := ls[:0]
		for _, l := range ls {
			if !borrar[l] {
				res = append(res, l)
			}
		}
		paginas[i] = res
	}
}

// ---------------- Párrafos y títulos ----------------

var reCapitulo = regexp.MustCompile(`(?i)^\s*(cap[ií]tulo|chapter|chapitre|kapitel|capitolo|cap[íi]tol|parte|part|partie|teil|libro|book|livre|buch|pr[oó]logo|prologue|prolog|ep[ií]logo|epilogue|epilog|introducci[oó]n|introduction|einleitung|prefacio|preface|pr[ée]face|vorwort|глава|часть|第.{1,4}[章部])\b`)

const finales = ".!?:;…»”\"’)]"

func terminaFrase(s string) bool {
	s = strings.TrimRightFunc(s, unicode.IsSpace)
	r, _ := utf8.DecodeLastRuneInString(s)
	return r != utf8.RuneError && strings.ContainsRune(finales, r)
}

func empiezaMinuscula(s string) bool {
	r, _ := utf8.DecodeRuneInString(strings.TrimLeftFunc(s, unicode.IsSpace))
	return unicode.IsLower(r)
}

func empiezaParrafo(s string) bool {
	r, _ := utf8.DecodeRuneInString(strings.TrimLeftFunc(s, unicode.IsSpace))
	return unicode.IsUpper(r) || unicode.IsDigit(r) || strings.ContainsRune("•·▪◦‣–—-«“\"¿¡*", r)
}

func esViñeta(s string) bool {
	r, _ := utf8.DecodeRuneInString(strings.TrimLeftFunc(s, unicode.IsSpace))
	return strings.ContainsRune("•·▪◦‣■□●○➢►✓", r)
}

func esCJK(r rune) bool {
	return unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) || unicode.Is(unicode.Hangul, r)
}

type marco struct{ izq, der float64 }

// contexto junta lo que se sabe del libro entero mientras se arman los párrafos.
type contexto struct {
	doc     *Documento
	m       metricas
	dosCol  []bool          // página en dos columnas
	mrc     []map[int]marco // márgenes por página y columna
	inter   float64         // interlineado del cuerpo
	niveles map[float64]int // tamaño de letra → nivel de título
}

// columna: 0 para todo el ancho o la izquierda de una página de dos columnas, 1 para la derecha.
func (c *contexto) columna(l *linea) int {
	if !c.dosCol[l.pagina] {
		return 0
	}
	ancho := c.doc.Paginas[l.pagina].Ancho
	if l.x0 >= ancho/2-ancho*0.03 {
		return 1
	}
	return 0
}

func (c *contexto) esCuerpo(l *linea) bool { return math.Abs(l.t-c.m.cuerpo) <= 0.6 }

// medirMarcos calcula, por página y columna, el margen izquierdo del texto (el más repetido) y
// el borde derecho.
func (c *contexto) medirMarcos(paginas [][]*linea) {
	c.mrc = make([]map[int]marco, len(paginas))
	for i, ls := range paginas {
		c.mrc[i] = map[int]marco{}
		porCol := map[int][]*linea{}
		for _, l := range ls {
			if l.imagen == nil && c.esCuerpo(l) {
				porCol[c.columna(l)] = append(porCol[c.columna(l)], l)
			}
		}
		for col, cls := range porCol {
			cuenta := map[float64]int{}
			der := 0.0
			for _, l := range cls {
				cuenta[math.Round(l.x0)]++
				der = math.Max(der, l.x1)
			}
			izq, mejor := 0.0, 0
			for x, n := range cuenta {
				if n > mejor || (n == mejor && x < izq) {
					izq, mejor = x, n
				}
			}
			c.mrc[i][col] = marco{izq, der}
		}
	}
}

// Analizar rearma el libro a partir de lo que extrajo pdf.js.
func Analizar(doc *Documento) *Analisis {
	c := &contexto{doc: doc, m: medirCuerpo(doc), dosCol: make([]bool, len(doc.Paginas))}
	paginas := make([][]*linea, len(doc.Paginas))
	for i := range doc.Paginas {
		paginas[i], c.dosCol[i] = lineasDePagina(doc, i, c.m)
	}
	quitarEncabezados(paginas, doc)
	c.medirMarcos(paginas)

	var todas []*linea
	for _, ls := range paginas {
		todas = append(todas, ls...)
	}
	c.inter = c.medirInterlineado(todas)
	c.niveles = nivelesTitulo(todas, c.m)

	an := &Analisis{Bloques: []Bloque{}}
	var actual []*linea
	actualTitulo := false
	cerrar := func() {
		if len(actual) > 0 {
			an.Bloques = append(an.Bloques, c.armarBloque(actual, actualTitulo))
			actual = nil
		}
	}
	for i, l := range todas {
		if l.imagen != nil {
			cerrar()
			an.Bloques = append(an.Bloques, Bloque{Tipo: "img", Imagen: l.imagen.ID, Ancho: l.imagen.W, Alto: l.imagen.H, Pagina: l.pagina, y: l.y})
			continue
		}
		titulo := c.esTitulo(l, i, todas)
		if len(actual) > 0 {
			ant := actual[len(actual)-1]
			var nuevo bool
			if titulo || actualTitulo {
				// Un título de varias líneas sigue junto si es del mismo tamaño y va pegado.
				nuevo = !(titulo && actualTitulo && l.pagina == ant.pagina && math.Abs(l.t-ant.t) < 0.6 &&
					l.y > ant.y && l.y-ant.y <= 2.2*l.t)
			} else {
				nuevo = c.cortaParrafo(ant, l, actual)
			}
			if nuevo {
				cerrar()
			}
		}
		if len(actual) == 0 {
			actualTitulo = titulo
		}
		actual = append(actual, l)
	}
	cerrar()

	an.Capitulos = sugerirCapitulos(an.Bloques, doc, c.m)
	var textos []string
	caracteres, conImagen := 0, 0
	for _, b := range an.Bloques {
		if b.Tipo == "img" {
			continue
		}
		textos = append(textos, b.Texto)
		an.Palabras += len(strings.Fields(b.Texto))
		caracteres += utf8.RuneCountInString(b.Texto)
	}
	for _, p := range doc.Paginas {
		if len(p.Imagenes) > 0 {
			conImagen++
		}
	}
	n := len(doc.Paginas)
	an.Escaneado = n > 0 && caracteres < 40*n && conImagen*2 >= n
	an.Idioma = detectarIdioma(textos)
	return an
}

func (c *contexto) medirInterlineado(ls []*linea) float64 {
	var difs []float64
	for i := 1; i < len(ls); i++ {
		a, b := ls[i-1], ls[i]
		if a.imagen != nil || b.imagen != nil || a.pagina != b.pagina || !c.esCuerpo(a) || !c.esCuerpo(b) {
			continue
		}
		if d := b.y - a.y; d > 0.5*c.m.cuerpo && d < 2.5*c.m.cuerpo {
			difs = append(difs, d)
		}
	}
	if len(difs) == 0 {
		return 1.2 * c.m.cuerpo
	}
	sort.Float64s(difs)
	return difs[len(difs)/2]
}

// nivelesTitulo asigna h1, h2 y h3 a los tamaños de letra más grandes que el cuerpo.
func nivelesTitulo(ls []*linea, m metricas) map[float64]int {
	tams := map[float64]bool{}
	for _, l := range ls {
		if l.imagen == nil && l.t >= 1.15*m.cuerpo && tieneLetras(l.texto()) {
			tams[math.Round(l.t)] = true
		}
	}
	var lista []float64
	for t := range tams {
		lista = append(lista, t)
	}
	sort.Sort(sort.Reverse(sort.Float64Slice(lista)))
	niveles := map[float64]int{}
	for i, t := range lista {
		niveles[t] = min(i+1, 3)
	}
	return niveles
}

func tieneLetras(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return true
		}
	}
	return false
}

func ultimaLetra(s string) rune {
	r, _ := utf8.DecodeLastRuneInString(s)
	return r
}

// nivelGrande devuelve el nivel de título de una línea por su tamaño de letra (0 si es del cuerpo).
func (c *contexto) nivelGrande(l *linea) int {
	if l.t < 1.15*c.m.cuerpo {
		return 0
	}
	return c.niveles[math.Round(l.t)]
}

// esTitulo: letra más grande que el cuerpo, o una línea corta y suelta en negrita, o «Capítulo 3».
func (c *contexto) esTitulo(l *linea, i int, ls []*linea) bool {
	texto := strings.TrimSpace(l.texto())
	if l.imagen != nil || !tieneLetras(texto) {
		return false
	}
	if c.nivelGrande(l) > 0 {
		return true
	}
	n := utf8.RuneCountInString(texto)
	suelta := func() bool {
		antes := i == 0 || ls[i-1].pagina != l.pagina || ls[i-1].imagen != nil || l.y-ls[i-1].y >= 1.3*c.inter
		despues := i == len(ls)-1 || ls[i+1].pagina != l.pagina || ls[i+1].imagen != nil || ls[i+1].y-l.y >= 1.3*c.inter
		return antes && despues
	}
	if reCapitulo.MatchString(texto) && n <= 60 && suelta() {
		return true
	}
	return l.negrita && !c.m.negrita && n <= 90 && !strings.ContainsRune(".,;", ultimaLetra(texto)) && suelta()
}

// cortaParrafo decide si la línea l empieza un párrafo nuevo después de ant.
func (c *contexto) cortaParrafo(ant, l *linea, parrafo []*linea) bool {
	textoAnt, texto := ant.texto(), l.texto()
	if esViñeta(texto) || math.Abs(l.t-ant.t) > 0.12*ant.t {
		return true
	}
	mismaColumna := l.pagina == ant.pagina && c.columna(l) == c.columna(ant) && l.y > ant.y
	if !mismaColumna {
		// Cambio de página o de columna: sigue el párrafo si la frase quedó a medias.
		return terminaFrase(textoAnt) || !(empiezaMinuscula(texto) || strings.HasSuffix(strings.TrimSpace(textoAnt), "-"))
	}
	if l.y-ant.y > 1.45*c.inter*(l.t/c.m.cuerpo) {
		return true
	}
	mc, ok := c.mrc[l.pagina][c.columna(l)]
	if !ok {
		return false
	}
	em := c.m.cuerpo
	// Sangría de primera línea.
	if l.x0 > mc.izq+0.6*em && ant.x0 <= mc.izq+0.3*em {
		return true
	}
	// Línea anterior corta: el párrafo terminó.
	if ant.x1 < mc.der-1.0*em && terminaFrase(textoAnt) {
		return true
	}
	if ant.x1 < mc.der-3*em && empiezaParrafo(texto) && !centrada(parrafo, mc, c.m) {
		return true
	}
	return false
}

func centrada(ls []*linea, mc marco, m metricas) bool {
	for _, l := range ls {
		izq, der := l.x0-mc.izq, mc.der-l.x1
		if izq < 1.5*m.cuerpo || math.Abs(izq-der) > 1.5*m.cuerpo {
			return false
		}
	}
	return true
}

// armarBloque convierte las líneas de un párrafo o un título en un bloque con su HTML.
func (c *contexto) armarBloque(ls []*linea, titulo bool) Bloque {
	b := Bloque{Tipo: "p", Pagina: ls[0].pagina, y: ls[0].y}
	var tramos []tramo
	for i, l := range ls {
		if i > 0 {
			tramos = unirLineas(tramos, l.tramos)
		} else {
			tramos = append(tramos, l.tramos...)
		}
	}
	texto := strings.Join(strings.Fields(limpiarXML(textoDe(tramos))), " ")
	b.Texto = texto
	if titulo {
		b.Tipo = "h"
		if b.Nivel = c.nivelGrande(ls[0]); b.Nivel == 0 {
			// Títulos en negrita del tamaño del cuerpo: un nivel por debajo de los grandes.
			b.Nivel = min(len(distintos(c.niveles))+1, 3)
		}
		b.capitular = reCapitulo.MatchString(texto) && utf8.RuneCountInString(texto) <= 60
		// Los títulos van sin negritas ni cursivas propias: ya los destaca su estilo.
		b.HTML = html.EscapeString(texto)
		return b
	}
	if mc, ok := c.mrc[b.Pagina][c.columna(ls[0])]; ok {
		b.Centro = len(ls) <= 6 && centrada(ls, mc, c.m)
	}
	b.HTML = htmlDe(tramos, c.m)
	return b
}

func distintos(niveles map[float64]int) map[int]bool {
	d := map[int]bool{}
	for _, n := range niveles {
		d[n] = true
	}
	return d
}

func textoDe(ts []tramo) string {
	var b strings.Builder
	for _, t := range ts {
		b.WriteString(t.s)
	}
	return b.String()
}

// unirLineas pega la línea siguiente: quita el guion de corte de palabra y pone el espacio.
func unirLineas(ts []tramo, sig []tramo) []tramo {
	if len(ts) == 0 || len(sig) == 0 {
		return append(ts, sig...)
	}
	ultimo := &ts[len(ts)-1]
	ultimo.s = strings.TrimRightFunc(ultimo.s, unicode.IsSpace)
	primero := strings.TrimLeftFunc(sig[0].s, unicode.IsSpace)
	r0, _ := utf8.DecodeRuneInString(primero)
	fin, tam := utf8.DecodeLastRuneInString(ultimo.s)
	switch {
	case fin == '­':
		ultimo.s = ultimo.s[:len(ultimo.s)-tam]
	case (fin == '-' || fin == '‐') && unicode.IsLower(r0):
		antes, _ := utf8.DecodeLastRuneInString(ultimo.s[:len(ultimo.s)-tam])
		if unicode.IsLetter(antes) {
			ultimo.s = ultimo.s[:len(ultimo.s)-tam]
		}
	case esCJK(fin) && esCJK(r0):
	default:
		ts = append(ts, tramo{s: " ", negrita: ultimo.negrita && sig[0].negrita, cursiva: ultimo.cursiva && sig[0].cursiva})
	}
	sig = append([]tramo{{s: primero, negrita: sig[0].negrita, cursiva: sig[0].cursiva}}, sig[1:]...)
	return append(ts, sig...)
}

// htmlDe arma el HTML del párrafo: escapa el texto y marca con <strong> y <em> lo que va en
// negrita o cursiva distinto del cuerpo del libro.
func htmlDe(ts []tramo, m metricas) string {
	type pedazo struct {
		s                string
		negrita, cursiva bool
	}
	var ps []pedazo
	for _, t := range ts {
		n, c := t.negrita && !m.negrita, t.cursiva && !m.cursiva
		if strings.TrimSpace(t.s) == "" && len(ps) > 0 {
			// Un espacio no cambia el estilo: queda dentro del pedazo anterior.
			ps[len(ps)-1].s += t.s
			continue
		}
		if k := len(ps); k > 0 && ps[k-1].negrita == n && ps[k-1].cursiva == c {
			ps[k-1].s += t.s
			continue
		}
		ps = append(ps, pedazo{t.s, n, c})
	}
	var b strings.Builder
	for i, p := range ps {
		s := p.s
		if i == 0 {
			s = strings.TrimLeftFunc(s, unicode.IsSpace)
		}
		if i == len(ps)-1 {
			s = strings.TrimRightFunc(s, unicode.IsSpace)
		}
		// El espacio final del pedazo va fuera de las marcas.
		cuerpo := strings.TrimRightFunc(s, unicode.IsSpace)
		resto := s[len(cuerpo):]
		e := html.EscapeString(limpiarXML(strings.Join(strings.Fields(cuerpo), " ")))
		if strings.HasPrefix(cuerpo, " ") && i > 0 {
			b.WriteString(" ")
		}
		switch {
		case e == "":
		case p.negrita && p.cursiva:
			b.WriteString("<strong><em>" + e + "</em></strong>")
		case p.negrita:
			b.WriteString("<strong>" + e + "</strong>")
		case p.cursiva:
			b.WriteString("<em>" + e + "</em>")
		default:
			b.WriteString(e)
		}
		if resto != "" {
			b.WriteString(" ")
		}
	}
	return b.String()
}

// limpiarXML quita los caracteres que no pueden ir en un XML (controles, sustitutos sueltos).
func limpiarXML(s string) string {
	return strings.Map(func(r rune) rune {
		switch {
		case r == '\t' || r == '\n' || r == '\r':
			return ' '
		case r < 0x20, r == 0xFFFE, r == 0xFFFF, r == utf8.RuneError, r >= 0xD800 && r <= 0xDFFF:
			return -1
		case r == '­':
			return -1
		}
		return r
	}, s)
}

// ---------------- Capítulos ----------------

func sugerirCapitulos(bs []Bloque, doc *Documento, m metricas) []CapituloSugerido {
	caps := capitulosDelIndice(bs, doc, m)
	if len(caps) < 2 {
		caps = capitulosDeTitulos(bs)
	}
	if len(caps) < 2 && len(doc.Paginas) > 30 {
		caps = capitulosPorPaginas(bs, 20)
	}
	if len(caps) == 0 && len(bs) > 0 {
		caps = []CapituloSugerido{{Desde: 0, Tipo: "inicio"}}
	}
	if len(caps) > 0 && caps[0].Desde > 0 {
		caps = append([]CapituloSugerido{{Desde: 0, Tipo: "inicio"}}, caps...)
	}
	return caps
}

// capitulosDelIndice usa los marcadores del PDF: el nivel menos profundo que tenga al menos dos.
func capitulosDelIndice(bs []Bloque, doc *Documento, m metricas) []CapituloSugerido {
	if len(bs) == 0 {
		return nil
	}
	porNivel := map[int]int{}
	for _, e := range doc.Indice {
		if e.Pagina >= 0 && strings.TrimSpace(e.Titulo) != "" {
			porNivel[e.Nivel]++
		}
	}
	nivel := -1
	for n := 0; n < 4; n++ {
		if porNivel[n] >= 2 {
			nivel = n
			break
		}
	}
	if nivel < 0 {
		return nil
	}
	var entradas []EntradaIndice
	for _, e := range doc.Indice {
		if e.Nivel == nivel && e.Pagina >= 0 && strings.TrimSpace(e.Titulo) != "" {
			entradas = append(entradas, e)
		}
	}
	sort.SliceStable(entradas, func(i, j int) bool {
		if entradas[i].Pagina != entradas[j].Pagina {
			return entradas[i].Pagina < entradas[j].Pagina
		}
		return entradas[i].Y < entradas[j].Y
	})
	var caps []CapituloSugerido
	for _, e := range entradas {
		desde := -1
		for i, b := range bs {
			if b.Pagina > e.Pagina || (b.Pagina == e.Pagina && b.y >= e.Y-2*m.cuerpo) {
				desde = i
				break
			}
		}
		if desde < 0 {
			continue
		}
		titulo := recortar(strings.Join(strings.Fields(limpiarXML(e.Titulo)), " "), 120)
		if n := len(caps); n > 0 && caps[n-1].Desde == desde {
			continue
		}
		caps = append(caps, CapituloSugerido{Titulo: titulo, Desde: desde})
	}
	return caps
}

// capitulosDeTitulos corta en los títulos del nivel más alto que se repite (y en «Capítulo N»).
func capitulosDeTitulos(bs []Bloque) []CapituloSugerido {
	cuenta := map[int]int{}
	for _, b := range bs {
		if b.Tipo == "h" {
			cuenta[b.Nivel]++
		}
	}
	nivel := 0
	for n := 1; n <= 3; n++ {
		if cuenta[n] >= 2 {
			nivel = n
			break
		}
	}
	var caps []CapituloSugerido
	for i, b := range bs {
		if b.Tipo != "h" || !(b.capitular || (nivel > 0 && b.Nivel <= nivel)) {
			continue
		}
		// «Capítulo 1» seguido de su nombre en otro título: un solo capítulo con los dos.
		if n := len(caps); n > 0 && caps[n-1].Desde == i-1 {
			ant := strings.TrimSpace(caps[n-1].Titulo)
			sep := ". "
			if strings.ContainsRune(finales, ultimaLetra(ant)) {
				sep = " "
			}
			caps[n-1].Titulo = recortar(ant+sep+b.Texto, 120)
			continue
		}
		caps = append(caps, CapituloSugerido{Titulo: recortar(b.Texto, 120), Desde: i})
	}
	return caps
}

func capitulosPorPaginas(bs []Bloque, cada int) []CapituloSugerido {
	var caps []CapituloSugerido
	sig := 0
	for i, b := range bs {
		if b.Pagina >= sig {
			caps = append(caps, CapituloSugerido{Desde: i, Tipo: "paginas"})
			sig = (b.Pagina/cada + 1) * cada
		}
	}
	return caps
}

func recortar(s string, n int) string {
	if utf8.RuneCountInString(s) <= n {
		return s
	}
	r := []rune(s)
	return strings.TrimSpace(string(r[:n-1])) + "…"
}

// ---------------- Idioma ----------------

var palabrasComunes = map[string][]string{
	"es": {"de", "la", "que", "el", "en", "los", "se", "del", "las", "por", "un", "una", "con", "para", "es", "su", "no", "lo", "como", "más"},
	"en": {"the", "of", "and", "to", "in", "is", "that", "it", "was", "for", "with", "as", "he", "on", "be", "at", "by", "this", "had", "not"},
	"fr": {"de", "la", "le", "et", "les", "des", "en", "un", "du", "une", "que", "est", "pour", "qui", "dans", "pas", "au", "sur", "il", "ne"},
	"de": {"der", "die", "und", "in", "den", "von", "zu", "das", "mit", "sich", "des", "auf", "für", "ist", "im", "nicht", "ein", "eine", "als", "auch"},
	"pt": {"de", "a", "o", "que", "e", "do", "da", "em", "um", "para", "com", "não", "uma", "os", "no", "se", "na", "por", "mais", "as"},
	"it": {"di", "che", "il", "la", "e", "per", "un", "in", "non", "una", "del", "della", "sono", "si", "con", "le", "da", "lo", "gli", "anche"},
	"ca": {"de", "la", "que", "el", "i", "en", "els", "les", "per", "amb", "una", "del", "és", "un", "va", "no", "als", "més", "seu", "com"},
	"nl": {"de", "het", "een", "en", "van", "in", "is", "op", "dat", "te", "zijn", "met", "voor", "niet", "die", "aan", "er", "ook", "als", "bij"},
	"ru": {"и", "в", "не", "на", "что", "с", "он", "как", "по", "это", "она", "к", "но", "его", "из", "у", "за", "то", "от", "так"},
}

// detectarIdioma cuenta palabras muy comunes de cada idioma; para chino y japonés mira la escritura.
func detectarIdioma(textos []string) string {
	conjuntos := map[string]map[string]bool{}
	for id, ps := range palabrasComunes {
		conjuntos[id] = map[string]bool{}
		for _, p := range ps {
			conjuntos[id][p] = true
		}
	}
	puntos := map[string]int{}
	var han, kana, total int
	palabras := 0
	for _, t := range textos {
		for _, r := range t {
			switch {
			case unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r):
				kana++
			case unicode.Is(unicode.Han, r):
				han++
			}
			if !unicode.IsSpace(r) {
				total++
			}
		}
		for _, w := range strings.FieldsFunc(strings.ToLower(t), func(r rune) bool { return !unicode.IsLetter(r) }) {
			for id, c := range conjuntos {
				if c[w] {
					puntos[id]++
				}
			}
			palabras++
		}
		if palabras > 30000 {
			break
		}
	}
	if total > 0 && kana*20 > total {
		return "ja"
	}
	if total > 0 && han*5 > total {
		return "zh"
	}
	mejor, id := 0, ""
	for _, k := range []string{"es", "en", "fr", "de", "pt", "it", "ca", "nl", "ru"} {
		if puntos[k] > mejor {
			mejor, id = puntos[k], k
		}
	}
	return id
}
