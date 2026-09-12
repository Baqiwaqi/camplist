// Dropdown behaviour for views.Menu (internal/views/menu.templ): open and
// close, arrow-key movement between items, focus back on the trigger.
// Registered before Alpine starts so every menu shares one definition.
document.addEventListener('alpine:init', () => {
  Alpine.data('menu', () => ({
    open: false,
    toggle() {
      this.open ? this.close() : this.show()
    },
    show(index) {
      this.open = true
      if (index !== undefined) this.$nextTick(() => this.focus(index))
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
  }))
})
