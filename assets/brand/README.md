# Mafsil identity

The articulated M joins two equal parts around an open center. Rounded shoulders and broad stems keep the mark clear; custom geometric lowercase glyphs give the wordmark its own voice. All core geometry is original and editable, with no embedded or outlined third-party typeface.

| Color | Value | Role |
| --- | --- | --- |
| Ink | `#163447` | Primary text and dark surfaces |
| Lagoon | `#007C83` | Mark and connections on light surfaces |
| Foam | `#6AE6CA` | Mark accent on dark surfaces |
| Mist | `#F1F7F8` | Light surfaces and reversed text |
| Slate | `#526B78` | Secondary text and icon outlines |

## Originals and exports

The original geometry is defined once in `generate.py`: `mark()` and `wordmark()` generate the vector masters and every application. SVG is resolution-independent. PNG exports are rendered directly from vectors rather than enlarged from an earlier bitmap.

| Asset | SVG / PNG dimensions | Use |
| --- | --- | --- |
| `mark-master.svg` / `.png` | 1024×1024 / **4096×4096** | Transparent original monogram |
| `logo-master.svg` / `.png` | 2048×512 / **4096×1024** | Transparent original full logo |
| `logo-light.svg` / `.png` | 1024×256 / 2048×512 | Logo on light surfaces |
| `logo-dark.svg` / `.png` | 1024×256 / 2048×512 | Logo on dark surfaces |
| `icon.svg` / `.png` | 128×128 / 1024×1024 | App icon with opaque Mist tile and a visible outline; works on either background |
| `icon-dark.svg` / `.png` | 128×128 / 1024×1024 | Optional reversed app tile |
| `icon-mono.svg` / `.png` | 128×128 / 1024×1024 | Transparent single-color mark on light backgrounds |
| `icon-16.svg`, `icon-24.svg` | 16×16, 24×24 | Optical small-size variants derived from the same structure |
| `mafsil.ico` | 16, 24, 32, 48, 64, 128 and 256 px | Windows icon frames, using optical variants at 16/24 px |
| `social.svg` / `.png` | 1280×640 | GitHub social preview; use this PNG for upload |
| `social-hd.png` | 2560×1280 | Larger export of the same 2:1 artwork |
| `../architecture.svg` / `.png` | 1280×480 / 2560×960 | Desktop README diagram |
| `../architecture-mobile.svg` / `.png` | 400×900 / 800×1800 | Mobile diagram with independently legible labels |

Use the full logo at its 4:1 aspect ratio, without an embedded tagline. The README supplies normal readable prose below it. The diagrams use a `picture` element to select the vertical layout at viewport widths of 600 px or below. The app icon is a separate application of the mark; do not substitute the transparent light-background mark for the universal Windows icon.

Keep at least 16 master units of clear space around the monogram. Never stretch the logo, fill the center joint or add third-party branding. Palette samples must use the exact values above. System-font text is limited to explanatory artwork; the core wordmark is all paths.

## Regeneration

Generate SVGs with Python 3, then render PNG and ICO exports with Node.js and an available installation of `sharp`:

```text
python assets/brand/generate.py
node assets/brand/render.cjs
```

Resolve `sharp` through the normal Node module path or a process-local `NODE_PATH` pointing to an existing tools directory. No global install is required. These commands generate artwork only; they never build Mafsil or release packages. Font substitutions can affect explanatory text, so visually inspect exports when changing the rendering environment.

All original Mafsil artwork uses the repository's MIT license. No font files are bundled.
