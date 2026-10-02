/**
 * Nhận đường dẫn "bấm số liệu → mở danh sách" từ trang Tổng quan (SRS M7.2.2 P0: mọi con số mở ra
 * đúng danh sách đứng sau nó, và số dòng bằng con số). Người dùng chốt 28/09/2026: "Làm luôn bộ nhận".
 *
 * HỢP ĐỒNG ĐƯỜNG DẪN — trang Tổng quan dựng đúng khuôn này:
 *
 *   /nhiem-vu?metric=<m>[&from=<RFC3339>&to=<RFC3339>]   → GET /api/v1/tasks
 *   /phan-anh?metric=<m>[&from&to]                        → GET /api/v1/citizen-reports
 *   /van-ban?metric=<m>[&from&to]                         → GET /api/v1/incoming-documents
 *
 * MỘT HÀM THUẦN CHO CẢ BA MÀN, không ba bản: phép "số liệu nào cần kỳ, số liệu nào cấm kỳ" là thứ
 * ba bản sao sẽ trôi khỏi nhau, và bản trôi sẽ là bản mở cả quyển sổ dưới một tiêu đề nói một con
 * số. Bảng số liệu dưới đây chép TÊN từ máy chủ — `service-petitions/internal/domain/
 * summary_metrics.go:56-66,106-115` và `service-documents/internal/domain/incoming_dashboard.go:44-51`
 * — vì hợp đồng sinh ra (`schema.gen.ts`) chỉ khai `metric?: string`, không khai enum.
 *
 * FAIL CLOSED, KHÔNG SỬA HỘ: số liệu lạ, thiếu kỳ ở số liệu theo kỳ, có kỳ ở số liệu tồn, mốc không
 * đúng RFC3339, `from` không trước `to`, một tham số lặp hai lần — tất cả là `invalid`, và màn hình
 * KHÔNG gửi gì của đường dẫn ấy lên máy chủ. Nó hiện danh sách thường kèm một câu nói rõ đường dẫn
 * hỏng; không bao giờ hiện cả quyển sổ như thể đã lọc.
 *
 * Tham số KHÔNG mang dữ liệu cá nhân: tên một số liệu và hai mốc thời gian. Đó là lý do đọc chúng từ
 * đường dẫn không đụng tới lệnh cấm URL của `ngan-van-ban-den.test.tsx` (luật 3, cấm #4).
 */

export type DrillDownRegister = "tasks" | "citizen-reports" | "incoming-documents";

type MetricSpec = { readonly label: string; readonly period: boolean };

/** Nhãn tiếng Việt và "có cần kỳ không" của từng số liệu — nhãn đúng như trang Tổng quan in. */
const METRICS = {
  tasks: {
    in_progress: { label: "Đang thực hiện", period: false },
    overdue: { label: "Quá hạn", period: false },
    suspended: { label: "Tạm dừng", period: false },
    completed: { label: "Hoàn thành trong kỳ", period: true },
    on_time_sample: { label: "Có hạn trong kỳ", period: true },
    on_time: { label: "Hoàn thành đúng hạn", period: true },
  },
  "citizen-reports": {
    in_progress: { label: "Đang xử lý", period: false },
    received: { label: "Nhận vào trong kỳ", period: true },
    on_time_sample: { label: "Có hạn trong kỳ", period: true },
    on_time: { label: "Đúng hạn", period: true },
    late: { label: "Trễ hạn trong kỳ", period: true },
    // STOCK, the three figures of docs/ui-ux/09 §3 cards 3–4 (`summary_metrics.go:116-132`): the
    // register as it stands, like the `rating_max` filter the low-rating list is.
    rating_sample: { label: "Được người dân chấm điểm", period: false },
    low_rating: { label: "Bị đánh giá thấp", period: false },
    publication_pending: { label: "Chờ kiểm duyệt", period: false },
  },
  "incoming-documents": {
    arrived: { label: "Đến trong kỳ", period: true },
    open: { label: "Chưa xử lý xong", period: false },
    overdue: { label: "Quá hạn xử lý", period: false },
  },
} as const satisfies Record<DrillDownRegister, Record<string, MetricSpec>>;

export type DrillDownMetric<R extends DrillDownRegister> = Extract<keyof (typeof METRICS)[R], string>;
export type TaskMetric = DrillDownMetric<"tasks">;
export type CitizenReportMetric = DrillDownMetric<"citizen-reports">;
export type IncomingDocumentMetric = DrillDownMetric<"incoming-documents">;

/**
 * Câu hiện khi đường dẫn mang tham số lọc mà không dùng được. KHÔNG nói "toàn bộ danh sách": sổ văn bản
 * đến vẫn lọc theo năm hiện tại khi mở bình thường, nên câu ấy sai ở đó (rà cách ly 28/09/2026).
 */
export const INVALID_DRILL_DOWN_LINE =
  "Đường dẫn lọc không hợp lệ — đang hiện danh sách như khi mở màn bình thường.";

