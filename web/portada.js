/* Editor de portada: un lienzo de 1600 px de ancho con un fondo (una página del PDF, una imagen
   propia o un color) y, encima, el título y el autor con una de las plantillas. */

const ANCHO = 1600;

export const FUENTES = {
  cormorant: { titulo: "700 {t}px 'Cormorant Garamond'", sub: "italic 500 {t}px 'Cormorant Garamond'", autor: "700 {t}px 'Cormorant Garamond'", escalaTitulo: 1.12, mayus: false },
  playfair: { titulo: "900 {t}px 'Playfair Display'", sub: "italic 500 {t}px 'Cormorant Garamond'", autor: "700 {t}px 'Playfair Display'", escalaTitulo: 0.96, mayus: false },
  inter: { titulo: "800 {t}px Inter", sub: "400 {t}px Inter", autor: "600 {t}px Inter", escalaTitulo: 0.92, mayus: true },
  mono: { titulo: "700 {t}px 'Space Mono'", sub: "400 {t}px 'Space Mono'", autor: "400 {t}px 'Space Mono'", escalaTitulo: 0.8, mayus: true },
};

export const COLORES_FONDO = ["#16120e", "#1d2b3a", "#24382d", "#5a1f24", "#3b2f4f", "#2b2b2b", "#ece2cf", "#f4f1ea"];
export const COLORES_TEXTO = ["#f4efe4", "#16120e", "#c9a45c", "#ffffff"];
export const COLORES_ACENTO = ["#c9a45c", "#c8483a", "#a48be0", "#6f93df", "#f4efe4", "#16120e"];

export function portadaInicial() {
  return {
    fondo: { tipo: "pagina", pagina: 1, color: "#1d2b3a", ajuste: "cubrir", zoom: 1, dx: 0, dy: 0, oscurecer: 0 },
    texto: { plantilla: "ninguna", titulo: "", subtitulo: "", autor: "", fuente: "cormorant", color: "#f4efe4", acento: "#c9a45c", escala: 1 },
  };
}

export async function cargarFuentes() {
  const usos = ["700 40px 'Cormorant Garamond'", "italic 500 40px 'Cormorant Garamond'", "500 40px 'Cormorant Garamond'",
    "700 40px 'Playfair Display'", "900 40px 'Playfair Display'", "800 40px Inter", "600 40px Inter", "400 40px Inter",
    "700 40px 'Space Mono'", "400 40px 'Space Mono'"];
  await Promise.all(usos.map((f) => document.fonts.load(f).catch(() => null)));
}

const fuenteDe = (plantilla, tam) => plantilla.replace("{t}", Math.round(tam));

// partir corta el texto en líneas que entran en el ancho, achicando la letra si hace falta.
function partir(ctx, texto, molde, tam, ancho, maxLineas, minimo = 0.45) {
  const palabras = texto.split(/\s+/).filter(Boolean);
  if (!palabras.length) return { lineas: [], tam };
  for (let t = tam; t >= tam * minimo; t *= 0.94) {
    ctx.font = fuenteDe(molde, t);
    const lineas = [];
    let actual = "";
    let cabe = true;
    for (const p of palabras) {
      if (ctx.measureText(p).width > ancho) { cabe = false; break; }
      const prueba = actual ? actual + " " + p : p;
      if (ctx.measureText(prueba).width <= ancho) actual = prueba;
      else { lineas.push(actual); actual = p; }
    }
    if (actual) lineas.push(actual);
    if (cabe && lineas.length <= maxLineas) return { lineas, tam: t };
  }
  ctx.font = fuenteDe(molde, tam * minimo);
  return { lineas: [palabras.join(" ")], tam: tam * minimo };
}

function escribir(ctx, lineas, x, y, alto, alinear = "center") {
  ctx.textAlign = alinear;
  lineas.forEach((l, i) => ctx.fillText(l, x, y + i * alto));
  return y + lineas.length * alto;
}

function oscurecerColor(hex, f) {
  const n = parseInt(hex.slice(1), 16);
  const c = [(n >> 16) & 255, (n >> 8) & 255, n & 255].map((v) => Math.round(v * f));
  return `rgb(${c[0]}, ${c[1]}, ${c[2]})`;
}

export class EditorPortada {
  constructor(lienzo) {
    this.lienzo = lienzo;
    this.ctx = lienzo.getContext("2d");
    this.imagen = null; // ImageBitmap o lienzo del fondo (página o imagen propia)
    this.proporcion = 1.6;
  }

  // medidas: la portada mide 1600 px de ancho y el alto sale de la proporción (1,6 en texto fluido).
  ajustarProporcion(p) {
    this.proporcion = Math.min(2, Math.max(1, p));
    const alto = Math.round(ANCHO * this.proporcion);
    if (this.lienzo.width !== ANCHO || this.lienzo.height !== alto) {
      this.lienzo.width = ANCHO;
      this.lienzo.height = alto;
      this.lienzo.style.aspectRatio = `${ANCHO} / ${alto}`;
    }
  }

