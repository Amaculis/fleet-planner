// Prebuild step for `npm run build:demo` only. The real build doesn't need this —
// internal/http/server.go serves these same files at /lx-fonts/* from
// web/static/lx-fonts/ (see the Dockerfile's own copy step), which the CSS's
// absolute url(/lx-fonts/...) references match exactly since the real app is
// deployed at its domain's root. GitHub Pages serves this project under a
// subpath, so the demo build needs its own copy under Vite's public/ (copied
// verbatim to the output root) plus the base-path rewrite in vite.config.mjs.
import { copyFileSync, mkdirSync } from "fs";
import { fileURLToPath } from "url";
import { dirname, join } from "path";

const __dirname = dirname(fileURLToPath(import.meta.url));
const srcDir = join(__dirname, "..", "node_modules", "@dativa-lv", "lx-ui", "dist", "lx-fonts");
const destDir = join(__dirname, "..", "public", "lx-fonts");

const files = [
  "IBMPlexMono-Italic.ttf",
  "IBMPlexMono-Light.ttf",
  "IBMPlexMono-LightItalic.ttf",
  "IBMPlexMono-Regular.ttf",
  "IBMPlexMono-SemiBold.ttf",
  "IBMPlexMono-SemiBoldItalic.ttf",
  "IBMPlexSansVar-Italic.ttf",
  "IBMPlexSansVar.ttf",
];

mkdirSync(destDir, { recursive: true });
for (const f of files) copyFileSync(join(srcDir, f), join(destDir, f));
console.log(`copy-demo-fonts: copied ${files.length} font files into public/lx-fonts/`);