/** Chữ thay cho kỳ ở số liệu tồn: nó đếm quyển sổ như đang đứng, không trong khoảng nào. */
export const STOCK_PERIOD_LABEL = "tính đến hiện tại";

/** Kết quả đọc của một màn, theo KIỂU SỐ LIỆU của màn ấy (`DrillDown<R>` bên dưới). */
export type DrillDownOf<M extends string> =
  /** Đường dẫn không mang `metric`, `from` hay `to` — màn hình như thường. */
  | { readonly kind: "none" }
  /** Có mang, nhưng không dùng được. Không gửi gì của nó lên máy chủ. */
  | { readonly kind: "invalid" }
  | {
      readonly kind: "active";
      readonly metric: M;
      readonly label: string;
      /** `null` ở số liệu tồn. Chuỗi GỐC của đường dẫn, gửi lại nguyên văn cho máy chủ. */
      readonly period: { readonly from: string; readonly to: string } | null;
      /** `dd/MM/yyyy–dd/MM/yyyy`, hoặc `STOCK_PERIOD_LABEL`. */
      readonly periodLabel: string;
    };

export type DrillDown<R extends DrillDownRegister> = DrillDownOf<DrillDownMetric<R>>;

export const NO_DRILL_DOWN = { kind: "none" } as const;

/** Hình dạng `searchParams` của trang Next — mỗi khoá có thể là một chuỗi, một mảng, hoặc vắng. */
export type RawSearchParams = Readonly<Record<string, string | readonly string[] | undefined>>;

/** Ba tham số của hợp đồng. Chỉ ba cái này được đọc; mọi tham số khác không phải việc của hàm này. */
const DRILL_DOWN_KEYS = ["metric", "from", "to"] as const;

/**
 * Đọc ba tham số lọc của một màn danh sách.
 *
 * Vắng cả ba ⇒ `none`. Có bất kỳ cái nào ⇒ hoặc `active` hợp lệ trọn vẹn, hoặc `invalid` — không có
 * nửa chừng. `from`/`to` KHÔNG kèm `metric` cũng là `invalid`: một kỳ không gắn số liệu nào không có
 * nghĩa, và máy chủ văn bản trả 400 cho đúng ca ấy (`http/incoming_dashboard.go:115-121`).
 */
export function parseDrillDown<R extends DrillDownRegister>(
  register: R,
  params: RawSearchParams,
): DrillDown<R> {
  const raw: Partial<Record<(typeof DRILL_DOWN_KEYS)[number], string>> = {};
  let present = false;
  for (const k of DRILL_DOWN_KEYS) {
    const v = params[k];
    if (v === undefined) continue;
    present = true;
    // Một tham số lặp hai lần là hai câu trả lời cho một câu hỏi. Chọn một là đoán.
    if (typeof v !== "string") return { kind: "invalid" };
    raw[k] = v;
  }
  if (!present) return NO_DRILL_DOWN;

  const metric = raw.metric;
  const spec = metric === undefined ? undefined : specOf(register, metric);
  if (metric === undefined || spec === undefined) return { kind: "invalid" };

  const hasFrom = raw.from !== undefined;
  const hasTo = raw.to !== undefined;
  if (!spec.period) {
    // Số liệu tồn KHÔNG nhận kỳ: máy chủ văn bản trả 400, máy chủ nhiệm vụ lặng lẽ bỏ qua — và một
    // tiêu đề in một kỳ mà danh sách không lọc theo nó là một tiêu đề nói sai.
    if (hasFrom || hasTo) return { kind: "invalid" };
    return {
      kind: "active",
      metric: metric as DrillDownMetric<R>,
      label: spec.label,
      period: null,
      periodLabel: STOCK_PERIOD_LABEL,
    };
  }

  if (raw.from === undefined || raw.to === undefined) return { kind: "invalid" };
  const fromMs = parseRfc3339(raw.from);
  const toMs = parseRfc3339(raw.to);
  // Nửa mở [from, to): `from` phải TRƯỚC HẲN `to`. Máy chủ từ chối, không đổi chỗ hai mốc.
  if (fromMs === null || toMs === null || fromMs >= toMs) return { kind: "invalid" };

  return {
    kind: "active",
    metric: metric as DrillDownMetric<R>,
    label: spec.label,
    period: { from: raw.from, to: raw.to },
    periodLabel: formatPeriod(fromMs, toMs),
  };
}

/** Số liệu này có đếm trong một kỳ không. Bộ gọi API dùng để KHÔNG gửi kỳ cho số liệu tồn. */
export function isPeriodMetric<R extends DrillDownRegister>(
  register: R,
  metric: DrillDownMetric<R>,
): boolean {
  return specOf(register, metric)?.period === true;
}

/** Tra một số liệu trong bảng; `undefined` khi nó không phải số liệu của sổ này. */
function specOf(register: DrillDownRegister, metric: string): MetricSpec | undefined {
  const table: Readonly<Record<string, MetricSpec>> = METRICS[register];
  // `hasOwn`, không `in`: `constructor`, `toString` là khoá của nguyên mẫu, không phải số liệu.
  return Object.hasOwn(table, metric) ? table[metric] : undefined;
}

