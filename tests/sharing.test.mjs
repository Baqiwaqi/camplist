import { test } from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import vm from 'node:vm'

const invitationURL = 'https://camplist.example/join/packing-list/list/owner/secret'
const source = await readFile(new URL('../static/sharing.js', import.meta.url), 'utf8')

function invitationShare(browser) {
  let alpineInit
  let factory
  const context = {
    document: {
      addEventListener(event, handler) {
        if (event === 'alpine:init') alpineInit = handler
      },
    },
    navigator: browser,
    Alpine: {
      data(name, componentFactory) {
        if (name === 'invitationShare') factory = componentFactory
      },
    },
  }
  vm.runInNewContext(source, context)
  alpineInit()
  const component = factory()
  component.$refs = { invitationLink: { value: invitationURL } }
  return component
}

test('shares the existing invitation through the browser share picker', async () => {
  const calls = []
  const browser = {
    canShare: data => data.url === invitationURL,
    share: async data => calls.push(data),
  }
  const sharing = invitationShare(browser)

  sharing.init()
  await sharing.shareInvitation()

  assert.equal(sharing.supported, true)
  assert.deepEqual(structuredClone(calls), [
    {
      title: 'Camplist invitation',
      text: 'Join my Camplist. Sign in and request access; I will approve your account.',
      url: invitationURL,
    },
  ])
})

test('does not offer browser sharing when the API or payload is unsupported', () => {
  const missing = invitationShare({})
  missing.init()
  assert.equal(missing.supported, false)

  const rejected = invitationShare({ share() {}, canShare: () => false })
  rejected.init()
  assert.equal(rejected.supported, false)
})

test('treats dismissing the share picker as a normal cancellation', async () => {
  const abort = new Error('user dismissed picker')
  abort.name = 'AbortError'
  const sharing = invitationShare({ share: async () => Promise.reject(abort) })

  sharing.init()
  await sharing.shareInvitation()

  assert.equal(sharing.shareError, false)
})

test('reports a share launch failure while leaving the component usable', async () => {
  const sharing = invitationShare({ share: async () => { throw new Error('share unavailable') } })

  sharing.init()
  await sharing.shareInvitation()

  assert.equal(sharing.supported, true)
  assert.equal(sharing.shareError, true)
})
