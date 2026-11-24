// Renders og.html to public/og-image.png (1200x630).
// Needs Playwright with Chromium: `npx -y -p playwright@1 node scripts/og/shot.mjs` from site/,
// or PLAYWRIGHT=/path/to/node_modules/playwright/index.mjs node scripts/og/shot.mjs
// After regenerating, bump ?v=N on og:image in index.astro and ArticleLayout.astro.
import { fileURLToPath, pathToFileURL } from "node:url";
import path from "node:path";
const { chromium } = await import(process.env.PLAYWRIGHT || "playwright");
const here = path.dirname(fileURLToPath(import.meta.url));
const b = await chromium.launch();
const p = await b.newPage({ viewport: { width: 1200, height: 630 } });
await p.goto(pathToFileURL(path.join(here, "og.html")).href);
await p.evaluate(() => document.fonts.ready);
await p.waitForTimeout(300);
await p.screenshot({ path: path.join(here, "../../public/og-image.png") });
await b.close();
