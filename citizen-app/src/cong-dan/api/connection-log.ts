/**
 * THE ONE `console` CALL OF THE STATE HALF — a failed call to ViGov, in the `--demo` build only.
 *
 * Owner, 01/10/2026: a petition send failed on a real phone with "mạng yếu hoặc mất kết nối", and that
 * sentence covers a CORS refusal, a timeout and a broken answer alike. The tester reads the Zalo debug
 * console; nothing is drawn on screen (the owner chose "console only, no button").
 *
 * ⚠ THE RECORD HAS A FIXED SHAPE AND IT CARRIES NO PERSONAL DATA (rule 3). Never a body, a header, a
 * token, a full URL or a lookup code — the body holds a citizen's name, phone and petition text. Only:
 * which route, which method, the host called, the page's own origin (what CORS compares), the HTTP status
 * or the error's NAME, and the elapsed time. An error MESSAGE is kept only for `TypeError`/`AbortError`
 * — the network failures — because a `SyntaxError` from `response.json()` quotes the start of the body.
 *
 * ⚠ KEEP THE GUARD A BARE `DEMO_BUILD`: it is inlined as a literal, the branch folds away, and the normal
 * bundle carries no `console.warn` at all (`demo-build.ts`). `cong-dan.test.tsx` exempts THIS file only.
 */
import { DEMO_BUILD } from "../../lib/demo-build";

export type ConnectionRoute =
  | "report-fields"
  | "send-petition"
  | "lookup-petition"
  | "my-petitions"
  | "rate-petition"
  | "commune-lookup"
  | "commune-staff"
  | "commune-profile"
  | "news"
  | "news-categories"
  | "news-item"
  | "banners"
  | "photo-slot"
  | "photo-storage"
  | "photo-complete"
  | "photo-list"
  | "verification-photo-list";

export type ConnectionFailure = {
  readonly route: ConnectionRoute;
  readonly method: "GET" | "POST";
  /** Host only — `new URL(address).host`. The path can carry a lookup code. */
  readonly host: string;
  /** What the branch became — `loi-mang`, `loi-may-chu`, … */
  readonly outcome: string;
  /** The HTTP status when the server answered; absent when nothing came back. */
  readonly status?: number;
  /** The thrown value's `name` when something threw. */
  readonly error?: string;
  readonly elapsed_ms: number;
};

/** Network-failure messages ("Failed to fetch", "Load failed", "signal is aborted…") quote no body. */
const MESSAGE_KEPT_FOR: ReadonlySet<string> = new Set(["TypeError", "AbortError"]);
const MESSAGE_MAX = 160;

/** Whether a thrown value is the network failing (fetch's `TypeError`, the timeout's `AbortError`). */
export function isNetworkFailure(thrown: unknown): boolean {
  const name = typeof thrown === "object" && thrown !== null ? (thrown as { name?: unknown }).name : undefined;
  return typeof name === "string" && MESSAGE_KEPT_FOR.has(name);
}

/** `name` and, for network failures only, a capped `message` of whatever was thrown. */
export function describeThrown(thrown: unknown): string {
  if (typeof thrown !== "object" || thrown === null) return typeof thrown;
  const name = typeof (thrown as { name?: unknown }).name === "string" ? (thrown as { name: string }).name : "Error";
  const message = (thrown as { message?: unknown }).message;
  if (!MESSAGE_KEPT_FOR.has(name) || typeof message !== "string") return name;
  return `${name}: ${message.slice(0, MESSAGE_MAX)}`;
}

/** The host of an address, or "" — never the address itself. */
export function hostOf(address: string): string {
  try {
    return new URL(address).host;
  } catch {
    return "";
  }
}

export function logConnectionFailure(failure: ConnectionFailure): void {
  if (!DEMO_BUILD) return;
  const origin = typeof location === "undefined" ? "" : location.origin;
  const online = typeof navigator === "undefined" ? undefined : navigator.onLine;
  console.warn("[ViGov] kết nối lỗi", { ...failure, origin, online, at: new Date().toISOString() });
}
