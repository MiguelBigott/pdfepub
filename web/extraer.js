/* Lectura del PDF con pdf.js: el texto de cada página con su posición y su tipografía, las
   imágenes recortadas de la página dibujada y el índice interno (marcadores). También dibuja
   las páginas para la portada y para el modo de páginas fijas. */

import * as pdfjs from "./pdfjs/pdf.mjs";

const base = (ruta) => new URL(ruta, location.href).href;
pdfjs.GlobalWorkerOptions.workerSrc = base("pdfjs/pdf.worker.mjs");

export const versionPdfjs = pdfjs.version;

// abrirPDF abre el documento; pedirClave se llama si tiene contraseña (devuelve la clave o null).
export async function abrirPDF(datos, pedirClave) {
  const tarea = pdfjs.getDocument({
    data: datos,
    cMapUrl: base("pdfjs/cmaps/"),
    cMapPacked: true,
    standardFontDataUrl: base("pdfjs/standard_fonts/"),
    wasmUrl: base("pdfjs/wasm/"),
    iccUrl: base("pdfjs/iccs/"),
    fontExtraProperties: true,
    isEvalSupported: false,
    enableXfa: false,
    enableScripting: false,
  });
  tarea.onPassword = async (actualizar, motivo) => {
    const clave = await pedirClave(motivo === pdfjs.PasswordResponses.INCORRECT_PASSWORD);
    if (clave === null) tarea.destroy();
    else actualizar(clave);
  };
  return tarea.promise;
}

// Los títulos que guardan los programas de oficina («Microsoft Word - doc1.docx») no sirven.
function tituloUtil(t) {
  t = (t || "").trim();
  if (t.length < 2 || /\.(docx?|odt|rtf|pdf|indd|tex|pages|pptx?|xlsx?)$/i.test(t)) return "";
  if (/^(untitled|sin t[ií]tulo|document\d*|documento\d*|microsoft word\b.*|title|titre|titel)$/i.test(t)) return "";
  return t;
}

function fechaPDF(d) {
  const m = /^D:(\d{4})(\d{2})?(\d{2})?/.exec(d || "");
  if (!m) return "";
  return [m[1], m[2], m[3]].filter(Boolean).join("-");
}

export async function datosDelPDF(pdf, nombreArchivo) {
  let info = {};
  try { info = (await pdf.getMetadata()).info || {}; } catch { /* sin metadatos */ }
  const delArchivo = nombreArchivo.replace(/\.pdf$/i, "").replace(/[_]+/g, " ").replace(/\s+/g, " ").trim();
  return {
    titulo: tituloUtil(info.Title) || delArchivo,
    autor: (info.Author || "").trim().replace(/\s*[,;]\s*$/, ""),
    descripcion: (info.Subject || "").trim(),
    etiquetas: (info.Keywords || "").split(/[;,]/).map((s) => s.trim()).filter(Boolean).join(", "),
    fecha: fechaPDF(info.CreationDate),
  };
}

// indice aplana los marcadores del PDF (hasta 4 niveles) con su página y su altura.
export async function indiceDelPDF(pdf) {
  let raiz;
  try { raiz = await pdf.getOutline(); } catch { return []; }
  if (!raiz) return [];
  const res = [];
  const recorrer = async (items, nivel) => {
    for (const it of items) {
      let pagina = -1, y = 0;
      try {
        let dest = it.dest;
        if (typeof dest === "string") dest = await pdf.getDestination(dest);
        if (Array.isArray(dest) && dest[0] != null) {
          pagina = typeof dest[0] === "number" ? dest[0] : await pdf.getPageIndex(dest[0]);
          const tipo = dest[1]?.name;
          const arriba = tipo === "XYZ" ? dest[3] : (tipo === "FitH" || tipo === "FitBH") ? dest[2] : null;
          if (typeof arriba === "number") {
            const vp = (await pdf.getPage(pagina + 1)).getViewport({ scale: 1 });
            y = Math.max(0, vp.convertToViewportPoint(0, arriba)[1]);
          }
        }
      } catch { pagina = -1; }
      res.push({ titulo: it.title || "", pagina, y, nivel });
      if (it.items?.length && nivel < 3) await recorrer(it.items, nivel + 1);
    }
  };
  await recorrer(raiz, 0);
  return res;
}

