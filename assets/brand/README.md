# Mafsil identity

The split M connects two solid parts across an intentional gap. The wordmark uses original vector glyphs; it does not embed a font. The palette is ink (`#1c2c32`), ember (`#ea693c`) and paper (`#f5f2e9`).

| Asset | Use |
| --- | --- |
| `logo-light.svg` / `.png` | Primary light-background lockup and README fallback |
| `logo-dark.svg` / `.png` | Dark-background lockup and README dark mode |
| `icon.svg` / `.png` | Standalone primary icon |
| `icon-dark.svg` / `.png` | Standalone icon on dark surfaces |
| `icon-mono.svg` | Single-color printing or constrained displays |
| `mafsil.ico` | Windows 16, 24, 32, 48, 64, 128 and 256 pixel icon frames |
| `social.svg` / `.png` | GitHub social preview, 1280×640 |
| `preview.svg` / `.png` | Identity contact sheet |
| `../architecture.svg` | README capability diagram |

Keep clear space of at least half an icon stem around the mark. Do not fill the central gap, stretch glyphs, add third-party logos or use the accent alone for small body text. Use the monochrome icon where two colors are unavailable.

`generate.py` is the editable geometry source for SVGs. Text outside the wordmark uses system sans-serif fallbacks; no font files are distributed. Raster exports and ICO frames can be regenerated with an SVG renderer. All original Mafsil assets are covered by the repository's MIT license.
