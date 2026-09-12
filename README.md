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
- Start packing sessions and toggle their items between checked and unchecked.
- View packing progress across sessions.
- Delete lists and sessions. Lists are soft-deleted; sessions are permanently removed.

Lists and sessions are stored in Cosmos DB, with database operations scoped to the signed-in user's ID. Both document types share a container and have a `type` field identifying their kind.

This is an evolving learning project. The functionality above describes the implemented flows; it is not a guarantee that every flow has been tested end to end. There are currently no automated tests in the repository.

## Architecture

The Go server renders HTML using templ. HTMX adds form navigation and actions such as deleting records and checking items. Several actions currently refresh the page after updating the database.

| Location | Purpose |
| --- | --- |
| `cmd/web/main.go` | Load configuration, connect dependencies, configure CSRF protection, and start the server |
| `internal/web/` | HTTP routes, handlers, and rendering helper |
| `internal/packing/` | Packing data types, constructors, form validation, and database operations |
| `internal/auth/` | Google OAuth/OpenID Connect login and cookie-based sessions |
| `internal/views/` | templ page templates and generated Go code |
| `internal/db.go` | Cosmos DB client setup |
| `static/` | Styles and the bundled HTMX script |

## Run locally

You need Go 1.25 or later, access to Azure Cosmos DB, and Google OAuth client credentials for a web application.

The app currently uses the database `dev` and container `packing_list`, configured in `cmd/web/main.go`. These must already exist. The container should use `/userId` as its partition key.

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

Open <http://localhost:3000>. You will be redirected to the login page if you are not signed in.

The current cookie and CSRF settings are configured for local HTTP development. Deployment over HTTPS requires updating those settings.

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

An `.air.toml` configuration is included for optional live reload with Air. It regenerates templates and rebuilds the server when source files change.

For collaboration on this learning project, see [CLAUDE.md](CLAUDE.md), which describes the preference for explanations and guided changes.

## Research

- [Market research](docs/market-research.md): similar apps, their positioning and features, and opportunities to validate for Camplist.
- [Technical research](docs/technical-research.md): how Go, templ, and HTMX fit the app, current implementation findings, and suggested next steps.
