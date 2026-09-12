# Azure SSE experiment

Date: 2026-09-12. Status: complete. Disposable data, Azure app, image repository and private configuration removed.

## Scope and isolation

The user approved the proof checklist in `realtime-updates-research.md` and asked
that costs stay small. The experiment uses `camplist-stream-probe` in the existing
West Europe Container Apps environment, with 0.25 vCPU, 0.5 GiB, minimum zero and
maximum one replica. It creates no new registry, environment, broker or premium
ingress. Its hostname is a separate real ACA hostname in the same environment;
the production Camplist app is not replaced.

The probe is pinned to commit `7fc051e`. It uses the real packing store, Cosmos,
auth middleware, HTTP API, browser persistence and conflict handling. A temporary
adapter sends minimal SSE notifications into the existing reconciliation path.
Fallback polling is 60 seconds in the experiment. Test login uses signed fixture
cookies; it is not a Google OAuth end-to-end test. Only disposable account
partitions are used. Synthetic fixture expiry is available for bounded expiration
tests. The production auth implementation subsequently changed in `2ed415c`;
that later change is outside this probe's baseline.

Two isolated Chrome browser contexts run through the Azure hostname, one with a
mobile viewport. Background/foreground is exercised by an injected visibility
transition: this tests the application lifecycle hook, not physical mobile OS
suspension. Physical-device Safari/Firefox acceptance remains separate.

## Measured observations

- Both accounts opened streams and loaded the shared trip.
- The silent stream disconnected around four minutes and reconnected; the
  heartbeat stream remained connected at the five-minute checkpoint.
- Four cross-browser completion samples took 304, 314, 313 and 315 milliseconds
  from click to the other browser displaying the changed state. These are a small
  sample on this connection, not a latency SLA.
- Simultaneous different-item edits and an offline edit followed by reconnect
  converged. Suppressed notifications recovered through polling.
- The visibility simulation closed the hidden client's stream and refreshed on
  return.
- Five quiet minutes added 100 Cosmos request units with the initial conservative
  permission recheck every 15 seconds. Heartbeats themselves contain no data;
  authorization reads dominate this idle database cost. A second revision uses
  60-second periodic revalidation, while retaining checks at subscription and
  notification delivery and immediate revocation notification in its own process.

## Cost basis

