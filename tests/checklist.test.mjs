import { test } from 'node:test'
import assert from 'node:assert/strict'

function option(value, dataset = {}) {
  return { value, dataset, remove() { datalist.options.splice(datalist.options.indexOf(this), 1) } }
}
const datalist = { options: [], append(child) { this.options.push(child) } }
globalThis.document = { createElement: () => option('') }
const { updateCategories } = await import('../static/offline/checklist.mjs')

test('trip categories are replaced per trip while rendered options stay', () => {
  datalist.options = [option('Shelter'), option('Tarps')]

  const trip = items => ({ list: { items: items.map(category => ({ category })) } })

  updateCategories(datalist, trip(['Fishing', ' tarps ', '', 'fishing']))
  assert.deepEqual(datalist.options.map(o => o.value), ['Shelter', 'Tarps', 'Fishing'])

  updateCategories(datalist, trip(['Paddling']))
  assert.deepEqual(datalist.options.map(o => o.value), ['Shelter', 'Tarps', 'Paddling'])
})

test('the offline shell offers the snapshot defaults before the trip categories', () => {
  datalist.options = []
  const session = { categories: ['Shelter', 'Kitchen and cooking'], list: { items: [{ category: 'kitchen AND cooking' }, { category: 'Fishing' }] } }

  updateCategories(datalist, session)
  assert.deepEqual(datalist.options.map(o => o.value), ['Shelter', 'Kitchen and cooking', 'Fishing'])

  updateCategories(datalist, { ...session, list: { items: [] } })
  assert.deepEqual(datalist.options.map(o => o.value), ['Shelter', 'Kitchen and cooking'])
})
