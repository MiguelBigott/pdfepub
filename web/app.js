import { abrirPDF, datosDelPDF, indiceDelPDF, dibujar, comoBlob, extraer, versionPdfjs } from "./extraer.js";
import { EditorPortada, portadaInicial, cargarFuentes, COLORES_FONDO, COLORES_TEXTO, COLORES_ACENTO } from "./portada.js";

const $ = (s, raiz = document) => raiz.querySelector(s);
const $$ = (s, raiz = document) => [...raiz.querySelectorAll(s)];
const t = (...args) => window.t(...args);
const esc = (s) => String(s ?? "").replace(/[&<>"']/g, (c) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;", "'": "&#39;" })[c]);
const idiomas = ["es", "en", "fr", "de", "pt", "it", "zh", "ja", "ru", "und"];

let config = { carpeta: "", modo: "fluido" };
let pdf = null;
let archivoActual = null;
let analisis = null;
let documento = null;
let capitulos = [];
let modo = "fluido";
let seccion = "datos";
let capActual = 0;
let bloqueElegido = -1;
let paginaElegida = 0;
let portada = portadaInicial();
let editorPortada = new EditorPortada($("#lienzo"));
let imagenPropia = null;
let fondoPaginaCargada = -1;
let paginasFijasCache = new Map();
let miniaturasCreadas = false;
let tokenTrabajo = null;
let avisoTimer = 0;
let tituloPortadaAutomatico = true;
let autorPortadaAutomatico = true;

async function api(ruta, cuerpo) {
  const opciones = cuerpo === undefined ? {} : {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(cuerpo),
  };
  const r = await fetch(ruta, opciones);
  const datos = await r.json().catch(() => ({}));
  if (!r.ok) {
    const err = new Error(t(datos?.error?.clave || "errorGeneral", datos?.error?.datos || { detalle: r.statusText }));
    err.mensaje = datos?.error;
    throw err;
  }
  return datos;
}

async function subir(blob) {
  const r = await fetch("/api/recurso", { method: "POST", headers: { "X-PdfEpub": "1" }, body: blob });
  const datos = await r.json().catch(() => ({}));
  if (!r.ok) {
    const err = new Error(t(datos?.error?.clave || "noGuardarImagen", datos?.error?.datos));
    err.mensaje = datos?.error;
    throw err;
  }
  return datos;
}

function textoError(err) {
  const m = err?.mensaje;
  if (!m) return err?.message || t("errorGeneral", { detalle: "" });
  let s = t(m.clave, m.datos || {});
  if (m.causa) s = s.replace("{causa}", textoError({ mensaje: m.causa }));
  return s;
}

function avisar(mensaje, error = false) {
  const el = $("#aviso");
  el.textContent = mensaje;
  el.classList.toggle("error", error);
  el.hidden = false;
  clearTimeout(avisoTimer);
  avisoTimer = setTimeout(() => { el.hidden = true; }, 5000);
}

function progreso(titulo, detalle, fraccion = null, cancelable = true) {
  $("#progreso-titulo").textContent = titulo;
  $("#progreso-detalle").textContent = detalle;
  $("#progreso-barra").style.width = fraccion === null ? "28%" : `${Math.round(fraccion * 100)}%`;
  $("#progreso-barra").classList.toggle("indeterminado", fraccion === null);
  $("#btn-cancelar").hidden = !cancelable;
  $("#progreso").hidden = false;
}

function cerrarProgreso() {
  $("#progreso").hidden = true;
  $("#progreso-barra").classList.remove("indeterminado");
  tokenTrabajo = null;
}

function comprobarCancelado(token = tokenTrabajo) {
  if (token?.cancelado) throw new DOMException("cancelado", "AbortError");
}

function dialogoClave(incorrecta) {
  const dlg = $("#dlg-clave"), input = $("#clave"), ayuda = $("#clave-ayuda");
  input.value = "";
  ayuda.textContent = incorrecta ? t("claveIncorrecta") : t("claveAyuda");
  return new Promise((resolve) => {
    dlg.addEventListener("close", () => resolve(dlg.returnValue === "ok" ? input.value : null), { once: true });
    dlg.showModal();
    input.focus();
  });
}

function nombreBonito(nombre) {
  return nombre.replace(/\.pdf$/i, "").replace(/[_]+/g, " ").replace(/\s+/g, " ").trim();
}

