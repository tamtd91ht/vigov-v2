/**
 * PHẢN ÁNH TRONG APP RIÊNG CỦA XÃ — trên sổ phản ánh THẬT của xã (29/09/2026, thay bản trải nghiệm chỉ
 * trong bộ nhớ). Giao diện theo ADR 0050: chỗ xung đột theo kho yêu cầu, còn lại theo prototype khách
 * (`../vigov-require/apps/miniapp` — `NewFeedbackPage`, `FeedbackDetailPage`, `StatusChip`, `RatingBlock`).
 *
 * DỮ LIỆU ĐI QUA ĐÚNG CLIENT CỦA APP CHUNG (`api/vigov-client.ts`: `submitReport`, `myReports`, `lookupReport`,
 * `rateReport` qua `CitizenReportRating`) — một client, hai giao diện. Không tệp nào ở đây gọi mạng hay cầm
 * bearer: phiên nằm ở `api/vigov-session.ts`, và chỉ mở ở việc cá nhân đầu tiên, sau lời giải thích
 * (`commune-session.ts`, gọi từ `CommuneHome.tsx`).
 *
 *   · Gửi: Lĩnh vực → Mô tả (tên xã đọc lại ngay trên nút gửi — README §Non-negotiables #5) → Đã gửi, với
 *     MÃ TRA CỨU máy chủ cấp (luật 10 #1). Một lần bấm = một `Idempotency-Key` (`api/send-attempt.ts`): "Gửi
 *     lại" sau mạng rớt dùng lại cùng khoá, nên không bao giờ thành hai phiếu.
 *   · Bước 1 là DANH MỤC LĨNH VỰC CỦA XÃ (`/my-citizen-report-fields`, tải khi màn mở — tức SAU cổng),
 *     theo thứ tự của xã; mã chọn được đi lên thành `field`. Không có danh sách dự phòng (ADR 0060 §3): 503
 *     là một câu kèm "Thử lại". 400 `field_not_offered` (xã vừa đổi danh mục) → tải lại, chọn lại.
 *   · Người dân thấy BỐN nhóm trạng thái (`status-groups.ts`); mã lạ hiện câu trung tính, không đoán nhóm,
 *     không in mã thô — ở chip, ở danh sách lọc, ở dòng thời gian.
 *   · 401 / 403 `chua_xac_thuc_so`: phiên bị quên (`dropCommuneAppSession`) và việc đang làm đi lại qua cổng
 *     (`onSessionLost`) — cùng lần gửi, cùng khoá.
 *   · Nháp đang soạn giữ trên máy (ADR 0050 #7) qua `draftStore` lớp vỏ tiêm; tệp này không chạm kho lưu trữ.
 *     Toạ độ KHÔNG vào nháp.
 *
 * Mọi ô nhập đi qua `input-field.tsx` — tệp duy nhất của nửa nhà nước được có ô nhập.
 */
import { type ReactNode, useEffect, useMemo, useRef, useState } from "react";

import {
  citizenReportFields,
  submitReport,
  type ListResult,
  type CallResult,
  myReports,
  lookupReport,
} from "../api/vigov-client";
import {
  type CitizenField,
  MAX_LENGTH,
  type MyReport,
  type MyReportSummary,
  type SceneLocation,
  submitReportBody,
} from "../api/citizen-report-contract";
import { type SendAttempt, createSendAttempt } from "../api/send-attempt";
import type { ReopenWithPhone } from "../api/open-vigov-session";
import { vnDateTime } from "../../lib/date-time";

import { Icon, type IconName } from "./Icon";
import { fieldLabelOf } from "./frame";
import { SubScreenHeader, StatusBlock, SubPage } from "./commune-frame";
import {
  MY_REPORTS,
  SEND,
  statusExplanation,
  CHANNEL_NOT_OPEN,
  EMERGENCY,
  SEND_ERROR,
  statusLabel,
  REPORT_CARD,
  LOOKUP,
  COMMUNE_APP_REPORTS,
  COMMUNE_APP_SCREENS,
} from "./copy";
import { TextAreaField, TextField } from "./input-field";
import { CitizenReportRating } from "./CitizenReportRating";
import {
  type GetSceneLocation,
  SceneLocationControl,
  type SceneLocationWords,
  useSceneLocation,
} from "./scene-location";
import {
  type FeedbackDraftStore,
  checkDraft,
  type DraftError,
  COMMUNE_APP_STEP_LABEL,
  COMMUNE_APP_GROUP_LABEL,
  communeAppGroupOf,
  type FilterGroup,
  type ReportDraft,
  LIFECYCLE,
} from "./commune-app-model";

/**
 * What a screen does when the server says the session is gone (401) or unverified (403
 * `chua_xac_thuc_so`): hand the act to `CommuneHome`, which forgets the session and runs the act again through
 * the gate. `retry` repeats EXACTLY the act (same body, same key).
 */
export type OnSessionLost = (retry: () => void) => void;

/**
 * Nhãn một trong BỐN nhóm người dân thấy (ADR 0050 #5). An unknown code gets the shared app's neutral
 * sentence (`statusLabel`) and a neutral style — never a guessed group, never the raw code.
 */
export function StatusChip({ status }: { status: string }) {
  const group = communeAppGroupOf(status);
  if (group === null) return <span className="xa-chip-tt xa-chip-tt--unknown">{statusLabel(status)}</span>;
  return <span className={`xa-chip-tt xa-chip-tt--${group}`}>{COMMUNE_APP_GROUP_LABEL[group]}</span>;
}

const at = (iso: string) => vnDateTime(iso) ?? "";

/** One row of "Phản ánh của tôi" — on the home screen and in the list. */
export function CommuneReportCard({ report, onOpen }: { report: MyReportSummary; onOpen: () => void }) {
  return (
    <button type="button" className="xa-the xa-hang-tin" onClick={onOpen}>
      <span className="xa-o-bt xa-mau--hong" aria-hidden="true">
        <Icon name="chat" size={24} />
      </span>
      <span className="xa-hang-tin__chu">
        <span className="xa-phu">
          #{report.lookup_code} · {fieldLabelOf(report.field, report.field_label)}
        </span>
        <strong className="xa-hang-tin__tieu-de xa-cat-2">{report.content_excerpt}</strong>
        <span className="xa-phu">{at(report.clock_from)}</span>
        <span>
          <StatusChip status={report.status} />
        </span>
      </span>
      <Icon name="right" size={20} />
    </button>
  );
}

/* ═══════════════════════════════ PHẢN ÁNH CỦA TÔI — trạng thái danh sách ═══════════════════════════════ */

