"use client";

import { KeyRound, Plus, Power, RefreshCw, Smartphone, TriangleAlert, Unplug } from "lucide-react";
import { useState, type FormEvent, type ReactNode } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FormMessage, hintClass, TextAreaField, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { Notice } from "@/components/ui/notice";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import {
  attachMiniApp,
  replaceMiniApp,
  retireMiniAppSecret,
  setMiniAppActivation,
  setMiniAppSecret,
  type CommuneDetail,
  type MiniAppChange,
} from "@/lib/api";
import { cn } from "@/lib/cn";
import { communeError, secretRetirementText, type CommuneField } from "@/lib/errors";

import { formatDateTime, miniAppModeLabel, StatusBadge } from "./commune-parts";

/**
 * The commune's own Mini App: attach, change, detach, reactivate, and its Zalo app secret (ADR 0070
 * with its 02/10/2026 addendum). Only `rieng` rows carry controls; the shared app is not the
 * commune's to change.
 *
 * ORDER OF A CHANGE: the replacement commits first (platform, one transaction); the new App ID's
 * secret is a SEPARATE call after it. A failed secret step therefore never undoes the change — the
 * dialog says the App ID changed but signs nobody in yet, and offers the step again
 * (transaction-boundaries.json `doi_app_id_mini_app_cua_xa`).
 *
 * THE SECRET: a password-type input, cleared after every submit whatever the answer, sent in a JSON
 * body only (lib/api.ts). It is never put into the outcome below, never logged, never in a URL.
 *
 * Every control is a hint; service-platform checks `ops.mini_app.manage` on each call.
 */

type App = CommuneDetail["mini_apps"][number];
type ActionKind = "replace" | "detach" | "reactivate" | "secret";
type Action = { kind: ActionKind; appId: string };
type FieldError = { field: CommuneField; text: string };

/** What the last change left to do, shown under the table until the page is left. Never a secret. */
export type MiniAppOutcome = {
  kind: ActionKind;
  /** The App ID acted on — the OLD one for a replacement. */
  appId: string;
  newAppId?: string;
  newSecretSet?: boolean;
  /** From the change answer; `false` offers "Thu hồi lại". */
  secretRetired?: boolean;
  retirementError?: string;
  /** Re-sent with "Thu hồi lại": the retirement belongs to the same act. */
  reason: string;
};

export const SECRET_STEP_FAILED = "Đã đổi App ID nhưng chưa đặt khoá — dân chưa đăng nhập được qua app mới.";
export const SECRET_NOT_RETIRED = "Khoá của App ID cũ chưa thu hồi";

/** Steps outside ViGov after a change (transaction-boundaries.json `doi_app_id_mini_app_cua_xa` (3)). */
export const MANUAL_STEPS = [
  "Ở vihat-miniapp: thêm cặp App ID mới vào biến ZALO_MINIAPP_COMMUNE_APP_SECRETS.",
  "Build citizen-app trỏ tới App ID mới, rồi đẩy lên Zalo bằng token của App ID mới.",
  "Cho tới khi bản app mới lên Zalo, dân mở bản app cũ sẽ không đăng nhập được.",
];

function detailOf(c: MiniAppChange): CommuneDetail {
  return { id: c.id, name: c.name, province: c.province, active: c.active, domains: c.domains, mini_apps: c.mini_apps };
}

/** Both labels occupy the same cell, so the button keeps its width while busy. */
function BusyLabel({ busy, label, busyLabel }: { busy: boolean; label: string; busyLabel: string }) {
  return (
    <span className="grid">
      <span className={cn("col-start-1 row-start-1", busy && "invisible")}>{label}</span>
      <span className={cn("col-start-1 row-start-1", !busy && "invisible")} aria-hidden={!busy}>
        {busyLabel}
      </span>
    </span>
  );
}

