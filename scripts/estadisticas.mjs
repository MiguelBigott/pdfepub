// Genera el panel de estadísticas del README (SVG) con la API de GitHub, sin servicios de terceros.
// Solo usa cantidades de actividad del repositorio (commits, pull requests, pushes, releases); ningún dato personal.
// Uso: GITHUB_TOKEN=... REPO=owner/repo node scripts/estadisticas.mjs > estadisticas.svg
const TOKEN = process.env.GITHUB_TOKEN, REPO = process.env.REPO;
if (!TOKEN || !REPO) { console.error("Faltan GITHUB_TOKEN y REPO"); process.exit(1); }

const DIA = 86_400_000, N = 30;
const hoy = new Date(); hoy.setUTCHours(0, 0, 0, 0);
const inicio = new Date(hoy.getTime() - (N - 1) * DIA), inicioPrev = new Date(inicio.getTime() - N * DIA);

async function api(ruta) {
  const r = await fetch(`https://api.github.com/repos/${REPO}${ruta}`, {
    headers: { Authorization: `Bearer ${TOKEN}`, Accept: "application/vnd.github+json", "X-GitHub-Api-Version": "2022-11-28" } });
  if (!r.ok) throw new Error(`GitHub API ${r.status} en ${ruta}`);
  return r.json();
}
async function todas(ruta, max = 10) {
  const out = [];
  for (let p = 1; p <= max; p++) { const pag = await api(`${ruta}${ruta.includes("?") ? "&" : "?"}per_page=100&page=${p}`); out.push(...pag); if (pag.length < 100) break; }
  return out;
}
// Cuenta fechas por día (últimos N días) y en el período anterior.
function serie(fechas) {
  const dias = new Array(N).fill(0); let prev = 0;
  for (const f of fechas) {
    const t = new Date(f).getTime();
    if (t >= inicio.getTime()) { const i = Math.floor((t - inicio.getTime()) / DIA); if (i < N) dias[i]++; }
    else if (t >= inicioPrev.getTime()) prev++;
  }
  return { dias, total: dias.reduce((a, b) => a + b, 0), prev };
}

const [commits, prs, eventos, releases] = await Promise.all([
  todas(`/commits?sha=main&since=${inicioPrev.toISOString()}`),
  todas(`/pulls?state=all&sort=created&direction=desc`, 3),
  todas(`/events`, 3),
  todas(`/releases`, 3),
]);
const c = serie(commits.map((x) => x.commit.committer.date));
const prA = serie(prs.map((p) => p.created_at)), prC = serie(prs.filter((p) => p.closed_at).map((p) => p.closed_at));
const pu = serie(eventos.filter((e) => e.type === "PushEvent").map((e) => e.created_at));
const publicadas = releases.filter((r) => !r.draft && r.published_at);
const re = serie(publicadas.map((r) => r.published_at));
const ultima = publicadas.find((r) => !r.prerelease)?.tag_name ?? "—";

// ---------------- Dibujo (estilo tarjetas oscuras) ----------------
const C = { bg: "#0d1117", borde: "#30363d", sub: "#8b949e", verde: "#3fb950", rojo: "#f85149",
  mechanicus: "#4d74c9", dorado: "#e8ad2e", morado: "#c25ae6", moradoOsc: "#8957e5", naranja: "#f0883e" };
