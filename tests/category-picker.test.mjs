import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../static/category-picker.js', import.meta.url), 'utf8')

// load runs static/category-picker.js against a fake document holding one
// datalist per entry in lists ({ value, custom?, default? } options) and the
// given picker inputs. It returns the script's listeners, its Alpine
// components and the context's globals.
function load(lists = [], inputs = []) {
  const listeners = {}
  const factories = {}
  const dispatched = []
  const datalists = lists.map(options => {
    const datalist = {
      options: [],
      append(option) { option.remove = () => datalist.options.splice(datalist.options.indexOf(option), 1); datalist.options.push(option) },
    }
    for (const { value, custom, used, trip, default: isDefault } of options) {
      const dataset = {}
      if (custom) dataset.custom = ''
      if (used) dataset.used = ''
      if (trip) dataset.trip = ''
      if (isDefault) dataset.default = ''
      datalist.append({ value, dataset })
    }
    return datalist
  })
  const context = {
    document: {
      addEventListener(event, handler) { listeners[event] = handler },
      createElement: () => ({ value: '', dataset: {} }),
      querySelectorAll(selector) {
        if (selector === 'datalist') return datalists
        if (selector.startsWith('input')) return inputs
        return []
      },
    },
    window: { dispatchEvent: event => dispatched.push(event) },
    CustomEvent: class { constructor(type, init) { this.type = type; this.detail = init.detail } },
    Alpine: { data(name, make) { factories[name] = make } },
  }
  vm.runInNewContext(source, context)
  listeners['alpine:init']()
  return { listeners, factories, datalists, dispatched, context }
}

// picker is a categoryPicker component whose datalist offers options.
function picker(options) {
  const { factories, datalists } = load([options])
  const component = factories.categoryPicker()
  component.$root = { querySelector: () => datalists[0] }
  component.open = true
  component.filtering = true
  return component
}

const defaults = ['Shelter', 'Clothing', 'Kitchen and cooking'].map(value => ({ value, default: true }))

test('a close typo offers the existing category before creating the new one', () => {
  const component = picker([...defaults, { value: 'Fishing', custom: true }])
  component.query = 'Fishnig'
  assert.deepEqual(Array.from(component.matches(), item => [item.kind, item.label, item.value]), [
    ['suggestion', 'Use “Fishing”?', 'Fishing'],
    ['create', 'Create “Fishnig”', 'Fishnig'],
  ])
  component.query = 'Fishin'
  assert.deepEqual(Array.from(component.matches(), item => item.label), ['Use “Fishing”?', 'Create “Fishin”'])
  component.query = 'Kitchen and cookign'
  assert.equal(component.matches()[0].label, 'Use “Kitchen and cooking”?')
})

test('the close match needs four letters and at most one edit', () => {
  const component = picker([...defaults, { value: 'Fishing', custom: true }, { value: 'Tarps', custom: true }])
  for (const [typed, want] of [['Shleter', 'Shelter'], ['Clothnig', 'Clothing'], ['Tarp', 'Tarps'], ['Hut', undefined], ['Fshng', undefined], ['Kitchn and cookign', undefined], ['Paddling', undefined]]) {
    component.query = typed
    const first = component.matches()[0]
    assert.equal(first?.kind === 'suggestion' ? first.value : undefined, want, typed)
  }
  component.query = 'fishing'
  assert.deepEqual(Array.from(component.matches(), item => item.label), ['Fishing'])
  component.query = 'Paddling'
  assert.deepEqual(Array.from(component.matches(), item => item.label), ['Create “Paddling”'])
})

test('only remembered custom categories offer rename and remove', () => {
  const component = picker([...defaults, { value: 'Fishnig', custom: true }, { value: 'Paddling' }])
  component.filtering = false
  assert.deepEqual(Array.from(component.matches().filter(item => item.custom), item => item.value), ['Fishnig'])
})

