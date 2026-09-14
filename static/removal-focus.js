// Deleting a row with hx-swap="delete" removes the button that had focus.
// Just before htmx removes the row, focus the same button in the next row, or
// the previous one, or the field named by the list's data-empty-focus once the
// last row is gone, so keyboard and screen reader users stay in the list.
document.addEventListener('htmx:beforeSwap', event => {
  const { target, shouldSwap } = event.detail
  const focused = document.activeElement
  const list = target.parentElement?.closest('[data-empty-focus]')
  if (!shouldSwap || !list || !target.contains(focused) || focused.getAttribute('hx-swap') !== 'delete') return
  const sibling = target.nextElementSibling || target.previousElementSibling
  const next = sibling?.querySelector('[hx-swap="delete"]') || document.getElementById(list.dataset.emptyFocus)
  next?.focus()
})
