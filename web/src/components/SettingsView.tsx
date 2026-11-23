import * as React from "react";
import { Check, PlugsConnected, Plus, Trash } from "@phosphor-icons/react";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";

type EmbeddingsCfg = {
  provider: string;
  model: string;
  endpoint: string;
  api_key_env: string;
  account_id: string;
  dimensions: number;
  timeout_ms: number;
  cosine_threshold: number;
};

type LLMCfg = {
  provider: string;
  model: string;
  endpoint: string;
  api_key_env: string;
  timeout_ms: number;
};

type VaultRow = {
  name: string;
  path: string;
  kind: string;
  default_write: string;
  match_paths?: string[];
  match_git_remotes?: string[];
  always_include?: string[];
};

type HarnessRow = {
  id: string;
  label: string;
  detected: boolean;
  marker: string;
};

const EMB_PRESETS: Record<string, { model: string; env: string; endpoint: string }> = {
  none: { model: "", env: "", endpoint: "" },
  ollama: { model: "bge-m3", env: "", endpoint: "http://127.0.0.1:11434/api/embeddings" },
  openrouter: { model: "baai/bge-m3", env: "OPENROUTER_API_KEY", endpoint: "" },
  deepinfra: { model: "BAAI/bge-m3", env: "DEEPINFRA_API_KEY", endpoint: "" },
  openai: { model: "text-embedding-3-small", env: "OPENAI_API_KEY", endpoint: "" },
  cloudflare: { model: "@cf/baai/bge-m3", env: "CLOUDFLARE_API_TOKEN", endpoint: "" },
};

const LLM_PRESETS: Record<string, { model: string; env: string }> = {
  none: { model: "", env: "" },
  groq: { model: "llama-3.3-70b-versatile", env: "GROQ_API_KEY" },
  openrouter: { model: "openai/gpt-4o-mini", env: "OPENROUTER_API_KEY" },
  openai: { model: "gpt-4o-mini", env: "OPENAI_API_KEY" },
  ollama: { model: "llama3.2", env: "" },
};

function splitCSV(s: string): string[] {
  return s
    .split(",")
    .map((x) => x.trim())
    .filter(Boolean);
}

