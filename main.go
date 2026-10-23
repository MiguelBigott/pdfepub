// PdfEpub — convierte libros PDF a EPUB, con portada, datos y capítulos editables.
// Servidor local + interfaz web embebida en una ventana nativa (WebView2). pdf.js lee el PDF
// dentro de la interfaz; Go rearma el texto y escribe el EPUB.
package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	webview2 "github.com/jchv/go-webview2"
)

const Version = "1.1.2"

// Puerto fijo: sirve también para detectar si la app ya está abierta.
var puerto = "127.0.0.1:47834"

// Topes: una imagen de página o de portada, y el texto extraído de un libro muy largo.
const (
	maxBytesImagen   = 20 << 20
	maxBytesAnalisis = 128 << 20
)

//go:embed web
var webFS embed.FS

type App struct {
	mu       sync.Mutex
	cfg      Config
	cfgPath  string
	dataDir  string
	recursos *Recursos
	analisis *Analisis
	ultimo   string // el último EPUB creado
	subidas  chan struct{}
}

type Config struct {
	Carpeta string `json:"carpeta"`
	Modo    string `json:"modo"`
}

func main() {
	dataDir := carpetaDatos()
	if d := os.Getenv("PDFEPUB_DATOS"); d != "" {
		dataDir = d
	}
	if p := os.Getenv("PDFEPUB_PUERTO"); p != "" {
		puerto = "127.0.0.1:" + p
	}
	_ = os.MkdirAll(dataDir, 0o755)
	logFile, err := os.OpenFile(filepath.Join(dataDir, "pdfepub.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err == nil {
		log.SetOutput(logFile)
	}

	ln, err := net.Listen("tcp", puerto)
	if err != nil {
		// Ya hay una instancia abierta: traemos su ventana al frente.
		traerAlFrente()
		return
	}

	app := &App{dataDir: dataDir, cfgPath: filepath.Join(dataDir, "config.json"), subidas: make(chan struct{}, 2)}
	app.recursos = NuevosRecursos(filepath.Join(dataDir, "libro"))
	app.cargarConfig()

	// pdf.js se carga como módulo: Windows a veces no conoce .mjs y lo serviría como texto.
	_ = mime.AddExtensionType(".mjs", "text/javascript; charset=utf-8")
	_ = mime.AddExtensionType(".wasm", "application/wasm")
	mux := http.NewServeMux()
	sub, _ := fs.Sub(webFS, "web")
	mux.Handle("/", http.FileServer(http.FS(sub)))
	if d := os.Getenv("PDFEPUB_PRUEBAS"); d != "" {
		// Solo para probar la interfaz en un navegador: sirve los PDF de una carpeta.
		mux.Handle("/pruebas/", http.StripPrefix("/pruebas/", http.FileServer(http.Dir(d))))
	}
	app.rutas(mux)

	go func() {
		server := &http.Server{
			Handler:           seguridadHTTP(mux),
			ReadHeaderTimeout: 5 * time.Second,
			IdleTimeout:       60 * time.Second,
			MaxHeaderBytes:    1 << 20,
		}
		if err := server.Serve(ln); err != nil {
			log.Println("servidor:", err)
		}
	}()

	url := "http://" + puerto + "/"
	if os.Getenv("PDFEPUB_SIN_VENTANA") != "" {
		log.Println("modo prueba, escuchando en", url)
		select {}
	}

	ancho, alto := tamanoInicial()
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		AutoFocus: true,
		DataPath:  filepath.Join(dataDir, "webview"),
		WindowOptions: webview2.WindowOptions{
			Title: tituloVentana, Width: ancho, Height: alto, Center: true, IconId: idIcono,
		},
	})
	if w == nil {
		mensaje(textosDe(idiomaApp()).FaltaWebView)
		return
	}
	defer w.Destroy()
	prepararVentana(uintptr(w.Window()))
	w.SetSize(980, 660, webview2.HintMin)
	_ = w.Bind("caArrastrar", ventanaArrastrar)
	_ = w.Bind("caRedimensionar", ventanaRedimensionar)
	_ = w.Bind("caMinimizar", ventanaMinimizar)
	_ = w.Bind("caMaximizar", ventanaMaximizar)
	_ = w.Bind("caMaximizada", ventanaMaximizada)
	_ = w.Bind("caCerrar", ventanaCerrar)
	w.Navigate(url)
	w.Run()

	time.Sleep(200 * time.Millisecond)
	_ = os.RemoveAll(app.recursos.dir)
}

func (a *App) cargarConfig() {
	a.cfg = Config{Carpeta: carpetaSalidaPorDefecto(), Modo: "fluido"}
	b, err := os.ReadFile(a.cfgPath)
	if err == nil {
		_ = json.Unmarshal(b, &a.cfg)
	}
	if a.cfg.Carpeta == "" {
		a.cfg.Carpeta = carpetaSalidaPorDefecto()
	}
	if a.cfg.Modo != "fijo" {
		a.cfg.Modo = "fluido"
	}
}

