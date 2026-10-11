"use client";

import { ChevronLeft, ChevronRight, CircleStop, CloudAlert, LoaderCircle, Pencil, Plus, Trash2, X } from "lucide-react";
import { useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { Badge, type BadgeTone } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { IconButton } from "@/components/ui/icon-button";
import { SkeletonRows } from "@/components/ui/skeleton";
// vi-name-ok: existing exports of the shared cursor stack, reused rather than copied
import { coTrangTruoc, sangTrangSau, TRANG_DAU, veTrangTruoc, type NganXepConTro } from "@/features/cau-hinh/ngan-xep-con-tro";
import { canEditContent } from "@/features/noi-dung/nhan-noi-dung";
import { OverlayDialog } from "@/features/noi-dung/overlay-dialog";
// vi-name-ok: existing exports of the session reader
import { usePhien, type PhienDaDoc } from "@/features/phien/phien-hien-tai";
import {
  createDisasterAlert,
  deleteDisasterAlert,
  endDisasterAlert,
  getDisasterAlert,
  listDisasterAlerts,
  updateDisasterAlert,
  type DisasterAlert,
  type DisasterAlertPage,
} from "@/lib/api/disaster-alerts";
import type { KetQua } from "@/lib/api/goi";

import { DeleteAlertDialog, EndAlertDialog } from "./alert-action-dialogs";
import {
  ADD_BUTTON,
  alertStatus,
  canEndEarly,
  CLOSE_LABEL,
  COLUMN_ACTIONS,
  COLUMN_KIND,
  COLUMN_STATUS,
  COLUMN_TITLE,
  COLUMN_WINDOW,
  CREATED_TOAST,
  DELETED_TOAST,
  DELETE_BUTTON,
  EDIT_BUTTON,
  EDIT_TITLE,
  EMPTY_HINT_EDITOR,
  EMPTY_TITLE,
  ENDED_NOT_EDITABLE,
  ENDED_TOAST,
  END_BUTTON,
  GONE_TOAST,
  isEndedEarly,
  kindLabel,
  LOAD_FAILED_TITLE,
  LOADING,
  NEXT_PAGE,
  PREV_PAGE,
  READ_ONLY_NOTE,
  SAVED_TOAST,
  STATUS_LABEL,
  TAB_DESCRIPTION,
  TAB_TITLE,
  validityText,
  type AlertStatus,
} from "./alert-form";
import { AlertFormDialog } from "./alert-form-dialog";

const PAGE_SIZE = 20;
const NOTICE_DIALOG_CLASS = "w-[min(32rem,calc(100vw-1rem))]";
const NOTICE_TITLE_ID = "disaster-alert-notice-title";

/** Red for what residents see now; amber for what they will see; grey for what they no longer see. */
const STATUS_TONE: Record<AlertStatus, BadgeTone> = {
  "in-force": "danger",
  upcoming: "warning",
  ended: "neutral",
};

/** Every call the screen makes — injected so the tests drive each answer without a network. */
export type DisasterAlertsApi = {
  readonly list: typeof listDisasterAlerts;
  readonly get: typeof getDisasterAlert;
  readonly create: typeof createDisasterAlert;
  readonly update: typeof updateDisasterAlert;
  readonly end: typeof endDisasterAlert;
  readonly remove: typeof deleteDisasterAlert;
};

const LIVE_API: DisasterAlertsApi = {
  list: listDisasterAlerts,
  get: getDisasterAlert,
  create: createDisasterAlert,
  update: updateDisasterAlert,
  end: endDisasterAlert,
  remove: deleteDisasterAlert,
};

/**
 * The "Cảnh báo thiên tai" tab of `/mini-app` (ADR 0091 §2) — mounted only when the account holds
 * `content.read` (`mini-app-tabs.ts`), so every read here is one the server will answer.
 *
 * WRITE CONTROLS FOLLOW `content.update` (`canEditContent`, as the content tab): without it the list is
 * read-only — no `Đăng cảnh báo`, no `Sửa` / `Kết thúc sớm` / `Xoá`. That is convenience; `service-comms`
 * checks the key on every call and its 403 sentence still reaches the screen (rule 5, forbidden #1).
 */
export function DisasterAlertsTab() {
  const session = usePhien();
  return <DisasterAlertsScreen session={session} api={LIVE_API} />;
}

type Opened =
  | { readonly kind: "create" }
  | { readonly kind: "edit"; readonly id: string }
  | { readonly kind: "end"; readonly alert: DisasterAlert }
  | { readonly kind: "delete"; readonly alert: DisasterAlert };

/** Presentational half, session and calls passed in — so the denied and allowed cases render in a test. */
export function DisasterAlertsScreen({
  session,
  api,
  now = Date.now,
}: {
  session: PhienDaDoc;
  api: DisasterAlertsApi;
  now?: () => number;
}) {
  const canEdit = canEditContent(session);
  const [stack, setStack] = useState<NganXepConTro>(TRANG_DAU);
  const [reloads, setReloads] = useState(0);
  const [loaded, setLoaded] = useState<{ key: string; result: KetQua<DisasterAlertPage> } | null>(null);
  const [opened, setOpened] = useState<Opened | null>(null);
  const [detail, setDetail] = useState<{ id: string; result: KetQua<DisasterAlert> } | null>(null);

  const key = `${stack.hienTai ?? ""}|${reloads}`;

  useEffect(() => {
    let gone = false;
    void api.list(stack.hienTai, PAGE_SIZE).then((result) => {
      if (!gone) setLoaded({ key, result });
    });
    return () => {
      gone = true;
    };
  }, [api, stack.hienTai, key]);

  // EDIT READS THE ALERT AGAIN: a list REPLACES the stored one whole on PATCH, so editing from a stale row
  // would silently put back what a colleague removed a minute ago.
  const editId = opened?.kind === "edit" ? opened.id : null;
  useEffect(() => {
    if (editId === null) return;
    let gone = false;
    void api.get(editId).then((result) => {
      if (!gone) setDetail({ id: editId, result });
    });
    return () => {
      gone = true;
    };
  }, [api, editId]);

  function reload(firstPage: boolean): void {
    if (firstPage) setStack(TRANG_DAU);
    setReloads((n) => n + 1);
  }

  function closeDialog(): void {
    setOpened(null);
    setDetail(null);
  }

  const page = loaded !== null && loaded.key === key ? loaded.result : null;
  const nowMs = now();

  function editDialog(id: string): ReactNode {
    if (detail === null || detail.id !== id) {
      return (
        <DialogNotice onClose={closeDialog}>
          <p className="m-0 flex items-center gap-2 text-sm text-ink-muted" role="status">
            <LoaderCircle aria-hidden="true" focusable="false" className="size-4 animate-spin" />
            Đang tải cảnh báo…
          </p>
        </DialogNotice>
      );
    }
    if (!detail.result.ok) {
      return (
        <DialogNotice onClose={closeDialog}>
          <p className="thong-bao-loi m-0" role="alert">
            {detail.result.thongBao}
          </p>
        </DialogNotice>
      );
    }
    if (isEndedEarly(detail.result.duLieu)) {
      return (
        <DialogNotice onClose={closeDialog}>
          <p className="m-0 text-sm text-ink-700" role="status">
            {ENDED_NOT_EDITABLE}
          </p>
        </DialogNotice>
      );
    }
    return (
      <AlertFormDialog
        alert={detail.result.duLieu}
        create={api.create}
        update={api.update}
        saved={() => {
          toast.success(SAVED_TOAST);
          closeDialog();
          reload(false);
        }}
        close={closeDialog}
      />
    );
  }

  return (
    <section className="flex min-w-0 flex-col gap-4" aria-labelledby="disaster-alerts-title">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div className="min-w-0 flex-1">
          <h2 id="disaster-alerts-title" className="m-0 text-lg font-semibold text-navy">
            {TAB_TITLE}
          </h2>
          <p className="m-0 mt-1 max-w-2xl text-[13px] text-ink-muted">{TAB_DESCRIPTION}</p>
        </div>
        {canEdit && (
          <Button
            type="button"
            variant="primary"
            onClick={() => setOpened({ kind: "create" })}
            icon={<Plus aria-hidden="true" focusable="false" />}
          >
            {ADD_BUTTON}
          </Button>
        )}
      </div>

      {!canEdit && (
        <p className="m-0 rounded-lg border border-line bg-surface-muted px-3 py-2 text-[13px] text-ink-muted" data-testid="read-only-note">
          {READ_ONLY_NOTE}
        </p>
      )}

      <div className="min-w-0 rounded-card border border-line bg-surface">
        {page === null ? (
          <div role="status" aria-busy="true">
            <span className="an-thi-giac">{LOADING}</span>
            <SkeletonRows rows={4} columns={4} />
          </div>
        ) : !page.ok ? (
          <ErrorState role="alert" title={LOAD_FAILED_TITLE} message={page.thongBao} onRetry={() => reload(false)} />
        ) : page.duLieu.items.length === 0 && stack.hienTai === null ? (
          <EmptyState role="status" icon={CloudAlert} title={EMPTY_TITLE} description={canEdit ? EMPTY_HINT_EDITOR : undefined} />
        ) : (
          <>
            {/* Sideways scroll below the table's width (320px screens); the page scrolls vertically. */}
            <div className="overflow-x-auto">
              <table className="w-full min-w-[40rem] border-collapse text-sm">
                <thead>
                  <tr className="border-b border-line text-left text-[12px] text-ink-muted">
                    <th scope="col" className="px-4 py-2 font-semibold">{COLUMN_KIND}</th>
                    <th scope="col" className="px-4 py-2 font-semibold">{COLUMN_TITLE}</th>
                    <th scope="col" className="px-4 py-2 font-semibold">{COLUMN_WINDOW}</th>
                    <th scope="col" className="px-4 py-2 font-semibold">{COLUMN_STATUS}</th>
                    {canEdit && <th scope="col" className="px-4 py-2 text-right font-semibold">{COLUMN_ACTIONS}</th>}
                  </tr>
                </thead>
                <tbody>
                  {page.duLieu.items.map((a) => {
                    const status = alertStatus(a, nowMs);
                    return (
                      <tr key={a.id} className="border-b border-line last:border-b-0" data-testid="alert-row">
                        <td className="px-4 py-2.5 whitespace-nowrap">{kindLabel(a.kind)}</td>
                        <td className="px-4 py-2.5">
                          <span className="font-semibold text-ink-900">{a.title}</span>
                          {a.level !== undefined && a.level !== "" && (
                            <span className="block text-[12px] text-ink-muted">{a.level}</span>
                          )}
                        </td>
                        <td className="px-4 py-2.5 text-[13px] whitespace-nowrap text-ink-700">{validityText(a)}</td>
                        <td className="px-4 py-2.5">
                          <Badge tone={STATUS_TONE[status]}>{STATUS_LABEL[status]}</Badge>
                        </td>
                        {canEdit && (
                          <td className="px-4 py-2.5">
                            <div className="flex justify-end gap-1">
                              {!isEndedEarly(a) && (
                                <IconButton type="button" label={`${EDIT_BUTTON} — ${a.title}`} onClick={() => setOpened({ kind: "edit", id: a.id })}>
                                  <Pencil aria-hidden="true" focusable="false" />
                                </IconButton>
                              )}
                              {canEndEarly(a, nowMs) && (
                                <IconButton type="button" label={`${END_BUTTON} — ${a.title}`} onClick={() => setOpened({ kind: "end", alert: a })}>
                                  <CircleStop aria-hidden="true" focusable="false" />
                                </IconButton>
                              )}
                              <IconButton type="button" label={`${DELETE_BUTTON} — ${a.title}`} onClick={() => setOpened({ kind: "delete", alert: a })}>
                                <Trash2 aria-hidden="true" focusable="false" />
                              </IconButton>
                            </div>
                          </td>
                        )}
                      </tr>
                    );
                  })}
                </tbody>
              </table>
            </div>
            {(coTrangTruoc(stack) || page.duLieu.has_more) && (
              <div className="flex justify-end gap-2 border-t border-line px-4 py-2.5">
                <Button
                  type="button"
                  size="sm"
                  disabled={!coTrangTruoc(stack)}
                  onClick={() => setStack((s) => veTrangTruoc(s))}
                  icon={<ChevronLeft aria-hidden="true" focusable="false" />}
                >
                  {PREV_PAGE}
                </Button>
                <Button
                  type="button"
                  size="sm"
                  disabled={!page.duLieu.has_more || page.duLieu.next_cursor === ""}
                  onClick={() => setStack((s) => sangTrangSau(s, page.duLieu.next_cursor))}
                >
                  {NEXT_PAGE}
                  <ChevronRight aria-hidden="true" focusable="false" />
                </Button>
              </div>
            )}
          </>
        )}
      </div>

      {opened?.kind === "create" && (
        <AlertFormDialog
          create={api.create}
          update={api.update}
          saved={() => {
            toast.success(CREATED_TOAST);
            closeDialog();
            reload(true);
          }}
          close={closeDialog}
        />
      )}

      {opened?.kind === "edit" && editDialog(opened.id)}

      {opened?.kind === "end" && (
        <EndAlertDialog
          alert={opened.alert}
          end={api.end}
          ended={() => {
            toast.success(ENDED_TOAST);
            closeDialog();
            reload(false);
          }}
          close={closeDialog}
        />
      )}

      {opened?.kind === "delete" && (
        <DeleteAlertDialog
          alert={opened.alert}
          remove={api.remove}
          deleted={() => {
            toast.success(DELETED_TOAST);
            closeDialog();
            reload(false);
          }}
          gone={() => {
            toast.info(GONE_TOAST);
            closeDialog();
            reload(false);
          }}
          close={closeDialog}
        />
      )}
    </section>
  );
}

/** The edit dialog before the form can open: loading, a read refusal, or an alert already ended. */
function DialogNotice({ onClose, children }: { onClose: () => void; children: ReactNode }) {
  return (
    <OverlayDialog titleId={NOTICE_TITLE_ID} onDismiss={onClose} className={NOTICE_DIALOG_CLASS}>
      <div className="flex flex-col gap-4">
        <div className="flex items-center gap-3 border-b border-line pb-3">
          <h3 id={NOTICE_TITLE_ID} className="m-0 min-w-0 flex-1 text-base font-semibold text-ink-900">
            {EDIT_TITLE}
          </h3>
          <IconButton type="button" label={CLOSE_LABEL} className="min-h-0" onClick={onClose}>
            <X aria-hidden="true" focusable="false" />
          </IconButton>
        </div>
        {children}
      </div>
    </OverlayDialog>
  );
}
