"use client";

import { CalendarClock, Check, Loader2 } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import type { KetQua } from "@/lib/api/goi";
import { getTaskExtensionHistory, layHangChoLuiHan } from "@/lib/api/nhiem-vu";
import type { LocHangChoLuiHan } from "@/lib/api/nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import type {
  page_Result_petitions_deNghiChoDuyetRa,
  petitions_deNghiChoDuyetRa,
  petitions_deNghiLuiHanRa,
} from "@/lib/api/schema.gen";
import { nhanThoiDiem } from "@/features/phan-anh/nhan-phieu";
import { cn } from "@/lib/cn";

import {
  CAU_KHONG_AI_DUYET_DUOC,
  CAU_THIEU_QUYEN_DUYET_GIA_HAN,
  DECISION_NOTE_LABEL,
  EXTENSION_NO_DUE,
  GHI_CHU_LUI_HAN,
  TASK_EXTENSIONS_LOADING,
  hienDongHangCho,
  nhanNgay,
} from "./nhan-nhiem-vu";
import { INPUT_CLASS, SECTION_CLASS, SECTION_TITLE_CLASS } from "./task-spec";

/**
 * `Đề nghị lùi hạn` in the drawer — spec 07 §6f / prototype `TaskDetailDrawer.tsx:538-613`: ONE state
 * at a time. A pending request → the tangerine box `Xin lùi hạn tới {d}` + its reason, with
 * `Duyệt lùi hạn` / `Từ chối` for the one person allowed to decide; none → the form to ask.
 *
 * THE PENDING REQUEST comes from `GET /api/v1/task-extensions?task=NV19` (ad7f821) — at most ONE per
 * task (`UNIQUE (tenant_id, nhiem_vu_id, moc_cho_duyet)`). Its gate is the queue's row gate
 * (`hienDongHangCho`): `task.extend` of the session (layer one) AND the business-code comparison with
 * the task's recorded assigner (ADR 0038, layer two). A second gate here would be a second copy of
 * ADR 0038 that drifts; the server re-checks both layers in the transaction (rule 5, forbidden #1).
 *
 * TWO KEYS, NEVER MERGED (ADR 0038): ASKING is `task.update` — the person doing the work;
 * DECIDING is `task.extend` + being the recorded assigner. One kept difference from the prototype:
 * the optional decision note (ADR 0076 #4a).
 *
 * OUTCOMES ARE TOASTS (spec 07 §6f): "Đã gửi đề nghị lùi hạn, chờ lãnh đạo duyệt.", "Đã duyệt lùi
 * hạn.", "Đã từ chối đề nghị.", "Cần chọn hạn mới và ghi lý do."; a refusal is the server's sentence.
 */

/** One task has at most ONE pending request; the page size only bounds a broken answer. */
const EXTENSION_PAGE_SIZE = 20;

/** The block's query — exported so `task=` (not a scan of the commune's queue) is checked. */
export function taskExtensionsQuery(taskCode: string): LocHangChoLuiHan {
  return { task: taskCode, limit: EXTENSION_PAGE_SIZE };
}

/** Spec 07 §6f words. */
export const EXTENSION_TITLE = "Đề nghị lùi hạn";
export const EXTENSION_WAITING = "Đang chờ lãnh đạo duyệt.";
export const EXTENSION_APPROVE = "Duyệt lùi hạn";
export const EXTENSION_REJECT = "Từ chối";
export const EXTENSION_SEND = "Gửi đề nghị lùi hạn";
export const EXTENSION_REASON_PLACEHOLDER = "Chờ kết quả đo đạc của huyện";
export const EXTENSION_SENT = "Đã gửi đề nghị lùi hạn, chờ lãnh đạo duyệt.";
export const EXTENSION_MISSING = "Cần chọn hạn mới và ghi lý do.";
export const EXTENSION_APPROVED = "Đã duyệt lùi hạn.";
export const EXTENSION_REJECTED = "Đã từ chối đề nghị.";

