// A fragment save on the list page (preparation Mark done) changes the list
// revision that every other form and delete button on the page embeds. The
// server triggers list-revision with the revision the request was sent with
// and the one its own save produced; only controls still on the sent revision
// move, so a control loaded after someone else's write keeps its conflict.
document.addEventListener('list-revision', event => {
  const { from, to } = event.detail
  if (!from || !to) return
  for (const input of document.querySelectorAll('input[name="revision"]')) {
    if (input.value === from) input.value = to
  }
  for (const element of document.querySelectorAll('[hx-headers]')) {
    const headers = JSON.parse(element.getAttribute('hx-headers'))
    if (headers['X-Camplist-Revision'] !== from) continue
    headers['X-Camplist-Revision'] = to
    element.setAttribute('hx-headers', JSON.stringify(headers))
  }
})

// Once a trip starts, Start trip stays disabled and htmx keeps the form busy
// while the trip page loads. A list page restored from the back/forward cache
// keeps that state and stale revisions, so load it again.
window.addEventListener('pageshow', event => {
  if (event.persisted && document.getElementById('start-trip')) location.reload()
})