async function abrirArchivo(file) {
  if (!file) return;
  if (!/\.pdf$/i.test(file.name) && file.type !== "application/pdf") {
    avisar(t("archivoIncorrecto"), true);
    return;
  }
  const token = { cancelado: false };
  tokenTrabajo = token;
  progreso(t("cargando"), file.name, 0, true);
  try {
    if (pdf) {
      await pdf.destroy().catch(() => {});
      pdf = null;
    }
    archivoActual = null;
    analisis = null;
    documento = null;
    capitulos = [];
    $("#editor").hidden = true;
    $("#inicio").hidden = false;
    $("#resultado").hidden = true;
    $("#btn-crear").hidden = false;
    await api("/api/nuevo", {});
    comprobarCancelado(token);
    const bytes = new Uint8Array(await file.arrayBuffer());
    const nuevoPdf = await abrirPDF(bytes, dialogoClave);
    comprobarCancelado(token);
    pdf = nuevoPdf;
    if (!pdf.numPages) throw new Error(t("pdfVacio"));

    progreso(t("leyendo"), t("paginaDe", { n: 0, total: pdf.numPages }), 0, true);
    documento = await extraer(pdf, {
      subir,
      cancelado: () => token.cancelado,
      progreso: (n, total) => progreso(t("leyendo"), t("paginaDe", { n, total }), n / total, true),
    });
    comprobarCancelado(token);
    documento.indice = await indiceDelPDF(pdf);
    progreso(t("preparando"), t("preparandoCapitulos"), null, false);
    analisis = await api("/api/analizar", documento);
    comprobarCancelado(token);

    const meta = await datosDelPDF(pdf, file.name);
    meta.editorial = "";
    meta.isbn = "";
    meta.serie = "";
    meta.numero = "";
    meta.idioma = idiomas.includes(analisis.idioma) ? analisis.idioma : "es";
    cargarDatos(meta);
    archivoActual = file;
    $("#archivo-nombre").textContent = file.name;
    $("#archivo-nombre").title = file.name;
    $("#archivo-detalle").textContent = `${t("paginas", { n: pdf.numPages })} · ${t("palabras", { n: analisis.palabras.toLocaleString("es-ES") })}`;
    $("#inicio").hidden = true;
    $("#editor").hidden = false;
    $("#resultado").hidden = true;
    crearCapitulos(analisis.capitulos || []);
    reiniciarPortada(meta);
    miniaturasCreadas = false;
    paginasFijasCache.clear();
    const preferido = analisis.escaneado || !analisis.bloques.length ? "fijo" : config.modo;
    cambiarModo(preferido, false);
    cambiarSeccion("datos");
    actualizarResumen();
    cerrarProgreso();
    if (analisis.escaneado) avisar(analisis.palabras ? t("escaneado") : t("sinTexto"));
  } catch (err) {
    cerrarProgreso();
    if (err?.name !== "AbortError") avisar(err?.message || t("errorGeneral", { detalle: "" }), true);
    if (pdf && !analisis) { pdf.destroy().catch(() => {}); pdf = null; }
  }
}

function cargarDatos(meta) {
  const f = $("#form-datos");
  for (const k of ["titulo", "autor", "idioma", "fecha", "editorial", "isbn", "serie", "numero", "etiquetas", "descripcion"]) {
    f.elements[k].value = meta[k] || "";
  }
  const sel = $("#sel-idioma");
  sel.replaceChildren(...idiomas.map((id) => {
    const op = document.createElement("option");
    op.value = id;
    op.textContent = t(`idioma_${id}`);
    return op;
  }));
  sel.value = idiomas.includes(meta.idioma) ? meta.idioma : "es";
}

function datosFormulario() {
  const f = $("#form-datos");
  return Object.fromEntries(["titulo", "autor", "idioma", "fecha", "editorial", "isbn", "serie", "numero", "etiquetas", "descripcion"]
    .map((k) => [k, String(f.elements[k].value || "").trim()]));
}

function crearCapitulos(sugeridos) {
  const bs = analisis?.bloques || [];
  const vistos = new Set();
  capitulos = sugeridos.map((c, i) => {
    const desde = Math.max(0, Math.min(bs.length - 1, Number(c.desde) || 0));
    const pagina = bs[desde]?.pagina ?? 0;
    if (vistos.has(desde)) return null;
    vistos.add(desde);
    return { titulo: c.titulo || (c.tipo === "inicio" ? t("inicio") : c.tipo === "paginas" ? t("paginasCapitulo", { n: pdf.numPages }) : t("capitulo", { n: i + 1 })), desde, pagina, incluir: true };
  }).filter(Boolean).sort((a, b) => a.desde - b.desde || a.pagina - b.pagina);
  if (!capitulos.length) capitulos = [{
    titulo: bs.length ? t("inicio") : t("paginasCapitulo", { n: pdf?.numPages || 0 }),
    desde: 0, pagina: 0, incluir: true,
  }];
  capitulos.forEach((c, i) => { if (!c.titulo) c.titulo = t("capitulo", { n: i + 1 }); });
  capActual = 0;
  bloqueElegido = -1;
  paginaElegida = capitulos[0]?.pagina || 0;
  pintarCapitulos();
}

function actualizarResumen() {
  $("#sec-datos").textContent = datosFormulario().autor || t("sinAutor");
  $("#sec-portada").textContent = portada.texto.plantilla === "ninguna" ? t("portadaPdf") : t("portadaDiseñada");
  $("#sec-capitulos").textContent = t("datosCapitulos", { n: capitulos.length, paginas: pdf?.numPages || 0 });
  $("#ruta-destino").textContent = config.carpeta || "…";
  $("#ruta-destino").title = config.carpeta || "";
}

function cambiarSeccion(nombre) {
  seccion = nombre;
  for (const b of $$("[data-seccion]")) b.setAttribute("aria-selected", String(b.dataset.seccion === nombre));
  for (const id of ["datos", "portada", "capitulos"]) $(`#v-${id}`).hidden = id !== nombre;
  if (nombre === "portada") dibujarPortada().catch((e) => avisar(e.message, true));
  if (nombre === "capitulos") {
    pintarCapitulos();
    if (modo === "fluido") pintarVistaFluida();
    else pintarVistaFija().catch((e) => avisar(e.message, true));
  }
}