/** `Xin lùi hạn tới {d}` (spec 07 §6f). */
export function extensionAskText(newDue: string): string {
  return `Xin lùi hạn tới ${newDue}`;
}

type Loaded =
  | { key: string; ok: true; rows: readonly petitions_deNghiChoDuyetRa[] }
  | { key: string; ok: false; message: string };

function toLoaded(key: string, result: KetQua<page_Result_petitions_deNghiChoDuyetRa>): Loaded {
  return result.ok
    ? { key, ok: true, rows: result.duLieu.items }
    : { key, ok: false, message: result.thongBao };
}

export function TaskExtensionSection({
  taskCode,
  dueAt,
  directory,
  sessionStaffCode,
  canApproveExtension,
  canRequest,
  refreshKey,
  decide,
  request,
  onDecided,
}: {
  taskCode: string;
  /** `due_at` of the task: no deadline, nothing to push back. */
  dueAt: string | null;
  directory: DanhBaTheoMa | null;
  /** `phien.staff.code` (`CB-…`), `""` while the session is unread — fail closed. */
  sessionStaffCode: string;
  /** `task.extend` of the session. */
  canApproveExtension: boolean;
  /** `task.update` of the session — the key of SENDING a request. */
  canRequest: boolean;
  /** Changes when a request may have appeared or been decided elsewhere. */
  refreshKey: string;
  decide: (requestId: string, approve: boolean, note?: string) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  request: (newDueISO: string, reason: string) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  /** A decision succeeded — the task's deadline may have moved. */
  onDecided: (taskCode: string) => void;
}) {
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [deciding, setDeciding] = useState(false);
  const [note, setNote] = useState("");
  const [newDue, setNewDue] = useState("");
  const [reason, setReason] = useState("");
  const [sending, setSending] = useState(false);
  const key = `${taskCode}|${refreshKey}`;

  useEffect(() => {
    let dropped = false;
    layHangChoLuiHan(taskExtensionsQuery(taskCode)).then((result) => {
      if (!dropped) setLoaded(toLoaded(key, result));
    });
    return () => {
      dropped = true;
    };
  }, [taskCode, key]);

  const current = loaded !== null && loaded.key === key ? loaded : null;

  function decideRow(row: petitions_deNghiChoDuyetRa, approve: boolean): void {
    setDeciding(true);
    // Same call as the queue: optional note, trimmed; empty means "no note".
    decide(row.id, approve, note.trim()).then((result) => {
      setDeciding(false);
      if (!result.ok) {
        toast.error(result.thongBao);
        return;
      }
      toast.success(approve ? EXTENSION_APPROVED : EXTENSION_REJECTED);
      setNote("");
      setLoaded((l) => (l === null || !l.ok ? l : { ...l, rows: l.rows.filter((r) => r.id !== row.id) }));
      onDecided(taskCode);
    });
  }

  function send(): void {
    if (sending) return;
    if (newDue === "" || reason.trim() === "") {
      toast.error(EXTENSION_MISSING);
      return;
    }
    const at = new Date(newDue);
    if (Number.isNaN(at.getTime())) {
      toast.error(EXTENSION_MISSING);
      return;
    }
    setSending(true);
    request(at.toISOString(), reason.trim()).then((r) => {
      setSending(false);
      if (!r.ok) {
        // 409 "already a pending request", 400 "not later than the current deadline" — verbatim.
        toast.error(r.thongBao);
        return;
      }
      toast.success(EXTENSION_SENT);
      setNewDue("");
      setReason("");
      // The caller bumps `refreshKey`: the request comes back from the server, never spliced in.
    });
  }

  return (
    <TaskExtensionView
      load={
        current === null
          ? { phase: "loading" }
          : current.ok
            ? { phase: "done", rows: current.rows }
            : { phase: "error", message: current.message }
      }
      dueAt={dueAt}
      directory={directory}
      sessionStaffCode={sessionStaffCode}
      canApproveExtension={canApproveExtension}
      canRequest={canRequest}
      deciding={deciding}
      note={note}
      setNote={setNote}
      decide={decideRow}
      newDue={newDue}
      setNewDue={setNewDue}
      reason={reason}
      setReason={setReason}
      sending={sending}
      send={send}
    />
  );
}

