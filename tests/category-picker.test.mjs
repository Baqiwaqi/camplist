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

test('the close match needs four letters and at most one edit on short names', () => {
  const component = picker([...defaults, { value: 'Fishing', custom: true }, { value: 'Tarps', custom: true }])
  for (const [typed, want] of [['Shleter', 'Shelter'], ['Clothnig', 'Clothing'], ['Tarp', 'Tarps'], ['Hut', undefined], ['Fshng', undefined], ['Paddling', undefined]]) {
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