function cambiarModo(nuevo, guardar = true) {
  modo = nuevo === "fijo" ? "fijo" : "fluido";
  document.body.classList.toggle("modo-fijo", modo === "fijo");
  $$('input[name="modo"]').forEach((r) => { r.checked = r.value === modo; });
  $("#calidad").hidden = modo !== "fijo";
  $("#nota-modo").textContent = analisis?.escaneado ? t("escaneado") : t(modo === "fijo" ? "modoFijoNota" : "modoFluidoNota");
  $("#nota-modo").classList.toggle("alerta", Boolean(analisis?.escaneado));
  $("#ayuda-capitulos").textContent = t(modo === "fijo" ? "ayudaCapitulosFijo" : "ayudaCapitulosFluido");
  $("#previa").hidden = modo !== "fluido";
  $("#paginas-previa").hidden = modo !== "fijo";
  $("#btn-dividir").hidden = modo !== "fluido";
  $("#cap-seleccion").hidden = modo !== "fluido";
  $("#cap-pagina-seleccionada").hidden = modo !== "fijo";
  $("#btn-dividir-pagina").hidden = modo !== "fijo";
  if (seccion === "capitulos") {
    if (modo === "fluido") pintarVistaFluida();
    else pintarVistaFija().catch((e) => avisar(e.message, true));
  }
  if (seccion === "portada") dibujarPortada().catch((e) => avisar(e.message, true));
  if (guardar) api("/api/modo", { modo }).catch((e) => avisar(e.message, true));
}

function pintarCapitulos() {
  const ol = $("#cap-lista");
  ol.replaceChildren();
  if (!capitulos.length) {
    const li = document.createElement("li");
    li.className = "cap-vacio";
    li.textContent = t("sinCapitulos");
    ol.append(li);
  }
  capitulos.forEach((c, i) => {
    const li = document.createElement("li");
    li.className = "cap";
    li.dataset.indice = i;
    if (i === capActual) li.classList.add("sel");
    if (!c.incluir) li.classList.add("fuera");
    const check = document.createElement("input");
    check.type = "checkbox";
    check.checked = c.incluir;
    check.setAttribute("aria-label", t(c.incluir ? "excluirCapitulo" : "incluirCapitulo"));
    check.addEventListener("change", () => { c.incluir = check.checked; pintarCapitulos(); actualizarResumen(); });
    const titulo = document.createElement("input");
    titulo.type = "text";
    titulo.className = "cap-titulo";
    titulo.value = c.titulo;
    titulo.maxLength = 180;
    titulo.setAttribute("aria-label", t("renombrarCapitulo", { n: i + 1 }));
    titulo.addEventListener("focus", () => seleccionarCapitulo(i));
    titulo.addEventListener("input", () => {
      c.titulo = titulo.value;
      actualizarResumen();
      $("#previa-titulo").textContent = c.titulo;
      if (modo === "fluido" && i === capActual) $("#etq-epub").textContent = t("etqEpub", { titulo: c.titulo });
    });
    const info = document.createElement("span");
    info.className = "cap-info";
    info.textContent = modo === "fijo" ? t("paginaPdf", { n: c.pagina + 1 }) : t("paginaPdf", { n: analisis?.bloques[c.desde]?.pagina + 1 || 1 });
    li.addEventListener("click", (e) => { if (e.target.closest("button,input[type=checkbox]")) return; seleccionarCapitulo(i); });
    li.append(check, titulo, info);
    if (i > 0) {
      const unir = document.createElement("button");
      unir.type = "button";
      unir.className = "cap-unir";
      unir.textContent = t("unirAnterior");
      unir.title = t("unirAnterior");
      unir.addEventListener("click", () => {
        capitulos.splice(i, 1);
        capActual = Math.max(0, i - 1);
        paginaElegida = capitulos[capActual]?.pagina || 0;
        pintarCapitulos();
        if (modo === "fluido") pintarVistaFluida(); else pintarVistaFija().catch(() => {});
        actualizarResumen();
      });
      li.append(unir);
    }
    ol.append(li);
  });
  $("#previa-titulo").textContent = capitulos[capActual]?.titulo || "";
  $("#previa-info").textContent = capitulos.length ? `${capActual + 1} / ${capitulos.length}` : "";
  $("#prev-ant").disabled = capActual <= 0;
  $("#prev-sig").disabled = capActual >= capitulos.length - 1;
  $("#btn-crear").disabled = !capitulos.some((c) => c.incluir);
}

function seleccionarCapitulo(i) {
  if (i < 0 || i >= capitulos.length) return;
  capActual = i;
  paginaElegida = capitulos[i].pagina || 0;
  pintarCapitulos();
  if (modo === "fluido") pintarVistaFluida();
  else pintarVistaFija().catch((e) => avisar(e.message, true));
}

function limitesCapitulo(i) {
  const desde = capitulos[i]?.desde ?? 0;
  const siguiente = capitulos[i + 1]?.desde ?? (analisis?.bloques.length || 0);
  return [desde, siguiente];
}

