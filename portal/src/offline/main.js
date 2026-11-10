// Script for offline.html, the page the service worker shows without a network.
import "@dativa-lv/lx-ui/dist/styles/lx-reset.css";
import "@dativa-lv/lx-ui/dist/styles/lx-fonts-carbon.css";
import "@dativa-lv/lx-ui/dist/styles/lx-pt-carbon.css";
import "@dativa-lv/lx-ui/dist/styles/lx-ut-carbon-light.css";
import "./offline.css";
import { pages as en } from "@/locales/en.json";
import { pages as lv } from "@/locales/lv.json";
import { pages as ru } from "@/locales/ru.json";
import { initialLocale } from "@/i18n/locale";

const texts = { en, lv, ru }[initialLocale()] ?? en;
document.documentElement.lang = initialLocale();
document.getElementById("offline-title").textContent = texts.offline.title;
document.getElementById("offline-description").textContent = texts.offline.description;
const retry = document.getElementById("offline-retry");
retry.textContent = texts.offline.retry;

// The worker serves this page in place of the one asked for, so the address is still
// the user's page: reloading it is the retry.
retry.addEventListener("click", () => window.location.reload());

// Reload by itself once the server answers again. The browser's "online" event alone
// isn't enough: it fires as soon as there is a network, internet or not, and may not
// fire at all. version.json is never cached by the worker, so it really asks the server.
const RETRY_EVERY_MS = 5000;
let checking = false;
async function reloadWhenReachable() {
  if (checking) return;
  checking = true;
  try {
    const res = await fetch(`${import.meta.env.BASE_URL}version.json`, { cache: "no-store" });
    if (res.ok) window.location.reload();
  } catch {
    // Still unreachable; the next check tries again.
  } finally {
    checking = false;
  }
}
window.addEventListener("online", reloadWhenReachable);
setInterval(reloadWhenReachable, RETRY_EVERY_MS);