function useFieldError() {
  const guarded = useGuardedError();
  return (err: unknown, action: string): FieldError | null => {
    let field: CommuneField = "form";
    const text = guarded(err, action, (e) => {
      const known = communeError(e);
      if (known) field = known.field;
      return known?.text ?? null;
    });
    return text === null ? null : { field, text };
  };
}

function ReasonField({
  value,
  onChange,
  disabled,
  error,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled: boolean;
  error: FieldError | null;
}) {
  return (
    <TextAreaField
      label="Lý do"
      hint="Bắt buộc, tối đa 500 ký tự. Lý do được ghi vào nhật ký vận hành."
      name="reason"
      rows={3}
      maxLength={500}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      disabled={disabled}
      error={error?.field === "reason" ? error.text : null}
    />
  );
}

function SecretField({
  value,
  onChange,
  disabled,
  error,
}: {
  value: string;
  onChange: (v: string) => void;
  disabled: boolean;
  error: FieldError | null;
}) {
  return (
    <TextField
      label="Khoá bí mật (App Secret) của App ID"
      hint="Sao chép từ trang quản lý Mini App của Zalo. Khoá không hiện lại sau khi lưu."
      name="app_secret"
      type="password"
      autoComplete="off"
      autoCapitalize="none"
      spellCheck={false}
      value={value}
      onChange={(e) => onChange(e.target.value)}
      disabled={disabled}
      error={error?.field === "secret" ? error.text : null}
    />
  );
}

function formLevel(error: FieldError | null, ...inline: CommuneField[]): string | null {
  return error && !inline.includes(error.field) ? error.text : null;
}

// --- the section -------------------------------------------------------------------------------

export function MiniAppSection({
  commune,
  allowed,
  onChanged,
  onMiniAppAttached,
  initialOutcome = null,
}: {
  commune: CommuneDetail;
  allowed: boolean;
  onChanged: (c: CommuneDetail) => void;
  onMiniAppAttached: (app: App) => void;
  /** Rendering seam for tests; the screen starts with none. */
  initialOutcome?: MiniAppOutcome | null;
}) {
  const [action, setAction] = useState<Action | null>(null);
  const [outcome, setOutcome] = useState<MiniAppOutcome | null>(initialOutcome);
  // The commune runs at most ONE own app (ADR 0070 #2): attaching beside it is refused server-side.
  const running = commune.mini_apps.some((a) => a.mode === "rieng" && a.active);
  const close = () => setAction(null);

  return (
    <>
      <MiniAppTable apps={commune.mini_apps} onAction={allowed ? setAction : undefined} />
      {outcome ? <MiniAppOutcomeNotices communeId={commune.id} outcome={outcome} onUpdate={setOutcome} /> : null}
      {allowed && !running ? <AttachMiniAppForm commune={commune} onAttached={onMiniAppAttached} /> : null}
      {allowed && running ? (
        <p className={cn(hintClass, "border-t border-line pt-4")}>
          Xã đang có một Mini App riêng chạy. Muốn dùng App ID khác, chọn “Đổi App ID” ở dòng App ID đó.
        </p>
      ) : null}

      {action?.kind === "replace" ? (
        <ReplaceDialog
          key={"r" + action.appId}
          commune={commune}
          appId={action.appId}
          onClose={close}
          onReplaced={(change, newAppId, reason) => {
            onChanged(detailOf(change));
            setOutcome({
              kind: "replace",
              appId: action.appId,
              newAppId,
              newSecretSet: false,
              secretRetired: change.secret_retired,
              retirementError: change.secret_retirement_error,
              reason,
            });
          }}
          onSecretSet={() => setOutcome((o) => (o ? { ...o, newSecretSet: true } : o))}
        />
      ) : null}
      {action?.kind === "detach" || action?.kind === "reactivate" ? (
        <ActivationDialog
          key={action.kind + action.appId}
          commune={commune}
          appId={action.appId}
          activate={action.kind === "reactivate"}
          onClose={close}
          onDone={(change, reason) => {
            onChanged(detailOf(change));
            setOutcome({
              kind: action.kind,
              appId: action.appId,
              secretRetired: change.secret_retired,
              retirementError: change.secret_retirement_error,
              reason,
            });
            close();
          }}
        />
      ) : null}
      {action?.kind === "secret" ? (
        <SecretDialog
          key={"s" + action.appId}
          commune={commune}
          appId={action.appId}
          onClose={close}
          onDone={(reason) => {
            setOutcome({ kind: "secret", appId: action.appId, reason });
            close();
          }}
        />
      ) : null}
    </>
  );
}