  // limites dice cuánto se puede mover la imagen sin que aparezca el borde (en modo cubrir).
  geometria(est) {
    const W = this.lienzo.width, H = this.lienzo.height;
    const im = this.imagen;
    if (!im) return null;
    const f = est.fondo;
    const base = f.ajuste === "entera" ? Math.min(W / im.width, H / im.height) : Math.max(W / im.width, H / im.height);
    const s = base * f.zoom;
    const w = im.width * s, h = im.height * s;
    const libreX = Math.max(0, (w - W) / 2), libreY = Math.max(0, (h - H) / 2);
    return { W, H, w, h, libreX, libreY };
  }

  mover(est, dx, dy) {
    const g = this.geometria(est);
    if (!g) return;
    const f = est.fondo;
    const tope = (v, lim) => (f.ajuste === "entera" ? Math.max(-g.W / 2, Math.min(g.W / 2, v)) : Math.max(-lim, Math.min(lim, v)));
    f.dx = tope(f.dx + dx, g.libreX);
    f.dy = tope(f.dy + dy, g.libreY);
  }

  pintar(est) {
    const ctx = this.ctx, W = this.lienzo.width, H = this.lienzo.height;
    const f = est.fondo;
    ctx.save();
    ctx.clearRect(0, 0, W, H);
    // Fondo: el color (con un degradado suave) siempre debajo; la imagen encima si la hay.
    const grad = ctx.createLinearGradient(0, 0, 0, H);
    grad.addColorStop(0, f.color);
    grad.addColorStop(1, oscurecerColor(f.color, 0.72));
    ctx.fillStyle = f.tipo === "color" ? grad : f.color;
    ctx.fillRect(0, 0, W, H);
    if (f.tipo !== "color" && this.imagen) {
      const g = this.geometria(est);
      f.dx = f.ajuste === "entera" ? f.dx : Math.max(-g.libreX, Math.min(g.libreX, f.dx));
      f.dy = f.ajuste === "entera" ? f.dy : Math.max(-g.libreY, Math.min(g.libreY, f.dy));
      ctx.imageSmoothingQuality = "high";
      ctx.drawImage(this.imagen, (W - g.w) / 2 + f.dx, (H - g.h) / 2 + f.dy, g.w, g.h);
    }
    if (f.oscurecer > 0) {
      ctx.fillStyle = `rgba(0, 0, 0, ${f.oscurecer})`;
      ctx.fillRect(0, 0, W, H);
    }
    ctx.restore();
    this.pintarTexto(est.texto);
  }