async function pintarVistaFluida() {
  if (!analisis || modo !== "fluido") return;
  const iframe = $("#previa"), doc = iframe.contentDocument;
  const [desde, fin] = limitesCapitulo(capActual);
  const css = await fetch("libro.css").then((r) => r.text()).catch(() => "body{font:16px Georgia;line-height:1.55;margin:5%}");
  const contenido = analisis.bloques.slice(desde, fin).map((b, j) => {
    const idx = desde + j;
    if (b.tipo === "img") return `<figure class="imagen"><img src="/api/recurso?id=${encodeURIComponent(b.imagen)}" alt=""/></figure>`;
    if (b.tipo === "h") return `<h${Math.min(3, Math.max(1, b.nivel || 1))} data-bloque="${idx}" tabindex="0">${b.html || esc(b.texto)}</h${Math.min(3, Math.max(1, b.nivel || 1))}>`;
    return `<p data-bloque="${idx}" tabindex="0"${b.centro ? ' class="centro"' : ""}>${b.html || esc(b.texto)}</p>`;
  }).join("\n");
  // La hoja derecha del pliego: el texto del capítulo como se leerá, sobre el papel del EPUB.
  const papel = "html{background:#f2ece0;color-scheme:light}::-webkit-scrollbar{width:8px}::-webkit-scrollbar-thumb{background:rgba(60,45,25,.28);border-radius:4px}body{margin:14px 22px 24px;color:#2a241c;font:17px/1.5 'Iowan Old Style','Palatino Linotype',Georgia,serif}h1,h2,h3{color:#16120e}";
  const marca = "[data-bloque]{cursor:pointer;outline:2px solid transparent;outline-offset:3px;border-radius:2px}[data-bloque]:hover{background:rgba(164,139,224,.08)}[data-bloque].seleccionado{outline-color:#a48be0;background:rgba(164,139,224,.16)}img{max-width:100%}";
  doc.open();
  doc.write(`<!doctype html><html lang="es"><head><meta charset="utf-8"><style>${css}\n${papel}\n${marca}</style></head><body>${contenido || `<p>${esc(t("sinTexto"))}</p>`}</body></html>`);
  doc.close();
  $("#etq-epub").textContent = t("etqEpub", { titulo: capitulos[capActual]?.titulo || "" });
  $("#etq-epub-der").textContent = t("asiSeLee");
  const marcar = (idx) => {
    bloqueElegido = idx;
    doc.querySelectorAll("[data-bloque]").forEach((n) => n.classList.toggle("seleccionado", Number(n.dataset.bloque) === idx));
    $("#cap-seleccion").textContent = `${t("seleccion")} · ${idx + 1}`;
    $("#btn-dividir").disabled = idx <= 0 || capitulos.some((c) => c.desde === idx);
    pintarOriginal(analisis.bloques[idx]?.pagina ?? 0).catch(() => {});
  };
  doc.body.addEventListener("click", (e) => {
    const el = e.target.closest("[data-bloque]");
    if (el) marcar(Number(el.dataset.bloque));
  });
  doc.body.addEventListener("keydown", (e) => {
    if ((e.key === "Enter" || e.key === " ") && e.target.matches("[data-bloque]")) { e.preventDefault(); marcar(Number(e.target.dataset.bloque)); }
  });
  if (bloqueElegido >= desde && bloqueElegido < fin) marcar(bloqueElegido);
  else {
    bloqueElegido = -1;
    $("#cap-seleccion").textContent = t("seleccionaParrafo");
    $("#btn-dividir").disabled = true;
    pintarOriginal(analisis.bloques[desde]?.pagina ?? 0).catch(() => {});
  }
}

// pintarOriginal dibuja en la hoja izquierda del pliego la página n del PDF (empieza en 0).
let originalPedido = 0;
async function pintarOriginal(n) {
  if (!pdf) return;
  n = Math.max(0, Math.min(pdf.numPages - 1, n));
  const caja = $("#previa-pagina");
  $("#etq-pdf").textContent = t("etqPdf", { n: n + 1 });
  if (caja.dataset.pagina === String(n) && caja.firstChild) return;
  const pedido = ++originalPedido;
  const lienzo = await paginaCanvas(n, 900);
  if (pedido !== originalPedido) return; // ya se pidió otra página
  const copia = document.createElement("canvas");
  copia.width = lienzo.width;
  copia.height = lienzo.height;
  copia.getContext("2d").drawImage(lienzo, 0, 0);
  caja.replaceChildren(copia);
  caja.dataset.pagina = String(n);
}

function paginaCanvas(n, lado) {
  const cacheable = lado === 170;
  const clave = `${n}:${lado}`;
  if (cacheable && paginasFijasCache.has(clave)) return Promise.resolve(paginasFijasCache.get(clave));
  return pdf.getPage(n + 1).then((p) => dibujar(p, { lado })).then((d) => {
    if (cacheable) {
      paginasFijasCache.set(clave, d.lienzo);
      while (paginasFijasCache.size > 60) paginasFijasCache.delete(paginasFijasCache.keys().next().value);
    }
    return d.lienzo;
  });
}

