"use client";

import { Replace, Smartphone, TriangleAlert } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FormMessage, TextAreaField, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { formatDateTime } from "@/features/communes/commune-parts";
import { usePermissionKeys } from "@/features/operator/operator-context";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import { ApiError, declareSharedMiniApp, getSharedMiniApp, type SharedMiniApp } from "@/lib/api";
import { SHARED_APP_NOT_DECLARED, sharedMiniAppError, type SharedMiniAppField } from "@/lib/errors";
import { canManageMiniApps } from "@/lib/permissions";

/**
 * `/mini-app-dung-chung` — the platform's SHARED Mini App (the `mini_app` row of mode `chinh`, no
 * commune; owner 04/10/2026, ADR 0073 #5). Read with any `ops.*` key; declare / replace only with
 * `ops.mini_app.manage` (a hint — the server checks).
 *
 * REPLACING IS NOT A TIDY-UP. It takes effect at once for citizen sign-ins (no cache), and every QR
 * already printed carries the OLD App ID: those stop opening a session. The dialog says so before the
 * button, in a warning, and the reason is mandatory.
 */

export type SharedAppState =
  | { status: "loading" }
  | { status: "none" }
  | { status: "ready"; app: SharedMiniApp }
  | { status: "error"; message: string };

export const REPLACE_WARNING =
  "Đổi App ID có hiệu lực ngay khi lưu. Mọi mã QR đã in mang App ID cũ sẽ không mở được phiên đăng nhập nữa, và người dân đăng nhập qua App ID cũ sẽ không đăng nhập được. Chỉ đổi khi App ID mới đã sẵn sàng trên Zalo, và chuẩn bị in lại mã QR cho các xã.";

export const DECLARE_NOTICE =
  "App ID này là Mini App dùng chung của nền tảng: mã QR mở Mini App của mọi xã được tạo từ App ID này. Kiểm tra kỹ App ID trên trang quản lý Mini App của Zalo trước khi lưu.";

export function SharedMiniAppView({
  state,
  canManage,
  onDeclare,
}: {
  state: SharedAppState;
  canManage: boolean;
  onDeclare: () => void;
}) {
  if (state.status === "loading") {
    return (
      <Card>
        <p role="status" className="sr-only">
          Đang tải Mini App dùng chung…
        </p>
        <SkeletonRows rows={3} columns={2} />
      </Card>
    );
  }
  if (state.status === "error") {
    return (
      <Card as="section">
        <ErrorState role="alert" title="Chưa tải được Mini App dùng chung" message={state.message} />
      </Card>
    );
  }
  return (
    <Card as="section" aria-labelledby="shared-mini-app-title">
      <CardHeader>
        <Smartphone aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
        <CardTitle id="shared-mini-app-title">App ID đang dùng</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {state.status === "none" ? (
          <EmptyState icon={Smartphone} tone="neutral" title={SHARED_APP_NOT_DECLARED} className="py-6" />
        ) : (
          <dl className="m-0 grid grid-cols-1 gap-x-8 gap-y-3 sm:grid-cols-3">
            <div className="flex min-w-0 flex-col gap-1">
              <dt className="text-xs font-medium text-ink-500">App ID</dt>
              <dd className="m-0 font-mono text-sm font-semibold break-all text-ink-900">{state.app.app_id}</dd>
            </div>
            <div className="flex min-w-0 flex-col gap-1">
              <dt className="text-xs font-medium text-ink-500">Khai báo lúc</dt>
              <dd className="m-0 text-sm text-ink-900">{formatDateTime(state.app.created_at)}</dd>
            </div>
            <div className="flex min-w-0 flex-col gap-1">
              <dt className="text-xs font-medium text-ink-500">Người khai báo</dt>
              <dd className="m-0 font-mono text-sm text-ink-900">{state.app.created_by}</dd>
            </div>
          </dl>
        )}
        {canManage ? (
          <div className="border-t border-line pt-4">
            <Button
              type="button"
              variant={state.status === "none" ? "primary" : "danger"}
              onClick={onDeclare}
              icon={
                state.status === "none" ? (
                  <Smartphone aria-hidden="true" focusable="false" strokeWidth={1.8} />
                ) : (
                  <Replace aria-hidden="true" focusable="false" strokeWidth={1.8} />
                )
              }
            >
              {state.status === "none" ? "Khai báo App ID" : "Đổi App ID dùng chung"}
            </Button>
          </div>
        ) : null}
      </CardContent>
    </Card>
  );
}

