# Implementation review

Baseline: `f568a6630b6a48a46929c7fe6db5eba0965b9fbb`.
Candidate: `2410595`. Reviewed with `git diff f568a66...HEAD`, followed by
focused review of the fixes in the working tree. Scope includes the current UI
changes, as approved by the user. Spec: [product direction](product-direction.md).

## Standards

The independent standards review found two documented violations and two
heuristic concerns:

- Repeated ember actions in the sessions overview violated CLAUDE.md's one-ember-action rule. Changed repeated actions to pine.
- Overview tests asserted SQL text instead of observable results, contrary to the TDD skill. Replaced them with public-operation assertions using mixed documents.
- The test database embedded an uninitialized Cosmos client. Replaced the embedding with explicit methods and a clear unsupported-patch error.
- Review actions used scattered strings. Added a named action type and constants for validation, application, and views.

Focused recheck: all four findings addressed; no unresolved findings.

## Spec

The independent spec review found three issues:

- Export followed by unconditional account cleanup could discard a newer edit from another tab. Cleanup now compares the exported snapshot inside the deletion transaction. A two-instance regression test verifies that concurrent changes stop deletion.
- The next-trip screen omitted preparation tasks and lacked an improvement boundary. New sessions show unresolved preparation from their initial snapshot and changes since the latest retained session of the same template.
- Unavailable storage or expired identity blocked normal sign-out. An explicit server-only sign-out option explains that local copies remain. Saved offline offers export and removal; invalid authentication cookies can still be expired.

Focused recheck: all three findings addressed; no remaining concrete blockers
from these findings. Browser-level acceptance remains pending.

Standards: 4 initial findings, 0 unresolved. Spec: 3 initial findings, 0 unresolved.

## Verification

Passed after the fixes:

- `templ generate`
- `go test ./...`
- `go vet ./...`
- `go build ./...`
- `npm test` — 9 offline module tests
- `npm run check`
- `git diff --check`

Tests exercise the approved public store, authenticated HTTP, and offline module
seams. The database adapter models conditional document writes; it does not
replace live Cosmos testing. IndexedDB tests use fake-indexeddb and do not replace
service-worker/cache validation in real browsers. The online packing page was
opened in a local Chrome preview, but full offline acceptance remains unverified.
See the [implementation plan](implementation-plan.md) for remaining limitations.
