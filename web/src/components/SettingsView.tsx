import * as React from "react";
import { Check, PlugsConnected, Plus, Trash } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { Select } from "@/components/ui/select";
import { cn } from "@/lib/utils";

type SettingsTab = "providers" | "vaults" | "harnesses";

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
  const [subTab, setSubTab] = React.useState<SettingsTab>("providers");
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
        flash("Saved");
        await onChanged();
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
        flash(`Saved ${vName.trim()}`);
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
        flash(`Removed ${name}`);
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
    <main className="flex min-h-0 min-w-0 flex-1 flex-col bg-background">
      <div className="flex h-12 shrink-0 items-center justify-between gap-4 border-b border-border bg-[#0c0c0e] px-5 md:px-7">
        <div className="flex items-center gap-1">
          {(
            [
              ["providers", "Providers"],
              ["vaults", "Vaults"],
              ["harnesses", "Harnesses"],
            ] as Array<[SettingsTab, string]>
          ).map(([id, label]) => (
            <Button
              key={id}
              variant="ghost"
              size="sm"
              onClick={() => setSubTab(id)}
              className={cn(
                "h-7 px-2.5 text-xs",
                subTab === id && "bg-[#1b1b20] text-foreground hover:bg-[#1b1b20]",
              )}
            >
              {label}
            </Button>
          ))}
        </div>
      </div>

      <div className="flex-1 overflow-y-auto p-5 md:p-8">
        <div className="mx-auto max-w-[760px]">
          {subTab === "providers" && (
            <form onSubmit={saveProviders} className="space-y-6">
              <div className="flex items-center justify-between border-b border-border pb-3">
                <h2 className="text-sm font-medium text-foreground">Embeddings &amp; Curator</h2>
                <Button type="submit" size="sm" disabled={busy}>
                  <Check size={14} />
                  Save
                </Button>
              </div>

              <div className="grid gap-5 md:grid-cols-2">
                <div className="space-y-3 rounded-md border border-border bg-[#0e0e11] p-4">
                  <div className="flex items-center justify-between text-xs">
                    <span className="font-medium text-foreground">Embeddings</span>
                    {emb.api_key_env ? (
                      <span className="inline-flex items-center gap-1.5 text-muted-foreground">
                        <span
                          className={cn(
                            "h-1.5 w-1.5 rounded-full",
                            envStatus.embeddings ? "bg-emerald-500" : "bg-amber-500",
                          )}
                        />
                        {envStatus.embeddings ? "env set" : "env missing"}
                      </span>
                    ) : (
                      <span className="text-muted-foreground">
                        {emb.provider === "none" ? "BM25 only" : "local"}
                      </span>
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
                          setEmb((c) => ({
                            ...c,
                            provider: p,
                            model: pre.model,
                            api_key_env: pre.env,
                            endpoint: pre.endpoint,
                          }));
                        }}
                      >
                        <option value="none">none</option>
                        <option value="ollama">ollama</option>
                        <option value="openrouter">openrouter</option>
                        <option value="deepinfra">deepinfra</option>
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

                  <label className="block space-y-1 text-xs text-muted-foreground">
                    <span>API key env</span>
                    <Input
                      value={emb.api_key_env}
                      onChange={(e) => setEmb((c) => ({ ...c, api_key_env: e.target.value }))}
                      placeholder="OPENROUTER_API_KEY"
                    />
                  </label>
                </div>

                <div className="space-y-3 rounded-md border border-border bg-[#0e0e11] p-4">
                  <div className="flex items-center justify-between text-xs">
                    <span className="font-medium text-foreground">LLM Curator</span>
                    {llm.api_key_env ? (
                      <span className="inline-flex items-center gap-1.5 text-muted-foreground">
                        <span
                          className={cn(
                            "h-1.5 w-1.5 rounded-full",
                            envStatus.llm ? "bg-emerald-500" : "bg-amber-500",
                          )}
                        />
                        {envStatus.llm ? "env set" : "env missing"}
                      </span>
                    ) : (
                      <span className="text-muted-foreground">
                        {llm.provider === "none" ? "disabled" : "local"}
                      </span>
                    )}
                  </div>

                  <label className="block space-y-1 text-xs text-muted-foreground">
                    <span>Provider</span>
                    <Select
                      value={llm.provider}
                      onChange={(e) => {
                        const p = e.target.value;
                        const pre = LLM_PRESETS[p] ?? LLM_PRESETS.none;
                        setLLM((c) => ({ ...c, provider: p, model: pre.model, api_key_env: pre.env }));
                      }}
                    >
                      <option value="none">none</option>
                      <option value="groq">groq</option>
                      <option value="openrouter">openrouter</option>
                      <option value="openai">openai</option>
                      <option value="ollama">ollama</option>
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
                    <span>API key env</span>
                    <Input
                      value={llm.api_key_env}
                      onChange={(e) => setLLM((c) => ({ ...c, api_key_env: e.target.value }))}
                      placeholder="GROQ_API_KEY"
                    />
                  </label>
                </div>
              </div>
            </form>
          )}

          {subTab === "vaults" && (
            <div className="space-y-6">
              <div className="overflow-x-auto rounded-md border border-border bg-[#0e0e11]">
                <table className="w-full min-w-[540px] border-collapse text-left text-xs">
                  <thead>
                    <tr className="border-b border-border text-muted-foreground">
                      <th className="px-4 py-2.5 font-medium">Vault</th>
                      <th className="px-4 py-2.5 font-medium">Write</th>
                      <th className="px-4 py-2.5 font-medium">Path &amp; Routing</th>
                      <th className="px-4 py-2.5 text-right font-medium" />
                    </tr>
                  </thead>
                  <tbody className="divide-y divide-border/70">
                    {vaults.map((v) => (
                      <tr key={v.name}>
                        <td className="px-4 py-2.5 font-medium text-foreground">
                          {v.name} <span className="ml-1 font-normal text-muted-foreground">{v.kind}</span>
                        </td>
                        <td className="px-4 py-2.5 text-muted-foreground">{v.default_write}</td>
                        <td className="px-4 py-2.5 text-muted-foreground">
                          <div className="text-foreground">{v.path}</div>
                          <div className="mt-0.5 text-[11px]">
                            {[
                              v.match_paths?.length ? v.match_paths.join(", ") : "",
                              v.match_git_remotes?.length ? v.match_git_remotes.join(", ") : "",
                              v.always_include?.length ? v.always_include.join(", ") : "",
                            ]
                              .filter(Boolean)
                              .join(" · ")}
                          </div>
                        </td>
                        <td className="px-4 py-2.5 text-right">
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

              <form onSubmit={saveVault} className="space-y-3 rounded-md border border-border bg-[#0e0e11] p-4">
                <div className="text-xs font-medium text-foreground">Add vault</div>
                <div className="grid gap-2.5 sm:grid-cols-2">
                  <Input value={vName} onChange={(e) => setVName(e.target.value)} placeholder="Name" />
                  <Input value={vPath} onChange={(e) => setVPath(e.target.value)} placeholder="Path (~/notes/acme)" />
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
                    placeholder="Match paths (~/work/acme/**)"
                  />
                  <Input
                    value={vMatchRemotes}
                    onChange={(e) => setVMatchRemotes(e.target.value)}
                    placeholder="Match remotes (github.com/acme/*)"
                  />
                </div>
                <div className="flex items-center justify-between gap-2.5">
                  <Input
                    value={vAlways}
                    onChange={(e) => setVAlways(e.target.value)}
                    placeholder="Always include (persona/**)"
                    className="max-w-xs"
                  />
                  <Button type="submit" size="sm" disabled={busy || !vName.trim() || !vPath.trim()}>
                    <Plus size={14} />
                    Add
                  </Button>
                </div>
              </form>
            </div>
          )}

          {subTab === "harnesses" && (
            <div className="divide-y divide-border/70 rounded-md border border-border bg-[#0e0e11]">
              {harnesses.map((h) => (
                <div key={h.id} className="flex items-center justify-between gap-3 px-4 py-3 text-xs">
                  <div className="flex items-center gap-2.5">
                    <span
                      className={cn("h-1.5 w-1.5 rounded-full", h.detected ? "bg-emerald-500" : "bg-zinc-600")}
                    />
                    <span className="font-medium text-foreground">{h.label}</span>
                    <span className="text-muted-foreground">{h.marker}</span>
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
          )}
        </div>
      </div>
    </main>
  );
}
