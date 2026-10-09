"use client";

import { useEffect, useState, type Dispatch, type ReactNode, type SetStateAction } from "react";
import { toast } from "sonner";

import { LargeDialog } from "@/components/ui/large-dialog";
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type
import {
  deNghiLuiHan, // vi-name-ok: existing task client
  doiTrangThaiNhiemVu, // vi-name-ok: existing task client
  layNhiemVu, // vi-name-ok: existing task client
  quyetDinhLuiHan, // vi-name-ok: existing task client
  reassignTask,
  suaNhiemVu, // vi-name-ok: existing task client
  taoNhiemVu, // vi-name-ok: existing task client
  xoaNhiemVu, // vi-name-ok: existing task client
} from "@/lib/api/nhiem-vu";
import type { identity_danhBaChonNguoiRa } from "@/lib/api/schema.gen"; // vi-name-ok: generated contract type

import { childCreatedText, type BangNhanTrangThai, type QuyenNhiemVu } from "./nhan-nhiem-vu"; // vi-name-ok: existing types of the register
import {
  ChiTietNhiemVu, // vi-name-ok: existing detail component
  DETAIL_DRAWER_CLASS,
  FormGiaoViec, // vi-name-ok: existing create form
  TASK_DETAIL_TITLE_ID,
  TaskTabPending,
  taskDeletedText,
  type DanhMucNhiemVu, // vi-name-ok: existing catalogue type
  type DrawerNhiemVu, // vi-name-ok: existing drawer state type
  type ViecDrawer, // vi-name-ok: existing drawer action type
} from "./so-nhiem-vu";

/**
 * THE ONE WIRING OF THE TASK DETAIL DRAWER (`ChiTietNhiemVu`) — every screen that opens a task's detail
 * mounts this, never its own copy of the twenty-five props (owner 09/10/2026). Two wirings of one drawer
 * drift: the day a write learns to refresh one more thing, the copy that was not edited shows a stale
 * task, and nothing turns red.
 *
 * WHAT THE HOST OWNS: the drawer's shell (`LargeDialog`, spec 07 §Vỏ), the "being read / refused"
 * content (`TaskTabPending`), and every write the detail makes — each through the register's own
 * client, each followed by the same `chuyenDrawer` move as before.
 *
 * WHAT THE SCREEN OWNS, passed in: the drawer state and its dispatcher (the register keeps its record
 * tabs on top of it, ADR 0076; the Sổ tay keeps one drawer), the catalogues it has read, and what
 * "the list behind me changed" means for it (`onRegisterChanged`) — the register re-reads its page,
 * the Sổ tay its four sections and their counts.
 *
 * Every gate stays where it was: the detail hides a write button the session lacks the key for, and the
 * route checks the key again on every request (rule 5, forbidden #1).
 */

/** What the detail needs from the screen's reads. */
export type TaskDetailContext = {
  readonly catalogues: DanhMucNhiemVu;
  /** The status words the detail draws — the register's (`withSpecLabels`). */
  readonly statusLabels: BangNhanTrangThai;
  readonly unitNames: ReadonlyMap<string, string>;
  /** The staff picker directory; `null` = not read yet. */
  readonly directory: KetQua<identity_danhBaChonNguoiRa> | null;
  /** Holders of `task.extend`, for the child form's leader field; `null` = not read yet. */
  readonly leaderDirectory: KetQua<identity_danhBaChonNguoiRa> | null;
  /** `phien.staff.code` — empty while the session is unread (every comparison then fails closed). */
  readonly staffCode: string;
  readonly permissions: QuyenNhiemVu;
  /** `task.read` — the `Xem` of the assignee's address (ADR 0082 #3). */
  readonly canRevealEmail: boolean;
};

/**
 * The detail's own UI state that outlives one task: the write in flight, the extension-queue tick and
 * the child form. A hook rather than state inside the host, because the register shares two of them
 * with its own create form (one `sending`; one create form on the page at a time).
 */
export type TaskDetailState = {
  readonly sending: boolean;
  readonly setSending: (sending: boolean) => void;
  /** Bumped when an extension request was just sent or decided — the drawer's block re-reads. */
  readonly extensionQueueTick: number;
  readonly bumpExtensionQueue: () => void;
  /** The parent whose `+ Thêm việc con` form is open, or `null`. */
  readonly childFormFor: string | null;
  readonly setChildFormFor: Dispatch<SetStateAction<string | null>>;
  readonly childFormSending: boolean;
  readonly setChildFormSending: (sending: boolean) => void;
};

export function useTaskDetailState(): TaskDetailState {
  const [sending, setSending] = useState(false);
  const [extensionQueueTick, setExtensionQueueTick] = useState(0);
  const [childFormFor, setChildFormFor] = useState<string | null>(null);
  const [childFormSending, setChildFormSending] = useState(false);
  return {
    sending,
    setSending,
    extensionQueueTick,
    bumpExtensionQueue: () => setExtensionQueueTick((n) => n + 1),
    childFormFor,
    setChildFormFor,
    childFormSending,
    setChildFormSending,
  };
}