  pintarTexto(tx) {
    if (tx.plantilla === "ninguna") return;
    const ctx = this.ctx, W = this.lienzo.width, H = this.lienzo.height;
    const fu = FUENTES[tx.fuente] || FUENTES.cormorant;
    const e = tx.escala;
    const titulo = tx.titulo.trim(), sub = tx.subtitulo.trim();
    const autor = fu.mayus || tx.plantilla === "moderna" ? tx.autor.trim().toUpperCase() : tx.autor.trim();
    ctx.save();
    ctx.textBaseline = "alphabetic";
    ctx.fillStyle = tx.color;
    const espaciado = (px) => { ctx.letterSpacing = `${px}px`; };

    const bloqueTitulo = (x, y, ancho, tam, lineasMax, alinear) => {
      const p = partir(ctx, titulo, fu.titulo, tam * fu.escalaTitulo * e, ancho, lineasMax);
      ctx.font = fuenteDe(fu.titulo, p.tam);
      espaciado(0);
      return { fin: escribir(ctx, p.lineas, x, y + p.tam * 0.8, p.tam * 1.08, alinear) - p.tam * 0.28, tam: p.tam, n: p.lineas.length };
    };
    const linea = (x, y, ancho, grosor) => {
      ctx.fillStyle = tx.acento;
      ctx.fillRect(x, y, ancho, grosor);
      ctx.fillStyle = tx.color;
    };
    const textoAutor = (x, y, ancho, tam, alinear, mayus = true) => {
      const molde = fu.autor;
      espaciado(mayus ? tam * 0.12 : 0);
      const p = partir(ctx, autor, molde, tam * e, ancho, 2, 0.6);
      ctx.font = fuenteDe(molde, p.tam);
      return escribir(ctx, p.lineas, x, y, p.tam * 1.25, alinear);
    };
    const textoSub = (x, y, ancho, tam, alinear) => {
      espaciado(0);
      const p = partir(ctx, sub, fu.sub, tam * e, ancho, 3, 0.6);
      ctx.font = fuenteDe(fu.sub, p.tam);
      return escribir(ctx, p.lineas, x, y + p.tam, p.tam * 1.25, alinear);
    };

    const m = W * 0.1;
    switch (tx.plantilla) {
      case "clasica": {
        let y = H * 0.19;
        linea(W / 2 - W * 0.09, y, W * 0.18, W * 0.004);
        y += W * 0.06;
        const t = bloqueTitulo(W / 2, y, W - 2 * m, W * 0.11, 5, "center");
        y = t.fin + W * 0.04;
        linea(W / 2 - W * 0.09, y, W * 0.18, W * 0.004);
        if (sub) textoSub(W / 2, y + W * 0.03, W - 2.4 * m, W * 0.045, "center");
        if (autor) textoAutor(W / 2, H - H * 0.09, W - 2 * m, W * 0.042, "center", fu.mayus);
        break;
      }
      case "moderna": {
        // De abajo hacia arriba: subtítulo, título, barra y autor.
        const ancho = W - 2 * m;
        ctx.font = fuenteDe(fu.titulo, W * 0.13 * fu.escalaTitulo * e);
        const p = partir(ctx, titulo, fu.titulo, W * 0.13 * fu.escalaTitulo * e, ancho, 5);
        const altoTit = p.lineas.length * p.tam * 1.04;
        let altoSub = 0, ps = null;
        if (sub) {
          ps = partir(ctx, sub, fu.sub, W * 0.04 * e, ancho, 3, 0.6);
          altoSub = ps.lineas.length * ps.tam * 1.3 + W * 0.03;
        }
        const yTit = H - H * 0.08 - altoSub - altoTit;
        if (autor) {
          linea(m, yTit - W * 0.12, W * 0.07, W * 0.008);
          ctx.fillStyle = tx.acento;
          textoAutor(m, yTit - W * 0.05, ancho, W * 0.034, "left");
          ctx.fillStyle = tx.color;
        }
        ctx.font = fuenteDe(fu.titulo, p.tam);
        espaciado(-p.tam * 0.01);
        escribir(ctx, p.lineas, m, yTit + p.tam * 0.82, p.tam * 1.04, "left");
        if (ps) {
          espaciado(0);
          ctx.font = fuenteDe(fu.sub, ps.tam);
          escribir(ctx, ps.lineas, m, yTit + altoTit + W * 0.03 + ps.tam, ps.tam * 1.3, "left");
        }
        break;
      }
      case "banda": {
        const ancho = W - 2 * m;
        const p = partir(ctx, titulo, fu.titulo, W * 0.1 * fu.escalaTitulo * e, ancho, 4);
        const altoTit = p.lineas.length * p.tam * 1.08;
        const ps = sub ? partir(ctx, sub, fu.sub, W * 0.04 * e, ancho, 2, 0.6) : null;
        const altoSub = ps ? ps.lineas.length * ps.tam * 1.25 + W * 0.02 : 0;
        const altoAutor = autor ? W * 0.09 : 0;
        const relleno = W * 0.07;
        const alto = relleno * 2 + altoTit + altoSub + altoAutor;
        const y0 = Math.min(H * 0.6, H - H * 0.07 - alto);
        ctx.fillStyle = tx.acento;
        ctx.globalAlpha = 0.94;
        ctx.fillRect(0, y0, W, alto);
        ctx.globalAlpha = 1;
        ctx.fillStyle = tx.color;
        ctx.font = fuenteDe(fu.titulo, p.tam);
        espaciado(0);
        let y = escribir(ctx, p.lineas, W / 2, y0 + relleno + p.tam * 0.8, p.tam * 1.08, "center");
        if (ps) {
          ctx.font = fuenteDe(fu.sub, ps.tam);
          y = escribir(ctx, ps.lineas, W / 2, y - p.tam * 0.2 + W * 0.02 + ps.tam, ps.tam * 1.25, "center");
        }
        if (autor) textoAutor(W / 2, y0 + alto - relleno, ancho, W * 0.036, "center");
        break;
      }
      case "marco": {
        ctx.strokeStyle = tx.acento;
        ctx.lineWidth = W * 0.006;
        ctx.strokeRect(W * 0.05, W * 0.05, W - W * 0.1, H - W * 0.1);
        ctx.lineWidth = W * 0.0025;
        ctx.strokeRect(W * 0.068, W * 0.068, W - W * 0.136, H - W * 0.136);
        const ancho = W - 2.6 * m;
        const p = partir(ctx, titulo, fu.titulo, W * 0.1 * fu.escalaTitulo * e, ancho, 5);
        const altoTit = p.lineas.length * p.tam * 1.08;
        const y0 = H * 0.42 - altoTit / 2;
        ctx.font = fuenteDe(fu.titulo, p.tam);
        espaciado(0);
        const fin = escribir(ctx, p.lineas, W / 2, y0 + p.tam * 0.8, p.tam * 1.08, "center");
        linea(W / 2 - W * 0.05, fin + W * 0.01, W * 0.1, W * 0.004);
        if (sub) textoSub(W / 2, fin + W * 0.05, ancho, W * 0.042, "center");
        if (autor) textoAutor(W / 2, H - H * 0.12, ancho, W * 0.04, "center", fu.mayus);
        break;
      }
    }
    ctx.restore();
  }
}
