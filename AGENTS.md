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
- Dropdowns are Pines UI style components on Alpine sharing one popup style
  (`.popup`, `.option`, `.option-danger`, `.popup-separator`); never add a
  bare `<select>` or a one-off popover. Menus: `views.Menu(label)` (compact,
  for card rows) and `views.PageMenu(label)` (page-head size) with `MenuLink`,
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
  Remembered custom options carry `data-custom` and can be renamed (items on
  lists the actor owns only, never started trips) or removed under
  `/categories`; results reach every picker via the `categories-changed`
  HX-Trigger (header values must stay ASCII).
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
  or runtime toggles (`.btn-*`, `.card`/`.card-brand`, `.item-row`, `.pack-row`,
  `.tick`, `.progress-*`, `.menu-*`, `.select-*`, `.popup`/`.option`, `.error`,
  `.dialog`, `.category-more`, `.status-toast`, `.sync-status`, the `drop-*`/toast transitions, and the
  `.muted`/`.tag` the offline scripts set) plus rules for elements the offline
  scripts create without classes. Utilities are generated from `internal/views`
  and `static/offline` only.
- `go test ./...` and `go vet ./...`
- `go run ./cmd/web` (requires database, Google OAuth, session, and CSRF env vars; see README.md)

## Design system

- Source of truth: `.claude/skills/camplist-design/` (readme, tokens, guidelines,
  logo assets, and a React click-through kit for reference only). Invoke
  `/camplist-design` for brand context before designing new UI.
- Runtime: tokens live in the `@theme` block of `assets/css/tailwind.css`;
  logomarks live in `static/`. Views combine utilities (fields, page and card
  heads are utility strings) with the component classes listed under Build /
  run; the React components are not shipped.
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
