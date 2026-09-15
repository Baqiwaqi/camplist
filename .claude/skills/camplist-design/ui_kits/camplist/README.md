# Camplist web app — UI kit

Click-through recreation of every templ view in `camplist/internal/views/`:

- **Login** (`login.templ`) — single "Login with google" link. Source is an unstyled anchor; kit centers it in a Card.
- **Lists / home** (`packing.templ`) — h1 "Camping", "Here you can find all you lists", Create, per-list Edit / Details / Delete.
- **New / Edit list** (`new-list-page.templ`) — Name + Description form, error banner, Cancel + Create list / Save.
- **List details** (`packing-details.templ`) — items with Delete, Edit + Start session, Add Item form (Name, Category).
- **Packing session** (`packing-session.templ`) — item rows with Check / Uncheck toggle.
- **Sessions overview** (`packing-sessions-page.templ`) — list name, date, "checked items: n / m", Delete Session.

Files: `index.html` (shell + router), `app.jsx` (state + routes mirroring chi routes), `screens.jsx` (one function per view), `data.js` (seed). Both guard against running outside this page (the compiler evaluates every .jsx).

Fidelity note: the source templates apply almost no layout (bare `<h1>`, `<li>`, unstyled buttons). The kit wraps content in the `.card` / `.item-row` patterns that `style.css` defines but the templates don't yet use, and styles buttons from the palette. Copy text is verbatim, typos included.
