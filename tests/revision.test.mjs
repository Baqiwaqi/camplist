import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../static/revision.js', import.meta.url), 'utf8')

// listPage runs revision.js against a list page whose #list-revision holds
// revision and returns a dispatcher for the htmx events it listens to.
// render puts a control with id on the page (a form, or with a question a
// button in one) holding fields, detaching any control that had that id. A
// swapped control, as a card swap inserts it, starts saving on its trigger
// only once htmx has processed it.
function listPage(revision) {
  const listeners = {}
  const current = { value: revision }
  const controls = new Map()
  const skipped = []
  const document = {
    addEventListener(event, handler) { (listeners[event] ||= []).push(handler) },
    getElementById(id) { return id === 'list-revision' ? current : controls.get(id) ?? null },
    dispatchEvent(event) { skipped.push(event.type) },
  }
  class Event { constructor(type) { this.type = type } }
  class FormData { constructor(form) { return Object.entries(form.fields).map(([name, field]) => [name, field.value]) } }
  const htmx = { process(elt) { elt.processed = true } }
  vm.runInNewContext(source, { document, window: { addEventListener() {} }, Event, CustomEvent: Event, FormData, htmx })
  const fire = (event, detail) => {
    const result = { prevented: false, stopped: false }
    for (const handler of listeners[event] || []) {
      if (result.stopped) break
      handler({ detail, preventDefault() { result.prevented = true }, stopPropagation() { result.stopped = true } })
    }
    return result
  }
  const page = { current, fire, skipped, saves: [] }
  page.render = (id, options = {}, { fields = {}, swapped = false } = {}) => {
    const old = controls.get(id)
    if (old) old.isConnected = false
    const form = { fields, elements: { namedItem: name => fields[name] ?? null } }
    const elt = options.question ? { closest: () => form } : form
    Object.assign(elt, {
      id,
      isConnected: true,
      processed: !swapped,
      dispatchEvent: event => { if (elt.processed) page.saves.push({ id, type: event.type, ...save(page, elt, options) }) },
    })
    form.closest = () => form
    controls.set(id, elt)
    return elt
  }
  page.remove = id => {
    controls.get(id).isConnected = false
    controls.delete(id)
  }
  page.input = (id, value = '') => {
    const input = { value }
    controls.set(id, input)
    return input
  }
  return page
}

// save models one htmx save from elt: confirm (a question opens the dialog,
// which the user accepts), then (when issued) the request htmx builds at that
// moment from elt's form and its completion.
function save(page, elt, { question, formRevision, headerRevision } = {}) {
  const sent = []
  const listeners = []
  const xhr = { addEventListener(event, handler) { if (event === 'loadend') listeners.push(handler) } }
  const request = () => {
    const formData = new Map(formRevision === undefined ? [] : [['revision', formRevision]])
    const headers = headerRevision === undefined ? {} : { 'X-Camplist-Revision': headerRevision }
    page.fire('htmx:configRequest', { formData, headers, verb: 'post' })
    page.fire('htmx:beforeRequest', { requestConfig: { verb: 'post' }, xhr, elt })
    const name = elt.closest?.('form').fields.name?.value
    sent.push({ form: formData.get('revision'), header: headers['X-Camplist-Revision'], ...(name === undefined ? {} : { name }) })
  }
  const triggeringEvent = { type: question ? 'click' : 'submit' }
  const detail = { elt, question, verb: 'post', triggeringEvent, issueRequest: () => request() }
  const { prevented, stopped } = page.fire('htmx:confirm', detail)
  let asked = false
  if (question && !stopped) {
    asked = true
    detail.issueRequest()
  } else if (!prevented) request()
  return { sent, asked, finish: newRevision => { page.current.value = newRevision; for (const handler of listeners) handler() } }
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
  const form = save(page, { isConnected: true }, { formRevision: 'r1' })
  assert.deepEqual(form.sent, [{ form: 'r2', header: undefined }])
  form.finish('r3')
  const del = save(page, { isConnected: true }, { headerRevision: 'r1', question: 'Delete?' })
  assert.deepEqual(del.sent, [{ form: undefined, header: 'r3' }])
})

test('a save started while another runs waits and is sent with the revision the first swapped in', () => {
  const page = listPage('r1')
  const first = save(page, { isConnected: true }, { headerRevision: 'r1', question: 'Delete?' })
  const second = save(page, { isConnected: true }, { headerRevision: 'r1', question: 'Delete?' })
  const third = save(page, { isConnected: true }, { formRevision: 'r1' })
  assert.equal(first.sent.length, 1)
  assert.equal(second.sent.length, 0)
  assert.equal(third.sent.length, 0)
  first.finish('r2')
  assert.deepEqual(second.sent, [{ form: undefined, header: 'r2' }])
  assert.equal(third.sent.length, 0)
  second.finish('r3')
  assert.deepEqual(third.sent, [{ form: 'r3', header: undefined }])
})

