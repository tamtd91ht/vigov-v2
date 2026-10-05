"use client";

import { useEffect, useState } from "react";

import type { DanhBaTheoMa } from "@/features/phan-anh/nhan-phieu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import type { KetQua } from "@/lib/api/goi";
import { layHangChoLuiHan } from "@/lib/api/nhiem-vu";
import type { LocHangChoLuiHan } from "@/lib/api/nhiem-vu"; // vi-name-ok: existing type, imported not declared (rule 12 inv 3)
import type {
  page_Result_petitions_deNghiChoDuyetRa,
  petitions_deNghiChoDuyetRa,
  petitions_deNghiLuiHanRa,
} from "@/lib/api/schema.gen";

import {
  DECISION_NOTE_LABEL,
  TASK_EXTENSIONS_EMPTY,
  TASK_EXTENSIONS_LOADING,
  TASK_EXTENSIONS_TITLE,
  extensionBlockNote,
  hienDongHangCho,
  quyetDinhDuyetLuiHan,
} from "./nhan-nhiem-vu";

/**
 * §5.8 in the drawer — THIS task's pending extension requests (#11), via
 * `GET /api/v1/task-extensions?task=NV19` (ad7f821).
 *
 * THE ONE PLACE A DECISION IS TAKEN ON THE NHIỆM VỤ SCREEN — the on-page queue was removed with the
 * prototype layout (ADR 0068 §Sửa đổi 06/10/2026 lần 5); Sổ tay lãnh đạo lists what waits and links
 * here. Each row goes through `hienDongHangCho` — `task.extend` of the session
 * (layer one) and the business-code comparison with `task_assigner` (ADR 0038, layer two) — and a
 * decision calls `quyetDinhLuiHan` with the optional note trimmed. A second gate here would be a
 * second copy of ADR 0038 that drifts; the server re-checks both layers inside the transaction
 * whatever this block shows (rule 5, forbidden #1).
 *
 * NO `approver=me`: the drawer must also show a request the viewer cannot decide, read-only — a
 * pending request that vanishes for everyone but the approver reads as "nobody asked".
 */

/** One task has at most ONE pending request (`UNIQUE (tenant_id, nhiem_vu_id, moc_cho_duyet)`). */
const EXTENSION_PAGE_SIZE = 20;

/** The block's query — exported so `task=` (not a scan of the commune's queue) is checked. */
export function taskExtensionsQuery(taskCode: string): LocHangChoLuiHan {
  return { task: taskCode, limit: EXTENSION_PAGE_SIZE };
}

type Loaded =
  | { key: string; ok: true; rows: readonly petitions_deNghiChoDuyetRa[] }
  | { key: string; ok: false; message: string };

function toLoaded(key: string, result: KetQua<page_Result_petitions_deNghiChoDuyetRa>): Loaded {
  return result.ok
    ? { key, ok: true, rows: result.duLieu.items }
    : { key, ok: false, message: result.thongBao };
}

