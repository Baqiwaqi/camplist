export function invitationShareData(url) {
  return {
    title: 'Camplist invitation',
    text: 'Join my Camplist. Sign in and request access; I will approve your account.',
    url,
  }
}

export function canShareInvitation(browser, data) {
  if (typeof browser.share !== 'function') return false
  if (typeof browser.canShare !== 'function') return true

  try {
    return browser.canShare(data)
  } catch {
    return false
  }
}

export async function shareInvitation(browser, data) {
  try {
    await browser.share(data)
    return 'shared'
  } catch (error) {
    if (error?.name === 'AbortError') return 'cancelled'
    throw error
  }
}

export function enhanceInvitationSharing(root, browser) {
  const input = root.querySelector('[data-invitation-link]')
  const button = root.querySelector('[data-share-invitation]')
  const errorMessage = root.querySelector('[data-share-error]')
  const data = invitationShareData(input.value)

  if (!canShareInvitation(browser, data)) return

  button.hidden = false
  button.addEventListener('click', async () => {
    errorMessage.hidden = true
    try {
      await shareInvitation(browser, data)
    } catch {
      errorMessage.hidden = false
    }
  })
}

if (typeof document !== 'undefined') {
  for (const root of document.querySelectorAll('[data-invitation-share]')) {
    enhanceInvitationSharing(root, navigator)
  }
}
