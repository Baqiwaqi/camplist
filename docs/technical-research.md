# Camplist technical research: Go and HTMX

Research date: 2026-09-12. Scope: repository inspection and current primary documentation, not a production audit. Recommendations are proposed learning exercises; application code was not changed. Related: [market research](market-research.md).

## Implementation follow-up

The initial findings below are preserved as the research baseline. The implementation now filters document types, validates point reads, fixes forms/deletion, starts independent unchecked sessions, supports reopening sessions and in-place checklist updates, and uses explicit packed state rather than toggle requests. Sign-out is a CSRF-protected POST; expired HTMX sessions trigger full login navigation. List replacements use ETags, session patches guard item identity, routing uses chi v5, and the server has timeouts, shutdown handling, and cookie security derived from the callback URL. Regression tests have been added. See [README](../README.md) for current usage.

Offline editing, household collaboration, reusable kits, and post-trip reviews remain product hypotheses. Storage tests verify calls and serialization using test doubles; live Cosmos behavior and browser interactions have not been verified in this implementation pass.

## Recommendation

Keep Go, chi, templ, and HTMX for the current online packing-list workflow. My assessment is that forms, lists, and individual check-off actions fit server-rendered HTML well. HTMX's own guidance identifies CRUD and updates to bounded page regions as good fits, while identifying full offline operation as difficult. [HTMX architecture guidance](https://htmx.org/essays/when-to-use-hypermedia/)

The next useful exercise is making one packing-session item update in place. First address the concrete data and validation defects below. There is no demonstrated need for a separate JSON API, frontend framework, microservices, or database migration.

## What the repository actually uses

| Component | Observed version and role |
| --- | --- |
| Go | `go.mod` declares `go 1.25.0`; HTTP handlers use `net/http`. |
| chi | Both v1.5.5 and `/v5` v5.3.0 appear in `go.mod`; current web imports use `github.com/go-chi/chi`, so the active router is v1. |
| templ | v0.3.1020; `.templ` components generate Go rendering code. |
| HTMX | Vendored `static/htmx.min.js` declares **2.0.10**, loaded by `internal/views/layout.templ:11`. |
| Storage | Azure `azcosmos` v1.4.2; one container client serves both document types. |
| Authentication | `go-oidc` v2.5.0, `oauth2` v0.36.0, Gorilla sessions v1.4.0 and CSRF v1.7.3. |

These are local observations, not claims that every dependency is current. HTMX's installation documentation currently also specifies 2.0.10. Reconcile chi imports and the duplicate dependency as a small maintenance exercise, separate from behavior changes. [HTMX installation](https://htmx.org/docs/#installing)

## Request and data architecture

`cmd/web/main.go:31` constructs the Cosmos client and packing store; `:53` constructs Google authentication; `:63` builds routes; `:74` wraps them with CSRF middleware.

```text
Browser action → CSRF middleware → chi route → authentication middleware
  → Go handler → packing store → Cosmos DB
  ← full templ page, HTML fragment, or navigation response
```

