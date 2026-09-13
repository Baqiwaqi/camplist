// Error toast behaviour for the signed-in layout (internal/views/layout.templ).
// Handlers answer failed htmx requests with a short plain-text reason
// (http.Error); show that reason instead of guessing from the status code.
// Registered before Alpine starts.
document.addEventListener('alpine:init', () => {
  const fallback = 'That change did not save. Try again or reload to see the latest saved state.'

  Alpine.data('requestError', () => ({
    message: '',
    reload: false,
    leaving: false,
    reset() {
      this.$el.hidden = true
      this.reload = false
    },
    failed(xhr) {
      const plain = (xhr.getResponseHeader('Content-Type') || '').startsWith('text/plain')
      const text = (xhr.responseText || '').trim()
      this.message = xhr.status < 500 && plain && text ? text : fallback
      this.reload = xhr.getResponseHeader('X-Camplist-Error') === 'csrf' || xhr.status === 409
      this.$el.hidden = false
    },
    lost() {
      this.message = 'Connection lost. Reconnect and reload to check the latest saved state.'
      this.reload = false
      this.$el.hidden = false
    },
    dismiss() {
      this.leaving = true
      setTimeout(() => {
        this.$root.hidden = true
        this.leaving = false
      }, 160)
    },
  }))
})
