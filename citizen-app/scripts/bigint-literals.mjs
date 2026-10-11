// BIGINT LITERALS → `BigInt("…")` CALLS, in MapLibre only (ADR 0072 §Sửa đổi 11/10/2026 (Mini App), the send form's map).
//
// WHY: the Mini App is ONE classic script (`vite.config.ts`), so ONE token the phone's JavaScript engine cannot parse
// is a SyntaxError for the WHOLE app — a white screen on every screen, not just "no map". maplibre-gl 6 carries eight
// `123n` literals (its MLT tile decoder), and the build target (es2015) cannot lower them: oxc reports them as
// TOLERATED_TRANSFORM and ships them as-is. Engines before Safari 14 / Chrome 67 cannot parse that.
//
// A `BigInt("123")` call PARSES everywhere and only fails if it RUNS — and it runs only inside MLT decoding, a tile
// format this app never requests (the archive is MVT). On such an old engine the map then fails like any map failure
// (no map, one sentence; its module worker would not start there anyway) while the rest of the app keeps working.
//
// AST-BASED, NOT A REGEX: minified code holds `2n`-shaped text inside strings and CSS (`:nth-child(2n)`); only real
// BigInt literal nodes are rewritten. Each span is checked to read exactly like a BigInt literal before it is replaced,
// so an offset mismatch STOPS the build instead of corrupting the bundle. `bundle-for-zalo.test.ts` asserts that the
// emitted bundle holds no BigInt literal at all.
import { parseAst } from "vite";

const TARGET = /[\\/]maplibre-gl[\\/]dist[\\/]maplibre-gl\.mjs$/;
const LITERAL = /^(?:0[xXoObB][0-9a-fA-F_]+|[0-9_]+)n$/;

/** Every BigInt literal node's `[start, end]` in `code`. */
export function bigintLiteralSpans(code) {
  const spans = [];
  const walk = (node) => {
    if (node === null || typeof node !== "object") return;
    if (Array.isArray(node)) {
      for (const child of node) walk(child);
      return;
    }
    if (node.type === "Literal" && typeof node.bigint === "string") spans.push([node.start, node.end]);
    for (const key of Object.keys(node)) {
      if (key !== "type" && key !== "start" && key !== "end") walk(node[key]);
    }
  };
  walk(parseAst(code));
  return spans;
}

/** `code` with every BigInt literal replaced by `BigInt("<digits>")`. Throws if a span does not read as one. */
export function bigintLiteralsToCalls(code) {
  const spans = bigintLiteralSpans(code).sort((a, b) => b[0] - a[0]);
  let out = code;
  for (const [start, end] of spans) {
    const text = out.slice(start, end);
    if (!LITERAL.test(text)) {
      throw new Error(`bigint-literals: span ${start}..${end} reads "${text.slice(0, 20)}", not a BigInt literal — refusing to rewrite`);
    }
    out = `${out.slice(0, start)}BigInt(${JSON.stringify(text.slice(0, -1).replace(/_/g, ""))})${out.slice(end)}`;
  }
  return out;
}

export const bigintLiteralsPlugin = {
  name: "maplibre-bigint-literals",
  enforce: "pre",
  transform(code, id) {
    if (!TARGET.test(id.split("?")[0])) return null;
    return { code: bigintLiteralsToCalls(code), map: null };
  },
};
