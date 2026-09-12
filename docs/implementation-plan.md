# Implementation plan: trip review and offline packing

Spec: [Product direction](product-direction.md).

## Slices

1. Persist post-trip observations and apply selected additions, removals, and preparation tasks to the current template using a revision check and durable applied-entry IDs. Keep sessions as history.
2. Expose the review and preparation workflow through authenticated HTML forms, including a recovery action when the template was deleted.
3. Add revision-aware, deduplicated session synchronization and an authenticated JSON interface that never trusts client ownership.
4. Implement an offline packing module with atomic local session/outbox writes, recoverable in-flight operations, conflict choices, and account isolation.
5. Add a generic cached offline shell, explicit download readiness, sign-out cleanup/export, and recovery messages.
6. Verify the agreed seams, review standards and spec independently, resolve findings, and commit on the current branch.

## Test seams approved by the user

- Public packing-store operations: review history, selective/idempotent application, preparation tasks, revision conflicts, and sync retry behavior. Use a stateful database adapter without cloud credentials.
- Authenticated HTTP routes: form/JSON validation, ownership, CSRF, response status, and normal/HTMX behavior.
- Offline packing module: save, reopen, edit, sync, conflict resolution, account changes, and storage failures. Persistence and network adapters are supplied at the seam.

## Verification limits

Human camper interviews and two real camping trips are validation activities, not implementation acceptance tests. Browser-specific offline behavior must be labeled experimental until verified on supported browsers. No live user data is needed for automated tests. The user approved including the current design-system UI changes in the feature commit. Review all changes since `f568a6630b6a48a46929c7fe6db5eba0965b9fbb`.

Automated tests use a stateful Cosmos adapter and IndexedDB simulation. Real Cosmos concurrency, Google OAuth, and full offline close/reopen behavior across supported browsers still need environment validation. The local browser preview rendered the new online packing page; it is not evidence of complete browser offline acceptance.


## Delivery notes

All six implementation slices are complete. New sessions use the most recent
retained session of the same template as the improvement boundary; deleting that
prior session can cause earlier improvements to appear again. Session preparation
is labeled as a snapshot at session start, with a link to manage current tasks.

Sign-out cleanup checks the exported snapshot within the same IndexedDB
transaction that deletes it. Concurrent edits cause cleanup to stop for a fresh
export. If identity or local storage is unavailable, the camper can explicitly
sign out of the server while keeping potentially inaccessible local copies.

Durable sync receipts currently remain within the session document. Very long
sessions can approach Cosmos document limits; production load validation should
measure document growth before promoting offline support beyond experimental.
