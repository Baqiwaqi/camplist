# Camplist

Camplist is a web app for preparing camping packing lists and checking off items as you pack for a trip. It is also a learning project for building server-rendered applications with Go, chi, templ, HTMX, and Azure Cosmos DB.

## How it works

1. Sign in with Google to access your lists and packing sessions.
2. Create a packing list with a name and an optional description.
3. Add items, such as a tent or sleeping bag, with optional categories.
4. Start a packing session from a list.
5. Check or uncheck items as you pack. The sessions overview shows the number of checked items against the total.

A **packing list** is a reusable template. A **packing session** is a saved snapshot of that list for a particular packing occasion. Checking items in a session does not change the original list, and later edits to the original list do not update existing sessions.

For example, you can keep a “Weekend camping” list and start a new session each time you go away.

## Current functionality

- Google sign-in and sign-out.
- Create and edit list names and descriptions.
- Add and remove items from a list.
- Start fresh packing sessions, reopen them from the overview, and mark items packed or unpacked.
- Update the checklist and progress in place, with visible feedback if saving fails.
- View packing progress across sessions.
- Delete lists and sessions. Lists are soft-deleted; sessions are permanently removed.

Lists and sessions are stored in Cosmos DB, with database operations scoped to the signed-in user's ID. Both document types share a container and have a `type` field identifying their kind.

This is an evolving learning project. The functionality above describes the implemented flows; it is not a guarantee that every flow has been tested end to end. Automated regression tests cover form validation, session independence, database request contracts, authentication redirects, and checklist responses. Live Google login and Cosmos DB integration still need end-to-end verification.

## Architecture

The Go server renders HTML using templ. HTMX adds form navigation and actions such as deleting records and checking items. Packing check-off replaces the checklist section with HTML from the server; deletion actions currently refresh the page. Normal packing forms also work without HTMX.

| Location | Purpose |
| --- | --- |
| `cmd/web/main.go` | Load configuration, connect dependencies, configure CSRF protection, and start the server |
| `internal/web/` | HTTP routes, handlers, and rendering helper |
| `internal/packing/` | Packing data types, constructors, form validation, and database operations |
| `internal/auth/` | Google OAuth/OpenID Connect login and cookie-based sessions |
| `internal/views/` | templ page templates and generated Go code |
| `internal/db.go` | Cosmos DB client setup |
| `static/` | Built stylesheet, logo marks, the dropdown menu behaviour, and the bundled htmx and Alpine.js scripts |
| `assets/css/tailwind.css` | Tailwind input: design tokens, base styles, and component classes |
| `.claude/skills/camplist-design/` | Design system: tokens, guidelines, logo assets, and a click-through UI kit |

## Run locally

You need Go 1.25 or later, access to Azure Cosmos DB, and Google OAuth client credentials for a web application.

The app currently uses the database `dev` and container `packing_list`, configured in `cmd/web/main.go`. These must already exist. The container should use `/userId` as its partition key. At startup, the app enables per-item Cosmos TTL (`defaultTtl: -1`) when TTL is not yet configured; share-link capability items set their own seven-day `ttl`, while lists, sessions, memberships, and shared references do not expire.

Set the following environment variables, or put them in a local `.env` file, which is ignored by Git:

| Variable | Purpose |
| --- | --- |
| `DB_URL` | Cosmos DB account endpoint |
| `DB_KEY` | Cosmos DB access key |
| `GOOGLE_CLIENT_ID` | Google OAuth client ID |
| `GOOGLE_CLIENT_SECRET` | Google OAuth client secret |
| `REDIRECT_URL` | OAuth callback URL, typically `http://localhost:3000/auth/callback` locally; must match the Google client's authorized redirect URI |
| `SESSION_KEY` | Secret key used to sign authentication cookies |
| `CSRF_KEY` | Secret key used for CSRF protection |