test('repeating a save from a control that is already saving or waiting is dropped', () => {
  const page = listPage('r1')
  const toggle = { isConnected: true }
  const running = save(page, toggle, { formRevision: 'r1' })
  const repeat = save(page, toggle, { formRevision: 'r1' })
  const other = { isConnected: true }
  const waiting = save(page, other, { formRevision: 'r1' })
  const repeatWaiting = save(page, other, { formRevision: 'r1' })
  running.finish('r2')
  assert.equal(running.sent.length, 1)
  assert.equal(repeat.sent.length, 0)
  assert.equal(waiting.sent.length, 1)
  assert.equal(repeatWaiting.sent.length, 0)
})

test('a waiting save whose control a swap replaced is sent from the replacement once htmx processed it, without asking again', () => {
  const page = listPage('r1')
  const toggleB = { formRevision: 'r1' }
  const removeC = { headerRevision: 'r1', question: 'Remove this preparation task?' }
  const first = save(page, page.render('task-toggle-a'), { formRevision: 'r1' })
  const toggle = save(page, page.render('task-toggle-b', toggleB), toggleB)
  const remove = save(page, page.render('task-remove-c', removeC), removeC)
  assert.equal(toggle.sent.length + remove.sent.length, 0)
  const repeat = save(page, page.render('task-toggle-b', toggleB), toggleB)
  page.render('task-toggle-a', {}, { swapped: true })
  page.render('task-toggle-b', toggleB, { swapped: true })
  page.render('task-remove-c', removeC, { swapped: true })

  first.finish('r2')
  assert.equal(toggle.sent.length + remove.sent.length + repeat.sent.length, 0)
  assert.deepEqual(page.saves.map(s => [s.id, s.type, s.sent, s.asked]), [['task-toggle-b', 'submit', [{ form: 'r2', header: undefined }], false]])

  page.saves[0].finish('r3')
  assert.equal(page.saves.length, 2)
  assert.deepEqual([page.saves[1].id, page.saves[1].type, page.saves[1].sent, page.saves[1].asked], ['task-remove-c', 'click', [{ form: undefined, header: 'r3' }], false])
  assert.deepEqual(page.skipped, [])
})

test('a task added while another save runs is sent with the name the user typed, not the empty new form', () => {
  const page = listPage('r1')
  const first = save(page, page.render('task-toggle-a'), { formRevision: 'r1' })
  const add = page.render('add-task', { formRevision: 'r1' }, { fields: { name: { value: 'Buy fuel' } } })
  save(page, add, { formRevision: 'r1' })
  page.render('add-task', { formRevision: 'r1' }, { fields: { name: { value: '' } }, swapped: true })

  first.finish('r2')
  assert.deepEqual(page.saves.map(s => [s.id, s.sent]), [['add-task', [{ form: 'r2', header: undefined, name: 'Buy fuel' }]]])
  assert.deepEqual(page.skipped, [])
})

test('a rename saved while a removal runs is sent with the new name, not the one the card re-renders', () => {
  const page = listPage('r1')
  const first = save(page, page.render('task-remove-a', { question: 'Remove?' }), { headerRevision: 'r1', question: 'Remove?' })
  const rename = page.render('task-rename-b', { formRevision: 'r1' }, { fields: { name: { value: 'Charge car' } } })
  save(page, rename, { formRevision: 'r1' })
  page.remove('task-remove-a')
  page.render('task-rename-b', { formRevision: 'r1' }, { fields: { name: { value: 'Charge phone' } }, swapped: true })

  first.finish('r2')
  assert.deepEqual(page.saves.map(s => [s.id, s.sent]), [['task-rename-b', [{ form: 'r2', header: undefined, name: 'Charge car' }]]])
})

test('a waiting save whose control is gone after the swap is skipped, keeps the typed name and tells the user', () => {
  const page = listPage('r1')
  const newTask = page.input('new-task')
  const first = save(page, page.render('task-remove-a', { question: 'Remove?' }), { headerRevision: 'r1', question: 'Remove?' })
  const gone = save(page, page.render('task-rename-a', {}, { fields: { name: { value: 'Buy fuel' } } }), { formRevision: 'r1' })
  const unnamed = save(page, { isConnected: true }, { formRevision: 'r1' })
  const next = save(page, page.render('task-toggle-b'), { formRevision: 'r1' })
  page.remove('task-remove-a')
  page.remove('task-rename-a')

  first.finish('r2')
  assert.equal(gone.sent.length, 0)
  assert.equal(newTask.value, 'Buy fuel')
  assert.deepEqual(unnamed.sent, [{ form: 'r2', header: undefined }])
  assert.deepEqual(page.skipped, [])
  unnamed.finish('r3')
  assert.deepEqual(next.sent, [{ form: 'r3', header: undefined }])
  assert.deepEqual(page.skipped, [])
  next.finish('r4')
  assert.deepEqual(page.skipped, ['camplist:save-skipped'])
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
