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
  vm.runInNewContext(source, { document, Alpine })
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
