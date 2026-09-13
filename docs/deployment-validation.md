# Browser and Cosmos validation

Validated 2026-09-12, starting from `e41b0cf`, with the current workspace UI.
Result: core flows passed after fixing a CSRF cookie-scope defect and a narrow-screen overflow.
This is functional validation, not a load test or Azure cost estimate.

## What ran

| Check | Environment | Result |
| --- | --- | --- |
| Lists/sessions stay separate; deleted lists disappear | Cosmos emulator and live Azure Cosmos | Passed |
| Same document ID belongs independently to two accounts | Emulator and live Cosmos | Passed |
| Add/remove item patches | Emulator and live Cosmos | Passed |
| Concurrent different-item updates survive | Emulator and live Cosmos | Passed |
| Concurrent same-item updates have one winner and one conflict | Emulator and live Cosmos | Passed |
| Replayed operation is acknowledged without another revision | Emulator and live Cosmos | Passed |
| Stale item/template writes are rejected | Emulator and live Cosmos | Passed |
| Apply review once; carry preparation into next trip | Emulator and live Cosmos | Passed |
| Recover deleted template; reject sync after session deletion | Emulator and live Cosmos | Passed |
| Save offline shell and session; reopen a new page during an outage | Chromium 150 with emulator and live Cosmos | Passed |
| Offline edits persist and sync after reconnect | Chromium 150 with emulator and live Cosmos | Passed |
| Conflicting states remain visible; explicit local choice resolves them | Chromium 150 with emulator | Passed |
| Submit review, apply task, start next trip showing that task | Chromium 150 with emulator | Passed |
| Switching accounts hides the first account's saved session and preserves its queue | Chromium 150 with emulator | Passed |
| Pending edits trigger sign-out choices | Chromium 150 with emulator | Passed |
| Missing authentication pauses sync while local editing remains usable | Chromium 150 with live Cosmos | Passed |
| Same-account sign-in resumes sync; normal sign-out clears saved copies | Chromium 150 with live Cosmos | Passed after CSRF fix |
| Narrow offline page does not horizontally overflow | Chromium iframe, approximately 320 CSS px | Passed after button-wrap fix |
| Service worker updates its asset cache from v3 to v4 | Chromium 150 | Passed |

The browser was T3Code's embedded Chromium 150, not Safari or a physical phone.
Outages were simulated by the local fixture closing application HTTP connections;
the real service worker, Cache Storage, IndexedDB, Go routes, auth middleware,
CSRF middleware, and Cosmos SDK still ran. A new page reopened the cached session;
the entire browser process was not terminated. Missing authentication was simulated
by expiring the server session cookie, then using the test account login again.
Google's interactive OAuth flow was replaced by a fixture-signed session cookie.

## Defects fixed

1. **Cross-path CSRF:** gorilla/csrf's default cookie path can scope a cookie to the
   current route directory. An identity token obtained under `/api` was rejected
   at `/auth/signout`. The production server and fixture now share
   `web.CSRFProtection`, with a root-scoped `camplist-csrf` cookie. The new name avoids
   older, more-specific `_gorilla_csrf` cookies shadowing it. A cookie-jar HTTP
   regression test first reproduced the 403, then passed after the fix, including
   a pre-existing legacy cookie.
2. **Phone-width overflow:** the long offline export/remove button exceeded the
   viewport. Offline buttons now wrap and stay within their container. Measured
   scroll width dropped from 412px to 316px for a 316px iframe viewport. The offline
   cache version was advanced to deliver the stylesheet change.

## Isolation and cleanup

Live validation used the existing `dev` database and `packing_list` container,
with unique `camplist-validation-*` and `camplist-browser-*` account partitions.
Only generated test records were modified. Cleanup removed all generated live
records, including soft-deleted templates and recovered templates. No Azure
resources or throughput settings were created or changed. Live requests incur the
account's normal Cosmos usage.