test('a rename updates every picker and a removal drops the option', () => {
  const { listeners, datalists } = load([
    [...defaults, { value: 'Fishnig', custom: true }],
    [...defaults, { value: 'Fishnig', custom: true }, { value: 'Fishing' }],
  ])
  listeners['categories-changed']({ detail: { from: 'Fishnig', to: 'Fishing', custom: true} })
  for (const datalist of datalists) {
    assert.deepEqual(datalist.options.map(option => [option.value, 'custom' in option.dataset]).slice(3), [['Fishing', true]])
  }

  listeners['categories-changed']({ detail: { from: 'fishing', to: '', custom: false} })
  assert.deepEqual(datalists[0].options.map(option => option.value), ['Shelter', 'Clothing', 'Kitchen and cooking'])

  listeners['categories-changed']({ detail: { from: 'Shleter', to: 'Shelter', custom: false} })
  assert.deepEqual(datalists[1].options.map(option => option.value), ['Shelter', 'Clothing', 'Kitchen and cooking'])
})

test('a rename leaves open fields alone and keeps the categories items in view use', () => {
  const editing = { value: 'Fishnig' }
  const { listeners, datalists } = load([
    [...defaults, { value: 'Fishnig', custom: true, used: true }],
    [...defaults, { value: 'Fishnig', trip: true }, { value: 'Tarps', custom: true, used: true }],
  ], [editing])
  listeners['categories-changed']({ detail: { from: 'Fishnig', to: 'Fishing', custom: true} })
  assert.equal(editing.value, 'Fishnig')
  assert.deepEqual(datalists[0].options.map(option => [option.value, 'custom' in option.dataset]).slice(3), [['Fishnig', false], ['Fishing', true]])
  assert.deepEqual(datalists[1].options.map(option => option.value).slice(3), ['Fishnig', 'Tarps', 'Fishing'])

  listeners['categories-changed']({ detail: { from: 'Tarps', to: '', custom: false} })
  assert.deepEqual(datalists[1].options.map(option => [option.value, 'custom' in option.dataset]).slice(3), [['Fishnig', false], ['Tarps', false], ['Fishing', true]])
})

test('a rename that moved the items in view drops the old name there but leaves open fields alone', () => {
  const editing = { value: 'Fishnig' }
  const { listeners, datalists } = load([
    [...defaults, { value: 'Fishnig', custom: true, used: true }],
  ], [editing])
  listeners['categories-changed']({ detail: { from: 'Fishnig', to: 'Fishing', custom: true, renamedInView: true} })
  assert.equal(editing.value, 'Fishnig')
  assert.deepEqual(datalists[0].options.map(option => [option.value, 'custom' in option.dataset]).slice(3), [['Fishing', true]])
})

test('the picker a rename starts from follows the new name, other fields do not', () => {
  const editing = { value: 'Fishnig' }
  const elsewhere = { value: 'Fishnig' }
  const { listeners, factories, datalists } = load([[...defaults, { value: 'Fishnig', custom: true }]], [editing, elsewhere])
  const component = factories.categoryPicker()
  component.$refs = { input: editing }
  component.manage('Fishnig')
  listeners['categories-changed']({ detail: { from: 'Fishnig', to: 'Fishing', custom: true, renamedInView: true } })
  assert.equal(editing.value, 'Fishing')
  assert.equal(elsewhere.value, 'Fishnig')
  assert.deepEqual(datalists[0].options.map(option => option.value).slice(3), ['Fishing'])

  // The next change comes from somewhere else, so no field follows it.
  listeners['categories-changed']({ detail: { from: 'Fishing', to: 'Angling', custom: true, renamedInView: true } })
  assert.equal(editing.value, 'Fishing')
})

