/**
 * TEST DATA ONLY — imported by `*.test.ts(x)` files, never by the screen.
 *
 * What `allowed_transitions` holds on a row in each status, as the server returns it (3b2330b,
 * `service-petitions/internal/domain/nhiem_vu.go:97-105`). The screen has NO copy of this table: it
 * reads the list from each row (`hasTransition`). Tests need rows that look like server replies, and
 * one fixture keeps three test files from carrying three copies that drift apart.
 *
 * If the server's map changes, this fixture going stale makes no test lie about the SCREEN — it
 * only makes the sample rows old.
 */
export const SERVER_TRANSITIONS: Readonly<Record<string, readonly string[]>> = {
  "moi-giao": ["da-tiep-nhan", "dang-thuc-hien", "tam-dung"],
  "da-tiep-nhan": ["dang-thuc-hien", "tam-dung"],
  "dang-thuc-hien": ["cho-duyet", "hoan-thanh", "tam-dung"],
  "cho-duyet": ["hoan-thanh", "dang-thuc-hien"],
  "tam-dung": ["moi-giao", "da-tiep-nhan", "dang-thuc-hien"],
  "chuyen-tiep": ["da-tiep-nhan", "dang-thuc-hien"],
  "hoan-thanh": ["dang-thuc-hien"],
};

/** The server's list for `status` — a fresh array; `[]` for a code the server does not know. */
export function serverTransitions(status: string): string[] {
  return [...(SERVER_TRANSITIONS[status] ?? [])];
}