func (a *App) guardarConfig() {
	b, _ := json.MarshalIndent(a.cfg, "", "  ")
	_ = os.WriteFile(a.cfgPath, b, 0o644)
}

func (a *App) config() Config {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.cfg
}

// ---------------- Crear el EPUB ----------------

type Pedido struct {
	Modo      string          `json:"modo"`
	Datos     Metadatos       `json:"datos"`
	Portada   string          `json:"portada"`
	Capitulos []CapituloFinal `json:"capitulos"`
	Paginas   []string        `json:"paginas"`
}

var reProhibidos = regexp.MustCompile(`[<>:"/\\|?*\x00-\x1f]+`)

// nombreArchivo arma «Autor - Título.epub» sin caracteres que Windows no acepta.
func nombreArchivo(d Metadatos) string {
	limpio := func(s string) string {
		s = reProhibidos.ReplaceAllString(s, " ")
		return strings.Trim(strings.Join(strings.Fields(s), " "), ". ")
	}
	nombre := limpio(d.Titulo)
	if nombre == "" {
		nombre = "libro"
	}
	if autor := limpio(strings.Join(autores(d.Autor), ", ")); autor != "" {
		nombre = autor + " - " + nombre
	}
	if utf8.RuneCountInString(nombre) > 150 {
		nombre = strings.TrimSpace(string([]rune(nombre)[:150]))
	}
	// Nombres reservados de Windows (CON, PRN, AUX...).
	if regexp.MustCompile(`(?i)^(con|prn|aux|nul|com\d|lpt\d)$`).MatchString(nombre) {
		nombre = "_" + nombre
	}
	return nombre
}

// rutaLibre no pisa un EPUB que ya existe: agrega « (2)», « (3)»...
func rutaLibre(carpeta, nombre string) string {
	ruta := filepath.Join(carpeta, nombre+".epub")
	for i := 2; ; i++ {
		if _, err := os.Stat(ruta); os.IsNotExist(err) {
			return ruta
		}
		ruta = filepath.Join(carpeta, fmt.Sprintf("%s (%d).epub", nombre, i))
	}
}

func (a *App) crear(p Pedido) (string, int64, error) {
	a.mu.Lock()
	an := a.analisis
	carpeta := a.cfg.Carpeta
	a.mu.Unlock()

	imagenes := a.recursos.Todas()
	lb := &Libro{Datos: p.Datos, Fijo: p.Modo == "fijo", Capitulos: p.Capitulos, Imagenes: imagenes}
	if p.Portada != "" {
		if im, ok := imagenes[p.Portada]; ok {
			lb.Portada = &im
		}
	}
	if lb.Fijo {
		for _, id := range p.Paginas {
			im, ok := imagenes[id]
			if !ok {
				return "", 0, msj("faltaPagina")
			}
			lb.Paginas = append(lb.Paginas, im)
		}
		if len(lb.Paginas) == 0 {
			return "", 0, msj("libroVacio")
		}
	} else {
		if an == nil || len(an.Bloques) == 0 {
			return "", 0, msj("libroVacio")
		}
		lb.Bloques = an.Bloques
		css, _ := webFS.ReadFile("web/libro.css")
		lb.Estilo = string(css)
	}

	if err := os.MkdirAll(carpeta, 0o755); err != nil {
		return "", 0, msj("carpetaNoCreada").conCausa(err)
	}
	ruta := rutaLibre(carpeta, nombreArchivo(p.Datos))
	tmp, err := os.CreateTemp(carpeta, ".pdfepub-*.tmp")
	if err != nil {
		return "", 0, msj("noGuardarEpub").conCausa(err)
	}
	err = EscribirEPUB(tmp, lb)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = os.Rename(tmp.Name(), ruta)
	}
	if err != nil {
		_ = os.Remove(tmp.Name())
		return "", 0, msj("noGuardarEpub").conCausa(err)
	}
	info, _ := os.Stat(ruta)
	a.mu.Lock()
	a.ultimo = ruta
	a.mu.Unlock()
	return ruta, info.Size(), nil
}

// ---------------- API ----------------

func responder(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	_ = json.NewEncoder(w).Encode(v)
}

// fallar responde con un error que la interfaz muestra en su idioma.
func fallar(w http.ResponseWriter, status int, m *Mensaje) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]*Mensaje{"error": m})
}

func leerJSON(w http.ResponseWriter, r *http.Request, v any, tope int64) error {
	if r.Method != http.MethodPost {
		return msj("peticionInvalida")
	}
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, tope)).Decode(v); err != nil {
		return msj("peticionInvalida")
	}
	return nil
}

// Solo acepta la interfaz local y exige Origin exacto en todas las operaciones que cambian estado.
func soloLocal(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Host != puerto {
			fallar(w, http.StatusForbidden, msj("accesoDenegado"))
			return
		}
		origen := r.Header.Get("Origin")
		if (r.Method == http.MethodPost && origen != "http://"+puerto) ||
			(r.Method != http.MethodPost && origen != "" && origen != "http://"+puerto) {
			fallar(w, http.StatusForbidden, msj("accesoDenegado"))
			return
		}
		h(w, r)
	}
}

