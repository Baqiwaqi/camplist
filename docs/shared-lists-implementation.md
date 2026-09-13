# Shared lists and trips

Implemented from [the research](shared-lists-research.md). The confirmed scope is
shared trip packing, independently editable shared templates, copyable invitation
links with owner approval, and shared offline packing in the initial release.
The agreed test seams are the public packing store, authenticated HTTP routes,
and the offline save/edit/sync interface. Review baseline: `4cc3376`, including
the existing item-edit and design-system changes.

## Behavior

Use **Sharing and access** on a list or trip. The owner creates a separate link
for each person, copies it to their existing conversation, and approves the
Google account that requests access. Camplist does not send email. A link alone
never grants checklist access. Links expire after seven days and bind to one
requester. Owners can revoke unused requests or remove members; members can leave.

Approving a list request asks whether to also share the owner's current
(unarchived) trips started from that list with the same account. Trips stay
unticked by default and the question is skipped when there are none. The prompt
opens in place with htmx (`/sharing/packing-list/{id}/invitations/{hash}/approve`).
Ticked trips gain the approved account as a trip member, exactly as if a trip
invitation had been approved, and are then managed and removed from each trip's
own sharing page. The list approval commits first and trips are then granted only
while the account is a list member, so a list at its member limit shares no trips,
a failed trip grant leaves the list shared (resubmission is idempotent), and a
removed list member is never re-added or given trips by a stale approval. Later
trips stay private.
The MVP allows 20 members and 20 unexpired invitation records per resource.

Trip packers can read the checklist and mark equipment. Template editors can
rename the list, add/edit/remove equipment and preparation tasks, and create
private trips. Template membership does not reveal anyone else's trips. Trip
membership does not grant template access. Only owners delete resources and
manage invitations. For the unresolved observation-submission choice, this
release keeps new trip observations owner-only. Applying existing observations
requires both source-trip access and independent destination-template access.

Opening a trip prepares an account-specific local copy automatically. Shared
trips explain that local changes are invisible to others until synchronized.
A persistent status warns about unknown freshness, outages, pending work, and
conflicts, and shows the last successful sync time. An online browser whose API
requests fail says **Can't sync this shared trip**. An online event alone cannot
clear the warning. Visible shared screens refresh every 15 seconds and on return.
A disconnected saved trip can reopen at the same URL through the public shell.

Only changed items are sent. Their observed revisions and stable operation IDs
are retained. Different items merge; matching stale intent is acknowledged
without changing the existing item revision or attribution. Disagreeing stale
intent leaves shared state untouched. **Keep shared state** is the first choice;
**Mark packed/unpacked instead** explicitly retries against the displayed version.
Another intervening edit requires review again. Removed access preserves the
local queue for export, explains that it will not upload, and never recreates the
server resource. Sign-out cleanup remains scoped to the signed-in account.

## Storage and authorization

Existing `/userId` partitions are unchanged. That field remains the immutable
storage owner. Authoritative memberships and hashed invitation records live on
the resource document. Legacy resources remain owner-only. A member's partition
holds a `shared-reference` discovery record; every access resolves the canonical
resource and checks its current membership. A stale reference is never a grant.

A request writes its discovery reference before claiming the invitation. Failure
is visible and retryable; an unapproved reference is filtered out of navigation.
Approval writes invitation status and membership in one ETag-conditioned resource
replacement. Duplicate approval does not re-add a removed member. No atomic
cross-partition transaction is claimed. Repeated request submission can repair
a missing discovery reference while its invitation remains valid.

Shared writes are authorized against the same document version that commits the
change. Session retries reload and reauthorize after ETag failure. Template forms
carry the version the editor saw and expose a conflict instead of retrying stale
form content. The former add-item and delete-list patch paths now use conditional
resource replacements. New sync receipts are scoped to the authenticated actor;
legacy owner receipts remain readable. The device snapshot contains only packing
fields, the default item categories, actor/owner IDs, and a shared flag, not membership, invitation, receipt,
or review metadata.