Today the fragment branch is an opportunity rather than the implemented mutation flow. Normal forms post and receive `303` redirects. HTMX deletions and check-offs return `HX-Refresh: true` (`internal/web/handler.go:222`, `:296`, `:375`, `:399`), which requests a full reload. Starting a session returns `HX-Redirect` (`:327`). These are valid HTMX behaviors. [HTMX response headers](https://htmx.org/docs/#response-headers)

Google login uses state, authorization-code exchange, and ID-token verification (`internal/auth/client.go:105`, `:144`). The authenticated subject becomes the user ID. These mechanisms correspond to Google's documented OpenID Connect flow. [Google OpenID Connect](https://developers.google.com/identity/openid-connect/openid-connect)

Store calls use that user ID as the partition key. `NewPackingSession` embeds a list snapshot (`internal/packing/packing.go:9`); subsequent check-offs patch the session document, preserving the template in storage. Lists are soft-deleted, while sessions are physically deleted (`internal/packing/store.go:166`, `:80`).

## Priorities grounded in this code

### 1. Correct data selection and visible form behavior

**Data defect inferred from code:** `internal/packing/store.go:48` and `:116` omit a `type` condition. Both query the same container configured at `cmd/web/main.go:31`. Names such as `sessions s` and `lists l` in `FROM` do not route queries to separate containers; the supplied container is the query context. Consequently, each query can select both document types and decode them into the wrong Go shape. Add explicit document-type conditions and validate type on point reads. This was not reproduced against the live database. [Cosmos FROM semantics](https://learn.microsoft.com/en-us/cosmos-db/query/from)

**Confirmed source defects:** `internal/views/packing.templ:17` omits `/` between `packing-list` and the ID. `internal/web/handler.go:262` replaces the submitted item form, discarding its values and validation errors; `internal/views/packing-details.templ:33` has no error rendering. Preserve the submitted form, restore server-owned action/button metadata, and display errors. Fix these before adding AJAX to forms.

### 2. Learn partial rendering through check-off

Reuse `PackingSessionItem` (`internal/views/packing-session.templ:21`). Give each row an explicit target, set `hx-swap="outerHTML"`, and return the updated row after persistence. HTMX supports selecting the replacement target, and templ documents directly rendering components for HTMX. [HTMX targets](https://htmx.org/attributes/hx-target/), [swap modes](https://htmx.org/attributes/hx-swap/), [templ integration](https://templ.guide/server-side-rendering/htmx/)

**Prerequisite defect:** `ToggleSessionItem` returns the old item's `Checked` value (`internal/packing/store.go:111`). The current handler ignores it, so its reload displays the persisted state; rendering that return value directly would display stale state. Return authoritative updated data first.

Add pending/error feedback and disable the triggering control during the request. HTMX provides `hx-disabled-elt`; this improves interaction but does not solve cross-device races. [Disabled elements](https://htmx.org/attributes/hx-disabled-elt/)

When enhancing forms later, deliberately choose validation responses: a `200` error fragment, or `422` with configured swapping. HTMX ignores `422` bodies by default. Never accidentally insert a whole layout inside an item row. [Response handling](https://htmx.org/docs/#response-handling)

### 3. Keep authentication and HTTP semantics correct

Retain `X-CSRF-Token` on HTMX DELETE requests. Go parses URL-encoded request bodies into forms for POST, PUT, and PATCH, not DELETE; Gorilla accepts a token header. [Go ParseForm](https://pkg.go.dev/net/http#Request.ParseForm), [Gorilla CSRF](https://pkg.go.dev/github.com/gorilla/csrf)

**Semantic defect:** sign-out mutates the session through GET (`internal/web/routes.go:29`). Move it to a CSRF-protected POST form: Gorilla skips token validation for safe methods including GET. [Gorilla design notes](https://pkg.go.dev/github.com/gorilla/csrf#section-readme)

Before partial swaps, handle expired authentication explicitly. `RequireAuth` currently returns `303` (`internal/auth/client.go:78`); an HTMX request can follow that redirect and swap the login page into its local target. For HTMX requests, a non-3xx response with `HX-Redirect: /login` requests full navigation. HTMX does not process these response headers on 3xx responses. [HX-Redirect](https://htmx.org/headers/hx-redirect/)

### 4. Make simultaneous changes deliberate

`RemoveItem` reads then upserts a full document (`internal/packing/store.go:194`); a concurrent item addition can be overwritten. Check-off reads a Boolean then writes its inverse (`:90`); two callers can act on the same old value. These are concurrency risks, not reproduced incidents.

Prefer an explicit desired checked state over a replay-sensitive toggle. For whole-document edits, preserve the read ETag and use conditional replacement, handling conflict rather than overwriting silently. Cosmos documents ETag-based optimistic concurrency; the pinned SDK exposes `ItemOptions.IfMatchEtag` and `PatchOperations.SetCondition`. A conditional patch can check the expected item identity/state before changing an array position. Validate the selected operation with an integration test. [Cosmos concurrency](https://learn.microsoft.com/en-us/azure/cosmos-db/database-transactions-optimistic-concurrency), [pinned Go SDK](https://pkg.go.dev/github.com/Azure/azure-sdk-for-go/sdk/data/azcosmos@v1.4.2)

### 5. Add focused tests and deployment configuration

`go test ./...` passed during research, reporting **no test files**. Start with validation, mixed document selection, session snapshot persistence, missing CSRF, and the rendered check-off response. `httptest` supplies request/response recorders and test servers. A small handler-owned store interface can enable fake storage (`internal/web/handler.go:15`); introduce only the seam needed for these tests. [Go httptest](https://pkg.go.dev/net/http/httptest)

Before deployment, configure both cookie `Secure` flags for HTTPS (`cmd/web/main.go:76`, `internal/auth/client.go:38`), production origins/redirects, and a deliberate session lifetime: currently 600 seconds. The single CookieStore key signs cookies; encryption requires an additional key. Do not put secrets in session values. [Gorilla sessions](https://pkg.go.dev/github.com/gorilla/sessions)

Replace the bare `ListenAndServe` call (`cmd/web/main.go:82`) with an explicit server, appropriate header/read/write/idle timeouts, checked startup errors, and graceful shutdown. Go exposes these controls on `http.Server`. [Go server configuration](https://pkg.go.dev/net/http#Server)

## Product boundary: camping without connectivity

An offline check-off promise changes the design: cacheable pages alone cannot persist edits to Cosmos without a network. My recommendation is to validate that requirement before adding local persistence, queued desired-state changes, replay, expired-login recovery, and conflict resolution. A downloadable/printable checklist is a smaller initial option. If reliable offline editing becomes central, prototype a limited client-side packing screen while retaining Go for authentication and synchronization. HTMX itself identifies full offline operation as a poor fit for pure hypermedia. [HTMX architecture guidance](https://htmx.org/essays/when-to-use-hypermedia/)