async function pintarVistaFija() {
  if (!pdf || modo !== "fijo") return;
  paginaElegida = Math.max(0, Math.min(pdf.numPages - 1, paginaElegida));
  $("#cap-pagina-seleccionada").textContent = t("paginaSeleccionada", { n: paginaElegida + 1 });
  $("#btn-dividir-pagina").disabled = capitulos.some((c) => c.pagina === paginaElegida);
  $("#etq-epub").textContent = t("paginasDelPdf");
  $("#etq-epub-der").textContent = t("eligePagina");
  if (!miniaturasCreadas) crearMiniaturas();
  await pintarOriginal(paginaElegida);
  $$(".pag-mini").forEach((el) => el.classList.toggle("sel", Number(el.dataset.pagina) === paginaElegida));
}

function crearMiniaturas() {
  if (!pdf) return;
  const caja = $("#paginas-previa");
  caja.replaceChildren();
  const observador = new IntersectionObserver((entradas) => {
    for (const ent of entradas) {
      if (!ent.isIntersecting) continue;
      const el = ent.target;
      observador.unobserve(el);
      const n = Number(el.dataset.pagina);
      paginaCanvas(n, 170).then((c) => {
        const canvas = document.createElement("canvas");
        canvas.width = c.width;
        canvas.height = c.height;
        canvas.getContext("2d").drawImage(c, 0, 0);
        el.querySelector(".hoja").replaceChildren(canvas);
      }).catch(() => {});
    }
  }, { root: caja, rootMargin: "120px" });
  for (let n = 0; n < pdf.numPages; n++) {
    const p = document.createElement("button");
    p.type = "button";
    p.className = "pag-mini";
    p.dataset.pagina = n;
    p.setAttribute("aria-label", t("paginaPdf", { n: n + 1 }));
    p.innerHTML = `<span class="hoja" style="--prop:${(documento.paginas[n].ancho / documento.paginas[n].alto).toFixed(4)}"></span><span class="pie-mini">${n + 1}</span>`;
    p.addEventListener("click", () => {
      paginaElegida = n;
      $$(".pag-mini").forEach((el) => el.classList.toggle("sel", Number(el.dataset.pagina) === n));
      pintarVistaFija().catch((e) => avisar(e.message, true));
    });
    caja.append(p);
    observador.observe(p);
  }
  miniaturasCreadas = true;
}

function tituloParaBloque(idx) {
  const b = analisis.bloques[idx];
  if (b?.tipo === "h" && b.texto?.trim()) return b.texto.trim().slice(0, 120);
  return t("capitulo", { n: capitulos.length + 1 });
}

function dividirEnBloque(idx) {
  if (idx <= 0 || idx >= (analisis?.bloques.length || 0) || capitulos.some((c) => c.desde === idx)) return;
  capitulos.push({ titulo: tituloParaBloque(idx), desde: idx, pagina: analisis.bloques[idx]?.pagina || 0, incluir: true });
  capitulos.sort((a, b) => a.desde - b.desde);
  capActual = capitulos.findIndex((c) => c.desde === idx);
  bloqueElegido = idx;
  pintarCapitulos();
  pintarVistaFluida();
  actualizarResumen();
  avisar(t("capituloCreado"));
}

function dividirEnPagina(n) {
  const desde = analisis.bloques.findIndex((b) => b.pagina >= n);
  if (desde < 0 || capitulos.some((c) => c.pagina === n)) { avisar(t("limiteCapitulos")); return; }
  capitulos.push({ titulo: t("capitulo", { n: capitulos.length + 1 }), desde, pagina: n, incluir: true });
  capitulos.sort((a, b) => a.pagina - b.pagina || a.desde - b.desde);
  capActual = capitulos.findIndex((c) => c.pagina === n);
  pintarCapitulos();
  pintarVistaFija().catch(() => {});
  actualizarResumen();
  avisar(t("capituloCreado"));
}

async function cargarPaginaPortada(n) {
  if (!pdf) return;
  n = Math.max(0, Math.min(pdf.numPages - 1, n));
  portada.fondo.pagina = n + 1;
  $("#pag-num").value = n + 1;
  $("#pag-total").textContent = `/ ${pdf.numPages}`;
  if (fondoPaginaCargada !== n) {
    const src = await paginaCanvas(n, 2200);
    editorPortada.imagen = src;
    fondoPaginaCargada = n;
  }
  pintarPortada();
}

function reiniciarPortada(meta) {
  portada = portadaInicial();
  portada.texto.titulo = meta.titulo || "";
  portada.texto.autor = meta.autor || "";
  imagenPropia = null;
  fondoPaginaCargada = -1;
  tituloPortadaAutomatico = true;
  autorPortadaAutomatico = true;
  const p = documento.paginas[0];
  editorPortada.ajustarProporcion(p.alto / p.ancho);
  sincronizarControlesPortada();
  cargarPaginaPortada(0).catch((e) => avisar(e.message, true));
}

