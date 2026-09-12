# Camplist Design System

Camplist is a small server-rendered web app for preparing camping packing lists and checking items off as you pack. A **packing list** is a reusable template; a **packing session** is a snapshot of that list for one trip — checking items in a session never alters the list. It is a personal learning project (Go + chi + templ + HTMX + Azure Cosmos DB) with Google sign-in. One product, one surface: the web app.

## Sources

- Codebase (mounted read-only): `camplist/` — views in `internal/views/*.templ`, the single stylesheet `static/style.css` (66 lines), copy in `internal/web/handler.go` and `internal/packing/form.go`, positioning in `README.md` and `docs/market-research.md`.
- No Figma, no brand guide, no icon set, no webfonts, no decks were provided. The source had no logo; the user granted creative freedom, so the tent logomark and pine accent in this system are **new brand work**, not recreations.

## Direction: "get outside"

The system should make you want to go. Three levers: **ember** (one orange "go" action per screen, and progress that turns ember when you're packed), **display type** (Bricolage Grotesque 800, tight, short imperative headlines — "Where to next?", "Pack once. Go every weekend.", "All packed. Go!"), and **momentum cues** (progress bars, pine ticks, "Keep packing"). Everything else stays calm sand and white so the accent reads as a signal.

## Content fundamentals

- **Voice:** plain, second person, short imperative sentences with forward motion: "Where to next?", "Let's get packing.", "Pick a list and start a session — your list stays untouched.", "All packed. Go!" Source copy was terse and had typos ("all you lists", "Keep is simple!"); the kit rewrites those lines — port them back to the templates.
- **Eyebrows** are uppercase tracked labels naming the context: YOUR LISTS · PACKING SESSION · SEP 5, 2026.
- **Casing:** page titles and button labels are Title Case ("Create New", "Add Item", "Delete Session", "New Packing List"); a few are sentence case ("Start session", "Login with google"); the nav's "signout" is all-lowercase. Labels are single words ("Name", "Category", "Description").
- **Buttons are verbs:** Create, Edit, Details, Delete, Save, Check / Uncheck, Start session.
- **Confirmations** are terse questions: "Delete this list?", "Delete this item?", "Delete this session".
- **Errors** are short declaratives without periods: "Name is required", "Storing packing list failed".
- **Status text** is literal key: value — "checked items: 3 / 7", "Session ID: …", "List Name: …".
- **Placeholders** are examples, not instructions: "Camping", "A short description of your list".
- No emoji, no exclamation beyond the one onboarding line, no marketing adjectives. Dates render as "Jan 2, 2006".

## Visual foundations

- **Palette:** a warm sand page (`#f6f5f2`) with pure-white cards and header; near-black blue-grey ink (`#1f2933`) for all text and links; one muted grey (`#6b7280`). Borders are three tints of sand (`#ddd6cc` header, `#e5ded3` card, `#eee7dd` row). Error red trio (`#fee2e2` / `#fca5a5` / `#b91c1c`). **Brand: pine green** (`--brand` `#2f5d3a`, hover `#1f3f28`, soft `#e3ede4`) for the logomark, primary buttons, hero cards, ticks, category tags, progress. **Accent: ember orange** (`--accent` `#e8762b`, hover `#c4581a`, soft `#fdebd9`) for exactly one "go" action per screen, eyebrows, and completed progress. The page stays sand + white + ink.
- **Type:** display = Bricolage Grotesque 800 (Google Fonts, loaded via `@import` in `tokens/typography.css`), 1.05 leading, −0.02em tracking, sizes 1.5 / 2.75 / 3.5rem. Body = `system-ui` 400/700. Eyebrows and category tags: 12px uppercase, 0.08em / 0.04em tracking. Nav links are bold, undecorated ink.
- **Spacing:** rem-based (0.25 / 0.5 / 0.75 / 1 / 2). Content column max 900px, centred; nav padding 1rem; main padding 2rem 1rem; card padding 1rem with 1rem bottom margin; rows 0.75rem vertical padding and 0.75rem gap.
- **Shape:** never use coloured left-border accents on cards or banners. Cards 12px radius; buttons, tags and progress bars are pills; fields and error box 4px; 1px borders everywhere. Elevation is border-only — **no shadows**, inner or outer. Hero/session-header cards may be solid pine (`tone="brand"`).
- **Backgrounds:** flat colour only. No images, gradients, patterns, textures, blur or transparency.
- **Cards:** white, 1px sand border, 12px radius, 1rem padding, stacked with 1rem gap; rows inside separated by hairlines, last row borderless.
- **Motion:** 120ms ease on colour + 1px lift for buttons; 240ms ease on progress width/colour; tick fills instantly. No bounces, no page transitions.
- **Hover / press:** buttons darken one step (pine 700→900, ember 500→700) and lift 1px; secondary tints to page sand; links dim to 70%. No shrink on press.
- **Layout rules:** header is static (not sticky), full width, content centred at 900px. Page actions sit top-right of the title block via flex space-between.
- **Imagery:** none. If added, keep it warm and natural (campsite, daylight), never cool-toned or stylised.
- **Iconography:** none — see below.

## Iconography

The product has no icons: no icon font, no SVGs, no PNGs, no unicode glyphs, no emoji. Every action is a text label. Keep it that way; if an icon set becomes necessary, prefer a thin-stroke set (e.g. Lucide from CDN) and flag it as an addition. **Logo:** `assets/logo.svg` (mark + wordmark), `assets/logomark.svg` (pine), `assets/logomark-white.svg` (for pine/dark grounds). A minimal tent silhouette with a door cut-out on a ground line — flat pine, no gradients. Wordmark is bold system-ui, never a custom face. Minimum mark size 22px (header); 56px on the login card. See `guidelines/brand-name.html`.

## Components

Inventory taken from `static/style.css` (`.card`, `.item-row`, `.muted`, `.error`, `header/nav`) and the native controls the templates use. All in `components/core/`:

- **Card** — `.card`
- **ItemRow** — `.item-row`
- **ErrorBanner** — `.error`, redesigned: solid red block, white glyph, display heading, 12px radius, no borders
- **AppHeader** — `header > nav` with brand, Sessions, signout, user name
- **Field** — `label + input` / `textarea` as used in the list and item forms
- **Button** — the templates' `<button>` elements (primary / accent / secondary / danger / link)
- **ProgressBar** — packing progress (intentional addition)

`.muted` is exposed as the `--text-muted` token rather than a component.

### Intentional additions
- **Button variants** (primary / secondary / danger / link) and Field styling: the source ships browser-default controls; styled from the palette so the kit is usable. Marked in `tokens/colors.css`.
- **Pine brand, ember accent, tent logomark, Bricolage display face, ProgressBar**: created at the user's request for a "get outside" feel (source had none of these).

## Index

- `styles.css` — entry; imports `tokens/colors.css`, `tokens/typography.css`, `tokens/spacing.css`, `tokens/shape.css`
- `assets/` — `logo.svg`, `logomark.svg`, `logomark-white.svg`
- `guidelines/` — specimen cards: Colors (surfaces, text, brand, accent, semantic, button additions), Type (families, headings, eyebrow & tags, scale, links), Spacing (scale, page layout), Shape (radius, card anatomy), Brand (logo)
- `components/core/` — seven components above + `core.card.html`
- `ui_kits/camplist/` — click-through web app (login, lists, new/edit list, details, session, sessions overview); see its README
- `thumbnail.html` — homepage tile
- `SKILL.md` — agent skill wrapper
