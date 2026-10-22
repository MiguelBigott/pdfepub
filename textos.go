package main

// Los mensajes visibles de la interfaz viven en web/i18n.js. Go envía claves y datos; los
// cuadros nativos usan el idioma compatible preferido en Windows o la preferencia de Codex.

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"golang.org/x/sys/windows"
)

// Mensaje es un texto para la interfaz: una clave de web/i18n.js, sus datos y, si lo hay, el
// error de fondo (Causa), que se traduce dentro de la frase. También sirve como error.
type Mensaje struct {
	Clave string            `json:"clave"`
	Datos map[string]string `json:"datos,omitempty"`
	Causa *Mensaje          `json:"causa,omitempty"`
}

func (m *Mensaje) Error() string { return m.Clave }

// msj arma un Mensaje; los datos van en pares nombre, valor.
func msj(clave string, datos ...string) *Mensaje {
	m := &Mensaje{Clave: clave}
	if len(datos) > 0 {
		m.Datos = map[string]string{}
		for i := 0; i+1 < len(datos); i += 2 {
			m.Datos[datos[i]] = datos[i+1]
		}
	}
	return m
}

// conCausa agrega el error de fondo, que se muestra traducido dentro del mensaje.
func (m *Mensaje) conCausa(err error) *Mensaje {
	m.Causa = comoMensaje(err)
	return m
}

// comoMensaje convierte cualquier error en un Mensaje: los que no vienen del diccionario
// (errores del sistema) se muestran tal cual.
func comoMensaje(err error) *Mensaje {
	var m *Mensaje
	if errors.As(err, &m) {
		return m
	}
	return msj("errorDetalle", "detalle", err.Error())
}

// idiomas que sabe mostrar la app: los mismos de Codex.
var idiomas = []string{"es", "en", "fr", "de", "ru", "pt", "zh", "ja"}

// idiomaSistema es el primero de los idiomas de Windows que la app sabe mostrar; si ninguno, inglés.
func idiomaSistema() string {
	lista, _ := windows.GetUserPreferredUILanguages(windows.MUI_LANGUAGE_NAME)
	for _, l := range lista {
		if base := strings.ToLower(strings.SplitN(l, "-", 2)[0]); slices.Contains(idiomas, base) {
			return base
		}
	}
	return "en"
}

// idiomaApp es el idioma elegido en Codex (%LOCALAPPDATA%\Codex\ajustes.json); en automático o
// sin Codex, el del PC. Se lee en cada consulta: si cambia en Codex, la app cambia sola.
func idiomaApp() string {
	if b, err := os.ReadFile(filepath.Join(os.Getenv("LOCALAPPDATA"), "Codex", "ajustes.json")); err == nil {
		var a struct {
			Idioma string `json:"idioma"`
		}
		if json.Unmarshal(b, &a) == nil && slices.Contains(idiomas, a.Idioma) {
			return a.Idioma
		}
	}
	return idiomaSistema()
}

// Textos que Go muestra por su cuenta, fuera de la interfaz.
type textosNativos struct{ FaltaWebView, ElegirCarpeta string }

var nativos = map[string]textosNativos{
	"es": {"PdfEpub necesita Microsoft Edge WebView2, que viene con Windows 11. Instálalo desde https://developer.microsoft.com/microsoft-edge/webview2/ y vuelve a abrir PdfEpub.", "Elige dónde guardar los libros EPUB"},
	"en": {"PdfEpub needs Microsoft Edge WebView2, which comes with Windows 11. Install it from https://developer.microsoft.com/microsoft-edge/webview2/ and open PdfEpub again.", "Choose where to save the EPUB books"},
	"fr": {"PdfEpub a besoin de Microsoft Edge WebView2, inclus dans Windows 11. Installez-le depuis https://developer.microsoft.com/microsoft-edge/webview2/ puis rouvrez PdfEpub.", "Choisissez où enregistrer les livres EPUB"},
	"de": {"PdfEpub benötigt Microsoft Edge WebView2, das in Windows 11 enthalten ist. Installiere es von https://developer.microsoft.com/microsoft-edge/webview2/ und öffne PdfEpub erneut.", "Wähle, wo die EPUB-Bücher gespeichert werden"},
	"ru": {"Для работы PdfEpub нужен Microsoft Edge WebView2, который входит в Windows 11. Установите его с https://developer.microsoft.com/microsoft-edge/webview2/ и снова откройте PdfEpub.", "Выберите, куда сохранять книги EPUB"},
	"pt": {"O PdfEpub precisa do Microsoft Edge WebView2, que vem com o Windows 11. Instale-o em https://developer.microsoft.com/microsoft-edge/webview2/ e abra o PdfEpub novamente.", "Escolha onde salvar os livros EPUB"},
	"zh": {"PdfEpub 需要 Microsoft Edge WebView2（Windows 11 已自带）。请从 https://developer.microsoft.com/microsoft-edge/webview2/ 安装后重新打开 PdfEpub。", "选择 EPUB 电子书的保存位置"},
	"ja": {"PdfEpub には Microsoft Edge WebView2 が必要です（Windows 11 に付属しています）。https://developer.microsoft.com/microsoft-edge/webview2/ からインストールして、PdfEpub をもう一度開いてください。", "EPUB ブックの保存先を選択"},
}

func textosDe(idioma string) textosNativos {
	if t, ok := nativos[idioma]; ok {
		return t
	}
	return nativos["en"]
}