export type TaskExtensionsLoad =
  | { phase: "loading" }
  | { phase: "error"; message: string }
  | { phase: "done"; rows: readonly petitions_deNghiChoDuyetRa[] };

/**
 * Rendering only — exported for `renderToStaticMarkup`.
 *
 * Nothing to show (no request waiting, and this account may not ask) → no section at all. A FAILED
 * read is never drawn as "no pending request": the server's sentence shows instead.
 */
export function TaskExtensionView({
  load,
  dueAt,
  directory,
  sessionStaffCode,
  canApproveExtension,
  canRequest,
  deciding,
  note,
  setNote,
  decide,
  newDue,
  setNewDue,
  reason,
  setReason,
  sending,
  send,
}: {
  load: TaskExtensionsLoad;
  dueAt: string | null;
  directory: DanhBaTheoMa | null;
  sessionStaffCode: string;
  canApproveExtension: boolean;
  canRequest: boolean;
  deciding: boolean;
  note: string;
  setNote: (v: string) => void;
  decide: (row: petitions_deNghiChoDuyetRa, approve: boolean) => void;
  newDue: string;
  setNewDue: (v: string) => void;
  reason: string;
  setReason: (v: string) => void;
  sending: boolean;
  send: () => void;
}) {
  const row = load.phase === "done" ? load.rows[0] : undefined;
  if (load.phase === "done" && row === undefined && !canRequest) return null;
  if (load.phase === "loading" && !canRequest) return null;

  return (
    <section className={SECTION_CLASS} aria-labelledby="task-detail-extensions">
      <h3 id="task-detail-extensions" className={SECTION_TITLE_CLASS}>
        {EXTENSION_TITLE}
      </h3>

      {load.phase === "loading" && (
        <p role="status" className="text-ink-muted m-0 text-[12px]">
          {TASK_EXTENSIONS_LOADING}
        </p>
      )}
      {load.phase === "error" && (
        <p className="thong-bao-loi m-0" role="alert">
          {load.message}
        </p>
      )}

      {row !== undefined && (
        <PendingRequest
          row={row}
          directory={directory}
          sessionStaffCode={sessionStaffCode}
          canApproveExtension={canApproveExtension}
          deciding={deciding}
          note={note}
          setNote={setNote}
          decide={decide}
        />
      )}

      {load.phase === "done" && row === undefined && canRequest && dueAt === null && (
        <p className="text-ink-muted m-0 text-[12px]">{EXTENSION_NO_DUE}</p>
      )}
      {load.phase === "done" && row === undefined && canRequest && dueAt !== null && (
        <form
          className="border-line bg-canvas m-0 space-y-3 rounded-[10px] border p-3"
          onSubmit={(e) => {
            e.preventDefault();
            send();
          }}
        >
          <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
            <div>
              <label htmlFor="han-moi-lui-han" className="text-ink block text-[11.5px] font-medium">
                Hạn mới
              </label>
              <input
                id="han-moi-lui-han"
                name="han-moi-lui-han"
                type="datetime-local"
                value={newDue}
                className={cn(INPUT_CLASS, "mt-1 px-2 text-[12.5px] md:text-[12.5px]")}
                onChange={(e) => setNewDue(e.target.value)}
              />
            </div>
            <div>
              <label htmlFor="ly-do-lui-han" className="text-ink block text-[11.5px] font-medium">
                Lý do
              </label>
              <input
                id="ly-do-lui-han"
                name="ly-do-lui-han"
                value={reason}
                autoComplete="off"
                placeholder={EXTENSION_REASON_PLACEHOLDER}
                className={cn(INPUT_CLASS, "mt-1 text-[12.5px] md:text-[12.5px]")}
                onChange={(e) => setReason(e.target.value)}
              />
            </div>
          </div>
          <Button
            type="submit"
            variant="outline"
            size="sm"
            disabled={sending}
            aria-busy={sending || undefined}
            icon={
              sending ? (
                <Loader2 aria-hidden="true" focusable="false" className="size-3.5 animate-spin" />
              ) : (
                <CalendarClock aria-hidden="true" focusable="false" className="size-3.5" />
              )
            }
          >
            {EXTENSION_SEND}
          </Button>
          <p className="text-ink-muted m-0 text-[11px]">{GHI_CHU_LUI_HAN}</p>
        </form>
      )}
    </section>
  );
}

