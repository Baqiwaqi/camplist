import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const source = await readFile(new URL('../static/request-error.js', import.meta.url), 'utf8')

// toast runs static/request-error.js with a manual clock and returns the
// Alpine component bound to a fake toast element.
function toast() {
  let now = 0
  let nextId = 1
  const timers = new Map()
  let init
  let factory
  const context = {
    document: { addEventListener(event, handler) { if (event === 'alpine:init') init = handler } },
    Alpine: { data(name, make) { if (name === 'requestError') factory = make } },
    setTimeout(fn, ms) { const id = nextId++; timers.set(id, { fn, at: now + ms }); return id },
    clearTimeout(id) { timers.delete(id) },
  }
  vm.runInNewContext(source, context)
  init()
  const component = factory()
  component.$root = { hidden: true }
  const advance = ms => {
    const until = now + ms
    for (;;) {
      const due = [...timers].filter(([, t]) => t.at <= until).sort((a, b) => a[1].at - b[1].at)[0]
      if (!due) break
      timers.delete(due[0])
      now = due[1].at
      due[1].fn()
    }
    now = until
  }
  return { component, advance }
}

function response(status, body, headers = {}) {
  const all = { 'Content-Type': 'text/plain; charset=utf-8', ...headers }
  return { status, responseText: body, getResponseHeader: name => all[name] ?? null }
}

test('a toast without Reload hides itself after about six seconds', () => {
  const { component, advance } = toast()
  component.failed(response(400, 'Trip name must be at most 200 characters\n'))
  assert.equal(component.$root.hidden, false)
  assert.equal(component.message, 'Trip name must be at most 200 characters')
  assert.equal(component.reload, false)

  advance(5900)
  assert.equal(component.$root.hidden, false)
  advance(100)
  assert.equal(component.leaving, true, 'uses the dismiss animation')
  advance(160)
  assert.equal(component.$root.hidden, true)
  assert.equal(component.leaving, false)
})

test('toasts offering Reload and the connection-lost toast stay until the user acts', () => {
  for (const show of [
    c => c.failed(response(409, 'The saved version changed. Review the current state and try again.')),
    c => c.failed(response(403, 'Your session expired. Reload the page and try again.', { 'X-Camplist-Error': 'csrf' })),
    c => c.lost(),
  ]) {
    const { component, advance } = toast()
    show(component)
    advance(60000)
    assert.equal(component.$root.hidden, false)
    component.dismiss()
    advance(160)
    assert.equal(component.$root.hidden, true)
  }
  const { component } = toast()
  component.failed(response(403, 'Your session expired. Reload the page and try again.', { 'X-Camplist-Error': 'csrf' }))
  assert.equal(component.reload, true)
  assert.equal(component.message, 'Your session expired. Reload the page and try again.')
})

test('hovering or focusing the toast pauses the timer', () => {
  const { component, advance } = toast()
  component.failed(response(403, 'Only the owner can do that.'))
  advance(5000)
  component.hold('hovered', true)
  advance(20000)
  assert.equal(component.$root.hidden, false)

  component.hold('focused', true)
  component.hold('hovered', false)
  advance(20000)
  assert.equal(component.$root.hidden, false, 'focus still holds it open')

  component.hold('focused', false)
  advance(5900)
  assert.equal(component.$root.hidden, false, 'the full delay restarts once released')
  advance(260)
  assert.equal(component.$root.hidden, true)
})

test('a new error resets the timer and replaces the message', () => {
  const { component, advance } = toast()
  component.failed(response(403, 'Only the owner can do that.'))
  advance(5000)
  component.failed(response(400, 'Trip name must be at most 200 characters'))
  advance(5000)
  assert.equal(component.$root.hidden, false)
  assert.equal(component.message, 'Trip name must be at most 200 characters')
  advance(1160)
  assert.equal(component.$root.hidden, true)
})

test('an error arriving during the leave animation stays visible', () => {
  const { component, advance } = toast()
  component.failed(response(400, 'First'))
  advance(6000)
  assert.equal(component.leaving, true)
  component.failed(response(409, 'Second'))
  advance(1000)
  assert.equal(component.$root.hidden, false)
  assert.equal(component.leaving, false)
})

test('a server error or non-text body falls back to the generic message and auto-hides', () => {
  const { component, advance } = toast()
  component.failed(response(500, 'panic: boom'))
  assert.match(component.message, /did not save/)
  component.failed({ status: 404, responseText: '<html>', getResponseHeader: name => (name === 'Content-Type' ? 'text/html' : null) })
  assert.match(component.message, /did not save/)
  advance(6160)
  assert.equal(component.$root.hidden, true)
})
