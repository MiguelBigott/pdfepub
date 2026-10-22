/* La interfaz se mantiene en español; el idioma seleccionado aquí también se guarda en los
   metadatos del EPUB y puede ser distinto al idioma de la interfaz. */
const textos = {
  cerrar: "Cerrar", maximizar: "Maximizar", minimizar: "Minimizar", restaurar: "Restaurar",
  soltarTitulo: "Suelta aquí un libro en PDF", soltarElige: "o elige", unArchivo: "un archivo",
  modoFluido: "texto fluido", modoFijo: "páginas fijas",
  notaFluido: "El texto se adapta a la pantalla y al tamaño de letra del lector. Ideal para novelas, ensayos y apuntes.",
  notaFijo: "Cada página queda idéntica al PDF. Ideal para cómics, revistas, libros ilustrados y escaneados.",
  privado: "Todo ocurre en tu PC: el PDF no sale del equipo.", libro: "libro", abrirOtro: "abrir otro pdf",
  secDatos: "datos del libro", secPortada: "portada", secCapitulos: "capítulos", conversion: "conversión",
  modoFluidoCorto: "se adapta a la pantalla", modoFijoCorto: "idéntico al pdf", nitidez: "nitidez",
  nitidezNormal: "normal", nitidezAlta: "alta", seGuarda: "se guarda en", cambiar: "cambiar",
  abrirCarpeta: "abrir carpeta", crearEpub: "crear epub", estListo: "listo", mostrar: "mostrar",
  ayudaDatos: "Así aparece el libro en tu lector y en tu biblioteca.", campoTitulo: "título", campoAutor: "autor",
  ayudaAutor: "Si son varios, sepáralos con punto y coma.", campoIdioma: "idioma", campoFecha: "fecha",
  ayudaFecha: "Año, o año-mes-día.", campoEditorial: "editorial", campoIsbn: "isbn", campoSerie: "serie",
  campoNumero: "número en la serie", campoEtiquetas: "etiquetas", ayudaEtiquetas: "Géneros o temas, separados por comas.",
  campoDescripcion: "descripción", ayudaPortada: "Arrastra la imagen para encuadrarla.", fondo: "fondo",
  fondoPagina: "página del pdf", fondoImagen: "imagen", fondoColor: "color", elegirImagen: "elegir imagen",
  oSueltala: "o suéltala sobre la portada", ajusteCubrir: "cubrir", ajusteEntera: "entera", zoom: "zoom",
  oscurecer: "oscurecer", textoPortada: "texto", campoSubtitulo: "subtítulo", colorTexto: "letra",
  colorAcento: "detalle", tamanoTexto: "tamaño", plNinguna: "sin texto", plClasica: "clásica",
  plModerna: "moderna", plBanda: "banda", plMarco: "marco", restablecer: "restablecer portada",
  pieMotor: "lectura de pdf con pdf.js de mozilla", piePrivado: "todo ocurre en tu pc", cancelar: "cancelar",
  claveTitulo: "Este PDF tiene contraseña", claveAyuda: "Escríbela para abrirlo.", abrir: "abrir",
  sueltaPdf: "Suelta el PDF para abrirlo", vistaPrevia: "Vista previa",

  cargando: "Abriendo el libro…", leyendo: "Leyendo el PDF", preparando: "Preparando el libro",
  paginaDe: "Página {n} de {total}", preparandoCapitulos: "Organizando el texto y detectando capítulos…",
  escaneado: "Este PDF parece escaneado. Para conservar su aspecto, elige páginas fijas.",
  sinTexto: "No se encontró texto seleccionable. Puedes conservar las páginas como imágenes.",
  datosCapitulos: "{n} capítulos · {paginas} páginas", paginas: "{n} páginas", palabras: "{n} palabras",
  inicio: "Inicio", paginasCapitulo: "Páginas", capitulo: "Capítulo {n}",
  ayudaCapitulosFluido: "Elige un párrafo de la vista previa para dividir el capítulo. Puedes renombrar, unir y excluir capítulos.",
  ayudaCapitulosFijo: "Selecciona una página para crear un capítulo desde ahí. Puedes renombrar, unir y excluir capítulos.",
  dividirAntes: "dividir antes del párrafo elegido", dividirPagina: "crear capítulo desde esta página",
  seleccionaParrafo: "Selecciona un párrafo en la vista previa.", paginaSeleccionada: "Página {n} seleccionada",
  unirAnterior: "unir con el anterior", incluirCapitulo: "Incluir capítulo", excluirCapitulo: "Excluir capítulo",
  sinCapitulos: "No hay capítulos. Añade uno desde la vista previa.", capituloCreado: "Capítulo añadido.",
  limiteCapitulos: "Ya existe un capítulo en ese punto.", sinContenido: "Incluye al menos un capítulo con contenido.",
  tituloRequerido: "Escribe un título para el libro.", guardandoPortada: "Preparando la portada…",
  guardandoPaginas: "Preparando las páginas", creandoEpub: "Creando el EPUB…", epubCreado: "EPUB creado.",
  abriendoCarpeta: "Abriendo la carpeta…", rutaCopiada: "No se pudo abrir la ubicación del libro.",
  errorPdf: "No se pudo abrir este PDF: {detalle}", errorGeneral: "No se pudo completar la operación: {detalle}",
  archivoIncorrecto: "Elige un archivo PDF.", pdfVacio: "El PDF no contiene páginas.", claveIncorrecta: "La contraseña no es correcta. Inténtalo de nuevo.",
  errorDetalle: "{detalle}", accesoDenegado: "La solicitud local fue rechazada.", peticionInvalida: "La solicitud no es válida.",
  carpetaNoCreada: "No se pudo crear la carpeta de destino.", demasiadoGrande: "El archivo es demasiado grande.",
  faltaPagina: "Falta una página del libro. Vuelve a abrir el PDF e inténtalo de nuevo.",
  libroVacio: "No hay contenido para incluir en el EPUB.", noEsImagen: "El archivo no es una imagen PNG o JPEG válida.",
  noGuardarEpub: "No se pudo guardar el EPUB.", noGuardarImagen: "No se pudo guardar la imagen.",
  elegirImagenError: "No se pudo abrir la imagen: {detalle}",
  idioma_es: "Español", idioma_en: "Inglés", idioma_fr: "Francés", idioma_de: "Alemán",
  idioma_pt: "Portugués", idioma_it: "Italiano", idioma_zh: "Chino", idioma_ja: "Japonés",
  idioma_ru: "Ruso", idioma_und: "Sin especificar", escaneadoEstado: "PDF escaneado",
  modoFluidoNota: "El EPUB se adapta a distintos lectores.", modoFijoNota: "Cada página se conserva como imagen.",
  paginaPdf: "Página {n}", tituloPdf: "PDF", seleccion: "selección", version: "versión {version}",
  original: "original", asiSeLee: "así se lee", etqPdf: "pdf · página {n}", etqEpub: "epub · {titulo}",
  paginasDelPdf: "páginas del pdf", eligePagina: "elige una",
  sinAutor: "sin autor", portadaPdf: "página del PDF", portadaDiseñada: "diseñada",
  colorPersonalizado: "Elegir un color personalizado", renombrarCapitulo: "Título del capítulo {n}",
};

window.t = (clave, datos = {}) => String(textos[clave] ?? clave).replace(/\{([^}]+)\}/g, (_, k) => datos[k] ?? `{${k}}`);
window.aplicarIdioma = () => {
  document.documentElement.lang = "es";
  document.querySelectorAll("[data-t]").forEach((el) => { el.textContent = window.t(el.dataset.t); });
  document.querySelectorAll("[data-t-aria]").forEach((el) => el.setAttribute("aria-label", window.t(el.dataset.tAria)));
  document.querySelectorAll("[data-t-title]").forEach((el) => el.setAttribute("title", window.t(el.dataset.tTitle)));
};