export type ListFailure = "server" | "network" | "closed";

/**
 * The list of THIS citizen's petitions, as loaded in this open. `idle` = not loaded (no session yet, or it
 * was dropped): the screens then offer the gate, never a guessed empty list.
 */
export type MyReportsState =
  | { readonly kind: "idle" }
  | { readonly kind: "loading" }
  | { readonly kind: "failed"; readonly failure: ListFailure }
  | {
      readonly kind: "ready";
      readonly items: readonly MyReportSummary[];
      readonly cursor: string;
      readonly hasMore: boolean;
      readonly loadingMore: boolean;
      readonly moreFailure: ListFailure | null;
    };

/** One list call's result → failure kind, `"session"` (gate again), or the page. PURE. */
export function listOutcome(result: ListResult): ListFailure | "session" | Extract<ListResult, { kind: "xong" }> {
  switch (result.kind) {
    case "xong":
      return result;
    case "chua-co-phien":
    case "het-phien":
    case "can-xac-thuc-so":
      return "session";
    case "chua-cau-hinh":
      return "closed";
    case "loi-mang":
      return "network";
    default:
      return "server";
  }
}

export function listFailureText(f: ListFailure): string {
  if (f === "network") return MY_REPORTS.network_error;
  if (f === "closed") return CHANNEL_NOT_OPEN.text;
  return MY_REPORTS.server_error;
}

/**
 * The list, loaded page by page through `myReports` — no "whose" parameter: citizen and commune come
 * from the session on the server (rule 4 forbidden #1, rule 1 forbidden #2).
 *
 * The in-flight guard is a ref, not state: StrictMode runs effects twice, and two first pages appended
 * would show every petition twice.
 */
export function useMyReports(onSessionLost: OnSessionLost) {
  const [state, setState] = useState<MyReportsState>({ kind: "idle" });
  const busy = useRef(false);
  const current = useRef<MyReportsState>(state);
  current.current = state;

  async function load() {
    if (busy.current) return;
    busy.current = true;
    setState({ kind: "loading" });
    const out = listOutcome(await myReports(""));
    busy.current = false;
    if (out === "session") {
      setState({ kind: "idle" });
      onSessionLost(() => void load());
      return;
    }
    if (typeof out === "string") {
      setState({ kind: "failed", failure: out });
      return;
    }
    setState({
      kind: "ready",
      items: out.page.entries,
      cursor: out.page.cursor,
      hasMore: out.page.has_more,
      loadingMore: false,
      moreFailure: null,
    });
  }

  async function loadMore() {
    const s = current.current;
    if (busy.current || s.kind !== "ready" || !s.hasMore) return;
    busy.current = true;
    setState({ ...s, loadingMore: true, moreFailure: null });
    const out = listOutcome(await myReports(s.cursor));
    busy.current = false;
    if (out === "session") {
      setState({ ...s, loadingMore: false });
      onSessionLost(() => void loadMore());
      return;
    }
    if (typeof out === "string") {
      setState({ ...s, loadingMore: false, moreFailure: out });
      return;
    }
    setState({
      kind: "ready",
      items: [...s.items, ...out.page.entries],
      cursor: out.page.cursor,
      hasMore: out.page.has_more,
      loadingMore: false,
      moreFailure: null,
    });
  }

  /** Load once if nothing is loaded yet — after the gate opened a session for any personal act. */
  function loadIfIdle() {
    if (current.current.kind === "idle") void load();
  }

  return { state, load, loadMore, loadIfIdle };
}

/** "Hơn n" when more pages exist: a count of loaded rows is a floor, never presented as the total. */
export function countLabel(n: number, hasMore: boolean): string {
  return hasMore ? `${n}+` : String(n);
}

/* ═══════════════════════════════ DANH SÁCH ═══════════════════════════════ */

const GROUPS: readonly FilterGroup[] = ["tat-ca", "da-tiep-nhan", "dang-xu-ly", "da-xu-ly-xong", "da-dong"];
const filterLabel = (group: FilterGroup) =>
  group === "tat-ca" ? COMMUNE_APP_SCREENS.filter_all : COMMUNE_APP_GROUP_LABEL[group];

/** The card that stands where the list will be until the citizen asks to see it (no session yet). */
export function NeedSessionCard({ onOpen }: { onOpen: () => void }) {
  return (
    <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-can-phien">
      <h2 className="xa-dau-khoi__tieu-de" id="xa-can-phien">
        {COMMUNE_APP_REPORTS.need_session_title}
      </h2>
      <p>{COMMUNE_APP_REPORTS.need_session_body}</p>
      <button type="button" className="xa-nut" onClick={onOpen}>
        {COMMUNE_APP_REPORTS.need_session_button}
      </button>
    </section>
  );
}

/** Loading / failed / idle for both the home block and the list. `null` when the list is ready. */
export function ListStatus(props: { state: MyReportsState; onOpen: () => void; onRetry: () => void }) {
  const { state } = props;
  if (state.kind === "idle") return <NeedSessionCard onOpen={props.onOpen} />;
  if (state.kind === "loading") return <StatusBlock icon="chat" text={MY_REPORTS.loading} loading />;
  if (state.kind === "failed") {
    return (
      <StatusBlock
        icon="alert"
        error
        text={listFailureText(state.failure)}
        button={state.failure === "closed" ? undefined : { label: MY_REPORTS.retry_button, onPress: props.onRetry }}
      />
    );
  }
  return null;
}

