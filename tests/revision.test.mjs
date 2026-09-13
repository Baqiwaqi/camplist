import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../static/revision.js', import.meta.url), 'utf8')

// listPage runs revision.js against a list page whose #list-revision holds
// revision and returns a dispatcher for the htmx events it listens to.
function listPage(revision) {
  const listeners = {}
  const current = { value: revision }
  const document = {
    addEventListener(event, handler) { (listeners[event] ||= []).push(handler) },
    getElementById(id) { return id === 'list-revision' ? current : null },
  }
  vm.runInNewContext(source, { document, window: { addEventListener() {} } })
  const fire = (event, detail) => {
    let prevented = false
    for (const handler of listeners[event] || []) handler({ detail, preventDefault() { prevented = true } })
    return prevented
  }
  return { current, fire }
}

// save models one htmx save from elt: confirm, then (when issued) the request
// htmx builds at that moment and its completion.
function save(page, elt, { question, formRevision, headerRevision } = {}) {
  const sent = []
  const xhr = {}
  const request = () => {
    const formData = new Map(formRevision === undefined ? [] : [['revision', formRevision]])
    const headers = headerRevision === undefined ? {} : { 'X-Camplist-Revision': headerRevision }
    page.fire('htmx:configRequest', { formData, headers, verb: 'post' })
    page.fire('htmx:beforeRequest', { requestConfig: { verb: 'post' }, xhr, elt })
    sent.push({ form: formData.get('revision'), header: headers['X-Camplist-Revision'] })
  }
  const detail = { elt, question, verb: 'post', issueRequest: () => request() }
  const prevented = page.fire('htmx:confirm', detail)
  if (question) detail.issueRequest()
  else if (!prevented) request()
  return { sent, finish: newRevision => { page.current.value = newRevision; page.fire('htmx:afterRequest', { xhr }) } }
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
  const form = save(page, {}, { formRevision: 'r1' })
  assert.deepEqual(form.sent, [{ form: 'r2', header: undefined }])
  form.finish('r3')
  const del = save(page, {}, { headerRevision: 'r1', question: 'Delete?' })
  assert.deepEqual(del.sent, [{ form: undefined, header: 'r3' }])
})

test('a save started while another runs waits and is sent with the revision the first swapped in', () => {
  const page = listPage('r1')
  const first = save(page, {}, { headerRevision: 'r1', question: 'Delete?' })
  const second = save(page, {}, { headerRevision: 'r1', question: 'Delete?' })
  const third = save(page, {}, { formRevision: 'r1' })
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
  const toggle = {}
  const running = save(page, toggle, { formRevision: 'r1' })
  const repeat = save(page, toggle, { formRevision: 'r1' })
  const other = {}
  const waiting = save(page, other, { formRevision: 'r1' })
  const repeatWaiting = save(page, other, { formRevision: 'r1' })
  running.finish('r2')
  assert.equal(running.sent.length, 1)
  assert.equal(repeat.sent.length, 0)
  assert.equal(waiting.sent.length, 1)
  assert.equal(repeatWaiting.sent.length, 0)
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
