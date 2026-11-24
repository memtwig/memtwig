import * as React from "react";
import { Check } from "@phosphor-icons/react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

type PlanOption = "pro" | "team" | "agency";
type TeamSize = "solo" | "2-10" | "10+";

const plans: Array<{ id: PlanOption; label: string }> = [
  { id: "pro", label: "Pro ($19/mo)" },
  { id: "team", label: "Team ($49/mo)" },
  { id: "agency", label: "Agency / Custom" },
];

const sizes: Array<{ id: TeamSize; label: string }> = [
  { id: "solo", label: "Solo" },
  { id: "2-10", label: "2–10" },
  { id: "10+", label: "10+" },
];

export function WaitlistForm() {
  const [email, setEmail] = React.useState("");
  const [plan, setPlan] = React.useState<PlanOption>("pro");
  const [teamSize, setTeamSize] = React.useState<TeamSize>("solo");
  const [primaryAgent, setPrimaryAgent] = React.useState("");
  const [website, setWebsite] = React.useState("");
  const [submitting, setSubmitting] = React.useState(false);
  const [error, setError] = React.useState<string | null>(null);
  const [status, setStatus] = React.useState<"joined" | "already_joined" | null>(null);

  React.useEffect(() => {
    const p = new URLSearchParams(window.location.search).get("plan")?.toLowerCase();
    if (p === "pro" || p === "team" || p === "agency") {
      setPlan(p);
    }

    const onSelectPlan = (ev: Event) => {
      const next = (ev as CustomEvent<string>).detail?.toLowerCase();
      if (next === "pro" || next === "team" || next === "agency") {
        setPlan(next);
      }
    };
    const onDocClick = (ev: MouseEvent) => {
      const target = (ev.target as HTMLElement | null)?.closest("[data-plan]");
      const attr = target?.getAttribute("data-plan")?.toLowerCase();
      if (attr === "pro" || attr === "team" || attr === "agency") {
        setPlan(attr);
      }
    };

    window.addEventListener("memtwig:select-plan", onSelectPlan);
    document.addEventListener("click", onDocClick);
    return () => {
      window.removeEventListener("memtwig:select-plan", onSelectPlan);
      document.removeEventListener("click", onDocClick);
    };
  }, []);

  const handleSubmit = async (e: React.FormEvent) => {
    e.preventDefault();
    setError(null);
    const trimmed = email.trim();
    if (!trimmed) {
      setError("Enter your work email.");
      return;
    }

    setSubmitting(true);
    try {
      const res = await fetch("/api/waitlist", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          email: trimmed,
          plan,
          teamSize,
          primaryAgent: primaryAgent.trim(),
          website,
        }),
      });
      const data = await res.json().catch(() => null);
      if (!res.ok || !data?.ok) {
        setError(data?.error || "Could not submit. Please retry.");
        return;
      }
      setStatus(data.status === "already_joined" ? "already_joined" : "joined");
    } catch {
      setError("Network error. Please retry.");
    } finally {
      setSubmitting(false);
    }
  };

  if (status) {
    return (
      <div className="rounded-2xl border border-[#58cc7d]/35 bg-[#58cc7d]/10 p-6">
        <div className="flex items-center gap-2.5 text-base font-medium text-white">
          <Check size={18} className="text-[#58cc7d]" />
          <span>
            {status === "already_joined"
              ? "Preferences updated — you're already on the waitlist."
              : "You're on the waitlist."}
          </span>
        </div>
        <p className="mt-2 text-sm text-zinc-300">
          {email.trim().toLowerCase()} · {plans.find((p) => p.id === plan)?.label}
        </p>
        <div className="mt-5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            onClick={() => setStatus(null)}
          >
            Update details
          </Button>
        </div>
      </div>
    );
  }

  return (
    <form onSubmit={handleSubmit} className="space-y-6">
      <div className="sr-only" aria-hidden="true">
        <label htmlFor="waitlist-website">Website</label>
        <Input
          id="waitlist-website"
          name="website"
          type="text"
          tabIndex={-1}
          autoComplete="off"
          value={website}
          onChange={(e) => setWebsite(e.target.value)}
        />
      </div>

      <div>
        <label htmlFor="waitlist-email" className="block text-sm font-medium text-white">
          Work email
        </label>
        <Input
          id="waitlist-email"
          name="email"
          type="email"
          required
          autoComplete="email"
          placeholder="you@company.com"
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          className="mt-2"
        />
      </div>

      <div className="grid gap-6 sm:grid-cols-2">
        <div>
          <span className="block text-sm font-medium text-white">Plan</span>
          <div className="mt-2.5 flex flex-wrap gap-2">
            {plans.map((p) => {
              const selected = plan === p.id;
              return (
                <Button
                  key={p.id}
                  type="button"
                  size="sm"
                  variant={selected ? "default" : "outline"}
                  onClick={() => setPlan(p.id)}
                  className={cn(selected && "font-medium")}
                >
                  {p.label}
                </Button>
              );
            })}
          </div>
        </div>

        <div>
          <span className="block text-sm font-medium text-white">Team size</span>
          <div className="mt-2.5 flex flex-wrap gap-2">
            {sizes.map((s) => {
              const selected = teamSize === s.id;
              return (
                <Button
                  key={s.id}
                  type="button"
                  size="sm"
                  variant={selected ? "default" : "outline"}
                  onClick={() => setTeamSize(s.id)}
                  className={cn(selected && "font-medium")}
                >
                  {s.label}
                </Button>
              );
            })}
          </div>
        </div>
      </div>

      <div>
        <label htmlFor="waitlist-agent" className="block text-sm font-medium text-white">
          Primary agents or bots <span className="font-normal text-muted-foreground">(optional)</span>
        </label>
        <Input
          id="waitlist-agent"
          name="primaryAgent"
          type="text"
          maxLength={120}
          placeholder="Claude Code, Grok Bot, Hermes, OpenBot..."
          value={primaryAgent}
          onChange={(e) => setPrimaryAgent(e.target.value)}
          className="mt-2"
        />
      </div>

      {error && (
        <p role="alert" className="text-sm text-red-400">
          {error}
        </p>
      )}

      <div className="pt-1">
        <Button type="submit" size="lg" disabled={submitting} className="w-full sm:w-auto">
          {submitting ? "Joining..." : "Join early access"}
        </Button>
        <p className="mt-3 text-xs leading-relaxed text-zinc-500">
          We only email you about Memtwig Cloud. See the{" "}
          <a href="/privacy" className="underline underline-offset-4 hover:text-zinc-300">
            privacy policy
          </a>
          .
        </p>
      </div>
    </form>
  );
}