export function SharedMiniAppScreen() {
  const guarded = useGuardedError();
  const keys = usePermissionKeys();
  const [state, setState] = useState<SharedAppState>({ status: "loading" });
  const [open, setOpen] = useState(false);

  useEffect(() => {
    let alive = true;
    getSharedMiniApp().then(
      (app) => {
        if (alive) setState({ status: "ready", app });
      },
      (err: unknown) => {
        if (!alive) return;
        // 404 here is the singleton not declared yet — a state of the page, not a failure.
        if (err instanceof ApiError && err.status === 404 && err.code === "shared_mini_app_not_declared") {
          setState({ status: "none" });
          return;
        }
        const message = guarded(err, "xem Mini App dùng chung", (e) => sharedMiniAppError(e)?.text ?? null);
        if (message !== null) setState({ status: "error", message });
      },
    );
    return () => {
      alive = false;
    };
  }, [guarded]);

  return (
    <>
      <SharedMiniAppView state={state} canManage={canManageMiniApps(keys)} onDeclare={() => setOpen(true)} />
      {open && (state.status === "none" || state.status === "ready") ? (
        <DeclareSharedMiniAppDialog
          current={state.status === "ready" ? state.app : null}
          onClose={() => setOpen(false)}
          onSaved={(app) => {
            setState({ status: "ready", app });
            setOpen(false);
          }}
        />
      ) : null}
    </>
  );
}

/** The dialog's body; no saving, so the warning is rendered in tests. */
export function DeclareSharedMiniAppFields({
  current,
  appId,
  reason,
  onAppId,
  onReason,
  busy,
  error,
}: {
  current: SharedMiniApp | null;
  appId: string;
  reason: string;
  onAppId: (v: string) => void;
  onReason: (v: string) => void;
  busy: boolean;
  error: { field: SharedMiniAppField; text: string } | null;
}) {
  return (
    <>
      {current !== null ? (
        <Notice tone="legal" icon={TriangleAlert} title="Lưu ý:">
          {REPLACE_WARNING}
        </Notice>
      ) : (
        <Notice tone="info">{DECLARE_NOTICE}</Notice>
      )}
      {current !== null ? (
        <p>
          App ID đang dùng: <code className="font-mono text-[13px] text-ink-900">{current.app_id}</code>
        </p>
      ) : null}
      <TextField
        label={current !== null ? "App ID mới" : "App ID"}
        hint="Chỉ gồm chữ số, tối đa 32 chữ số — lấy từ trang quản lý Mini App của Zalo."
        name="app_id"
        type="text"
        inputMode="numeric"
        autoComplete="off"
        spellCheck={false}
        maxLength={32}
        value={appId}
        onChange={(e) => onAppId(e.target.value)}
        disabled={busy}
        error={error?.field === "appId" ? error.text : null}
      />
      <TextAreaField
        label="Lý do"
        hint="Bắt buộc, tối đa 500 ký tự. Lý do được ghi vào nhật ký vận hành."
        name="reason"
        rows={3}
        maxLength={500}
        value={reason}
        onChange={(e) => onReason(e.target.value)}
        disabled={busy}
        error={error?.field === "reason" ? error.text : null}
      />
      <FormMessage text={error?.field === "form" ? error.text : null} />
    </>
  );
}

function DeclareSharedMiniAppDialog({
  current,
  onClose,
  onSaved,
}: {
  current: SharedMiniApp | null;
  onClose: () => void;
  onSaved: (app: SharedMiniApp) => void;
}) {
  const guarded = useGuardedError();
  const [appId, setAppId] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<{ field: SharedMiniAppField; text: string } | null>(null);
  const replacing = current !== null;

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const id = appId.trim();
    if (id === "") return setError({ field: "appId", text: "Hãy nhập App ID." });
    if (replacing && id === current.app_id) {
      return setError({ field: "appId", text: "App ID mới trùng App ID đang dùng. Nhập App ID khác." });
    }
    if (reason.trim() === "") return setError({ field: "reason", text: "Hãy ghi lý do." });
    setBusy(true);
    setError(null);
    try {
      onSaved(await declareSharedMiniApp({ appId: id, reason }));
    } catch (err) {
      let field: SharedMiniAppField = "form";
      const text = guarded(err, replacing ? "đổi App ID Mini App dùng chung" : "khai báo Mini App dùng chung", (e) => {
        const known = sharedMiniAppError(e);
        if (known) field = known.field;
        return known?.text ?? null;
      });
      if (text !== null) setError({ field, text });
    } finally {
      setBusy(false);
    }
  }

  const label = replacing ? "Đổi App ID dùng chung" : "Khai báo App ID";
  return (
    <Dialog
      open
      title={replacing ? "Đổi App ID của Mini App dùng chung?" : "Khai báo Mini App dùng chung"}
      tone={replacing ? "danger" : "default"}
      icon={replacing ? TriangleAlert : Smartphone}
      onClose={() => !busy && onClose()}
    >
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <DeclareSharedMiniAppFields
          current={current}
          appId={appId}
          reason={reason}
          onAppId={setAppId}
          onReason={setReason}
          busy={busy}
          error={error}
        />
        <DialogActions>
          <Button type="submit" variant={replacing ? "danger-solid" : "primary"} disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : label}
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
