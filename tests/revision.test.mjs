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
  vm.runInNewContext(source, { document })
  return detail => listener({ detail })
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
