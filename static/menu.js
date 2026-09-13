// Dropdown behaviour for views.Menu (internal/views/menu.templ), after the
// Pines UI dropdown menu: open and close, arrow-key and type-ahead movement
// between items, focus back on the trigger, and which side of the trigger the
// popup opens toward so it stays on screen.
// Registered before Alpine starts so every menu shares one definition.
document.addEventListener('alpine:init', () => {
  Alpine.data('menu', () => ({
    open: false,
    // The popup lines up with the trigger's right edge and grows left. A
    // trigger near the left edge (a phone card row) flips it to grow right.
    alignStart: false,
    // The popup opens below the trigger unless only the space above fits it.
    up: false,
    typed: '',
    typedAt: 0,
    toggle() {
      this.open ? this.close() : this.show()
    },
    show(index) {
      this.open = true
      this.$nextTick(() => {
        this.place()
        if (index !== undefined) this.focus(index)
      })
    },
    place() {
      const gutter = 8
      const width = this.$refs.popup.offsetWidth
      const trigger = this.$refs.trigger.getBoundingClientRect()
      const viewport = document.documentElement.clientWidth
      const overflowsLeft = trigger.right - width < gutter
      const fitsRight = trigger.left + width <= viewport - gutter
      this.alignStart = overflowsLeft && (fitsRight || trigger.left < viewport - trigger.right)
      const height = this.$refs.popup.offsetHeight
      const below = document.documentElement.clientHeight - trigger.bottom
      this.up = height + gutter > below && trigger.top > below
    },
    close(returnFocus = true) {
      if (!this.open) return
      this.open = false
      if (returnFocus) this.$refs.trigger.focus()
    },
    // Tab or a click elsewhere moved focus out of the menu.
    leave(event) {
      if (!this.$root.contains(event.relatedTarget)) this.close(false)
    },
    items() {
      return Array.from(this.$refs.popup.querySelectorAll('[role="menuitem"]')).filter(item => !item.disabled)
    },
    focus(index) {
      const items = this.items()
      if (items.length) items[(index + items.length) % items.length].focus()
    },
    move(step) {
      const current = this.items().indexOf(document.activeElement)
      this.focus(current === -1 && step < 0 ? -1 : current + step)
    },
    // Type-ahead: a letter moves to the next item whose label starts with the
    // letters typed in the last half second.
    find(event) {
      if (event.key.length !== 1 || event.key === ' ' || event.altKey || event.ctrlKey || event.metaKey) return
      const now = Date.now()
      this.typed = (now - this.typedAt < 500 ? this.typed : '') + event.key.toLowerCase()
      this.typedAt = now
      const items = this.items()
      const labels = items.map(item => item.textContent.trim().toLowerCase())
      const current = items.indexOf(document.activeElement)
      const start = this.typed.length === 1 ? current + 1 : Math.max(current, 0)
      for (let i = 0; i < items.length; i++) {
        const index = (start + i) % items.length
        if (labels[index].startsWith(this.typed)) {
          event.preventDefault()
          return items[index].focus()
        }
      }
    },
  }))
})
