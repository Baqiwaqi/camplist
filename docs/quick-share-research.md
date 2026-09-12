# Quick sharing invitation links

Research date: 2026-09-12. Scope: the current repository and primary platform,
standards, and app documentation. This document records the recommendation that
the subsequent implementation follows.

## Recommendation

Add one progressively enhanced **Share invitation** button beside the existing
**Copy link** button. When the browser supports it, call `navigator.share()`
with the invitation URL and a short explanation. This opens the operating
system's own share picker, where installed and eligible targets can include
WhatsApp, Signal, Messages/SMS, Mail, Nearby/Quick Share, AirDrop, contacts, or
other apps chosen by the user. The Web Share specification deliberately leaves
the target list to the browser and operating system; a site does not enumerate
or choose installed targets. [W3C Web Share API](https://www.w3.org/TR/web-share/#share-targets)

Keep **Copy link** visible in every browser. Web Share availability depends on
the browser and operating system, and the API requires HTTPS, permission to use
`web-share`, and a direct user gesture. Feature detection is therefore part of
the design, not an exceptional path. [W3C API definition](https://www.w3.org/TR/web-share/#api-definition),
[W3C implementation status](https://www.w3.org/TR/web-share/#implementation-status)

Do not add a made-up Signal deep link. Signal officially participates in the
native share mechanisms: its Android app accepts `ACTION_SEND` including
`text/plain`, and its iOS project contains a share extension. No first-party
generic browser URL for composing an arbitrary Signal message was found.
[Signal Android manifest](https://github.com/signalapp/Signal-Android/blob/main/app/src/main/AndroidManifest.xml),
[Signal iOS share extension](https://github.com/signalapp/Signal-iOS/blob/main/SignalShareExtension/ShareViewController.swift)

If product evidence later shows a useful desktop gap, add **WhatsApp** and
**Email** as optional fallback links only when native Web Share is unavailable.
WhatsApp documents `https://wa.me/?text=<URL-encoded text>` for a prefilled
message with contact selection. Email has the standardized `mailto:` scheme.
These fallbacks should supplement, not replace, the system picker.
[WhatsApp click to chat](https://faq.whatsapp.com/5913398998672934),
[RFC 6068: `mailto`](https://datatracker.ietf.org/doc/html/rfc6068)

## Why the system share picker is the right primary interface

| Option | Coverage | Recommendation |
| --- | --- | --- |
| `navigator.share({title, text, url})` | Uses the user's installed, eligible share targets and device-level ranking. | Primary action when supported. |
| Existing clipboard action | Works when no suitable share target exists and keeps the URL available for any channel. | Always retain. |
| WhatsApp `wa.me/?text=...` | Official web/mobile path; can open contact selection without Camplist knowing a phone number. | Optional desktop fallback. |
| Signal-specific URL | No documented generic compose/share URL found. `signal.me` links in Signal's app are for Signal-owned deep-link purposes, not a public arbitrary-message composer. | Do not implement. Use native share. |
| `mailto:?subject=...&body=...` | Opens the configured mail handler with a draft. | Optional fallback. |
| `sms:?body=...` | Standardized draft mechanism, but the actual handler and browser/device behavior vary. | Let native share expose Messages first; add only after physical-device testing. |
| Individual social-network buttons | Expands maintenance and sends a private invitation capability through more service-specific URLs. | Omit unless user research identifies a real channel. |

Android explicitly recommends its Sharesheet instead of an app-owned list of
targets because the system has better target and contact suggestions and a
consistent ranking. Apple's guidance similarly recommends the system activity
view and warns against duplicating common actions already present there.
[Android Sharesheet guidance](https://developer.android.com/develop/ui/compose/sharing/send#why-use-the-android-sharesheet),
[Apple activity-view guidance](https://developer.apple.com/design/human-interface-guidelines/activity-views)

## Proposed Camplist behavior

The current flow creates the invitation on the server, renders its absolute URL
once, and says it will not be shown again. The new action should reuse that exact
rendered value. It must not create another invitation merely because the user
opens, cancels, or retries the share picker. This preserves the current one-link-
per-person model, seven-day expiry, and owner-approval step.

A suitable payload is intentionally short:

```js
const data = {
  title: "Camplist invitation",
  text: "Join my Camplist. Sign in and request access; I will approve your account.",
  url: invitationLink,
};
```

Recommended interaction:

1. Render **Copy link** unconditionally.
2. Render or reveal **Share invitation** only when `navigator.share` exists and,
   where available, `navigator.canShare(data)` accepts the payload.
3. Call `navigator.share(data)` synchronously from the button click. The API
   consumes transient user activation, so it must not wait on a preliminary
   network request. [W3C share algorithm](https://www.w3.org/TR/web-share/#share-method)
4. Treat `AbortError` as a normal cancellation and leave the link available.
   For another failure, show a brief error and keep **Copy link** usable.
5. Do not claim that the invitation was delivered. Promise resolution differs
   by platform and can mean only that the share UI opened or that data reached a
   target, not that a recipient received or opened it.
   [MDN `navigator.share()` return behavior](https://developer.mozilla.org/en-US/docs/Web/API/Navigator/share#return_value)

The current `SharingView` contains kind/ID/owner/actor but not the resource name,
so a first implementation can use the generic copy above without expanding the
server view. If the name is added later, keep the approval explanation: the link
does not itself grant access.

No third-party JavaScript SDK is needed. The feature fits the current Alpine
enhancement already used by the clipboard button and preserves the invitation
page's deliberate no-third-party-assets posture.

## Channel-specific notes

### WhatsApp

WhatsApp's official no-recipient form is
`https://wa.me/?text=${encodeURIComponent(message)}`; it then presents contacts.
This is a reasonable fallback because it also works with WhatsApp Web. Use the
documented HTTPS form rather than undocumented `whatsapp://` links. If opened in
a new tab, prevent opener/referrer leakage using the normal safe link attributes.
[WhatsApp click to chat](https://faq.whatsapp.com/5913398998672934)

### Signal

Use the system share picker. Signal registers as an Android receiver for shared
plain text and implements an iOS share extension, so it can appear when installed
and enabled. Availability is controlled by the OS and the user's configuration;
Camplist should not promise that a named app will always appear. There is no
supported reason to detect whether Signal is installed.
[Signal Android manifest](https://github.com/signalapp/Signal-Android/blob/main/app/src/main/AndroidManifest.xml),
[Signal iOS share extension](https://github.com/signalapp/Signal-iOS/blob/main/SignalShareExtension/ShareViewController.swift)

### Email and SMS

`mailto:` permits percent-encoded subject and body fields. RFC 5724 permits an
`sms:` URI with a percent-encoded UTF-8 `body` field and says it should open a
composer rather than send without confirmation. Apple's URL-scheme guidance,
however, says an `sms:` URL must not include message text. That first-party
conflict makes body-prefill a best-effort behavior, not a portable promise. Use
URI builders/`encodeURIComponent`, never concatenate the capability URL into
query syntax without encoding.
[RFC 6068 sections 2 and 5](https://datatracker.ietf.org/doc/html/rfc6068#section-2),
[RFC 5724 sections 2.2 and 2.3](https://datatracker.ietf.org/doc/html/rfc5724#section-2.2),
[Apple SMS links](https://developer.apple.com/library/archive/featuredarticles/iPhoneURLScheme_Reference/SMSLinks/SMSLinks.html)

Both are handler-launch mechanisms, not delivery services. Blank-recipient email
is useful on desktop. SMS should remain a lower-priority, mobile-only fallback
until iPhone and Android acceptance checks confirm that the intended browsers
prefill the body consistently.

## Security and privacy boundary

The invitation URL contains a random capability token. Owner approval prevents
the URL alone from granting list access, but forwarding it still exposes the open
request path to another account. Share only the minimum text and URL, never member
data, Google identity details, or checklist contents.

Share targets may fetch a URL to create a preview, and the Web Share specification
calls out the resulting information-leak risk. Preserve the current properties:
GET only displays/request status, joining requires authenticated CSRF-protected
POST, the link expires, request logs redact invitation paths, and invitation pages
send no third-party assets or referrers. Consider a deliberately generic preview;
do not put the token or private list content into Open Graph metadata.
[W3C Web Share privacy considerations](https://www.w3.org/TR/web-share/#privacy-considerations)

Direct fallback links necessarily reveal the encoded invitation to the selected
service when opened. That is expected for sending it, but it is another reason to
keep the system picker and clipboard as the default rather than render a large
catalog of service-specific outbound links.

## Acceptance checks

- iPhone Safari over HTTPS: Share opens the activity view; installed Signal and
  WhatsApp can accept the URL; cancel leaves Copy link intact.
- Android Chrome over HTTPS: Share opens the Android Sharesheet; WhatsApp, Signal,
  Messages, and Quick Share appear only when eligible on that device.
- Desktop browsers with and without Web Share: supported browsers open the native
  picker; unsupported browsers retain Copy link and any selected fallback links.
- Share a generated invitation, cancel, and retry: the displayed URL is unchanged
  and only one invitation record exists.
- Expired, revoked, and already-claimed links retain their current server behavior
  regardless of channel.
- A messaging service's preview fetch cannot request access, disclose checklist
  content, or leak the invitation through third-party page resources/referrers.
- Keyboard and screen-reader use announces a normal **Share invitation** button;
  cancellation is not presented as an error.

## Suggested scope

First release: native **Share invitation** plus the existing **Copy link**.
Validate it on physical iOS and Android devices. Only then decide from usage or
support feedback whether desktop-only WhatsApp and Email shortcuts earn their
extra UI. This covers Signal and device-specific channels more reliably than a
hard-coded list while keeping the current security model unchanged.