const r2 = (n) => Math.round(n * 100) / 100;

// dibujar pinta una página en un lienzo con el lado más largo de «lado» píxeles (o con la escala dada).
export async function dibujar(pagina, { lado, escala, lienzo } = {}) {
  const vp1 = pagina.getViewport({ scale: 1 });
  const s = escala ?? lado / Math.max(vp1.width, vp1.height);
  const vp = pagina.getViewport({ scale: s });
  const c = lienzo || document.createElement("canvas");
  c.width = Math.max(1, Math.round(vp.width));
  c.height = Math.max(1, Math.round(vp.height));
  const ctx = c.getContext("2d", { alpha: false });
  ctx.fillStyle = "#fff";
  ctx.fillRect(0, 0, c.width, c.height);
  await pagina.render({ canvasContext: ctx, canvas: c, viewport: vp }).promise;
  return { lienzo: c, escala: s };
}

export const comoBlob = (c, tipo = "image/jpeg", calidad = 0.9) =>
  new Promise((ok, mal) => c.toBlob((b) => (b ? ok(b) : mal(new Error("toBlob"))), tipo, calidad));

// cajasDeImagen recorre las operaciones de dibujo de la página y devuelve dónde quedó cada imagen
// (en puntos, con el origen arriba a la izquierda) y cuánto dibujo vectorial hay.
function cajasDeImagen(ops, vp) {
  const { OPS, Util } = pdfjs;
  let ctm = vp.transform.slice();
  const pila = [];
  const cajas = [];
  let trazos = 0;
  for (let i = 0; i < ops.fnArray.length; i++) {
    const a = ops.argsArray[i];
    switch (ops.fnArray[i]) {
      case OPS.save: pila.push(ctm); break;
      case OPS.restore: ctm = pila.pop() || ctm; break;
      case OPS.transform: ctm = Util.transform(ctm, a); break;
      case OPS.paintFormXObjectBegin:
        pila.push(ctm);
        if (a?.[0]) ctm = Util.transform(ctm, a[0]);
        break;
      case OPS.paintFormXObjectEnd: ctm = pila.pop() || ctm; break;
      case OPS.paintImageXObject:
      case OPS.paintInlineImageXObject:
      case OPS.paintImageXObjectRepeat:
      case OPS.paintImageMaskXObject: {
        const ps = [[0, 0], [1, 0], [0, 1], [1, 1]].map((p) => Util.applyTransform(p, ctm));
        const xs = ps.map((p) => p[0]), ys = ps.map((p) => p[1]);
        cajas.push({ x0: Math.min(...xs), y0: Math.min(...ys), x1: Math.max(...xs), y1: Math.max(...ys) });
        break;
      }
      case OPS.constructPath: case OPS.fill: case OPS.stroke: case OPS.eoFill: case OPS.fillStroke: case OPS.shadingFill:
        trazos++;
        break;
    }
  }
  return { cajas, trazos };
}

// unirCajas junta las imágenes que se tocan (algunos PDF parten una foto en tiras).
function unirCajas(cajas) {
  const res = cajas.map((c) => ({ ...c }));
  for (let cambio = true; cambio;) {
    cambio = false;
    for (let i = 0; i < res.length && !cambio; i++) {
      for (let j = i + 1; j < res.length; j++) {
        const a = res[i], b = res[j];
        if (a.x0 <= b.x1 + 2 && b.x0 <= a.x1 + 2 && a.y0 <= b.y1 + 2 && b.y0 <= a.y1 + 2) {
          res[i] = { x0: Math.min(a.x0, b.x0), y0: Math.min(a.y0, b.y0), x1: Math.max(a.x1, b.x1), y1: Math.max(a.y1, b.y1) };
          res.splice(j, 1);
          cambio = true;
          break;
        }
      }
    }
  }
  return res;
}

/* extraer lee todas las páginas. subir(blob) guarda una imagen en la app y devuelve su id;
   progreso(n, total) avisa por dónde va; cancelado() corta a mitad. */
