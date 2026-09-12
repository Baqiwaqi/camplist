# Landing page brief (design team, Sep 2026)

Source canvas: `Camplist Landing Page.dc.html` (desktop 1280 and phone layouts),
preview in `thumbnail.webp`. Implemented in `internal/views/landing.templ`.

## Brief

Landing page for Camplist. Audience: weekend campers, families, backpackers.
Primary CTA: try a demo list. Lead story: pack once, reuse every trip (list vs
session); free, no caps, any device. One polished page, desktop and mobile.

## Market research

Gear apps (LighterPack, Packstack, Hikt, PackWizard) compete on gram-level
weight tracking, gear catalogs and calorie planning, which is heavy for a
weekend trip. Reusable-list apps (Packd, Repack, Checked) win on "make it once,
reset with a tap", but cap free lists at 1 to 3 and are phone-only. Users' real
fears: forgetting an item, and rebuilding the same list every trip.

## Design decisions

Show the product, not adjectives: the hero previews a live packing session.
The list-vs-session split is the one idea people must get, so it gets the only
explainer. One ember action per screen (Try a demo list). No icons, no imagery,
no shadows, per system. Mobile keeps the session preview above the fold and the
CTA thumb-reachable.

## Implementation notes

- Copy uses the product's own words: "Sign in with Google" (not "Login with
  google"), rows say "Pack" / "Unpack" with the tick, and progress reads
  "5 of 5 items packed" instead of "checked items: 5 / 5".
- Row buttons stay secondary so the ember stays on "Try a demo list".
- `/demo` is the full session behind the CTA; it runs on Alpine state only.
