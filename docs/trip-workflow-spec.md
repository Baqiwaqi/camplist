# Agreed trip workflow

The interview approved these changes alongside a separate realtime experiment.

- English navigation: Trips (`/trips`) and Packing lists (`/packing-lists`), with legacy links supported. Saved trips appear within Trips with an Available offline label.
- Trips have their own suggested name (list and date); their owner can rename them. Template names stay unchanged.
- Template gear and preparation tasks default to Shared. Per person creates separate unchecked copies for every participant. A participant joining later receives copies from the trip's current definitions, never a reread of the latest template.
- Everyone sees shared and personal entries grouped by participant. Shared entries can be checked by any trip member; personal entries only by their assigned participant.
- During a trip, additions choose Shared, Just for me, or For each person. Also save for future trips is explicit and requires independent template edit access.
- Preparation remains actionable during the trip and resets for each new trip.
- Only the original template owner manages template access; only the trip creator manages trip access. Trip members may add shared entries.
- Category autocomplete suggests existing categories in this list/trip and permits new values.
- Previously opened trips support offline adding and checking gear and tasks. Preserve durable operation IDs, permission rechecks, conflict review, automatic reconnect synchronization and polling fallback. Invitations and permission changes require connectivity.

Approved test seams: public packing store, authenticated HTTP routes, and offline save/edit/sync. Realtime transport and infrastructure belong to a separate investigation.

## Implementation and validation

Trip documents retain the source list name and a separate trip name. New trips also
retain the authenticated creator's display name. Legacy trips keep their existing
name and owner-label fallbacks.

Per-person sources and assignees live in the trip snapshot. Approval expands
missing source/participant pairs in the same conditional Cosmos write as membership.
Total expanded entries are bounded at 2,000. Preparation and gear share the sync
operation interface; personal permission checks remain server-authoritative.

The device snapshot includes preparation as entries with `kind: task`, permitting
one durable queue and conflict-review flow. A separate durable future-save queue
handles the cross-resource partial-success case: a trip addition can succeed while
saving to its reusable list fails. The trip queue continues, the UI explains the
partial result, and the camper can retry or stop the future save. No membership or
invitation metadata enters that snapshot.

Canonical routes coexist with legacy aliases, including legacy POST/DELETE
handlers. Existing bookmarks and queued older clients remain usable. The public
service-worker shell also opens `/trips` and `/trips/{id}` when offline. It never
caches authenticated HTML. The existing polling and refresh seam remain intact;
no streaming endpoint or infrastructure change is part of this implementation.

Validation performed in the isolated implementation branch:

- Go race suite, vet and build; generated template checks; JavaScript syntax checks
  and the full Node suite.
- Public-store regressions for personal ownership, late joining from trip sources,
  reset preparation, duplicate operation replay, independent future-template access,
  and consistent task attribution across form and sync APIs.
- HTTP regressions for authenticated task addition/completion, rejecting assignee
  injection, partial-save form retries without duplicate entries, and server-rendered
  participant groups. Existing legacy-route browser HTTP integration stays intact.
- Two isolated headless Chrome accounts against a loopback memory fixture: invite
  request/approval, per-person copies and permissions, offline personal task add and
  completion, offline reload, reconnect reconciliation, visibility to the other
  account, trip rename and Trips navigation. No page JavaScript errors.
- A 390px viewport check showed no horizontal overflow; current-list category
  suggestions were present. This is browser emulation, not a physical-device test.

Standards and Spec reviews compared against `7fc051e`. One substantive Standards
finding (fallback task attribution) and two Spec findings (fallback partial saves
and preparation grouping) were fixed and re-reviewed with no remaining substantive
findings. This branch does not claim production deployment or real Google-account
end-to-end validation.
