import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../static/revision.js', import.meta.url), 'utf8')

// listPage runs revision.js against a list page whose #list-revision holds
// revision and returns a dispatcher for the htmx events it listens to.
function listPage(revision) {
  const listeners = {}
  const elements = new Map([['list-revision', { value: revision }]])
  const document = {
    addEventListener(event, handler) { (listeners[event] ||= []).push(handler) },
    getElementById(id) { return elements.get(id) ?? null },
  }
  vm.runInNewContext(source, { document, window: { addEventListener() {} } })
  const fire = (event, detail) => {
    let prevented = false
    for (const handler of listeners[event] || []) handler({ detail, preventDefault() { prevented = true } })
    return prevented
  }
  return { current: elements.get('list-revision'), elements, fire }
}

// send models the request htmx builds for a save and returns what it sends.
function send(page, { formRevision, headerRevision } = {}) {
  const formData = new Map(formRevision === undefined ? [] : [['revision', formRevision]])
  const headers = headerRevision === undefined ? {} : { 'X-Camplist-Revision': headerRevision }
  page.fire('htmx:configRequest', { formData, headers, verb: 'post' })
  return { form: formData.get('revision'), header: headers['X-Camplist-Revision'] }
}

// card renders the preparation card: one form per [form, inputs] entry, each
// input an [id, rendered value] pair, registered on the page by id.
function card(page, forms) {
  const inputs = []
  const built = forms.map(fields => {
    const form = { closest: () => form }
    form.inputs = fields.map(([id, value]) => ({ id, value, defaultValue: value, form }))
    inputs.push(...form.inputs)
    for (const input of form.inputs) page.elements.set(input.id, input)
    return form
  })
  const target = { id: 'list-preparation', querySelectorAll: () => inputs }
  return { target, forms: built }
}

// swap models htmx swapping in a new card after a save from elt: beforeSwap,
// the new card replacing the old, then the request's end.
function swap(page, target, elt, newForms) {
  const ended = []
  const xhr = { addEventListener(event, handler) { if (event === 'loadend') ended.push(handler) } }
  page.fire('htmx:beforeSwap', { target, elt, xhr })
  const next = card(page, newForms)
  for (const handler of ended) handler()
  return next
}

function startTripPage(form) {
  let pageshow
  const reloads = []
  const window = {
    addEventListener(event, handler) {
      if (event === 'pageshow') pageshow = handler
    },
  }
  const document = {
    addEventListener() {},
    getElementById(id) { return id === 'start-trip' ? form : null },
  }
  const location = { reload() { reloads.push(true) } }
  vm.runInNewContext(source, { document, window, location })
  return persisted => {
    pageshow({ persisted })
    return reloads.length
  }
}

test('htmx saves send the page revision instead of the one a control was rendered with', () => {
  const page = listPage('r2')
  assert.deepEqual(send(page, { formRevision: 'r1' }), { form: 'r2', header: undefined })
  page.current.value = 'r3'
  assert.deepEqual(send(page, { headerRevision: 'r1' }), { form: undefined, header: 'r3' })
})

test('saves racing each other are sent at once with the same revision and never sent again', () => {
  const page = listPage('r1')
  let issued = 0
  const confirm = { elt: {}, verb: 'post', issueRequest: () => issued++ }
  assert.equal(page.fire('htmx:confirm', confirm), false)
  assert.deepEqual(send(page, { formRevision: 'r1' }), { form: 'r1', header: undefined })
  assert.deepEqual(send(page, { headerRevision: 'r1' }), { form: undefined, header: 'r1' })
  assert.equal(issued, 0)
})

test('text typed into the new task and an open rename survives a card swap from another control', () => {
  const page = listPage('r1')
  const { target, forms } = card(page, [[['task-a', 'Gas']], [['task-b', 'Tent']], [['new-task', '']]])
  forms[1].inputs[0].value = 'Tent pegs'
  forms[2].inputs[0].value = 'Buy fuel'
  swap(page, target, forms[0], [[['task-a', 'Gas']], [['task-b', 'Tent']], [['new-task', '']]])
  assert.equal(page.elements.get('task-a').value, 'Gas')
  assert.equal(page.elements.get('task-b').value, 'Tent pegs')
  assert.equal(page.elements.get('new-task').value, 'Buy fuel')
})

test('the saved form comes back as the server rendered it', () => {
  const page = listPage('r1')
  const { target, forms } = card(page, [[['task-a', 'Gas']], [['new-task', '']]])
  forms[1].inputs[0].value = 'Buy fuel'
  swap(page, target, forms[1], [[['task-a', 'Gas']], [['task-buy', 'Buy fuel']], [['new-task', '']]])
  assert.equal(page.elements.get('new-task').value, '')
})

test('swaps outside the preparation card are left alone', () => {
  const page = listPage('r1')
  const { forms } = card(page, [[['new-task', '']]])
  forms[0].inputs[0].value = 'Buy fuel'
  swap(page, { id: 'add-item', querySelectorAll: () => forms[0].inputs }, {}, [[['new-task', '']]])
  assert.equal(page.elements.get('new-task').value, '')
})

test('a list page restored from the back/forward cache reloads so Start trip works again', () => {
  assert.equal(startTripPage({})(true), 1)
})

test('a fresh page load does not reload', () => {
  assert.equal(startTripPage({})(false), 0)
})

test('pages without Start trip ignore a restore', () => {
  assert.equal(startTripPage(null)(true), 0)
})