Official West Europe Consumption retail rates observed during this investigation
were $0.000034 per vCPU-second and $0.000004 per GiB-second. One fully active
0.25-vCPU / 0.5-GiB replica for 30 minutes therefore costs $0.0189 in compute before
subscription-wide allowances, excluding logs, transfer, Cosmos and additional
runtime. This is an estimate from allocation and time, not an Azure invoice.
[Azure Retail Prices API](https://prices.azure.com/api/retail/prices?$filter=serviceName%20eq%20%27Azure%20Container%20Apps%27%20and%20armRegionName%20eq%20%27westeurope%27%20and%20priceType%20eq%20%27Consumption%27)

Do not equate low observed CPU with free compute. Persistent requests can keep a
replica active, and minimum-zero running replicas do not meet the documented
minimum-replica condition for discounted idle rates. At these rates, a continuously
active small replica is approximately $27.22 per 30 days before allowances.
[ACA billing](https://learn.microsoft.com/en-us/azure/container-apps/billing)

The existing Cosmos account reports Serverless and Session consistency. Report
measured request units separately; do not infer database cost from HTTP request
counts alone. Scale-to-zero checks must use ARM/Monitor rather than repeated app
HTTP requests that wake it up.

## Acceptance coverage

| Check | Observed result |
| --- | --- |
| More than ten minutes, two browser accounts | Passed; silent and heartbeat streams exercised |
| Shared update latency | Four initial cross-browser samples 304–315 ms; 201 ms after revision replacement |
| Simultaneous edits and offline/reconnect | Passed through real HTTP API and local persistence |
| Missed notification | Deliberately suppressed event recovered through the 60-second fallback |
| Background/foreground | Application visibility hook passed; physical OS suspension untested |
| Revision replacement | Two connections opened automatically on the new replica before the test forced any reconnect |
| Expiry | Signed short-lived fixture session produced sign-in state |
| Membership removal | Upload denied and local pending operation preserved for export |
| Account switch | Other account could not read trip; local sync reported account mismatch |
| Origin / anonymous access | Cross-origin stream 403, unauthenticated stream 401 |
| Conflict review | Stale disagreement required explicit review; keeping shared state preserved server value |
| Resource usage | Observed peak 0.1271 CPU cores, 57.34 MiB working set; one replica in sampled metrics |
| Idle cost comparison | Initial 100 RU / 300 seconds; revised 8.2 RU / 65 seconds with the same two identities |
| Scale-to-zero / cold start | Zero replicas observed at 17:18:57 UTC; first cold response 21.688 seconds, trip ready 22.379 seconds |

The idle comparison corresponds to about 20 versus 7.57 RU/minute in these short
quiet windows, around 62% fewer measured RU. This is not a general workload cost
benchmark: events and initial loads still cause authenticated reads. Periodic
revalidation alone is not the revocation mechanism: subscriptions and delivered
events are authorized, same-process removal publishes immediately, and fallback
reads independently authorize. Cross-process behavior still needs a bounded
strategy before increasing replicas.

Azure Monitor reported 268 external requests in the sampled window ending after
the initial run (not the entire experiment). SDK response request-charge headers
provide the RU measurements; they are not a billing invoice. The memory and CPU
peaks include startup/revision activity, not only idle streams.

## Interpretation

SSE with heartbeats, reconnect reconciliation and polling fallback meets this
small experiment's responsiveness goal. WebSockets were not tested because SSE
did not exhibit a disadvantage that justified the extra comparison. Maintain
minimum zero, maximum one and the existing small allocation; no premium ingress
or broker is justified by this evidence. Close streams in hidden pages and avoid
Cosmos reads solely for heartbeat generation. Do not promise two-second latency
during outages, cold starts or fallback polling.

This evidence is scoped to the pinned prototype and Chrome contexts. It does not
prove physical iOS/Android lifecycle behavior, Safari/Firefox, the latest auth
change, many-user load, or durable cross-replica event delivery. Future personal
items/tasks and offline additions must retain their own domain validation; a
notification transport does not implement those features.

## Scale-down, limitations and cleanup

The first idle observation was interrupted by diagnostics: Azure CLI 2.74's
`logs show` validator calls `_ping_containerapp_if_need`, which can send an HTTP
request to wake the app. Source inspection confirmed this in installed
`containerapp/_validators.py` and `_ssh_utils.py`. Replica listing itself only
uses ARM. Do not use console-log attachment during a no-traffic scale-down test.
The interrupted samples are retained separately; they are not a failed scale-down
result. Azure metrics also recorded a zero-replica minimum during that interval.

After removing that interference, ARM reported zero replicas at 17:18:57 UTC.
The diagnostic wake request was logged at 17:12:59, so the observed boundary was
about six minutes after that request, sampled every roughly 32 seconds. A fresh
Chrome context then received its first response after 21.688 seconds and had a
synchronized trip view after 22.379 seconds. The fixture login allowed a 90-second
request timeout. The app's 15-second API timeout is shorter than this cold start:
production must retain retry/reconnect behavior and clear waiting status. This
run does not prove an already-open client with pending work through that exact
cold-start timeout combination.

A cleanup-reporting typo (`Response.ok` treated as a function) happened after the
cleanup HTTP request had executed. The harness was corrected and cleanup repeated;
the second pass found zero remaining fixture documents. The raw result retains
this reporting error and recovery note. All browser clients were closed.

Azure list queries confirmed the disposable app and dedicated `stream-probe`
image repository are absent. The private configuration files were removed. The
app existed for roughly half an hour, making the gross single-replica compute
estimate about US$0.02 before allowances and other meters; this is not an invoice.
No production app, environment capacity or registry configuration was changed.
No production realtime rollout occurred.
