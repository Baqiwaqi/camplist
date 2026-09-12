# Shared packing lists with Google sign-in

Research date: 2026-09-12. Scope: current repository inspection and primary documentation. This is a proposal; no sharing feature, deployment, email delivery, or live-service validation was performed. Existing offline work in the working tree was inspected without changing it.

## Recommendation

Yes: each person can join using their own Google account. Keep the existing Google login and add resource memberships inside Camplist. Google establishes who signed in; Camplist decides which trip or template that person may access.

**User-confirmed scope includes both shared trip sessions and editable shared reusable lists.** Model these as independently shared resources: a session packer marks today's equipment packed or unpacked; a template editor changes the reusable list for future trips. Implementing session sharing first is a suggested sequence, not a reduction of the requested scope. Each resource has an owner who manages its invitations, membership, and deletion.

**The user selected the copyable invitation-link flow.** Keep the proposed owner-approval step: the recipient signs in with Google, requests to join, and the owner approves the actual signed-in account. This avoids an email-delivery dependency while preventing possession of a forwarded link from immediately granting checklist access. Direct email-bound invitations remain a future alternative. Sharing and approval are Camplist behavior, not Google-provided sharing features.

## Current application facts

| Observation | Repository evidence |
| --- | --- |
| Google OIDC authorization-code flow verifies the ID token and saves `claims.Sub` as `userID`. Requested scopes are `openid email profile`. | [Authentication](../internal/auth/client.go) |
| Claims include email and `email_verified`, but no `hd`; session values retain only subject and name. There is no persistent user directory here. | [Claims](../internal/auth/type.go), [callback](../internal/auth/client.go) |
| Lists and sessions have one `userId`; storage reads and writes use that user's partition. The documented container partition key is `/userId`. | [Store](../internal/packing/store.go), [README](../README.md) |
| A session embeds a snapshot of its template and resets its checkboxes. Existing sessions do not automatically change when the template changes. | [Session creation](../internal/packing/packing.go), [types](../internal/packing/type.go) |
| Whole-list replacements use ETags. Session sync uses operation IDs, expected item revisions, and conditional updates. | [Store](../internal/packing/store.go), [sync](../internal/packing/sync.go), [review updates](../internal/packing/review.go) |
| Offline code ties the local account to `session.userId`; the API checks `X-Camplist-Account` against the signed-in user. | [Offline packing](../static/offline/packing.mjs), [automatic sync](../static/offline/automatic.mjs), [API](../internal/web/offline.go) |

The main change is therefore authorization and storage lookup, not replacing authentication. Today, passing the current user ID into a store method also chooses the only partition they may access. A member needs authorized access to a resource in another user's partition.

## Identity: use the Google subject, not email

Google documents `sub` as unique and never reused, while a Google account's email can change. Continue using the verified subject as the membership key. Email and display name are presentation or invitation-address fields. If other identity providers are introduced later, identify external accounts by issuer plus subject. [Google ID-token verification](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token)

Proposed user profile: subject, display name, email, email-verification status, optional hosted-domain claim, and last authentication time. Populate it from verified server-side claims, not invitation form fields. A profile is useful for showing the owner who requested access; it is not a replacement for membership checks. Do not auto-merge different subjects because their email strings match.

