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
