# Camplist Design System

Camplist is a small server-rendered web app for preparing camping packing lists and checking items off as you pack. A **packing list** is a reusable template; a **packing session** is a snapshot of that list for one trip — checking items in a session never alters the list. It is a personal learning project (Go + chi + templ + HTMX + Azure Cosmos DB) with Google sign-in. One product, one surface: the web app.

## Sources

- Codebase (mounted read-only): `camplist/` — views in `internal/views/*.templ`, the single stylesheet `static/style.css` (66 lines), copy in `internal/web/handler.go` and `internal/packing/form.go`, positioning in `README.md` and `docs/market-research.md`.
- No Figma, no brand guide, no icon set, no webfonts, no decks were provided. The source had no logo; the user granted creative freedom, so the tent logomark and pine accent in this system are **new brand work**, not recreations.

## Direction: "get outside"

The system should make you want to go. Three levers: **ember** (one orange "go" action per screen, and progress that turns ember when you're packed), **display type** (Bricolage Grotesque 800, tight, short imperative headlines — "Where to next?", "Pack once. Go every weekend.", "All packed. Go!"), and **momentum cues** (progress bars, pine ticks, "Keep packing"). Everything else stays calm sand and white so the accent reads as a signal.

## Content fundamentals

- **Voice:** plain, second person, short imperative sentences with forward motion: "Where to next?", "Let's get packing.", "Pick a list and start a session — your list stays untouched.", "All packed. Go!" Source copy was terse and had typos ("all you lists", "Keep is simple!"); the kit rewrites those lines — port them back to the templates.
- **No eyebrows:** page heads are a title, one meta line and actions. The nav already says where you are, so there is no uppercase label above the title.
- **Casing:** sentence case everywhere: titles, buttons, labels and tags ("Start trip", "Add an item", "Available offline"). No uppercase tracked labels. Some older strings are still Title Case ("Add Item", "Create New"); fix them when you touch them.
- **Buttons are verbs:** Create, Edit, Details, Delete, Save, Check / Uncheck, Start session.
- **Confirmations** are terse questions: "Delete this list?", "Delete this item?", "Delete this session".
- **Errors** are short declaratives without periods: "Name is required", "Storing packing list failed".
- **Status text** is literal key: value — "checked items: 3 / 7", "Session ID: …", "List Name: …".
- **Placeholders** are examples, not instructions: "Camping", "A short description of your list".
- No emoji, no exclamation beyond the one onboarding line, no marketing adjectives. Dates render as "Jan 2, 2006".

## Visual foundations

- **Palette:** a warm sand page (`#f6f5f2`) with pure-white cards and header; near-black blue-grey ink (`#1f2933`) for all text and links; one warm muted stone (`#6b645a`, 5.8:1 on white). Borders are tints of sand (`#ddd6cc` header and controls, `#e5ded3` card, `#eee7dd` row) plus a hover line (`#c9c0b3`, sand-400). Error red trio (`#fee2e2` / `#fca5a5` / `#b91c1c`). **Brand: pine green** (`--brand` `#2f5d3a`, hover `#1f3f28`, soft `#e3ede4`) for the logomark, primary buttons, hero cards, ticks, category tags, progress. **Accent: ember orange** (`--accent` `#e8762b`, hover `#c4581a`, soft `#fdebd9`) for exactly one "go" action per screen and completed progress. The page stays sand + white + ink.
- **Type roles** (1.2 ratio from 16px, `--size-*` in `tokens/typography.css`): moment 40px (finished trip, landing hero), page 32px on phones / 40px from 721px (the h1), section 22px (h2, dialog titles), object 19px (list and trip names on cards), all Bricolage Grotesque 800 (Google Fonts) with −0.02em tracking; body 16/1.5 `system-ui` (row names 600, text 400, buttons 700); meta 14/1.45 muted (dates, counts, hints; labels 600 ink); caption 13/1.35 (tags, group counts, save status). Nav links are bold, undecorated ink.
- **Spacing:** a 4px grid: 4 tag inner, 8 between buttons, 12 section title to sheet and card to card, 16 page gutter and field to field, 24 page head to content, 32 / 48 section to section on phones / desktop. Content column max 900px, centred; nav padding 1rem; main padding 2rem 1rem; card padding 1rem with 1rem bottom margin; rows 0.75rem vertical padding and 0.75rem gap.
- **Shape:** never use coloured left-border accents on cards or banners. Cards 12px radius; popups and dialogs 12px; buttons, tags, tabs and progress bars are pills; fields, select triggers and options 10px; checkboxes 6px; 1px borders everywhere. Elevation is border-only — **no shadows**, inner or outer. Hero/session-header cards may be solid pine (`tone="brand"`).
- **Backgrounds:** flat colour only. No images, gradients, patterns, textures, blur or transparency.
- **Cards:** white, 1px sand border, 12px radius, 1rem padding, stacked with 1rem gap; rows inside separated by hairlines, last row borderless. A list or trip card on an index page opens on tap: its title link (`.card-link`) stretches over the card, More sits beside the title, the status closes the card (tags, then progress or item count), 12px between cards and two columns from 721px.
- **Motion:** 120ms colour, 150ms switch, 160ms popup drop, 200ms tab marker, 240ms progress width/colour; tick fills instantly; all off under reduced motion. No lifts, bounces or page transitions.
- **Hover / press:** buttons darken one step (pine 700→900, ember 500→700); secondary tints to page sand with a sand-400 line; quiet steps sand-100→200; links dim to 70%. No shrink or lift.
- **Layout rules:** header is static (not sticky), full width, content centred at 900px. Page actions sit top-right of the title block via flex space-between.
- **Imagery:** none. If added, keep it warm and natural (campsite, daylight), never cool-toned or stylised.
- **Iconography:** none — see below.

## Iconography

The product has no icons: no icon font, no SVGs, no PNGs, no unicode glyphs, no emoji. Every action is a text label. Keep it that way; if an icon set becomes necessary, prefer a thin-stroke set (e.g. Lucide from CDN) and flag it as an addition. **Logo:** `assets/logo.svg` (mark + wordmark), `assets/logomark.svg` (pine), `assets/logomark-white.svg` (for pine/dark grounds). A minimal tent silhouette with a door cut-out on a ground line — flat pine, no gradients. Wordmark is bold system-ui, never a custom face. Minimum mark size 22px (header); 56px on the login card. See `guidelines/brand-name.html`.

## Shared component set (what ships)

The app's own components live in `internal/views/components.templ` with their
classes in `assets/css/tailwind.css`; the React kit below is reference only.
Reach for one of these rather than pasting class strings or adding a one-off:

- **PageHead** (`PageHeading{Title, Meta, BackHref, BackLabel}`, actions as
  children) — the way back on a child page, the title, one meta line, actions
  at natural width. No eyebrow, no full-width action bars.
- **SectionHead(title, count)** with at most one text action as children, then
  one white sheet of hairline rows.
- **Field(TextField{…})** — the one input and textarea: `.label`, `.field`,
  `.hint`. Every text control in the app is `.field`, so it matches the select
  trigger beside it.
- **One popup** (`.popup` + `.option`) behind all three dropdowns: `Menu` /
  `PageMenu` (`static/menu.js`), `Select` / `SelectField` / `ScopeSelect`
  (`static/select.js`) and `CategoryPicker` (`static/category-picker.js`).
  Option states are hover/active (`data-active`), selected
  (`aria-selected`, pine with a tick), `.option-danger` and `.option-new`.
  A row that carries a control of its own puts it in `.option-action` beside
  the option, never inside it — see the picker note below.
- **Tabs** — `TabLinks` switches pages as plain links (no script needed);
  `TabGroup` + `TabPanel` switch panels in place with the sliding marker in
  `static/tabs.js`. Without scripts the `TabGroup` strip stays hidden and its
  panels stay stacked.
- **Dialog(id, sheet, attrs)** — one modal: centred, or rising from the bottom
  edge on phones with `sheet`. The confirm dialog is its centred variant.
- **ToastStack / Toast(id, danger, attrs)** — messages at the bottom edge: ink
  for a confirmation, red for a failure (the request error toast).
- **Switch(name, label, checked, attrs)** for a yes/no setting and
  **Chip(name, value, label, checked)** for a pill multi-select; both keep a
  real checkbox as the submitted field.
- **ProgressBar** and **ErrorBanner** as before.

A combobox row may not hold a button inside its option: ARIA forbids
interactive descendants there, so screen readers never expose them. The
category picker's popup is therefore a `role="grid"` whose rows pair the
category cell with its Edit cell; left and right arrows move between them.

## React kit inventory (reference)

Inventory taken from `static/style.css` (`.card`, `.item-row`, `.muted`, `.error`, `header/nav`) and the native controls the templates use. All in `components/core/`:

- **Card** — `.card`
- **ItemRow** — `.item-row`
- **ErrorBanner** — `.error`, redesigned: solid red block, white glyph, display heading, 12px radius, no borders
- **AppHeader** — `header > nav` with brand, Sessions, signout, user name
- **Field** — `label + input` / `textarea` as used in the list and item forms
- **Button** — `.btn` with primary (pine), go (ember, once per screen), secondary (white outline), quiet (sand fill, overflow), ghost (resting row action), danger (solid red, confirm dialogs only), link; 44px, or 36px with `.btn-sm` (44px on coarse pointers). The React kit still names go `accent` and shows the older outline danger.
- **ProgressBar** — packing progress (intentional addition)

`.muted` is exposed as the `--text-muted` token rather than a component.

### Intentional additions
- **Button variants** (primary / secondary / danger / link) and Field styling: the source ships browser-default controls; styled from the palette so the kit is usable. Marked in `tokens/colors.css`.
- **Pine brand, ember accent, tent logomark, Bricolage display face, ProgressBar**: created at the user's request for a "get outside" feel (source had none of these).

## Index

- `styles.css` — entry; imports `tokens/colors.css`, `tokens/typography.css`, `tokens/spacing.css`, `tokens/shape.css`
- `assets/` — `logo.svg`, `logomark.svg`, `logomark-white.svg`
- `guidelines/` — specimen cards: Colors (surfaces, text, brand, accent, semantic, button additions), Type (families, headings, tags, scale, links), Spacing (scale, page layout), Shape (radius, card anatomy), Brand (logo)
- `components/core/` — seven components above + `core.card.html`
- `ui_kits/camplist/` — click-through web app (login, lists, new/edit list, details, session, sessions overview); see its README
- `thumbnail.html` — homepage tile
- `SKILL.md` — agent skill wrapper