Keep the existing scopes. This proposal stores collaboration in Cosmos and needs no Gmail, Drive, or Contacts access. Google supports `prompt=select_account` for choosing among signed-in accounts and `login_hint` for suggesting one; neither grants Camplist membership. Add an account-switch action and preserve the pending invitation through the login flow using server-controlled state and an internal return destination. [Google OpenID Connect](https://developers.google.com/identity/openid-connect/openid-connect)

Before testing invitations with friends, inspect the actual OAuth audience configuration: an Internal app restricts access to its organization; an External app supports outside Google accounts. Google's Testing restrictions have an explicit exception for basic sign-in scopes such as this app's `openid email profile`: these users need not be on the test-user list. Check Workspace administrator restrictions too. The deployed console settings were not inspected. [Google app audience](https://support.google.com/cloud/answer/15549945?hl=en)

## Invitation choices

| Approach | User experience | Trade-off |
| --- | --- | --- |
| Copy link — selected; approval as proposed | Owner copies a link; recipient signs in and requests access; owner approves their account. | No mail service; requires the owner to recognize the requester, potentially confirming through their existing conversation. |
| Email-bound invitation | Owner enters an address; recipient signs in with the matching account and accepts. | Familiar, but requires correct email authority checks and possibly mailbox verification. |
| Bearer invitation | Any signed-in account holding the link can accept immediately. | Fewest steps; forwarding or leaking the link transfers the ability to join. Label this behavior explicitly if offered. |

All three can support someone who has never used Camplist: create their local profile at first successful Google sign-in. Do not expose an account-search directory or reveal whether arbitrary email addresses are already registered.

### Email verification caveat

Google is authoritative for Gmail addresses, and for Workspace addresses when `email_verified` is true and `hd` is present. For other addresses, even `email_verified=true` can reflect verification performed when the Google account was created, rather than current mailbox ownership. [Google's email-authority rules](https://developers.google.com/identity/gsi/web/guides/verify-google-id-token)

Proposed email-bound acceptance rule: require the invitation address to match the verified sign-in claim, and require authoritative Google email evidence or a fresh Camplist challenge delivered to that mailbox. A link manually copied into chat is not proof of mailbox access. Once accepted, save membership against the subject; later authorization must not depend on repeating the email comparison. Avoid undocumented alias rewriting such as stripping dots or `+suffixes` from arbitrary addresses. Support resending to the precise address the recipient actually uses.

### Proposed invitation lifecycle

Use a random 32-byte secret, store only its hash, and set a short expiry (for example, seven days). Each invitation targets one resource and an explicit role. Store creator, timestamps, status, and accepted subject. For approval mode, a request binds the invitation to the requester; only the owner can activate membership. An expired, rejected, revoked, or consumed invitation cannot grant new access.

Require sign-in and a CSRF-protected POST to request, accept, approve, or revoke; loading a link must not join automatically. Use HTTPS, suppress referrer leakage (`Referrer-Policy: no-referrer`), avoid third-party assets on invitation pages, redact secrets from request logs, and rate-limit attempts. These token-handling recommendations adapt OWASP's one-time URL-token guidance; that document concerns password recovery, not a ready-made invitation protocol. [OWASP token guidance](https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html)

Commit acceptance and membership together. Retrying the same successful acceptance should return the existing result; another account must not be able to claim it. Removing an existing member is distinct from revoking an unused invitation. Owner approval should show the account identity, not only a user-editable display name.

## Membership and permissions

Proposed MVP permission matrix:

| Action | Owner | Packer | Non-member / pending |
| --- | --- | --- | --- |
| Read shared trip checklist | Yes | Yes | No |
| Mark items packed/unpacked | Yes | Yes | No |
| Invite, approve, remove members | Yes | No | No |
| Delete the trip | Yes | No | No |
| Read/edit the source template | Requires separate template permission | Requires separate template permission | Trip membership grants no template access |
| Apply the post-trip review to that template | Requires template edit permission | Requires template edit permission | No source-trip access |
| Leave the trip | Not without a future transfer flow | Yes | Not applicable |

A read-only viewer can be added if needed. For the required reusable-template sharing, use these separate grants:

| Template action | Owner | Editor | Non-member |
| --- | --- | --- | --- |
| Read, rename, add/remove/edit equipment and preparation tasks | Yes | Yes | No |
| Start a private trip from the template | Yes | Yes | No |
| Manage template invitations/members or delete template | Yes | No | No |
| See other people's trips made from the template | Only if separately granted trip access | Only if separately granted trip access | No |

Trip membership must never imply template-edit permission, nor does template membership grant access to past or future trips. Starting a private trip from a shared template creates an independent snapshot owned by the person starting it. This requires changing today's `NewPackingSession` behavior, which inherits the template owner. Previous-trip improvements and history must be selected from that actor's authorized trips, not the template owner's history. Editing a template does not retroactively alter session snapshots.

Applying a review requires permission to read the source session and edit the destination template. A session packer alone cannot apply changes to its template. Keep the existing explicit review/ETag conflict process; a removed template editor must lose review-application permission too. Memberships are independent, so an already-created private snapshot may remain after template access is removed.

Resolve actor identity from the authenticated session. Resolve the resource's storage location separately, then check membership for the requested operation on every server path: HTML, HTMX, JSON, sync, review, and deletion. Deny by default; guessing a resource ID or supplying an owner's ID does not authorize access. These principles follow [OWASP authorization guidance](https://cheatsheetseries.owasp.org/cheatsheets/Authorization_Cheat_Sheet.html).

Do not put a long-lived membership list into the login cookie. Use an explicit response shape so packers do not receive internal invitation hashes, pending applicants, operation receipts, or unrelated review/template fields. A shared session already embeds template information; define the shared view deliberately and explain to the owner what becomes visible.

## Cosmos: keep existing partitions for the MVP

Retain `userId` as the immutable storage owner for existing documents. Add a bounded membership map and bounded invitation records to each shared session or template document. This is the authoritative access record for that resource only. Legacy records without sharing metadata stay owner-only.

Add small discovery references in each member's own partition, containing resource type, resource ID, and storage owner ID. The member's home page lists these and rechecks each canonical resource before returning content. References are navigation indexes, never permission grants. Creation/removal may lag; retry and repair them. A missing reference affects discoverability, while a stale reference must not preserve access. Avoid copying the shared checklist into every member's partition.

Cosmos identifies an item by partition key plus ID. Its transaction boundary is one logical partition, and partition keys cannot be changed in place. Thus membership in the owner's partition and discovery references in the member's partition cannot be one cross-partition atomic update. A new partitioning scheme requires migration. [Cosmos partitioning](https://learn.microsoft.com/en-us/azure/cosmos-db/partitioning-overview)

Embedding the MVP's invitation and membership data permits a single conditional resource replacement. If these become separate documents, keep authoritative records in the same partition and design a transaction that actually conditions the resource write on the authorization state; a prior membership read alone is insufficient. Cosmos transactional batches support atomic operations within one logical partition. [Transactional batches](https://learn.microsoft.com/en-us/azure/cosmos-db/transactional-batch)

For longer-term household workspaces, consider a new `/workspaceId` or `/resourceId` container when group ownership or owner-independent lifecycles justify migration. Do not reassign existing `userId` values as if ownership transfer were a field edit. Owner account deletion and transfer need an explicit future policy.

Queries without a partition-key restriction can fan out. Prefer per-user discovery plus canonical point reads for normal navigation; inspect RU costs with real data before choosing a broader cross-partition listing. [Cosmos query routing](https://learn.microsoft.com/en-us/azure/cosmos-db/how-to-query-container)

## Concurrency and revocation

Keep desired-state operations (`checked=true/false`), item revisions, and duplicate-operation receipts. Include authenticated actor identity in new shared-operation receipt keys/records; clients cannot choose the actor. A retry must recheck current membership before returning shared state, even if that operation was accepted earlier.

For each write, read the canonical session or template, check the actor's current role, apply the operation, and replace using that document's ETag. On conflict, reload and repeat authorization before retrying. Revocation changes the same document, so an in-flight write based on earlier membership cannot silently overwrite it. Patches must provide equivalent atomic authorization conditions. ETags change on updates; stale `If-Match` writes fail with HTTP 412. [Cosmos optimistic concurrency](https://learn.microsoft.com/en-us/azure/cosmos-db/database-transactions-optimistic-concurrency)

Audit the existing direct patch paths too: `AddItem` and `DeletePackingList` currently condition only on document type/deletion state, bypassing whole-document replacement. Adding a membership check before these calls would leave a revocation race; change the atomic write condition or use the authorized ETag replacement path. Preserve sharing metadata in every full replacement.

This guards writes at the database commit boundary. Immediate read revocation also depends on the deployed consistency settings and caching; do not claim that a membership read always observes the newest revocation without validating those settings. Start with a single write region and verify actual behavior. Already delivered content cannot be recalled.

For two people editing different items, retry the document conflict while preserving each item's revision rule. For the same item, retain the existing explicit conflict choice. Refresh on focus and periodically while the shared screen is open; WebSockets are optional later. “Shared” should not imply instant presence indicators or character-level simultaneous editing.

## Offline implications

### User-required warning and preservation of other members' changes

**Shared offline packing must warn users that their view can be out of date, and synchronization must never silently undo another member's packing changes.** This is a user requirement for shared offline usage, not an optional improvement. Packed state belongs to a trip session; editing the reusable shared template remains online in the proposed first version.

Show a persistent warning when opening or using a shared trip without a successful current server refresh, including connection failures rather than relying only on the browser's online flag:

> You're offline. Other people's packing changes may be missing. Your changes are saved on this device and will sync when you reconnect. Conflicting changes will need your review.

For an online connection whose refresh/sync is failing, use “Can't sync this shared trip” instead of claiming the device is offline. Show the last successful sync time, or “Not synced yet,” and pending-change count. Keep the warning visible while edits are pending or freshness is unknown; reconnecting alone is not successful synchronization. Mark local changes as “Waiting to sync,” and conflicts as “Needs review.” If local storage fails, show “Couldn't save on this device” rather than promising persistence. Before saving a shared trip for offline use, explain that other members cannot see local changes until they sync.

Required synchronization behavior (proposed protocol extending the current item-revision implementation):

1. Send only explicit item changes, never an entire offline checklist replacement. An unchecked item in an old snapshot is not an instruction to unpack it.
2. Each change carries the item revision the member actually saw, a desired Boolean, and a stable operation ID. The server supplies actor identity. Never silently replace the expected revision with the newest revision and retry stale intent.
3. Apply a change only if its expected item revision still matches, using the conditional resource write described above. Changes to different items merge independently.
4. If the revision changed but the desired state already equals the server state, acknowledge the intent without changing that item's state, revision, or last-change attribution. Persist the receipt conditionally with the current resource version; recheck on a concurrent write. This is a proposed extension: today's `SyncSessionItem` returns a conflict for every revision mismatch.
5. If the revision changed and the states disagree, preserve the server state and the pending local intention separately. Automatic sync must not overwrite either. For example: “Alex marked Stove as packed while you were offline. Your change would mark it unpacked.” Store server-side last-change actor information to support that explanation, with a generic fallback if unavailable.
6. Default to keeping the shared state. Offer an explicit “Mark unpacked instead” or “Mark packed instead” action only after showing the current shared state. That deliberate resolution creates a new operation against the displayed revision; if another member changes the item again, require another review. Do not automatically resolve by latest device timestamp or by always preferring checked items, since deliberate unpacking must remain possible.
7. Acknowledgements remove only the acknowledged operation, preserving newer local edits. Retries are idempotent, and sync rechecks membership before accepting changes or returning shared data.

Acceptance examples for two independently signed-in members:

| Scenario | Required result |
| --- | --- |
| A packs the tent online; B's offline snapshot still shows it unchecked and B packs the stove | After B syncs, both tent and stove remain packed. |
| A changes an item to packed after B's snapshot; B has a pending unpack action for that item | The shared item stays packed; B sees a conflict until they keep the shared state or deliberately resolve it. |
| A and B both pack the same item from the same revision | The item stays packed; B's matching intent is acknowledged without overwriting A's attribution. |
| Another change arrives while B resolves a conflict | B's stale resolution fails safely and shows the updated conflict. |
| Connection returns but authentication, storage, or sync fails | The UI does not claim synchronization succeeded; pending changes and the relevant warning remain. |

### Account separation and removed access

The existing offline implementation needs a deliberate sharing change: separate the **signed-in/local account** from the **resource storage owner**. `packing.mjs` rejects snapshots whose `userId` differs from the local account, and `automatic.mjs` derives its account from the session owner. Keep IndexedDB partitioned by the signed-in member while storing the shared resource's locator separately. Continue sending the member identity in `X-Camplist-Account`.

Reauthorize every replay. Add an access-removed result distinct from expired sign-in; today some 403 responses become a sign-in prompt. On removal, stop synchronization and stop claiming pending changes will upload. Never recreate a deleted/revoked shared resource during replay. Keep a deliberate local export/delete choice for pending work.

Offline devices can retain copies they already downloaded; revocation cannot remotely erase disconnected storage or screenshots. Describe removal as stopping future server access. Do not enable offline saving for shared sessions until account separation, revoked-access handling, and member-specific cleanup are complete. Private offline sessions can remain as they are during an online sharing MVP.

## Suggested implementation slices and acceptance evidence

1. **Authorization boundary:** introduce actor/resource separation, owner/packer checks, and legacy owner-only behavior. Verify another user cannot access private data through any route.
2. **One shared online trip:** add link requests, owner approval, membership display/removal, and member discovery. Verify a new Google user can join; a forwarded link alone cannot read the checklist.
3. **Shared editable templates — required scope:** reuse invitation infrastructure with independent editor grants; support shared list edits and creating privately owned sessions. Verify template members cannot see another person's trip, session membership cannot edit templates, snapshots stay unchanged, and review application checks both resources.
4. **Race and failure handling:** test duplicate acceptance, two claimants, expiry/revocation, partial discovery writes, removed-member replay, concurrent edits, and revocation racing an edit against Cosmos. Template forms need visible conflict handling rather than silently retrying a whole stale form over newer content.
5. **Browser validation:** verify OAuth return-to-invitation, wrong-account switching, two accounts checking off items and editing the shared template, denied cross-resource access, and owner/member deletion behavior.
6. **Shared offline release gate:** verify the warning and two-member acceptance examples above, local account isolation, pending changes after removal, another account signing in, and duplicate acknowledgements before enabling shared offline packing.

User-confirmed decisions: share both sessions and editable reusable lists; use a copyable invitation link; warn for shared offline usage; never silently overwrite another member's packing changes during synchronization. Owner approval remains the proposed invitation behavior. Unresolved product choices: whether session members should submit post-trip observations and whether shared offline support must ship in the initial release. An online-first implementation sequence remains a recommendation; shared offline usage must satisfy the release gate whenever enabled.

Implementation follow-up: [shared lists and trips](shared-lists-implementation.md), including the user-selected shared offline release and validation evidence.