Use independently generated random keys for `SESSION_KEY` and `CSRF_KEY` (32 characters each is suitable for this app's raw-string configuration).

Start the server from the repository root:

```sh
go run ./cmd/web
```

Open <http://localhost:3000>. Visitors see the landing page and can try a demo session at `/demo` without an account; members see their lists. Pages under the app itself redirect to the login page when you are not signed in.

Cookie security and the trusted CSRF origin are derived from `REDIRECT_URL`: use an HTTPS callback URL for deployment behind HTTPS, and an HTTP localhost callback for local development. The server includes request timeouts and graceful shutdown. Authentication sessions persist for one year unless the user signs out, clears browser data, or the deployment's `SESSION_KEY` changes.

## Development

Build the project:

```sh
go build ./...
```

After editing a `.templ` file, regenerate the Go views before building:

```sh
templ generate
```

Use the templ CLI version matching `go.mod`. Edit `.templ` source files rather than generated `*_templ.go` files.

Styling is Tailwind CSS v4: utility classes in the templ views plus a few component classes and the Camplist design tokens in `assets/css/tailwind.css`. The built file `static/tailwind.css` is not committed, so build it once after `npm install` (or keep it building while you work):

```sh
npm run css
npm run css:watch
```

An `.air.toml` configuration is included for optional live reload with Air. It regenerates templates, rebuilds the stylesheet and the server when source files change. The Docker build runs the same stylesheet build in a Node stage.

Run the automated checks:

```sh
go test ./...
go vet ./...
```

For collaboration on this project, see [CLAUDE.md](CLAUDE.md). Requested changes are implemented directly, with explanations and appropriate verification.

## Research

- [Product direction](docs/product-direction.md): the selected focus on improving each trip and offline check-off for saved sessions, with the proposed implementation and acceptance criteria.

- [Market research](docs/market-research.md): similar apps, their positioning and features, and opportunities to validate for Camplist.
- [Technical research](docs/technical-research.md): how Go, templ, and HTMX fit the app, current implementation findings, and suggested next steps.

## Improve your next trip

Open a packing session and choose **Review this trip**. Record forgotten or unused
gear and repairs or replacements. Select additions, removals, and preparation
tasks to apply to the reusable list. Old sessions keep their original checklist.
Preparation tasks are completed separately from packing items. If the original
list was deleted, the review offers to create a new list from that trip.

## Experimental offline packing

Open an existing packing session while connected. It saves to this browser
in the background; keep using the regular checklist when the connection drops.
The status confirms when offline reopening is ready. Reopen the same trip URL
or use **On this device** to find saved trips. Changes sync automatically on
reconnect, returning to the app, or a periodic retry while pending. No Save or
Sync button is needed. Creating sessions, editing lists, and trip reviews still
require a connection.

Changes persist in this browser and synchronize while the app is open, connected,
and signed into the same account. Conflicting changes offer a choice between this
device and the online state. Expired login or a deleted session keeps the local
copy available for export. Sign-out offers sync or export before clearing this
account's local records. If storage cannot be inspected, an explicit server-only
sign-out option explains that local copies may remain. Browser storage can be evicted; local changes are not a
cloud backup. Offline support requires HTTPS (or localhost), IndexedDB, service
workers, and JavaScript. Cross-browser acceptance remains pending.

Browser module checks use Node.js 22 or later:

```sh
npm ci
npm run check
npm test
```

See [the implementation plan](docs/implementation-plan.md) for test seams and
remaining environment validation.

## Deployment

[Azure Container Apps setup](docs/deployment.md) documents the production URL,
GitHub Actions workflow, Azure resources, costs, secrets, and rollback commands.

## Sharing lists and trips

Open **Sharing and access** on a list or session to create a copyable invitation.
The recipient signs in with Google and requests access; the owner approves their
account. Share reusable templates and individual trips separately; approving a
list request also offers to share that list's current trips with the same
account. Template
editors can change equipment/preparation and create their own private trips.
Trip packers can check equipment without gaining access to its template.

Shared offline packing shows pending changes, last sync time, and stale-view
warnings. Conflicts keep the shared state until you deliberately choose a
resolution. Removing a member blocks future server access; local copies already
on a disconnected device cannot be recalled.

See [sharing behavior and validation](docs/shared-lists-implementation.md).
