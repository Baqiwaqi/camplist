// Archive, restore and delete on a trip card remove the card with an htmx
// delete swap (views.removesTripCard). Focus was on the removed card's menu,
// so move it to the next card's heading link, the previous one, or the empty
// state the response revealed.
document.addEventListener('htmx:beforeSwap', event => {
  const card = event.detail.target
  if (!event.detail.shouldSwap || !card.matches('[data-saved-trip]')) return
  const cards = Array.from(document.querySelectorAll('[data-saved-trip]'))
  const index = cards.indexOf(card)
  const neighbour = cards[index + 1] || cards[index - 1]
  // The swap, including the out-of-band empty state, runs right after this.
  setTimeout(() => {
    const empty = document.getElementById('trips-empty')
    const target = neighbour ? neighbour.querySelector('h2 a, a') : empty && !empty.hidden ? empty : null
    if (target) target.focus()
  })
})
