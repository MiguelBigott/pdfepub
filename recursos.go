package main

// Recursos del libro en curso: las imágenes que manda la interfaz (la portada, las imágenes
// recortadas del PDF y las páginas fijas). Se guardan en una carpeta temporal que se vacía al
// abrir otro PDF y al cerrar la app.

import (
	"bytes"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"os"
	"path/filepath"
	"sync"
)

const (
	maxPixelesRecurso int64 = 40_000_000
	maxBytesRecursos  int64 = 512 << 20
)

type Recursos struct {
	mu      sync.Mutex
	dir     string
	n       int
	bytes   int64
	subidas chan struct{}
	lista   map[string]Imagen
}

func NuevosRecursos(dir string) *Recursos {
	r := &Recursos{dir: dir, subidas: make(chan struct{}, 2)}
	r.Vaciar()
	return r
}

// Vaciar borra todo lo del libro anterior.
func (r *Recursos) Vaciar() {
	r.mu.Lock()
	defer r.mu.Unlock()
	_ = os.RemoveAll(r.dir)
	_ = os.MkdirAll(r.dir, 0o755)
	r.lista = map[string]Imagen{}
	r.bytes = 0
}

func tipoImagen(cabecera []byte) string {
	switch {
	case bytes.HasPrefix(cabecera, []byte{0xFF, 0xD8, 0xFF}):
		return "image/jpeg"
	case bytes.HasPrefix(cabecera, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png"
	}
	return ""
}

// Guardar recibe una imagen JPEG o PNG y devuelve su id.
func (r *Recursos) Guardar(cuerpo io.Reader) (string, Imagen, error) {
	select {
	case r.subidas <- struct{}{}:
		defer func() { <-r.subidas }()
	default:
		return "", Imagen{}, msj("demasiadoGrande", "nombre", "imagen")
	}
	datos, err := io.ReadAll(cuerpo)
	if err != nil {
		return "", Imagen{}, msj("demasiadoGrande")
	}
	tipo := tipoImagen(datos)
	if tipo == "" {
		return "", Imagen{}, msj("noEsImagen")
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(datos))
	if err != nil || cfg.Width <= 0 || cfg.Height <= 0 {
		return "", Imagen{}, msj("noEsImagen")
	}
	if cfg.Width > 50_000 || cfg.Height > 50_000 || int64(cfg.Width) > maxPixelesRecurso/int64(cfg.Height) {
		return "", Imagen{}, msj("demasiadoGrande", "nombre", "imagen")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if int64(len(datos)) > maxBytesRecursos-r.bytes {
		return "", Imagen{}, msj("demasiadoGrande", "nombre", "recursos")
	}
	id := fmt.Sprintf("r%d", r.n+1)
	im := Imagen{Ruta: filepath.Join(r.dir, id+map[string]string{"image/jpeg": ".jpg", "image/png": ".png"}[tipo]), Tipo: tipo, Ancho: cfg.Width, Alto: cfg.Height}
	if err := os.WriteFile(im.Ruta, datos, 0o644); err != nil {
		return "", Imagen{}, msj("noGuardarImagen").conCausa(err)
	}
	r.n++
	r.bytes += int64(len(datos))
	r.lista[id] = im
	return id, im, nil
}

func (r *Recursos) Ver(id string) (Imagen, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	im, ok := r.lista[id]
	return im, ok
}

// Todas devuelve una copia de la lista, para escribir el EPUB sin tener el candado.
func (r *Recursos) Todas() map[string]Imagen {
	r.mu.Lock()
	defer r.mu.Unlock()
	c := make(map[string]Imagen, len(r.lista))
	for k, v := range r.lista {
		c[k] = v
	}
	return c
}
