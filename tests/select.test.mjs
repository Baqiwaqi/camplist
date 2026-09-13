import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../static/select.js', import.meta.url), 'utf8')

// select builds the Alpine component for views.Select around a fake native
// <select>, trigger and listbox, and runs its init like Alpine would.
function select({ labels, selectedIndex = 0, trigger = { top: 300, bottom: 344 }, listHeight = 120, viewportHeight = 844 }) {
  let init
  let factory
  const timers = []
  const document = {
    documentElement: { clientHeight: viewportHeight },
    addEventListener(event, handler) {
      if (event === 'alpine:init') init = handler
    },
  }
  const Alpine = {
    data(name, build) {
      if (name === 'select') factory = build
    },
  }
  class Event {
    constructor(type) { this.type = type }
  }
  const microtasks = []
  vm.runInNewContext(source, { document, Alpine, Event, Date, setTimeout: callback => timers.push(callback), queueMicrotask: callback => microtasks.push(callback) })
  init()
  const component = factory()
  const listeners = {}
  const form = {
    addEventListener(type, handler) { listeners[type] = handler },
    removeEventListener(type) { delete listeners[type] },
  }
  const native = {
    form,
    selectedIndex,
    changes: 0,
    options: labels.map(text => ({ text })),
    dispatchEvent(event) { if (event.type === 'change') this.changes++ },
  }
  const focused = []
  const scrolled = []
  component.$refs = {
    native,
    trigger: { getBoundingClientRect: () => trigger, focus() { focused.push('trigger') } },
    listbox: {
      offsetHeight: listHeight,
      querySelectorAll: () => labels.map(textContent => ({ textContent, scrollIntoView() { scrolled.push({ option: textContent, up: component.up }) } })),
    },
  }
  component.$root = { contains: node => node === 'inside' }
  component.$nextTick = callback => callback()
  component.init()
  return { component, native, form: { reset: () => { native.selectedIndex = 0; listeners.reset?.(); timers.splice(0).forEach(run => run()) }, listeners }, focused, scrolled, flush: () => { while (microtasks.length) microtasks.shift()() } }
}

function press(component, value, extra = {}) {
  const event = { key: value, prevented: false, preventDefault() { this.prevented = true }, ...extra }
  component.key(event)
  return event
}

test('the trigger shows the option the server selected', () => {
  const { component } = select({ labels: ['Shared', 'For each person'], selectedIndex: 1 })
  assert.equal(component.label(), 'For each person')
})

test('arrow keys, Home and End move through the list and Enter writes the native field', () => {
  const { component, native, focused } = select({ labels: ['Shared', 'Just for me', 'For each person'] })
  press(component, 'ArrowDown')
  assert.equal(component.open, true)
  assert.equal(component.active, 0)
  press(component, 'ArrowDown')
  assert.equal(component.active, 1)
  press(component, 'End')
  assert.equal(component.active, 2)
  press(component, 'Home')
  assert.equal(component.active, 0)
  press(component, 'ArrowUp')
  assert.equal(component.active, 0, 'ArrowUp stops at the first option')
  press(component, 'End')
  const enter = press(component, 'Enter')
  assert.equal(enter.prevented, true)
  assert.equal(component.open, false)
  assert.equal(native.selectedIndex, 2)
  assert.equal(native.changes, 1)
  assert.equal(component.label(), 'For each person')
  assert.deepEqual(focused, ['trigger'])
})

test('Escape closes without changing the value', () => {
  const { component, native } = select({ labels: ['Shared', 'For each person'] })
  press(component, 'Enter')
  press(component, 'ArrowDown')
  press(component, 'Escape')
  assert.equal(component.open, false)
  assert.equal(native.selectedIndex, 0)
  assert.equal(native.changes, 0)
})

test('typing opens the list on the first matching option', () => {
  const { component } = select({ labels: ['Packing item', 'Before departure task'] })
  press(component, 'b')
  assert.equal(component.open, true)
  assert.equal(component.active, 1)
})

test('Tab closes the list and lets focus move on', () => {
  const { component, focused } = select({ labels: ['Shared', 'For each person'] })
  press(component, 'ArrowDown')
  const tab = press(component, 'Tab')
  assert.equal(tab.prevented, false)
  assert.equal(component.open, false)
  assert.deepEqual(focused, [])
})

test('focus leaving the select closes it; moving inside does not', () => {
  const { component } = select({ labels: ['Shared', 'For each person'] })
  component.show()
  component.leave({ relatedTarget: 'inside' })
  assert.equal(component.open, true)
  component.leave({ relatedTarget: 'elsewhere' })
  assert.equal(component.open, false)
})

test('a form reset shows the default option again', () => {
  const { component, form } = select({ labels: ['Shared', 'Just for me', 'For each person'], selectedIndex: 2 })
  assert.equal(component.label(), 'For each person')
  form.reset()
  assert.equal(component.label(), 'Shared')
  component.destroy()
  assert.equal(form.listeners.reset, undefined)
})

test('the list opens upward only when it does not fit below and there is more room above', () => {
  const low = select({ labels: ['Shared', 'For each person'], trigger: { top: 760, bottom: 804 }, listHeight: 96 })
  low.component.show()
  assert.equal(low.component.up, true)
  const shortListNearBottom = select({ labels: ['Shared', 'For each person'], trigger: { top: 690, bottom: 734 }, listHeight: 96 })
  shortListNearBottom.component.show()
  assert.equal(shortListNearBottom.component.up, false)
})

test('opening near the bottom scrolls to the chosen option only after placing the list up', () => {
  const low = select({ labels: ['Shared', 'For each person'], selectedIndex: 1, trigger: { top: 760, bottom: 804 }, listHeight: 96 })
  low.component.show()
  assert.equal(low.component.up, true)
  assert.deepEqual(low.scrolled, [], 'scrolling waits until Alpine has applied the placement')
  low.flush()
  assert.deepEqual(low.scrolled, [{ option: 'For each person', up: true }])
})
