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
  `static/offline/` modules. Deletes keep `hx-confirm`; the dialog in the layout
  intercepts `htmx:confirm`, and `data-confirm-action` / `data-confirm-detail`
  on the button supply the label and explanation. Packing updates return
  a checklist fragment; other mutations may redirect or refresh.
- Reusable dropdown: `views.Menu(label)` (compact, for card rows) and
  `views.PageMenu(label)` (page-head size) with `MenuLink`, `MenuButton` and
  `MenuSeparator` children follow Base UI's Menu anatomy (trigger, popup with
  `role="menu"`, items). Their behaviour is one `Alpine.data("menu")` in
  `static/menu.js`, loaded before Alpine. Build destructive items with the
  `confirmDelete` helper so the CSRF header and confirm dialog stay consistent.
- Public pages: `/` is the landing page for visitors (members get their lists
  there via `auth.OptionalAuth`), `/demo` is a packing session held only in
  Alpine state, `/login` is the sign-in page. They use `views.PublicHeader` and
  `views.PublicFooter` on `Shell`; `Layout` is for signed-in pages only.
- CSRF: `gorilla/csrf`, field name `_csrf`, header `X-CSRF-Token`.
  Note: Go's `ParseForm` ignores request bodies for `DELETE`, so send the CSRF
  token via the `X-CSRF-Token` header (not a form field) on htmx delete requests.
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
  Camplist tokens (`bg-pine-700`, `text-muted`, `rounded-card`, `font-display`,
  `tracking-eyebrow`; body sizes keep 1.5 leading) and removes Tailwind's default
  colours, shadows and breakpoints, so `max-sm:` means phones (<= 600px) and
  `md:` the wide layouts (>= 721px). Style with utilities in the templ views;
  `@layer base` re-adds body colour, headings, links, focus ring and checkboxes,
  and `@layer components` keeps the classes that carry state, pseudo elements or
  runtime toggles (`.btn-*`, `.card`/`.card-brand`, `.item-row`, `.pack-row`,
  `.tick`, `.progress-*`, `.menu-*`, `.error`, `.dialog`, `.sync-status`, the
  `drop-*`/toast transitions) plus rules for elements the offline scripts create
  without classes. Utilities are generated from `internal/views` and
  `static/offline` only.
- `go test ./...` and `go vet ./...`
- `go run ./cmd/web` (requires database, Google OAuth, session, and CSRF env vars; see README.md)

## Design system

- Source of truth: `.claude/skills/camplist-design/` (readme, tokens, guidelines,
  logo assets, and a React click-through kit for reference only). Invoke
  `/camplist-design` for brand context before designing new UI.
- Runtime: tokens live in the `@theme` block of `assets/css/tailwind.css`;
  logomarks live in `static/`. Views combine utilities (eyebrows, tags, fields,
  page and card heads are utility strings) with the component classes listed
  under Build / run; the React components are not shipped.
- The offline shell `static/offline/offline.html` is static HTML on the same
  stylesheet. Its scripts in `static/offline/` look up elements by id and set
  classes such as `.error`, `.card` and `.item-row` at runtime, so keep those
  ids and the `offline-page`, `saved-sessions`, `sync-status` and
  `offline-items` hooks when restyling, and add any new static asset to the
  list in `sw.js`. The worker serves those assets network-first (cache only as
  offline fallback), so a restyle needs no cache version bump; bump `CACHE`
  only when the asset list changes.
- Rules: one ember (`.btn-accent`) "go" action per screen, pine for primary
  actions, red only for errors and delete; border-only elevation (no shadows);
  pills for buttons and tags; flat colour, no icons, images or gradients;
  h1/h2 in Bricolage Grotesque 800, body in system-ui; must work on phones.

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
- Sharing behavior and limits: `docs/shared-lists-implementation.md`.

## Maintaining this file

Keep this file for knowledge useful to almost every future agent session in this project.
Do not repeat what the codebase already shows; point to the authoritative file or command instead.
Prefer rewriting or pruning existing entries over appending new ones.
When updating this file, preserve this bar for all agents and keep entries concise.
