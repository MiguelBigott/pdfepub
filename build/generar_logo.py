"""Logo de PdfEpub «Cosido», en el estilo del códice de Codex: tres hojas de pergamino apiladas,
cosidas al lomo con puntadas de oro, y el marcapáginas rojo de la rúbrica. Las hojas sueltas del
PDF se encuadernan en un libro.

    python build/generar_logo.py

Escribe:
  web/logo.svg       logo para el encabezado de la app
  build/logo.png     logo para el README y para Codex (256 px)
  build/app.ico      icono del .exe: versión reforzada en 16-32 px, normal desde 36 px
  build/lateral.bmp  imagen lateral del instalador (al doble de tamaño, para pantallas con escala)
Requiere Pillow y Microsoft Edge (renderiza el SVG).
"""
import os
import subprocess
import tempfile
import time

from PIL import Image

BUILD = os.path.dirname(os.path.abspath(__file__))
RAIZ = os.path.dirname(BUILD)
EDGE = r"C:\Program Files (x86)\Microsoft\Edge\Application\msedge.exe"

TINTA, BORDE, PERGAMINO, PERGAMINO_OSCURO, ORO, RUBRICA = "#16120e", "#3a3128", "#ece2cf", "#d8ccb2", "#c9a45c", "#c8483a"
VIOLETA = "#a48be0"


def svg(chico=False):
    """Logo en viewBox 512. `chico`: dos hojas más grandes y puntadas más gruesas para 16-32 px."""
    if chico:
        partes = [
            '<rect x="184" y="70" width="240" height="304" rx="10" fill="{po}"/>',
            '<rect x="120" y="112" width="250" height="304" rx="10" fill="{p}" stroke="{t}" stroke-width="14"/>',
            '<path d="M196 206 H316 M196 278 H296" stroke="{t}" stroke-width="24" stroke-linecap="round"/>',
            '<path d="M90 176 H160 M90 264 H160 M90 352 H160" stroke="{o}" stroke-width="30" stroke-linecap="round"/>',
            '<path d="M296 416 V482 L316 464 L336 482 V416 Z" fill="{r}"/>',
        ]
        borde = 0
    else:
        partes = [
            '<rect x="196" y="92" width="210" height="270" rx="6" fill="{po}"/>',
            '<rect x="172" y="114" width="210" height="270" rx="6" fill="{p}" stroke="{t}" stroke-width="8"/>',
            '<rect x="148" y="136" width="210" height="270" rx="6" fill="{p}" stroke="{t}" stroke-width="8"/>',
            '<path d="M196 198 H320 M196 236 H320 M196 274 H320 M196 312 H282" stroke="{t}" stroke-width="10" stroke-linecap="round"/>',
            '<path d="M128 174 H170 M128 228 H170 M128 282 H170 M128 336 H170" stroke="{o}" stroke-width="14" stroke-linecap="round"/>',
            '<path d="M296 406 V462 L306 452 L316 462 V406 Z" fill="{r}"/>',
        ]
        borde = 10
    cuerpo = "\n  ".join(x.format(p=PERGAMINO, po=PERGAMINO_OSCURO, t=TINTA, o=ORO, r=RUBRICA) for x in partes)
    return f'''<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 512 512" width="512" height="512">
  <rect x="8" y="8" width="496" height="496" rx="112" fill="{TINTA}" stroke="{BORDE}" stroke-width="{borde}"/>
  {cuerpo}
</svg>
'''


def renderizar(contenido_html, ancho, alto, destino):
    with tempfile.TemporaryDirectory(ignore_cleanup_errors=True) as perfil:
        html = os.path.join(perfil, "p.html")
        with open(html, "w", encoding="utf-8") as f:
            f.write(contenido_html)
        if os.path.exists(destino):
            os.remove(destino)
        subprocess.run([EDGE, "--headless", "--disable-gpu", "--hide-scrollbars", "--allow-file-access-from-files",
                        f"--user-data-dir={os.path.join(perfil, 'edge')}", "--default-background-color=00000000",
                        f"--window-size={ancho},{alto}", "--virtual-time-budget=3000",
                        f"--screenshot={destino}", "file:///" + html.replace(os.sep, "/")],
                       check=True, capture_output=True, timeout=60)
        # Edge puede volver antes de escribir la captura (la guarda otro de sus procesos): se espera
        # a que el archivo exista y deje de crecer.
        tamano = -1
        for _ in range(150):
            time.sleep(0.2)
            if os.path.exists(destino) and os.path.getsize(destino) == tamano > 0:
                return
            tamano = os.path.getsize(destino) if os.path.exists(destino) else -1
        raise RuntimeError(f"Edge no generó {destino}")


def png_de_svg(texto_svg, lado, destino):
    with tempfile.TemporaryDirectory() as tmp:
        ruta = os.path.join(tmp, "logo.svg")
        with open(ruta, "w", encoding="utf-8") as f:
            f.write(texto_svg)
        renderizar(f'<html><body style="margin:0;background:transparent"><img src="file:///{ruta.replace(os.sep, "/")}" '
                   f'width="{lado}" height="{lado}" style="display:block"></body></html>', lado, lado, destino)
    im = Image.open(destino).convert("RGBA")
    im.load()
    return im


def lateral(logo_png, destino):
    fuente = os.path.join(RAIZ, "web", "fonts", "inter-latin-600-normal.woff2").replace(os.sep, "/")
    with tempfile.TemporaryDirectory() as tmp:
        png = os.path.join(tmp, "lateral.png")
        renderizar(f'''<html><head><style>
@font-face {{ font-family: Titulo; src: url("file:///{fuente}"); font-weight: 600; }}
html, body {{ margin: 0; width: 328px; height: 628px; overflow: hidden; background: {TINTA}; }}
body {{ display: flex; flex-direction: column; align-items: center; color: {PERGAMINO}; }}
img {{ width: 200px; height: 200px; margin-top: 150px; }}
h1 {{ margin: 30px 0 0; font-family: Titulo; font-weight: 600; font-size: 46px; letter-spacing: -.03em; }}
h1 span {{ color: {VIOLETA}; }}
</style></head><body><img src="file:///{logo_png.replace(os.sep, "/")}"><h1>Pdf<span>Epub</span></h1></body></html>''', 328, 628, png)
        Image.open(png).convert("RGB").save(destino, format="BMP")


def main():
    with open(os.path.join(RAIZ, "web", "logo.svg"), "w", encoding="utf-8", newline="\n") as f:
        f.write(svg())
    with tempfile.TemporaryDirectory() as tmp:
        normal = png_de_svg(svg(), 1024, os.path.join(tmp, "normal.png"))
        chico = png_de_svg(svg(chico=True), 1024, os.path.join(tmp, "chico.png"))
        normal.resize((256, 256), Image.LANCZOS).save(os.path.join(BUILD, "logo.png"))
        tam = [16, 20, 24, 30, 32, 36, 40, 48, 64, 128, 256]
        cuadros = [(chico if s <= 32 else normal).resize((s, s), Image.LANCZOS) for s in tam]
        cuadros[-1].save(os.path.join(BUILD, "app.ico"), format="ICO",
                         sizes=[(s, s) for s in tam], append_images=cuadros[:-1])
        logo_lat = os.path.join(tmp, "lat.png")
        normal.resize((400, 400), Image.LANCZOS).save(logo_lat)
        lateral(logo_lat, os.path.join(BUILD, "lateral.bmp"))
    print("logo generado")


if __name__ == "__main__":
    main()
