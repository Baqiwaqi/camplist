# camplist

A learning project. Go + chi + templ + htmx, backed by Azure Cosmos DB.

## How to help me

The teaching-only phase is complete. Default to **implementing requested changes**:

- You are authorized to edit files and implement improvements, bug fixes,
  refactors, and cleanups within the requested scope. Do not stop at instructions
  for me to write the code or ask permission for each routine change.
- Explain what changed, why, and how it was verified. I can still learn from
  the implementation without having to write every change myself.
- Prioritize changes by payoff and distinguish correctness fixes from style.
- Keep changes focused on the task and run appropriate checks.
- When I explicitly ask for an explanation or a teaching exercise, follow that
  preference for the request.

## Stack notes

- Routing: `go-chi/chi/v5`. Routes and handlers live in `internal/web/`; `cmd/web/main.go` wires dependencies.
- Views: `templ` — edit `*.templ`, then run `templ generate` before building.
  Never hand-edit the generated `*_templ.go` files.
- Frontend: `htmx` plus `Alpine.js` (both vendored in `static/`, loaded in
  `internal/views/layout.templ`). Client-side state lives in `x-data` on the
  element that needs it (menu, error toast, confirm dialog); packing persistence/sync lives in the explicit
  `static/offline/` modules. Deletes and access removals keep `hx-confirm`; the layout dialog
  intercepts `htmx:confirm`, and `data-confirm-action` / `data-confirm-detail`
  on the triggering element supply the label and explanation. Packing updates and the
  list page's item and preparation controls return fragments; other mutations
  may redirect or refresh. Trip page forms keep the focused form in place and swap
  only what changed by id (`hx-swap-oob` / `hx-select-oob`); once
  `static/offline/session.mjs` mounts it owns the checklist and cancels server
  swaps into it. The list page holds its ETag once in
  `#list-revision`: `static/revision.js` sends it as the `revision` field or
  `X-Camplist-Revision` header of every htmx request there, so a fragment save
  must swap `views.ListRevision(list, true)` out of band with the revision
  its own write made (see `EditPreparationTask`, `listItemsChanged`). Failed htmx
  requests answer with a user-facing plain-text reason (`http.Error`); the
  layout's error toast (`static/request-error.js`) shows it for statuses under
  500, offers Reload on 409 and CSRF failures (`X-Camplist-Error: csrf`), and
  auto-hides after 6s unless it offers Reload or reports a lost connection.
- htmx mutations: branch on `isHTMX(r)` (`internal/web/item.go`) and give real
  forms a plain 303 fallback. Stop double submits with `hx-sync` (plus
  `hx-indicator` for a wider busy area), styled by the shared `.htmx-request`
  rule in `assets/css/tailwind.css`; never `hx-disabled-elt`, which drops
  keyboard focus. Swapped content keeps focus via stable ids or `autofocus`,
  and live regions (`role="status"`) stay outside swapped elements.
- Shared components live in `internal/views/components.templ`: `PageHead`
  (back link, title, one meta line, actions as children) and `SectionHead`
  (title with count, at most one text action); `Field(TextField{...})` for
  every input and textarea; `Dialog(id, sheet, attrs)` for every modal (the
  confirm dialog is its centred variant, `sheet` rises from the bottom on
  phones); `ToastStack`/`Toast` at the bottom edge, ink for a confirmation and
  red for a failure; `Switch` for a yes/no setting and `Chip` for a pill
  multi-select, both keeping a real checkbox as the submitted field;
  `TabLinks` (page switch, plain links, no script) and `TabGroup`/`TabPanel`
  (panels in place, sliding marker in `static/tabs.js`, strip hidden and
  panels stacked without scripts). Use these instead of pasting class strings
  or adding a one-off.
- Dropdowns are Pines UI style components on Alpine sharing one popup style
  (`.popup`, `.option`, `.option-danger`, `.option-new`, `.option-action`,
  `.popup-separator`); never add a bare `<select>` or a one-off popover.
  Menus: `views.Menu(label)` (compact, for card rows) and `views.PageMenu(label)` (page-head size) with `MenuLink`,
  `MenuButton` and `MenuSeparator` children (trigger, popup with
  `role="menu"`, items). Their behaviour is one `Alpine.data("menu")` in
  `static/menu.js`, loaded before Alpine; it flips the popup right
  (`.menu-popup-start`) or up (`.menu-popup-up`) to stay on screen. Build
  destructive items with the `confirmDelete` helper so the confirm dialog stays consistent;
  `confirmDeleteCard` removes a card in place (the handler branches on `HX-Target`).