export async function extraer(pdf, { subir, progreso, cancelado }) {
  const fuentes = [];
  const idFuente = new Map();
  const paginas = [];
  let lienzo = null;

  for (let n = 1; n <= pdf.numPages; n++) {
    if (cancelado()) throw new DOMException("cancelado", "AbortError");
    progreso(n, pdf.numPages);
    const pag = await pdf.getPage(n);
    const vp = pag.getViewport({ scale: 1 });
    // La lista de operaciones carga las tipografías (con su nombre real) y dice dónde van las imágenes.
    const ops = await pag.getOperatorList({ annotationMode: pdfjs.AnnotationMode.DISABLE });
    const tc = await pag.getTextContent();

    const fuente = (nombre) => {
      if (idFuente.has(nombre)) return idFuente.get(nombre);
      let obj = null;
      try { if (pag.commonObjs.has(nombre)) obj = pag.commonObjs.get(nombre); } catch { /* sin datos */ }
      const real = obj?.name || tc.styles?.[nombre]?.fontFamily || "";
      const f = {
        nombre: real,
        negrita: !!obj?.bold || !!obj?.black || /bold|black|heavy|semibold|demi|negrita/i.test(real),
        cursiva: !!obj?.italic || /italic|oblique|kursiv|cursiva/i.test(real),
      };
      idFuente.set(nombre, fuentes.length);
      fuentes.push(f);
      return fuentes.length - 1;
    };

    const textos = [];
    let caracteres = 0;
    for (const it of tc.items) {
      if (!it.str || !it.str.trim()) continue;
      const m = pdfjs.Util.transform(vp.transform, it.transform);
      // Solo texto horizontal y al derecho: lo girado suele ser decoración o marcas al margen.
      if (m[0] <= 0 || Math.abs(m[1]) > Math.abs(m[0]) * 0.2) continue;
      const t = Math.hypot(m[2], m[3]);
      if (t < 1) continue;
      textos.push({ s: it.str, x: r2(m[4]), y: r2(m[5]), w: r2(it.width * vp.scale), t: r2(t), f: fuente(it.fontName) });
      caracteres += it.str.trim().length;
    }

    // Imágenes: las que importan por tamaño, recortadas de la página dibujada.
    const imagenes = [];
    const { cajas, trazos } = cajasDeImagen(ops, vp);
    const area = vp.width * vp.height;
    let utiles = unirCajas(cajas)
      .map((c) => ({ x0: Math.max(0, c.x0), y0: Math.max(0, c.y0), x1: Math.min(vp.width, c.x1), y1: Math.min(vp.height, c.y1) }))
      .filter((c) => c.x1 - c.x0 >= 24 && c.y1 - c.y0 >= 24 && (c.x1 - c.x0) * (c.y1 - c.y0) >= area * 0.015);
    if (caracteres < 12 && (utiles.length || trazos > 20)) {
      // Página sin texto (escaneada, ilustración o dibujo vectorial): va entera.
      utiles = [{ x0: 0, y0: 0, x1: vp.width, y1: vp.height }];
    } else if (caracteres > 200) {
      // Una imagen de fondo debajo de mucho texto (por ejemplo, un escaneo con OCR) no se repite.
      utiles = utiles.filter((c) => (c.x1 - c.x0) * (c.y1 - c.y0) < area * 0.85);
    }
    if (utiles.length) {
      const d = await dibujar(pag, { escala: Math.min(2.2, 2400 / Math.max(vp.width, vp.height)), lienzo });
      lienzo = d.lienzo;
      for (const c of utiles) {
        const s = d.escala;
        const recorte = document.createElement("canvas");
        recorte.width = Math.max(1, Math.round((c.x1 - c.x0) * s));
        recorte.height = Math.max(1, Math.round((c.y1 - c.y0) * s));
        recorte.getContext("2d").drawImage(lienzo, c.x0 * s, c.y0 * s, recorte.width, recorte.height, 0, 0, recorte.width, recorte.height);
        const r = await subir(await comoBlob(recorte, "image/jpeg", 0.9));
        imagenes.push({ id: r.id, x: r2(c.x0), y: r2(c.y0), w: r2(c.x1 - c.x0), h: r2(c.y1 - c.y0) });
      }
    }
    paginas.push({ ancho: r2(vp.width), alto: r2(vp.height), textos, imagenes });
    pag.cleanup();
  }
  return { paginas, fuentes };
}