Identity continues to use Google's verified subject. Verified sign-in claims
supply the name/email shown on approval requests; no account directory or new
Google scopes are introduced. Invitations survive login through a signed cookie
with an allowlisted internal return path. Account switching requests Google's
account chooser. Invitation pages load no third-party assets, suppress referrers,
and use CSRF-protected POST actions. Application logs redact invitation paths;
join attempts have a bounded per-IP rate limit.

No-referrer navigation forms can send `Origin: null`. The CSRF wrapper recognizes
only browser-confirmed `Sec-Fetch-Site: same-origin` in that case and still requires
Gorilla's valid cookie/token pair. Null cross-site/unknown origins and missing
tokens remain rejected. This interaction is described in the [Go HTTP security
proposal](https://github.com/golang/go/issues/73626).

## Validation (2026-09-12)

- Store checks cover approval, separate grants, private snapshots, stale template
  forms, preparation edits, expiry, competing claimants, duplicate approval,
  discovery-write failure/retry, review permissions, and revocation between
  authorization and commit.
- HTTP checks cover account isolation, snapshot redaction, removed-access results,
  login return-path restrictions, account selection, and no-referrer CSRF behavior.
- Offline module checks cover local account separation, persistence, conflict
  review, idempotent retries, last-successful-sync status, and removed-access queues.
- `TestCosmosSharing` passed against the existing live Cosmos account using
  disposable owner/member partitions: shared discovery/editing, private trip
  creation, concurrent independent checks, matching intent, conflict and removal.
  All five test documents were removed afterward.
- Chromium browser checks used two independent signed fixture sessions on separate
  localhost origins with the production routes, middleware, templates and live
  Cosmos store. Link request/approval, denied pre-approval access, shared template
  item editing, private trip creation and owner denial passed. The owner packed a
  tent while the member was offline packing a stove; offline reload preserved the
  queue and reconnect kept both packed. A stale unpack showed a conflict while
  the server stayed packed. Keeping shared state resolved it. Removal preserved
  an unsynced edit and displayed the access-removed message. All five fixture
  documents were removed afterward.

Reproduce live checks with:

```sh
CAMPLIST_LIVE_COSMOS=1 go test ./internal/validation -run TestCosmosSharing -count=1 -v
CAMPLIST_LIVE_COSMOS=1 CAMPLIST_BROWSER=1 go test ./internal/validation -run TestBrowserSimulation -count=1 -v -timeout=40m
```

For two browser accounts, use `camplist-validation.localhost:3001/__validation/account`
and `camplist-member.localhost:3001/__validation/account?name=other`. The fixture
supports `__validation/network?offline=1&host=camplist-member.localhost:3001` to
interrupt only the member. Restore with `offline=0`, then stop gracefully via
`/__validation/stop` to clean up. These endpoints exist only in the test binary.

Read-only environment inspection confirmed Cosmos Session consistency, one West
Europe region, and multiple writes disabled. ETags protect writes; no stronger
claim is made about instantaneous read revocation or erasure of downloaded copies.
Google's console shows an External audience in Testing. No audience/billing
settings were changed. The two-account browser checks replace Google sign-in with
signed fixture cookies; a real invitation round-trip with two independent Google
accounts and physical-device Safari/Firefox offline acceptance remain unverified.
Browser storage eviction, closed-browser background sync, transfer of ownership,
and email delivery are not promises of this release. Measure shared polling RU
and receipt/document growth with real usage.

## Final review

Reviewed all changes since `4cc3376`, including item editing and the current UI.
The standards review found two issues, both fixed and re-reviewed: inline item
responses now reload the committed revision so immediate deletion succeeds, and
preparation-task removal uses the established confirmation dialog. An authenticated
shared edit-to-delete regression failed with 409 before the fix and passes after it.
There are no remaining material standards findings. The spec review found no
blocking gaps; the real Google-account and physical-device limitations above remain.

Final checks passed: `npm run check`, all 12 Node tests, `templ generate -check`,
`go test -race ./...`, `go vet ./...`, `go build ./...`, and `git diff --check`.
