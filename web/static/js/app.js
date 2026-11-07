// Minimal progressive enhancement. No inline handlers anywhere: the CSP is
// nonce-based, so behaviour attaches here, from a file served by this origin.
(function () {
  "use strict";

  // Autofocus the first empty field of the sign-in form without an inline attribute.
  var form = document.querySelector('form[action="/login"]');
  if (form) {
    var email = form.querySelector('input[name="email"]');
    if (email && !email.value) {
      email.focus();
    }
  }

  // Forms marked data-confirm ask before submitting (deletes, erasure, cancelling a
  // trip, a driver's start/finish tap). An empty value means "no question".
  document.addEventListener("submit", function (event) {
    var message = event.target && event.target.getAttribute && event.target.getAttribute("data-confirm");
    if (message && !window.confirm(message)) {
      event.preventDefault();
    }
  }, true);

  // Register the service worker that backs the driver PWA. Browsers only allow this
  // over HTTPS (or localhost), which is exactly where the app runs.
  if ("serviceWorker" in navigator) {
    window.addEventListener("load", function () {
      navigator.serviceWorker.register("/sw.js").catch(function () {
        // An unavailable service worker must never break the page: the app works
        // perfectly well without it, just without the offline shell.
      });
    });
  }
})();