- Form dropdowns: `views.Select` (stacked field) or `views.SelectField`
  (grids and inline rows; `ScopeSelect` wraps it) render a listbox
  (`static/select.js`) over a hidden native `<select>`, which is the submitted
  field, the no-JS fallback and the source of the shown value after an htmx
  swap, error re-render or form reset. Toggle their popup with `hidden`, not
  `x-show`: htmx settles `style`/`class` on same-id elements. The offline shell
  copies the trip entry selects; `TestOfflineShellSelectsMatchSelectField`
  keeps the copy in sync.
- Item categories: every category field is `views.CategoryPicker` (combobox in
  `static/category-picker.js`; the text input posts `category`, options come
  from its datalist). Build options with `handler.categorySuggestions`:
  `packing.DefaultCategories`, the items in view, then the actor's remembered
  categories (one `item-categories` doc per user partition). Call
  `rememberCategory` after saving an item. Categories compare trimmed and
  case-insensitively; never rewrite the category of a replayable operation.
  Remembered custom options carry `data-custom` and an Edit button that opens
  `views.CategoryDialog` to rename (items on lists the actor owns only, never
  started trips) or remove them; `/categories` does the same without scripts.
  Results reach every picker via the `categories-changed` HX-Trigger (header
  values must stay ASCII). The picker's popup is a `role="grid"`, not a
  listbox: ARIA forbids interactive descendants inside an option, so the Edit
  button is its own `role="gridcell"` beside the category and left/right
  arrows move between them.
- Bulk add (`internal/web/bulk.go`): Add several and Add from a list are
  pages without scripts and panels loaded into the list page's `#bulk-add`
  dialog (`.dialog-sheet`, a bottom sheet below 721px). A finished add answers
  only out-of-band parts, which empties the panel and closes the dialog.
  `Store.AddItems` skips names already on the list or repeated in the
  same add (trimmed, case-insensitive); it and `AddItem` enforce the limits in
  `internal/packing/bulk.go` (2,000 items plus tasks per list).
- Public pages: `/` is the landing page for visitors (members get their lists
  there via `auth.OptionalAuth`), `/demo` is a packing session held only in
  Alpine state, `/login` is the sign-in page. They use `views.PublicHeader` and
  `views.PublicFooter` on `Shell`; `Layout` is for signed-in pages only.
- CSRF: `gorilla/csrf`, field name `_csrf`, header `X-CSRF-Token`. `Layout`
  sets the header once via `hx-headers` on `<body>`, so htmx requests (including
  `DELETE`, whose body Go ignores) need no per-element token; keep hidden `_csrf`
  fields on forms that must work without JS.
- History: `Shell` disables the htmx history cache (pages are `no-store`). A
  boosted POST that renders instead of redirecting gets `HX-Push-Url: false`
  from `render`, because htmx 2 ignores `hx-push-url="false"` on boosted forms.
- Storage: Cosmos DB. Lists and sessions share a container; filter queries by
  `type` as well as user ID. Lists soft-delete via a conditional replacement setting `deletedAt`; `deletedAt` is
  `omitempty`, so active docs have **no** field — filter with
  `NOT IS_DEFINED(l.deletedAt)`, not `IS_NULL(...)`.

## Build / run

