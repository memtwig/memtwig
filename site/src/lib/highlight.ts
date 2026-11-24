// Tiny, dependency-free tokenizer for the handful of languages memtwig shows:
// shell, json, toml, yaml, diff and plain-text agent prompts. Output is a list of
// lines, each a list of [text, tokenClass] pairs; the classes map to `.tok-*` CSS.

export type TokenClass =
  | ""
  | "cmd"
  | "flag"
  | "str"
  | "var"
  | "com"
  | "key"
  | "num"
  | "punct"
  | "sec"
  | "add"
  | "del"
  | "hunk"
  | "head"
  | "list"
  | "code"
  | "url";

export type Token = [string, TokenClass];
export type Line = Token[];

export type Lang = "bash" | "json" | "toml" | "yaml" | "diff" | "prompt" | "text";

export function normalizeLang(lang?: string): Lang {
  const l = (lang ?? "").toLowerCase().trim();
  if (["bash", "sh", "shell", "zsh", "console", "terminal"].includes(l)) return "bash";
  if (["json", "jsonc", "mcp.json"].includes(l)) return "json";
  if (["toml", "config.toml"].includes(l)) return "toml";
  if (["yaml", "yml"].includes(l)) return "yaml";
  if (["diff", "patch"].includes(l)) return "diff";
  if (["prompt", "md", "markdown"].includes(l)) return "prompt";
  return "text";
}

type Rule = [RegExp, TokenClass | ((m: RegExpExecArray, ctx: { atCmd: boolean }) => TokenClass)];

function scan(line: string, rules: Rule[], ctx = { atCmd: true }): Line {
  const out: Line = [];
  let i = 0;
  let plain = "";
  const flush = () => {
    if (plain) out.push([plain, ""]);
    plain = "";
  };
  while (i < line.length) {
    let matched = false;
    for (const [re, cls] of rules) {
      re.lastIndex = i;
      const m = re.exec(line);
      if (m && m.index === i && m[0].length > 0) {
        flush();
        const c = typeof cls === "function" ? cls(m, ctx) : cls;
        out.push([m[0], c]);
        i += m[0].length;
        matched = true;
        break;
      }
    }
    if (!matched) {
      const ch = line[i];
      plain += ch;
      i++;
    }
  }
  flush();
  return out;
}

const sticky = (src: string) => new RegExp(src, "y");

function bashLine(line: string): Line {
  const ctx = { atCmd: true };
  const rules: Rule[] = [
    [sticky("#.*$"), "com"],
    [sticky("\\s+"), ""],
    [sticky("(?:&&|\\|\\||[|;])"), (_m, c) => ((c.atCmd = true), "punct")],
    [sticky("\\\\$"), "punct"],
    [sticky("\"(?:[^\"\\\\]|\\\\.)*\"|'[^']*'"), (_m, c) => ((c.atCmd = false), "str")],
    [sticky("\\$\\{[^}]*\\}|\\$\\(|\\$[A-Za-z_][A-Za-z0-9_]*"), "var"],
    [sticky("https?://[^\\s\"']+"), (_m, c) => ((c.atCmd = false), "url")],
    [sticky("--?[A-Za-z][\\w-]*(?:=[^\\s]*)?"), (_m, c) => ((c.atCmd = false), "flag")],
    [
      sticky("[^\\s|;&\"'$]+"),
      (_m, c) => {
        const cls: TokenClass = c.atCmd ? "cmd" : "";
        c.atCmd = false;
        return cls;
      },
    ],
  ];
  // Continuation lines (after a trailing backslash) are arguments, not commands.
  const toks = scan(line, rules, ctx);
  return toks;
}

function bash(code: string): Line[] {
  const lines = code.split("\n");
  let continued = false;
  return lines.map((l) => {
    let toks = bashLine(l);
    if (continued) {
      toks = toks.map(([t, c]) => [t, c === "cmd" ? "" : c] as Token);
    }
    continued = /\\\s*$/.test(l);
    return toks;
  });
}