export function CommuneReportList(props: {
  state: MyReportsState;
  onOpenReport: (code: string) => void;
  onLookup: () => void;
  /** No session yet: the citizen asks to see the list (goes through the gate). */
  onOpen: () => void;
  onRetry: () => void;
  onLoadMore: () => void;
}) {
  const { state } = props;
  const [filter, setFilter] = useState<FilterGroup>("tat-ca");
  const items = state.kind === "ready" ? state.items : [];
  const shown = useMemo(
    () => (filter === "tat-ca" ? items : items.filter((p) => communeAppGroupOf(p.status) === filter)),
    [items, filter],
  );
  const count = (group: FilterGroup) =>
    group === "tat-ca" ? items.length : items.filter((p) => communeAppGroupOf(p.status) === group).length;

  return (
    <>
      <button type="button" className="xa-the xa-hang xa-hang--vien" onClick={props.onLookup}>
        <span className="xa-o-bt xa-mau--xanh" aria-hidden="true">
          <Icon name="search" size={24} />
        </span>
        <span className="xa-hang__chu">
          <strong>{COMMUNE_APP_REPORTS.lookup_title}</strong>
          <span className="xa-phu">{COMMUNE_APP_REPORTS.lookup_hint}</span>
        </span>
        <Icon name="right" size={20} />
      </button>
      <ListStatus state={state} onOpen={props.onOpen} onRetry={props.onRetry} />
      {state.kind === "ready" && (
        <>
          <div className="xa-chips" role="group" aria-label={COMMUNE_APP_SCREENS.filter_group}>
            {GROUPS.map((k) => (
              <button
                key={k}
                type="button"
                className={`xa-chip${filter === k ? " xa-chip--on" : ""}`}
                aria-pressed={filter === k}
                onClick={() => setFilter(k)}
              >
                {filterLabel(k)} ({countLabel(count(k), state.hasMore)})
              </button>
            ))}
          </div>
          {shown.length === 0 ? (
            <StatusBlock icon="chat" text={items.length === 0 ? COMMUNE_APP_REPORTS.no_reports_yet : COMMUNE_APP_SCREENS.filter_empty} />
          ) : (
            <ul className="xa-ds">
              {shown.map((p) => (
                <li key={p.lookup_code}>
                  <CommuneReportCard report={p} onOpen={() => props.onOpenReport(p.lookup_code)} />
                </li>
              ))}
            </ul>
          )}
          {state.moreFailure !== null && (
            <p className="xa-loi-o" role="alert">
              {listFailureText(state.moreFailure)}
            </p>
          )}
          {state.loadingMore && (
            <p className="xa-phu" role="status">
              {MY_REPORTS.loading_more}
            </p>
          )}
          {state.hasMore && !state.loadingMore && (
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onLoadMore}>
              {MY_REPORTS.load_more_button}
            </button>
          )}
        </>
      )}
    </>
  );
}

/* ═══════════════════════════════ CHI TIẾT ═══════════════════════════════ */

function Row({ label, children }: { label: string; children: ReactNode }) {
  return (
    <>
      <p className="xa-nhan-o">{label}</p>
      <div>{children}</div>
    </>
  );
}

/**
 * Dòng thời gian — CHỈ các bước đã qua (prototype: sự kiện đã xảy ra, `feedback-adapter.ts:128-137`), nhãn
 * của prototype. Hai nhánh kết thúc dừng sau "Đang phân loại". THUẦN, xuất để test.
 */
export function stepsPassed(status: string): string[] {
  if (status === "khong-tiep-nhan" || status === "chuyen-cap-tren") return ["da-tiep-nhan", "dang-phan-loai", status];
  const i = LIFECYCLE.indexOf(status);
  return i < 0 ? [status] : LIFECYCLE.slice(0, i + 1);
}

/** A step's words: its label, or — for a code this app does not know — the neutral sentence, never the code. */
export function stepLabel(status: string): string {
  return Object.prototype.hasOwnProperty.call(COMMUNE_APP_STEP_LABEL, status) ? COMMUNE_APP_STEP_LABEL[status]! : statusLabel(status);
}

export function TimelineRow({ report }: { report: MyReport }) {
  const steps = stepsPassed(report.status);
  return (
    <ol className="xa-dong-tg">
      {steps.map((status, i) => {
        const last = i === steps.length - 1;
        return (
          <li
            key={status}
            className={`xa-dong-tg__buoc ${last ? "xa-dong-tg__buoc--dang" : "xa-dong-tg__buoc--qua"}`}
            aria-current={last ? "step" : undefined}
          >
            <strong>{stepLabel(status)}</strong>
            <span className="xa-phu">{COMMUNE_APP_REPORTS.handling_unit}</span>
            {i === 0 && <span className="xa-phu">{at(report.clock_from)}</span>}
            {last && statusExplanation(status) && <span className="xa-phu">{statusExplanation(status)}</span>}
          </li>
        );
      })}
    </ol>
  );
}

/**
 * The petition as the server returns it to its sender — no staff notes, no routing history (rule 4
 * forbidden #5). Rating is `CitizenReportRating`, the same block as the shared app: one network path, one rule.
 */
export function CommuneReportBody(props: {
  report: MyReport;
  /** Absent = read-only (no rating block), e.g. in a test. */
  onRated?: (report: MyReport) => void;
  onReload?: () => void;
  reopenWithPhone?: ReopenWithPhone;
}) {
  const p = props.report;
  const sender = p.anonymous
    ? COMMUNE_APP_REPORTS.hide_name
    : [p.masked_reporter_name || REPORT_CARD.name_not_given, p.masked_reporter_phone].filter(Boolean).join(" · ");
  return (
    <>
      <div className="xa-the xa-the--dem xa-khoi">
        <p className="xa-phu">
          {COMMUNE_APP_REPORTS.report_code}: <strong className="xa-ma">#{p.lookup_code}</strong>
        </p>
        <Row label={REPORT_CARD.status}>
          <StatusChip status={p.status} />
        </Row>
        <Row label={REPORT_CARD.field}>{fieldLabelOf(p.field, p.field_label)}</Row>
        <Row label={REPORT_CARD.sent_at}>{at(p.clock_from)}</Row>
        {/* The server's stored deadline, verbatim — never computed here (rule 10 #2, #4). */}
        <Row label={COMMUNE_APP_REPORTS.expected_done}>{p.resolve_due ? at(p.resolve_due) : REPORT_CARD.due_at_not_yet}</Row>
        <div className="xa-ke" />
        <Row label={REPORT_CARD.content}>
          <p className="xa-giu-dong">{p.content}</p>
        </Row>
        <Row label={REPORT_CARD.address}>{p.address || REPORT_CARD.address_empty}</Row>
        <Row label={REPORT_CARD.reporter}>{sender}</Row>
        {p.result !== "" && (
          <Row label={REPORT_CARD.result}>
            <p className="xa-giu-dong">{p.result}</p>
          </Row>
        )}
        {p.status === "khong-tiep-nhan" && <Row label={REPORT_CARD.rejection_reason}>{p.reason || COMMUNE_APP_REPORTS.not_recorded}</Row>}
        {p.status === "chuyen-cap-tren" && (
          <>
            <Row label={REPORT_CARD.receiving_body_label}>{p.receiving_body || COMMUNE_APP_REPORTS.not_recorded}</Row>
            <Row label={REPORT_CARD.referral_reason}>{p.reason || COMMUNE_APP_REPORTS.not_recorded}</Row>
          </>
        )}
      </div>
      <div className="xa-the xa-the--dem xa-khoi">
        <h2 className="xa-dau-khoi__tieu-de">{COMMUNE_APP_REPORTS.progress}</h2>
        <TimelineRow report={p} />
      </div>
      {props.onRated && props.onReload && (
        // `key` by status: a server-side change (a reopen after a low rating) starts a fresh block.
        <CitizenReportRating
          key={`${p.lookup_code}:${p.status}`}
          report={p}
          onRated={props.onRated}
          onReload={props.onReload}
          reopenWithPhone={props.reopenWithPhone}
        />
      )}
    </>
  );
}