/**
 * THE DETAIL READ — see `chuyenDrawer`. Runs again on every new read turn (open, or after a write);
 * the code and the turn travel with the answer so the reducer drops an answer to an older turn.
 * `dispatch` must be stable (a `useReducer` dispatcher).
 */
export function useTaskDetailRead(drawer: DrawerNhiemVu | null, dispatch: (action: ViecDrawer) => void): void {
  const code = drawer?.nhiemVu.code ?? null;
  const turn = drawer?.luotDoc ?? 0;
  useEffect(() => {
    if (code === null) return;
    let cancelled = false;
    layNhiemVu(code).then((kq) => {
      if (!cancelled) dispatch({ loai: "chiTietVe", ma: code, luotDoc: turn, kq });
    });
    return () => {
      cancelled = true;
    };
  }, [code, turn, dispatch]);
}

/** The register's record-tab strip, drawn above the content; absent on screens with one drawer. */
export type TaskDetailTabs = {
  readonly strip: ReactNode;
  /** The `tabpanel` id the strip's tabs control. */
  readonly panelId: string;
  /** The id of the active tab, which labels the panel. */
  readonly labelledBy: string;
};

export function TaskDetailHost({
  shownCode,
  drawer,
  pendingTitle,
  pendingError,
  dispatch,
  context,
  state,
  onRegisterChanged,
  onDeleted,
  onChildFormToggle,
  tabs,
}: {
  /** The code on screen. The drawer of another code (or none) draws the pending content instead. */
  shownCode: string;
  drawer: DrawerNhiemVu | null;
  /** The pending content's heading while `shownCode` is read. */
  pendingTitle: string;
  /** The server's refusal of that read, verbatim; `null` while it is being read. */
  pendingError: string | null;
  dispatch: (action: ViecDrawer) => void;
  context: TaskDetailContext;
  state: TaskDetailState;
  /** A write succeeded: the list behind the drawer is stale. */
  onRegisterChanged: () => void;
  /** The task was soft-deleted: the screen drops what showed it. */
  onDeleted: (code: string) => void;
  /** Called before the child form toggles — the register closes its own create form (same field ids). */
  onChildFormToggle?: () => void;
  tabs?: TaskDetailTabs;
}) {
  const hide = () => dispatch({ loai: "dong" });
  return (
    <LargeDialog titleId={TASK_DETAIL_TITLE_ID} className={DETAIL_DRAWER_CLASS} onDismiss={hide}>
      {tabs?.strip}
      <div
        id={tabs?.panelId}
        role={tabs === undefined ? undefined : "tabpanel"}
        aria-labelledby={tabs?.labelledBy}
        className="flex min-h-0 min-w-0 flex-1 flex-col"
      >
        {drawer === null || drawer.nhiemVu.code !== shownCode ? (
          <TaskTabPending code={shownCode} title={pendingTitle} error={pendingError} onHide={hide} />
        ) : (
          <TaskDetailBody
            drawer={drawer}
            dispatch={dispatch}
            context={context}
            state={state}
            onRegisterChanged={onRegisterChanged}
            onDeleted={onDeleted}
            onChildFormToggle={onChildFormToggle}
          />
        )}
      </div>
    </LargeDialog>
  );
}

