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
// or waiting drops the repeat. The turn passes when the request ends, since a
// swap can detach the control htmx reports the end on. A swap can also replace
// a waiting control (the preparation card swaps whole), so the save is replayed
// on the control with the same id with the values the user sent; when none
// takes it, the typed task name goes back into the new task field and the user
// is told to try again once the saves queued with it are done.
let saving = null
let replaying = null
let skipped = false
const waiting = []

const isListSave = detail => detail.verb !== 'get' && document.getElementById('list-revision')
const sameControl = (a, b) => a === b || (Boolean(a.id) && a.id === b.id)

function issueInTurn(detail, issue) {
  const { elt } = detail
  if ((saving && sameControl(saving.elt, elt)) || waiting.some(entry => sameControl(entry.elt, elt))) return
  const form = elt.closest?.('form')
  const name = form?.elements.namedItem('name')
  const entry = {
    elt,
    trigger: detail.triggeringEvent?.type,
    values: form ? [...new FormData(form)] : [],
    typed: form === elt && name && name.type !== 'hidden' ? name.value : '',
    issue,
  }
  if (saving) waiting.push(entry)
  else issue()
}

function replay({ elt, trigger, values, issue }) {
  if (elt.isConnected) return issue()
  const replacement = elt.id && trigger && document.getElementById(elt.id)
  if (!replacement) return
  const form = replacement.closest('form')
  for (const [name, value] of values) {
    const field = form?.elements.namedItem(name)
    if (field) field.value = value
  }
  htmx.process(replacement)
  replaying = replacement
  replacement.dispatchEvent(new Event(trigger, { bubbles: true, cancelable: true }))
  replaying = null
}

function passTurn(xhr) {
  if (xhr !== saving?.xhr) return
  saving = null
  while (!saving && waiting.length) {
    const entry = waiting.shift()
    replay(entry)
    if (saving) continue
    skipped = true
    const input = document.getElementById('new-task')
    if (entry.typed && input && !input.value) input.value = entry.typed
  }
  if (skipped && !saving) {
    skipped = false
    document.dispatchEvent(new CustomEvent('camplist:save-skipped', { bubbles: true }))
  }
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
  if (!isListSave(requestConfig)) return
  saving = { xhr, elt }
  xhr.addEventListener('loadend', () => passTurn(xhr))
})

// Once a trip starts, Start trip stays disabled and htmx keeps the form busy
// while the trip page loads. A list page restored from the back/forward cache
// keeps that state and stale revisions, so load it again.
window.addEventListener('pageshow', event => {
  if (event.persisted && document.getElementById('start-trip')) location.reload()
})
