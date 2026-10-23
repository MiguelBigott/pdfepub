package main

import (
	"strings"
	"testing"
)

// Una letra capitular de tres renglones se pega al primero, y los renglones que tiene al lado
// siguen en el mismo párrafo.
func TestCapitular(t *testing.T) {
	// Texto justificado: cada renglón llega al margen derecho (x = 360).
	renglon := func(s string, x, y float64) Trozo { return Trozo{S: s, X: x, Y: y, W: 360 - x, T: 10} }
	doc := &Documento{Paginas: []PaginaPDF{{Ancho: 420, Alto: 595, Textos: []Trozo{
		{S: "Y", X: 50, Y: 128, W: 22, T: 34},
		renglon("a sabéis que jamás os metería anuncios por", 76, 106),
		renglon("vuestros globos oculares, ¿verdad? ¡PUES", 76, 118),
		renglon("BIEN, os ofrece camisetas serigrafiadas", 76, 130),
		renglon("esta semana! Podéis encontrarlas en tiendas de", 50, 142),
		renglon("Kabuki. Lo único que tenéis que hacer es gritar.", 50, 154),
		renglon("Otro párrafo empieza aquí con su sangría y sigue", 65, 166),
		renglon("hasta el final de la página sin más problemas.", 50, 178),
	}}}}
	an := Analizar(doc)
	if len(an.Bloques) != 2 {
		for _, b := range an.Bloques {
			t.Log(b.Tipo, b.Texto)
		}
		t.Fatalf("bloques = %d, quería 2", len(an.Bloques))
	}
	p := an.Bloques[0].Texto
	if !strings.HasPrefix(p, "Ya sabéis que jamás") {
		t.Errorf("el párrafo empieza con %q", p[:30])
	}
	if !strings.Contains(p, "serigrafiadas esta semana") || !strings.HasSuffix(p, "hacer es gritar.") {
		t.Errorf("párrafo = %q", p)
	}
}

// Un título que es solo el número del capítulo («4») también corta capítulos.
func TestTituloNumero(t *testing.T) {
	pagina := func(num string) PaginaPDF {
		return PaginaPDF{Ancho: 420, Alto: 595, Textos: []Trozo{
			{S: num, X: 205, Y: 80, W: 8, T: 14},
			{S: "Texto del capítulo, con varias palabras para que sea cuerpo.", X: 50, Y: 120, W: 300, T: 10},
			{S: "Y otra línea más del mismo párrafo, que sigue hasta aquí.", X: 50, Y: 132, W: 280, T: 10},
		}}
	}
	an := Analizar(&Documento{Paginas: []PaginaPDF{pagina("3"), pagina("4")}})
	if len(an.Capitulos) != 2 || an.Capitulos[0].Titulo != "3" || an.Capitulos[1].Titulo != "4" {
		t.Errorf("capítulos = %+v", an.Capitulos)
	}
}
