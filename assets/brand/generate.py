#!/usr/bin/env python3
"""Editable, original Mafsil identity geometry. No font files or external art."""
from pathlib import Path
import html

HERE = Path(__file__).resolve().parent
INK, ACCENT, PAPER, MUTED = "#1c2c32", "#ea693c", "#f5f2e9", "#697c80"

def svg(w, h, body, title):
    return f'<svg xmlns="http://www.w3.org/2000/svg" width="{w}" height="{h}" viewBox="0 0 {w} {h}" role="img" aria-label="{html.escape(title)}"><title>{html.escape(title)}</title>{body}</svg>\n'

def icon(x=0, y=0, scale=1, color=INK, accent=ACCENT):
    return f'<g transform="translate({x} {y}) scale({scale})"><path fill="{color}" d="M18 96V24h22l24 24v28L40 52v44z"/><path fill="{accent}" d="M68 48l22-24h22v72H90V52L68 76z"/></g>'

def wordmark(x, y, scale=1, color=INK):
    # Custom single-line glyphs, intentionally independent of a typeface file.
    paths = 'M0 56V24Q0 12 12 12Q24 12 24 24V56M24 24Q24 12 36 12Q48 12 48 24V56 M106 12V56M106 34a22 22 0 1 0-44 0a22 22 0 1 0 44 0 M126 56V10Q126-6 145-6M115 18H143 M183 18C169 6 150 13 152 25C153 38 180 33 181 46C182 59 162 62 150 51 M203 26V56 M225-5V44Q225 56 238 56'
    return f'<g transform="translate({x} {y}) scale({scale})" fill="none" stroke="{color}" stroke-width="7" stroke-linecap="round" stroke-linejoin="round"><path d="{paths}"/><circle cx="203" cy="8" r="0.8" stroke-width="8"/></g>'

def text(x, y, value, size=20, color=INK, weight=400):
    return f'<text x="{x}" y="{y}" fill="{color}" font-family="Segoe UI,DejaVu Sans,sans-serif" font-size="{size}" font-weight="{weight}">{html.escape(value)}</text>'

def lockup(bg, fg):
    return f'<rect width="860" height="230" fill="{bg}"/>' + icon(48, 34, 1.28, fg) + wordmark(243, 59, 1.58, fg) + text(242, 190, "Workspace tools. On your terms.", 20, fg)

def social():
    return f'<rect width="1280" height="640" fill="{INK}"/><path d="M1000 0v640M0 496h1280" stroke="#35464b"/>' + icon(55, 52, 1.2, PAPER) + wordmark(242, 76, 1.65, PAPER) + text(76, 315, "Your agent.", 68, PAPER, 600) + text(76, 394, "Your workspace.", 68, PAPER, 600) + text(80, 553, "MCP tools for Windows + Linux", 26, PAPER) + text(80, 592, "Read only by default. Capabilities you choose.", 19, "#aab8b6") + icon(977, 207, 2.0, PAPER)

def main():
    (HERE / "logo-light.svg").write_text(svg(860,230,lockup(PAPER,INK),"Mafsil — Workspace tools. On your terms."),encoding="utf-8",newline="\n")
    (HERE / "logo-dark.svg").write_text(svg(860,230,lockup(INK,PAPER),"Mafsil — dark identity"),encoding="utf-8",newline="\n")
    for name,fg,ac in (("icon",INK,ACCENT),("icon-dark",PAPER,ACCENT),("icon-mono",INK,INK)):
        (HERE / f"{name}.svg").write_text(svg(128,128,icon(color=fg,accent=ac),"Mafsil icon"),encoding="utf-8",newline="\n")
    (HERE / "social.svg").write_text(svg(1280,640,social(),"Mafsil — Your agent. Your workspace."),encoding="utf-8",newline="\n")
    body=f'<rect width="1600" height="1430" fill="{PAPER}"/>'
    body+=text(70,75,"MAFSIL  /  IDENTITY PROPOSAL",21,MUTED,600)+text(70,145,"A clear connection. A deliberate boundary.",44,INK,600)
    body+=f'<g transform="translate(55 205) scale(.84)">{lockup(PAPER,INK)}</g>'
    body+=f'<g transform="translate(817 205) scale(.84)">{lockup(INK,PAPER)}</g>'
    body+=text(70,445,"01  PRIMARY / LIGHT",16,MUTED)+text(832,445,"02  PRIMARY / DARK",16,MUTED)
    body+=icon(70,484,1.05)+icon(237,484,1.05,INK,INK)
    body+=text(425,520,"03  STANDALONE / SCALE",16,MUTED)
    for x,size in ((428,16),(505,24),(594,32),(697,48),(814,64)):
        body+=icon(x,551,size/128)+text(x,653,str(size)+" px",14,MUTED)
    for x,color,label in ((1055,INK,"INK"),(1210,ACCENT,"EMBER"),(1365,"#dfded5","PAPER")):
        body+=f'<rect x="{x}" y="520" width="110" height="84" rx="8" fill="{color}"/>'+text(x,637,label,14,MUTED)
    body+=text(70,720,"04  SOCIAL PREVIEW / 1280 × 640",16,MUTED)
    body+=f'<g transform="translate(70 755) scale(.93)">{social()}</g>'
    body+=text(1308,815,"SOURCE",15,MUTED,600)+text(1308,850,"Editable SVG",17,INK)+text(1308,900,"Original glyphs",17,INK)+text(1308,950,"No font bundle",17,INK)
    body+=text(70,1393,"MAFSIL   /   Original vector identity   /   MIT",16,MUTED)
    (HERE / "preview.svg").write_text(svg(1600,1430,body,"Mafsil identity review: light and dark logos, icon sizes, palette and social preview"),encoding="utf-8",newline="\n")
    architecture=f'<rect width="1100" height="410" rx="12" fill="{PAPER}"/>'
    architecture+=text(40,53,"Small surface. Clear capabilities.",28,INK,600)
    for x,y,w,h,title,sub in ((40,128,235,150,"MCP client","Local stdio connection"),(369,128,296,150,"Mafsil","Read only by default"),(765,87,295,99,"Workspace files","Conditional, precise edits"),(765,228,295,99,"Processes + terminals","Explicitly enabled")):
        architecture+=f'<rect x="{x}" y="{y}" width="{w}" height="{h}" rx="6" fill="{INK if title=="Mafsil" else "#e8e8df"}"/>'
        architecture+=text(x+22,y+42,title,25,PAPER if title=="Mafsil" else INK,600)+text(x+22,y+75,sub,16,PAPER if title=="Mafsil" else MUTED)
    architecture+='<path d="M278 202h86m-10-7l10 7-10 7M670 202h43V136h47m-10-7l10 7-10 7M713 202v75h47m-10-7l10 7-10 7" fill="none" stroke="'+ACCENT+'" stroke-width="3"/>'
    architecture+=text(41,364,"File tools stay within your workspace. Commands run with your account’s authority.",18,MUTED)
    (HERE.parent / "architecture.svg").write_text(svg(1100,410,architecture,"Mafsil architecture and capability boundaries"),encoding="utf-8",newline="\n")

if __name__ == "__main__":
    main()
