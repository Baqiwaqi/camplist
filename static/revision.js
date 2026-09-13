// The reusable list page shows one list revision, in #list-revision, and every
// save there is conditioned on it. Each save swaps in the revision it made out
// of band, so htmx requests send that input's value in place of the revision a
// control was rendered with. The rendered copies stay for forms posted
// without scripts, which reload the page after every save.
document.addEventListener('htmx:configRequest', event => {
  const current = document.getElementById('list-revision')
  if (!current) return
  const { formData, headers } = event.detail
  if (formData.has('revision')) formData.set('revision', current.value)
  if ('X-Camplist-Revision' in headers) headers['X-Camplist-Revision'] = current.value
})

// A preparation save swaps the whole card, so text still being typed into
// another task field there (the new task, an open rename) moves into the new
// card instead of being lost.
document.addEventListener('htmx:beforeSwap', event => {
  const { target, elt, xhr } = event.detail
  if (target.id !== 'list-preparation') return
  const form = elt.closest('form')
  const typed = [...target.querySelectorAll('input:not([type])')]
    .filter(input => input.id && input.form !== form && input.value !== input.defaultValue)
    .map(input => [input.id, input.value])
  if (!typed.length) return
  xhr.addEventListener('loadend', () => {
    for (const [id, value] of typed) {
      const input = document.getElementById(id)
      if (input) input.value = value
    }
  })
})

// Once a trip starts, Start trip stays disabled and htmx keeps the form busy
// while the trip page loads. A list page restored from the back/forward cache
// keeps that state and stale revisions, so load it again.
window.addEventListener('pageshow', event => {
  if (event.persisted && document.getElementById('start-trip')) location.reload()
})