The local emulator was Microsoft's vNext image, version `EN20260907`, digest
`sha256:2db1f9e74c506bcf6fc347aa937aea1c00fa756061296a5a9efba530ce86ec02`.
It supports a subset of Cosmos features and does not model Request Units, so it
cannot establish Azure billing or production capacity. [Microsoft emulator documentation](https://learn.microsoft.com/en-us/azure/cosmos-db/emulator-linux)

## Repeat the checks

Ordinary tests skip external validation:

```sh
go test ./...
go vet ./...
go build ./...
npm test
npm run check
```

The opt-in live test reads `DB_URL` and `DB_KEY` from the repository's `.env` and
cleans its unique test partitions afterward:

```sh
CAMPLIST_LIVE_COSMOS=1 go test ./internal/validation -run TestCosmosValidation -count=1 -v
```

For local emulation, start Docker, then create the public test key and container:

```sh
python3 -c 'import base64; open("/tmp/camplist-emulator-validation.key", "w").write(base64.b64encode(bytes(64)).decode())'
docker run -d --name camplist-validation-cosmos \
  -p 127.0.0.1:8081:8081 -p 127.0.0.1:8080:8080 \
  --mount type=bind,source=/tmp/camplist-emulator-validation.key,target=/tmp/key,readonly \
  mcr.microsoft.com/cosmosdb/linux/azure-cosmos-emulator@sha256:2db1f9e74c506bcf6fc347aa937aea1c00fa756061296a5a9efba530ce86ec02 \
  --key-file /tmp/key --enable-explorer false --enable-telemetry false
curl -fsS http://127.0.0.1:8080/ready
CAMPLIST_EMULATOR=1 go test ./internal/validation -run TestCosmosValidation -count=1 -v
```

To run the browser fixture, use either live mode or emulator mode, never both:

```sh
CAMPLIST_LIVE_COSMOS=1 CAMPLIST_BROWSER=1 go test ./internal/validation -run TestBrowserSimulation -count=1 -v -timeout=30m
```

Open `http://camplist-validation.localhost:3001/__validation/account`. The fixture
binds only loopback and exists only in a test binary. Its test-account endpoint is
not part of the production executable. Use `?name=other` to simulate another account.
Simulate an outage and restore it with:

```sh
curl -fsS 'http://127.0.0.1:3001/__validation/network?offline=1'
curl -fsS 'http://127.0.0.1:3001/__validation/network?offline=0'
```

Stop gracefully to run database cleanup, then remove the local emulator:

```sh
curl -fsS http://127.0.0.1:3001/__validation/stop
docker rm -f camplist-validation-cosmos
rm /tmp/camplist-emulator-validation.key
```

Do not force-kill the live test: that bypasses Go test cleanup. The account
partition names are logged for recovery if the process is interrupted.

## Production follow-up (2026-09-12)

The app was deployed to Azure Container Apps with HTTPS ingress and health probes.
The production Google OAuth callback was added alongside localhost. A real Google
sign-in completed successfully on the deployed domain and displayed the signed-in
user's existing lists from Cosmos. No existing list was changed during this check.
Public `/healthz` and `/login` returned 200; unauthenticated `/api/identity`
returned 401. See [deployment setup](deployment.md) for resources and CI.

## Automatic packing follow-up

The ordinary packing screen now persists checks locally, automatically saves
opened trips, and synchronizes without Save/Sync buttons. Chromium validation
against a disposable live Cosmos partition confirmed an offline check updated
progress immediately, reopening the same trip URL loaded the public offline
shell and preserved the check, and a second offline check synchronized after a
reconnect event. Both server item revisions advanced once. The worker upgrade
path now waits for installation before reporting offline readiness.

## Deleted trips offline follow-up (2026-09-13)

Deleting a trip, removing a member or archiving a trip used to leave the device
copy offered as "Available offline" on `/trips` and packable in the offline
shell. Chromium at a 390px phone viewport against the in-memory fixture
(`CAMPLIST_BROWSER_MEMORY=1`) confirmed the fix:

| Check | Result |
|---|---|
| Owner deletes a trip from its card | Copy removed in the same tab, without a reload or `/api/sessions` call |
| Owner reloads `/trips` | No card for deleted or archived trips; the archive page labels the archived card |
| Owner offline, fresh tab on the deleted trip URL | Offline shell lists only the remaining copies |
| Member reloads `/trips` after the owner deleted a shared trip or removed them | Both copies removed (404 and 403 `access_removed`) |
| Member with an unsynced check when the owner deletes | Copy kept; `/trips` shows "Deleted online · 1 unsynced change(s)" linking to the shell; the trip page and the offline shell (also offline in a fresh tab) keep the deleted status and disable packing |
| Member has the trip page open when the owner deletes it or removes them | Page says "This trip was deleted" or "Your access to this trip was removed" and disables packing |

Take one host offline with `network?offline=1&host=camplist-member.localhost:3001`.

## Remaining deployment checks

- Cold start, scale-to-zero wakeup, and authenticated session continuity across container revisions.
- Safari/Firefox, physical phones, browser-process restart, storage eviction, and installed-app behavior.
- Lost-response, transaction-failure, and concurrent export/cleanup cases remain covered by module tests; they were not all fault-injected in the real browser.
- Real usage measurements for Cosmos RU/document growth and Azure Container Apps cost.

Keep the offline feature labeled experimental until the intended mobile browsers
and deployed HTTPS environment have been exercised.