function colorBotones(contenedor, colores, clave, etiqueta) {
  const padre = $(contenedor);
  for (const color of colores) {
    const b = document.createElement("button");
    b.type = "button";
    b.className = "muestra";
    b.style.background = color;
    b.title = color;
    b.setAttribute("aria-label", `${etiqueta}: ${color}`);
    b.dataset.color = color;
    b.addEventListener("click", () => {
      if (clave === "fondo") portada.fondo.color = color;
      else portada.texto[clave] = color;
      pintarPortada();
      sincronizarColores();
    });
    padre.append(b);
  }
  const custom = document.createElement("input");
  custom.type = "color";
  custom.setAttribute("aria-label", t("colorPersonalizado"));
  custom.addEventListener("input", () => {
    if (clave === "fondo") portada.fondo.color = custom.value;
    else portada.texto[clave] = custom.value;
    pintarPortada();
    sincronizarColores();
  });
  padre.append(custom);
}

function sincronizarColores() {
  const grupos = [
    ["#fila-color", "fondo", portada.fondo.color],
    ["#fila-color-texto", "texto", portada.texto.color],
    ["#fila-color-acento", "acento", portada.texto.acento],
  ];
  for (const [sel, key, color] of grupos) {
    const cont = $(sel);
    $$(".muestra", cont).forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.color.toLowerCase() === color.toLowerCase())));
    const custom = $("input[type=color]", cont);
    if (custom) custom.value = color;
  }
}

