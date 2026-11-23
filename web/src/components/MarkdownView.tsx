import * as React from "react";

const META_HEADER_RE = /^(status|stato|updated|aggiornat[oa]|valid_from|valido\s+dal|owner|proprietario)\s*:/i;

function stripTopHeader(raw: string, skipTitle: boolean): { body: string; strippedH1: boolean } {
  let lines = raw.replace(/\r\n/g, "\n").split("\n");
  if (lines.length > 0 && lines[0].trim() === "---") {
    for (let i = 1; i < lines.length; i++) {
      if (lines[i].trim() === "---") {
        lines = lines.slice(i + 1);
        break;
      }
    }
  }

  let start = 0;
  let seenH1 = false;
  let keptH1: string | null = null;
  const maxScan = Math.min(lines.length, 14);
  while (start < maxScan) {
    const trimmed = lines[start].trim();
    if (trimmed === "") {
      start++;
      continue;
    }
    if (META_HEADER_RE.test(trimmed)) {
      start++;
      continue;
    }
    if (!seenH1 && trimmed.startsWith("# ")) {
      seenH1 = true;
      if (!skipTitle) {
        keptH1 = lines[start];
      }
      start++;
      continue;
    }
    break;
  }

  const rest = lines.slice(start).join("\n").replace(/^\n+/, "");
  const body = keptH1 ? `${keptH1}\n\n${rest}` : rest;
  return { body, strippedH1: seenH1 && skipTitle };
}

function renderInline(text: string): React.ReactNode[] {
  const tokens = text.split(/(`[^`]+`|\*\*[^*]+\*\*)/g);
  return tokens.map((tok, i) => {
    if (tok.startsWith("`") && tok.endsWith("`") && tok.length > 2) {
      return (
        <span
          key={i}
          className="rounded-[4px] border border-border bg-[#16161a] px-1.5 py-0.5 text-[13px] font-medium text-zinc-200"
        >
          {tok.slice(1, -1)}
        </span>
      );
    }
    if (tok.startsWith("**") && tok.endsWith("**") && tok.length > 4) {
      return (
        <strong key={i} className="font-semibold text-foreground">
          {tok.slice(2, -2)}
        </strong>
      );
    }
    return <React.Fragment key={i}>{tok}</React.Fragment>;
  });
}

export function MarkdownView({ raw, skipTitle }: { raw: string; skipTitle?: boolean }) {
  const { body, strippedH1 } = stripTopHeader(raw, Boolean(skipTitle));
  const lines = body.split("\n");
  const blocks: React.ReactNode[] = [];

  let i = 0;
  let skippedFirstH1 = strippedH1;
  while (i < lines.length) {
    const line = lines[i];
    const trimmed = line.trim();

    if (trimmed === "") {
      i++;
      continue;
    }

    if (trimmed.startsWith("```")) {
      const codeLines: string[] = [];
      i++;
      while (i < lines.length && !lines[i].trim().startsWith("```")) {
        codeLines.push(lines[i]);
        i++;
      }
      i++;
      blocks.push(
        <div
          key={`code-${i}`}
          className="my-3 overflow-x-auto rounded-md border border-border bg-[#0f0f12] px-3.5 py-3 text-[13px] leading-relaxed text-zinc-300 whitespace-pre"
        >
          {codeLines.join("\n")}
        </div>,
      );
      continue;
    }

    if (trimmed.startsWith("# ")) {
      if (skipTitle && !skippedFirstH1) {
        skippedFirstH1 = true;
        i++;
        continue;
      }
      blocks.push(
        <h1 key={`h1-${i}`} className="mt-1 mb-3 text-2xl font-medium tracking-tight text-foreground">
          {renderInline(trimmed.slice(2))}
        </h1>,
      );
      i++;
      continue;
    }
    if (trimmed.startsWith("## ")) {
      blocks.push(
        <h2 key={`h2-${i}`} className="mt-6 mb-2.5 text-[15px] font-medium tracking-tight text-foreground">
          {renderInline(trimmed.slice(3))}
        </h2>,
      );
      i++;
      continue;
    }
    if (trimmed.startsWith("### ")) {
      blocks.push(
        <h3 key={`h3-${i}`} className="mt-4 mb-1.5 text-sm font-medium text-foreground">
          {renderInline(trimmed.slice(4))}
        </h3>,
      );
      i++;
      continue;
    }

    if (/^[-*]\s+/.test(trimmed)) {
      const items: string[] = [];
      while (i < lines.length && /^[-*]\s+/.test(lines[i].trim())) {
        items.push(lines[i].trim().replace(/^[-*]\s+/, ""));
        i++;
      }
      blocks.push(
        <ul key={`ul-${i}`} className="my-2.5 space-y-2">
          {items.map((item, idx) => {
            const isArchive = item.includes("· invalidated]");
            return (
              <li
                key={idx}
                className={
                  isArchive
                    ? "border-l-2 border-zinc-600 bg-[#111114] px-3 py-2 text-[13px] leading-relaxed text-muted-foreground"
                    : "relative pl-4 text-[14.5px] leading-relaxed text-zinc-300"
                }
              >
                {!isArchive && (
                  <span className="absolute left-0 top-[10px] h-1 w-1 rounded-[1px] bg-zinc-500" />
                )}
                {renderInline(item)}
              </li>
            );
          })}
        </ul>,
      );
      continue;
    }

    if (/^\d+\.\s+/.test(trimmed)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s+/.test(lines[i].trim())) {
        items.push(lines[i].trim().replace(/^\d+\.\s+/, ""));
        i++;
      }
      blocks.push(
        <ol key={`ol-${i}`} className="my-2.5 list-decimal space-y-1.5 pl-5 text-[14.5px] leading-relaxed text-zinc-300">
          {items.map((item, idx) => (
            <li key={idx}>{renderInline(item)}</li>
          ))}
        </ol>,
      );
      continue;
    }

    if (trimmed.startsWith("> ")) {
      const qLines: string[] = [];
      while (i < lines.length && lines[i].trim().startsWith("> ")) {
        qLines.push(lines[i].trim().slice(2));
        i++;
      }
      blocks.push(
        <blockquote
          key={`q-${i}`}
          className="my-3 border-l-2 border-zinc-600 bg-[#111114] px-3.5 py-2 text-[13.5px] leading-relaxed text-muted-foreground"
        >
          {renderInline(qLines.join(" "))}
        </blockquote>,
      );
      continue;
    }

    const pLines: string[] = [];
    while (
      i < lines.length &&
      lines[i].trim() !== "" &&
      !/^(#{1,3}\s+|[-*]\s+|\d+\.\s+|>\s+|```)/.test(lines[i].trim())
    ) {
      pLines.push(lines[i].trim());
      i++;
    }
    blocks.push(
      <p key={`p-${i}`} className="my-2.5 text-[14.5px] leading-relaxed text-zinc-300">
        {renderInline(pLines.join(" "))}
      </p>,
    );
  }

  return <div className="min-w-0">{blocks}</div>;
}