/** The tangerine box of a waiting request (spec 07 §6f). */
function PendingRequest({
  row,
  directory,
  sessionStaffCode,
  canApproveExtension,
  deciding,
  note,
  setNote,
  decide,
}: {
  row: petitions_deNghiChoDuyetRa;
  directory: DanhBaTheoMa | null;
  sessionStaffCode: string;
  canApproveExtension: boolean;
  deciding: boolean;
  note: string;
  setNote: (v: string) => void;
  decide: (row: petitions_deNghiChoDuyetRa, approve: boolean) => void;
}) {
  // THE QUEUE'S OWN ROW GATE — `cauChan === null` is the one condition for the two buttons.
  const view = hienDongHangCho(row, directory, sessionStaffCode, canApproveExtension);
  const noteId = `ghi-chu-quyet-dinh-drawer-${view.id}`;
  return (
    <div className="border-tangerine/30 bg-tangerine/6 rounded-[10px] border p-3">
      <p className="text-navy m-0 text-[12.5px] font-semibold">{extensionAskText(view.hanDeNghi)}</p>
      <p className="text-ink-muted m-0 mt-1 text-[12px]">{view.lyDo}</p>
      {view.cauChan === null ? (
        <>
          <label htmlFor={noteId} className="text-ink mt-2.5 block text-[11.5px] font-medium">
            {DECISION_NOTE_LABEL}
          </label>
          <input
            id={noteId}
            name={noteId}
            value={note}
            autoComplete="off"
            className={cn(INPUT_CLASS, "mt-1 text-[12.5px] md:text-[12.5px]")}
            onChange={(e) => setNote(e.target.value)}
          />
          <div className="mt-2.5 flex gap-2">
            <Button
              type="button"
              variant="primary"
              size="sm"
              disabled={deciding}
              aria-label={`${EXTENSION_APPROVE} ${view.maNhiemVu}`}
              icon={<Check aria-hidden="true" focusable="false" className="size-3.5" />}
              onClick={() => decide(row, true)}
            >
              {EXTENSION_APPROVE}
            </Button>
            <Button
              type="button"
              variant="outline"
              size="sm"
              className="text-danger hover:not-disabled:text-danger"
              disabled={deciding}
              aria-label={`${EXTENSION_REJECT} lùi hạn ${view.maNhiemVu}`}
              onClick={() => decide(row, false)}
            >
              {EXTENSION_REJECT}
            </Button>
          </div>
        </>
      ) : (
        // Nobody can approve when the task records no assigner (ADR 0038) — said as it is, never as
        // "waiting", which would promise a decision that cannot come. The recorded assigner who lacks
        // `task.extend` is told why too: "waiting" would tell them to wait for themselves.
        <p className="text-tangerine m-0 mt-2 text-[11.5px]">
          {view.cauChan === CAU_KHONG_AI_DUYET_DUOC || view.cauChan === CAU_THIEU_QUYEN_DUYET_GIA_HAN
            ? view.cauChan
            : EXTENSION_WAITING}
        </p>
      )}
    </div>
  );
}

/* ── Lịch sử gia hạn (spec 07 §6g) ─────────────────────────────────────────────────────────── */

export const EXTENSION_HISTORY_TITLE = "Lịch sử gia hạn";
/** One task has few requests; the page bounds a broken answer, and `Xem thêm` is not drawn. */
const EXTENSION_HISTORY_LIMIT = 50;

