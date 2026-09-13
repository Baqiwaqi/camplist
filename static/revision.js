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

// Saves there run one at a time, so each is sent with the revision the save
// before it swapped in. A save started while another runs waits its turn (a
// confirmed delete waits after its dialog); a control that is already saving
// or waiting drops the repeat.
let saving = null
const waiting = []

const isListSave = detail => detail.verb !== 'get' && document.getElementById('list-revision')

function issueInTurn(elt, issue) {
  if (saving?.elt === elt || waiting.some(entry => entry.elt === elt)) return
  if (saving) waiting.push({ elt, issue })
  else issue()
}

document.addEventListener('htmx:confirm', event => {
  const { detail } = event
  if (!isListSave(detail)) return
  const issueRequest = detail.issueRequest
  if (detail.question) {
    detail.issueRequest = () => issueInTurn(detail.elt, () => issueRequest(true))
    return
  }
  if (saving || waiting.length) {
    event.preventDefault()
    issueInTurn(detail.elt, () => issueRequest(true))
  }
})

document.addEventListener('htmx:beforeRequest', event => {
  const { requestConfig, xhr, elt } = event.detail
  if (isListSave(requestConfig)) saving = { xhr, elt }
})

document.addEventListener('htmx:afterRequest', event => {
  if (event.detail.xhr !== saving?.xhr) return
  saving = null
  while (!saving && waiting.length) waiting.shift().issue()
})

// Once a trip starts, Start trip stays disabled and htmx keeps the form busy
// while the trip page loads. A list page restored from the back/forward cache
// keeps that state and stale revisions, so load it again.
window.addEventListener('pageshow', event => {
  if (event.persisted && document.getElementById('start-trip')) location.reload()
})
