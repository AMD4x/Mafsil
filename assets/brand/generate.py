#!/usr/bin/env python3
"""Original Mafsil vector masters and their applications. No font files."""
from pathlib import Path
import html

HERE = Path(__file__).resolve().parent
INK, LAGOON, FOAM, MIST, SLATE = "#163447", "#007C83", "#6AE6CA", "#F1F7F8", "#526B78"
LINE, WHITE = "#C7D9DF", "#FFFFFF"


def svg(width, height, body, title, viewbox=None):
    viewbox = viewbox or f"0 0 {width} {height}"
    title = html.escape(title)
    return f'<svg xmlns="http://www.w3.org/2000/svg" width="{width}" height="{height}" viewBox="{viewbox}" role="img" aria-label="{title}"><title>{title}</title>{body}</svg>\n'


def save(name, width, height, body, title, viewbox=None):
    (HERE / name).write_text(svg(width, height, body, title, viewbox), encoding="utf-8", newline="\n")


def group(body, x=0, y=0, scale=1):
    return f'<g transform="translate({x} {y}) scale({scale})">{body}</g>'


def rect(x, y, width, height, fill, radius=0, stroke=None):
    edge = f' stroke="{stroke}" stroke-width="1.5"' if stroke else ""
    return f'<rect x="{x}" y="{y}" width="{width}" height="{height}" rx="{radius}" fill="{fill}"{edge}/>'


def text(x, y, content, size=24, color=INK, weight=400, utility=False):
    family = "Consolas,DejaVu Sans Mono,monospace" if utility else "Segoe UI,DejaVu Sans,sans-serif"
    return f'<text x="{x}" y="{y}" font-family="{family}" font-size="{size}" font-weight="{weight}" fill="{color}">{html.escape(content)}</text>'


def mark(foreground=INK, accent=LAGOON):
    # A single 128-unit original: square proportions and an eight-unit joint.
    half = "M20 108V32C20 25.4 25.4 20 32 20H35L60 47V77L40 56V108Z"
    return f'<path d="{half}" fill="{foreground}"/><path d="{half}" transform="translate(128 0) scale(-1 1)" fill="{accent}"/>'


def wordmark(color=INK):
    # Original geometric glyphs. No typeface is embedded or outlined.
    glyphs = [
        "M5 80V35C5 24 12 18 24 18C36 18 43 25 43 36V80M43 36C43 25 50 18 62 18C74 18 81 25 81 36V80",
        "M158 49C158 31 147 18 131 18C114 18 103 31 103 49C103 67 114 80 131 80C147 80 158 67 158 49M158 18V80",
        "M192 80V19C192 6 199 0 212 0H218M177 30H216",
        "M270 25C257 14 233 16 230 31C225 53 270 44 270 64C270 83 240 87 228 74",
        "M292 32V80",
        "M324 0V64C324 76 329 80 342 80",
    ]
    paths = "".join(f'<path d="{d}"/>' for d in glyphs)
    return f'<g fill="none" stroke="{color}" stroke-width="9" stroke-linecap="round" stroke-linejoin="round">{paths}</g><circle cx="292" cy="9" r="5.2" fill="{color}"/>'


def lockup(dark=False):
    foreground, accent = (MIST, FOAM) if dark else (INK, LAGOON)
    return group(mark(foreground, accent), 8, 4, 1.9) + group(wordmark(foreground), 274, 45, 2)


def badge(dark=False, size=128):
    background, foreground, accent, border = (INK, MIST, FOAM, SLATE) if dark else (MIST, INK, LAGOON, SLATE)
    if size in (16, 24):
        # Optical small-size cut: the same structure, with a visible center gap.
        scale = size / 16
        left = "M3 13V4Q3 3 4 3H5L7.5 5.5V8.5L5 6V13Z"
        body = rect(.5, .5, size-1, size-1, background, 3.5*scale, border)
        body += group(f'<path d="{left}" fill="{foreground}"/><path d="{left}" transform="translate(16 0) scale(-1 1)" fill="{accent}"/>', scale=scale)
        return body
    return rect(1, 1, 126, 126, background, 28, border) + mark(foreground, accent)


def social():
    body = rect(0, 0, 1280, 640, MIST)
    # A joining tab makes the product's connection visible without extra symbols.
    body += '<path d="M878 0H1280V640H930C886 640 850 604 850 560V346H820V294H850V0Z" fill="'+INK+'"/>'
    body += group(lockup(), 40, 30, .59)
    body += text(70, 303, "Your workspace.", 68, INK, 650)
    body += text(70, 388, "Connected.", 68, LAGOON, 650)
    body += text(73, 514, "MCP tools for Windows + Linux", 28, INK, 500)
    body += text(73, 559, "Read only by default. You choose the capabilities.", 23, SLATE)
    body += group(mark(MIST, FOAM), 846, 154, 3.3)
    return body


