// Error toast behaviour for the signed-in layout (internal/views/layout.templ).
// Handlers answer failed htmx requests with a short plain-text reason
// (http.Error); show that reason instead of guessing from the status code.
// A toast the user only needs to read hides itself after a few seconds; one
// offering Reload, and the connection-lost toast, stay until the user acts.
// Registered before Alpine starts.
document.addEventListener('alpine:init', () => {
  const fallback = 'That change did not save. Try again or reload to see the latest saved state.'
  const autoDismissMs = 6000
  const leaveMs = 160

  Alpine.data('requestError', () => ({
    message: '',
    reload: false,
    sticky: false,
    leaving: false,
    hovered: false,
    focused: false,
    timer: null,
    leaveTimer: null,
    reset() {
      this.stop()
      this.hide()
      this.reload = false
    },
    failed(xhr) {
      const plain = (xhr.getResponseHeader('Content-Type') || '').startsWith('text/plain')
      const text = (xhr.responseText || '').trim()
      this.message = xhr.status < 500 && plain && text ? text : fallback
      this.reload = xhr.getResponseHeader('X-Camplist-Error') === 'csrf' || xhr.status === 409
      this.show(this.reload)
    },
    skipped() {
      this.message = 'A change could not be saved because the list changed before it was sent. Try again.'
      this.reload = false
      this.show(false)
    },
    lost() {
      this.message = 'Connection lost. Reconnect and reload to check the latest saved state.'
      this.reload = false
      this.show(true)
    },
    show(sticky) {
      this.stop()
      this.sticky = sticky
      this.leaving = false
      this.$root.hidden = false
      this.schedule()
    },
    // Hovering or focusing the toast holds it open so it can be read.
    hold(kind, active) {
      this[kind] = active
      this.schedule()
    },
    schedule() {
      clearTimeout(this.timer)
      this.timer = null
      if (this.sticky || this.hovered || this.focused || this.$root.hidden) return
      this.timer = setTimeout(() => this.dismiss(), autoDismissMs)
    },
    stop() {
      clearTimeout(this.timer)
      clearTimeout(this.leaveTimer)
      this.timer = null
      this.leaveTimer = null
    },
    dismiss() {
      this.stop()
      this.leaving = true
      this.leaveTimer = setTimeout(() => this.hide(), leaveMs)
    },
    hide() {
      this.$root.hidden = true
      this.leaving = false
      this.hovered = false
      this.focused = false
    },
  }))
})
