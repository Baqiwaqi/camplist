import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../static/removal-focus.js', import.meta.url), 'utf8')

// node is just enough of an element for removal-focus.js: a tag, attributes,
// children and the lookups the script makes.
function node(tag, attrs = {}, children = []) {
  const el = {
    tag, attrs, children, parentElement: null,
    dataset: attrs['data-empty-focus'] ? { emptyFocus: attrs['data-empty-focus'] } : {},
    getAttribute(name) { return name in attrs ? attrs[name] : null },
    focus() { focused.current = el },
    descendants() { return el.children.flatMap(child => [child, ...child.descendants()]) },
    contains(other) { return other === el || el.descendants().includes(other) },
    closest(selector) {
      for (let at = el; at; at = at.parentElement) if (matches(at, selector)) return at
      return null
    },
    querySelectorAll(selector) { return el.descendants().filter(child => matches(child, selector)) },
    querySelector(selector) { return el.querySelectorAll(selector)[0] ?? null },
  }
  for (const child of children) child.parentElement = el
  return el
}

// matches understands the selectors the script uses.
function matches(el, selector) {
  return selector.split(',').map(part => part.trim()).some(part => {
    if (part === '[data-empty-focus]') return 'data-empty-focus' in el.attrs
    if (part === 'a[href]') return el.tag === 'a' && 'href' in el.attrs
    if (part === 'input:not([type=hidden])') return el.tag === 'input' && el.attrs.type !== 'hidden'
    return el.tag === part
  })
}

const focused = { current: null }

const row = (name, controls) => node('li', { id: `item-${name}` }, controls)
const editLink = name => node('a', { id: `edit-${name}`, href: '#' })

// gear builds the list page's grouped gear with Mugs open for editing, its
// select popup holding li options of its own.
function gear() {
  const del = node('button', { 'hx-swap': 'delete' })
  const mugs = row('mugs', [node('input', { type: 'text' }), node('ul', {}, [node('li', {}, []), node('li', {}, [])]), del])
  const list = node('div', { id: 'list-items', 'data-empty-focus': 'item-name-new' }, [
    node('div', {}, [node('ul', {}, [row('stove', [editLink('stove')]), mugs])]),
    node('div', {}, [node('ul', {}, [row('tarp', [editLink('tarp')])])]),
  ])
  return { list, mugs, del }
}

function run(target, activeElement, extra = {}) {
  let handler
  const document = {
    activeElement,
    addEventListener(event, fn) { if (event === 'htmx:beforeSwap') handler = fn },
    getElementById(id) { return extra[id] ?? null },
  }
  vm.runInNewContext(source, { document })
  focused.current = null
  handler({ detail: { target, shouldSwap: true } })
  return focused.current
}

test('deleting the last row of a group focuses the next group’s first row, past the open row’s own options', () => {
  const { mugs, del } = gear()
  assert.equal(run(mugs, del).attrs.id, 'edit-tarp')
})

test('deleting the last row focuses the previous row', () => {
  const { list } = gear()
  const tarp = list.querySelectorAll('li').find(li => li.attrs.id === 'item-tarp')
  const del = node('button', { 'hx-swap': 'delete' })
  tarp.children.push(del)
  del.parentElement = tarp
  const previous = run(tarp, del)
  assert.equal(previous.tag, 'input', 'the open Mugs row takes focus in its first field')
})

test('deleting the only row focuses the add field', () => {
  const del = node('button', { 'hx-swap': 'delete' })
  const only = row('only', [del])
  node('div', { 'data-empty-focus': 'item-name-new' }, [node('ul', {}, [only])])
  const field = node('input', { id: 'item-name-new' })
  assert.equal(run(only, del, { 'item-name-new': field }), field)
})

test('a swap that is not a delete leaves focus alone', () => {
  const { mugs } = gear()
  const save = node('button', { type: 'submit' })
  mugs.children.push(save)
  save.parentElement = mugs
  assert.equal(run(mugs, save), null)
})
