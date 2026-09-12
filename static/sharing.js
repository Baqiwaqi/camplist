// Native invitation sharing for the sharing page. Registered before Alpine
// starts so the view can progressively reveal the system share picker.
document.addEventListener('alpine:init', () => {
  Alpine.data('invitationShare', () => ({
    copied: false,
    supported: false,
    shareError: false,
    shareData: null,
    init() {
      this.shareData = {
        title: 'Camplist invitation',
        text: 'Join my Camplist. Sign in and request access; I will approve your account.',
        url: this.$refs.invitationLink.value,
      }
      if (typeof navigator.share !== 'function') return

      try {
        this.supported = typeof navigator.canShare !== 'function' || navigator.canShare(this.shareData)
      } catch {
        this.supported = false
      }
    },
    async shareInvitation() {
      this.shareError = false
      try {
        await navigator.share(this.shareData)
      } catch (error) {
        if (error?.name !== 'AbortError') this.shareError = true
      }
    },
  }))
})
