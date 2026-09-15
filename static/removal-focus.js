// Deleting a row with hx-swap="delete" removes the button that had focus.
// Just before htmx removes the row, focus the first control of the next row in
// the list, or the previous one, or the field named by the list's
// data-empty-focus once the last row is gone, so keyboard and screen reader
// users stay in the list. Rows may sit in groups, so neighbours are looked up
// across the whole list. A control with an id keeps focus when the response
// swaps the list again, because htmx refocuses the element with that id.
document.addEventListener('htmx:beforeSwap', event => {
  const { target, shouldSwap } = event.detail
  const focused = document.activeElement
  const list = target.parentElement?.closest('[data-empty-focus]')
  if (!shouldSwap || !list || !target.contains(focused) || focused.getAttribute('hx-swap') !== 'delete') return
  // An open edit row holds popups of its own li options; skip those.
  const rows = Array.from(list.querySelectorAll('li')).filter(row => !row.parentElement.closest('li'))
  const index = rows.indexOf(target)
  const sibling = rows[index + 1] || rows[index - 1]
  const next = sibling?.querySelector('a[href], button, input:not([type=hidden])') || document.getElementById(list.dataset.emptyFocus)
  next?.focus()
})

// htmx focuses the first [autofocus] in content it swaps in, and that includes
// rows the list page's gear keeps with hx-preserve. Once swapped content has
// been focused its autofocus is spent, so a kept row cannot take focus again.
document.addEventListener('htmx:load', event => {
  if (event.target === document.body) return
  for (const el of event.target.querySelectorAll('[autofocus]')) el.removeAttribute('autofocus')
  if (event.target.hasAttribute?.('autofocus')) event.target.removeAttribute('autofocus')
})
