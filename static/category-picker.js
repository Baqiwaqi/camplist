// Combobox behaviour for views.CategoryPicker (internal/views/category-picker.templ),
// shaped like the Pines UI combobox: typing filters the options, arrow keys move
// through them, Enter or a click picks one, and a name that is not in the list
// is offered as a new category. A new name that looks like a typo of an
// existing category (closeCategory, the same rule as packing.CloseCategory)
// offers the existing one first. The text input stays the real form field, so
// picking an option only fills it in. Options are read from the picker's
// datalist each time, which lets the offline scripts add categories to it.
// Remembered custom categories (data-custom) get a small menu: Rename opens
// the layout's category dialog and Remove posts straight away; both answer
// with a categories-changed event that updates every picker on the page.
// Registered before Alpine starts so every picker shares one definition.
const categoryKey = value => value.trim().toLowerCase()

// editDistance counts the single-letter insertions, deletions, substitutions
// and swaps of neighbouring letters that turn a into b.
function editDistance(a, b) {
  const d = Array.from({ length: a.length + 1 }, (_, i) => [i, ...Array(b.length).fill(0)])
  for (let j = 1; j <= b.length; j++) d[0][j] = j
  for (let i = 1; i <= a.length; i++) {
    for (let j = 1; j <= b.length; j++) {
      d[i][j] = Math.min(d[i - 1][j] + 1, d[i][j - 1] + 1, d[i - 1][j - 1] + (a[i - 1] === b[j - 1] ? 0 : 1))
      if (i > 1 && j > 1 && a[i - 1] === b[j - 2] && a[i - 2] === b[j - 1]) d[i][j] = Math.min(d[i][j], d[i - 2][j - 2] + 1)
    }
  }
  return d[a.length][b.length]
}

// closeCategory returns the option typed is most likely a typo of, or
// undefined: at least four letters, not already an option, and one edit away
// (two when both names have eight letters or more).
function closeCategory(typed, options) {
  const typedKey = Array.from(categoryKey(typed))
  if (typedKey.length < 4 || options.some(option => categoryKey(option) === categoryKey(typed))) return undefined
  let best
  let bestDistance = 3
  for (const option of options) {
    const optionKey = Array.from(categoryKey(option))
    const allowed = typedKey.length >= 8 && optionKey.length >= 8 ? 2 : 1
    const distance = editDistance(typedKey, optionKey)
    if (distance <= allowed && distance < bestDistance) {
      best = option
      bestDistance = distance
    }
  }
  return best
}

// A rename or removal answers with HX-Trigger categories-changed. Every picker
// on the page drops the old option, a rename adds the new name unless it is
// already offered, and a field holding the old name takes the new one. The
// result message goes to the layout's status toast.
document.addEventListener('categories-changed', event => {
  const { from, to, custom, message } = event.detail
  for (const datalist of document.querySelectorAll('datalist')) {
    for (const option of Array.from(datalist.options)) {
      if (categoryKey(option.value) === categoryKey(from)) option.remove()
    }
    if (!to) continue
    const same = Array.from(datalist.options).find(option => categoryKey(option.value) === categoryKey(to))
    if (same) {
      if (custom) same.dataset.custom = ''
      continue
    }
    const option = document.createElement('option')
    option.value = to
    if (custom) option.dataset.custom = ''
    else option.dataset.default = ''
    datalist.append(option)
  }
  if (to) {
    for (const input of document.querySelectorAll('input[role="combobox"][name="category"]')) {
      if (categoryKey(input.value) === categoryKey(from)) input.value = to
    }
  }
  window.dispatchEvent(new CustomEvent('camplist-status', { detail: { message } }))
})

document.addEventListener('alpine:init', () => {
  const key = categoryKey

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
    // The category whose rename/remove menu is open.
    managing: null,
    options() {
      return Array.from(this.$root.querySelector('datalist').options, option => ({ value: option.value, custom: 'custom' in option.dataset }))
    },
    // matches lists what the popup shows for the text typed so far, ending
    // with the typed name itself when it is new. A new name close to an
    // existing category starts with that category and the new name instead.
    // It reads the datalist again whenever the popup opens.
    matches() {
      if (!this.open) return []
      const typed = this.filtering ? this.query.trim() : ''
      const options = this.options()
      const found = options.filter(option => key(option.value).includes(key(typed)))
      const items = found.map(option => ({ ...option, label: option.value, kind: 'option' }))
      if (!typed || options.some(option => key(option.value) === key(typed))) return items
      const close = closeCategory(typed, options.map(option => option.value))
      const create = { value: typed, label: `Create “${typed}”`, kind: 'create', custom: false }
      if (close === undefined) return [...items, create]
      const suggestion = { value: close, label: `Use “${close}”?`, kind: 'suggestion', custom: false }
      return [suggestion, create, ...items.filter(item => item.value !== close)]
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
      this.managing = null
    },
    manage(value) {
      this.managing = this.managing === value ? null : value
      if (this.managing === null) this.$refs.input.focus()
    },
    rename(value) {
      this.close()
      window.dispatchEvent(new CustomEvent('category-rename', { detail: { name: value, input: this.$refs.input } }))
    },
    remove(value) {
      this.close()
      this.$refs.input.focus()
      window.dispatchEvent(new CustomEvent('category-remove', { detail: { name: value } }))
    },
    filter() {
      this.managing = null
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

  // The rename form in views.CategoryDialog, and the remove form beside it.
  // A failure stays inside the dialog because the modal covers the error
  // toast; closing it puts focus back in the picker that opened it.
  Alpine.data('categoryDialog', () => ({
    from: '',
    to: '',
    error: '',
    returnTo: null,
    open({ name, input }) {
      this.from = name
      this.to = name
      this.error = ''
      this.returnTo = input || null
      this.$root.showModal()
      this.$nextTick(() => this.$refs.name.select())
    },
    closed() {
      this.returnTo?.focus()
      this.returnTo = null
    },
    // note says what the typed name does: nothing yet, or join a category
    // the picker already offers.
    note() {
      const to = this.to.trim()
      if (!to || categoryKey(to) === categoryKey(this.from)) return ''
      const existing = Array.from(document.querySelectorAll('datalist option'), option => option.value).find(value => categoryKey(value) === categoryKey(to))
      return existing === undefined ? '' : `Items join the existing category “${existing}”.`
    },
    failed(xhr) {
      const plain = (xhr.getResponseHeader('Content-Type') || '').startsWith('text/plain')
      const text = (xhr.responseText || '').trim()
      this.error = xhr.status < 500 && plain && text ? text : 'The category did not rename. Try again or reload the page.'
    },
    lost() {
      this.error = 'Connection lost. Reconnect and try again.'
    },
    remove(name) {
      this.$refs.removeName.value = name
      this.$refs.removeForm.requestSubmit()
    },
  }))
})