export function TaskExtensionBlock({
  taskCode,
  assigner,
  directory,
  sessionStaffCode,
  canApproveExtension,
  refreshKey,
  decide,
  onDecided,
  onPendingChange,
}: {
  /**
   * Told whether this read found a pending request — the detail's `Chờ duyệt lùi hạn` strip uses it
   * instead of reading the same route a second time. A failed read says `false`: the block itself
   * then shows the server's sentence.
   */
  onPendingChange?: (pending: boolean) => void;
  taskCode: string;
  /** `assigner` of the task — `lanh_dao_giao_viec_ma`, the one approver ADR 0038 names. */
  assigner: string;
  directory: DanhBaTheoMa | null;
  /** `phien.staff.code` (`CB-…`), `""` while the session is unread — fail closed. */
  sessionStaffCode: string;
  /** `task.extend` of the session. */
  canApproveExtension: boolean;
  /** Changes when a request may have appeared or been decided elsewhere. */
  refreshKey: string;
  decide: (
    requestId: string,
    approve: boolean,
    note?: string,
  ) => Promise<KetQua<petitions_deNghiLuiHanRa>>;
  /** A decision succeeded — the task's deadline may have moved. */
  onDecided: (taskCode: string) => void;
}) {
  const [loaded, setLoaded] = useState<Loaded | null>(null);
  const [deciding, setDeciding] = useState<string | null>(null);
  const [rowError, setRowError] = useState<{ id: string; message: string } | null>(null);
  const [notes, setNotes] = useState<Readonly<Record<string, string>>>({});
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
  const pending = current !== null && current.ok && current.rows.length > 0;
  useEffect(() => {
    onPendingChange?.(pending);
  }, [pending, onPendingChange]);

  function decideRow(row: petitions_deNghiChoDuyetRa, approve: boolean): void {
    setDeciding(row.id);
    setRowError(null);
    // Same call as the queue: optional note, trimmed; empty means "no note".
    decide(row.id, approve, (notes[row.id] ?? "").trim()).then((result) => {
      setDeciding(null);
      if (!result.ok) {
        setRowError({ id: row.id, message: result.thongBao });
        return;
      }
      setLoaded((l) =>
        l === null || !l.ok ? l : { ...l, rows: l.rows.filter((r) => r.id !== row.id) },
      );
      onDecided(taskCode);
    });
  }

  return (
    <TaskExtensionList
      load={
        current === null
          ? { phase: "loading" }
          : current.ok
            ? { phase: "done", rows: current.rows }
            : { phase: "error", message: current.message }
      }
      assigner={assigner}
      directory={directory}
      sessionStaffCode={sessionStaffCode}
      canApproveExtension={canApproveExtension}
      deciding={deciding}
      rowError={rowError}
      notes={notes}
      setNote={(id, value) => setNotes((n) => ({ ...n, [id]: value }))}
      decide={decideRow}
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
 * The task-level sentence (`extensionBlockNote`) shows even while loading: "only the recorded
 * assigner may approve" is a fact about the task, not about whether a request exists. A FAILED read
 * is never drawn as "no pending request" — that sentence would tell an approver they owe nothing.
 */
export function TaskExtensionList({
  load,
  assigner,
  directory,
  sessionStaffCode,
  canApproveExtension,
  deciding,
  rowError,
  notes,
  setNote,
  decide,
}: {
  load: TaskExtensionsLoad;
  assigner: string;
  directory: DanhBaTheoMa | null;
  sessionStaffCode: string;
  canApproveExtension: boolean;
  deciding: string | null;
  rowError: { id: string; message: string } | null;
  notes: Readonly<Record<string, string>>;
  setNote: (id: string, value: string) => void;
  decide: (row: petitions_deNghiChoDuyetRa, approve: boolean) => void;
}) {
  const gate = quyetDinhDuyetLuiHan(sessionStaffCode, assigner, canApproveExtension);
  const note = extensionBlockNote(gate, load.phase === "done" && load.rows.length > 0);

  return (
    <div className="form-danh-muc" aria-labelledby="tieu-de-de-nghi-cua-viec">
      <h4 id="tieu-de-de-nghi-cua-viec">{TASK_EXTENSIONS_TITLE}</h4>
      {note !== null && <p className="trang-thai-rong">{note}</p>}

      {load.phase === "loading" && <p role="status">{TASK_EXTENSIONS_LOADING}</p>}
      {load.phase === "error" && (
        <p className="thong-bao-loi" role="alert">
          {load.message}
        </p>
      )}
      {load.phase === "done" && load.rows.length === 0 && (
        <p className="trang-thai-rong">{TASK_EXTENSIONS_EMPTY}</p>
      )}
      {load.phase === "done" && load.rows.length > 0 && (
        <ol aria-label={TASK_EXTENSIONS_TITLE}>
          {load.rows.map((row) => {
            // THE QUEUE'S OWN ROW GATE — `cauChan === null` is the one condition for buttons.
            const view = hienDongHangCho(row, directory, sessionStaffCode, canApproveExtension);
            const noteId = `ghi-chu-quyet-dinh-drawer-${view.id}`;
            return (
              <li key={view.id}>
                <dl className="danh-sach-truong">
                  <dt>Hạn đang có</dt>
                  <dd>{view.hanHienTai}</dd>
                  <dt>Hạn đề nghị</dt>
                  <dd>{view.hanDeNghi}</dd>
                  <dt>Lý do</dt>
                  <dd>{view.lyDo}</dd>
                  <dt>Người đề nghị</dt>
                  <dd>
                    {view.nguoiDeNghi} ·{" "}
                    <time dateTime={view.lucDeNghiISO}>{view.lucDeNghi}</time>
                  </dd>
                </dl>

                {view.cauChan === null && (
                  <>
                    <div className="o-nhap">
                      <label htmlFor={noteId}>{DECISION_NOTE_LABEL}</label>
                      <input
                        id={noteId}
                        name={noteId}
                        value={notes[view.id] ?? ""}
                        autoComplete="off"
                        onChange={(e) => setNote(view.id, e.target.value)}
                      />
                    </div>
                    <div className="cum-nut">
                      <button
                        type="button"
                        className="nut-chinh"
                        disabled={deciding !== null}
                        onClick={() => decide(row, true)}
                        aria-label={`Duyệt lùi hạn ${view.maNhiemVu}`}
                      >
                        {deciding === view.id ? "Đang gửi…" : "Duyệt"}
                      </button>
                      <button
                        type="button"
                        className="nut-phu"
                        disabled={deciding !== null}
                        onClick={() => decide(row, false)}
                        aria-label={`Từ chối lùi hạn ${view.maNhiemVu}`}
                      >
                        Từ chối
                      </button>
                    </div>
                  </>
                )}

                {rowError !== null && rowError.id === view.id && (
                  <p className="thong-bao-loi" role="alert">
                    {rowError.message}
                  </p>
                )}
              </li>
            );
          })}
        </ol>
      )}
    </div>
  );
}
