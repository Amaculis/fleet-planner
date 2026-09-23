---
name: lx-ui-frontend
description: Conventions for Vue 3 SPA frontends that use @dativa-lv/lx-ui (LxShell, LxForm, LxDataGrid, LxButton, LxTextInput, etc.) with Vite, Pinia, Vue Router, vue-i18n, and Axios. Use when scaffolding a new portal/ frontend, adding views/components, configuring i18n, handling auth/route guards, or working with LX UI components. Covers package manager rules (bun for new, pnpm for existing pnpm-lock projects), directory layout, store/service/hook naming, and the LX-UI-only component vocabulary.
---

# LX UI Frontend Framework — Project Structure & Usage Skill

This skill documents how to structure and build a Vue 3 SPA frontend using the **LX UI** component library (`@dativa-lv/lx-ui`), Vite, Pinia, Vue Router, vue-i18n, and Axios. It is derived from a reference portal project and codified as generic guidance for LX UI applications, with package manager rules: use **bun** for new projects, and in existing projects use **pnpm** when `pnpm-lock.yaml` is present.

---

## Table of Contents

1. [Key Rules & Constraints](#key-rules--constraints)
2. [Project Placement](#project-placement)
3. [Initializing a New Project](#initializing-a-new-project)
4. [Existing Project Package Manager Detection](#existing-project-package-manager-detection)
5. [Directory Layout](#directory-layout)
6. [Entry Point — index.html](#entry-point--indexhtml)
7. [Application Bootstrap — main.js](#application-bootstrap--mainjs)
8. [App.vue — Root Component](#appvue--root-component)
9. [Vite Configuration](#vite-configuration)
10. [Router Setup](#router-setup)
11. [Layouts — LxShell](#layouts--lxshell)
12. [Views (Pages)](#views-pages)
13. [Components](#components)
14. [Stores (Pinia)](#stores-pinia)
15. [Services (API layer)](#services-api-layer)
16. [API Clients (Axios)](#api-clients-axios)
17. [Hooks (Composables)](#hooks-composables)
18. [Internationalization (i18n)](#internationalization-i18n)
19. [Constants](#constants)
20. [Styles & Theming](#styles--theming)
21. [Error Handling Patterns](#error-handling-patterns)
22. [Authentication Flow](#authentication-flow)
23. [Authorization & Route Guards](#authorization--route-guards)
24. [Testing](#testing)
25. [Docker / Deployment](#docker--deployment)
26. [LX UI Patterns vs Other Vue Frameworks](#lx-ui-patterns-vs-other-vue-frameworks)
27. [LX UI Component Props Reference](#lx-ui-component-props-reference)
28. [Common LX UI Components Quick Reference](#common-lx-ui-components-quick-reference)

---

## Key Rules & Constraints

These rules **MUST** be followed in every LX UI frontend project:

| # | Rule |
|---|------|
| 1 | **For new projects, MUST use `bun`** for dependency management. For existing projects, detect the lock file first: if `pnpm-lock.yaml` exists, **MUST use `pnpm`** (install, add, remove, run scripts) to match the existing project tooling. |
| 2 | If the SPA UI will be embedded into a backend library, **MUST create the frontend project under a `portal/` subdirectory** of the backend project root. |
| 3 | **Component-first: LX UI is the default and only sanctioned UI vocabulary.** Before writing any markup, check whether an LX UI component already covers the use case (consult the Component Props Reference and Quick Reference sections of this skill). If yes, use it. Custom components are only acceptable when no LX UI component fits, and they **MUST be built by composing existing LX UI sub-components** — never by re-implementing what LX UI already provides (buttons, inputs, modals, lists, dialogs, tooltips, badges, etc.). Pulling in third-party Vue UI libraries is forbidden. |
| 4 | **Views (pages) MUST contain ZERO custom HTML and ZERO custom CSS.** A `views/*.vue` file is a thin orchestrator: it imports LX UI components, supplies props/events, and composes them inside structural LX UI containers (`LxShell` is provided by the layout, then `LxForm` / `LxSection` / `LxRow` / `LxList` / `LxDataGrid` / `LxWidget` / `LxTile`, etc.). The only raw element allowed in a view template is a `<div>` wrapper when an LX UI grid container truly cannot express the layout — in which case prefer the LX UI utility class (`lx-dashboard`, `lx-button-set`, etc.) over hand-rolled CSS. **No `<style>` blocks in views.** Custom HTML/CSS, if absolutely unavoidable, lives in a dedicated component under `src/components/`, never inline in a view. |
| 5 | **MUST NOT create new CSS** except when absolutely necessary, and only inside a custom component (never in a view). Rely on LX UI's built-in classes and component props. |
| 6 | If new CSS is absolutely required, **MUST NOT define new colors** — only reference colors from the LX UI CSS theme (CSS custom properties such as `--color-brand`, `--color-label`, `--color-region`, etc.). |
| 7 | **MUST use as little raw HTML as possible.** Prefer LX UI components over bare HTML elements (`LxButton` not `<button>`, `LxTextInput` not `<input>`, `LxList`/`LxListItem` not `<ul>/<li>`, `LxRichTextDisplay` not `<div v-html>`). Use `<div>` wrappers only when structurally required for layout. |
| 8 | **File naming: MUST NOT duplicate prefixes/suffixes in filenames.** Store files drop the `use` prefix and `Store` suffix (e.g., `stores/app.js` not `stores/useAppStore.js`). Service files drop the `Service` suffix (e.g., `services/auth.js` not `services/authService.js`). Hook files drop the `use` prefix (e.g., `hooks/errors.js` not `hooks/useErrors.js`). The **exported symbol name** keeps the full conventional name (e.g., `export default useAppStore`), and **imports** use that full name: `import useAppStore from '@/stores/app'`. |
| 9 | **MUST provide two mandatory views in every project**: a **404 page** rendering `LxErrorPage kind="404"` (catch-all route, e.g. `/:pathMatch(.*)*`), and an **accessibility settings page** rendering `<LxAccessibilitySettings />` (route e.g. `/accessibility`, marked `anonymous: true`). Both views are thin wrappers — render the LX UI component, do **not** rebuild the layout or add custom styling. Wire the accessibility route into `LxShell` `navItems` (or expose it via the shell's accessibility button) so users can reach it. |
| 10 | **MUST NOT use emoji in UI.** Use `LxIcon` for iconography, `LxBadge` for status/count indicators, `LxStateDisplay` for state labels, and `LxFlag` for country/language flags. Emoji are invisible to screen readers, render inconsistently across platforms, and duplicate what LX UI components already provide. |

---

## Project Placement

### Standalone frontend

```
my-app/
├── src/
├── public/
├── index.html
├── package.json
├── vite.config.mjs
└── ...
```

### Embedded in backend library

When the SPA is served by a Go (azugo) backend or any other backend library, place the entire frontend under `portal/`:

```
my-backend-project/
├── app.go
├── routes/
├── portal/            ← frontend project root
│   ├── src/
│   ├── public/
│   ├── index.html
│   ├── package.json
│   ├── vite.config.mjs
│   └── ...
├── go.mod
└── ...
```

---

## Initializing a New Project

```sh
# Create the project directory (standalone or portal/)
mkdir -p portal && cd portal

# Initialize with bun
bun init

# Install core dependencies
bun add @dativa-lv/lx-ui vue vue-router pinia vue-i18n axios @vueuse/core vue-dompurify-html

# Install dev dependencies
bun add -d vite @vitejs/plugin-vue vite-plugin-html vite-plugin-compression vitest @vue/test-utils jsdom
```

## Existing Project Package Manager Detection

When working in an existing project, choose the package manager by lock file:

```sh
# From project root
if [ -f pnpm-lock.yaml ]; then
  echo "Use pnpm for all package operations"
else
  echo "No pnpm lockfile detected; follow project conventions (bun for new projects)"
fi
```

If `pnpm-lock.yaml` exists, use `pnpm install`, `pnpm add`, `pnpm remove`, and `pnpm run ...`.

For new projects, keep `package.json` aligned with bun-based tooling (do not force npm/pnpm in `packageManager` unless required by existing project conventions):

```json
{
  "name": "my-portal",
  "version": "1.0.0",
  "scripts": {
    "dev": "vite --host",
    "build": "vite build",
    "preview": "vite preview",
    "test": "vitest --run",
    "lint": "eslint src/"
  }
}
```

---

## Directory Layout

```
portal/
├── public/                         # Static assets served as-is
│   ├── favicon.ico
│   ├── icon.svg
│   ├── imgs/                       # Static images
│   ├── meta/                       # OG images etc.
│   ├── themes/                     # LX UI theme CSS files (copied at build time)
│   └── robots.txt
├── src/
│   ├── main.js                     # App bootstrap
│   ├── App.vue                     # Root Vue component
│   ├── constants.js                # App-wide constants (config, keys)
│   ├── api.js                      # Default Axios instance (main backend)
│   ├── apiAuth.js                  # Auth Axios instance
│   ├── assets/
│   │   └── styles.css              # Minimal custom CSS (only if necessary)
│   ├── components/                 # Reusable custom components (must wrap LX UI components)
│   │   └── MyWidget.vue
│   ├── hooks/                      # Composables (useErrors, useLanguage, useRights, etc.)
│   │   ├── errors.js               # import useErrors from '@/hooks/errors'
│   │   ├── language.js             # import { useLanguageSwitcher } from '@/hooks/language'
│   │   └── rights.js               # import useRights from '@/hooks/rights'
│   ├── i18n/
│   │   └── index.js                # vue-i18n setup
│   ├── layouts/
│   │   └── MainLayout.vue          # LxShell wrapper layout
│   ├── locales/
│   │   ├── en.json                 # English translations
│   │   └── lv.json                 # Latvian translations (add more as needed)
│   ├── router/
│   │   ├── index.js                # createRouter setup
│   │   ├── routes.js               # Route definitions
│   │   └── events.js               # Navigation guards (beforeEach/afterEach)
│   ├── services/                   # API call functions grouped by domain
│   │   ├── auth.js                 # import { session, logout } from '@/services/auth'
│   │   └── entity.js               # import { getEntities } from '@/services/entity'
│   ├── stores/                     # Pinia stores
│   │   ├── app.js                  # import useAppStore from '@/stores/app'
│   │   ├── auth.js                 # import useAuthStore from '@/stores/auth'
│   │   ├── notify.js               # import useNotifyStore from '@/stores/notify'
│   │   ├── confirm.js              # import useConfirmStore from '@/stores/confirm'
│   │   └── view.js                 # import useViewStore from '@/stores/view'
│   ├── utils/                      # Pure utility functions
│   │   └── helpers.js
│   └── views/                      # Page-level components (one per route)
│       ├── Dashboard.vue
│       ├── NotFound.vue
│       ├── Error404.vue
│       └── feature/
│           ├── FeatureList.vue
│           └── FeatureDetail.vue
├── tests/
│   ├── setup.js
│   └── unit/
├── index.html
├── package.json
├── vite.config.mjs
├── vite.config.server.mjs          # Dev server proxy configuration
├── jsconfig.json                   # Path aliases for IDE
├── tsconfig.json                   # Type checking config
├── .eslintrc.js
├── .gitignore
└── Dockerfile
```

---

## Entry Point — index.html

The `index.html` at the project root is the single HTML entry point. Key patterns:

- Body has class `lx` on the `<body>` tag — required by LX UI.
- A single `<div id="app"></div>` mount point.
- A `<script>` block that defines `window.config` for runtime configuration (URLs, client IDs, environment). These values use template placeholders (`<%= ... %>` via `vite-plugin-html`) that are replaced at build time and via env-substitution in Docker.
- Module script loading the app: `<script type="module" src="/src/main.js"></script>`

```html
<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8" />
  <meta name="viewport" content="width=device-width,initial-scale=1.0" />
  <link rel="icon" href="<%= BASE_URL %>icon.svg" type="image/svg+xml" />
  <title><%= VUE_APP_NAME %></title>
  <script>
    window.config = {
      publicUrl: '<%= BASE_URL %>',
      version: '<%= VUE_APP_VERSION %>',
      environment: '<%= VUE_APP_ENVIRONMENT %>',
      serviceUrl: '<%= VUE_APP_SERVICE_URL %>',
      authUrl: '<%= VUE_APP_AUTH_URL %>',
      clientId: '<%= VUE_APP_CLIENT_ID %>',
    };
  </script>
</head>
<body class="lx">
  <noscript><strong>This app requires JavaScript.</strong></noscript>
  <div id="app"></div>
  <script type="module" src="/src/main.js"></script>
</body>
</html>
```

---

## Application Bootstrap — main.js

`main.js` is the Vue application entry point. It:

1. Creates the Vue app from `App.vue`.
2. Installs Pinia, Router, i18n.
3. Installs the LX UI plugin via `createLx`.
4. Imports all required LX UI CSS stylesheets.
5. Mounts to `#app`.

### Mandatory LX UI CSS imports

Import the LX UI styles **in `main.js`** — these are the CSS modules the framework provides. Import only what you need, but the reset, fonts, theme, and core modules are typically required:

```js
// Reset & fonts (always required)
import '@dativa-lv/lx-ui/dist/styles/lx-reset.css';
import '@dativa-lv/lx-ui/dist/styles/lx-fonts-carbon.css';

// Theme (pick one product theme + utility themes for light/dark/contrast)
import '@dativa-lv/lx-ui/dist/styles/lx-pt-carbon.css';
import '@dativa-lv/lx-ui/dist/styles/lx-ut-carbon-light.css';
import '@dativa-lv/lx-ui/dist/styles/lx-ut-carbon-dark.css';
import '@dativa-lv/lx-ui/dist/styles/lx-ut-carbon-contrast.css';

// Component-specific styles (import as needed)
import '@dativa-lv/lx-ui/dist/styles/lx-buttons.css';
import '@dativa-lv/lx-ui/dist/styles/lx-data-grid.css';
import '@dativa-lv/lx-ui/dist/styles/lx-inputs.css';
import '@dativa-lv/lx-ui/dist/styles/lx-steps.css';
import '@dativa-lv/lx-ui/dist/styles/lx-forms.css';
import '@dativa-lv/lx-ui/dist/styles/lx-notifications.css';
import '@dativa-lv/lx-ui/dist/styles/lx-modal.css';
import '@dativa-lv/lx-ui/dist/styles/lx-loaders.css';
import '@dativa-lv/lx-ui/dist/styles/lx-lists.css';
import '@dativa-lv/lx-ui/dist/styles/lx-expanders.css';
import '@dativa-lv/lx-ui/dist/styles/lx-tabs.css';
import '@dativa-lv/lx-ui/dist/styles/lx-animations.css';
import '@dativa-lv/lx-ui/dist/styles/lx-master-detail.css';
import '@dativa-lv/lx-ui/dist/styles/lx-ratings.css';
import '@dativa-lv/lx-ui/dist/styles/lx-day-input.css';
import '@dativa-lv/lx-ui/dist/styles/lx-map.css';
import '@dativa-lv/lx-ui/dist/styles/lx-shell-grid.css';
import '@dativa-lv/lx-ui/dist/styles/lx-forms-grid.css';
import '@dativa-lv/lx-ui/dist/styles/lx-treelist.css';
import '@dativa-lv/lx-ui/dist/styles/lx-date-pickers.css';
import '@dativa-lv/lx-ui/dist/styles/lx-data-visualizer.css';
import '@dativa-lv/lx-ui/dist/styles/lx-stack.css';
import '@dativa-lv/lx-ui/dist/styles/lx-cards.css';
import '@dativa-lv/lx-ui/dist/styles/lx-transparency.css';
import '@dativa-lv/lx-ui/dist/styles/lx-toggles.css';
```

### Bootstrap code

```js
import { createApp } from 'vue';
import { createPinia } from 'pinia';
import App from '@/App.vue';
import router from '@/router';
import events from '@/router/events';
import i18n from '@/i18n';
import { createLx } from '@dativa-lv/lx-ui';
import { APP_CONFIG, AUTH_KEY_TOKEN_SESSION } from '@/constants';

// LX UI CSS imports (see above) ...

// Optional: project-specific custom styles (only if absolutely needed)
// import '@/assets/styles.css';

const app = createApp(App);

app.use(createPinia());
events(router);
app.use(router);
app.use(i18n);

app.use(createLx, {
  systemId: 'my-system',
  authSessionKey: AUTH_KEY_TOKEN_SESSION,
  authUrl: APP_CONFIG.authUrl,
  authClientId: APP_CONFIG.clientId,
  publicUrl: APP_CONFIG.publicUrl,
  environment: APP_CONFIG.environment,
});

app.mount('#app');
```

---

## App.vue — Root Component

The root component is minimal. It watches the route to update `document.title` and renders either an error page or the `<router-view>`:

```vue
<script setup>
import { watch } from 'vue';
import { useRoute } from 'vue-router';
import { useI18n } from 'vue-i18n';
import useAppStore from '@/stores/app';
import Error404 from '@/views/Error404.vue';

const i18n = useI18n();
const route = useRoute();
const appStore = useAppStore();

watch(
  route,
  () => {
    let title = i18n.t('title.prefix');
    if (typeof route.meta.title === 'function') {
      title += route.meta.title(i18n);
    } else {
      title = i18n.t(route.meta?.title?.toString() || 'title.default');
    }
    document.title = title;
  },
  { immediate: true }
);
</script>
<template>
  <div class="lx-layout lx-override" v-if="appStore.showError">
    <Error404 />
  </div>
  <router-view v-else />
</template>
```

---

## Vite Configuration

### `vite.config.mjs`

Key configuration points:

- **Alias `@`** → `./src` for clean imports.
- **LX fonts alias** `/lx-fonts` → `node_modules/@dativa-lv/lx-ui/dist/lx-fonts`.
- **Plugins**: `vue()`, `createHtmlPlugin` (for template variable injection into `index.html`), `mkcert` (HTTPS in dev), `viteCompression` (gzip), LX UI vite plugins (`lxViteSecureHeadersPlugin`, `lxVitePortalVersionPlugin`).
- **Theme copy function**: copies `lx-pt-*.css` files from `node_modules/@dativa-lv/lx-ui/dist/styles` to `public/themes/` at build start.
- **Build target**: `es2020`.
- **Environment variable management**: `loadEnv` is used to create environment variables. In dev mode, actual URLs are used; in production builds, placeholder strings like `{{SERVICE_URL}}` are emitted so Docker entrypoint scripts can substitute them at runtime.

```js
import { defineConfig, loadEnv } from 'vite';
import vue from '@vitejs/plugin-vue';
import path from 'path';
import { createHtmlPlugin } from 'vite-plugin-html';
import viteCompression from 'vite-plugin-compression';
import { lxViteSecureHeadersPlugin, lxVitePortalVersionPlugin } from '@dativa-lv/lx-ui/vite';
import { fileURLToPath } from 'url';
import packageJson from './package.json';

export default defineConfig((command) => {
  const serving = command?.command === 'serve' && command?.mode === 'development';
  const env = loadEnv(command.mode, process.cwd(), '');

  const envVariables = {
    VUE_APP_VERSION: packageJson.version,
    VUE_APP_SERVICE_URL: serving ? '/api/' : (env.SERVICE_URL || '{{SERVICE_URL}}'),
    VUE_APP_AUTH_URL: serving ? '/idauth/' : (env.AUTH_URL || '{{AUTH_URL}}'),
    VUE_APP_CLIENT_ID: env.CLIENT_ID || 'my-client',
    VUE_APP_ENVIRONMENT: serving ? 'local' : (env.NODE_ENV || '{{ENVIRONMENT}}'),
    BASE_PATH: env.BASE_PATH || (serving ? '/' : '//BASE_PATH//'),
    BASE_URL: env.PUBLIC_URL || (serving ? 'https://localhost:44342/' : '{{PUBLIC_URL}}'),
  };

  return {
    base: envVariables.BASE_PATH,
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
        '/lx-fonts': fileURLToPath(
          new URL('./node_modules/@dativa-lv/lx-ui/dist/lx-fonts', import.meta.url)
        ),
      },
    },
    plugins: [
      vue(),
      createHtmlPlugin({ minify: true, inject: { data: envVariables } }),
      viteCompression({ algorithm: 'gzip', ext: '.gz' }),
      lxViteSecureHeadersPlugin({ /* CSP, referrer policy, etc. */ }),
      lxVitePortalVersionPlugin(),
    ],
    build: {
      target: ['es2020'],
      outDir: './dist',
      sourcemap: true,
    },
  };
});
```

### `vite.config.server.mjs`

Defines dev proxy settings to forward `/api/` and `/idauth/` to actual backend URLs:

```js
export const devServerSettings = (env) => ({
  port: new URL(env.BASE_URL).port,
  https: new URL(env.BASE_URL).protocol === 'https:',
  proxy: {
    '/api': {
      target: env.VUE_APP_SERVICE_URL_PROXY,
      changeOrigin: true,
      secure: false,
      rewrite: (p) => p.replace(/^\/api/, ''),
    },
    '/idauth': {
      target: env.VUE_APP_AUTH_URL_PROXY,
      changeOrigin: true,
      secure: false,
      rewrite: (p) => p.replace(/^\/idauth/, ''),
    },
  },
});
```

---

## Router Setup

### `router/index.js`

Uses `createWebHistory`. The history base is derived from the `publicUrl` config. **Language switching is NOT done via URL locale prefixes** — `LxShell` has a built-in language picker that operates at the UI level (see the LxShell setup section). The router is straightforward:

```js
import { createRouter, createWebHistory } from 'vue-router';
import routes from '@/router/routes';
import { APP_CONFIG } from '@/constants';

const router = createRouter({
  history: createWebHistory(
    APP_CONFIG.publicUrl.indexOf('://') !== -1
      ? new URL(APP_CONFIG.publicUrl).pathname
      : APP_CONFIG.publicUrl
  ),
  scrollBehavior(to, from, savedPosition) {
    if (savedPosition) return savedPosition;
    return new Promise((resolve) => {
      setTimeout(() => resolve({ left: 0, top: 0 }), 15);
    });
  },
  routes,
});

export default router;
```

### `router/routes.js`

Routes use a flat layout — no locale prefix in the URL. The `MainLayout` is the top-level component wrapping all authenticated pages:

```js
const routes = [
  {
    path: '/',
    name: 'home',
    component: () => import('@/layouts/MainLayout.vue'),
    meta: { title: 'pages.home.title' },
    children: [
      {
        path: 'dashboard',
        name: 'dashboard',
        meta: { title: 'pages.dashboard.title' },
        component: () => import('@/views/Dashboard.vue'),
      },
      {
        path: 'entity/:id',
        name: 'entityDetail',
        meta: {
          title: 'pages.entity.title',
          breadcrumbs: [
            { text: 'pages.entities.title', to: { name: 'entities' } },
          ],
        },
        component: () => import('@/views/entity/EntityDetail.vue'),
      },
      // ... more child routes
      {
        path: 'accessibility',
        name: 'accessibility',
        meta: { title: 'pages.accessibility.title', anonymous: true },
        component: () => import('@/views/Accessibility.vue'),
      },
      {
        path: ':pathMatch(.*)*',
        name: 'notFound',
        meta: { title: 'pages.notFound.title', anonymous: true },
        component: () => import('@/views/NotFound.vue'),
      },
    ],
  },
];
```

**Route meta properties:**

| Property | Type | Description |
|----------|------|-------------|
| `title` | `string \| (i18n) => string` | Page title (i18n key or function) |
| `description` | `string \| (i18n) => string` | Page description for header |
| `anonymous` | `boolean` | Route accessible without authentication |
| `onlyAnonymous` | `boolean` | Route accessible only for anonymous users |
| `breadcrumbs` | `Array<{text, to}>` | Breadcrumb trail |
| `access` | `(rights) => boolean` | Access control function |
| `category` | `string` | Route category (e.g., `'important'`) for site map grouping |

### `router/events.js`

Defines global navigation guards using LX UI flow utilities:

```js
import useAuthStore from '@/stores/auth';
import useAppStore from '@/stores/app';
import { lxFlowUtils } from '@dativa-lv/lx-ui';
import useRights from '@/hooks/rights';
import useViewStore from '@/stores/view';

function checkRouteAccess(record, rights) {
  if (typeof record.meta.access === 'function') return record.meta.access(rights);
  if (record.meta.access === undefined) return true;
  throw new Error('Invalid access property in route');
}

function routeCheckCallback(to, from, next) {
  const rights = useRights();
  const onlyAnonymous = to.matched.some((r) => r.meta.onlyAnonymous);
  const routesWithAccessControl = to.matched.filter((r) => r.meta.access);
  const canAccessRoute =
    routesWithAccessControl.length === 0 ||
    routesWithAccessControl.some((r) => checkRouteAccess(r, rights));
  if (canAccessRoute && !onlyAnonymous) { next(); return; }
  next({ name: 'forbidden', replace: true });
}

export default (router) => {
  router.beforeEach(async (to, from, next) => {
    const authStore = useAuthStore();
    const appStore = useAppStore();
    await lxFlowUtils.beforeEach(to, from, next, appStore, authStore, routeCheckCallback);
  });
  router.afterEach(async (to, from) => {
    const appStore = useAppStore();
    const viewStore = useViewStore();
    viewStore.$reset();
    await lxFlowUtils.afterEach(to, from, appStore);
    lxFlowUtils.removeFocus();
  });
};
```

---

## Layouts — LxShell

The `MainLayout.vue` uses `LxShell` — the primary LX UI application shell component that provides:

- Navigation bar (side or top)
- Header with page title, breadcrumbs, back button
- Footer with site map
- Notifications area
- Theme/language/accessibility pickers
- Session idle monitoring
- Confirmation dialogs
- Mega menu
- Cover mode for landing pages

### Essential LxShell setup

```vue
<script setup>
import { computed } from 'vue';
import { useRoute, useRouter } from 'vue-router';
import { useI18n } from 'vue-i18n';
import { LxShell, LxSiteMap, LxIcon } from '@dativa-lv/lx-ui';
import useAuthStore from '@/stores/auth';
import useAppStore from '@/stores/app';
import useNotifyStore from '@/stores/notify';
import useConfirmStore from '@/stores/confirm';
import useViewStore from '@/stores/view';

const route = useRoute();
const router = useRouter();
const i18n = useI18n();
const authStore = useAuthStore();
const appStore = useAppStore();
const notify = useNotifyStore();
const confirmStore = useConfirmStore();
const viewStore = useViewStore();

const nav = computed(() => [
  { id: 'dashboard', label: i18n.t('pages.dashboard.title'), icon: 'dashboard', to: { name: 'dashboard' } },
  { id: 'entities', label: i18n.t('pages.entities.title'), icon: 'list', to: { name: 'entities' } },
  // User menu items:
  { id: 'userProfile', label: i18n.t('pages.userProfile.title'), icon: 'user-profile', to: { name: 'userProfile' }, type: 'user-menu' },
]);

const selectedNavItems = computed(() => {
  const ret = {};
  ret[router.currentRoute.value.name] = true;
  if (route.meta?.breadcrumbs) {
    route.meta.breadcrumbs.forEach((item) => { ret[item.to?.name] = true; });
  }
  return ret;
});

const pageTitle = computed(() => {
  if (typeof route.meta.title === 'function') return route.meta.title(i18n);
  if (typeof route.meta.title === 'string') return viewStore?.pageTitle || i18n.t(route.meta.title);
  return '';
});

const breadcrumbs = computed(() => {
  if (!route.meta.breadcrumbs) return [];
  return route.meta.breadcrumbs.map((item) => ({
    label: i18n.t(item.text),
    to: item.to,
  }));
});

const userInfo = computed(() => {
  if (!authStore.isAuthorized) return null;
  return {
    firstName: authStore.session?.given_name,
    lastName: authStore.session?.family_name,
    description: authStore.session?.role ? i18n.t(`roles.${authStore.session?.role.code}`) : null,
    institution: authStore.session?.institution?.name,
  };
});

function goBack(path) { path !== -1 ? router.push(path) : router.back(); }
function goHome(path) { router.push(path); }
async function logout() { /* auth logout logic */ }

// Language picker — LxShell handles language switching via its built-in picker.
// Pass available languages and v-model the selected one; on @languageChange switch vue-i18n locale.
const useLanguage = useLanguageSwitcher(); // from hooks/language.js
const languages = [
  { id: 'lv', name: i18n.t('languages.lv') },
  { id: 'en', name: i18n.t('languages.en') },
];
const selectedLanguage = computed({
  get: () => languages.find((l) => l.id === i18n.locale.value) ?? languages[0],
  set: (lang) => useLanguage.switchLocale(lang.id),
});

// Shell texts: a large computed object with all i18n keys for shell UI labels.
// See the "LxShell texts pattern" section below.
const mainTexts = computed(() => ({
  defaultBack: i18n.t('shellTexts.defaultBack'),
  logOut: i18n.t('shellTexts.logOut'),
  // ... (full set of shell text keys)
}));
</script>

<template>
  <LxShell
    :routeName="route.name.toString()"
    :systemName="i18n.t('title.fullName')"
    :systemNameShort="i18n.t('title.shortName')"
    :userInfo="userInfo"
    :navItems="nav"
    :navItemsSelected="selectedNavItems"
    :pageLabel="pageTitle"
    :pageBackButtonVisible="breadcrumbs.length > 0"
    :pageBreadcrumbs="breadcrumbs"
    :pageIndexPath="{ name: 'home' }"
    :navigating="appStore.$state.isNavigating"
    :confirmDialogData="confirmStore"
    :hideNavBar="!viewStore?.isNavBarShown"
    :hideHeaderText="!viewStore?.isHeaderShown"
    :texts="mainTexts"
    has-language-picker
    :languages="languages"
    v-model:selected-language="selectedLanguage"
    v-model:notifications="notify.notifications"
    @goHome="goHome"
    @goBack="goBack"
    @logOut="logout"
  >
    <template #logo>
      <!-- SVG logo -->
    </template>
    <template #footer>
      <!-- Footer content using LxSiteMap etc. -->
    </template>
    <router-view />
  </LxShell>
</template>
```

### LxShell named slots

| Slot | Purpose |
|------|---------|
| `#logo` | Main logo SVG (displayed in nav bar) |
| `#logoSmall` | Compact logo variant for collapsed nav |
| `#footer` | Footer area (typically with `LxSiteMap` + links) |
| `#backdrop` | Background for cover mode |
| `#coverArea` | Content area for cover/landing mode |
| (default) | Main content area — put `<router-view />` here |

### LxShell modes

Set via `:mode` prop:

- `'default'` — Standard sidebar navigation
- `'cover'` — Landing/cover page mode
- `'public'` — Public-facing mode
- `'digives-lite'`, `'latvijalv'` — Special product themes

---

## Views (Pages)

Views are page-level components placed in `src/views/`. Each view corresponds to a route.

### Conventions

- Use `<script setup>` syntax.
- Import LX UI components directly: `import { LxButton, LxForm, LxRow } from '@dativa-lv/lx-ui'`.
- Use `useI18n()` for translations.
- Use stores for state and services for API calls.
- Keep HTML minimal — compose UI from LX UI components.

### Example: simple list page

```vue
<script setup>
import { ref, onMounted } from 'vue';
import { LxLoaderView, LxList } from '@dativa-lv/lx-ui';
import { useI18n } from 'vue-i18n';
import useNotifyStore from '@/stores/notify';
import useErrors from '@/hooks/errors';
import { getItems } from '@/services/entity';

const i18n = useI18n();
const notify = useNotifyStore();
const errors = useErrors();
const items = ref([]);
const loading = ref(false);

async function loadItems() {
  loading.value = true;
  try {
    const response = await getItems();
    if (response.status >= 200 && response.status < 300) {
      items.value = response.data;
    }
  } catch (error) {
    const err = errors.get(error);
    if (err.status === 403) {
      notify.pushError(i18n.t('errors.forbidden'));
    } else {
      notify.pushError(i18n.t('errors.loadFailed'));
    }
  } finally {
    loading.value = false;
  }
}

onMounted(() => loadItems());
</script>

<template>
  <LxLoaderView :loading="loading">
    <LxList
      :items="items"
      :column-definitions="[
        { id: 'name', attributeName: 'name', name: $t('list.name'), kind: 'primary' },
        { id: 'description', attributeName: 'description', name: $t('list.description') },
      ]"
      :action-definitions="[{ id: 'open', name: $t('actions.open'), icon: 'open' }]"
      @actionClick="(actionId, row) => router.push({ name: 'entity', params: { id: row.id } })"
    />
  </LxLoaderView>
</template>
```

### Mandatory views: 404 and Accessibility

Every project MUST ship two thin views (Rule #9). Both render a single LX UI component — no other markup, no custom layout.

**`views/NotFound.vue`** — bound to the catch-all `:pathMatch(.*)*` route (`name: 'notFound'`):

```vue
<script setup>
import { LxErrorPage } from '@dativa-lv/lx-ui';
import { useRouter } from 'vue-router';

const router = useRouter();

function action(actionName) {
  if (actionName === 'dashboard') { router.push({ name: 'dashboard' }); return; }
  router.go(-1);
}
</script>

<template>
  <LxErrorPage
    kind="404"
    :title="$t('pages.notFound.title')"
    :description="$t('pages.notFound.description')"
    :action-definitions="[
      { id: 'back', name: $t('pages.error.goBack'), icon: 'undo' },
      { id: 'dashboard', name: $t('pages.error.goHome'), icon: 'dashboard', kind: 'secondary' },
    ]"
    @actionClick="action"
  />
</template>
```

The same component covers `400`/`401`/`403`/`500`/`sessionTimeout` errors — reuse this pattern for any dedicated error view, just change `kind`.

**`views/Accessibility.vue`** — bound to `name: 'accessibility'`, `meta.anonymous: true`. The component reads/writes accessibility state through `inject` from the surrounding `LxShell`, so it just needs to be rendered:

```vue
<script setup>
import { LxAccessibilitySettings } from '@dativa-lv/lx-ui';
</script>

<template>
  <LxAccessibilitySettings />
</template>
```

Do **not** wrap it in additional containers, do **not** add headings/breadcrumbs (the shell + the component own those), and do **not** try to bind `v-model` for individual prefs — the shell already exposes them via `update:hasReducedAnimations` / `update:hasReducedTransparency` / `update:hasDeviceFonts` / `update:isTouchSensitive`, which the `appStore` should persist.

Wire access to this view by adding an entry to `LxShell` `navItems` (or a footer/site-map link), e.g. `{ id: 'accessibility', label: i18n.t('shellTexts.accessibility'), icon: 'accessibility', to: { name: 'accessibility' } }`. Because `meta.anonymous: true`, it must remain reachable on the public/cover modes too.

### Example: dashboard with widgets, tiles, forms

```vue
<script setup>
import { LxTile, LxList, LxForm, LxRow, LxIcon, LxWidget } from '@dativa-lv/lx-ui';
import { computed } from 'vue';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

const i18n = useI18n();
const router = useRouter();

const tiles = computed(() => [
  { id: 'entities', icon: 'list', label: i18n.t('pages.entities.title'), to: { name: 'entities' } },
  { id: 'settings', icon: 'settings', label: i18n.t('pages.settings.title'), to: { name: 'settings' } },
]);
</script>

<template>
  <div class="lx-dashboard">
    <LxTile
      v-for="tile in tiles"
      :key="tile.id"
      :icon="tile.icon"
      :label="tile.label"
      :to="tile.to"
    />
  </div>
</template>
```

---

## Components

Custom components go in `src/components/`. They **MUST** wrap or compose LX UI components — never build raw HTML equivalents.

### Example: custom component using LX UI subcomponents

```vue
<script setup>
import { LxIcon, LxButton } from '@dativa-lv/lx-ui';

defineProps({
  title: { type: String, required: true },
  icon: { type: String, default: 'info' },
});

defineEmits(['action']);
</script>

<template>
  <div class="lx-list-item">
    <LxIcon :value="icon" />
    <p class="lx-primary">{{ title }}</p>
    <LxButton label="Open" kind="ghost" icon="next" variant="icon-only" @click="$emit('action')" />
  </div>
</template>
```

---

## Stores (Pinia)

### Built-in LX UI stores

LX UI provides pre-built store factories. Use them with `defineStore`:

```js
// stores/app.js
import { defineStore } from 'pinia';
import { LxAppStore } from '@dativa-lv/lx-ui';
export default defineStore('appStore', LxAppStore);

// stores/notify.js
import { defineStore } from 'pinia';
import { LxNotifyStore } from '@dativa-lv/lx-ui';
export default defineStore('notifyStore', LxNotifyStore);

// stores/confirm.js
import { defineStore } from 'pinia';
import { LxConfirmStore } from '@dativa-lv/lx-ui';
export default defineStore('confirmStore', LxConfirmStore);

// stores/view.js
import { defineStore } from 'pinia';
import { LxViewStore } from '@dativa-lv/lx-ui';
export default defineStore('viewStore', LxViewStore);
```

### Auth store

The auth store uses `LxAuthStore` and `LxAuthService` from LX UI:

```js
// stores/auth.js
import { defineStore } from 'pinia';
import { LxAuthStore, LxAuthService } from '@dativa-lv/lx-ui';
import { AUTH_KEY_TOKEN_SESSION, AUTH_SCOPE, APP_CONFIG } from '@/constants';

export default defineStore(
  'authStore',
  LxAuthStore(
    LxAuthService,
    APP_CONFIG.authUrl,
    APP_CONFIG.publicUrl,
    APP_CONFIG.clientId,
    AUTH_SCOPE,
    AUTH_KEY_TOKEN_SESSION
  )
);
```

### Custom stores

For domain-specific state, create setup-function stores:

```js
// stores/entity.js
import { defineStore } from 'pinia';
import { ref } from 'vue';
import { getEntities } from '@/services/entity';

export const useEntityStore = defineStore('entityStore', () => {
  const entities = ref([]);
  const loading = ref(false);

  async function fetch() {
    loading.value = true;
    try {
      const resp = await getEntities();
      if (resp.status === 200) entities.value = resp.data;
    } finally {
      loading.value = false;
    }
  }

  function reset() { entities.value = []; }

  return { entities, loading, fetch, reset };
});
```

---

## Services (API layer)

Services live in `src/services/` and contain functions that call API endpoints using Axios instances:

```js
// services/entity.js
import api from '@/api';

export function getEntities() {
  return api().get('/entities');
}

export function getEntity(id) {
  return api().get(`/entities/${id}`);
}

export function createEntity(data) {
  return api().post('/entities', data);
}

export function updateEntity(id, data) {
  return api().put(`/entities/${id}`, data);
}

export function deleteEntity(id) {
  return api().delete(`/entities/${id}`);
}
```

### Auth service

```js
// services/auth.js
import apiAuth from '@/apiAuth';
import { APP_CONFIG, AUTH_SCOPE } from '@/constants';

export function session() { return apiAuth().get('/api/1.0/session'); }
export function keepAlive() { return apiAuth().get('/api/1.0/session/keep-alive'); }
export function roles() { return apiAuth().get('/api/1.0/session/roles'); }
export function setRole(roleCode) { return apiAuth().patch('/api/1.0/session', { role: roleCode }); }

export function authorize() {
  const params = new URLSearchParams({ client_id: APP_CONFIG.clientId, scope: AUTH_SCOPE });
  window.location.href = `${APP_CONFIG.authUrl}authorize?${params.toString()}`;
}

export async function logout() { await apiAuth().delete('/api/1.0/session'); }
```

---

## API Clients (Axios)

Each API base URL gets its own Axios factory function in the `src/` root:

```js
// api.js — default backend API
import axios from 'axios';
import { getSessionKey } from '@/apiAuth';
import { APP_CONFIG } from '@/constants';

export default () => {
  const http = axios.create({
    baseURL: APP_CONFIG.apiUrl,
    withCredentials: true,
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getSessionKey()}`,
    },
  });
  return http;
};
```

```js
// apiAuth.js — auth API
import axios from 'axios';
import { APP_CONFIG, AUTH_KEY_TOKEN_SESSION } from '@/constants';

export const setSessionKey = (key) => sessionStorage.setItem(AUTH_KEY_TOKEN_SESSION, key);
export const getSessionKey = () => sessionStorage.getItem(AUTH_KEY_TOKEN_SESSION);
export const removeSessionKey = () => sessionStorage.removeItem(AUTH_KEY_TOKEN_SESSION);

export default () => {
  const http = axios.create({
    baseURL: APP_CONFIG.authUrl,
    withCredentials: false,
    headers: {
      Accept: 'application/json',
      'Content-Type': 'application/json',
      Authorization: `Bearer ${getSessionKey()}`,
    },
  });
  return http;
};
```

**Pattern**: Each factory returns a **new Axios instance** on every call, ensuring the `Authorization` header uses the latest session token.

---

## Hooks (Composables)

Hooks/composables go in `src/hooks/`. They encapsulate reusable logic.

### `hooks/errors.js` — Error handling composable

Extracts error details from Axios error responses, maps to user-friendly messages, and handles 401/403/404 redirects:

```js
import useAppStore from '@/stores/app';
import useAuthStore from '@/stores/auth';
import useNotifyStore from '@/stores/notify';
import { useI18n } from 'vue-i18n';
import { useRouter } from 'vue-router';

export default () => {
  const get = (error) => {
    const { response, code } = error;
    if (code === 'ECONNABORTED') return { status: 499, data: null };
    if (!response) return { status: 500, data: null };
    const { request, ...errorObject } = response;
    return { status: errorObject.status || 500, data: errorObject.data || null };
  };

  const push = async (error) => {
    const router = useRouter();
    const authStore = useAuthStore();
    const appStore = useAppStore();
    const err = get(error);
    if (err.status === 401) { authStore.$reset(); router.push({ name: 'notAuthorized' }); return; }
    if (err.status === 404) { await router.replace({ name: 'notFound' }); return; }
    if (err.data) { appStore.error = err.data; appStore.showError = true; }
  };

  return { get, push };
};
```

### `hooks/rights.js` — Permission checking

```js
import useAuthStore from '@/stores/auth';

export default () => {
  const authStore = useAuthStore();
  return {
    canViewEntities: () => authStore.session?.scope?.some((s) => s.includes('entity:read')),
    canEditEntities: () => authStore.session?.scope?.some((s) => s.includes('entity:write')),
  };
};
```

### `hooks/language.js` — Locale management

Manages locale storage and i18n locale switching. Uses `@vueuse/core`'s `useStorage` for persisting the user's language preference. **Does not perform URL routing** — language selection is handled by `LxShell`'s built-in language picker, which emits `@languageChange`; this hook responds by switching the `vue-i18n` locale and persisting the choice.

---

## Internationalization (i18n)

### `i18n/index.js`

```js
import { createI18n } from 'vue-i18n';
import lv from '@/locales/lv.json';
import en from '@/locales/en.json';
import { APP_CONFIG } from '@/constants';

const i18n = createI18n({
  locale: APP_CONFIG.defaultLocale,
  fallbackLocale: APP_CONFIG.fallbackLocale,
  legacy: false,
  messages: { lv, en },
});

export default i18n;
```

### Locale file structure (`locales/en.json`)

```json
{
  "title": {
    "prefix": "My App — ",
    "default": "My App",
    "fullName": "My Application",
    "shortName": "MyApp",
    "subheader": "Subtitle"
  },
  "languages": {
    "lv": "Latvian (Latviešu)",
    "en": "English"
  },
  "pages": {
    "home": { "title": "Home" },
    "dashboard": { "title": "Dashboard" },
    "notFound": { "title": "Page Not Found", "description": "The page you are looking for does not exist." },
    "accessibility": { "title": "Accessibility settings" },
    "error": { "title": "Error", "goBack": "Go Back", "goHome": "Home" }
  },
  "errors": {
    "forbidden": "You do not have access to this resource.",
    "loadFailed": "Failed to load data."
  },
  "shellTexts": {
    "defaultBack": "Back",
    "logOut": "Log Out",
    "openAlerts": "Open Notifications",
    "noAlerts": "No Notifications",
    "accessibility": "Accessibility"
  }
}
```

**Key rules:**
- All user-visible text must be in locale files — never hardcoded in templates.
- Use `i18n.t('key')` or `$t('key')` in templates.
- Shell texts are passed to `LxShell` as a `texts` prop (a large computed object mapping all required shell labels).

---

## Constants

```js
// constants.js
export const APP_CONFIG = {
  apiUrl: window.config.serviceUrl,
  authUrl: window.config.authUrl,
  publicUrl: window.config.publicUrl,
  clientId: window.config.clientId,
  environment: window.config.environment,
  defaultLocale: 'en',
  fallbackLocale: navigator.language,
  supportedLocales: ['lv', 'en'],
};

export const AUTH_KEY_TOKEN_SESSION = 'my-app-sessionkey';
export const AUTH_SCOPE = 'my-scope';
```

---

## Styles & Theming

### Rules (reiterated)

1. **Do NOT create new CSS** unless absolutely necessary.
2. **Do NOT create new color values** in CSS. Only use LX UI theme CSS custom properties.
3. Use LX UI's built-in CSS classes (`lx-layout`, `lx-dashboard`, `lx-divider`, `lx-primary`, `lx-secondary`, `lx-data`, `lx-label`, `lx-list-item`, `lx-toolbar`, etc.).

### Available LX UI CSS custom properties (colors)

These are defined by the active theme and available in any CSS:

| Variable | Purpose |
|----------|---------|
| `--color-brand` | Primary brand color |
| `--color-label` | Label/secondary text |
| `--color-data` | Data/primary text |
| `--color-region` | Region background |
| `--color-region-2` | Secondary region background |
| `--color-background` | Page background |
| `--color-chrome` | Border/chrome |
| `--color-interactive-hover-background` | Hover state |
| `--color-list-primary` | List primary text |
| `--color-nav-foreground` | Navigation text |
| `--color-footer-label` | Footer label text |
| `--color-grid-hover-background` | Data grid hover |
| `--color-region-hover-background` | Region hover |
| `--color-region-hover-foreground` | Region hover text |
| `--color-highlight` | Highlight accent |
| `--color-red`, `--color-orange`, `--color-yellow`, `--color-green`, `--color-teal`, `--color-blue`, `--color-purple` | Semantic colors |

### LX UI CSS utility classes

| Class | Purpose |
|-------|---------|
| `lx-layout` | Top-level layout wrapper |
| `lx-dashboard` | Dashboard grid container |
| `lx-divider` | Visual separator |
| `lx-primary` | Primary text styling |
| `lx-secondary` | Secondary text styling |
| `lx-data` | Data display text |
| `lx-label` | Label text |
| `lx-toolbar` | Toolbar container |
| `lx-button-set` | Group of buttons |
| `lx-list-item` | List item wrapper |
| `lx-list-item-container` | List item content area |

### If custom CSS is absolutely necessary

Put it in `src/assets/styles.css` and import it last in `main.js`. Only use theme variables:

```css
/* ONLY use theme colors — never hardcoded hex/rgb values */
.my-custom-section {
  background-color: var(--color-region);
  border: 1px solid var(--color-chrome);
  padding: 1rem;
}

.my-custom-section .label {
  color: var(--color-label);
  font-size: var(--small-font-size);
}
```

---

## Error Handling Patterns

### In views / components

```js
import useErrors from '@/hooks/errors';
import useNotifyStore from '@/stores/notify';

const errors = useErrors();
const notify = useNotifyStore();

try {
  const response = await someService();
  // handle success
} catch (error) {
  const err = errors.get(error);
  if (err.status === 403) {
    notify.pushError(i18n.t('errors.forbidden'));
  } else if (err.status >= 500) {
    await errors.push(error);  // shows error page
  } else {
    notify.pushError(i18n.t('errors.loadFailed'));
  }
}
```

### Notification methods

From `useNotifyStore` (LX UI's `LxNotifyStore`):

- `notify.pushSuccess(message)` — Green success notification
- `notify.pushError(message)` — Red error notification
- `notify.pushWarning(message)` — Yellow warning notification
- `notify.pushInfo(message, description?)` — Blue info notification

### Error pages

Use `LxErrorPage` component with `kind` prop:

- `"404"` — Not Found
- `"402"` — Generic Error
- `"403"` — Forbidden
- `"500"` — Server Error

---

## Authentication Flow

1. **Login**: User clicks login → `authorize()` from `services/auth.js` redirects to auth provider.
2. **Callback**: Auth provider redirects back with token → stored via `setSessionKey()`.
3. **Session check**: On every route change, `lxFlowUtils.beforeEach` checks if session is active via `authStore`.
4. **Keep-alive**: `authStore.keepAlive()` is called periodically and on mount.
5. **Logout**: Calls `authStore.logout()` → deletes session → redirects to session-ended page.
6. **Token usage**: Every Axios factory reads `getSessionKey()` from `sessionStorage` to set `Authorization: Bearer <token>`.

---

## Authorization & Route Guards

- Define access functions in route meta: `access: (rights) => rights.canViewEntities()`.
- `useRights()` hook checks `authStore.session.scope` for required scopes.
- `events.js` runs `routeCheckCallback` which checks all matched routes for `meta.access`.
- Unauthorized users are redirected to `forbidden` route.
- Anonymous routes are marked with `meta: { anonymous: true }`.

---

## Testing

### Setup

Tests use **Vitest** with `jsdom` environment:

```js
// vite.config.mjs (test section)
test: {
  globals: true,
  setupFiles: ['./tests/setup.js'],
  environment: 'jsdom',
  include: ['tests/unit/**/*.js'],
}
```

### Running tests

```sh
# New projects (bun)
bun run test
bun run test -- --watch

# Existing projects with pnpm-lock.yaml
pnpm test
pnpm test -- --watch
```

### Test file location

```
tests/
├── setup.js           # Global test setup
└── unit/
    ├── components/
    │   └── MyWidget.test.js
    └── stores/
        └── entity.test.js
```

### Example test

```js
import { describe, it, expect } from 'vitest';
import { mount } from '@vue/test-utils';
import MyWidget from '@/components/MyWidget.vue';

describe('MyWidget', () => {
  it('renders title', () => {
    const wrapper = mount(MyWidget, {
      props: { title: 'Test Title', icon: 'info' },
    });
    expect(wrapper.text()).toContain('Test Title');
  });
});
```

---

## Docker / Deployment

### Dockerfile

```dockerfile
FROM ghcr.io/wntrtech/nginx:latest

USER root
RUN apk update && apk add --no-cache gzip
USER nginx

COPY --chown=nginx:nginx dist/ /usr/share/nginx/html
COPY --chown=nginx:nginx docker/30-envsubst-content.sh /docker-entrypoint.d/30-envsubst-content.sh
COPY --chown=nginx:nginx docker/nginx.conf /etc/nginx/conf.d/default.conf

RUN chmod +x /docker-entrypoint.d/30-envsubst-content.sh
```

### Build

```sh
# New projects (bun)
bun run build

# Existing projects with pnpm-lock.yaml
pnpm build
```

This produces `dist/` with the static SPA files. The Docker entrypoint script (`30-envsubst-content.sh`) replaces `{{SERVICE_URL}}`, `{{AUTH_URL}}`, `{{ENVIRONMENT}}`, etc. placeholders in the built files at container startup.

### .gitignore

```
node_modules
.DS_Store
dist
*.local
.env
eslint-report.json
/test-results/
/playwright-report/
```

---

## LX UI Patterns vs Other Vue Frameworks

LX UI deliberately diverges from Vuetify, PrimeVue, Element Plus, Quasar, etc. If you reach for the "obvious" Vue convention, you will be wrong. Read this section before writing components.

### 1. Content goes through props, not the default slot

In most Vue frameworks the visible label/content of a component lives in the default slot:

```vue
<!-- WRONG — this is Vuetify/PrimeVue style, not LX UI -->
<LxButton kind="primary" @click="save">Save</LxButton>
<LxModal v-model="open"><h2>Confirm delete</h2>...</LxModal>
<LxRow><label>Name</label><input /></LxRow>
```

In LX UI the text is a **prop**:

```vue
<!-- CORRECT -->
<LxButton label="Save" kind="primary" @click="save" />
<LxModal :label="$t('modals.confirmDelete')" ref="modal"><MyForm /></LxModal>
<LxRow :label="$t('fields.name')"><LxTextInput v-model="name" /></LxRow>
```

Components that take **`label` (and often `description`) as a prop** instead of slot content:
`LxButton` (label is REQUIRED), `LxRow`, `LxSection`, `LxModal`, `LxModalForm`, `LxDialog`, `LxTile`, `LxWidget`, `LxListItem`, `LxCheckbox`, `LxRadioButton`, `LxInfoBox`, `LxEmptyState`, `LxErrorPage` (uses `title`/`description`), `LxExpander`, `LxLoaderView`, `LxStateDisplay`, `LxLogoDisplay` (uses `value`).

`LxIcon` is similar but uses `value` for the icon name: `<LxIcon value="check" />` — never a slot.

`LxButton` requires a `label` even when `variant="icon-only"` — it becomes the ARIA label and tooltip:

```vue
<LxButton label="Delete" icon="delete" variant="icon-only" kind="ghost" destructive @click="del" />
```

### 2. `kind` for behavior/style, `variant` for layout

Most frameworks merge these into one of `type` / `color` / `severity` / `appearance`. LX UI splits them:

| Concern | LX UI prop | Typical values |
|---|---|---|
| Visual style/role | `kind` | `default`, `primary`, `secondary`, `tertiary`, `ghost` (button); `info`, `warning`, `error`, `success`, `question` (dialog/info-box); `single`, `multiple` (selectors); `compact` / `default` (forms, steps) |
| Layout shape | `variant` | `default`, `icon-only` (button); `default`, `country`, `state`, `custom` (dropdown/autocomplete); `dropdown`, `tiles`, `tags`, `rotator` (value-picker); `default`, `bar` (loader); `default`, `highlighted` (expander) |

Do **not** invent `type` props. Do **not** use HTML's native `type="submit"` — buttons are not native. Wire submit via `@click`.

### 3. Selectors take `:items` + `idAttribute` / `nameAttribute` — no `<option>` children

```vue
<!-- WRONG — Vuetify/native style -->
<LxDropdown v-model="role">
  <option value="admin">Admin</option>
  <option value="user">User</option>
</LxDropdown>

<!-- CORRECT -->
<LxDropdown
  v-model="role"
  :items="[{ id: 'admin', name: 'Admin' }, { id: 'user', name: 'User' }]"
/>
<!-- Or, if the source has different keys: -->
<LxDropdown v-model="role" :items="roles" id-attribute="code" name-attribute="title" />
```

This `items + idAttribute + nameAttribute` triplet is shared across `LxDropdown`, `LxAutoComplete`, `LxValuePicker`, `LxContentSwitcher`, `LxSteps`, `LxList` (for filters/selecting). For `LxValuePicker` also: `iconAttribute`, `categoryAttribute`, `descriptionAttribute`.

### 4. `actionDefinitions` is the universal "buttons inside this component" prop

Other frameworks ask you to slot `<v-btn>` children into a `<v-toolbar>`/`<v-card-actions>` etc. LX UI components instead expose `:action-definitions` and emit a single `actionClick` event:

```vue
<LxModal
  :label="$t('users.edit')"
  :action-definitions="[
    { id: 'save',   name: $t('actions.save'),   kind: 'primary', icon: 'save' },
    { id: 'delete', name: $t('actions.delete'), kind: 'ghost', destructive: true, icon: 'delete' },
  ]"
  @actionClick="onAction"
>
  <MyForm />
</LxModal>
```

`actionDefinition` object keys: `id` (required), `name`, `icon`, `iconSet`, `kind`, `variant`, `disabled`, `loading`, `busy`, `destructive`, `active`, `badge`, `badgeIcon`, `badgeType`, `badgeTitle`, `tooltip`.

Components that consume `actionDefinitions`: `LxList`, `LxListItem`, `LxDataGrid`, `LxDataBlock`, `LxToolbar`, `LxToolbarGroup`, `LxDropDownMenu`, `LxForm`, `LxModalForm`, `LxModal` (action buttons in footer), `LxDialog`, `LxRow`, `LxSection`, `LxWidget`, `LxErrorPage`, `LxEmptyState`, `LxInfoBox`. Listen to `@actionClick="(actionId, row?) => ..."`.

### 5. i18n via the `texts` prop, not slots

When a component renders fixed strings (e.g. "No items", "Open menu", "Loading…"), you override them with a `:texts="{...}"` object — **never** by templating slots:

```vue
<LxList :items="items" :column-definitions="cols" :texts="{
  emptyState: { description: $t('users.empty') },
  search:    { placeholder: $t('users.searchPlaceholder') },
}" />
```

The component owns the text-table shape. Inspect each component's `textsDefault` constant to know which keys are available.

### 6. Badge is a 4-prop combo (and `badgeTitle` is required if used)

```vue
<LxButton label="Inbox" icon="mail" badge="3" badge-type="success" :badge-title="$t('a11y.unreadCount')" />
```

Props: `badge` (text/number), `badgeIcon` (alternative to text), `badgeType` (`default | info | success | warning | error`), `badgeTitle` (REQUIRED whenever `badge` or `badgeIcon` is set — used for screen readers; LX UI logs a dev warning otherwise). Same combo on `LxSection`, `LxTile`, `LxExpander`, `LxListItem`, etc.

### 7. v-model is the standard Vue 3 contract, with quirks

- All form controls are `v-model`-friendly.
- `LxToggle` is **tri-state**: `null` ⇒ indeterminate, `true`/`false` ⇒ on/off. Initialize to `false`, never leave it `undefined`.
- `LxValuePicker` returns either a single value or array depending on `kind`; force consistent array output with `:always-as-array="true"`.
- `LxAutoComplete` and `LxDropdown` only fire the v-model emit; do not bind to `@change` like a native `<select>`.
- `LxSteps` uses `modelValue` as the **current step id**, not an index.
- `LxModal` / `LxModalForm` / `LxDialog` open via **template refs and `.open()` / `.close()` methods** — they do not use `v-model="open"`.

```vue
<script setup>
const modal = ref(null);
function ask() { modal.value.open(); }
</script>
<template>
  <LxModal
    ref="modal"
    :label="$t('confirm')"
    :action-definitions="[
      { id: 'save', name: $t('actions.save'), kind: 'primary' },
      { id: 'cancel', name: $t('actions.cancel'), kind: 'ghost' },
    ]"
    @actionClick="(id) => id === 'save' && onConfirm()"
  >…</LxModal>
</template>
```

### 8. Four orthogonal "not-quite-disabled" states

LX UI distinguishes:
- `disabled` — non-interactive, dimmed, in tab order skipped.
- `loading` — input replaced by spinner; user is waiting for fetch.
- `busy` — control still visible but locked while a side-effect runs (e.g. saving). Often paired with `busyTooltip`.
- `readOnly` — value is shown but cannot be edited; rendering frequently changes (text instead of input). Many controls also accept `readOnlyRenderType` (`row` / `column`).

Use the right one. They are not aliases.

### 9. `iconSet` selects the icon library at the component, not via name prefix

Don't write `icon="mdi-check"` or `icon="pi pi-check"`. LX UI swaps the registry:

```vue
<LxIcon value="check" />                          <!-- inherits global iconSet -->
<LxIcon value="rocket-launch" icon-set="phosphor" />
<LxButton label="…" icon="dativa-logo" icon-set="brand" />
```

Available sets: `cds` (default), `material`, `brand`, `phosphor`. Set the global default in `createLx({ iconSet: 'material' })`.

### 10. IDs are auto-generated; pass one only when something else has to refer to it

Almost every LX component computes `id: { default: () => generateUUID() }`. Do not invent your own unless you need a stable selector for tests, `aria-labelledby`, or `<label for>`-style binding (most form controls accept `labelId`). Don't reuse the same id across iterations of a `v-for`.

### 11. Forms: `LxForm` owns the layout, `LxRow` owns the field label, controls are children

Other frameworks let you mix labels and inputs inline. In LX UI:

```vue
<LxForm
  :column-count="2"
  :action-definitions="[{ id: 'save', name: $t('actions.save'), kind: 'primary' }]"
  @actionClick="onAction"
>
  <LxSection :label="$t('sections.basic')">
    <LxRow :label="$t('fields.name')" :required="true" :column-span="2">
      <LxTextInput v-model="user.name" />
    </LxRow>
    <LxRow :label="$t('fields.role')">
      <LxDropdown v-model="user.role" :items="roles" />
    </LxRow>
  </LxSection>
</LxForm>
```

`LxRow` props worth knowing: `label`, `description`, `required` (`true`/`false`/`null`), `orientation` (`vertical|horizontal`), `column-span` (`1|2|3|4|6|8`), `row-span`, `hide-label`, `input-id`. Form-level `requiredMode` (`required` / `required-asterisk` / `optional` / `indicator`) controls how required fields are marked.

### 12. Notifications, confirmations, and shell are stores, not components

Don't render `<LxToast>`. Push via the store:

```js
const notify = useNotifyStore();
notify.pushSuccess(i18n.t('notify.saved'));
notify.pushError(i18n.t('errors.loadFailed'));
notify.pushWarning(...);
notify.pushInfo(...);
```

Same for confirmation dialogs (`useConfirmStore().push({ ... })`) and global app state (`useAppStore()`). The single `<LxNotification>` rendered inside `LxShell` reads from the store.

### 13. Common emit names you will not guess

LX UI consistently uses **`-Click` / `-Change` (singular, present tense)** suffixes — never `Clicked` / `Changed`. The most common gotchas:

| Emit | Where | Payload |
|---|---|---|
| `actionClick` | List, DataGrid, Modal, ModalForm, Form, Toolbar, Row, Section, Widget, ErrorPage, EmptyState, InfoBox, Dialog, ListItem, Expander | `(actionId, row?)` |
| `selectionChange` | LxList, LxDataGrid | (NOT `selectionChanged`) selection payload |
| `sortingChange` | LxDataGrid | `{ column, direction }` (NOT `sortingChanged`) |
| `itemsPerPageChange` | LxDataGrid | new page size (NOT `*Changed`) |
| `selectPage` | LxDataGrid | new page number — paging lives on DataGrid, not List |
| `search` | LxList, LxDataGrid, LxToolbar | search string (NOT `searched`) |
| `loadMore` / `loadChildren` | LxList | — |
| `selectionActionClick` / `toolbarActionClick` / `emptyStateActionClick` | List, DataGrid | `(actionId, selection?)` |
| `close` | LxModal, LxModalForm, LxDialog | — (NOT `closed`) |
| `resetFilters` | LxExpander (with `hasShortlistReset`) | — |
| `goBack` / `goHome` / `logOut` / `languageChange` | LxShell | — (NOTE: `languageChange`, not `languageChanged`) |
| `contextPersonChange` / `alternativeProfileChange` / `confirmModalClose` | LxShell | — |

`LxShell` accessibility prefs are emitted as `update:hasReducedAnimations`, `update:hasReducedTransparency`, `update:hasDeviceFonts`, `update:isTouchSensitive` (note `hasReducedAnimations`, not `hasAnimations`). Other shell `update:*` events: `update:notifications`, `update:selected-language`, `update:selected-context-person`, `update:selected-alternative-profile`, `update:nav-bar-switch`, `update:selectedMegaMenuItem`, `update:customButtonOpened`, `update:customButtonBlink`, `update:spotlightItemCurrent`. Bind them back to your `appStore`.

---

## LX UI Component Props Reference

Concise per-component prop tables for the components used in 95% of screens. **REQ** = required, **AG** = auto-generated UUID. Only the most commonly used props are listed; consult the source under `node_modules/@dativa-lv/lx-ui/src/components/` for the long tail.

### LxButton

| Prop | Type | Notes |
|---|---|---|
| `label` | String | **REQ.** Visible text and ARIA label (also used when `variant="icon-only"`). |
| `kind` | String | `default` \| `primary` \| `secondary` \| `tertiary` \| `ghost` \| `menuitem`. |
| `variant` | String | `default` \| `icon-only`. With `icon-only`, supply `icon` and a meaningful `label`. |
| `icon` | String | Icon name. |
| `iconSet` | String | `cds` \| `material` \| `brand` \| `phosphor`. Defaults to global. |
| `destructive` | Boolean | Red styling for delete/remove actions. |
| `disabled` / `loading` / `busy` | Boolean | Distinct states (see §8 above). |
| `busyTooltip` | String | Tooltip while `busy` is true. |
| `href` | Object | Vue Router location object — turns the button into a link. |
| `openInNewTab` | Boolean | Adds a 2nd arg to `@click` so caller can ctrl/cmd-click. |
| `active` | Boolean | Sticky pressed state (toolbar toggles). |
| `badge` / `badgeIcon` / `badgeType` / `badgeTitle` | — | See §6 above. |
| `tabindex`, `customClass`, `ariaLabel`, `texts` | — | Standard. |

Emits: `click(event, openInNewTab?)`.

### LxIcon

| Prop | Type | Notes |
|---|---|---|
| `value` | String | Icon name (no slot, no children). |
| `iconSet` | String | Override global registry. |
| `variant` | String | `default` \| `gradient-brand` \| `gradient-brand-vertical`. |
| `animation` | String | `default` \| `spin`. |
| `meaningful` | Boolean | If `true`, icon is announced to screen readers; pair with `title`. |
| `title` / `desc` | String | SVG `<title>` / `<desc>` for a11y. |

### LxTextInput

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | String/Number | v-model. |
| `kind` | String | `default` \| `password` \| `search` \| `phone`. |
| `mask` | String | `default` \| `integer` \| `decimal` \| `gpslat` \| `gpslon` \| `time` \| `personCodeLv` \| `newbornId` \| `currency` \| `email` \| `numeric` \| `custom`. |
| `customMaskValue` | RegExp | When `mask="custom"`. |
| `scale` | Number | Decimal places (with `mask="decimal"`). |
| `signed` | Boolean | Allow negatives. |
| `placeholder`, `maxlength`, `tooltip`, `autocomplete`, `labelId` | — | Standard. |
| `disabled` / `readOnly` / `invalid` | Boolean | + `invalidationMessage`. |
| `uppercase` | Boolean | Force-upper while typing. |
| `options` | Object | Mask-specific config (e.g. `{ phone: 'lv' }`, `{ locale: 'en' }`). |

### LxTextArea

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | String | v-model. |
| `rows` | Number | Default 3. |
| `dynamicHeight` | Boolean | Auto-resize to content. |
| `placeholder`, `maxlength`, `tooltip`, `labelId` | — | Standard. |
| `disabled` / `readOnly` / `invalid` + `invalidationMessage` | — | Standard. |

### LxDropdown

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | String/Number | v-model — selected id. |
| `items` | Array | **REQ.** Array of objects. |
| `idAttribute` / `nameAttribute` | String | Default `id` / `name`. |
| `idAttributeArray` | Array \| String | Composite-key support — pass an array of attribute names if the id is split across multiple fields. |
| `kind` | String | `default` (LX-rendered panel) \| `native` (renders `<select>`). |
| `placeholder`, `tooltip`, `labelId`, `tabindex`, `meaningful`, `texts` | — | Standard. |
| `readOnly` / `disabled` / `invalid` + `invalidationMessage` | — | Standard. |

v-model only — no `change` event. The country/state/custom variants live on `LxAutoComplete` and `LxValuePicker`, not here.

### LxAutoComplete

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | String/Object | v-model. |
| `items` | Array \| Function | Sync array OR async fetcher `async (query) => items`. |
| `idAttribute` / `nameAttribute` | String | Default `id` / `name`. |
| `selectionKind` | String | `single` \| `multiple`. |
| `detailMode` | String | `simple` \| `detailed` (used with `hasDetails`). |
| `queryMinLength`, `queryMaxLength`, `queryDebounce` | Number | Default debounce 200 ms. |
| `searchAttributes` | Array | Which fields the local matcher inspects. |
| `tooltipAttribute` | String | Field name on each item to use as the option tooltip. |
| `hasSelectAll`, `hasDetails`, `enableAdditionalText`, `loading` | Boolean | Feature toggles. |
| `preloadedItems` | Array | Items always shown above the live results. |
| `groupId`, `placeholder`, `tooltip`, `labelId`, `texts` | — | Standard. |
| `readOnly` / `disabled` / `invalid` + `invalidationMessage` | — | Standard. |

Emits (in addition to v-model): `openDetails`. (No `variant` prop on AutoComplete — country/state pickers live on `LxValuePicker`.)

### LxCheckbox / LxRadioButton

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | Boolean | **REQ.** v-model. |
| `label` | String | Visible label (slot fallback exists but prefer prop). |
| `groupId` | String | Logical grouping for radios. |
| `value` | String | Radio value. |
| `disabled`, `tabindex`, `labelId` | — | Standard. |

### LxToggle

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | Boolean \| null | Tri-state. |
| `size` | String | `s` \| `m` (default `m`). |
| `label` | String | Inline label text. |
| `role` | String | ARIA role. Default `switch`. |
| `disabled` / `readOnly` / `invalid` + `invalidationMessage` | — | Standard. |
| `tooltip`, `labelId`, `texts` | — | `texts.valueYes`, `texts.valueNo`. |

Slots: `on`, `off`, `indeterminate` (custom on/off/indeterminate labels), default (used in read-only display).

### LxValuePicker

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | Array/String/Number | v-model. |
| `items` | Array | List of options. |
| `idAttribute` / `nameAttribute` / `iconAttribute` / `iconSetAttribute` / `categoryAttribute` / `descriptionAttribute` | String | Map item keys. |
| `selectionKind` | String | `single` (radios) \| `multiple` (checkboxes). |
| `variant` | String | `default`, `dropdown`, `tiles`, `tags`, `rotator`, `indicator`, `horizontal` — each also has a `*-custom` variant (`default-custom`, `dropdown-custom`, `tiles-custom`, `tags-custom`, `rotator-custom`, `horizontal-custom`) that hands rendering off to the `default` scoped slot. |
| `nullable` | Boolean | Only with `selectionKind="single"`: adds an explicit "Not selected" option. |
| `alwaysAsArray` | Boolean | Always emits an array even when `selectionKind="single"`. |
| `hasSearch`, `hasSelectAll`, `searchAttributes` | — | Search controls. |
| `readOnly` / `readOnlyRenderType` (`row` \| `column`) | — | Read-only display. |
| `groupId`, `placeholder`, `tooltip`, `disabled`, `invalid`, `invalidationMessage`, `labelId`, `texts` | — | Standard. |

### LxDayInput

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | Number/Object | Days representation. |
| `kind` | String | `label` \| `icon` (display style). |
| `disabled` / `readOnly` / `invalid` + `invalidationMessage` | — | Standard. |
| `labelId`, `texts` | — | `texts` carries unit labels. |

### LxModal / LxModalForm / LxDialog

Open/close via **`ref` + `.open()` / `.close()`**, not `v-model`. All footer buttons are driven by `actionDefinitions` — there are no separate `buttonPrimary*` / `buttonSecondary*` / `buttonCloseLabel` props.

| Prop (Modal) | Type | Notes |
|---|---|---|
| `label` | String | Modal title. |
| `size` | String | `default` \| `s` \| `m` \| `l` \| `xl`. |
| `kind` | String | `default` \| `native` (renders `<dialog>`). |
| `escEnabled` | Boolean | Default `true`. |
| `disableClosing` | Boolean | Locks the modal. |
| `buttonSecondaryIsCancel` | Boolean | Default `true` — treats the secondary action as a Cancel. |
| `actionDefinitions` | Array | Footer buttons. |
| `texts` | Object | i18n. |

| Prop (ModalForm — adds form layout to a Modal) | Type | Notes |
|---|---|---|
| `columnCount` | Number | Default `1`. |
| `index` / `indexType` | — | `indexType="expanders"` enables grouped sections (no tabs in modal-form). |
| `requiredMode` | String | `none` \| `required` \| `required-asterisk` \| `optional` \| `indicator` (default `indicator`). |
| `kind` | String | `default` \| `compact`. |
| `orientation` | String | `vertical` \| `horizontal`. |
| `showPreHeaderInfo` / `showPostHeaderInfo` | Boolean | Default `true`. |
| (plus all Modal props above) | | |

| Prop (Dialog) | Type | Notes |
|---|---|---|
| `label` / `description` | String | Title + body text — Dialog has no default slot. |
| `kind` | String | `default` \| `question` \| `info` \| `warning` \| `error` \| `success` \| `custom`. |
| `pictogram` | String | Used with `kind="custom"`. |
| `actionDefinitions` | Array | Buttons. |
| `escEnabled`, `disableClosing`, `buttonSecondaryIsCancel`, `texts` | — | As Modal. |

Emits (all three): `close`, `actionClick`. ModalForm additionally emits `update:index`. The `actionClick` payload is `(actionId)` matching the `id` of the corresponding entry in `actionDefinitions`. There is no `primaryAction` / `secondaryAction` event — model the Save / Cancel pair as two action definitions and dispatch on `actionId` in your handler.

### LxForm / LxSection / LxRow

`LxForm`:

| Prop | Type | Notes |
|---|---|---|
| `columnCount` | Number | Default 1. |
| `kind` | String | `default` \| `compact` \| `stripped`. |
| `index` | Array | Section index data when `indexType="tabs"` or `"expanders"`. |
| `indexType` | String | `default` \| `tabs` \| `expanders`. |
| `requiredMode` | String | `required` \| `required-asterisk` \| `optional` \| `indicator`. |
| `actionDefinitions` | Array | Header/footer action buttons. |
| `showHeader` / `stickyHeader` / `showFooter` / `stickyFooter` / `showPreHeaderInfo` / `showPostHeaderInfo` | Boolean | All default `true`. |
| `texts` | Object | Override built-in strings. |

Emits: `actionClick`, `update:index`. Slots: `pre-header`, `pre-header-info`, `header`, `post-header`, `post-header-info`, `sections`, default.

`LxSection`:

| Prop | Type | Notes |
|---|---|---|
| `label` / `description` | String | Section heading. |
| `columnCount` | Number | Override form's column count for this section. |
| `requiredMode` | String | `none` \| `required` \| `required-asterisk` \| `optional`. |
| `icon` / `iconSet` | String | Optional section icon. |
| `badge` / `badgeIcon` / `badgeType` / `badgeTitle` | — | Standard badge combo. |
| `actionDefinitions` | Array | Section-level toolbar. |
| `orientation` | String | `vertical` \| `horizontal`. |
| `expanderRenderMode` | String | `default` \| `dynamic` (lazy). |

`LxRow`:

| Prop | Type | Notes |
|---|---|---|
| `label` | String | Field label. |
| `description` | String | Helper text. |
| `required` | Boolean \| null | `true` / `false` / `null`. |
| `columnSpan` | Number/String | `1`, `2`, `3`, `4`, `6`, `8`. |
| `rowSpan` | Number/String | Row span. |
| `orientation` | String | `vertical` \| `horizontal`. |
| `hideLabel` | Boolean | Hide label visually but keep for a11y. |
| `inputId` | String | Bind `<label for>` to a specific input. |
| `actionDefinitions` | Array | Icon buttons next to the label. |

Slots: default (the input control), `info` (info-tooltip content).

### LxList

`LxList` is a vertical list of cards / tiles / tree items — **not** a tabular grid. It does **not** support paging, column-definitions, server-side sorting, or `clickableRole`. For a tabular surface with paging/sorting, use `LxDataGrid` (next section).

| Prop | Type | Notes |
|---|---|---|
| `items` | Array | Rows. |
| `kind` | String | `default` \| `draggable` \| `treelist`. |
| `listType` | String | Visual list style. Default `'3'`. |
| `groupDefinitions` | Array | Optional group headers (collapsible row groupings). |
| `idAttribute` / `nameAttribute` / `descriptionAttribute` / `hrefAttribute` / `groupAttribute` / `clickableAttribute` / `iconAttribute` / `iconSetAttribute` / `tooltipAttribute` / `categoryAttribute` / `childrenAttribute` / `hasChildrenAttribute` / `selectableAttribute` / `orderAttribute` | String | Map item keys. |
| `icon` / `iconSet` | String | Default row icon (default `open`). |
| `actionDefinitions` | Array | Per-row actions. |
| `actionsLayout` | String | `default` \| `vertical`. |
| `toolbarActionDefinitions` | Array | List-wide toolbar buttons. |
| `selectionActionDefinitions` | Array | Bulk actions visible when rows are selected. |
| `emptyStateActionDefinitions` | Array | Buttons shown in empty state. |
| `emptyStateIcon` | String | Empty-state icon. |
| `mode` | String | `client` (default) \| `server` — drives whether filtering/searching is local or `@search`-emitted. |
| `searchSide` | String | `client` (default) \| `server`. |
| `hasSearch`, `searchString`, `searchMode` (`default` \| `compact`) | — | Search controls. |
| `hasSelecting`, `selectionKind` (`single` \| `multiple`) | — | Selection. |
| `showLoadMore` | Boolean | Renders a "load more" trigger; emit `loadMore` to fetch next batch. |
| `hideFilteredItems` | Boolean | Hide rows that don't match current filters/search. |
| `includeUnspecifiedGroups` | Boolean | Render rows that don't fall into any defined group. |
| `itemsStates` | Object | Per-item state map (e.g. selected/expanded/checked); v-model-friendly via `update:itemsStates`. |
| `hasVirtualization` | Boolean | Default `true`. |
| `hasSkipLink` | Boolean | Adds an in-list skip link for a11y. |
| `loading` / `busy` | Boolean | Distinct. |
| `labelId`, `texts` | — | A11y / i18n. |

Emits: `update:searchString`, `update:itemsStates`, `update:items` (after drag-reorder when `kind="draggable"`), `search`, `loadMore`, `loadChildren`, `selectionChange`, `actionClick`, `selectionActionClick`, `toolbarActionClick`, `emptyStateActionClick`. **Note:** event names are `selectionChange`/`selectionActionClick`/`toolbarActionClick`/`search` — not `*Changed`/`*Clicked`/`searched`.

### LxDataGrid

Tabular grid with paging, sorting, multi-row selection, and column-driven layout. Used for record-style lists.

| Prop | Type | Notes |
|---|---|---|
| `label` / `description` | String | Header text. |
| `items` | Array | Rows. |
| `columnDefinitions` | Array | `[{ id, attributeName, name, kind, size, sortable, ... }]`. Default has `id` (size `s`) and `name` (size `*`). |
| `idAttribute` | String | Default `id`. |
| `badgeDefinitions` | Array | Configurable per-row badges. |
| `actionDefinitions` | Array | Per-row actions. |
| `actionAdditionalParameter` | String | Extra parameter passed alongside `actionId` in `actionClick`. |
| `defaultActionName` | String | Action triggered on row click (default `open`). |
| `selectionActionDefinitions` / `toolbarActionDefinitions` / `emptyStateActionDefinitions` | Array | Bulk / toolbar / empty-state actions. |
| `emptyStateIcon` | String | Empty-state icon. |
| `loading` / `busy` | Boolean | Distinct states. |
| `skeletonRowCount` | Number | Default 10. |
| `scrollable` | Boolean/String | `auto` (default) \| `true` \| `false`. |
| `stickyHeader`, `showHeader`, `showToolbar`, `showStatusbar`, `showAllColumns`, `showItemsCountSelector`, `fullBleed` | Boolean | Layout toggles. |
| `hasSearch`, `searchMode` (`default` \| `compact`), `searchString`, `searchSide` (default `server`) | — | Search. |
| `hasSorting`, `sortingSide` (`client` \| `server`), `sortingIgnoreEmpty`, `sortingMode` (`default` \| `strip`) | — | Sorting. |
| `hasPaging`, `pageCurrent`, `itemsPerPage` (default 20), `itemsTotal` | Number | Paging. |
| `hasSelecting`, `selectionKind` (default `multiple`) | — | Selection — `multiple` = checkboxes, `single` = radios. |
| `clickableRole` | String | `link` (default) \| `button`. |
| `locale` | String | `lv` \| `en`. |
| `texts` | Object | i18n. |

Emits: `actionClick`, `update:searchString`, `search`, `selectPage`, `sortingChange`, `selectionChange`, `itemsPerPageChange`, `selectionActionClick`, `toolbarActionClick`, `emptyStateActionClick`. **Note** the `Change`/`Click` (singular) suffixes — not `*Changed`/`*Clicked`. Slots: `toolbar`, `leftToolbar`, `customResponsiveItem` (`{ item }`), `customResponsiveHeader` (`{ item, expanded }`). **There is no per-cell or per-column slot** — cell rendering is driven entirely by `columnDefinitions` (see below).

#### `columnDefinitions` shape

Each entry: `{ id?, attributeName, attributeDescription?, name?, title?, type?, kind?, size?, options?, dictionary?, sortingTooltips? }`.

| Field | Notes |
|---|---|
| `attributeName` | **Required.** Key on each row. `id` defaults to this. |
| `attributeDescription` | Secondary attribute used for accessible name / `tooltip-text` content. |
| `name` | Visible header label (defaults to `attributeName`). |
| `title` | Header tooltip. |
| `size` | `xs` \| `s` \| `m` \| `l` \| `xl` \| `*` (stretch). Default `*`. |
| `kind` | `default` \| `primary` (treated as the row's main column) \| `clickable` (renders the cell as a router link / button — wires `defaultActionName`) \| `extra` (hidden until `showAllColumns="true"`). |
| `type` | Drives cell rendering — see table below. |
| `options` | Type-specific (e.g. `{ scale: 2 }` for decimal, `{ displayItemsCount: N }` for array, `{ customAttributes }` for person). |
| `dictionary` | For `type: 'state'` — array consumed by the embedded `LxStateDisplay`. Falls back to `options` if `dictionary` is missing. |
| `sortingTooltips` | Override sort-direction tooltips per column. |

#### Column `type` → embedded LX UI component

Cells **never** take custom HTML — set `type` and the grid renders the right LX UI component automatically:

| `type` | Renders | Cell value shape |
|---|---|---|
| `default` (omit) | Plain text. | string / number |
| `tooltip-text` | Text truncated with hover tooltip. | string; tooltip pulled from `attributeDescription` |
| `bool` / `boolean` | Yes/No text (Latvian/English via `texts`). | boolean |
| `number` | `Intl.NumberFormat('lv-LV')` integer. Right-aligned by default; set `options: { justify: 'left' }` to left-align. | number |
| `decimal` / `float` | Formatted number with `options.fractionDigits` decimal places (default 2). | number |
| `date` / `dateTime` / `dateTimeFull` | Formatted via `lxDateUtils`. | ISO date string |
| `array` | Indicator with hover panel listing all items (uses `LxInfoWrapper`). | array; `options.displayItemsCount` controls how many show inline (default 1) |
| `icon` | `LxIcon` with optional label and hover tooltip. See details below. | string (icon name) **or** `{ icon, iconSet, label, title, category }` |
| `flag` / `country` | `LxFlag` for string codes; `LxFlagItemDisplay` for objects. | ISO code string or `{ id, name, ... }` |
| `person` | `LxPersonDisplay`. | person object; pass `options.customAttributes` to remap keys |
| `state` | `LxStateDisplay` keyed by `dictionary` (or `options`). | dictionary-key value |
| `rating` | Read-only `LxRating`. | number |

**`type: 'icon'` value detail:**
- `string` value → renders plain `<LxIcon>` with no label or tooltip.
- `object` value `{ icon, iconSet, label, title, category }`:
  - `icon` — icon name (falls back to `'default'` if omitted but label is present).
  - `iconSet` — optional icon set override.
  - `label` — text shown next to the icon (hidden in `xs`-size columns). Also used as the tooltip fallback.
  - `title` — hover tooltip shown in the `LxInfoWrapper` panel (falls back to `label`).
  - `category` — applied as a CSS class on the icon for color theming (e.g. `'good'`, `'bad'`, `'data'`).
- Sorting order for `icon` columns: icons with labels come first, then icons without labels, then empty/undefined; within each tier sorted alphabetically by icon name.
- Empty value (null / `{}`) renders `—`.

If you genuinely need a non-standard cell, use `kind: 'clickable'` to drop into an action handler, or restructure your row data so one of the supported `type` values fits — **don't reach for `<template>` overrides**, there are none.

#### Filtering and paging rule

The grid **does not host a filter UI** — there is no `hasFilters` prop. There are two layers:

1. **Built-in inline search** — set `hasSearch` and use `searchString` (v-model). **`searchSide` only works as `"server"` (the default).** Client-side search (`searchSide="client"`) is not implemented in the component — setting it suppresses the `@search` event and does nothing. Always use server-side search: listen to `@search`, re-fetch, and pass filtered `items` back.
2. **Rich filter forms** — render a separate `LxFilters` (or `LxFilterBuilder`) **above** the grid. Listen to `@filter`/`@resetFilters`/`@fastFilterClick`, run your query, and feed the result into the grid's `:items`. Do **not** wrap the grid in custom HTML — `LxFilters` is the sanctioned filter container and lives next to the grid in the same view.

Paging follows the same client/server split:

- **Client-side** (small dataset, all rows are already in `items`):
  ```vue
  <LxDataGrid
    :items="rows"
    :column-definitions="cols"
    :has-paging="true"
    v-model:page-current="pageCurrent"
    :items-per-page="20"
    :items-total="rows.length"
    :sorting-side="'client'"
    has-sorting
  />
  ```
  Bind `pageCurrent` with `v-model:page-current`. The grid slices `items` itself. Do **not** set `search-side="client"` — it is unimplemented; if you need client-side search, filter `rows` in your store before passing them to `:items`.

- **Server-side** (paged API):
  ```vue
  <LxDataGrid
    :items="page.items"
    :column-definitions="cols"
    :loading="loading"
    :has-paging="true"
    :page-current="pageCurrent"
    :items-per-page="pageSize"
    :items-total="page.total"
    sorting-side="server"
    search-side="server"
    has-sorting has-search
    @selectPage="(p) => { pageCurrent = p; reload(); }"
    @itemsPerPageChange="(n) => { pageSize = n; pageCurrent = 0; reload(); }"
    @sortingChange="({ column, direction }) => { sort = { column, direction }; reload(); }"
    @search="(q) => { searchString = q; pageCurrent = 0; reload(); }"
  />
  ```

**Rule of thumb:** if the grid will ever hold more than a few hundred rows, switch to server-side and let the API do the work. Mixing modes (e.g. client-side sorting on a server-paged window) gives wrong-looking results — keep `sortingSide` and `searchSide` aligned with where the data is.

### LxListItem (use inside `LxList` custom row slots, or standalone)

| Prop | Type | Notes |
|---|---|---|
| `label` | String | **REQ.** |
| `description`, `tooltip`, `searchString` | — | Display helpers. |
| `icon` / `iconSet` | String | Default icon `next`. |
| `href` | String/Object | Router link or URL. |
| `kind` | String | `default` \| `inactive` \| `tile`. |
| `category` | String | Many: `red`, `blue`, `teal`, `green`, `purple`, `orange`, `yellow`, `bad`, `new`, `good`, `draft`, `error`, `edited`, `signed`, `ongoing`, `waiting`, `deleted`, `disabled`, `inactive`, `finished`, `incomplete`. |
| `clickable` / `selected` / `disabled` / `busy` / `loading` / `active` / `invalid` | Boolean | Distinct states. |
| `actionDefinitions` | Array | Per-item actions. |
| `actionsLayout` | String | `default` \| `vertical`. |

Slots: `customItem`.

### LxDataBlock

Stat/summary block.

| Prop | Type | Notes |
|---|---|---|
| `id` | String/Number | Block id. |
| `modelValue` | Boolean | Expanded state (with `expandable`). |
| `size` | String | `m` \| `l`. |
| `name` / `description` | String | Heading and detail. |
| `icon` / `iconSet` | — | Icon. |
| `uppercase` | Boolean | Default `true` — uppercases the `name`. (Not `forceUppercase`.) |
| `expandable` | Boolean | Allow toggling. |
| `actionDefinitions` | Array | Action buttons. |
| `hasSelecting`, `selectionKind` (`single`/`multiple`), `selected` | — | Selection support. (Use `selectionKind`, not `selectingKind`.) |
| `ariaLabel` | String | Override the announced label. |
| `disabled` / `loading` / `busy` / `invalid` + `invalidationMessage` / `texts` | — | Standard. |

Emits (in addition to v-model on `modelValue` and `selected`): `actionClick`, `selectingClick`.

### LxShell

Drives the entire app chrome. Prop-driven for navigation/branding/accessibility, with a few content slots.

| Prop | Type | Notes |
|---|---|---|
| `mode` | String | `default` \| `public` \| `cover` \| `digives` \| `digives-lite` \| `digimaks` \| `digimaks-lite` \| `latvijalv` \| `full-screen`. |
| `systemNameShort` / `systemName` | String | **REQ.** Brand identity. |
| `systemSubheader` / `systemNameFormatted` / `systemIcon` | String | Optional brand extras. |
| `navItems` / `navItemsSelected` | Array / Object | Nav tree (`{ id, label, icon, to, children, type }`) and the currently-active item. |
| `hideNavBar` | Boolean | Mobile/cover modes. |
| `userInfo` | Object | `{ firstName, lastName, description }`. |
| `hasAvatar` | Boolean | Render initials/avatar. |
| `hasLoginButton` | Boolean | Cover/public modes. |
| `alternativeProfilesInfo` / `selectedAlternativeProfile` | Array / Object | Multi-profile picker. |
| `contextPersonsInfo` / `selectedContextPerson` | Array / Object | "Acting on behalf of" selector. |
| `pageLabel` / `pageDescription` / `pageBackLabel` / `pageBackPath` / `pageBreadcrumbs` / `pageHeaderVisible` / `pageBackButtonVisible` / `pageIndexPath` | — | Page-header overrides (router-driven by default). |
| `notifications` | Array | Notification queue (v-model via `update:notifications`). |
| `hasThemePicker`, `availableThemes` (default `['auto','light','contrast','dark']`) | — | Theme selector. |
| `hasReducedAnimations` / `hasReducedTransparency` / `hasDeviceFonts` / `isTouchSensitive` | Boolean \| null | Accessibility prefs (each with `update:*` emit). |
| `hasLanguagePicker`, `languages`, `selectedLanguage` | — | Language selector. |
| `hasAlerts`, `alerts`, `alertCount`, `alertLevel`, `alertsKind`, `clickSafeAlerts` | — | Alert/notification center. |
| `hasHelp` | Boolean | Help button. |
| `coverImage` / `coverImageDark` / `coverLogo` / `hasCoverLogo` | — | Cover-mode visuals. |
| `environment`, `headerNavDisable`, `headerNavReadOnly`, `navigating` | — | Misc state. |
| `customButtonIcon` / `customButtonBadge` / `customButtonBadgeType` / `customButtonBadgeIcon` / `customButtonOpened` / `customButtonBlink` / `customButtonKind` (`button` \| `dropdown`) | — | Optional custom shell button. |

Emits: `goBack`, `goHome`, `logOut`, `languageChange`, `alertItemClick`, `logInClick`, `alertsClick`, `helpClick`, `contextPersonChange`, `alternativeProfileChange`, `megaMenuShowAllClick`, `idleModalPrimary`, `idleModalSecondary`, `confirmModalClose`, `navClick`, `customButtonClick`, plus `update:*` events: `update:notifications`, `update:selected-language`, `update:selected-context-person`, `update:selected-alternative-profile`, `update:hasReducedAnimations`, `update:hasReducedTransparency`, `update:hasDeviceFonts`, `update:isTouchSensitive`, `update:nav-bar-switch`, `update:selectedMegaMenuItem`, `update:customButtonOpened`, `update:customButtonBlink`, `update:spotlightItemCurrent`. **Note:** events are `*Change` / `*Click` / `*Close` (singular) — not `*Changed` / `*Clicked` / `*Closed`. Accessibility flags are `hasReducedAnimations` / `hasReducedTransparency` / `hasDeviceFonts` / `isTouchSensitive` — there is no `hasAnimations` event.

Slots: `logo`, `footer`, `customButtonPanel`, `customButtonSafePanel`, plus mode-specific named slots.

### LxTile / LxWidget

`LxTile`:

| Prop | Type | Notes |
|---|---|---|
| `label` | String | Tile title. |
| `description` / `title` (tooltip) | String | — |
| `icon` / `iconSet` | String | Icon. |
| `kind` | String | `default` \| `mini`. |
| `to` | Object | Vue Router target. |
| `href` | Object | Plain link fallback. |
| `disabled` / `loading` / `busy` | Boolean | — |
| `badge` / `badgeIcon` / `badgeType` / `badgeTitle` | — | Badge combo. |

`LxWidget`:

| Prop | Type | Notes |
|---|---|---|
| `label` | String | Widget header. |
| `width` / `height` | String | `s` \| `m` \| `l` \| `xl`. |
| `kind` | String | `default` \| `fancy`. |
| `coverImage` | String | Background image URL. |
| `actionDefinitions` | Array | Header buttons. |
| `showHeader` / `showFooter` | Boolean | `true` / `false`. |
| `href` | Object | Make whole widget clickable. |

### LxLoaderView / LxLoader

| Prop | Type | Notes |
|---|---|---|
| `loading` | Boolean | Show loader / show content. |
| `size` | String | `s` \| `l`. |
| `variant` | String | `default` \| `bar`. |
| `kind` | String | `indeterminate` \| `progress`. |
| `modelValue` | Number | Progress 0–1 (for `kind="progress"`). |
| `faked` / `fakedDuration` | — | Run a fake animation. |
| `state` | String | `default` \| `error` \| `success`. |
| `label` / `labelDone` / `description` | String | Visible text. |

### LxErrorPage

| Prop | Type | Notes |
|---|---|---|
| `kind` | String | `400` \| `401` \| `403` \| `404` \| `500` \| `sessionTimeout`. |
| `title` / `description` / `icon` | String | Override defaults. |
| `actionDefinitions` | Array | Action buttons. |
| `texts` | Object | i18n overrides. |

### LxBadge

| Prop | Type | Notes |
|---|---|---|
| `value` | String | Badge text. |
| `icon` / `iconSet` | String | Alternative to text. |
| `tooltip` | String | **REQ.** when `icon` or `value` is present. |
| `type` | String | `auto` \| `number` \| `text`. |

### LxNotification

Renders a queue from a notify store. Don't pass content directly.

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | Array | Notifications: `{ uid, type, title, subtitle, autoCloseSeconds }`. |

v-model only. Note that LxNotification mutates the bound array (drops entries) when auto-close timers fire — keep your store reactive to that.

### LxContentSwitcher / LxTabControl

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | String | Selected id (ContentSwitcher). |
| `items` / `value` | Array | `[{ id, name, icon? }]`. (TabControl uses `value`.) |
| `idAttribute` / `nameAttribute` | String | Map item keys. |
| `kind` | String | `default` \| `icon-only` \| `combo`. |
| `level` (TabControl) | Number | Nesting depth. |

### LxExpander

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | Boolean | Expanded state. |
| `label` / `description` | String | Header text. |
| `region` | Boolean | Marks an ARIA region. |
| `kind` | String | `row` \| `column` (layout direction). |
| `variant` | String | `default` \| `highlighted`. |
| `renderMode` | String | `default` \| `dynamic` (lazy renders default slot only when expanded). |
| `disabled` / `invalid` + `invalidationMessage` | — | Standard. |
| `hasSelectButton`, `selectStatus` (`none`\|`some`\|`all`) | — | Bulk-select header. |
| `hasShortlistReset` | Boolean | If `true`, expander header shows a "reset filters" button — pair with `@resetFilters`. |
| `badge` / `badgeIcon` / `badgeType` / `badgeTitle` | — | Badge combo. |
| `icon` / `iconSet` / `tooltip` / `customClass` / `texts` / `ariaLabel` | — | Standard. |

Emits (in addition to v-model): `selectAll`, `resetFilters`. Slots: default.

### LxSteps / LxWizard

`LxSteps`:

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | String | Current step **id** (not index). |
| `items` | Array | `[{ id, name, description, state }]`, where `state` is `current` \| `complete` \| `invalid`. |
| `idAttribute` / `nameAttribute` / `descriptionAttribute` / `stateAttribute` | String | Map item keys. |
| `kind` | String | `default` \| `compact`. |

`LxWizard`:

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | String | Current step. |
| `items` | Array | Wizard steps. |

Slots: `body` (step content).

### LxToolbar / LxToolbarGroup / LxDropDownMenu

`LxToolbar`:

| Prop | Type | Notes |
|---|---|---|
| `actionDefinitions` | Array | Buttons. (Default `[]`, not required.) |
| `noBorders`, `disabled`, `loading`, `busy` | Boolean | Standard states. |
| `hasSearch` | Boolean | Renders an integrated search field. |
| `searchString` | String | v-model for search; emit `update:searchString`. |
| `searchSide` | String | `client` (default) \| `server`. |
| `searchMode` | String | `default` (default) \| `compact` \| `defaultForce`. |
| `useSearchDebounce` | Boolean | Debounce typed input before emitting. |
| `hasSelectAll` | Boolean | Renders a select-all toggle in the toolbar. |
| `selectionState` | String | `checkbox` (default) \| `checkbox-filled` \| `checkbox-indeterminate` \| `radiobutton-filled` — controls icon shown by select-all. |
| `selectAllSide` | String | `right` (default) \| `left`. |
| `selectAllVariant` | String | `icon-only` (default) \| `default`. |
| `defaultArea` | String | `auto` (default) \| `left` \| `right` — where actions render when not grouped. |
| `texts` | Object | i18n. |

Emits: `actionClick`, `search`, `selectAll`, `deselectAll`, `update:searchString`. Slots: `default`, `leftArea`, `rightArea`, `secondRow`.

`LxDropDownMenu`:

| Prop | Type | Notes |
|---|---|---|
| `actionDefinitions` | Array | Menu items. |
| `groupDefinitions` | Array | Optional grouping for menu items (group headers). |
| `placement` | String | `bottom` (default) \| `top` \| `left` \| `right`. |
| `triggerClick` | String | `left` (default) \| `right` (right-click context menu). |
| `offsetSkid` | String | Pixel offset along the trigger axis. |
| `datePickerType` | String | Internal hint when used as a date-picker trigger. |
| `disabled`, `tabindex`, `customClass`, `texts` | — | Standard. |

Emits: `actionClick`. Slots: default (the trigger), `panel` (custom menu body). Methods: `openMenu()`, `closeMenu()`, `preventClose()`.

### LxCard / LxInfoBox / LxInfoWrapper / LxStateDisplay / LxEmptyState

`LxCard`:

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | Boolean | Flipped state. |
| `kind` | String | `default` \| `clickable` \| `button`. |
| `interactive` | Boolean | 3D tilt on hover. |
| `texts` | Object | Flip-card a11y. |

Slots: default (front), `reverse` (back).

`LxInfoBox`:

| Prop | Type | Notes |
|---|---|---|
| `label` / `description` | String | — |
| `variant` | String | `default` \| `info` \| `warning` \| `error` \| `success`. |
| `kind` | String | `default` \| `clickable` \| `button`. |
| `actionDefinitions` | Array | Buttons. |

`LxInfoWrapper`:

| Prop | Type | Notes |
|---|---|---|
| `placement` | String | `top` \| `bottom` (default) \| `left` \| `right`. |
| `offsetSkid` / `offsetDistance` / `arrowPadding` | String | Popper offset / arrow padding. |
| `arrow` | Boolean | Render the popper arrow. Default `true`. |
| `hover` | Boolean | Open on hover. Default `true`. |
| `focusable` | Boolean | Default `true`. |
| `disabled` | Boolean | Suppress the popper. |
| `locked` | Boolean | Keep the popper open (sticky). |
| `label` / `description` | String | Optional structured tooltip body. |
| `content` | any | Inline content (alternative to slot `panel`). |
| `customRole` | String | ARIA role override. |
| `texts` | Object | i18n. |

Slots: default (trigger content), `panel` (tooltip body — preferred over `content` for rich markup).

`LxStateDisplay`:

| Prop | Type | Notes |
|---|---|---|
| `value` | Object/String/Number | Current status value. |
| `dictionary` | Array | **REQ.** Status mapping (`{ value, name, icon, color, ... }`). |

`LxEmptyState`:

| Prop | Type | Notes |
|---|---|---|
| `label` / `description` / `icon` | String | Display. |
| `actionDefinitions` | Array | Buttons. |
| `announce` | Boolean | Default `true` — announces the state to assistive tech via aria-live. |
| `texts` | Object | i18n. |

Emits: `actionClick`. Slots: default (custom empty-state body — replaces icon+label+description).

### LxLogoDisplay / LxRichTextDisplay

`LxLogoDisplay`:

| Prop | Type | Notes |
|---|---|---|
| `value` | String | Logo code. Defaults to `dativa` (was `zzdats` pre-2.1). Pass explicitly if you need the old default. |
| `kind` | String | `default` (16:9) \| `square` (1:1). |
| `size` | String | `auto` (default) \| `s` \| `m` \| `l`. |
| `theme` | String | `auto` (default) \| `light` \| `dark`. |

`LxRichTextDisplay`:

| Prop | Type | Notes |
|---|---|---|
| `value` | String | HTML content. **Sanitization runs unconditionally** via `v-clean-html` — there is no `sanitize` opt-out prop. |
| `loading` | Boolean | Show skeleton while content is being fetched. |
| `id` | String | Element id. |

### LxAccessibilitySettings

Renders the full accessibility settings UI (theme, transparency, animations, system fonts, touch mode). It reads/writes the shell-provided accessibility context internally — **do not** wire model bindings or emits, just drop it into the page.

| Prop | Type | Notes |
|---|---|---|
| `headingTag` | String | `div` (default) \| `h1` \| `h2` \| `h3` \| `h4` \| `h5` \| `h6`. Section heading element. |
| `headingLevel` | Number | `1`–`6`, default `2`. ARIA heading level. |
| `texts` | Object | Override the (Latvian) defaults for every label — `appearance`, `animations`, `fonts`, `interactions`, `reset`, `themeTitle`, `themeAuto`/`Light`/`Dark`/`Contrast` (+ `*Description`), `transparencyTitle`/`Description`, `animationsTitle`/`Description`, `fontsTitle`/`Description`, `touchModeTitle`/`Description`, link labels, etc. |

Emits: none. Slots: none. **The component uses `inject` to read accessibility state from `LxShell` — it must render somewhere inside an `LxShell` tree (typically via `<router-view>`).**

### LxTooltip

Lightweight tooltip wrapper. Body is **prop-driven** (not slotted) — pass `value` or the `label` + `description` pair. For a popper-style hover panel with arbitrary HTML, use `LxInfoWrapper` (which has a `panel` slot).

| Prop | Type | Notes |
|---|---|---|
| `id` | String | AG. |
| `value` | String | Tooltip text. |
| `label` / `description` | String | Optional structured body (used when `value` is null). |
| `disabled` | Boolean | Suppress the tooltip. |
| `customRole` | String | ARIA role override. |

Slots: default (the trigger element). No `panel` slot — use `LxInfoWrapper` if you need rich content.

### LxRating

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | Number | Current rating. Default `0`. |
| `kind` | String | `5stars` (currently the only supported kind). |
| `variant` | String | `default` \| `colorful`. |
| `readOnly` / `disabled` / `invalid` + `invalidationMessage` | — | Standard. |
| `focusable` | Boolean | Default `true`. |
| `texts` | Object | i18n. |

Methods: `focus()`, `scrollIntoView()`.

### LxAppendableListSimple (forms)

Lightweight repeating-row list — for the full form-builder flavour use `LxAppendableList`.

| Prop | Type | Notes |
|---|---|---|
| `modelValue` | Array | Items in the list. |
| `columnCount` | Number | Default `1`. |
| `kind` | String | `default` \| `compact`. |
| `requiredMode` | String | `optional` \| `required` \| `required-asterisk`. |
| `canAddItems` | Boolean | Default `true`. |
| `readOnly` | Boolean | Read-only display. |
| `texts` | Object | i18n. |

Slots: `customItem` — scoped slot receives `{ item, index }` for rendering each row.

---

## Common LX UI Components Quick Reference

| Component | Import | Purpose / key props |
|-----------|--------|---------------------|
| `LxShell` | `@dativa-lv/lx-ui` | App chrome — `mode`, `systemName(Short)`, `navItems`, `userInfo`. Emits `goBack`/`goHome`/`logOut`. |
| `LxAccessibilitySettings` | `@dativa-lv/lx-ui` | Drop-in **accessibility settings page** (theme/transparency/animations/fonts/touch). Reads state via `inject` from LxShell — render `<LxAccessibilitySettings />` inside an LxShell-wrapped route, no other props needed. Optional: `headingTag`, `headingLevel`, `texts`. |
| `LxButton` | `@dativa-lv/lx-ui` | Button — REQUIRED `label` (no slot). `kind`: `default`/`primary`/`secondary`/`tertiary`/`ghost`/`menuitem`. `variant`: `default`/`icon-only`. `destructive`, `disabled`/`loading`/`busy`, `icon`+`iconSet`, `href`. |
| `LxIcon` | `@dativa-lv/lx-ui` | Icon — `value` prop (no slot). `iconSet`: `cds`/`material`/`brand`/`phosphor`. `variant`, `animation`, `meaningful`. |
| `LxForm` | `@dativa-lv/lx-ui` | Form container — `columnCount`, `kind` (`default`/`compact`/`stripped`), `indexType` (`tabs`/`expanders`), `requiredMode`, `actionDefinitions`. |
| `LxAppendableList` / `LxAppendableListSimple` | `@dativa-lv/lx-ui` | Repeating row list inside forms. Simple variant: v-model = items array, `columnCount`, `kind` (`default`/`compact`), `requiredMode`, `canAddItems`; scoped slot `customItem({ item, index })`. |
| `LxRow` | `@dativa-lv/lx-ui` | Form row — `label`, `description`, `required`, `columnSpan` (`1`/`2`/`3`/`4`/`6`/`8`), `orientation`, `inputId`. Slot: default (the input). |
| `LxSection` | `@dativa-lv/lx-ui` | Form/content section — `label`, `columnCount`, `requiredMode`, `actionDefinitions`, `expanderRenderMode`. |
| `LxList` | `@dativa-lv/lx-ui` | Vertical list (cards/tiles/tree). `items`, `kind` (`default`/`draggable`/`treelist`), `groupDefinitions`, attribute-mapping props (`idAttribute` etc.), `mode` (`client`/`server`), `hasSearch`, `hasSelecting`, `selectionKind`, `showLoadMore`, `hasVirtualization`. **No `columnDefinitions`/paging/sorting** — use `LxDataGrid` for those. |
| `LxListItem` | `@dativa-lv/lx-ui` | List row — `label`, `description`, `kind` (`default`/`inactive`/`tile`), `category` (status colors), `actionDefinitions`. |
| `LxTile` | `@dativa-lv/lx-ui` | Dashboard tile — `label`, `description`, `icon`+`iconSet`, `kind` (`default`/`mini`), `to` (router), badge combo. |
| `LxWidget` | `@dativa-lv/lx-ui` | Dashboard widget — `label`, `width`/`height` (`s`/`m`/`l`/`xl`), `kind` (`default`/`fancy`), `coverImage`, `actionDefinitions`. |
| `LxModal` | `@dativa-lv/lx-ui` | Modal — `label`, `size` (`default`/`s`/`m`/`l`/`xl`), `actionDefinitions`. Open via `ref.open()` / `ref.close()`. Emits `close`, `actionClick` (no `primaryAction`/`secondaryAction`/`closed`). |
| `LxModalForm` | `@dativa-lv/lx-ui` | Modal that wraps a form — adds `columnCount`, `requiredMode`, `index(Type)` over LxModal. Emits `close`, `actionClick`, `update:index`. |
| `LxDialog` | `@dativa-lv/lx-ui` | Confirmation/alert dialog — `label`, `description`, `kind` (`default`/`question`/`info`/`warning`/`error`/`success`/`custom`), `actionDefinitions`. Ref-controlled; emits `close`, `actionClick`. |
| `LxLoaderView` | `@dativa-lv/lx-ui` | Loading wrapper — `loading`, `size` (`s`/`l`), `variant` (`default`/`bar`), `kind` (`indeterminate`/`progress`), `state`. |
| `LxLoader` | `@dativa-lv/lx-ui` | Inline loader — same shape as LxLoaderView; required `loading`. |
| `LxErrorPage` | `@dativa-lv/lx-ui` | Error page — `kind`: `400`/`401`/`403`/`404`/`500`/`sessionTimeout`. `title`/`description`/`icon` overrides, `actionDefinitions`. |
| `LxEmptyState` | `@dativa-lv/lx-ui` | Empty list/grid state — `label`, `description`, `icon`, `actionDefinitions`, `announce`. |
| `LxSiteMap` | `@dativa-lv/lx-ui` | Footer site map. |
| `LxRichTextDisplay` | `@dativa-lv/lx-ui` | HTML renderer (always sanitised) — `value`, `loading`, `id`. |
| `LxDataGrid` | `@dativa-lv/lx-ui` | Tabular grid — `columnDefinitions`, `items`, `hasPaging`+`pageCurrent`/`itemsPerPage`/`itemsTotal`, `hasSorting`+`sortingSide`, `hasSelecting`+`selectionKind`, `hasSearch`, `actionDefinitions`. Emits `selectPage`, `sortingChange`, `selectionChange`, `itemsPerPageChange`, `actionClick`, `search`. **Distinct from `LxList` — DataGrid is the table-style surface.** |
| `LxDataBlock` | `@dativa-lv/lx-ui` | Stat/summary block — `name`, `description`, `size` (`m`/`l`), `expandable`, `hasSelecting`+`selectionKind`, `uppercase` (default `true`). |
| `LxDataVisualizer` | `@dativa-lv/lx-ui` | Bar chart / Latvia map with a built-in table view (graph ↔ table switcher using `LxContentSwitcher`). `kind`: `bars-horizontal` (default) / `bars-vertical` / `latvia` / `latvia2024`. `items` array (each item has `id`, `name`, `value`, optional `color`). `thresholds` array (`{ min, max, color, excludes? }`). `targets` (array of numbers for target lines). `showValues`: `default` / `always` / `never`. `showLegend`: boolean. `mode`: `default` / `compact`. `maxValue`: override max axis value. Attribute remapping: `idAttribute`, `nameAttribute`, `colorAttribute` (defaults `color`), `valueAttribute` (defaults `value`). Emits `@click` with item id. The built-in table view renders an `LxDataGrid` internally — columns are fixed (name, value, color-coded icon); you do **not** pass `columnDefinitions`. |
| `LxExpander` | `@dativa-lv/lx-ui` | Collapsible — v-model, `label`, `kind` (`row`/`column`), `variant` (`default`/`highlighted`), `renderMode: dynamic` for lazy. |
| `LxTabControl` | `@dativa-lv/lx-ui` | Tab navigation — `value` (items), `kind` (`default`/`icon-only`/`combo`), `level`. |
| `LxContentSwitcher` | `@dativa-lv/lx-ui` | Segmented control — v-model id, `items`, `kind` (`default`/`icon-only`/`combo`). |
| `LxSteps` | `@dativa-lv/lx-ui` | Step indicator — v-model = current step **id**, `items` with `state` (`current`/`complete`/`invalid`). |
| `LxWizard` | `@dativa-lv/lx-ui` | Multi-step wizard — v-model + `items`; slot `body`. |
| `LxDropdown` | `@dativa-lv/lx-ui` | Select — v-model, REQUIRED `items`, `idAttribute`/`nameAttribute`, `kind` (`default`/`native`). No `variant` prop, no `@change` event — bind to v-model. |
| `LxAutoComplete` | `@dativa-lv/lx-ui` | Async select — `items` (Array OR `async fn(query)`), `selectionKind` (`single`/`multiple`), `queryDebounce`, `searchAttributes`, `hasDetails`+`detailMode`. |
| `LxValuePicker` | `@dativa-lv/lx-ui` | Variant-rich picker — `selectionKind` (`single`/`multiple`), `variant` (`default`/`dropdown`/`tiles`/`tags`/`rotator`/`indicator`/`horizontal`, plus `*-custom` siblings that delegate rendering to the default scoped slot), `nullable`, `alwaysAsArray`. |
| `LxDateTimePicker` | `@dativa-lv/lx-ui` | Date / time / datetime picker. |
| `LxDayInput` | `@dativa-lv/lx-ui` | Days/duration input — v-model, `kind` (`label`/`icon`). |
| `LxTextInput` | `@dativa-lv/lx-ui` | Text input — `kind` (`default`/`password`/`search`/`phone`), `mask` (`integer`/`decimal`/`personCodeLv`/`email`/`numeric`/`custom`/...), `customMaskValue`, `scale`, `signed`, `uppercase`. |
| `LxTextArea` | `@dativa-lv/lx-ui` | Multi-line — `rows`, `dynamicHeight`. |
| `LxCheckbox` | `@dativa-lv/lx-ui` | Checkbox — v-model REQUIRED, `label`, `groupId`. |
| `LxRadioButton` | `@dativa-lv/lx-ui` | Radio — same shape as Checkbox + `value`. |
| `LxToggle` | `@dativa-lv/lx-ui` | Toggle — TRI-STATE (`true`/`false`/`null`); `size` (`s`/`m`, default `m`), `label`, `role` (default `switch`); slots `on`/`off`/`indeterminate`. |
| `LxRating` | `@dativa-lv/lx-ui` | Star rating — v-model (Number); `kind` (`5stars`), `variant` (`default`/`colorful`), `focusable`. |
| `LxBadge` | `@dativa-lv/lx-ui` | Badge — `value` or `icon`, `tooltip` REQUIRED, `type` (`auto`/`number`/`text`). |
| `LxTooltip` | `@dativa-lv/lx-ui` | Tooltip wrapper — `value` OR `label`+`description` (props, **not slots**); only the default slot wraps the trigger. For rich/HTML tooltip body use `LxInfoWrapper`. |
| `LxNotification` | `@dativa-lv/lx-ui` | Notification queue — v-model = array. Driven by `LxNotifyStore`; mutates the array when auto-close timers fire. |
| `LxToolbar` / `LxToolbarGroup` | `@dativa-lv/lx-ui` | Action bars — `actionDefinitions`, `hasSearch`+`searchString`/`searchSide`/`searchMode`, `hasSelectAll`+`selectionState`, `selectAllSide`, `defaultArea`. Emits `actionClick`, `search`, `selectAll`, `deselectAll`, `update:searchString`. |
| `LxDropDownMenu` | `@dativa-lv/lx-ui` | Context menu — `actionDefinitions`, `groupDefinitions`, `placement`, `triggerClick` (`left`/`right`), `texts`. Methods: `openMenu()`/`closeMenu()`/`preventClose()`. |
| `LxInfoBox` | `@dativa-lv/lx-ui` | Inline info/alert — `label`, `description`, `variant` (`info`/`warning`/`error`/`success`), `kind` (`default`/`clickable`/`button`). |
| `LxInfoWrapper` | `@dativa-lv/lx-ui` | Popper-style tooltip — `placement`, `hover`, `arrow`, `disabled`, `locked`, `label`/`description`, `content`. Slots default (trigger) and `panel` (rich body). |
| `LxStateDisplay` | `@dativa-lv/lx-ui` | Status pill — `value` + `dictionary`. |
| `LxCard` | `@dativa-lv/lx-ui` | Flip card — `modelValue`, `kind` (`default`/`clickable`/`button`), `interactive`; slots default/`reverse`. |
| `LxLogoDisplay` | `@dativa-lv/lx-ui` | Logo — `value` (defaults to `dativa`; was `zzdats` pre-2.1 — set explicitly if you need the old default), `kind` (`default` 16:9 / `square` 1:1), `size` (`auto`/`s`/`m`/`l`), `theme` (`auto`/`light`/`dark`). |

### LX UI utilities

| Utility | Import | Purpose |
|---------|--------|---------|
| `createLx` | `@dativa-lv/lx-ui` | Vue plugin installer |
| `lxFlowUtils` | `@dativa-lv/lx-ui` | Router guard helpers (`beforeEach`, `afterEach`, `removeFocus`) |
| `lxDateUtils` | `@dativa-lv/lx-ui` | Date formatting utilities |
| `lxVersionCheckUtils` | `@dativa-lv/lx-ui` | App version change monitoring |
| `LxAuthStore` | `@dativa-lv/lx-ui` | Auth store factory |
| `LxAuthService` | `@dativa-lv/lx-ui` | Auth service for LxAuthStore |
| `LxAppStore` | `@dativa-lv/lx-ui` | App state store factory |
| `LxNotifyStore` | `@dativa-lv/lx-ui` | Notification store factory |
| `LxConfirmStore` | `@dativa-lv/lx-ui` | Confirmation dialog store factory |
| `LxViewStore` | `@dativa-lv/lx-ui` | View/page state store factory |

### Vite plugins from LX UI

| Plugin | Import | Purpose |
|--------|--------|---------|
| `lxViteSecureHeadersPlugin` | `@dativa-lv/lx-ui/vite` | Generates security headers (CSP, referrer policy, etc.) |
| `lxVitePortalVersionPlugin` | `@dativa-lv/lx-ui/vite` | Generates `version.json` for version change detection |

---

## Summary of Patterns

1. **All UI is composed from LX UI components** — no raw HTML input fields, buttons, modals, etc.
2. **State management** follows the pattern: LX UI store factories for framework concerns (app, auth, notify, confirm, view) + custom Pinia stores for domain data.
3. **API calls** go through service files that use Axios factory functions.
4. **Language switching** is handled by `LxShell`'s built-in language picker (`hasLanguagePicker`, `languages`, `v-model:selected-language`) — **not** via URL locale prefixes. The `useLanguageSwitcher` hook in `hooks/language.js` persists the locale choice and switches the `vue-i18n` locale in response to `@languageChange`.
5. **The LxShell** is the single layout component — everything else is rendered inside it via `<router-view>`.
6. **Error handling** uses the `useErrors` composable to extract and route errors, combined with `LxNotifyStore` for user notifications and `LxErrorPage` for full-page errors.
7. **Theming** is handled entirely by LX UI CSS — import the right theme files and reference CSS custom properties only.
8. **Dependency management follows project context** — use `bun` for new projects; in existing projects with `pnpm-lock.yaml`, use `pnpm` (`pnpm install`, `pnpm add`, `pnpm dev`, `pnpm build`, etc.).