// Keep failed requests visible while leaving the last saved checklist intact.
function showRequestError(message) {
  const banner = document.getElementById("request-error");
  const text = document.getElementById("request-error-message");
  if (!banner || !text) return;
  text.textContent = message;
  banner.hidden = false;
}

document.addEventListener("htmx:beforeRequest", () => {
  const banner = document.getElementById("request-error");
  if (banner) banner.hidden = true;
});
document.addEventListener("htmx:responseError", (event) => {
  const status = event.detail.xhr.status;
  if (status === 409) {
    showRequestError("This list changed while you were working. Reload it before trying again.");
    return;
  }
  showRequestError(status === 403
    ? "Your session may have expired. Reload the page and try again."
    : "We couldn't save that change. Please try again or reload to see the latest saved state.");
});
document.addEventListener("htmx:sendError", () => {
  showRequestError("Connection lost. Reconnect and reload to check the latest saved state.");
});
