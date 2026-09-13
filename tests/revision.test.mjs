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

test('a list page restored from the back/forward cache reloads so Start trip works again', () => {
  assert.equal(startTripPage({})(true), 1)
})

test('a fresh page load does not reload', () => {
  assert.equal(startTripPage({})(false), 0)
})

test('pages without Start trip ignore a restore', () => {
  assert.equal(startTripPage(null)(true), 0)
})