function sincronizarControlesPortada() {
  const f = portada.fondo, tx = portada.texto;
  $("#pt-titulo").value = tx.titulo;
  $("#pt-subtitulo").value = tx.subtitulo;
  $("#pt-autor").value = tx.autor;
  $("#zoom").value = f.zoom;
  $("#oscurecer").value = f.oscurecer;
  $("#escala").value = tx.escala;
  $("#pag-num").value = f.pagina;
  $$("#fondo-tipo [data-valor]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.valor === f.tipo)));
  $$("#fondo-ajuste [data-valor]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.valor === f.ajuste)));
  $$("#plantilla [data-valor]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.valor === tx.plantilla)));
  $$("#fuente [data-valor]").forEach((b) => b.setAttribute("aria-pressed", String(b.dataset.valor === tx.fuente)));
  $("#fila-pagina").hidden = f.tipo !== "pagina";
  $("#fila-imagen").hidden = f.tipo !== "imagen";
  $("#fila-color").hidden = f.tipo !== "color";
  $("#ajustes-imagen").hidden = f.tipo === "color";
  $("#campos-texto").hidden = tx.plantilla === "ninguna";
  if (imagenPropia?.name) $("#imagen-nombre").textContent = imagenPropia.name;
  sincronizarColores();
}

function pintarPortada() {
  editorPortada.pintar(portada);
  actualizarResumen();
}

async function dibujarPortada() {
  if (!pdf) return;
  sincronizarControlesPortada();
  if (portada.fondo.tipo === "pagina") {
    const n = Math.max(0, Math.min(pdf.numPages - 1, Number(portada.fondo.pagina || 1) - 1));
    await cargarPaginaPortada(n);
  } else {
    editorPortada.imagen = portada.fondo.tipo === "imagen" ? imagenPropia?.bitmap || null : null;
    pintarPortada();
  }
}

async function elegirImagen(file) {
  if (!file || !file.type.startsWith("image/")) return;
  try {
    const bitmap = await createImageBitmap(file);
    imagenPropia?.bitmap?.close?.();
    imagenPropia = { bitmap, name: file.name };
    portada.fondo.tipo = "imagen";
    editorPortada.imagen = bitmap;
    sincronizarControlesPortada();
    pintarPortada();
  } catch (e) { avisar(t("elegirImagenError", { detalle: e.message }), true); }
}

function enlazarPortada() {
  colorBotones("#fila-color", COLORES_FONDO, "fondo", t("fondoColor"));
  colorBotones("#fila-color-texto", COLORES_TEXTO, "color", t("colorTexto"));
  colorBotones("#fila-color-acento", COLORES_ACENTO, "acento", t("colorAcento"));
  $$("#fondo-tipo [data-valor]").forEach((b) => b.addEventListener("click", () => {
    portada.fondo.tipo = b.dataset.valor;
    if (portada.fondo.tipo === "imagen" && !imagenPropia) portada.fondo.tipo = "pagina";
    dibujarPortada().catch((e) => avisar(e.message, true));
  }));
  $$("#fondo-ajuste [data-valor]").forEach((b) => b.addEventListener("click", () => { portada.fondo.ajuste = b.dataset.valor; pintarPortada(); sincronizarControlesPortada(); }));
  $$("#plantilla [data-valor]").forEach((b) => b.addEventListener("click", () => { portada.texto.plantilla = b.dataset.valor; pintarPortada(); sincronizarControlesPortada(); }));
  $$("#fuente [data-valor]").forEach((b) => b.addEventListener("click", () => { portada.texto.fuente = b.dataset.valor; pintarPortada(); sincronizarControlesPortada(); }));
  for (const [id, prop] of [["pt-titulo", "titulo"], ["pt-subtitulo", "subtitulo"], ["pt-autor", "autor"]]) {
    $("#" + id).addEventListener("input", (e) => {
      portada.texto[prop] = e.target.value;
      if (prop === "titulo") tituloPortadaAutomatico = false;
      if (prop === "autor") autorPortadaAutomatico = false;
      pintarPortada();
    });
  }
  for (const [id, obj, prop] of [["zoom", "fondo", "zoom"], ["oscurecer", "fondo", "oscurecer"], ["escala", "texto", "escala"]]) {
    $("#" + id).addEventListener("input", (e) => { portada[obj][prop] = Number(e.target.value); pintarPortada(); });
  }
  $("#pag-ant").addEventListener("click", () => cargarPaginaPortada(Number(portada.fondo.pagina) - 2).catch(() => {}));
  $("#pag-sig").addEventListener("click", () => cargarPaginaPortada(Number(portada.fondo.pagina)).catch(() => {}));
  $("#pag-num").addEventListener("change", () => cargarPaginaPortada(Number($("#pag-num").value) - 1).catch(() => {}));
  $("#btn-imagen").addEventListener("click", () => $("#in-imagen").click());
  $("#in-imagen").addEventListener("change", (e) => elegirImagen(e.target.files?.[0]));
  $("#btn-restablecer").addEventListener("click", () => reiniciarPortada(datosFormulario()));
  const lienzo = $("#lienzo"), area = $("#portada-caja");
  area.addEventListener("dragover", (e) => { e.preventDefault(); area.classList.add("encima"); });
  area.addEventListener("dragleave", () => area.classList.remove("encima"));
  area.addEventListener("drop", (e) => {
    e.preventDefault(); area.classList.remove("encima");
    if (e.dataTransfer.files[0]) elegirImagen(e.dataTransfer.files[0]);
  });
  let arrastre = null;
  lienzo.addEventListener("pointerdown", (e) => { if (portada.fondo.tipo !== "color" && portada.fondo.ajuste === "cubrir") { arrastre = { x: e.clientX, y: e.clientY }; lienzo.setPointerCapture(e.pointerId); } });
  lienzo.addEventListener("pointermove", (e) => {
    if (!arrastre) return;
    const r = lienzo.getBoundingClientRect();
    editorPortada.mover(portada, (e.clientX - arrastre.x) * lienzo.width / r.width, (e.clientY - arrastre.y) * lienzo.height / r.height);
    arrastre = { x: e.clientX, y: e.clientY };
    pintarPortada();
  });
  lienzo.addEventListener("pointerup", () => { arrastre = null; });
  $("#form-datos").addEventListener("input", () => {
    const d = datosFormulario();
    if (tituloPortadaAutomatico) portada.texto.titulo = d.titulo;
    if (autorPortadaAutomatico) portada.texto.autor = d.autor;
    if (seccion === "portada") pintarPortada();
    $("#archivo-nombre").title = archivoActual?.name || "";
    actualizarResumen();
  });
}

async function prepararPortada() {
  await dibujarPortada();
  editorPortada.pintar(portada);
  return subir(await comoBlob($("#lienzo"), "image/jpeg", 0.94));
}

async function crearPaginasFijas(token) {
  const paginas = [];
  const alta = $("input[name=calidad]:checked").value === "alta";
  const lado = alta ? 2400 : 1500;
  const calidad = alta ? 0.94 : 0.88;
  for (let n = 0; n < pdf.numPages; n++) {
    comprobarCancelado(token);
    progreso(t("guardandoPaginas"), t("paginaDe", { n: n + 1, total: pdf.numPages }), n / pdf.numPages, true);
    const c = await paginaCanvas(n, lado);
    const recurso = await subir(await comoBlob(c, "image/jpeg", calidad));
    paginas.push(recurso.id);
  }
  return paginas;
}

async function crearEpub() {
  const datos = datosFormulario();
  if (!datos.titulo) { avisar(t("tituloRequerido"), true); cambiarSeccion("datos"); $("#form-datos [name=titulo]").focus(); return; }
  if (!capitulos.some((c) => c.incluir)) { avisar(t("sinContenido"), true); cambiarSeccion("capitulos"); return; }
  const token = { cancelado: false };
  tokenTrabajo = token;
  $("#btn-crear").disabled = true;
  try {
    progreso(t("guardandoPortada"), datos.titulo, null, true);
    const portadaRecurso = await prepararPortada();
    comprobarCancelado(token);
    let paginas = [];
    if (modo === "fijo") paginas = await crearPaginasFijas(token);
    comprobarCancelado(token);
    progreso(t("creandoEpub"), datos.titulo, null, false);
    const pedido = {
      modo,
      datos,
      portada: portadaRecurso.id,
      capitulos: capitulos.map((c) => ({ titulo: c.titulo.trim(), desde: c.desde, pagina: c.pagina, incluir: c.incluir })),
      paginas,
    };
    const res = await api("/api/crear", pedido);
    $("#res-nombre").textContent = res.nombre;
    $("#res-nombre").title = res.nombre;
    $("#res-peso").textContent = formatoPeso(res.peso);
    $("#resultado").hidden = false;
    cerrarProgreso();
    avisar(t("epubCreado"));
  } catch (err) {
    cerrarProgreso();
    if (err?.name !== "AbortError") avisar(textoError(err), true);
  } finally {
    $("#btn-crear").disabled = false;
    $("#btn-crear").hidden = false;
  }
}

function formatoPeso(n) {
  if (n < 1024) return `${n} B`;
  if (n < 1024 * 1024) return `${(n / 1024).toFixed(0)} KB`;
  return `${(n / 1024 / 1024).toFixed(1)} MB`;
}

function restablecerEditor() {
  $("#editor").hidden = true;
  $("#inicio").hidden = false;
  $("#resultado").hidden = true;
  $("#btn-crear").hidden = false;
  archivoActual = null;
  analisis = null;
  documento = null;
  capitulos = [];
  if (pdf) pdf.destroy().catch(() => {});
  pdf = null;
}

function instalarEventos() {
  $("#btn-elegir").addEventListener("click", () => $("#in-pdf").click());
  $("#in-pdf").addEventListener("change", (e) => { abrirArchivo(e.target.files?.[0]); e.target.value = ""; });
  $("#btn-otro").addEventListener("click", () => $("#in-pdf").click());
  $("#soltar").addEventListener("dragover", (e) => { e.preventDefault(); $("#soltar").classList.add("encima"); });
  $("#soltar").addEventListener("dragleave", () => $("#soltar").classList.remove("encima"));
  $("#soltar").addEventListener("drop", (e) => { e.preventDefault(); $("#soltar").classList.remove("encima"); abrirArchivo(e.dataTransfer.files[0]); });
  let arrastres = 0;
  window.addEventListener("dragenter", (e) => { if (e.dataTransfer?.types.includes("Files")) { e.preventDefault(); arrastres++; $("#arrastre").hidden = false; } });
  window.addEventListener("dragleave", (e) => { if (e.dataTransfer?.types.includes("Files")) { arrastres = Math.max(0, arrastres - 1); if (!arrastres) $("#arrastre").hidden = true; } });
  window.addEventListener("dragover", (e) => { if (e.dataTransfer?.types.includes("Files")) e.preventDefault(); });
  window.addEventListener("drop", (e) => {
    if (!e.target.closest("#soltar, #portada-caja")) { e.preventDefault(); abrirArchivo(e.dataTransfer.files[0]); }
    arrastres = 0; $("#arrastre").hidden = true;
  });
  $$("[data-seccion]").forEach((b) => b.addEventListener("click", () => cambiarSeccion(b.dataset.seccion)));
  $$('input[name="modo"]').forEach((r) => r.addEventListener("change", () => { if (r.checked) cambiarModo(r.value); }));
  $("#btn-dividir").addEventListener("click", () => dividirEnBloque(bloqueElegido));
  $("#btn-dividir-pagina").addEventListener("click", () => dividirEnPagina(paginaElegida));
  $("#prev-ant").addEventListener("click", () => seleccionarCapitulo(capActual - 1));
  $("#prev-sig").addEventListener("click", () => seleccionarCapitulo(capActual + 1));
  $("#btn-crear").addEventListener("click", crearEpub);
  $("#btn-cancelar").addEventListener("click", () => { if (tokenTrabajo) tokenTrabajo.cancelado = true; cerrarProgreso(); });
  $("#btn-cambiar-carpeta").addEventListener("click", async () => {
    try { const r = await api("/api/carpeta/elegir", {}); config.carpeta = r.carpeta; actualizarResumen(); }
    catch (e) { avisar(e.message, true); }
  });
  $("#btn-abrir-carpeta").addEventListener("click", () => api("/api/carpeta/abrir", {}).catch((e) => avisar(e.message, true)));
  $("#btn-mostrar").addEventListener("click", () => api("/api/carpeta/abrir", { ultimo: true }).catch((e) => avisar(e.message, true)));
  $("#form-datos").addEventListener("submit", (e) => e.preventDefault());

  if (typeof window.caArrastrar === "function") {
    $("#controles").hidden = false;
    const barra = $("#barra"), interactivo = "button, a, input, select, textarea";
    barra.addEventListener("mousedown", (e) => { if (e.button === 0 && e.detail < 2 && !e.target.closest(interactivo)) window.caArrastrar(); });
    const max = async () => {
      const estado = await window.caMaximizada();
      document.body.classList.toggle("maximizada", estado);
      $("#btn-max").setAttribute("aria-label", t(estado ? "restaurar" : "maximizar"));
    };
    barra.addEventListener("dblclick", (e) => { if (!e.target.closest(interactivo)) window.caMaximizar(); });
    $("#btn-min").addEventListener("click", () => window.caMinimizar());
    $("#btn-max").addEventListener("click", () => window.caMaximizar().then(max));
    $("#btn-cerrar").addEventListener("click", () => window.caCerrar());
    for (const borde of $$("[data-borde]")) borde.addEventListener("mousedown", () => window.caRedimensionar(borde.dataset.borde));
    max();
  }
}

async function iniciar() {
  window.aplicarIdioma("es");
  instalarEventos();
  enlazarPortada();
  cargarFuentes().catch(() => {});
  try {
    const estado = await api("/api/estado");
    config = estado.config || config;
    $("#version").textContent = t("version", { version: estado.version || "1.0.1" });
    document.body.classList.remove("cargando");
    actualizarResumen();
  } catch (e) {
    document.body.classList.remove("cargando");
    avisar(e.message, true);
  }
  document.title = "PdfEpub";
  void versionPdfjs;
}

iniciar();
