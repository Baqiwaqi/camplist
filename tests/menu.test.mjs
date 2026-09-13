import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../static/menu.js', import.meta.url), 'utf8')

function openMenu({ trigger, popupWidth, viewport }) {
  let init
  let factory
  const document = {
    documentElement: { clientWidth: viewport },
    addEventListener(event, handler) {
      if (event === 'alpine:init') init = handler
    },
  }
  const Alpine = {
    data(name, build) {
      if (name === 'menu') factory = build
    },
  }
  vm.runInNewContext(source, { document, Alpine, queueMicrotask: callback => callback() })
  init()
  const menu = factory()
  menu.$refs = {
    trigger: { getBoundingClientRect: () => trigger, focus() {} },
    popup: { offsetWidth: popupWidth },
  }
  menu.$nextTick = callback => callback()
  menu.show()
  return menu
}

test('a phone card trigger near the left edge opens the popup rightward', () => {
  const menu = openMenu({ trigger: { left: 100, right: 170 }, popupWidth: 176, viewport: 360 })
  assert.equal(menu.open, true)
  assert.equal(menu.alignStart, true)
})

test('a trigger with room on its left keeps the popup growing left', () => {
  const menu = openMenu({ trigger: { left: 280, right: 350 }, popupWidth: 176, viewport: 360 })
  assert.equal(menu.alignStart, false)
})

test('a popup that just fits left of the trigger does not flip', () => {
  const menu = openMenu({ trigger: { left: 114, right: 184 }, popupWidth: 176, viewport: 360 })
  assert.equal(menu.alignStart, false)
})

test('when neither side fits the popup opens toward the side with more room', () => {
  const moreRoomRight = openMenu({ trigger: { left: 60, right: 130 }, popupWidth: 300, viewport: 320 })
  assert.equal(moreRoomRight.alignStart, true)
  const moreRoomLeft = openMenu({ trigger: { left: 200, right: 270 }, popupWidth: 300, viewport: 320 })
  assert.equal(moreRoomLeft.alignStart, false)
})

// Vertical placement and keyboard use of the Pines-style menu.

function menuWithItems({ trigger = { left: 280, right: 350, top: 100, bottom: 144 }, height = 160, viewportHeight = 844, labels = [] } = {}) {
  let init
  let factory
  const document = {
    activeElement: null,
    documentElement: { clientWidth: 390, clientHeight: viewportHeight },
    addEventListener(event, handler) {
      if (event === 'alpine:init') init = handler
    },
  }
  const Alpine = {
    data(name, build) {
      if (name === 'menu') factory = build
    },
  }
  const microtasks = []
  vm.runInNewContext(source, { document, Alpine, Date, queueMicrotask: callback => microtasks.push(callback) })
  init()
  const menu = factory()
  const items = labels.map(label => ({ textContent: label, disabled: false, focusedUp: null, focus() { this.focusedUp = menu.up; document.activeElement = this } }))
  menu.$refs = {
    trigger: { getBoundingClientRect: () => trigger, focus() { document.activeElement = this } },
    popup: { offsetWidth: 176, offsetHeight: height, querySelectorAll: () => items },
  }
  menu.$nextTick = callback => callback()
  const flush = () => { while (microtasks.length) microtasks.shift()() }
  return { menu, items, document, flush }
}

function key(value) {
  return { key: value, prevented: false, preventDefault() { this.prevented = true } }
}

test('a menu near the bottom of the screen opens upward', () => {
  const { menu } = menuWithItems({ trigger: { left: 280, right: 350, top: 760, bottom: 800 } })
  menu.show()
  assert.equal(menu.up, true)
})

test('a menu with room below opens downward', () => {
  const { menu } = menuWithItems()
  menu.show()
  assert.equal(menu.up, false)
})

test('a keyboard-opened menu near the bottom focuses its item only after placing the popup up', () => {
  const { menu, items, document, flush } = menuWithItems({ trigger: { left: 280, right: 350, top: 760, bottom: 800 }, labels: ['Edit list', 'Delete list'] })
  menu.show(0)
  assert.equal(menu.up, true)
  assert.equal(document.activeElement, null, 'focus waits until Alpine has applied the placement')
  flush()
  assert.equal(document.activeElement, items[0])
  assert.equal(items[0].focusedUp, true)
})

test('typing a letter moves to the next item starting with it', () => {
  const { menu, items, document, flush } = menuWithItems({ labels: ['Edit list', 'Sharing', 'Delete list'] })
  menu.show(0)
  flush()
  assert.equal(document.activeElement, items[0])
  const event = key('d')
  menu.find(event)
  assert.equal(document.activeElement, items[2])
  assert.equal(event.prevented, true)
})

test('space and modified keys are left to the item', () => {
  const { menu, items, document, flush } = menuWithItems({ labels: ['Sharing', 'Sign out'] })
  menu.show(0)
  flush()
  for (const event of [key(' '), { ...key('s'), ctrlKey: true }]) {
    menu.find(event)
    assert.equal(event.prevented, false)
  }
  assert.equal(document.activeElement, items[0])
})