func seguridadHTTP(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; worker-src 'self' blob:; connect-src 'self' blob:; img-src 'self' data: blob:; style-src 'self' 'unsafe-inline'; font-src 'self' data:; object-src 'none'; base-uri 'none'; frame-ancestors 'none'")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Frame-Options", "DENY")
		h.ServeHTTP(w, r)
	})
}

func (a *App) rutas(mux *http.ServeMux) {
	mux.HandleFunc("/api/estado", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		responder(w, map[string]any{
			"version": Version,
			"idioma":  idiomaApp(),
			"config":  a.config(),
		})
	}))

	// nuevo empieza otro libro: borra las imágenes y el texto del anterior.
	mux.HandleFunc("/api/nuevo", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fallar(w, http.StatusMethodNotAllowed, msj("peticionInvalida"))
			return
		}
		a.recursos.Vaciar()
		a.mu.Lock()
		a.analisis = nil
		a.mu.Unlock()
		responder(w, map[string]bool{"ok": true})
	}))

	// recurso recibe una imagen (POST, el archivo tal cual) o la sirve (GET ?id=).
	mux.HandleFunc("/api/recurso", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet {
			im, ok := a.recursos.Ver(r.URL.Query().Get("id"))
			if !ok {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Cache-Control", "no-store")
			http.ServeFile(w, r, im.Ruta)
			return
		}
		if r.Method != http.MethodPost {
			fallar(w, http.StatusMethodNotAllowed, msj("peticionInvalida"))
			return
		}
		select {
		case a.subidas <- struct{}{}:
			defer func() { <-a.subidas }()
		default:
			fallar(w, http.StatusTooManyRequests, msj("demasiadoGrande", "nombre", "imagen"))
			return
		}
		id, im, err := a.recursos.Guardar(http.MaxBytesReader(w, r.Body, maxBytesImagen))
		if err != nil {
			fallar(w, http.StatusBadRequest, comoMensaje(err))
			return
		}
		responder(w, map[string]any{"id": id, "ancho": im.Ancho, "alto": im.Alto})
	}))

	mux.HandleFunc("/api/analizar", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		var doc Documento
		if err := leerJSON(w, r, &doc, maxBytesAnalisis); err != nil {
			fallar(w, http.StatusBadRequest, comoMensaje(err))
			return
		}
		an := Analizar(&doc)
		a.mu.Lock()
		a.analisis = an
		a.mu.Unlock()
		responder(w, an)
	}))

	mux.HandleFunc("/api/crear", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		var p Pedido
		if err := leerJSON(w, r, &p, 16<<20); err != nil {
			fallar(w, http.StatusBadRequest, comoMensaje(err))
			return
		}
		ruta, peso, err := a.crear(p)
		if err != nil {
			log.Println("crear:", err)
			fallar(w, http.StatusInternalServerError, comoMensaje(err))
			return
		}
		responder(w, map[string]any{"archivo": ruta, "nombre": filepath.Base(ruta), "peso": peso})
	}))

	mux.HandleFunc("/api/modo", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Modo string `json:"modo"`
		}
		if err := leerJSON(w, r, &req, 1<<10); err != nil || (req.Modo != "fluido" && req.Modo != "fijo") {
			fallar(w, http.StatusBadRequest, msj("peticionInvalida"))
			return
		}
		a.mu.Lock()
		a.cfg.Modo = req.Modo
		a.guardarConfig()
		a.mu.Unlock()
		responder(w, map[string]bool{"ok": true})
	}))

	mux.HandleFunc("/api/carpeta/elegir", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			fallar(w, http.StatusMethodNotAllowed, msj("peticionInvalida"))
			return
		}
		actual := a.config().Carpeta
		elegida, err := elegirCarpeta(textosDe(idiomaApp()).ElegirCarpeta, actual)
		if err != nil || elegida == "" {
			responder(w, map[string]any{"carpeta": actual, "cambiada": false})
			return
		}
		a.mu.Lock()
		a.cfg.Carpeta = elegida
		a.guardarConfig()
		a.mu.Unlock()
		responder(w, map[string]any{"carpeta": elegida, "cambiada": true})
	}))

	// carpeta/abrir abre la carpeta de los EPUB; con «ultimo», muestra el último creado.
	mux.HandleFunc("/api/carpeta/abrir", soloLocal(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Ultimo bool `json:"ultimo"`
		}
		if err := leerJSON(w, r, &req, 1<<10); err != nil {
			fallar(w, http.StatusBadRequest, comoMensaje(err))
			return
		}
		a.mu.Lock()
		ultimo := a.ultimo
		a.mu.Unlock()
		if req.Ultimo && ultimo != "" {
			if _, err := os.Stat(ultimo); err == nil {
				mostrarArchivo(ultimo)
				responder(w, map[string]bool{"ok": true})
				return
			}
		}
		c := a.config().Carpeta
		_ = os.MkdirAll(c, 0o755)
		abrirCarpeta(c)
		responder(w, map[string]bool{"ok": true})
	}))
}
