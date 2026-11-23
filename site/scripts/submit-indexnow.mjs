import { readFileSync, existsSync } from "node:fs";

const key = "e8a4c9f1b2064d7fa3581e90c7b42d61";
const host = "memtwig.com";
const keyLocation = `https://${host}/${key}.txt`;

const sm = ["dist/sitemap.xml", "public/sitemap.xml"].find((p) => existsSync(p));
const urlList = sm
  ? [...readFileSync(sm, "utf8").matchAll(/<loc>([^<]+)<\/loc>/g)].map((m) => m[1])
  : [`https://${host}/`, `https://${host}/llms.txt`, `https://${host}/llms-full.txt`];

const response = await fetch("https://api.indexnow.org/indexnow", {
  method: "POST",
  headers: { "content-type": "application/json; charset=utf-8" },
  body: JSON.stringify({ host, key, keyLocation, urlList }),
});

if (!response.ok) {
  const body = await response.text().catch(() => "");
  throw new Error(`IndexNow failed: ${response.status} ${response.statusText} ${body}`);
}

console.log(`IndexNow accepted ${urlList.length} URL(s) for ${host} (HTTP ${response.status}).`);
