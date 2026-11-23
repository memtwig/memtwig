import * as React from "react";

function stripFrontmatter(raw: string): string {
  const lines = raw.replace(/\r\n/g, "\n").split("\n");
  if (lines.length > 0 && lines[0].trim() === "---") {
    for (let i = 1; i < lines.length; i++) {
      if (lines[i].trim() === "---") {
        return lines.slice(i + 1).join("\n").replace(/^\n+/, "");
      }
    }
  }
  // Also strip lightweight Italian/English top headers if present in first 6 lines
  let start = 0;
  while (
    start < Math.min(lines.length, 6) &&
    /^(status|stato|updated|aggiornat[oa]|valid_from|valido\s+dal|owner|proprietario)\s*:/i.test(lines[start].trim())
  ) {
    start++;
  }
  return lines.slice(start).join("\n").replace(/^\n+/, "");
}

function renderInline(text: string): React.ReactNode[] {
  const tokens = text.split(/(`[^`]+`|\*\*[^*]+\*\*)/g);
  return tokens.map((tok, i) => {
    if (tok.startsWith("`") && tok.endsWith("`") && tok.length > 2) {
      return (
        <span
          key={i}
          className="rounded-[4px] border border-border bg-muted px-1.5 py-0.5 text-[13px] font-medium text-[#0f4c47]"
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
  const body = stripFrontmatter(raw);
  const lines = body.split("\n");
  const blocks: React.ReactNode[] = [];

  let i = 0;
  let skippedFirstH1 = false;
  while (i < lines.length) {
    const line = lines[i];
    const trimmed = line.trim();

    if (trimmed === "") {
      i++;
      continue;
    }

    // Fenced code block
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
          className="my-3 overflow-x-auto rounded-md border border-border bg-[#f6f8f8] px-3.5 py-3 text-[13px] leading-relaxed text-foreground whitespace-pre"
        >
          {codeLines.join("\n")}
        </div>,
      );
      continue;
    }

    // Headings
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
        <h2 key={`h2-${i}`} className="mt-6 mb-2.5 text-[15px] font-semibold tracking-tight text-foreground">
          {renderInline(trimmed.slice(3))}
        </h2>,
      );
      i++;
      continue;
    }
    if (trimmed.startsWith("### ")) {
      blocks.push(
        <h3 key={`h3-${i}`} className="mt-4 mb-1.5 text-sm font-semibold text-foreground">
          {renderInline(trimmed.slice(4))}
        </h3>,
      );
      i++;
      continue;
    }

    // Unordered list
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
                    ? "border-l-2 border-primary bg-muted/60 px-3 py-2 text-[13px] leading-relaxed text-muted-foreground"
                    : "relative pl-4 text-[14.5px] leading-relaxed text-[#223030]"
                }
              >
                {!isArchive && (
                  <span className="absolute left-0 top-[10px] h-1 w-1 rounded-[1px] bg-primary/70" />
                )}
                {renderInline(item)}
              </li>
            );
          })}
        </ul>,
      );
      continue;
    }

    // Ordered list
    if (/^\d+\.\s+/.test(trimmed)) {
      const items: string[] = [];
      while (i < lines.length && /^\d+\.\s+/.test(lines[i].trim())) {
        items.push(lines[i].trim().replace(/^\d+\.\s+/, ""));
        i++;
      }
      blocks.push(
        <ol key={`ol-${i}`} className="my-2.5 list-decimal space-y-1.5 pl-5 text-[14.5px] leading-relaxed text-[#223030]">
          {items.map((item, idx) => (
            <li key={idx}>{renderInline(item)}</li>
          ))}
        </ol>,
      );
      continue;
    }

    // Blockquote
    if (trimmed.startsWith("> ")) {
      const qLines: string[] = [];
      while (i < lines.length && lines[i].trim().startsWith("> ")) {
        qLines.push(lines[i].trim().slice(2));
        i++;
      }
      blocks.push(
        <blockquote
          key={`q-${i}`}
          className="my-3 border-l-2 border-primary bg-muted/50 px-3.5 py-2 text-[13.5px] leading-relaxed text-muted-foreground"
        >
          {renderInline(qLines.join(" "))}
        </blockquote>,
      );
      continue;
    }

    // Paragraph
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
      <p key={`p-${i}`} className="my-2.5 text-[14.5px] leading-relaxed text-[#223030]">
        {renderInline(pLines.join(" "))}
      </p>,
    );
  }

  return <div className="min-w-0">{blocks}</div>;
}