- `go build ./...`
- `templ generate` after changing any `.templ`
- `npm run css` builds `static/tailwind.css` (gitignored, the only stylesheet)
  from `assets/css/tailwind.css`; Air and the Dockerfile run it too. Tailwind v4
  is the styling system, with Preflight on: the `@theme` block holds the
  Camplist tokens (`bg-pine-700`, `text-muted`, `rounded-card`, `rounded-field`,
  `font-display`, the `text-meta`/`text-caption` type roles; body sizes keep
  1.5 leading) and removes Tailwind's default colours, shadows and breakpoints,
  so `max-sm:` means phones (<= 600px) and `md:` the wide layouts (>= 721px).
  Style with utilities in the templ views; `@layer base` re-adds body colour,
  headings, links, focus ring and checkboxes, and `@layer components` keeps the
  display type roles (`.t-*`) and the classes that carry state, pseudo elements
  or runtime toggles (`.btn-*`, `.card`/`.card-brand`/`.card-link`, `.item-row`, `.pack-row`,
  `.tick`, `.progress-*`, `.menu-*`, `.select-*`, `.popup`/`.option`,
  `.field`/`.label`/`.hint`, `.page-head`/`.section-head`/`.back-link`,
  `.tabs`/`.tab`, `.switch`, `.chip`, `.error`, `.toast`, `.dialog`,
  `.sync-status`, the `drop-*`/toast transitions, and the `.muted`/`.tag` the
  offline scripts set) plus rules for elements the offline scripts create
  without classes. Utilities are generated from
  `internal/views` and `static/offline` only.
- `go test ./...` and `go vet ./...`
- `go run ./cmd/web` (requires database, Google OAuth, session, and CSRF env vars; see README.md)

## Design system

- Source of truth: `.claude/skills/camplist-design/` (readme, tokens, guidelines,
  logo assets, and a React click-through kit for reference only). Invoke
  `/camplist-design` for brand context before designing new UI.
- Runtime: tokens live in the `@theme` block of `assets/css/tailwind.css`;
  logomarks live in `static/`. Views combine utilities with the shared templ
  components and the component classes listed under Build / run; the React
  components are not shipped. A page head is a back link, a title, one meta
  line and actions at natural width; a section is a title with its count and
  at most one text action, then one white sheet of hairline rows. List and
  trip cards open through one title link with `.card-link` (it covers the
  card); keep the More menu after that link in the markup so it stays above
  the cover without a z-index. A trip card's tags sit in its `data-card-tags`
  row, where `account.mjs` adds "Available offline". Gear on a list page is
  one sheet grouped by category (`groupByCategory`), each group head with its
  count; a resting row is the name, tags and one ghost Edit, and Delete lives
  in the edit row. Item saves there swap all of `#list-items` (`ListGear`),
  since they can regroup it.
- The offline shell `static/offline/offline.html` is static HTML on the same
  stylesheet. Its scripts in `static/offline/` look up elements by id and set
  classes such as `.error`, `.card` and `.item-row` at runtime, so keep those
  ids and the `offline-page`, `saved-sessions`, `sync-status` and
  `offline-items` hooks when restyling, and add any new static asset to the
  list in `sw.js`. The worker serves those assets network-first (cache only as
  offline fallback), so a restyle needs no cache version bump; bump `CACHE`
  only when the asset list changes.
- Rules: one ember (`.btn-go`) "go" action per screen, pine for primary
  actions, red only for errors and delete (solid `.btn-danger` only in confirm
  dialogs); border-only elevation (no shadows); pills for buttons and tags;
  flat colour, no icons, images or gradients; titles in Bricolage Grotesque 800
  on the type roles (h1 page, h2 section, h3 or `.t-object` for names on cards),
  body in system-ui; sentence case, no uppercase eyebrows or tracked labels;
  must work on phones.

## Shared resources

- Keep authenticated actor identity separate from the resource's immutable storage
  `userId`. Membership lives on the canonical resource; discovery references are
  not access grants. Trip and template grants are independent.
- Authorize every read/write/replay. Writes must condition on the same resource
  ETag as the permission check. Shared template forms carry their displayed revision.
- Send device snapshots through `PackingSession.Snapshot`; never serialize raw
  memberships, invitations, receipts or review metadata into packing pages/APIs.
- Preserve item revisions and operation IDs through offline retries. A reconnect
  is not a successful sync. Shared conflicts need an explicit user decision.
- A deleted or no-longer-accessible trip goes through `OfflinePacking.gone`
  (`static/offline/packing.mjs`): the copy is dropped unless it holds unsynced
  work, which keeps a sticky `gone` marker until a successful refresh. The Trips
  overview asks `reconcile` before showing any copy the server did not list.
- Sharing behavior and limits: `docs/shared-lists-implementation.md`.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
