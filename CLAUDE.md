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
  element that needs it (menu, error toast); there is no separate app script. Packing updates return
  a checklist fragment; other mutations may redirect or refresh.
- CSRF: `gorilla/csrf`, field name `_csrf`, header `X-CSRF-Token`.
  Note: Go's `ParseForm` ignores request bodies for `DELETE`, so send the CSRF
  token via the `X-CSRF-Token` header (not a form field) on htmx delete requests.
- Storage: Cosmos DB. Lists and sessions share a container; filter queries by
  `type` as well as user ID. Lists soft-delete via a `deletedAt` patch; `deletedAt` is
  `omitempty`, so active docs have **no** field — filter with
  `NOT IS_DEFINED(l.deletedAt)`, not `IS_NULL(...)`.

## Build / run

- `go build ./...`
- `templ generate` after changing any `.templ`
- `go test ./...` and `go vet ./...`
- `go run ./cmd/web` (requires database, Google OAuth, session, and CSRF env vars; see README.md)

## Design system

- Source of truth: `.claude/skills/camplist-design/` (readme, tokens, guidelines,
  logo assets, and a React click-through kit for reference only). Invoke
  `/camplist-design` for brand context before designing new UI.
- Runtime: tokens are inlined at the top of `static/style.css`; logomarks live in
  `static/`. Views use the `.btn-*`, `.card`, `.item-row`, `.field`, `.error`,
  `.progress`, `.tag`, `.eyebrow` classes; the React components are not shipped.
- Rules: one ember (`.btn-accent`) "go" action per screen, pine for primary
  actions, red only for errors and delete; border-only elevation (no shadows);
  pills for buttons and tags; flat colour, no icons, images or gradients;
  h1/h2 in Bricolage Grotesque 800, body in system-ui; must work on phones.