def arrow(points, color=LAGOON):
    coords = points.split()
    x, y = map(float, coords[-1].split(","))
    px, py = map(float, coords[-2].split(","))
    head = f"M{x-7},{y-8}L{x},{y}L{x+7},{y-8}" if y > py else f"M{x-8},{y-7}L{x},{y}L{x-8},{y+7}"
    return f'<polyline points="{points}" fill="none" stroke="{color}" stroke-width="3" stroke-linejoin="round"/><path d="{head}" fill="none" stroke="{color}" stroke-width="3" stroke-linecap="round" stroke-linejoin="round"/>'


def architecture(mobile=False):
    if mobile:
        body = rect(0, 0, 400, 900, MIST, 24)
        body += text(32, 48, "One connection.", 29, INK, 650)
        body += text(32, 83, "Explicit capabilities.", 29, INK, 650)
        body += rect(32, 122, 336, 108, WHITE, 18, LINE)
        body += text(56, 164, "MCP client", 28, INK, 600) + text(56, 202, "Local stdio connection", 21, SLATE)
        body += arrow("200,232 200,269")
        body += rect(32, 274, 336, 134, INK, 18)
        body += group(mark(MIST, FOAM), 48, 286, .55) + group(wordmark(MIST), 133, 299, .57)
        body += text(59, 383, "Read only by default", 21, MIST)
        body += '<path d="M200 408V430H20V703" fill="none" stroke="'+LAGOON+'" stroke-width="3"/>'
        body += arrow("20,513 43,513") + arrow("20,703 43,703")
        for y, title, line1, line2 in ((445, "Workspace files", "Read and inspect", "Enable precise edits"), (635, "Commands + terminals", "Explicitly enabled", "Uses account permissions")):
            body += rect(48, y, 320, 146, WHITE, 18, LINE)
            body += text(69, y+42, title, 24, INK, 600)
            body += text(69, y+82, line1, 20, SLATE) + text(69, y+113, line2, 20, SLATE)
        body += text(32, 838, "File tools stay in your workspace.", 20, SLATE)
        return body
    body = rect(0, 0, 1280, 480, MIST, 24)
    body += text(44, 64, "One connection. Explicit capabilities.", 36, INK, 650)
    body += rect(44, 178, 270, 156, WHITE, 20, LINE)
    body += text(69, 237, "MCP client", 32, INK, 600) + text(69, 285, "Local stdio connection", 22, SLATE)
    body += arrow("316,256 387,256")
    body += rect(393, 178, 304, 156, INK, 20)
    body += group(mark(MIST, FOAM), 416, 188, .6) + group(wordmark(MIST), 510, 205, .45)
    body += text(419, 292, "Read only by default", 23, MIST)
    body += '<path d="M699 256H755V168M755 256V339" fill="none" stroke="'+LAGOON+'" stroke-width="3"/>'
    body += arrow("755,168 807,168") + arrow("755,339 807,339")
    for y, title, sub in ((103, "Workspace files", "Read, inspect, enable precise edits"), (275, "Commands + terminals", "Explicitly enabled")):
        body += rect(815, y, 421, 132, WHITE, 20, LINE)
        body += text(843, y+51, title, 29, INK, 600) + text(843, y+93, sub, 22, SLATE)
    body += text(44, 448, "File tools stay in the workspace. Enabled commands use account permissions.", 23, SLATE)
    return body


def main():
    save("mark-master.svg", 1024, 1024, mark(), "Mafsil original articulated M mark", "0 0 128 128")
    save("logo-master.svg", 2048, 512, lockup(), "Mafsil original vector logo", "0 0 1024 256")
    for dark in (False, True):
        name = "dark" if dark else "light"
        save(f"logo-{name}.svg", 1024, 256, lockup(dark), f"Mafsil logo for {name} backgrounds")
        save("icon-dark.svg" if dark else "icon.svg", 128, 128, badge(dark), f"Mafsil app icon, {name}")
    save("icon-mono.svg", 128, 128, mark(INK, INK), "Mafsil monochrome mark")
    for size in (16, 24):
        save(f"icon-{size}.svg", size, size, badge(size=size), f"Mafsil optical {size}-pixel app icon")
    save("social.svg", 1280, 640, social(), "Mafsil — Your workspace. Connected.")
    save("../architecture.svg", 1280, 480, architecture(), "Mafsil: MCP client connects to files and explicitly enabled command tools")
    save("../architecture-mobile.svg", 400, 900, architecture(True), "Mafsil capabilities, arranged for small screens")


if __name__ == "__main__":
    main()
