import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../static/revision.js', import.meta.url), 'utf8')

function element(attributes) {
  return {
    attributes: { ...attributes },
    getAttribute(name) { return this.attributes[name] },
    setAttribute(name, value) { this.attributes[name] = value },
  }
}

function listPage({ inputs, buttons }) {
  let listener
  const document = {
    addEventListener(event, handler) {
      if (event === 'list-revision') listener = handler
    },
    querySelectorAll(selector) {
      if (selector === 'input[name="revision"]') return inputs
      if (selector === '[hx-headers]') return buttons
      return []
    },
  }
  vm.runInNewContext(source, { document, window: { addEventListener() {} } })
  return detail => listener({ detail })
}

function startTripPage(form) {
  let pageshow
  const window = {
    addEventListener(event, handler) {
      if (event === 'pageshow') pageshow = handler
    },
  }
  const document = {
    addEventListener() {},
    getElementById(id) { return id === 'start-trip' ? form : null },
  }
  vm.runInNewContext(source, { document, window })
  return persisted => pageshow({ persisted })
}

function startedTrip() {
  const classes = new Set(['flex', 'htmx-request'])
  const button = { disabled: true }
  return {
    button,
    classList: { remove(name) { classes.delete(name) }, contains(name) { return classes.has(name) } },
    querySelectorAll(selector) { return selector === 'button' ? [button] : [] },
  }
}

test('mark done advances only controls still on the revision it was sent with', () => {
  const addTask = { value: 'r1' }
  const openEdit = { value: 'r1' }
  const editOpenedLater = { value: 'r3' }
  const deleteTent = element({ 'hx-headers': JSON.stringify({ 'X-CSRF-Token': 'token', 'X-Camplist-Revision': 'r1' }) })
  const deleteStoveLater = element({ 'hx-headers': JSON.stringify({ 'X-CSRF-Token': 'token', 'X-Camplist-Revision': 'r3' }) })
  const deleteList = element({ 'hx-headers': JSON.stringify({ 'X-CSRF-Token': 'token' }) })

  const advance = listPage({ inputs: [addTask, openEdit, editOpenedLater], buttons: [deleteTent, deleteStoveLater, deleteList] })
  advance({ from: 'r1', to: 'r2', elt: {} })

  assert.equal(addTask.value, 'r2')
  assert.equal(openEdit.value, 'r2')
  assert.equal(editOpenedLater.value, 'r3')
  assert.deepEqual(JSON.parse(deleteTent.getAttribute('hx-headers')), { 'X-CSRF-Token': 'token', 'X-Camplist-Revision': 'r2' })
  assert.equal(JSON.parse(deleteStoveLater.getAttribute('hx-headers'))['X-Camplist-Revision'], 'r3')
  assert.deepEqual(JSON.parse(deleteList.getAttribute('hx-headers')), { 'X-CSRF-Token': 'token' })
})

test('a trigger without both revisions changes nothing', () => {
  const input = { value: '' }
  const advance = listPage({ inputs: [input], buttons: [] })
  advance({ from: '', to: 'r2' })
  assert.equal(input.value, '')
})

test('a list page restored from the back/forward cache can start another trip', () => {
  const form = startedTrip()
  startTripPage(form)(true)
  assert.equal(form.button.disabled, false)
  assert.equal(form.classList.contains('htmx-request'), false)
})

test('a fresh page load leaves Start trip alone', () => {
  const form = startedTrip()
  startTripPage(form)(false)
  assert.equal(form.button.disabled, true)
  assert.equal(form.classList.contains('htmx-request'), true)
})

test('pages without Start trip ignore a restore', () => {
  assert.doesNotThrow(() => startTripPage(null)(true))
})