/** Result of reading ONE petition by code → what the screen shows. PURE. */
export type LookupOutcome =
  | { readonly kind: "found"; readonly report: MyReport }
  | { readonly kind: "session" }
  | { readonly kind: "failed"; readonly text: string };

export function lookupOutcome(result: CallResult): LookupOutcome {
  switch (result.kind) {
    case "xong":
      return { kind: "found", report: result.report };
    case "chua-co-phien":
    case "het-phien":
    case "can-xac-thuc-so":
      return { kind: "session" };
    case "khong-thay":
      return { kind: "failed", text: COMMUNE_APP_REPORTS.report_not_found };
    case "chua-cau-hinh":
      return { kind: "failed", text: CHANNEL_NOT_OPEN.text };
    case "loi-mang":
      return { kind: "failed", text: LOOKUP.network_error };
    default:
      return { kind: "failed", text: LOOKUP.server_error };
  }
}

type DetailState =
  | { readonly kind: "loading" }
  | { readonly kind: "session" }
  | { readonly kind: "failed"; readonly text: string }
  | { readonly kind: "found"; readonly report: MyReport };

export function CommuneReportDetail(props: {
  code: string;
  onBack: () => void;
  onSessionLost: OnSessionLost;
  /** A rating or reload changed the petition — the list refreshes. */
  onChanged: () => void;
  reopenWithPhone?: ReopenWithPhone;
}) {
  const { code } = props;
  const [state, setState] = useState<DetailState>({ kind: "loading" });
  const [round, setRound] = useState(0);

  useEffect(() => {
    let alive = true;
    setState({ kind: "loading" });
    void lookupReport(code).then((result) => {
      if (!alive) return;
      const out = lookupOutcome(result);
      setState(out);
      if (out.kind === "session") props.onSessionLost(() => setRound((n) => n + 1));
    });
    return () => {
      alive = false;
    };
    // `round` is the reload trigger; `code` is fixed for this screen.
  }, [code, round]);

  const reload = () => setRound((n) => n + 1);

  return (
    <>
      <SubScreenHeader title={COMMUNE_APP_REPORTS.detail_title} onBack={props.onBack} />
      <SubPage>
        {state.kind === "loading" && <StatusBlock icon="chat" text={COMMUNE_APP_REPORTS.loading_ticket} loading />}
        {state.kind === "session" && (
          <StatusBlock icon="alert" error text={COMMUNE_APP_REPORTS.session_expired} button={{ label: MY_REPORTS.retry_button, onPress: reload }} />
        )}
        {state.kind === "failed" && (
          <StatusBlock icon="alert" error text={state.text} button={{ label: MY_REPORTS.retry_button, onPress: reload }} />
        )}
        {state.kind === "found" && (
          <CommuneReportBody
            report={state.report}
            onRated={(p) => {
              setState({ kind: "found", report: p });
              props.onChanged();
            }}
            onReload={reload}
            reopenWithPhone={props.reopenWithPhone}
          />
        )}
      </SubPage>
    </>
  );
}

/* ═══════════════════════════════ TRA CỨU PHIẾU ═══════════════════════════════ */

