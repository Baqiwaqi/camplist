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
// or waiting drops the repeat. A save that finishes can swap out a waiting
// control (the preparation card swaps whole), so the wait replays that save on
// the control with the same id; without one it tells the user to try again
// once the saves queued with it are done.
let saving = null
let replaying = null
let skipped = false
const waiting = []

const isListSave = detail => detail.verb !== 'get' && document.getElementById('list-revision')
const sameControl = (a, b) => a === b || (Boolean(a.id) && a.id === b.id)

function issueInTurn(detail, issue) {
  const { elt } = detail
  if ((saving && sameControl(saving.elt, elt)) || waiting.some(entry => sameControl(entry.elt, elt))) return
  const entry = { elt, trigger: detail.triggeringEvent?.type, issue }
  if (saving) waiting.push(entry)
  else issue()
}

function replay({ elt, trigger, issue }) {
  if (elt.isConnected) {
    issue()
    return true
  }
  const replacement = elt.id && trigger && document.getElementById(elt.id)
  if (!replacement) return false
  replaying = replacement
  replacement.dispatchEvent(new Event(trigger, { bubbles: true, cancelable: true }))
  replaying = null
  return true
}

document.addEventListener('htmx:confirm', event => {
  const { detail } = event
  if (!isListSave(detail)) return
  const issueRequest = detail.issueRequest
  if (detail.elt === replaying) {
    replaying = null
    event.preventDefault()
    event.stopPropagation()
    issueRequest(true)
    return
  }
  if (detail.question) {
    detail.issueRequest = () => issueInTurn(detail, () => issueRequest(true))
    return
  }
  if (saving || waiting.length) {
    event.preventDefault()
    issueInTurn(detail, () => issueRequest(true))
  }
})

document.addEventListener('htmx:beforeRequest', event => {
  const { requestConfig, xhr, elt } = event.detail
  if (isListSave(requestConfig)) saving = { xhr, elt }
})

document.addEventListener('htmx:afterRequest', event => {
  if (event.detail.xhr !== saving?.xhr) return
  saving = null
  while (!saving && waiting.length) if (!replay(waiting.shift())) skipped = true
  if (skipped && !saving) {
    skipped = false
    document.dispatchEvent(new CustomEvent('camplist:save-skipped', { bubbles: true }))
  }
})

// Once a trip starts, Start trip stays disabled and htmx keeps the form busy
// while the trip page loads. A list page restored from the back/forward cache
// keeps that state and stale revisions, so load it again.
window.addEventListener('pageshow', event => {
  if (event.persisted && document.getElementById('start-trip')) location.reload()
})