export function SettingsView({
  onChanged,
  flash,
}: {
  onChanged: () => Promise<void>;
  flash: (msg: string) => void;
}) {
  const [configPath, setConfigPath] = React.useState("");
  const [emb, setEmb] = React.useState<EmbeddingsCfg>({
    provider: "none",
    model: "bge-m3",
    endpoint: "",
    api_key_env: "",
    account_id: "",
    dimensions: 1024,
    timeout_ms: 3000,
    cosine_threshold: 0.45,
  });
  const [llm, setLLM] = React.useState<LLMCfg>({
    provider: "none",
    model: "",
    endpoint: "",
    api_key_env: "",
    timeout_ms: 15000,
  });
  const [envStatus, setEnvStatus] = React.useState<{ embeddings: boolean; llm: boolean }>({
    embeddings: false,
    llm: false,
  });
  const [vaults, setVaults] = React.useState<VaultRow[]>([]);
  const [harnesses, setHarnesses] = React.useState<HarnessRow[]>([]);
  const [busy, setBusy] = React.useState(false);

  const [vName, setVName] = React.useState("");
  const [vPath, setVPath] = React.useState("");
  const [vKind, setVKind] = React.useState("org");
  const [vWrite, setVWrite] = React.useState("propose");
  const [vMatchPaths, setVMatchPaths] = React.useState("");
  const [vMatchRemotes, setVMatchRemotes] = React.useState("");
  const [vAlways, setVAlways] = React.useState("");

  const loadSettings = React.useCallback(async () => {
    const [cfgRes, hRes] = await Promise.all([fetch("/api/v1/config"), fetch("/api/v1/harnesses")]);
    if (cfgRes.ok) {
      const d = await cfgRes.json();
      setConfigPath(d.config_path ?? "");
      if (d.embeddings) setEmb(d.embeddings);
      if (d.llm) setLLM(d.llm);
      if (d.env_status) setEnvStatus(d.env_status);
      setVaults(d.vaults ?? []);
    }
    if (hRes.ok) {
      const d = await hRes.json();
      setHarnesses(d.harnesses ?? []);
    }
  }, []);

  React.useEffect(() => {
    void loadSettings();
  }, [loadSettings]);

  const saveProviders = async (e: React.FormEvent) => {
    e.preventDefault();
    setBusy(true);
    try {
      const res = await fetch("/api/v1/config", {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ embeddings: emb, llm }),
      });
      if (res.ok) {
        const d = await res.json();
        if (d.env_status) setEnvStatus(d.env_status);
        flash("Providers saved & index refreshed");
        await onChanged();
      } else {
        flash("Failed to save providers");
      }
    } finally {
      setBusy(false);
    }
  };

  const saveVault = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!vName.trim() || !vPath.trim()) return;
    setBusy(true);
    try {
      const res = await fetch("/api/v1/vaults", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: vName.trim(),
          path: vPath.trim(),
          kind: vKind,
          default_write: vWrite,
          match_paths: splitCSV(vMatchPaths),
          match_git_remotes: splitCSV(vMatchRemotes),
          always_include: splitCSV(vAlways),
          init_if_missing: true,
        }),
      });
      if (res.ok) {
        flash(`Vault "${vName.trim()}" saved`);
        setVName("");
        setVPath("");
        setVMatchPaths("");
        setVMatchRemotes("");
        setVAlways("");
        await loadSettings();
        await onChanged();
      }
    } finally {
      setBusy(false);
    }
  };

  const removeVault = async (name: string) => {
    setBusy(true);
    try {
      const res = await fetch(`/api/v1/vaults/${encodeURIComponent(name)}`, { method: "DELETE" });
      if (res.ok) {
        flash(`Removed vault "${name}" from config`);
        await loadSettings();
        await onChanged();
      }
    } finally {
      setBusy(false);
    }
  };

  const connectHarness = async (id: string, label: string) => {
    setBusy(true);
    try {
      const res = await fetch(`/api/v1/harnesses/${encodeURIComponent(id)}/install`, { method: "POST" });
      if (res.ok) {
        flash(`Connected ${label}`);
        await loadSettings();
        await onChanged();
      }
    } finally {
      setBusy(false);
    }
  };

  return (
    <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-[#0a0d0e]">
      <div className="flex h-12 shrink-0 items-center justify-between gap-4 border-b border-border bg-[#0d1214] px-5 md:px-7">
        <span className="shrink-0 text-[13px] font-medium text-foreground">Settings</span>
        <span className="truncate text-xs text-muted-foreground">{configPath || "config.toml"}</span>
      </div>

      <div className="flex-1 overflow-y-auto p-5 md:p-8">
        <div className="mx-auto max-w-[900px] space-y-8">
          <form onSubmit={saveProviders} className="space-y-4">
            <div className="flex items-center justify-between gap-3">
              <div>
                <h2 className="text-sm font-semibold text-foreground">Providers (Embeddings &amp; LLM Curator)</h2>
                <p className="mt-0.5 text-xs text-muted-foreground">
                  Keys are read from environment variables and never written into config.toml.
                </p>
              </div>
              <Button type="submit" size="sm" disabled={busy}>
                <Check size={14} />
                Save &amp; Reindex
              </Button>
            </div>

            <div className="grid gap-4 md:grid-cols-2">
              <div className="space-y-3 rounded-md border border-border bg-[#0f1416] p-4">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                    Embeddings (Hybrid RRF)
                  </span>
                  {emb.api_key_env ? (
                    <Badge variant={envStatus.embeddings ? "default" : "warn"}>
                      {envStatus.embeddings ? `${emb.api_key_env} set` : `${emb.api_key_env} missing`}
                    </Badge>
                  ) : (
                    <Badge variant="neutral">{emb.provider === "none" ? "BM25 only" : "local"}</Badge>
                  )}
                </div>

                <div className="grid grid-cols-2 gap-2.5">
                  <label className="space-y-1 text-xs text-muted-foreground">
                    <span>Provider</span>
                    <Select
                      value={emb.provider}
                      onChange={(e) => {
                        const p = e.target.value;
                        const pre = EMB_PRESETS[p] ?? EMB_PRESETS.none;
                        setEmb((curr) => ({
                          ...curr,
                          provider: p,
                          model: pre.model,
                          api_key_env: pre.env,
                          endpoint: pre.endpoint,
                        }));
                      }}
                    >
                      <option value="none">none (FTS5 BM25 only)</option>
                      <option value="ollama">ollama (local)</option>
                      <option value="openrouter">openrouter (bge-m3)</option>
                      <option value="deepinfra">deepinfra (bge-m3)</option>
                      <option value="openai">openai</option>
                      <option value="cloudflare">cloudflare</option>
                    </Select>
                  </label>

                  <label className="space-y-1 text-xs text-muted-foreground">
                    <span>Dimensions</span>
                    <Input
                      type="number"
                      value={emb.dimensions}
                      onChange={(e) => setEmb((c) => ({ ...c, dimensions: Number(e.target.value) || 1024 }))}
                    />
                  </label>
                </div>

                <label className="block space-y-1 text-xs text-muted-foreground">
                  <span>Model</span>
                  <Input
                    value={emb.model}
                    onChange={(e) => setEmb((c) => ({ ...c, model: e.target.value }))}
                    placeholder="baai/bge-m3"
                  />
                </label>

                <div className="grid grid-cols-2 gap-2.5">
                  <label className="space-y-1 text-xs text-muted-foreground">
                    <span>API key env var</span>
                    <Input
                      value={emb.api_key_env}
                      onChange={(e) => setEmb((c) => ({ ...c, api_key_env: e.target.value }))}
                      placeholder="OPENROUTER_API_KEY"
                    />
                  </label>
                  <label className="space-y-1 text-xs text-muted-foreground">
                    <span>Endpoint (optional)</span>
                    <Input
                      value={emb.endpoint}
                      onChange={(e) => setEmb((c) => ({ ...c, endpoint: e.target.value }))}
                      placeholder="auto"
                    />
                  </label>
                </div>
              </div>

              <div className="space-y-3 rounded-md border border-border bg-[#0f1416] p-4">
                <div className="flex items-center justify-between gap-2">
                  <span className="text-xs font-semibold uppercase tracking-wider text-muted-foreground">
                    LLM Curator (curate --llm)
                  </span>
                  {llm.api_key_env ? (
                    <Badge variant={envStatus.llm ? "default" : "warn"}>
                      {envStatus.llm ? `${llm.api_key_env} set` : `${llm.api_key_env} missing`}
                    </Badge>
                  ) : (
                    <Badge variant="neutral">{llm.provider === "none" ? "disabled" : "local"}</Badge>
                  )}
                </div>

                <label className="block space-y-1 text-xs text-muted-foreground">
                  <span>Provider</span>
                  <Select
                    value={llm.provider}
                    onChange={(e) => {
                      const p = e.target.value;
                      const pre = LLM_PRESETS[p] ?? LLM_PRESETS.none;
                      setLLM((curr) => ({ ...curr, provider: p, model: pre.model, api_key_env: pre.env }));
                    }}
                  >
                    <option value="none">none (deterministic only)</option>
                    <option value="groq">groq (fast inference)</option>
                    <option value="openrouter">openrouter</option>
                    <option value="openai">openai</option>
                    <option value="ollama">ollama (local)</option>
                  </Select>
                </label>

                <label className="block space-y-1 text-xs text-muted-foreground">
                  <span>Model</span>
                  <Input
                    value={llm.model}
                    onChange={(e) => setLLM((c) => ({ ...c, model: e.target.value }))}
                    placeholder="llama-3.3-70b-versatile"
                  />
                </label>

                <label className="block space-y-1 text-xs text-muted-foreground">
                  <span>API key env var</span>
                  <Input
                    value={llm.api_key_env}
                    onChange={(e) => setLLM((c) => ({ ...c, api_key_env: e.target.value }))}
                    placeholder="GROQ_API_KEY"
                  />
                </label>
              </div>
            </div>
          </form>

          <section className="space-y-3">
            <div>
              <h2 className="text-sm font-semibold text-foreground">Vaults &amp; Multi-Org Routing</h2>
              <p className="mt-0.5 text-xs text-muted-foreground">
                Add or update vaults directly from Studio; new directories are initialized with Git automatically.
              </p>
            </div>

            <div className="overflow-x-auto rounded-md border border-border bg-[#0f1416]">
              <table className="w-full min-w-[600px] border-collapse text-left text-xs">
                <thead>
                  <tr className="border-b border-border bg-[#12191c] text-muted-foreground">
                    <th className="px-3.5 py-2.5 font-medium">Vault</th>
                    <th className="px-3.5 py-2.5 font-medium">Policy</th>
                    <th className="px-3.5 py-2.5 font-medium">Path &amp; Routing</th>
                    <th className="px-3.5 py-2.5 text-right font-medium">Action</th>
                  </tr>
                </thead>
                <tbody className="divide-y divide-border/60">
                  {vaults.map((v) => (
                    <tr key={v.name}>
                      <td className="px-3.5 py-2.5 font-medium text-foreground">
                        {v.name}{" "}
                        <Badge variant={v.kind === "org" ? "default" : "warn"} className="ml-1.5">
                          {v.kind}
                        </Badge>
                      </td>
                      <td className="px-3.5 py-2.5 text-muted-foreground">{v.default_write}</td>
                      <td className="px-3.5 py-2.5 text-muted-foreground">
                        <div className="text-foreground">{v.path}</div>
                        <div className="mt-0.5 text-[11px]">
                          {[
                            v.match_paths?.length ? `paths: ${v.match_paths.join(", ")}` : "",
                            v.match_git_remotes?.length ? `remotes: ${v.match_git_remotes.join(", ")}` : "",
                            v.always_include?.length ? `always: ${v.always_include.join(", ")}` : "",
                          ]
                            .filter(Boolean)
                            .join(" · ") || "default scope"}
                        </div>
                      </td>
                      <td className="px-3.5 py-2.5 text-right">
                        <Button
                          variant="ghost"
                          size="icon"
                          aria-label={`Remove ${v.name}`}
                          disabled={busy}
                          onClick={() => void removeVault(v.name)}
                        >
                          <Trash size={14} />
                        </Button>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>

            <form
              onSubmit={saveVault}
              className="grid gap-2.5 rounded-md border border-border bg-[#0f1416] p-4 sm:grid-cols-6"
            >
              <Input
                value={vName}
                onChange={(e) => setVName(e.target.value)}
                placeholder="Vault name (e.g. acme)"
                className="sm:col-span-2"
              />
              <Input
                value={vPath}
                onChange={(e) => setVPath(e.target.value)}
                placeholder="Directory (e.g. ~/notes/acme)"
                className="sm:col-span-2"
              />
              <Select
                value={vKind}
                onChange={(e) => {
                  setVKind(e.target.value);
                  setVWrite(e.target.value === "org" ? "propose" : "direct");
                }}
              >
                <option value="org">org</option>
                <option value="personal">personal</option>
              </Select>
              <Select value={vWrite} onChange={(e) => setVWrite(e.target.value)}>
                <option value="propose">propose</option>
                <option value="direct">direct</option>
                <option value="readonly">readonly</option>
              </Select>
              <Input
                value={vMatchPaths}
                onChange={(e) => setVMatchPaths(e.target.value)}
                placeholder="match_paths (e.g. ~/work/acme/**)"
                className="sm:col-span-2"
              />
              <Input
                value={vMatchRemotes}
                onChange={(e) => setVMatchRemotes(e.target.value)}
                placeholder="match_git_remotes (e.g. github.com/acme/*)"
                className="sm:col-span-2"
              />
              <Input
                value={vAlways}
                onChange={(e) => setVAlways(e.target.value)}
                placeholder="always_include (persona/**)"
              />
              <Button type="submit" size="sm" disabled={busy || !vName.trim() || !vPath.trim()} className="h-8">
                <Plus size={14} />
                Add vault
              </Button>
            </form>
          </section>

          <section className="space-y-3">
            <div>
              <h2 className="text-sm font-semibold text-foreground">AI Harnesses (Hooks &amp; MCP)</h2>
              <p className="mt-0.5 text-xs text-muted-foreground">
                One-click setup for local coding agents detected on this machine.
              </p>
            </div>

            <div className="divide-y divide-border/70 rounded-md border border-border bg-[#0f1416]">
              {harnesses.map((h) => (
                <div key={h.id} className="flex items-center justify-between gap-3 px-4 py-3 text-xs">
                  <div className="flex items-center gap-2.5">
                    <span className="font-medium text-foreground">{h.label}</span>
                    <span className="text-muted-foreground">{h.marker}</span>
                    <Badge variant={h.detected ? "default" : "neutral"}>
                      {h.detected ? "detected" : "not found"}
                    </Badge>
                  </div>
                  <Button
                    variant="outline"
                    size="sm"
                    disabled={busy}
                    onClick={() => void connectHarness(h.id, h.label)}
                  >
                    <PlugsConnected size={13} />
                    Connect
                  </Button>
                </div>
              ))}
            </div>
          </section>
        </div>
      </div>
    </main>
  );
}
