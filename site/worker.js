const EMAIL_RE = /^[^\s@]+@[^\s@]+\.[^\s@]{2,}$/;
const ALLOWED_PLANS = new Set(["pro", "team", "agency"]);
const ALLOWED_SIZES = new Set(["solo", "2-10", "10+"]);

function json(data, status = 200, extraHeaders = {}) {
  return new Response(JSON.stringify(data), {
    status,
    headers: {
      "Content-Type": "application/json; charset=utf-8",
      "Cache-Control": "no-store",
      ...extraHeaders,
    },
  });
}

export default {
  async fetch(request, env) {
    const url = new URL(request.url);

    if (url.pathname === "/api/waitlist") {
      if (request.method !== "POST") {
        return json({ ok: false, error: "Method not allowed." }, 405, { Allow: "POST" });
      }

      let raw = "";
      try {
        raw = await request.text();
      } catch {
        return json({ ok: false, error: "Invalid request payload." }, 400);
      }
      if (!raw || raw.length > 4096) {
        return json({ ok: false, error: "Invalid request payload." }, 400);
      }

      let body;
      try {
        body = JSON.parse(raw);
      } catch {
        return json({ ok: false, error: "Invalid request payload." }, 400);
      }
      if (!body || typeof body !== "object") {
        return json({ ok: false, error: "Invalid request payload." }, 400);
      }

      if (typeof body.website === "string" && body.website.trim().length > 0) {
        return json({ ok: true, status: "joined" }, 200);
      }

      const email = typeof body.email === "string" ? body.email.trim().toLowerCase() : "";
      if (!email || email.length > 254 || !EMAIL_RE.test(email)) {
        return json({ ok: false, error: "Enter a valid email address." }, 400);
      }

      const plan =
        typeof body.plan === "string" && ALLOWED_PLANS.has(body.plan.trim().toLowerCase())
          ? body.plan.trim().toLowerCase()
          : "pro";
      const teamSize =
        typeof body.teamSize === "string" && ALLOWED_SIZES.has(body.teamSize.trim())
          ? body.teamSize.trim()
          : "solo";
      const primaryAgent =
        typeof body.primaryAgent === "string" ? body.primaryAgent.trim().slice(0, 120) : "";

      const key = `lead:${email}`;
      const now = new Date().toISOString();

      try {
        const existing = await env.WAITLIST.get(key, "json");
        const record = {
          email,
          plan,
          teamSize,
          primaryAgent,
          source: "memtwig.com/cloud",
          createdAt: existing?.createdAt || now,
          updatedAt: now,
        };
        await env.WAITLIST.put(key, JSON.stringify(record));
        return json({ ok: true, status: existing ? "already_joined" : "joined" }, 200);
      } catch (err) {
        console.error(
          "waitlist_kv_error",
          err?.name || "Error",
          String(err?.message || "").slice(0, 60),
        );
        return json({ ok: false, error: "Temporarily unavailable. Please retry." }, 503, {
          "Retry-After": "60",
        });
      }
    }

    if (url.pathname.startsWith("/api/")) {
      return json({ ok: false, error: "Not found." }, 404);
    }

    return env.ASSETS.fetch(request);
  },
};