function TaskDetailBody({
  drawer,
  dispatch,
  context,
  state,
  onRegisterChanged,
  onDeleted,
  onChildFormToggle,
}: {
  drawer: DrawerNhiemVu;
  dispatch: (action: ViecDrawer) => void;
  context: TaskDetailContext;
  state: TaskDetailState;
  onRegisterChanged: () => void;
  onDeleted: (code: string) => void;
  onChildFormToggle?: () => void;
}) {
  const task = drawer.nhiemVu;
  return (
    <ChiTietNhiemVu
      // A new task is a new detail: an open edit, a half-typed note or reason do not follow.
      key={task.code}
      nhiemVu={task}
      vanBan={drawer.vanBan}
      danhMuc={context.catalogues}
      nhanTT={context.statusLabels}
      tenBoPhan={context.unitNames}
      danhBa={context.directory}
      // The read turn goes up on opening AND after every write — exactly when the timeline re-reads.
      lanLamMoiNhatKy={drawer.luotDoc}
      bayGio={new Date()}
      maNguoiDangNhap={context.staffCode}
      quyen={context.permissions}
      canRevealEmail={context.canRevealEmail}
      dangGui={state.sending}
      dong={() => dispatch({ loai: "dong" })}
      // The answer goes back to the pipeline: it keeps the typed note and toasts the outcome.
      doiTrangThai={(status, note, extra) => {
        const code = task.code;
        state.setSending(true);
        const call = doiTrangThaiNhiemVu(code, status, note, extra);
        call.then((kq) => {
          state.setSending(false);
          if (!kq.ok) return;
          dispatch({ loai: "ghiXong", nhiemVu: kq.duLieu });
          onRegisterChanged();
        });
        return call;
      }}
      xoa={(reason) => {
        const code = task.code;
        state.setSending(true);
        return xoaNhiemVu(code, reason).then((kq) => {
          state.setSending(false);
          // A refusal ("còn 3 việc con chưa xoá…", ADR 0037 decision 3) goes back to the confirm
          // dialog, which shows it verbatim and stays open.
          if (!kq.ok) return kq;
          toast.success(taskDeletedText(code));
          onDeleted(code);
          onRegisterChanged();
          return kq;
        });
      }}
      guiDeNghiLuiHan={(newDue, reason) =>
        deNghiLuiHan(task.code, newDue, reason).then((kq) => {
          if (kq.ok) {
            // `pending_extension` of the task row changed: the drawer's strip and the list read it
            // from the task, so both are read again — never set by hand here.
            dispatch({ loai: "docLai", ma: task.code });
            onRegisterChanged();
            state.bumpExtensionQueue();
          }
          return kq;
        })
      }
      quyetDinh={(requestId, approve, note) => quyetDinhLuiHan(task.code, requestId, approve, note)}
      // The drawer's extension block re-reads on every drawer re-read AND after a request is sent.
      extensionRefreshKey={`${drawer.luotDoc}|${state.extensionQueueTick}`}
      onExtensionDecided={(code) => {
        // Approving moves `due_at`: the drawer and the list are stale.
        dispatch({ loai: "docLai", ma: code });
        onRegisterChanged();
        state.bumpExtensionQueue();
      }}
      openTask={(n) => dispatch({ loai: "mo", nhiemVu: n })}
      addChild={
        // `+ Thêm việc con` stands behind the SAME key as `+ Giao việc mới` — `task.create`
        // (`quyen.giaoViec`), the key of `POST /api/v1/tasks`. The server checks it anyway.
        context.permissions.giaoViec
          ? {
              open: state.childFormFor === task.code,
              toggle: () => {
                // ONE create form on the page at a time: both render the same field ids
                // (`giao-loai`…), and two labels pointing at one id is a broken form for a
                // screen reader.
                onChildFormToggle?.();
                state.setChildFormFor((f) => (f === task.code ? null : task.code));
              },
              form: (
                <FormGiaoViec
                  // A new form (and a new idempotency key) per parent.
                  key={task.code}
                  dialog
                  taskScreen
                  danhMuc={context.catalogues}
                  danhBa={context.directory}
                  danhBaLanhDao={context.leaderDirectory}
                  coDanhSachVanBan
                  staffSearch
                  maChaCoSan={task.code}
                  dangGui={state.childFormSending}
                  loi={null}
                  huy={() => state.setChildFormFor(null)}
                  giaoViec={(body, idempotencyKey) => {
                    const parentCode = task.code;
                    state.setChildFormSending(true);
                    taoNhiemVu(body, idempotencyKey).then((kq) => {
                      state.setChildFormSending(false);
                      if (!kq.ok) {
                        // VERBATIM — an invalid parent is the server's 409 `task_tree` sentence.
                        toast.error(kq.thongBao);
                        return;
                      }
                      state.setChildFormFor(null);
                      toast.success(childCreatedText(kq.duLieu.code));
                      // STAY ON THE PARENT: re-read it (its `child_count`, its children block)
                      // instead of jumping to the new child, which would lose the tree.
                      dispatch({ loai: "docLai", ma: parentCode });
                      onRegisterChanged();
                    });
                  }}
                />
              ),
            }
          : null
      }
      // §5.4 `Sửa` and the two approval ticks. On success the PATCH answer CARRIES `documents`
      // (`app/nhiem_vu.go:696-741`), so `ghiXong` replaces both the scalar fields and the document
      // block, then the list re-reads like after every write. On failure the `KetQua` goes back to
      // the form untouched, so the form KEEPS what the officer typed; the drawer re-reads (a 409
      // `task_changed` means the task moved on).
      suaKhoiVanBan={(body) => {
        const code = task.code;
        return suaNhiemVu(code, body).then((kq) => {
          if (kq.ok) {
            dispatch({ loai: "ghiXong", nhiemVu: kq.duLieu });
            onRegisterChanged();
          } else {
            dispatch({ loai: "docLai", ma: code });
          }
          return kq;
        });
      }}
      docLaiChiTiet={() => layNhiemVu(task.code)}
      // §5.7 — same refresh as `Sửa`: `ghiXong` takes the returned task (keeping the document
      // block, which this reply does not carry) and bumps `luotDoc`, so the detail AND the
      // timeline re-read; the list re-reads too — the card may have changed column.
      reassign={(body) =>
        reassignTask(task.code, body).then((kq) => {
          if (kq.ok) {
            dispatch({ loai: "ghiXong", nhiemVu: kq.duLieu });
            onRegisterChanged();
          }
          return kq;
        })
      }
    />
  );
}