function json(code: string): Line[] {
  const rules: Rule[] = [
    [sticky("\"(?:[^\"\\\\]|\\\\.)*\"(?=\\s*:)"), "key"],
    [sticky("\"(?:[^\"\\\\]|\\\\.)*\""), "str"],
    [sticky("-?\\d+(?:\\.\\d+)?(?:[eE][+-]?\\d+)?|\\btrue\\b|\\bfalse\\b|\\bnull\\b"), "num"],
    [sticky("[{}\\[\\],:]"), "punct"],
    [sticky("//.*$"), "com"],
  ];
  return code.split("\n").map((l) => scan(l, rules));
}

function toml(code: string): Line[] {
  const rules: Rule[] = [
    [sticky("#.*$"), "com"],
    [sticky("^\\s*\\[\\[?[^\\]]+\\]\\]?"), "sec"],
    [sticky("^\\s*[A-Za-z0-9_.\"-]+(?=\\s*=)"), "key"],
    [sticky("\"(?:[^\"\\\\]|\\\\.)*\"|'[^']*'"), "str"],
    [sticky("-?\\d+(?:\\.\\d+)?\\b|\\btrue\\b|\\bfalse\\b"), "num"],
    [sticky("[=\\[\\],{}]"), "punct"],
  ];
  return code.split("\n").map((l) => scan(l, rules));
}

function yaml(code: string): Line[] {
  const rules: Rule[] = [
    [sticky("#.*$"), "com"],
    [sticky("^\\s*-?\\s*[A-Za-z0-9_.\"/-]+(?=:(?:\\s|$))"), "key"],
    [sticky("\"(?:[^\"\\\\]|\\\\.)*\"|'[^']*'"), "str"],
    [sticky("\\b\\d+(?:\\.\\d+)?\\b|\\btrue\\b|\\bfalse\\b"), "num"],
    [sticky("[:\\[\\],{}-]"), "punct"],
  ];
  return code.split("\n").map((l) => scan(l, rules));
}

function diff(code: string): Line[] {
  return code.split("\n").map((l): Line => {
    if (l.startsWith("@@")) return [[l, "hunk"]];
    if (l.startsWith("+++") || l.startsWith("---")) return [[l, "com"]];
    if (l.startsWith("+")) return [[l, "add"]];
    if (l.startsWith("-")) return [[l, "del"]];
    return [[l, ""]];
  });
}

function prompt(code: string): Line[] {
  const rules: Rule[] = [
    [sticky("`[^`]+`"), "code"],
    [sticky("https?://[^\\s)]*[^\\s).,:;]|\\b[a-z0-9-]+\\.(?:com|txt)\\b[^\\s)]*"), "url"],
  ];
  return code.split("\n").map((l) => {
    const head = /^(\s*)(#{1,3}\s.*)$/.exec(l);
    if (head) return [[head[1], ""], [head[2], "head"]] as Line;
    const list = /^(\s*)(\d+\.|[-*])(\s+)(.*)$/.exec(l);
    if (list) return [[list[1], ""], [list[2], "list"], [list[3], ""], ...scan(list[4], rules)] as Line;
    const label = /^([A-Z][A-Za-z ]{1,24}:)(\s.*)$/.exec(l);
    if (label) return [[label[1], "key"], ...scan(label[2], rules)] as Line;
    return scan(l, rules);
  });
}

export function highlight(code: string, lang?: string): Line[] {
  switch (normalizeLang(lang)) {
    case "bash":
      return bash(code);
    case "json":
      return json(code);
    case "toml":
      return toml(code);
    case "yaml":
      return yaml(code);
    case "diff":
      return diff(code);
    case "prompt":
      return prompt(code);
    default:
      return code.split("\n").map((l) => [[l, ""]] as Line);
  }
}