test('a merge that unified case on the list in view stops offering the old spelling', () => {
  const editing = { value: 'Fishnig' }
  const { listeners, datalists } = load([
    [...defaults, { value: 'Fishnig', custom: true, used: true }, { value: 'fishing', custom: true, used: true }],
  ], [editing])
  listeners['categories-changed']({ detail: { from: 'Fishnig', to: 'Fishing', custom: true, renamedInView: true } })
  assert.deepEqual(datalists[0].options.map(option => [option.value, 'custom' in option.dataset]).slice(3), [['Fishing', true]])
  assert.equal(editing.value, 'Fishnig')
})

test('a merge elsewhere keeps the spelling the items in view use', () => {
  const { listeners, datalists } = load([[...defaults, { value: 'fishing', trip: true }]])
  listeners['categories-changed']({ detail: { from: 'Fishnig', to: 'Fishing', custom: true } })
  assert.deepEqual(datalists[0].options.map(option => option.value).slice(3), ['fishing'])
})

// A remembered category's row has two cells: the category and its Edit
// button. Right and left move between them, so the button a screen reader now
// sees is one a keyboard can reach.
test('right and left move between a remembered category and its Edit button', () => {
  const input = { id: 'item-category', value: '' }
  const component = picker([...defaults, { value: 'Fishnig', custom: true }])
  component.$refs = { input }
  component.filtering = false
  const event = { preventDefault() { this.prevented = true } }

  component.active = 3
  assert.equal(component.activeID(), 'item-category-option-3')
  component.across(event, 1)
  assert.equal(event.prevented, true)
  assert.equal(component.column, 1)
  assert.equal(component.activeID(), 'item-category-action-3')

  component.across(event, -1)
  assert.equal(component.column, 0)

  // Past either end of the row the field keeps the key, so the caret moves.
  const ignored = { preventDefault() { this.prevented = true } }
  component.across(ignored, -1)
  assert.equal(ignored.prevented, undefined)
  assert.equal(component.column, 0)
})

test('a category with no Edit button leaves the arrow keys to the field', () => {
  const input = { id: 'item-category', value: '' }
  const component = picker([...defaults, { value: 'Fishnig', custom: true }])
  component.$refs = { input }
  component.filtering = false
  component.active = 0
  const event = { preventDefault() { this.prevented = true } }
  component.across(event, 1)
  assert.equal(event.prevented, undefined)
  assert.equal(component.column, 0)
})

test('Enter on the Edit cell opens the category dialog instead of picking', () => {
  const input = { id: 'item-category', value: '', focus() {} }
  const { factories, datalists, dispatched } = load([[...defaults, { value: 'Fishnig', custom: true }]], [input])
  const component = factories.categoryPicker()
  component.$root = { querySelector: () => datalists[0] }
  component.$refs = { input }
  component.open = true
  component.filtering = false
  component.active = 3
  component.column = 1
  const event = { preventDefault() { this.prevented = true } }
  component.enter(event)
  assert.equal(event.prevented, true)
  assert.equal(input.value, '')
  assert.deepEqual(dispatched.map(sent => [sent.type, sent.detail.name]), [['category-rename', 'Fishnig']])
})

test('pointing at an Edit button leaves Enter picking the category', () => {
  const input = { id: 'item-category', value: '', focus() {} }
  const { factories, datalists, dispatched } = load([[...defaults, { value: 'Fishnig', custom: true }]], [input])
  const component = factories.categoryPicker()
  component.$root = { querySelector: () => datalists[0] }
  component.$refs = { input }
  component.open = true
  component.filtering = false
  component.point(3)
  assert.equal(component.activeID(), 'item-category-option-3')
  const event = { preventDefault() {} }
  component.enter(event)
  assert.equal(input.value, 'Fishnig')
  assert.deepEqual(dispatched, [])
})

test('moving between rows returns to the category cell', () => {
  const input = { id: 'item-category', value: '' }
  const component = picker([...defaults, { value: 'Fishnig', custom: true }])
  component.$refs = { input }
  component.$nextTick = () => {}
  component.filtering = false
  component.active = 3
  component.column = 1
  component.move(1)
  assert.equal(component.column, 0)
})
