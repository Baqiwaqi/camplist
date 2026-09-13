// Combobox behaviour for views.CategoryPicker (internal/views/category-picker.templ),
// shaped like the Pines UI combobox: typing filters the options, arrow keys move
// through them, Enter or a click picks one, and a name that is not in the list
// is offered as a new category. The text input stays the real form field, so
// picking an option only fills it in. Options are read from the picker's
// datalist each time, which lets the offline scripts add categories to it.
// Registered before Alpine starts so every picker shares one definition.
document.addEventListener('alpine:init', () => {
  const key = value => value.trim().toLowerCase()

  Alpine.data('categoryPicker', () => ({
    open: false,
    active: -1,
    query: '',
    // Opening the picker shows every option; typing narrows them down.
    filtering: false,
    init() {
      this.detach()
      this.query = this.$refs.input.value
    },
    // The popup replaces the browser's own datalist suggestions. Focus repeats
    // this because an htmx swap can put the rendered list attribute back.
    detach() {
      this.$refs.input.removeAttribute('list')
    },
    options() {
      return Array.from(this.$root.querySelector('datalist').options, option => option.value)
    },
    // matches lists what the popup shows for the text typed so far, ending
    // with the typed name itself when it is new. It reads the datalist again
    // whenever the popup opens.
    matches() {
      if (!this.open) return []
      const typed = this.filtering ? this.query.trim() : ''
      const options = this.options()
      const found = options.filter(option => key(option).includes(key(typed)))
      const items = found.map(value => ({ value, label: value, create: false }))
      if (typed && !options.some(option => key(option) === key(typed))) {
        items.push({ value: typed, label: `New category “${typed}”`, create: true })
      }
      return items
    },
    listboxID() {
      return `${this.$refs.input.id}-listbox`
    },
    optionID(index) {
      return `${this.$refs.input.id}-option-${index}`
    },
    activeID() {
      return this.open && this.active >= 0 ? this.optionID(this.active) : null
    },
    show() {
      if (this.open) return
      this.query = this.$refs.input.value
      this.filtering = false
      this.open = true
    },
    close() {
      this.open = false
      this.active = -1
    },
    filter() {
      this.show()
      this.query = this.$refs.input.value
      this.filtering = true
      this.active = -1
    },
    move(step) {
      if (!this.open) {
        this.show()
        return
      }
      const count = this.matches().length
      if (!count) return
      this.active = this.active === -1 && step < 0 ? count - 1 : (this.active + step + count) % count
      this.$nextTick(() => document.getElementById(this.optionID(this.active))?.scrollIntoView({ block: 'nearest' }))
    },
    pick(item) {
      this.$refs.input.value = item.value
      this.query = item.value
      this.close()
      this.$refs.input.focus()
    },
    // Enter picks the highlighted option instead of submitting the form.
    enter(event) {
      const item = this.open && this.matches()[this.active]
      if (!item) return
      event.preventDefault()
      this.pick(item)
    },
    escape(event) {
      if (!this.open) return
      event.preventDefault()
      event.stopPropagation()
      this.close()
    },
    // Focus left the picker: close it, and adopt a default category's spelling
    // when the typed name matches one apart from case and spaces. A custom
    // category keeps the typed spelling, so a camper can recase it.
    leave(event) {
      if (this.$root.contains(event.relatedTarget)) return
      this.close()
      const input = this.$refs.input
      const defaults = Array.from(this.$root.querySelectorAll('datalist option[data-default]'), option => option.value)
      const same = defaults.find(option => key(option) === key(input.value))
      if (same !== undefined) input.value = same
      this.query = input.value
    },
  }))
})
