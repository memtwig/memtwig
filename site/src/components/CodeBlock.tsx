import * as React from "react";
import { Check, Copy } from "@phosphor-icons/react";
import { highlight } from "@/lib/highlight";
import { cn } from "@/lib/utils";

type CodeBlockProps = {
  code: string;
  /** bash | json | toml | yaml | diff | prompt | text (aliases like "shell" work too) */
  lang?: string;
  /** Left side of the window header: file name or context ("mcp.json", "Terminal"). */
  title?: React.ReactNode;
  /** Right side, next to the copy button. Defaults to the language name. */
  meta?: React.ReactNode;
  /** Wrap long lines instead of scrolling horizontally (prompts). */
  wrap?: boolean;
  /** Text copy button instead of an icon-only one. */
  copyLabel?: string;
  className?: string;
  bodyClassName?: string;
};

function useCopy(text: string) {
  const [copied, setCopied] = React.useState(false);
  const timer = React.useRef<number | undefined>(undefined);
  React.useEffect(() => () => window.clearTimeout(timer.current), []);
  const copy = React.useCallback(async () => {
    try {
      await navigator.clipboard.writeText(text);
    } catch {
      // Fallback for insecure contexts (plain http previews).
      const ta = document.createElement("textarea");
      ta.value = text;
      ta.setAttribute("readonly", "");
      ta.style.position = "fixed";
      ta.style.opacity = "0";
      document.body.appendChild(ta);
      ta.select();
      try {
        document.execCommand("copy");
      } finally {
        ta.remove();
      }
    }
    setCopied(true);
    window.clearTimeout(timer.current);
    timer.current = window.setTimeout(() => setCopied(false), 1800);
  }, [text]);
  return { copied, copy };
}

export function CopyButton({ text, label, name }: { text: string; label?: string; name: string }) {
  const { copied, copy } = useCopy(text);
  return (
    <button
      type="button"
      onClick={copy}
      aria-label={label ? undefined : copied ? `Copied ${name}` : `Copy ${name}`}
      className={cn(
        "relative inline-flex shrink-0 cursor-pointer items-center justify-center gap-1.5 rounded-full border text-xs font-medium transition-[color,background-color,border-color] duration-200 outline-none focus-visible:ring-2 focus-visible:ring-[#58cc7d]/60",
        label ? "h-8 px-3.5" : "size-8",
        copied
          ? "border-[#58cc7d]/40 bg-[#58cc7d]/12 text-[#7ee09e]"
          : "border-white/[0.09] bg-white/[0.03] text-zinc-400 hover:border-white/20 hover:bg-white/[0.07] hover:text-white",
      )}
    >
      <span className="relative inline-flex size-[15px] items-center justify-center" aria-hidden="true">
        <Copy
          size={15}
          className={cn(
            "absolute transition-[opacity,transform] duration-200 motion-reduce:transition-none",
            copied ? "scale-75 opacity-0" : "scale-100 opacity-100",
          )}
        />
        <Check
          size={15}
          weight="bold"
          className={cn(
            "absolute transition-[opacity,transform] duration-200 motion-reduce:transition-none",
            copied ? "scale-100 opacity-100" : "scale-75 opacity-0",
          )}
        />
      </span>
      {label && <span>{copied ? "Copied" : label}</span>}
      <span className="sr-only" aria-live="polite">
        {copied ? "Copied to clipboard" : ""}
      </span>
    </button>
  );
}

const LANG_NAMES: Record<string, string> = {
  bash: "bash",
  sh: "bash",
  shell: "shell",
  json: "json",
  toml: "toml",
  yaml: "yaml",
  diff: "diff",
  prompt: "prompt",
};

export function CodeBlock({
  code,
  lang = "bash",
  title,
  meta,
  wrap = false,
  copyLabel,
  className,
  bodyClassName,
}: CodeBlockProps) {
  const lines = React.useMemo(() => highlight(code, lang), [code, lang]);
  const metaNode = meta ?? LANG_NAMES[lang.toLowerCase()] ?? lang;
  const copyName = typeof title === "string" ? title : `${metaNode} snippet`;

  return (
    <figure
      className={cn(
        "min-w-0 overflow-hidden rounded-[20px] border border-white/[0.08] bg-[#111113] shadow-[inset_0_1px_0_rgba(255,255,255,0.04),0_18px_50px_-30px_rgba(0,0,0,0.9)]",
        className,
      )}
    >
      <figcaption className="flex min-h-12 items-center justify-between gap-3 border-b border-white/[0.06] bg-[#18181b] py-2 pr-2 pl-4 sm:pl-5">
        <div className="flex min-w-0 items-center gap-3">
          <span className="flex shrink-0 items-center gap-1.5" aria-hidden="true">
            <span className="size-2 rounded-full bg-white/[0.12]" />
            <span className="size-2 rounded-full bg-white/[0.12]" />
            <span className="size-2 rounded-full bg-white/[0.12]" />
          </span>
          {title && <span className="truncate text-sm font-medium text-zinc-200">{title}</span>}
        </div>
        <div className="flex shrink-0 items-center gap-2.5">
          {metaNode && <span className="code-surface hidden text-[11px] text-zinc-500 sm:inline">{metaNode}</span>}
          <CopyButton text={code} label={copyLabel} name={copyName} />
        </div>
      </figcaption>
      <pre
        className={cn(
          "code-surface m-0 overflow-x-auto px-4 py-4 text-[12.5px] leading-[1.75] text-zinc-200 sm:px-5 sm:text-[13px]",
          wrap ? "whitespace-pre-wrap break-words" : "whitespace-pre",
          bodyClassName,
        )}
      >
        <code>
          {lines.map((line, li) => (
            <React.Fragment key={li}>
              {line.map(([text, cls], ti) =>
                cls ? (
                  <span key={ti} className={`tok-${cls}`}>
                    {text}
                  </span>
                ) : (
                  <React.Fragment key={ti}>{text}</React.Fragment>
                ),
              )}
              {li < lines.length - 1 ? "\n" : null}
            </React.Fragment>
          ))}
        </code>
      </pre>
    </figure>
  );
}
