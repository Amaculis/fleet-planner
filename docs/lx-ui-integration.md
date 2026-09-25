# lx-ui integration

The app's frontend is server-rendered `templ` + `htmx` (see `CLAUDE.md`, `stack-conventions`
skill). This is the deliberate exception: a set of individual UI controls —
[`@dativa-lv/lx-ui`](https://github.com/dativa-lv/lx-ui) components — mounted as isolated
Vue 3 "islands" inside otherwise static, server-rendered pages. The same pattern Astro
calls islands: small, self-contained interactive widgets, not the page (or the app)
becoming a single-page application.

## Why islands, not a wider adoption

`lx-ui` is a full Vue 3 component library with 40+ dependencies (Tiptap, Leaflet, PDF
handling, QR codes, its own auth/store layer). Adopting it as the app's whole frontend
would mean rebuilding the server-rendered auth/CSRF/RBAC pipeline as an SPA-over-JSON-API
instead — a different architecture, not a styling upgrade. See the conversation history
for the fuller tradeoff discussion; this file covers the integration actually built.

## What's enhanced, and what deliberately isn't

**Field islands** (`data-kind` on a `[data-lx-field]` mount point):

| kind | component | used on |
|---|---|---|
| `calendar` | `LxDateTimePicker` | Timeline's day picker |
| `text` | `LxTextInput` | bus plate/model, driver name/phone/licence, trip origin/destination, user email |
| `toggle` | `LxToggle` | driver `is_active` |
| `select` | `LxValuePicker` (dropdown variant) | bus status, driver pay type, user role, user's linked driver |
| `toolbar` | `LxToolbar` | the nav bar, every page |
| `button` | `LxButton` | the primary Save button on bus/driver/trip/user forms |
| `notification` | `LxInfoBox` | the success/error flash banner |

**Deliberately excluded, and why:**
- **Passwords** (field and the password-form Save button) — masking/autofill/password-
  manager behaviour is exactly what this app cannot afford to get subtly wrong, and it
  was never verified against lx-ui's actual behaviour the way everything else here was.
- **Per-row table buttons** (Edit/Delete/Deactivate, repeated per row) — mounting a
  separate Vue app per row would mean dozens of `createApp()` calls on one page for no
  real benefit.
- **Tables themselves (`LxDataGrid`)** — would need row data fed as JSON rather than
  server-rendered `<tr>`s, a real architecture question of its own, not attempted here.
- **The driver mobile view's start/finish buttons, and the login button** — kept
  maximally simple and reliable: a driver's phone connectivity and the auth flow are the
  two places in this app where "always just works" matters most.

## How it works

- `web/vue/main.js` is the only Vue entry point in the app. `FIELD_KINDS` maps each
  `data-kind` to a component and how to build its props from the mount element's
  `data-*` attributes; `mountFieldIslands()` finds every `[data-lx-field]` element and
  mounts the right one.
- **Progressive enhancement is load-bearing, not decorative, for every one of these.**
  Each mount point sits beside a real, complete native element (an `<input>`, a
  `<select>`, the real `<nav>`, the real submit `<button>`). The mount `<div>` starts
  `hidden`; only on a *successful* mount does `main.js` hide the native element and
  reveal the island. If the script fails to load, throws, or JS is off, the native
  element — already fully wired to the server, already tested independently — is what
  the user interacts with. Nothing about a page's actual function depends on any island
  succeeding.
- Each field kind either **syncs its value back** into the native element (`text`,
  `toggle`, `select` — so the surrounding `<form>` still submits the same way
  regardless of enhancement) or **acts directly** (`calendar` navigates via `?date=`;
  `button` calls `.click()` on the real button it replaces, triggering the same native
  form submission a direct click would; `toolbar` submits the real logout form or
  navigates to a plain URL; `notification` has no interaction at all).
- `createLx()` is registered per mount (locale, first-day-of-week) per lx-ui's
  documented setup (`docs/CreateLx.md` in their repo), matching the page's own locale.
  `preload.components` (`docs/ComponentPreload.md`) is set explicitly — see below.

## Bugs found while building this — read before adding another component

Two real, separate runtime bugs surfaced building this, both producing the identical
symptom — `TypeError: <mangled name> is not a function`, thrown synchronously inside
`app.use(createLx, ...)`, on *every* field kind, no build-time warning, nothing in the
CSP or Network tab to point at it. Fixing the first one did not fix the second; they
just happened to fail the same way. **The lesson under both:** this library's own
README example code was wrong or misleading in two different places, and trusting a
paraphrased fetch of it (rather than the actual source) is what let both ship.

**1. `createLx` is a plugin object, not a factory function.** Its real definition
(`src/lib.js`): `const plugin = { install }; export const createLx = plugin;`. Vue's
plugin signature is `app.use(plugin, ...options)` — call it as `app.use(createLx,
options)`. This project originally wrote `app.use(createLx(options))`, calling the
plugin object as if it were a function, which throws exactly "createLx is not a
function" (mangled to an opaque name once minified). The library's own README shows
`myApp.use(createLx())` — called with parens — which is the wrong form; whether that's
a documentation bug in their repo or an artifact of how it got paraphrased when first
read here, verbatim source is what settled it, not the doc.

**2. Do not add a Pinia or `@vueuse/core` dependency for lx-ui.** Both were installed
here at first, again following the README's setup code. `lx-ui`'s own `package.json`
lists both as plain `dependencies`, not `peerDependencies`; only `vue` is a peer
dependency. A regular dependency means the library bundles and uses its own nested copy
(`node_modules/@dativa-lv/lx-ui/node_modules/pinia`, pinned to `^2.1.7`) — self-
sufficient, no need for the host app to provide one. A separate top-level Pinia (this
project initially picked `4.0.3`) put two different major versions of Pinia in one
bundle — a real bug on its own, independent of bug #1 above, just discovered at the
same time and initially (wrongly) assumed to be the whole story. Removed from
`package.json` and from `main.js`'s `app.use()` calls.

The general lesson for adding another component: **verify against the actual source in
`github.com/dativa-lv/lx-ui/blob/main/src/...`, quoted verbatim, not a summary of it** —
this file's own export names, prop names, and event names were each gotten wrong at
least once from a paraphrased read before being corrected this way. For a new
dependency question specifically: check whether `lx-ui` lists it under
`peerDependencies` (host must provide it) or plain `dependencies` (lx-ui provides its
own — installing your own is optional at best, wrong at worst). `npm ls <package>
--all` or reading `node_modules/@dativa-lv/lx-ui/package.json` directly settles it.

**Also worth knowing:** `install()` guards itself with a module-level singleton flag
(`if (install.installed) return`) — it only actually runs once per page load, not once
per island, even though each island is its own independent `createApp()` instance.
Whichever island mounts first "wins" and its `lxConfig()` options are what apply
page-wide; every other island's `app.use(createLx, ...)` call silently no-ops. This is
harmless here only because every call site builds the same locale and the same full
`PRELOAD_COMPONENTS` list regardless of which kind is mounting — if a future change
makes that config vary per call site, only the first-mounted island's version would
ever take effect.

**3. Nothing renders correctly without `class="lx"` on an ancestor element.** Every
lx-ui design token — icon sizes, colours, spacing, fonts, 300+ CSS custom properties —
is defined under one bare selector in their CSS bundle: `.lx { --icon-size-m: 1.5rem;
... }`. With no element carrying that class, none of those variables exist anywhere in
the page's cascade, and every property that reads one of them (`width:
var(--icon-size-m)`, etc.) resolves to nothing. The visible symptom was one icon
rendering at an enormous, unconstrained size — but every other island was silently
affected the same way, just without anything as dramatic to notice. Fixed by adding
`class="lx"` to `#main` in `Layout()` — the one ancestor common to every island on every
page (see `Nav()` and `Page()`).

**4. lx-ui's pre-compiled dist bundle assumes it is deployed at the domain root.**
Chunk imports, `@font-face url()`s — anything the compiled code fetches at runtime is
requested as `new URL(path, window.location.origin)` or an equivalent root-relative
path (`/js/...`, `/css/...`, `/lx-fonts/...`), never relative to where the entry script
itself was actually loaded from. This app serves everything under `/static/`, so every
one of those requests 404'd. This logic is compiled into the dependency — nothing in
this project's own Vite config affects it. Fixed two ways, since it wasn't possible to
verify from source alone which one lx-ui's runtime actually honours: `createLx()`'s
`publicUrl` option is set (their documented field for exactly this), **and**
`internal/http/server.go` mirrors the same embedded static tree at root-level `/js/*`,
`/css/*`, `/lx-fonts/*` routes (see the comment there) — whichever mechanism is real,
the files exist where the compiled code looks for them either way.

**On documentation reliability, generally:** none of bugs #1–#4 are documented for a
consumer building outside lx-ui's own tooling. Their `DEVELOPMENT.md` covers only
contributing to the library itself (local setup, `bun link`, security scanning) — there
is no guide for deploying the built output into an existing app. The library reads as
built to be used through lx-ui's own project scaffolding, which would presumably set
the base path, copy the fonts, and add the `.lx` class automatically; none of that
exists for someone integrating a single component by hand, which is the position this
project is in. Budget real time for this kind of archaeology if extending the
integration further, and see "verify against actual source" above — it is what found
every one of these.

## Other known, accepted costs

- **The build emits a long tail of lazily-loaded chunks** (Leaflet, a PDF worker, QR/
  crypto code, other companies' logo assets bundled into the package itself — Swedbank,
  eID, Digimaks) that this app's usage never triggers. `import()` code-splitting means
  browsers should never fetch them in normal use, but they exist as static files on the
  server (~1,700 files, ~22MB as of the current component set). They were **not**
  pruned from the build output: doing so safely would need verifying in a real browser
  that no code path reaches them, which this environment cannot do (no headless browser
  available). If you're reading this because `/static/js/calendar-assets/…` came up
  unexpectedly, that's why — pruning is a reasonable follow-up once verified.
- **`npm audit` on this dependency set reports vulnerabilities** entirely in transitive
  lx-ui dependencies (Tiptap, ExifReader) unrelated to anything this app actually uses.
  None of `govulncheck`'s Go-side scanning covers this — `npm audit` needs to be run and
  reviewed separately whenever `package.json` changes, and is now part of this
  project's effective security-review checklist for that reason.
- **No headless browser is available in this development environment.** Every claim
  above about "the island mounts / falls back correctly" is verified at the HTTP/HTML
  level (the server sends the right markup, the right assets, with the right status
  codes) and by reading the component source directly — never by actually watching Vue
  execute in a real browser. Real browser testing (console errors, the Network tab,
  visual comparison) has to happen by hand, in an actual browser, by whoever is running
  the app locally.

## If you want to enhance another field or component

The pattern generalizes: add another `[data-lx-field]` mount point with a `data-kind`
matching an entry in `FIELD_KINDS`, or add a new kind there if it's a genuinely new
component. Before doing either, check the component's actual prop names, event names,
and `modelValue` type/format against its source on GitHub — every field kind in the
table above was built that way after getting at least one of those wrong from a docs
summary alone. And check its dependency list per the Pinia lesson above.
