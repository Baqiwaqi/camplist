import { test } from 'node:test'
import assert from 'node:assert/strict'

import {
  canShareInvitation,
  enhanceInvitationSharing,
  invitationShareData,
  shareInvitation,
} from '../static/sharing.mjs'

const invitationURL = 'https://camplist.example/join/packing-list/list/owner/secret'

test('shares the existing invitation through the browser share picker', async () => {
  const calls = []
  const browser = {
    canShare: data => data.url === invitationURL,
    share: async data => calls.push(data),
  }

  const data = invitationShareData(invitationURL)

  assert.equal(canShareInvitation(browser, data), true)
  assert.equal(await shareInvitation(browser, data), 'shared')
  assert.deepEqual(calls, [
    {
      title: 'Camplist invitation',
      text: 'Join my Camplist. Sign in and request access; I will approve your account.',
      url: invitationURL,
    },
  ])
})

test('does not offer browser sharing when the payload is unsupported', () => {
  const data = invitationShareData(invitationURL)

  assert.equal(canShareInvitation({}, data), false)
  assert.equal(canShareInvitation({ share() {}, canShare: () => false }, data), false)
})

test('treats dismissing the share picker as a normal cancellation', async () => {
  const abort = new Error('user dismissed picker')
  abort.name = 'AbortError'
  const browser = { share: async () => Promise.reject(abort) }

  assert.equal(await shareInvitation(browser, invitationShareData(invitationURL)), 'cancelled')
})

test('progressively reveals sharing and reports launch failures without removing copy fallback', async () => {
  let click
  const input = { value: invitationURL }
  const button = {
    hidden: true,
    addEventListener: (_event, handler) => { click = handler },
  }
  const errorMessage = { hidden: true }
  const copyButton = { hidden: false }
  const elements = new Map([
    ['[data-invitation-link]', input],
    ['[data-share-invitation]', button],
    ['[data-share-error]', errorMessage],
    ['[data-copy-invitation]', copyButton],
  ])
  const root = { querySelector: selector => elements.get(selector) }
  const browser = { share: async () => { throw new Error('share unavailable') } }

  enhanceInvitationSharing(root, browser)
  assert.equal(button.hidden, false)
  assert.equal(copyButton.hidden, false)

  await click()
  assert.equal(errorMessage.hidden, false)
})