const esc = (s) => String(s).replace(/[&<>"]/g, (ch) => ({ "&": "&amp;", "<": "&lt;", ">": "&gt;", '"': "&quot;" })[ch]);
const W = 960, PAD = 12, GAP = 12, CW = (W - 2 * PAD - 2 * GAP) / 3;
const tarjeta = (x, y, w, h, contenido) => `<g transform="translate(${x},${y})"><rect width="${w}" height="${h}" rx="8" fill="${C.bg}" stroke="${C.borde}"/>${contenido}</g>`;
const variacion = (act, prev) => {
  const d = act - prev, pct = prev ? Math.round((d / prev) * 100) : act ? 100 : 0;
  const sube = d >= 0, col = sube ? C.verde : C.rojo;
  return { texto: `${d >= 0 ? "+" : ""}${d} (${pct >= 0 ? "+" : ""}${pct}%)`, flecha: sube ? "↑" : "↓", col };
};
function resumen(x, y, titulo, color, act, prev, etiqueta = "mes anterior") {
  const v = variacion(act, prev);
  return tarjeta(x, y, CW, 78, `<text x="14" y="30" fill="${color}" font-size="17" font-weight="600">${esc(titulo)}</text>
    <text x="14" y="62" fill="${C.sub}" font-size="12" font-family="ui-monospace,Consolas,monospace">${esc(v.texto)}</text>
    <text x="${CW - 14}" y="62" fill="${v.col}" font-size="12" text-anchor="end" font-family="ui-monospace,Consolas,monospace">${v.flecha}  vs ${etiqueta}</text>`);
}
function barras(x, y, titulo, series, leyenda) {
  const h = 130, max = Math.max(1, ...series.flatMap((s) => s.dias));
  const area = { x: 12, y: 56, w: CW - 24, h: h - 64 }, bw = area.w / N;
  let b = "";
  series.forEach((s, k) => s.dias.forEach((n, i) => {
    if (!n) return;
    const alto = Math.max(3, (n / max) * area.h), ancho = series.length > 1 ? bw / 2 - 1 : bw - 2;
    b += `<rect x="${(area.x + i * bw + (series.length > 1 ? k * (bw / 2) : 1)).toFixed(1)}" y="${(area.y + area.h - alto).toFixed(1)}" width="${Math.max(1.5, ancho).toFixed(1)}" height="${alto.toFixed(1)}" rx="1" fill="${s.color}"/>`;
  }));
  const ley = leyenda.map((l, i) => `<circle cx="${CW - 92}" cy="${24 + i * 17}" r="5" fill="${l.color}"/><text x="${CW - 82}" y="${28 + i * 17}" fill="${l.color}" font-size="12">${esc(l.texto)}</text>`).join("");
  return tarjeta(x, y, CW, h, `<text x="14" y="30" fill="${titulo.color}" font-size="16" font-weight="600">${esc(titulo.texto)}</text>${ley}${b}
    <line x1="${area.x}" y1="${area.y + area.h + 0.5}" x2="${area.x + area.w}" y2="${area.y + area.h + 0.5}" stroke="${C.borde}"/>`);
}

const maxC = Math.max(1, ...c.dias);
const cuadros = c.dias.map((n, i) => {
  const op = n ? 0.35 + 0.65 * (n / maxC) : 1, col = n ? C.mechanicus : "#2a2f3a";
  return `<rect x="${W - 2 * PAD - 14 - (N - i) * 13}" y="23" width="10" height="10" rx="2" fill="${col}" fill-opacity="${op.toFixed(2)}"/>`;
}).join("");
const y1 = PAD, y2 = y1 + 56 + GAP, y3 = y2 + 78 + GAP, H = y3 + 130 + PAD;

const svg = `<svg xmlns="http://www.w3.org/2000/svg" width="${W}" height="${H}" viewBox="0 0 ${W} ${H}" font-family="-apple-system,BlinkMacSystemFont,'Segoe UI',Helvetica,Arial,sans-serif">
<title>Actividad del repositorio PNGWebp (últimos ${N} días)</title>
${tarjeta(PAD, y1, W - 2 * PAD, 56, `<g transform="translate(14,18)" fill="${C.mechanicus}"><rect width="18" height="4" rx="1"/><rect y="7" width="18" height="4" rx="1"/><rect y="14" width="18" height="4" rx="1"/></g>
  <text x="42" y="34" fill="${C.mechanicus}" font-size="17" font-weight="600">${c.total} ${c.total === 1 ? "contribución" : "contribuciones"} en los últimos ${N} días</text>${cuadros}`)}
${resumen(PAD, y2, `Versión ${ultima}`, C.dorado, re.total, re.prev)}
${resumen(PAD + CW + GAP, y2, `${prA.total} pull requests abiertos`, C.morado, prA.total, prA.prev)}
${resumen(PAD + 2 * (CW + GAP), y2, `${pu.total} pushes`, C.naranja, pu.total, pu.prev)}
${barras(PAD, y3, { texto: "Releases", color: C.dorado }, [{ dias: re.dias, color: C.dorado }], [{ texto: "Publicadas", color: C.dorado }])}
${barras(PAD + CW + GAP, y3, { texto: "Pull requests", color: C.morado }, [{ dias: prA.dias, color: C.morado }, { dias: prC.dias, color: C.moradoOsc }], [{ texto: "Abiertos", color: C.morado }, { texto: "Cerrados", color: C.moradoOsc }])}
${barras(PAD + 2 * (CW + GAP), y3, { texto: "Pushes", color: C.naranja }, [{ dias: pu.dias, color: C.naranja }], [{ texto: "Pushes", color: C.naranja }])}
</svg>
`;
process.stdout.write(svg);
