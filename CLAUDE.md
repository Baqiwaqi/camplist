# camplist

A learning project. Go + chi + templ + htmx, backed by Azure Cosmos DB.

## How to help me

This is a project I'm using to learn. Default to **teaching, not doing**:

- For refactors, cleanups, "clean up X", structure, or "how should I…" requests:
  **point out what to change and why — do not edit the files for me.**
  Give `file:line` references, name the idiom/pattern, and a short illustrative
  snippet at most. I write the actual code.
- Order suggestions by payoff and call out anything that's a real bug vs. style.
- It's fine to fix a concrete bug directly **when I explicitly ask you to fix it**
  (e.g. "fix this query", "the delete isn't working"). When in doubt, explain first.
- Don't silently restructure working code. Surface the option, let me decide.

## Stack notes

- Routing: `go-chi/chi/v5`. Handlers currently inline in `cmd/web/main.go`.
- Views: `templ` — edit `*.templ`, then run `templ generate` before building.
  Never hand-edit the generated `*_templ.go` files.
- Frontend: `htmx` (loaded in `internal/views/layout.templ`).
- CSRF: `gorilla/csrf`, field name `_csrf`, header `X-CSRF-Token`.
  Note: Go's `ParseForm` ignores request bodies for `DELETE`, so send the CSRF
  token via the `X-CSRF-Token` header (not a form field) on htmx delete requests.
- Storage: Cosmos DB. Soft-delete via a `deletedAt` patch; `deletedAt` is
  `omitempty`, so active docs have **no** field — filter with
  `NOT IS_DEFINED(l.deletedAt)`, not `IS_NULL(...)`.

## Build / run

- `go build ./...`
- `templ generate` after changing any `.templ`
- `go run ./cmd/web` (needs `DB_URL`, `DB_KEY`, `CSRF_KEY` env vars)
