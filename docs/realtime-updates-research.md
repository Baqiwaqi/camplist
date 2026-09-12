# Realtime and near-realtime trip updates

Researched 2026-09-12. **Research only; transport choice remains open for user approval.**

## Recommendation to discuss

Use **Server-Sent Events (SSE) for change notifications, existing authenticated HTTP operations for edits, and polling as a recovery fallback**. SSE is realtime too: the server sends a notification when a change happens. WebSockets are viable, but bidirectional messaging is not required merely to see another camper check an item. This is an architectural recommendation inferred from the product needs and sources below, not an implemented or benchmarked result.

First prove streaming and reconnect behavior through the actual Azure Container Apps ingress. If that proof is unreliable, start with visible-trip polling every 3–5 seconds. Choose WebSockets if the infrastructure experiment favors them or later requirements need frequent bidirectional messages, such as presence. Do not introduce a second write/synchronization protocol just to deliver notifications.

## Options

| Option | Update behavior | HTMX / Go fit | Camplist tradeoff |
| --- | --- | --- | --- |
| Short polling | Refresh on an interval; typical additional delay around half the interval under steady conditions | HTMX supports `hx-trigger="every 3s"`; ordinary Go HTTP handlers | Smallest change; repeated requests and database reads even when nothing changes |
| Long polling | Hold a request until something changes or a timeout, then repeat | Ordinary HTTP with cancellation and a bounded wait | Faster than short polling without a permanent stream, but still needs subscriber state, timeout and reconnect handling |
| SSE | Immediate server-to-browser notifications over an HTTP response | Native `EventSource`, optional HTMX 2 SSE extension; Go `net/http` streaming | Best match for server notifications alongside current HTTP writes; ingress behavior needs validation |
| WebSocket | Immediate messages in both directions over an upgraded connection | HTMX 2 WS extension; Go library such as `coder/websocket` | Useful capabilities, but origin checks, connection lifecycle and message protocol add work; offline persistence is still separate |

