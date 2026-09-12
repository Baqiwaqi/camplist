# Product direction: better repeat trips and dependable offline packing

Date: 2026-09-12. Status: **product direction selected by the user**. The user chose improving each trip through forgotten/unused/replacement gear and offline check-off for an existing session with later synchronization. The implementation now includes post-trip reviews, selected template improvements, preparation tasks, and experimental offline packing. The contract below remains the acceptance specification; browser and live-service validation limits are recorded in the implementation plan.

## The product promise to test

**Make the next camping trip easier than the last.**

Initial audience hypothesis: people who camp repeatedly and already have familiar gear, but forget what needed changing after the last outing.

The proposed workflow is:

1. Prepare a new session from a familiar list.
2. Pack using a checklist that remains usable without connectivity.
3. After the trip, record forgotten items, unused gear, and things needing repair or replacement.
4. Review proposed changes and deliberately apply selected changes to the reusable list.
5. On the next trip, surface unresolved preparation tasks and the changes made since the previous outing.

Example: the camper notes that the stove ran out of fuel. Camplist carries a “replace gas canister” task into preparation for the next trip. Checking the stove as packed must not silently complete that preparation task.

This is a hypothesis about a useful experience, not a claim of a feature no competitor has. Reuse and offline access already appear in [Packr](https://packr.app/) and [PackParrot](https://packparrot.com/). The [market research](market-research.md) also identifies alternatives for shared household planning. Offline support would underpin reliability; the improvement between real trips is the proposed reason to choose Camplist.

## A small first product slice

Implement an online post-trip review with three independent observations: forgotten, unused, and needs attention. Preserve the session as history. Let users propose an addition, removal, or preparation task, then select which proposals affect future trips.

Do not automatically remove an unused item: an unused first-aid kit may still be essential. Applying a review twice must not duplicate items or preparation tasks. If the template changed after the trip, show the current state and resolve conflicts before applying changes. If the template was deleted, retain the historical review and offer an explicit new-list action.

A preparation task is separate from an item's packed state. Finishing a repair or buying fuel does not mean the item has been packed. Start with this distinction only where needed by review outcomes, without introducing a general task manager.

Validate with five to eight repeat campers over two actual trips. Look for whether they finish a review, use its changes next time, and prefer the process to their previous checklist. Record examples of avoided repeat mistakes. Those observations would inform the direction; this small pilot cannot establish market demand or pricing.

## Selected offline scope

**A session opened and automatically saved on this device can be reopened and checked off without connectivity. Changes synchronize when the app is open and connectivity and authentication are available.**

The first offline release will support:

- Automatically save an existing session when it is opened online. Confirm offline readiness only after local storage and required app files are successfully written. Keep the regular checklist usable as connectivity changes.
- Read the session, pack/unpack items, and view locally calculated progress offline.
- Retain pending changes across closing and reopening the app.
- Show distinct states: saved on this device, waiting to sync, synchronized, sign-in required, or needs a decision.
- Keep creating sessions, editing templates, and applying post-trip reviews online initially.

A download-ready badge must not appear before the app can actually reopen the session offline. If storage fails or is unavailable, keep the online experience usable and explicitly say the offline copy was not saved. Browser storage can be cleared or evicted, so local-only changes are not a cloud backup. [IndexedDB and storage behavior](https://developer.mozilla.org/en-US/docs/Web/API/IndexedDB_API)

## Technical design

Keep Go, chi, templ, and HTMX for online pages. Introduce one offline packing **module**, with the packing screen as its **seam**. Its small interface should let the screen open a saved session, set an item's desired state, observe save/sync status, and request synchronization. It should hide persistence, retries, and reconciliation from event handlers.

Use IndexedDB transactions for the saved session and pending changes. Use a service worker to make the static offline shell and its assets available without a network. These technologies provide local structured storage and request interception/caching respectively; they do not by themselves define synchronization or conflict behavior. [IndexedDB](https://developer.mozilla.org/en-US/docs/Web/API/IndexedDB_API), [Service workers](https://developer.mozilla.org/en-US/docs/Web/API/Service_Worker_API)

The offline packing screen owns its displayed item state; HTMX must not simultaneously replace that screen. This limited client-side implementation can coexist with the rest of the server-rendered app. HTMX's guidance explicitly identifies full offline operation as a poor fit for pure hypermedia. [HTMX architecture guidance](https://htmx.org/essays/when-to-use-hypermedia/)

### Save and synchronize contract

1. A packing action atomically saves both the local desired state and its pending operation. Only then show it as saved on the device.
2. Each operation identifies the session, item, desired Boolean state, expected server item revision, and a stable operation ID. The authenticated server determines the owner; a client-supplied user ID is not authorization.
3. On reconnect, the server applies a new operation only against the expected revision and records its acknowledgement atomically with the change. Retrying an acknowledged operation returns its result without another state transition.
4. Changes to different items can merge. If the same item's revision changed, compare the current server state with the local intention. If they disagree, preserve both and ask which state should win. Do not resolve this using device timestamps.
5. An acknowledgement removes only the operation it acknowledges. It must not erase a newer local change made while the request was in flight. Coalesce unsent changes per item; serialize in-flight operations for that item.
6. If the session was deleted, preserve an exportable local copy and explain that it cannot sync. Never silently recreate a deleted server session.
7. Refresh server state after reconciliation and retain any newer pending local intentions over it.

The existing `SetSessionItem` writes a desired Boolean, which helps retries, but its identity condition does **not** detect an old offline state overwriting a more recent state. Offline synchronization still needs server revisions and acknowledgement handling. The existing ETag protection for list replacements does not provide this automatically.

### Authentication and device behavior

Cache a generic offline shell and automatically saved session data for trips the user opens, rather than indiscriminately caching authenticated HTML or login responses. Keep local records separated by account. While authentication is expired, an already saved local session can remain usable, but synchronization pauses until the same account signs in and a fresh CSRF token is obtained.

Do not synchronize one account's pending changes after a different account signs in. Define sign-out to remove local account data, with an explicit opportunity to sync or export pending changes first. Offline access is a device feature and must not be presented as continuing server authentication.

Attempt synchronization after each packing edit, on app opening, successful reconnect, returning to the app, and periodic retries of pending work. Keep manual retry and export under recovery options. Do not promise that synchronization occurs while the app is closed; background execution is not a prerequisite of this design.

## Delivery sequence and acceptance checks

1. **Post-trip review:** record observations, propose changes, and apply selected changes once. Verify old sessions remain intact and review proposals are useful on a second trip.
2. **Offline persistence:** open a session and wait for automatic offline readiness, then disable connectivity, close/reopen the app, and check/uncheck items. Verify progress and pending changes survive.
3. **Synchronization:** reconnect and verify the same session on another device. Test lost acknowledgements, repeated requests, rapid local changes, and conflicting changes to the same item.
4. **Account and recovery cases:** test expired login, another account signing in, session deletion, storage failure, sign-out with pending changes, and app upgrades with queued operations.

Until these checks pass in supported browsers, describe offline behavior as experimental. The design is intended to keep offline complexity within the packing module while preserving the existing Go application.
