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

  updateCategories(datalist, [{ category: 'Fishing' }, { category: ' tarps ' }, { category: '' }, { category: 'fishing' }])
  assert.deepEqual(datalist.options.map(o => o.value), ['Shelter', 'Tarps', 'Fishing'])

  updateCategories(datalist, [{ category: 'Paddling' }])
  assert.deepEqual(datalist.options.map(o => o.value), ['Shelter', 'Tarps', 'Paddling'])
})
