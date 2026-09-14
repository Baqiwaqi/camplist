// Tab behaviour for views.TabGroup (internal/views/components.templ), after
// the Pines UI tabs: a segmented strip whose white marker slides to the
// selected tab, with the arrow-key movement the ARIA tabs pattern expects.
// The strip is hidden until Alpine runs (x-cloak), so a page without scripts
// shows every panel stacked instead of a strip that cannot switch them.
// Registered before Alpine starts so every group shares one definition.
document.addEventListener('alpine:init', () => {
  Alpine.data('tabs', () => ({
    current: 0,
    // The marker's inline style, empty until it has been measured.
    marker: '',
    init() {
      this.$nextTick(() => this.place())
      this.onResize = () => this.place()
      window.addEventListener('resize', this.onResize)
    },
    destroy() {
      window.removeEventListener('resize', this.onResize)
    },
    tabs() {
      return Array.from(this.$refs.list?.querySelectorAll('[role=tab]') || [])
    },
    select(index, focus = false) {
      const tabs = this.tabs()
      if (!tabs.length) return
      this.current = (index + tabs.length) % tabs.length
      this.$nextTick(() => {
        this.place()
        if (focus) tabs[this.current].focus()
      })
    },
    // The marker only appears once it has a measured tab to sit on; until
    // then .tabs-marked is absent and the selected tab carries the pill.
    place() {
      const tab = this.tabs()[this.current]
      if (!tab || !tab.offsetWidth) return
      this.marker = `left:${tab.offsetLeft}px;width:${tab.offsetWidth}px`
      this.$refs.list.classList.add('tabs-marked')
    },
    key(event) {
      const moves = { ArrowRight: 1, ArrowLeft: -1 }
      if (event.key in moves) {
        event.preventDefault()
        this.select(this.current + moves[event.key], true)
        return
      }
      if (event.key === 'Home' || event.key === 'End') {
        event.preventDefault()
        this.select(event.key === 'Home' ? 0 : this.tabs().length - 1, true)
      }
    },
  }))
})