/**
 * The status word of a request (prototype `TaskDetailDrawer.tsx:632-636`) for the server's three
 * Vietnamese codes (`domain.TrangThaiDeNghi`, ADR 0011). Unknown → the code itself.
 */
export function extensionStatusText(status: string): string {
  switch (status) {
    case "da-duyet":
      return "đã duyệt";
    case "tu-choi":
      return "từ chối";
    case "cho-duyet":
      return "chờ duyệt";
    default:
      return status;
  }
}

function extensionStatusClass(status: string): string {
  return status === "da-duyet" ? "text-leaf" : status === "tu-choi" ? "text-danger" : "text-tangerine";
}

type HistoryLoad =
  | { key: string; ok: true; rows: readonly petitions_deNghiLuiHanRa[] }
  | { key: string; ok: false; message: string };

/**
 * Every extension request of the task, newest first (`GET /api/v1/tasks/{ma}/extensions`, `task.read`).
 * Drawn ONLY when there is at least one, as the prototype; a failed read says the server's sentence
 * (never silence, which would read as "never extended").
 *
 * The prototype's line is `{hạn cũ} → {hạn mới}`; the row does not carry the deadline before the
 * request, so the line says `Lùi tới {hạn mới}` — never an arrow with nothing before it.
 */
export function TaskExtensionHistory({ taskCode, refreshKey }: { taskCode: string; refreshKey: string }) {
  const [loaded, setLoaded] = useState<HistoryLoad | null>(null);
  const key = `${taskCode}|${refreshKey}`;

  useEffect(() => {
    let dropped = false;
    getTaskExtensionHistory(taskCode, { limit: EXTENSION_HISTORY_LIMIT }).then((r) => {
      if (dropped) return;
      setLoaded(r.ok ? { key, ok: true, rows: r.duLieu.items } : { key, ok: false, message: r.thongBao });
    });
    return () => {
      dropped = true;
    };
  }, [taskCode, key]);

  const current = loaded !== null && loaded.key === key ? loaded : null;
  return <TaskExtensionHistoryView load={current === null ? null : current.ok ? current.rows : current.message} />;
}

/** Rendering only. `null` = loading (nothing drawn), a string = the server's refusal, else the rows. */
export function TaskExtensionHistoryView({ load }: { load: readonly petitions_deNghiLuiHanRa[] | string | null }) {
  if (load === null || (typeof load !== "string" && load.length === 0)) return null;
  return (
    <section className={SECTION_CLASS} aria-labelledby="task-detail-extension-history">
      <h3 id="task-detail-extension-history" className={SECTION_TITLE_CLASS}>
        {EXTENSION_HISTORY_TITLE}
      </h3>
      {typeof load === "string" ? (
        <p className="thong-bao-loi m-0" role="alert">
          {load}
        </p>
      ) : (
        <ul className="m-0 list-none space-y-2 p-0">
          {load.map((item) => (
            <li key={item.id} className="border-line rounded-[9px] border px-3 py-2.5 text-[12px]">
              <div className="text-navy font-semibold">
                Lùi tới {nhanNgay(item.new_due_at)}
                <span className={cn("ml-2 text-[11px]", extensionStatusClass(item.status))}>
                  {extensionStatusText(item.status)}
                </span>
              </div>
              <p className="text-ink-muted m-0 mt-1">{item.reason}</p>
              {item.decision_note !== null && item.decision_note !== "" && (
                <p className="text-ink-muted m-0 mt-1 text-[11.5px]">Ghi chú quyết định: {item.decision_note}</p>
              )}
              <p className="text-ink-muted m-0 mt-1 text-[10.5px]">
                Gửi lúc {nhanThoiDiem(item.requested_at)}
                {item.decided_at !== null && ` · quyết định lúc ${nhanThoiDiem(item.decided_at)}`}
              </p>
            </li>
          ))}
        </ul>
      )}
    </section>
  );
}