/** Ba trường truyền vào bộ lọc của bộ gọi API; rỗng khi không có lọc hợp lệ. */
export function drillDownQuery<M extends string>(
  d: DrillDownOf<M>,
): { metric?: M; from?: string; to?: string } {
  if (d.kind !== "active") return {};
  return d.period === null
    ? { metric: d.metric }
    : { metric: d.metric, from: d.period.from, to: d.period.to };
}

/** Khoá `key` của React: đổi lọc là dựng lại màn từ đầu — trang, ngăn xếp con trỏ, bản nháp. */
export function drillDownKey(d: DrillDownOf<string>): string {
  if (d.kind !== "active") return d.kind;
  return `${d.metric}|${d.period?.from ?? ""}|${d.period?.to ?? ""}`;
}

/* ---- RFC3339 ---------------------------------------------------------------------------- */

const RFC3339 =
  /^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d{1,9}))?(?:Z|([+-])(\d{2}):(\d{2}))$/;

/**
 * Mốc RFC3339 → mili giây, hoặc `null`.
 *
 * KHÔNG DÙNG `Date.parse` MỘT MÌNH: nó nhận `2026-02-30` (lăn sang tháng 3), giờ `24`, và dạng có
 * dấu cách thay chữ `T` — cả ba bị `time.Parse(time.RFC3339, …)` của máy chủ từ chối. Một mốc máy
 * chủ từ chối mà màn hình nhận là một tiêu đề in một kỳ còn danh sách hiện trang lỗi 400.
 *
 * ⚠ DẤU `+` CỦA MÚI GIỜ PHẢI ĐƯỢC MÃ HOÁ (`%2B`) TRONG ĐƯỜNG DẪN: không mã hoá thì trình duyệt đọc nó
 * thành dấu cách và mốc thành không hợp lệ ở đây. Không "sửa hộ" dấu cách thành `+` — đó là đoán.
 */
export function parseRfc3339(s: string): number | null {
  const m = RFC3339.exec(s);
  if (m === null) return null;
  // Khuôn đã bảo đảm mọi nhóm số có mặt; `Number(undefined)` là NaN và rơi ở các phép so dưới.
  const n = (i: number): number => Number(m[i]);
  const [year, month, day, hour, minute, second] = [n(1), n(2), n(3), n(4), n(5), n(6)];
  if (month < 1 || month > 12) return null;
  const daysInMonth = new Date(Date.UTC(year, month, 0)).getUTCDate();
  if (day < 1 || day > daysInMonth) return null;
  if (hour > 23 || minute > 59 || second > 59) return null;

  let offsetMinutes = 0;
  if (m[8] !== undefined) {
    const oh = n(9);
    const om = n(10);
    if (oh > 23 || om > 59) return null;
    offsetMinutes = (m[8] === "-" ? -1 : 1) * (oh * 60 + om);
  }
  const millis = m[7] === undefined ? 0 : Number(m[7].slice(0, 3).padEnd(3, "0"));

  const utc = new Date(0);
  // setUTCFullYear thay vì Date.UTC: Date.UTC đọc năm 0–99 thành 1900–1999.
  utc.setUTCFullYear(year, month - 1, day);
  utc.setUTCHours(hour, minute, second, millis);
  return utc.getTime() - offsetMinutes * 60_000;
}

/* ---- in kỳ -------------------------------------------------------------------------------- */

/**
 * Múi giờ GHIM — hằng của nền tảng, không phải giá trị của một xã: Việt Nam một múi giờ cho cả 300
 * xã. Trang Tổng quan tính kỳ theo đúng múi này (`summary_metrics.go:24-28`).
 */
const ADMINISTRATIVE_TIME_ZONE = "Asia/Ho_Chi_Minh";

const DATE_FORMAT = new Intl.DateTimeFormat("vi-VN", {
  timeZone: ADMINISTRATIVE_TIME_ZONE,
  day: "2-digit",
  month: "2-digit",
  year: "numeric",
});

/** Một mốc → `dd/MM/yyyy`, ghép từ từng phần để không phụ thuộc dấu phân cách của bản địa hoá. */
function formatDate(ms: number): string {
  const parts = DATE_FORMAT.formatToParts(new Date(ms));
  const get = (t: Intl.DateTimeFormatPartTypes) => parts.find((p) => p.type === t)?.value ?? "";
  return `${get("day")}/${get("month")}/${get("year")}`;
}

/**
 * Kỳ nửa mở [from, to) → `dd/MM/yyyy–dd/MM/yyyy`.
 *
 * NGÀY CUỐI LÀ NGÀY CỦA `to − 1ms`: `to` không thuộc kỳ. Tuần kết thúc ở 00:00 thứ Hai in ra ngày
 * Chủ nhật, không phải ngày thứ Hai — thứ Hai ấy thuộc kỳ sau.
 */
export function formatPeriod(fromMs: number, toMs: number): string {
  return `${formatDate(fromMs)}–${formatDate(toMs - 1)}`;
}
