/**
 * Wording and small decisions of the "Nhật ký hệ thống" tab (ADR 0054). Pure: no network, no DOM.
 *
 * THE ACTION COLUMN SHOWS THE RAW VERB (`khoa_tai_khoan_can_bo`), deliberately. A Vietnamese label
 * table typed here would be a second copy of Go constants spread over five services, and it would
 * drift without a test turning red; the right source is each service answering its own labels, and
 * that is ADR 0054 §Còn mở #3, still open.
 */

import type { AuditFilter, AuditSourceKey } from "@/lib/api/audit-entries";
import type { audit_EntryView, JsonValue } from "@/lib/api/schema.gen";

/**
 * The "phân hệ" each source is, in the words of the contract's own route summaries
 * ("Nhật ký hệ thống của phân hệ …" in `kb/20-contracts/openapi.json`).
 */
export const AUDIT_SOURCE_LABEL: Readonly<Record<AuditSourceKey, string>> = {
  identity: "Tổ chức & tài khoản",
  documents: "Văn bản",
  finance: "Tài chính",
  comms: "Thông tin – truyền thông",
  petitions: "Tiếp dân – Nhiệm vụ",
};

/** §7: a source that did not answer is NAMED — a trail missing a module must not look complete. */
export function sourceErrorLine(source: AuditSourceKey, message: string): string {
  return `Không tải được nhật ký của phân hệ ${AUDIT_SOURCE_LABEL[source]}: ${message}`;
}

/**
 * "Người thực hiện". The server sends the staff code, `system`, or — for a citizen — an EMPTY code on
 * purpose (ADR 0054 §4: the stored id names nobody but links one citizen's acts together). So a
 * citizen row says "Công dân", never an empty cell that reads as a missing value.
 */
export function actorLabel(e: Pick<audit_EntryView, "actor_kind" | "actor_code">): string {
  if (e.actor_kind === "citizen") return "Công dân";
  if (e.actor_kind === "system" || e.actor_code === "system") return "Hệ thống";
  return e.actor_code !== "" ? e.actor_code : "Không có mã người thực hiện";
}

/** "Địa chỉ IP". Empty for a citizen by decision (ADR 0054 §4, open item #2), said as such. */
export function ipLabel(e: Pick<audit_EntryView, "actor_kind" | "actor_ip">): string {
  if (e.actor_kind === "citizen") return "Không hiển thị";
  return e.actor_ip !== "" ? e.actor_ip : "—";
}

const TIME_ZONE = "Asia/Ho_Chi_Minh";
const TIME_FORMAT = new Intl.DateTimeFormat("vi-VN", {
  timeZone: TIME_ZONE,
  hour: "2-digit",
  minute: "2-digit",
  second: "2-digit",
  hour12: false,
});
const DATE_FORMAT = new Intl.DateTimeFormat("vi-VN", {
  timeZone: TIME_ZONE,
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
});

/** `14:05:09 22/09/2026`, pinned to Asia/Ho_Chi_Minh — never the machine's zone. */
export function atLabel(at: string): string {
  const d = new Date(at);
  if (Number.isNaN(d.getTime())) return "Mốc thời gian không đọc được";
  return `${TIME_FORMAT.format(d)} ${DATE_FORMAT.format(d)}`;
}

/**
 * The delta as TEXT, pretty-printed, or `null` when the entry has none. Rendered inside a `<pre>` as a
 * React text node — never as HTML (rule 13, invariant 3): a delta holds what staff typed.
 */
export function deltaText(delta: JsonValue): string | null {
  if (delta === null) return null;
  if (typeof delta === "object" && !Array.isArray(delta) && Object.keys(delta).length === 0) {
    return null;
  }
  return JSON.stringify(delta, null, 2);
}

/** The filter form as typed: dates are `yyyy-mm-dd` from `<input type="date">`. */
export type AuditFilterDraft = {
  readonly fromDate: string;
  readonly toDate: string;
  readonly actor: string;
  readonly action: string;
  readonly subject: string;
};

export const EMPTY_AUDIT_FILTER: AuditFilterDraft = {
  fromDate: "",
  toDate: "",
  actor: "",
  action: "",
  subject: "",
};

const DATE_RE = /^(\d{4})-(\d{2})-(\d{2})$/;
const ZONE_OFFSET = "+07:00";

/** `yyyy-mm-dd` of the day after, by calendar arithmetic (UTC fields only — no local zone involved). */
function nextDay(date: string): string | null {
  const m = DATE_RE.exec(date);
  if (m === null) return null;
  const d = new Date(Date.UTC(Number(m[1]), Number(m[2]) - 1, Number(m[3]) + 1));
  if (Number.isNaN(d.getTime())) return null;
  const p = (n: number) => String(n).padStart(2, "0");
  return `${d.getUTCFullYear()}-${p(d.getUTCMonth() + 1)}-${p(d.getUTCDate())}`;
}

export const DATE_ORDER_REFUSED = "Ngày bắt đầu phải trước hoặc bằng ngày kết thúc.";
export const DATE_UNREADABLE = "Ngày lọc không đọc được. Vui lòng chọn lại ngày.";

/**
 * The form → the query the five services read.
 *
 * DATES ARE WHOLE DAYS IN VIET NAM, sent as a half-open `[from, to)` (ADR 0054 §4, ADR 0053 §3):
 * "từ 01/09 đến 30/09" is `from=2026-09-01T00:00:00+07:00`, `to=2026-10-01T00:00:00+07:00` — the
 * last chosen day is included whole. A reversed range is refused HERE with one sentence, rather than
 * as five identical 400s, one per module.
 */
export function filterFromDraft(
  d: AuditFilterDraft,
): { ok: true; filter: AuditFilter } | { ok: false; message: string } {
  const fromDate = d.fromDate.trim();
  const toDate = d.toDate.trim();
  if (fromDate !== "" && !DATE_RE.test(fromDate)) return { ok: false, message: DATE_UNREADABLE };
  const toExclusive = toDate === "" ? "" : nextDay(toDate);
  if (toExclusive === null) return { ok: false, message: DATE_UNREADABLE };
  if (fromDate !== "" && toDate !== "" && fromDate > toDate) {
    return { ok: false, message: DATE_ORDER_REFUSED };
  }
  return {
    ok: true,
    filter: {
      from: fromDate === "" ? undefined : `${fromDate}T00:00:00${ZONE_OFFSET}`,
      to: toExclusive === "" ? undefined : `${toExclusive}T00:00:00${ZONE_OFFSET}`,
      actor: d.actor.trim() || undefined,
      action: d.action.trim() || undefined,
      subject: d.subject.trim() || undefined,
    },
  };
}