Polling syntax is documented by [HTMX](https://htmx.org/docs/#polling). SSE's one-way connection and event-driven HTTP callbacks are documented by the [HTMX SSE extension](https://htmx.org/extensions/sse/); the [WS extension](https://htmx.org/extensions/ws/) supports receiving HTML and sending form data. Delay and complexity comparisons above are engineering estimates, not measurements.

## HTMX and Go details

The repository vendors HTMX **2.0.10** in [static/htmx.min.js](../static/htmx.min.js). Use the 2.x extension APIs, not the different examples under `four.htmx.org`.

The HTMX SSE extension can swap named event content or trigger an HTTP request through `hx-trigger="sse:event-name"`. It adds exponential reconnection logic over browser reconnection. For Camplist's offline-controlled packing screen, a small native `EventSource` adapter is likely a better fit than automatic HTML swaps: it can request reconciliation through the existing packing module. Other server-rendered screens can still use the extension. [HTMX SSE extension](https://htmx.org/extensions/sse/)

The WS extension treats received HTML as out-of-band swaps, serializes `ws-send` forms, reconnects on specified unexpected close codes using jittered exponential backoff, and queues outgoing messages **in memory** while disconnected. That queue does not replace Camplist's durable device queue, operation receipts or conflict handling. Avoid enabling it as a parallel write path. [HTMX WS extension](https://htmx.org/extensions/ws/)

Native SSE supports `id`, `retry` and `Last-Event-ID` on reconnect. An event ID does not create server-side replay storage. For invalidation-only events, simply fetch current authorized state after connecting/reconnecting; no durable event log is necessary for correctness. SSE comment heartbeats can help with idle intermediaries, but do not prove that an ingress total timeout is bypassed. [HTML SSE standard](https://html.spec.whatwg.org/multipage/server-sent-events.html)

Go's `net/http` provides flushing and per-response deadline controls through `ResponseController`; verify wrappers expose these capabilities. Its `Server.Shutdown` does not close or wait for hijacked WebSockets, so those connections need explicit shutdown handling. [Go HTTP documentation](https://pkg.go.dev/net/http)

If choosing WebSockets, `coder/websocket` offers contexts, ping/pong and read-only connection helpers, and identifies Coder as its maintainer. Gorilla is another established option. The Go `x/net/websocket` documentation itself points to Coder and Gorilla as more actively maintained alternatives. Prefer one of those over starting with `x/net/websocket`; pin and verify a release during implementation. [Coder repository](https://github.com/coder/websocket), [Go x/net documentation](https://pkg.go.dev/golang.org/x/net/websocket)

## Integration with this app

Local code observations, not external claims:

- [automatic.mjs](../static/offline/automatic.mjs) currently synchronizes visible shared trips every 15 seconds and on online/focus events. [ui.mjs](../static/offline/ui.mjs) can synchronize all saved records, so attaching a faster notification loop to it unchanged could amplify requests.
- [packing.mjs](../static/offline/packing.mjs) owns durable pending operations, identity separation and conflict reconciliation. [transport.mjs](../static/offline/transport.mjs) uses a 15-second fetch abort timeout; its request helper is unsuitable for a persistent stream.
- [main.go](../cmd/web/main.go) sets a 30-second write timeout, 15-second read timeout, 60-second idle timeout and 20-second shutdown budget. SSE requires endpoint-aware write deadlines and explicit stream cancellation; do not disable ordinary request protection globally.
- [auth/client.go](../internal/auth/client.go) configures cookies with `MaxAge: 600`. Long connections must not silently extend authorization beyond the intended session lifetime.
- [app.bicep](../infra/app.bicep) declares single-revision mode, 0–1 replicas, 0.25 CPU and 0.5 GiB memory. These are repository settings, not a fresh audit of deployed Azure configuration.

Proposed flow:

1. Authorize a same-origin subscription for the signed-in actor and trip using canonical membership checks.
2. Keep writes in the existing durable queue and authenticated HTTP routes, with operation IDs, expected revisions and conditional Cosmos writes.
3. Publish a minimal `trip-changed` notification **after a successful commit**, keyed by storage owner and trip ID. Do not include checklist content, personal information, invitations or membership documents.
4. Coalesce notifications and schedule one reconciliation for the active trip. Events arriving during reconciliation must cause a subsequent refresh, not disappear behind a busy flag. Preserve pending local additions, completion changes, conflicts and active form input.
5. On initial connect, reconnect, return to foreground and fallback polling, fetch current authorized state. Missed, duplicated or reordered notifications must be harmless. A connected socket alone does not mean data is current or pending edits are saved remotely.
6. Stop on logout/account change or access removal; close connections in hidden tabs and reconnect when visible. Preserve exportable pending work if access is removed.

This transport work does not itself implement the separately discussed personal/shared items, preparation tasks, offline additions, naming or navigation changes. Those operations must first have explicit domain and offline reconciliation behavior.

## Authorization and fanout

For SSE, use same-origin authenticated requests with no permissive cross-origin access and `Cache-Control: no-store`. Do not place credentials in URLs. For WebSockets, authenticate before upgrade and enforce an exact allowed browser origin; Coder's default rejects cross-origin connections, and its documentation warns against unsafe origin relaxation. HTTP CSRF middleware alone does not authorize future WS messages. [Coder AcceptOptions](https://pkg.go.dev/github.com/coder/websocket#AcceptOptions)

Both options need connection expiry, membership revocation, bounded subscriber queues, slow-client disconnects and cleanup. Recheck membership when delivering events or close affected subscriptions on access changes, with a bounded revalidation interval as defense against changes from another process. Every subsequent data fetch/write must independently authorize access. Expired authentication must surface “sign in again,” not an endless reconnect storm.

An in-process publisher is an economical first option, but only delivers events within one process. Even a configured maximum of one replica is not an absolute lifetime guarantee: Azure documents temporary extra replicas during platform maintenance. Retain polling/reconnect reconciliation for gaps during deployments and restarts. [ACA scaling](https://learn.microsoft.com/en-us/azure/container-apps/scale-app)

If scaling out, add cross-process fanout or accept a bounded polling delay. Sticky sessions do not ensure two participants reach the same process. A database change feed or external pub/sub would be a separate design and cost decision, not a requirement for the initial notification experiment.

The prior deployment validation recorded Cosmos **Session** consistency. Publishing after commit does not alone guarantee a read through a different SDK instance immediately observes that write. Validate session-token propagation or another suitable read strategy before promising cross-replica freshness; Microsoft explicitly describes this multi-node caveat. [Cosmos session tokens](https://learn.microsoft.com/en-us/azure/cosmos-db/how-to-manage-consistency#utilize-session-tokens)

## Azure constraints and cost

ACA explicitly supports WebSockets and documents a 240-second HTTP request timeout. The overview does not precisely establish how a continuously flushed SSE response versus an upgraded WS connection behaves under that timeout. **Do not claim a universal four-minute WebSocket cutoff or guaranteed unlimited SSE lifetime.** Test the deployed path. [ACA ingress overview](https://learn.microsoft.com/en-us/azure/container-apps/ingress-overview)

Premium ingress offers configurable idle request timeouts of 4–30 minutes, but requires a non-Consumption workload profile with dedicated capacity. It is not a free timeout switch for this tiny app. Do not upgrade infrastructure just for this feature without evidence. [ACA environment ingress configuration](https://learn.microsoft.com/en-us/azure/container-apps/ingress-environment-configuration)

Consumption billing includes running compute and external HTTP requests. Zero replicas incur no compute usage charges; reduced idle pricing requires minimum replicas greater than zero plus idle conditions. A min-zero app with a live replica should not be assumed to receive idle pricing. Persistent activity can affect scale-down and active runtime; actual SSE/WS connection metering and scale behavior need measurement. Polling can also keep the app active. [ACA billing](https://learn.microsoft.com/en-us/azure/container-apps/billing)

Illustrative arithmetic: 15-second polling gives **240 cycles/client-hour**; 3-second polling gives **1,200**, five times as many. The current identity-plus-session path requires at least two app requests per normal cycle, and shared authorization can require multiple Cosmos reads. Thus simply shortening the timer is not cost-neutral. These are calculated request counts, not measured RU usage or monetary estimates. SSE reduces unchanged-state polling, but total savings depend on active connections, heartbeats, fallback frequency and database access patterns.

## Proof required before choosing

- Run two browsers through the actual ACA hostname for more than ten minutes: silent periods, heartbeats, simultaneous edits and latency measurement. A proposed acceptance target is updates visible within two seconds on a stable connection; this is a goal, not a guarantee. Compare SSE and WS only if SSE fails or has meaningful operational disadvantages.
- Exercise offline/reconnect, mobile background/foreground, cold starts, revision deployment and dropped events. Verify eventual reconciliation without losing pending work.
- Expire authentication, revoke membership, change account and attempt cross-origin access. Confirm no data leakage and clear UI states.
- Measure request counts, Cosmos RU, active runtime, memory and open connections with realistic participant/tab counts. Confirm hidden/closed clients permit eventual scale-down.
- Preserve automatic polling fallback and existing conflict review regardless of the selected transport.

No streaming endpoint, infrastructure change, live benchmark or deployment was performed for this research.