export function MiniAppTable({ apps, onAction }: { apps: App[]; onAction?: (a: Action) => void }) {
  if (apps.length === 0) {
    return <EmptyState icon={Smartphone} tone="neutral" title="Xã chưa gắn Mini App riêng nào." className="py-6" />;
  }
  const icon = (I: typeof KeyRound) => <I aria-hidden="true" focusable="false" strokeWidth={1.8} />;
  return (
    <TableScroll aria-label="Mini App riêng của xã" className="shadow-none">
      <table className={DATA_TABLE_CLASS}>
        <thead>
          <tr>
            <th scope="col">App ID</th>
            <th scope="col">Chế độ</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Gắn lúc</th>
            <th scope="col">Người gắn</th>
            {onAction ? <th scope="col">Thao tác</th> : null}
          </tr>
        </thead>
        <tbody>
          {apps.map((a) => (
            <tr key={a.app_id}>
              <td>
                <code className="font-mono text-[13px] text-ink-900">{a.app_id}</code>
              </td>
              <td className="text-ink-700">{miniAppModeLabel(a.mode)}</td>
              <td>
                <StatusBadge active={a.active} />
              </td>
              <td className="text-ink-700">{formatDateTime(a.created_at)}</td>
              <td className="text-ink-700">{a.created_by}</td>
              {onAction ? (
                <td>
                  {a.mode === "rieng" ? (
                    <div className="flex flex-wrap gap-2">
                      {a.active ? (
                        <>
                          <Button type="button" size="sm" icon={icon(RefreshCw)} onClick={() => onAction({ kind: "replace", appId: a.app_id })}>
                            Đổi App ID
                          </Button>
                          <Button type="button" size="sm" icon={icon(KeyRound)} onClick={() => onAction({ kind: "secret", appId: a.app_id })}>
                            Đặt/đổi khoá bí mật
                          </Button>
                          <Button
                            type="button"
                            size="sm"
                            variant="danger"
                            icon={icon(Unplug)}
                            onClick={() => onAction({ kind: "detach", appId: a.app_id })}
                          >
                            Gỡ khỏi xã
                          </Button>
                        </>
                      ) : (
                        <Button type="button" size="sm" icon={icon(Power)} onClick={() => onAction({ kind: "reactivate", appId: a.app_id })}>
                          Bật lại
                        </Button>
                      )}
                    </div>
                  ) : null}
                </td>
              ) : null}
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

// --- outcome -----------------------------------------------------------------------------------

export function MiniAppOutcomeNotices({
  communeId,
  outcome,
  onUpdate,
}: {
  communeId: string;
  outcome: MiniAppOutcome;
  onUpdate: (o: MiniAppOutcome) => void;
}) {
  const toError = useFieldError();
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);

  async function retryRetire() {
    if (busy) return;
    setBusy(true);
    setError(null);
    try {
      // `retired: false` = nothing was live: the same end state, so both answers settle it.
      await retireMiniAppSecret(communeId, outcome.appId, { reason: outcome.reason });
      onUpdate({ ...outcome, secretRetired: true, retirementError: undefined });
    } catch (err) {
      setError(toError(err, "thu hồi khoá bí mật")?.text ?? null);
    } finally {
      setBusy(false);
    }
  }

  const blocks: ReactNode[] = [];
  if (outcome.secretRetired === false) {
    blocks.push(
      <Notice key="retire" tone="legal" icon={TriangleAlert} title={SECRET_NOT_RETIRED}>
        <p>
          App ID {outcome.appId} đã tắt nên khoá cũ không đăng nhập được cho ai, nhưng khoá vẫn còn lưu ở hệ thống
          xác thực. {secretRetirementText(outcome.retirementError)}
        </p>
        <div>
          <Button type="button" size="sm" onClick={retryRetire} disabled={busy} aria-busy={busy}>
            <BusyLabel busy={busy} label="Thu hồi lại" busyLabel="Đang thu hồi…" />
          </Button>
        </div>
        <FormMessage text={error} />
      </Notice>,
    );
  }
  if (outcome.kind === "replace") {
    blocks.push(
      <Notice key="manual" tone="info" title="Việc còn phải làm ngoài hệ thống:">
        <ul className="m-0 list-disc pl-5">
          {outcome.newSecretSet ? null : (
            <li>{`Đặt khoá bí mật cho App ID ${outcome.newAppId ?? ""} (nút “Đặt/đổi khoá bí mật” ở dòng của App ID đó) — chưa có khoá thì dân chưa đăng nhập được qua app mới.`}</li>
          )}
          {MANUAL_STEPS.map((s) => (
            <li key={s}>{s}</li>
          ))}
        </ul>
      </Notice>,
    );
  }
  if (outcome.kind === "detach") {
    blocks.push(
      <Notice key="detach" tone="neutral">
        <p>App ID {outcome.appId} đã được gỡ khỏi xã (tắt, không xoá). Dân mở Mini App này không đăng nhập được nữa.</p>
      </Notice>,
    );
  }
  if (outcome.kind === "reactivate") {
    blocks.push(
      <Notice key="reactivate" tone="legal" title="Khoá bí mật không tự bật lại:">
        <p>Đặt lại khoá bí mật cho App ID {outcome.appId} thì dân mới đăng nhập được qua app này.</p>
      </Notice>,
    );
  }
  if (outcome.kind === "secret") {
    blocks.push(
      <Notice key="secret" tone="info">
        <p>Đã lưu khoá bí mật cho App ID {outcome.appId}.</p>
      </Notice>,
    );
  }
  return (
    <div role="status" className="flex flex-col gap-3">
      {blocks}
    </div>
  );
}

// --- dialogs -----------------------------------------------------------------------------------

function ReplaceDialog({
  commune,
  appId,
  onClose,
  onReplaced,
  onSecretSet,
}: {
  commune: CommuneDetail;
  appId: string;
  onClose: () => void;
  onReplaced: (change: MiniAppChange, newAppId: string, reason: string) => void;
  onSecretSet: () => void;
}) {
  const toError = useFieldError();
  const [step, setStep] = useState<"app" | "secret">("app");
  const [newAppId, setNewAppId] = useState("");
  const [reason, setReason] = useState("");
  const [secret, setSecret] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<FieldError | null>(null);

  async function submitApp(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const id = newAppId.trim();
    if (!/^\d+$/.test(id)) return setError({ field: "appId", text: "App ID chỉ gồm chữ số, tối đa 32 chữ số." });
    if (reason.trim() === "") return setError({ field: "reason", text: "Hãy ghi lý do đổi App ID." });
    setBusy(true);
    setError(null);
    try {
      onReplaced(await replaceMiniApp(commune.id, appId, { newAppId: id, reason }), id, reason);
      setNewAppId(id);
      setStep("secret");
    } catch (err) {
      setError(toError(err, "đổi App ID"));
    } finally {
      setBusy(false);
    }
  }

  async function submitSecret(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (secret === "") return setError({ field: "secret", text: "Hãy dán khoá bí mật của App ID mới, hoặc chọn “Để sau”." });
    setBusy(true);
    setError(null);
    try {
      await setMiniAppSecret(commune.id, newAppId, { secret, reason });
      onSecretSet();
      onClose();
    } catch (err) {
      const e = toError(err, "đặt khoá bí mật");
      if (e) setError(e.field === "secret" ? e : { field: "form", text: `${SECRET_STEP_FAILED} ${e.text}` });
    } finally {
      setSecret("");
      setBusy(false);
    }
  }

  if (step === "secret") {
    return (
      <Dialog open title="Khoá bí mật của App ID mới" icon={KeyRound} onClose={() => !busy && onClose()}>
        <form className="flex flex-col gap-4" method="post" onSubmit={submitSecret} noValidate>
          <p>
            Đã đổi sang App ID <code className="font-mono text-[13px] text-ink-900">{newAppId}</code>. Dân chỉ đăng nhập
            được qua app mới khi khoá bí mật của App ID này đã được đặt.
          </p>
          <SecretField value={secret} onChange={setSecret} disabled={busy} error={error} />
          <FormMessage text={formLevel(error, "secret")} />
          <DialogActions>
            <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
              <BusyLabel busy={busy} label="Lưu khoá" busyLabel="Đang lưu…" />
            </Button>
            <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
              Để sau
            </Button>
          </DialogActions>
        </form>
      </Dialog>
    );
  }

  return (
    <Dialog open title={`Đổi App ID ${appId} của ${commune.name}`} icon={RefreshCw} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submitApp} noValidate>
        <p>
          App ID mới được gắn và App ID {appId} được tắt (không xoá) trong cùng một lần lưu; khoá bí mật của App ID cũ tự
          thu hồi. Từ lúc đó dân mở bản app cũ không đăng nhập được.
        </p>
        <TextField
          label="App ID mới"
          hint="Dãy chữ số Zalo cấp cho Mini App mới của xã."
          name="new_app_id"
          type="text"
          inputMode="numeric"
          autoComplete="off"
          maxLength={32}
          value={newAppId}
          onChange={(e) => setNewAppId(e.target.value)}
          disabled={busy}
          error={error?.field === "appId" ? error.text : null}
        />
        <ReasonField value={reason} onChange={setReason} disabled={busy} error={error} />
        <FormMessage text={formLevel(error, "appId", "reason")} />
        <DialogActions>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            <BusyLabel busy={busy} label="Đổi App ID" busyLabel="Đang đổi…" />
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

function ActivationDialog({
  commune,
  appId,
  activate,
  onClose,
  onDone,
}: {
  commune: CommuneDetail;
  appId: string;
  activate: boolean;
  onClose: () => void;
  onDone: (change: MiniAppChange, reason: string) => void;
}) {
  const toError = useFieldError();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<FieldError | null>(null);
  const label = activate ? "Bật lại" : "Gỡ App ID";

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (reason.trim() === "") return setError({ field: "reason", text: "Hãy ghi lý do." });
    setBusy(true);
    setError(null);
    try {
      onDone(await setMiniAppActivation(commune.id, appId, { active: activate, reason }), reason);
    } catch (err) {
      setError(toError(err, activate ? "bật lại App ID" : "gỡ App ID"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      open
      title={activate ? `Bật lại App ID ${appId} cho ${commune.name}?` : `Gỡ App ID ${appId} khỏi ${commune.name}?`}
      tone={activate ? "default" : "danger"}
      icon={activate ? Power : TriangleAlert}
      onClose={() => !busy && onClose()}
    >
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        {activate ? (
          <p>
            Xã chỉ chạy một Mini App riêng: nếu đang có App ID khác chạy, hệ thống sẽ từ chối. Khoá bí mật cũ đã thu hồi,
            cần đặt lại khoá sau khi bật.
          </p>
        ) : (
          <Notice tone="legal" icon={TriangleAlert} title="Lưu ý:">
            App ID được tắt, không xoá: từ lúc này dân mở Mini App này không đăng nhập được nữa, và khoá bí mật của App ID
            tự thu hồi. Có thể bật lại sau, nhưng phải nhập lại khoá.
          </Notice>
        )}
        <ReasonField value={reason} onChange={setReason} disabled={busy} error={error} />
        <FormMessage text={formLevel(error, "reason")} />
        <DialogActions>
          <Button type="submit" variant={activate ? "primary" : "danger-solid"} disabled={busy} aria-busy={busy}>
            <BusyLabel busy={busy} label={label} busyLabel="Đang lưu…" />
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

function SecretDialog({
  commune,
  appId,
  onClose,
  onDone,
}: {
  commune: CommuneDetail;
  appId: string;
  onClose: () => void;
  onDone: (reason: string) => void;
}) {
  const toError = useFieldError();
  const [secret, setSecret] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<FieldError | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (secret === "") return setError({ field: "secret", text: "Hãy dán khoá bí mật của App ID." });
    if (reason.trim() === "") return setError({ field: "reason", text: "Hãy ghi lý do." });
    setBusy(true);
    setError(null);
    try {
      await setMiniAppSecret(commune.id, appId, { secret, reason });
      onDone(reason);
    } catch (err) {
      setError(toError(err, "đặt khoá bí mật"));
    } finally {
      setSecret("");
      setBusy(false);
    }
  }

  return (
    <Dialog open title={`Đặt khoá bí mật cho App ID ${appId}`} icon={KeyRound} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <p>Khoá mới thay khoá đang dùng (nếu có). Dân đăng nhập qua app này bằng khoá mới ngay sau khi lưu.</p>
        <SecretField value={secret} onChange={setSecret} disabled={busy} error={error} />
        <ReasonField value={reason} onChange={setReason} disabled={busy} error={error} />
        <FormMessage text={formLevel(error, "secret", "reason")} />
        <DialogActions>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            <BusyLabel busy={busy} label="Lưu khoá" busyLabel="Đang lưu…" />
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

// --- attach --------------------------------------------------------------------------------------

function AttachMiniAppForm({ commune, onAttached }: { commune: CommuneDetail; onAttached: (app: App) => void }) {
  const toError = useFieldError();
  const [appId, setAppId] = useState("");
  const [note, setNote] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<FieldError | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const id = appId.trim();
    if (!/^\d+$/.test(id)) {
      setError({ field: "appId", text: "App ID chỉ gồm chữ số, tối đa 32 chữ số." });
      return;
    }
    setBusy(true);
    setError(null);
    try {
      onAttached(await attachMiniApp(commune.id, { appId: id, note: note.trim() === "" ? undefined : note }));
      setAppId("");
      setNote("");
    } catch (err) {
      setError(toError(err, "gắn Mini App"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <form className="flex max-w-xl flex-col gap-3 border-t border-line pt-4" method="post" onSubmit={submit} noValidate>
      <TextField
        label="App ID của Mini App riêng"
        hint="Dãy chữ số Zalo cấp cho Mini App của xã."
        name="app_id"
        type="text"
        inputMode="numeric"
        autoComplete="off"
        maxLength={32}
        value={appId}
        onChange={(e) => setAppId(e.target.value)}
        disabled={busy}
        error={error?.field === "appId" ? error.text : null}
      />
      <TextAreaField
        label="Ghi chú (không bắt buộc)"
        name="note"
        rows={2}
        maxLength={500}
        value={note}
        onChange={(e) => setNote(e.target.value)}
        disabled={busy}
        error={error?.field === "note" ? error.text : null}
      />
      <FormMessage text={formLevel(error, "appId", "note")} />
      <div>
        <Button
          type="submit"
          variant="secondary"
          disabled={busy}
          aria-busy={busy}
          icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        >
          <BusyLabel busy={busy} label="Gắn Mini App" busyLabel="Đang gắn…" />
        </Button>
      </div>
    </form>
  );
}