/** The typed code, as the route takes it: trimmed, a leading "#" (as the card shows it) removed. */
export function normaliseCode(typed: string): string {
  return typed.trim().replace(/^#/, "").trim();
}

export function CommuneReportLookup(props: {
  onBack: () => void;
  onSessionLost: OnSessionLost;
  onChanged: () => void;
  reopenWithPhone?: ReopenWithPhone;
}) {
  const [code, setCode] = useState("");
  const [state, setState] = useState<DetailState | { readonly kind: "missing" } | null>(null);

  async function search(value: string) {
    if (value === "") {
      setState({ kind: "missing" });
      return;
    }
    setState({ kind: "loading" });
    const out = lookupOutcome(await lookupReport(value));
    setState(out);
    if (out.kind === "session") props.onSessionLost(() => void search(value));
  }

  return (
    <>
      <SubScreenHeader title={COMMUNE_APP_REPORTS.lookup_title} onBack={props.onBack} />
      <SubPage>
        <div className="xa-the xa-the--dem xa-khoi">
          <TextField id="xa-ma-tra-cuu" label={COMMUNE_APP_REPORTS.report_code} suggestion={COMMUNE_APP_REPORTS.lookup_hint} value={code} max={40} onChange={setCode} />
          <button
            type="button"
            className="xa-nut"
            disabled={state?.kind === "loading"}
            onClick={() => void search(normaliseCode(code))}
          >
            <Icon name="search" size={20} />
            {LOOKUP.lookup_button}
          </button>
        </div>
        {state?.kind === "missing" && <StatusBlock icon="info" error text={COMMUNE_APP_REPORTS.missing_code} />}
        {state?.kind === "loading" && <StatusBlock icon="search" text={LOOKUP.looking_up} loading />}
        {state?.kind === "session" && <StatusBlock icon="alert" error text={COMMUNE_APP_REPORTS.session_expired} />}
        {state?.kind === "failed" && <StatusBlock icon="search" error text={state.text} />}
        {state?.kind === "found" && (
          <CommuneReportBody
            report={state.report}
            onRated={(p) => {
              setState({ kind: "found", report: p });
              props.onChanged();
            }}
            onReload={() => void search(state.report.lookup_code)}
            reopenWithPhone={props.reopenWithPhone}
          />
        )}
      </SubPage>
    </>
  );
}

/* ═══════════════════════════════ GỬI — Lĩnh vực → Mô tả → Đã gửi ═══════════════════════════════ */

function StepBar({ step }: { step: 1 | 2 | 3 }) {
  const labels = [COMMUNE_APP_SCREENS.step_field, COMMUNE_APP_REPORTS.step_description, COMMUNE_APP_REPORTS.step_done];
  return (
    <ol className="xa-thanh-buoc" aria-label={COMMUNE_APP_SCREENS.step(step, 3)}>
      {labels.map((label, i) => {
        const number = i + 1;
        const status = number < step ? "xong" : number === step ? "dang" : "cho";
        return (
          <li key={label} className={`xa-thanh-buoc__muc xa-thanh-buoc__muc--${status}`} aria-current={status === "dang" ? "step" : undefined}>
            <span className="xa-thanh-buoc__cham">{status === "xong" ? <Icon name="check" size={16} /> : number}</span>
            <span>{label}</span>
          </li>
        );
      })}
    </ol>
  );
}

/** "Bà con" words for the shared location control (`scene-location.tsx`), from `COMMUNE_APP_SCREENS` / `COMMUNE_APP_REPORTS`. */
export const COMMUNE_LOCATION_WORDS: SceneLocationWords = {
  button: COMMUNE_APP_SCREENS.location_button,
  button_again: COMMUNE_APP_SCREENS.location_again,
  locating: COMMUNE_APP_SCREENS.location_locating,
  why: COMMUNE_APP_REPORTS.location_why,
  found: COMMUNE_APP_SCREENS.location_found,
  failures: {
    "tu-choi": COMMUNE_APP_SCREENS.location_denied,
    "ngoai-zalo": COMMUNE_APP_SCREENS.location_outside_zalo,
    "qua-nhieu-lan": COMMUNE_APP_SCREENS.location_rate_limited,
    "thu-lai": COMMUNE_APP_SCREENS.location_retry,
    "tam-ngung": COMMUNE_APP_SCREENS.location_unavailable,
  },
};

/* ───────────── THE COMMUNE'S FIELD CATALOGUE (step 1) — `GET /api/v1/my-citizen-report-fields` ───────────── */

export type CatalogueFailure = "unavailable" | "network" | "server" | "closed";

/**
 * The fields this commune offers on the form, loaded when the send screen opens — which is always AFTER
 * the gate, because the route needs a session. There is NO built-in list to fall back on (ADR 0060 §3):
 * failed means "say so, offer Thử lại", never "show twelve names the commune may not use".
 */
export type Catalogue =
  | { readonly kind: "loading" }
  | { readonly kind: "failed"; readonly failure: CatalogueFailure }
  | { readonly kind: "ready"; readonly fields: readonly CitizenField[] };

/** One catalogue call's result → the next state, or `"session"` (gate again). PURE. */
export function catalogueOutcome(result: Awaited<ReturnType<typeof citizenReportFields>>): Catalogue | "session" {
  switch (result.kind) {
    case "xong":
      return { kind: "ready", fields: result.fields };
    case "chua-co-phien":
    case "het-phien":
    case "can-xac-thuc-so":
      return "session";
    case "field-catalogue-unavailable":
      return { kind: "failed", failure: "unavailable" };
    case "chua-cau-hinh":
      return { kind: "failed", failure: "closed" };
    case "loi-mang":
      return { kind: "failed", failure: "network" };
    default:
      return { kind: "failed", failure: "server" };
  }
}

export function catalogueFailureText(f: CatalogueFailure): string {
  switch (f) {
    case "unavailable":
      return COMMUNE_APP_REPORTS.fields_unavailable;
    case "network":
      return COMMUNE_APP_REPORTS.fields_network;
    case "closed":
      return CHANNEL_NOT_OPEN.text;
    case "server":
      return COMMUNE_APP_REPORTS.fields_server;
  }
}

/** The catalogue of this send screen. Loaded once on mount (ref guard: StrictMode runs effects twice). */
function useFieldCatalogue(onSessionLost: OnSessionLost) {
  const [catalogue, setCatalogue] = useState<Catalogue>({ kind: "loading" });
  const busy = useRef(false);

  async function load() {
    if (busy.current) return;
    busy.current = true;
    setCatalogue({ kind: "loading" });
    const next = catalogueOutcome(await citizenReportFields());
    busy.current = false;
    if (next === "session") {
      onSessionLost(() => void load());
      return;
    }
    setCatalogue(next);
  }

  const started = useRef(false);
  useEffect(() => {
    if (started.current) return;
    started.current = true;
    void load();
  }, []);

  return { catalogue, reload: () => void load() };
}

/** The codes the commune offers now, or `null` while not known (loading / failed). */
export function offeredCodes(c: Catalogue): readonly string[] | null {
  return c.kind === "ready" ? c.fields.map((f) => f.code) : null;
}

/**
 * The platform's lucide icon names (`service-platform/migrations/0011_petition_field.sql:134-145`) drawn by
 * this app's own icon set, where it has a matching drawing. Anything else — including a name added to the
 * platform later — gets the NEUTRAL icon, never a guessed one.
 */
const FIELD_ICON: Readonly<Record<string, IconName>> = {
  ShieldAlert: "shield",
  MessageSquare: "chat",
  Construction: "build",
  Hammer: "build",
};

/** The platform's six tones → this app's measured colour classes (`xa-mau--*`); unknown/absent → neutral. */
const FIELD_TONE: Readonly<Record<string, string>> = {
  blue: "xanh",
  cyan: "xanh",
  green: "luc",
  orange: "cam",
  purple: "tim",
  red: "hong",
};

export function fieldIcon(icon: string | null): IconName {
  return icon !== null && Object.prototype.hasOwnProperty.call(FIELD_ICON, icon) ? FIELD_ICON[icon]! : "text";
}

export function fieldTone(tone: string | null): string {
  return tone !== null && Object.prototype.hasOwnProperty.call(FIELD_TONE, tone) ? FIELD_TONE[tone]! : "navy";
}

/**
 * "Tiếp tục" on a found draft → the form to show and the step to open. PURE, exported for tests.
 *
 * The draft keeps a field CODE. It is restored only while it is still OFFERED (`offered`): a code the
 * commune has since switched off — or a name left by the old temporary list — is dropped and the citizen
 * picks again on step 1. `offered` null (catalogue not loaded yet): the code is kept for now, and the send
 * screen drops it the moment the catalogue arrives without it. Otherwise step 2, the writing step, as the
 * prototype does (`NewFeedbackPage.tsx:222`). An empty name in the draft (an anonymous one keeps none)
 * falls back to the name taken from Zalo in this session.
 *
 * The draft's field names (`linh_vuc`, `noi_dung`, …) are the keys already written to phones under
 * `vigov.feedback.draft.v1`; they change only with a `v2` key and a read of the old one.
 */
export function restoreDraft(
  draft: ReportDraft,
  zaloName: string | null,
  offered: readonly string[] | null,
): { form: ReportDraft; step: 1 | 2 } {
  const field = offered === null || offered.includes(draft.linh_vuc) ? draft.linh_vuc : "";
  return {
    form: {
      linh_vuc: field,
      noi_dung: draft.noi_dung,
      dia_chi: draft.dia_chi,
      ho_ten: draft.ho_ten !== "" ? draft.ho_ten : (zaloName ?? ""),
      dien_thoai: draft.dien_thoai,
      an_danh: draft.an_danh,
    },
    step: field !== "" ? 2 : 1,
  };
}

/**
 * The form a fresh send screen opens with. PURE, exported for tests. The name field starts with the Zalo
 * name taken at entry, or EMPTY — never a placeholder name: the field is required when not anonymous
 * (`checkDraft`), so an empty field makes the citizen type it, while a guessed one would be sent.
 * The name PRE-FILLS; it grants nothing — who sent it is the session's citizen, on the server (rule 4).
 */
export function blankForm(nameFromEntry: string | null): ReportDraft {
  return {
    linh_vuc: "",
    noi_dung: "",
    dia_chi: "",
    ho_ten: nameFromEntry ?? "",
    dien_thoai: "",
    // Gửi ẩn danh là tuỳ chọn của bà con (SRS M4.2, ADR 0050 #3): bật thì không gửi họ tên, số điện thoại.
    an_danh: false,
  };
}

/**
 * The request body for this form + location. PURE. The five fields of the contract, `field` = the CODE
 * picked on step 1 (from the commune's catalogue), plus `lat`/`lng` only when the citizen tapped for them.
 */
export function sendBody(form: ReportDraft, location: SceneLocation | null): string {
  return submitReportBody({
    content: form.noi_dung,
    address: form.dia_chi,
    full_name: form.ho_ten,
    phone: form.dien_thoai,
    anonymous: form.an_danh,
    scene_location: location,
    field: form.linh_vuc,
  });
}

type SendFailure = keyof typeof SEND_ERROR;

/** One send's result → what the screen does next. PURE. */
export type SendOutcome =
  | { readonly kind: "sent"; readonly report: MyReport }
  | { readonly kind: "session" }
  | { readonly kind: "failed"; readonly failure: SendFailure };

export function sendOutcome(result: CallResult): SendOutcome {
  switch (result.kind) {
    case "xong":
      return { kind: "sent", report: result.report };
    case "chua-co-phien":
    case "het-phien":
    case "can-xac-thuc-so":
      // Asked again through the gate with the SAME attempt: a 401/403 is answered before anything is
      // recorded, so the retry is this send, not a second petition.
      return { kind: "session" };
    case "chua-cau-hinh":
      return { kind: "failed", failure: "kenh-chua-mo" };
    case "khong-thay":
      return { kind: "failed", failure: "loi-may-chu" };
    default:
      return { kind: "failed", failure: result.kind };
  }
}

/**
 * Step 1 — the commune's own fields, in its order, with the platform's icon and tone where this app can
 * draw them (neutral otherwise). Loading and failures are WORDS with a next step; an empty catalogue says
 * the commune takes no petitions through the app — it never invents a list to pick from.
 */
export function FieldStep(props: {
  catalogue: Catalogue;
  picked: string;
  /** The last send was refused with `field_not_offered`: explain before the list. */
  fieldChanged: boolean;
  onPick: (code: string) => void;
  onRetry: () => void;
}) {
  const { catalogue } = props;
  if (catalogue.kind === "loading") return <StatusBlock icon="text" text={COMMUNE_APP_REPORTS.fields_loading} loading />;
  if (catalogue.kind === "failed") {
    return (
      <StatusBlock
        icon="alert"
        error
        text={catalogueFailureText(catalogue.failure)}
        button={catalogue.failure === "closed" ? undefined : { label: MY_REPORTS.retry_button, onPress: props.onRetry }}
      />
    );
  }
  if (catalogue.fields.length === 0) return <StatusBlock icon="info" text={COMMUNE_APP_REPORTS.fields_empty} />;
  return (
    <div className="xa-the xa-the--dem xa-khoi">
      {props.fieldChanged && (
        <p className="xa-loi-o" role="alert">
          {SEND_ERROR["field-not-offered"].text}
        </p>
      )}
      <p>{COMMUNE_APP_REPORTS.choose_field}</p>
      <div className="xa-luoi-lv" role="radiogroup" aria-label={COMMUNE_APP_SCREENS.step_field}>
        {catalogue.fields.map((f) => (
          <button
            key={f.code}
            type="button"
            role="radio"
            aria-checked={props.picked === f.code}
            className={`xa-o-lv${props.picked === f.code ? " xa-o-lv--on" : ""}`}
            onClick={() => props.onPick(f.code)}
          >
            <span className={`xa-o-bt xa-mau--${fieldTone(f.tone)}`} aria-hidden="true">
              <Icon name={fieldIcon(f.icon)} size={22} />
            </span>
            {f.label}
          </button>
        ))}
      </div>
    </div>
  );
}

export function CommuneSendScreen(props: {
  commune_name: string;
  /** Họ tên lấy từ Zalo lúc mở app, hoặc `null`. CHỈ để điền sẵn — màn này không gọi Zalo. */
  full_name: string | null;
  /** The location exchange, injected by the shell; absent = no location button (outside Zalo, tests). */
  getSceneLocation?: GetSceneLocation;
  /** Draft kept on the phone (ADR 0050 #7) — commune app only. Absent: no draft at all. */
  draftStore?: FeedbackDraftStore;
  onBack: () => void;
  onSessionLost: OnSessionLost;
  /** 201 — the petition as the server recorded it. */
  onSent: (report: MyReport) => void;
  onOpenReport: (code: string) => void;
}) {
  const { draftStore } = props;
  const [step, setStep] = useState<1 | 2 | 3>(1);
  /**
   * A draft found when the screen opened, until the citizen picks "Tiếp tục" or "Bỏ nháp". While it is
   * pending the form is hidden and nothing is saved, so the old draft cannot be overwritten by a new one
   * before the citizen has answered (the prototype asks first, `NewFeedbackPage.tsx:79`).
   */
  const [draftOffer, setDraftOffer] = useState<ReportDraft | null>(() => draftStore?.load() ?? null);
  const [form, setForm] = useState<ReportDraft>(() => blankForm(props.full_name));
  const [location, setLocation] = useState<SceneLocation | null>(null);
  const [errors, setErrors] = useState<DraftError>({});
  const [confirmCancel, setConfirmCancel] = useState(false);
  /** The attempt in flight — kept across "Gửi lại" and the gate, dropped when what is sent changes. */
  const [attempt, setAttempt] = useState<SendAttempt | null>(null);
  const [sending, setSending] = useState(false);
  const [failure, setFailure] = useState<SendFailure | null>(null);
  const [sent, setSent] = useState<MyReport | null>(null);
  /** The server said the picked field is no longer offered: say so on step 1 while the citizen re-picks. */
  const [fieldChanged, setFieldChanged] = useState(false);
  const { catalogue, reload } = useFieldCatalogue(props.onSessionLost);
  const sceneLocation = useSceneLocation(props.getSceneLocation, (l) => {
    setLocation(l);
    setAttempt(null);
  });

  // A picked code the catalogue does not (or no longer) offer is dropped as soon as the catalogue is known —
  // a restored draft's code, or one the commune switched off. Back to step 1; what was written stays.
  useEffect(() => {
    const offered = offeredCodes(catalogue);
    if (offered === null || form.linh_vuc === "" || offered.includes(form.linh_vuc)) return;
    setForm((current) => ({ ...current, linh_vuc: "" }));
    setAttempt(null);
    setStep((s) => (s === 2 ? 1 : s));
  }, [catalogue, form.linh_vuc]);

  const pickedLabel =
    catalogue.kind === "ready" ? (catalogue.fields.find((f) => f.code === form.linh_vuc)?.label ?? "") : "";

  const edit = (k: "noi_dung" | "dia_chi" | "ho_ten" | "dien_thoai") => (v: string) => {
    setForm((current) => ({ ...current, [k]: v }));
    setAttempt(null);
    setFailure(null);
  };
  const hasContent = form.linh_vuc !== "" || form.noi_dung.trim() !== "" || form.dia_chi.trim() !== "";

  // Save as the citizen types, on the writing step only (the prototype's rule, `NewFeedbackPage.tsx:124`).
  // Not while a found draft is still waiting for an answer, and not after sending (step 3).
  useEffect(() => {
    if (draftStore === undefined || draftOffer !== null || step !== 2 || form.linh_vuc === "") return;
    draftStore.save(form);
  }, [draftStore, draftOffer, step, form]);

  function resumeDraft() {
    if (draftOffer === null) return;
    const restored = restoreDraft(draftOffer, props.full_name, offeredCodes(catalogue));
    setForm(restored.form);
    setDraftOffer(null);
    setStep(restored.step);
  }

  function discardDraft() {
    draftStore?.clear();
    setDraftOffer(null);
  }

  /** "Huỷ bỏ" in the cancel dialog: what was typed is dropped — on the phone too. */
  function cancelFeedback() {
    draftStore?.clear();
    props.onBack();
  }

  async function send(a: SendAttempt) {
    setSending(true);
    setFailure(null);
    const out = sendOutcome(await submitReport(a));
    setSending(false);
    if (out.kind === "session") {
      props.onSessionLost(() => void send(a));
      return;
    }
    if (out.kind === "failed" && out.failure === "field-not-offered") {
      // The commune changed its list since it was loaded. Reload it and let the citizen pick again; the
      // attempt is dropped — the next send has a different body, so it is a different act (`send-attempt.ts`).
      setAttempt(null);
      setForm((current) => ({ ...current, linh_vuc: "" }));
      setFieldChanged(true);
      setStep(1);
      reload();
      return;
    }
    if (out.kind === "failed") {
      setFailure(out.failure);
      return;
    }
    setAttempt(null);
    draftStore?.clear();
    setSent(out.report);
    setStep(3);
    props.onSent(out.report);
  }

  function submit() {
    if (sending) return;
    const e = checkDraft(form, {
      missing: COMMUNE_APP_REPORTS.missing_description,
      missing_reporter: COMMUNE_APP_REPORTS.missing_reporter,
      too_long: (n) => SEND.too_long(COMMUNE_APP_SCREENS.this_tile, n),
    });
    setErrors(e);
    if (Object.keys(e).length > 0) return;
    let a = attempt;
    if (a === null) {
      try {
        a = createSendAttempt(sendBody(form, location));
      } catch {
        setFailure("khong-tao-duoc-khoa");
        return;
      }
      setAttempt(a);
    }
    void send(a);
  }

  function back() {
    if (step === 2) return setStep(1);
    if (step === 1 && hasContent) return setConfirmCancel(true);
    props.onBack();
  }

  const failed = failure === null ? null : SEND_ERROR[failure];

  return (
    <>
      <SubScreenHeader title={SEND.title} onBack={step === 3 ? props.onBack : back} />
      <StepBar step={step} />
      <SubPage>
        {confirmCancel && (
          <div className="xa-the xa-the--dem xa-khoi" role="alertdialog" aria-label={COMMUNE_APP_SCREENS.cancel_title}>
            <h2 className="xa-dau-khoi__tieu-de">{COMMUNE_APP_SCREENS.cancel_title}</h2>
            <p>{COMMUNE_APP_REPORTS.cancel_question}</p>
            <button type="button" className="xa-nut" onClick={() => setConfirmCancel(false)}>
              {COMMUNE_APP_SCREENS.continue_editing}
            </button>
            <button type="button" className="xa-nut xa-nut--phu xa-nut--do-vien" onClick={cancelFeedback}>
              {COMMUNE_APP_SCREENS.cancel}
            </button>
          </div>
        )}

        {draftOffer !== null && (
          <section className="xa-the xa-the--dem xa-khoi" aria-labelledby="xa-nhap-tieu-de">
            <h2 className="xa-dau-khoi__tieu-de" id="xa-nhap-tieu-de">
              {COMMUNE_APP_REPORTS.draft_title}
            </h2>
            <p>{COMMUNE_APP_REPORTS.draft_body}</p>
            <p className="xa-phu">{COMMUNE_APP_REPORTS.draft_kept_on_phone}</p>
            <button type="button" className="xa-nut" onClick={resumeDraft}>
              {COMMUNE_APP_REPORTS.draft_resume}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={discardDraft}>
              {COMMUNE_APP_REPORTS.draft_discard}
            </button>
          </section>
        )}

        {step === 1 && draftOffer === null && (
          <FieldStep
            catalogue={catalogue}
            picked={form.linh_vuc}
            fieldChanged={fieldChanged}
            onRetry={reload}
            onPick={(code) => {
              setForm((current) => ({ ...current, linh_vuc: code }));
              setAttempt(null);
              setFieldChanged(false);
              setStep(2);
            }}
          />
        )}

        {step === 2 && (
          <div className="xa-the xa-the--dem xa-khoi">
            <TextAreaField id="xa-noi-dung" label={COMMUNE_APP_REPORTS.incident} suggestion={COMMUNE_APP_REPORTS.hint_incident} value={form.noi_dung} max={MAX_LENGTH.content} required onChange={edit("noi_dung")} />
            {errors.noi_dung && <p className="xa-loi-o" role="alert">{errors.noi_dung}</p>}
            <div className="xa-hang xa-hang--tinh xa-hang--sat xa-lv-dang">
              <span className="xa-hang__chu">
                <span>
                  {COMMUNE_APP_REPORTS.field}: <strong>{pickedLabel}</strong>
                </span>
              </span>
              <button type="button" className="xa-dau-khoi__them" onClick={() => setStep(1)}>
                {COMMUNE_APP_SCREENS.change}
              </button>
            </div>
            {/* SRS M4.2 bắt buộc ảnh/video và vị trí trên bản đồ. Ảnh CHƯA có; vị trí hiện tại lấy được
                nhưng chưa có bản đồ. Không chặn nút gửi vì hai ô ấy (xem `checkDraft`). */}
            <p className="xa-nhan-o">{COMMUNE_APP_REPORTS.photo_required}</p>
            <p className="xa-phu">{COMMUNE_APP_REPORTS.photo_coming_soon}</p>
            <p className="xa-nhan-o">{COMMUNE_APP_REPORTS.location_required}</p>
            <p className="xa-phu">{COMMUNE_APP_REPORTS.location_coming_soon}</p>
            <TextField id="xa-dia-chi" label={COMMUNE_APP_REPORTS.address} suggestion={COMMUNE_APP_REPORTS.hint_address} value={form.dia_chi} max={MAX_LENGTH.address} onChange={edit("dia_chi")} />
            {errors.dia_chi && <p className="xa-loi-o" role="alert">{errors.dia_chi}</p>}
            {props.getSceneLocation && (
              <SceneLocationControl
                words={COMMUNE_LOCATION_WORDS}
                look="commune"
                locating={sceneLocation.locating}
                location={location}
                failure={sceneLocation.failure}
                onLocate={() => void sceneLocation.locate()}
              />
            )}
            <div className="xa-hang xa-hang--tinh xa-hang--sat">
              <span className="xa-hang__chu">
                <strong>{COMMUNE_APP_REPORTS.anonymous}</strong>
                <span className="xa-phu">{COMMUNE_APP_REPORTS.anonymous_explanation}</span>
              </span>
              <button
                type="button"
                role="switch"
                aria-checked={form.an_danh}
                aria-label={COMMUNE_APP_REPORTS.anonymous}
                className={`xa-cong-tac${form.an_danh ? " xa-cong-tac--bat" : ""}`}
                onClick={() => {
                  setForm((current) => ({ ...current, an_danh: !current.an_danh }));
                  setAttempt(null);
                }}
              >
                <span className="xa-cong-tac__nut" />
              </button>
            </div>
            {!form.an_danh && (
              <>
                <TextField id="xa-ho-ten" label={COMMUNE_APP_REPORTS.reporter_name} suggestion={COMMUNE_APP_REPORTS.hint_name} value={form.ho_ten} max={MAX_LENGTH.full_name} onChange={edit("ho_ten")} />
                {errors.ho_ten && <p className="xa-loi-o" role="alert">{errors.ho_ten}</p>}
                <TextField id="xa-dien-thoai" label={COMMUNE_APP_REPORTS.phone_number} suggestion={COMMUNE_APP_REPORTS.hint_phone} value={form.dien_thoai} max={MAX_LENGTH.phone} input_mode="tel" onChange={edit("dien_thoai")} />
                {errors.dien_thoai && <p className="xa-loi-o" role="alert">{errors.dien_thoai}</p>}
              </>
            )}
            <p className="xa-phu">{form.an_danh ? COMMUNE_APP_REPORTS.required_anonymous : COMMUNE_APP_REPORTS.required}</p>
            {draftStore !== undefined && <p className="xa-phu">{COMMUNE_APP_REPORTS.draft_kept_on_phone}</p>}
            <div className="xa-ghi-chu">
              <Icon name="alert" size={22} />
              <p>{EMERGENCY}</p>
            </div>
            {/* THE COMMUNE, READ AGAIN AT THE LAST STEP (README §Non-negotiables #5): the name on the header,
                which the session was checked against when it opened (`openCommuneAppSession`). */}
            <p className="xa-xa-nhan">{COMMUNE_APP_REPORTS.send_to(props.commune_name)}</p>
            {failed !== null && (
              <p className="xa-loi-o" role="alert">
                {failed.text}
              </p>
            )}
            {sending && (
              <p className="xa-phu" role="status">
                {COMMUNE_APP_REPORTS.sending}
              </p>
            )}
            {failed !== null && failed.can_resend && attempt !== null ? (
              // Same key, same body — never a second petition (`api/send-attempt.ts`).
              <button type="button" className="xa-nut xa-nut--hong" disabled={sending} onClick={() => void send(attempt)}>
                <Icon name="send" size={20} />
                {SEND.resend_button}
              </button>
            ) : (
              <button
                type="button"
                className="xa-nut xa-nut--hong"
                onClick={submit}
                disabled={sending || form.noi_dung.trim() === ""}
              >
                <Icon name="send" size={20} />
                {SEND.title}
              </button>
            )}
          </div>
        )}

        {step === 3 && sent !== null && (
          <div className="xa-ket-qua">
            <span className="xa-ket-qua__dau" aria-hidden="true">
              <Icon name="check" size={46} />
            </span>
            <h2 className="xa-bai__tieu-de">{COMMUNE_APP_REPORTS.done_title}</h2>
            <p className="xa-phu">{COMMUNE_APP_REPORTS.done_description}</p>
            <div className="xa-the xa-the--dem xa-ket-qua__ma" role="status">
              <p className="xa-phu">{COMMUNE_APP_REPORTS.your_report_code}</p>
              <strong>#{sent.lookup_code}</strong>
            </div>
            {sent.acknowledge_due !== null && vnDateTime(sent.acknowledge_due) !== null && (
              <p className="xa-phu">{COMMUNE_APP_REPORTS.acknowledge_by(vnDateTime(sent.acknowledge_due)!)}</p>
            )}
            <button type="button" className="xa-nut xa-nut--hong" onClick={() => props.onOpenReport(sent.lookup_code)}>
              {COMMUNE_APP_REPORTS.follow_up}
            </button>
            <button type="button" className="xa-nut xa-nut--phu" onClick={props.onBack}>
              {COMMUNE_APP_SCREENS.home_button}
            </button>
          </div>
        )}
      </SubPage>
    </>
  );
}
