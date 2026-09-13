// Listbox behaviour for views.Select (internal/views/select.templ), after the
// Pines UI select: the trigger keeps focus and points at the active option
// with aria-activedescendant. Choosing an option writes it to the hidden
// native <select>, which is what the form submits.
document.addEventListener('alpine:init', () => {
  Alpine.data('select', () => ({
    open: false,
    up: false,
    selected: 0,
    active: -1,
    typed: '',
    typedAt: 0,
    init() {
      this.sync()
      // A form reset (the trip entry form after an add) puts the native
      // select back to its default option; show that choice too.
      this.form = this.$refs.native.form
      this.onReset = () => setTimeout(() => this.sync())
      this.form?.addEventListener('reset', this.onReset)
    },
    destroy() {
      this.form?.removeEventListener('reset', this.onReset)
    },
    sync() {
      this.selected = Math.max(0, this.$refs.native.selectedIndex)
    },
    options() {
      return Array.from(this.$refs.listbox.querySelectorAll('[role="option"]'))
    },
    label() {
      return this.$refs.native.options[this.selected]?.text ?? ''
    },
    toggle() {
      this.open ? this.close() : this.show()
    },
    show(index = this.selected) {
      this.open = true
      this.$nextTick(() => this.place())
      this.move(index)
    },
    // Open upwards when the space below the trigger cannot hold the list but
    // the space above is larger. The list is measured once it is shown.
    place() {
      const gutter = 8
      const box = this.$refs.trigger.getBoundingClientRect()
      const below = document.documentElement.clientHeight - box.bottom
      this.up = this.$refs.listbox.offsetHeight + gutter > below && box.top > below
    },
    close(returnFocus = true) {
      if (!this.open) return
      this.open = false
      this.active = -1
      if (returnFocus) this.$refs.trigger.focus()
    },
    // Tab or a click elsewhere moved focus out of the select.
    leave(event) {
      if (!this.$root.contains(event.relatedTarget)) this.close(false)
    },
    move(index) {
      const count = this.options().length
      this.active = Math.min(Math.max(index, 0), count - 1)
      this.$nextTick(() => queueMicrotask(() => this.options()[this.active]?.scrollIntoView({ block: 'nearest' })))
    },
    choose(index) {
      const native = this.$refs.native
      this.selected = index
      if (native.selectedIndex !== index) {
        native.selectedIndex = index
        native.dispatchEvent(new Event('change', { bubbles: true }))
      }
      this.close()
    },
    key(event) {
      if (event.altKey || event.ctrlKey || event.metaKey) return
      const last = this.options().length - 1
      const typing = event.key.length === 1 && (event.key !== ' ' || Date.now() - this.typedAt < 500)
      if (typing) {
        event.preventDefault()
        if (!this.open) this.show()
        this.find(event.key)
        return
      }
      const keys = this.open
        ? { ArrowDown: () => this.move(this.active + 1), ArrowUp: () => this.move(this.active - 1), Home: () => this.move(0), End: () => this.move(last), PageDown: () => this.move(this.active + 10), PageUp: () => this.move(this.active - 10), Enter: () => this.choose(this.active), ' ': () => this.choose(this.active), Escape: () => this.close() }
        : { ArrowDown: () => this.show(), ArrowUp: () => this.show(), Enter: () => this.show(), ' ': () => this.show(), Home: () => this.show(0), End: () => this.show(last) }
      if (event.key === 'Tab') {
        this.close(false)
      } else if (keys[event.key]) {
        event.preventDefault()
        keys[event.key]()
      }
    },
    // Type-ahead: jump to the next option starting with the typed letters.
    find(char) {
      const now = Date.now()
      this.typed = (now - this.typedAt < 500 ? this.typed : '') + char.toLowerCase()
      this.typedAt = now
      const labels = this.options().map(option => option.textContent.trim().toLowerCase())
      const start = this.typed.length === 1 ? this.active + 1 : this.active
      for (let i = 0; i < labels.length; i++) {
        const index = (start + i) % labels.length
        if (labels[index].startsWith(this.typed)) return this.move(index)
      }
    },
  }))
})
