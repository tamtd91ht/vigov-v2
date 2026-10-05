"use client";

import { KeyRound, Plus, RefreshCw, Smartphone, TriangleAlert, Unplug } from "lucide-react";
import { useState, type FormEvent, type ReactNode } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FormMessage, hintClass, TextAreaField, TextField } from "@/components/form-parts";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { Notice } from "@/components/ui/notice";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import {
  attachMiniApp,
  removeMiniApp,
  replaceMiniApp,
  retireMiniAppSecret,
  setMiniAppSecret,
  type CommuneDetail,
  type MiniAppChange,
  type MiniAppSecretSet,
  type MiniAppSecretStatus,
} from "@/lib/api";
import { cn } from "@/lib/cn";
import { communeError, secretRetirementText, type CommuneField } from "@/lib/errors";

import { formatDateTime, miniAppModeLabel, StatusBadge } from "./commune-parts";

/**
 * The commune's own Mini App: attach, change, remove, and its Zalo app secret (ADR 0070 with its
 * 02/10/2026 addendum and its 05/10/2026 amendment). Only `rieng` rows carry controls; the shared app
 * is not the commune's to change.
 *
 * ONE APP ID AT A TIME, REMOVAL IS PERMANENT (§Sửa đổi 05/10/2026 #2): "Đổi App ID" and "Gỡ khỏi xã"
 * soft-delete the old row, which then no longer appears in the detail. There is no "Bật lại": a
 * removed App ID can never be attached again, even to the same commune. The confirmation dialog with
 * its reason is the only guard against a mistaken removal — the owner accepted that cost.
 *
 * ORDER OF A CHANGE: the replacement commits first (platform, one transaction); the new App ID's
 * secret is a SEPARATE call after it. A failed secret step therefore never undoes the change — the
 * dialog says the App ID changed but signs nobody in yet, and offers the step again
 * (transaction-boundaries.json `doi_app_id_mini_app_cua_xa`).
 *
 * THE SECRET IS WRITE-ONLY (§Sửa đổi #3): a password-type input, cleared after every submit whatever
 * the answer, sent in a JSON body only (lib/api.ts). The screen shows only "đặt lúc … bởi …" — never
 * the value, never put into the outcome below, never logged, never in a URL.
 *
 * Every control is a hint; service-platform checks `ops.mini_app.manage` on each call.
 */

type App = CommuneDetail["mini_apps"][number];
type ActionKind = "replace" | "remove" | "secret" | "retire";
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
export const UNBOUND_SECRETS_TITLE = "Khoá còn lưu của App ID đã gỡ";
export const REMOVAL_IS_PERMANENT =
  "Gỡ là vĩnh viễn: App ID này không gắn lại được nữa, kể cả cho chính xã này. Muốn xã có lại Mini App riêng, phải đăng ký Mini App mới trên Zalo rồi gắn App ID mới.";

/**
 * Steps outside ViGov after a change (ADR 0070 §Hệ quả and §Sửa đổi 05/10/2026, last paragraph;
 * transaction-boundaries.json `doi_app_id_mini_app_cua_xa` (3)).
 */
export const MANUAL_STEPS = [
  "Dựng lại citizen-app trỏ tới App ID mới, rồi đẩy lên Zalo bằng token của App ID mới.",
  "Ở vihat-miniapp: thêm cặp App ID mới và khoá bí mật vào biến ZALO_MINIAPP_COMMUNE_APP_SECRETS — bản sao thứ hai của khoá, dùng khi dân gửi vị trí hiện trường.",
  "Cho tới khi bản app mới lên Zalo, dân mở bản app cũ sẽ không đăng nhập được.",
];

/** The detail half of a change answer — unknown extra keys are not carried into the screen's state. */
function detailOf(c: MiniAppChange): CommuneDetail {
  return {
    id: c.id,
    name: c.name,
    province: c.province,
    active: c.active,
    domains: c.domains,
    mini_apps: c.mini_apps,
    unbound_secrets: c.unbound_secrets,
  };
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

/**
 * The secret's state in words — metadata only. `undefined` is the attach answer (it carries no
 * status): say so rather than guess "Chưa đặt". `chua_dat` is a warning because nobody can sign in.
 */
export function SecretStatusText({ secret }: { secret?: MiniAppSecretStatus }) {
  if (secret === undefined) {
    return <span className="text-ink-500">Chưa có thông tin — tải lại trang để xem.</span>;
  }
  switch (secret.status) {
    case "da_dat":
      return (
        <span className="text-ink-700">
          {`Đã đặt lúc ${secret.set_at ? formatDateTime(secret.set_at) : "—"} bởi ${secret.set_by ?? "—"}`}
        </span>
      );
    case "chua_dat":
      return (
        <span className="flex flex-col items-start gap-1">
          <Badge tone="warning" icon={TriangleAlert}>
            Chưa đặt
          </Badge>
          <span className="text-xs text-ink-500">Dân chưa đăng nhập được qua app này.</span>
        </span>
      );
    default:
      return <span className="text-ink-500">Không rõ (hệ thống xác thực chưa trả lời)</span>;
  }
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

  // The server answered with the version metadata: the row shows it without a reload.
  function markSecretSet(set: MiniAppSecretSet) {
    onChanged({
      ...commune,
      mini_apps: commune.mini_apps.map((a) =>
        a.app_id === set.app_id ? { ...a, secret: { status: "da_dat", set_at: set.set_at, set_by: set.set_by } } : a,
      ),
    });
  }

  // `retired` true or false both leave no live secret (false = nothing was live): drop the entry.
  function markRetired(appId: string) {
    if (commune.unbound_secrets === null) return;
    onChanged({ ...commune, unbound_secrets: commune.unbound_secrets.filter((u) => u.app_id !== appId) });
  }

  return (
    <>
      <MiniAppTable apps={commune.mini_apps} onAction={allowed ? setAction : undefined} />
      <UnboundSecretList
        unbound={commune.unbound_secrets}
        onRetire={allowed ? (appId) => setAction({ kind: "retire", appId }) : undefined}
      />
      {outcome ? (
        <MiniAppOutcomeNotices communeId={commune.id} outcome={outcome} onUpdate={setOutcome} onRetired={markRetired} />
      ) : null}
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
          onSecretSet={(set) => {
            markSecretSet(set);
            setOutcome((o) => (o ? { ...o, newSecretSet: true } : o));
          }}
        />
      ) : null}
      {action?.kind === "remove" ? (
        <RemoveDialog
          key={"d" + action.appId}
          commune={commune}
          appId={action.appId}
          onClose={close}
          onDone={(change, reason) => {
            onChanged(detailOf(change));
            setOutcome({
              kind: "remove",
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
          onDone={(set, reason) => {
            markSecretSet(set);
            setOutcome({ kind: "secret", appId: action.appId, reason });
            close();
          }}
        />
      ) : null}
      {action?.kind === "retire" ? (
        <RetireSecretDialog
          key={"k" + action.appId}
          commune={commune}
          appId={action.appId}
          onClose={close}
          onDone={(reason) => {
            markRetired(action.appId);
            setOutcome({ kind: "retire", appId: action.appId, reason });
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
            <th scope="col">Khoá bí mật</th>
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
              <td>
                <SecretStatusText secret={a.secret} />
              </td>
              <td className="text-ink-700">{formatDateTime(a.created_at)}</td>
              <td className="text-ink-700">{a.created_by}</td>
              {onAction ? (
                <td>
                  {/* Only the running own app has controls. A removed App ID is never listed, and
                      there is nothing to do with one that is not running: no reactivation exists. */}
                  {a.mode === "rieng" && a.active ? (
                    <div className="flex flex-wrap gap-2">
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
                        onClick={() => onAction({ kind: "remove", appId: a.app_id })}
                      >
                        Gỡ khỏi xã
                      </Button>
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

/**
 * Secrets identity still holds live for App IDs this commune no longer binds — an automatic
 * retirement after a change or a removal that did not complete (ADR 0070 #4). `null` = identity could
 * not be asked, said as such; `[]` = nothing to show.
 */
export function UnboundSecretList({
  unbound,
  onRetire,
}: {
  unbound: CommuneDetail["unbound_secrets"];
  onRetire?: (appId: string) => void;
}) {
  if (unbound === null) {
    return (
      <p className={hintClass}>
        Chưa biết còn khoá bí mật nào của App ID đã gỡ được lưu hay không (hệ thống xác thực chưa trả lời). Tải lại
        trang sau ít phút.
      </p>
    );
  }
  if (unbound.length === 0) return null;
  return (
    <section aria-labelledby="unbound-secrets" className="flex flex-col gap-2 border-t border-line pt-4">
      <h3 id="unbound-secrets" className="m-0 text-sm font-semibold text-ink-900">
        {UNBOUND_SECRETS_TITLE}
      </h3>
      <p className={cn(hintClass, "m-0")}>
        Các App ID này đã gỡ khỏi xã nên khoá không đăng nhập được cho ai, nhưng khoá vẫn còn lưu ở hệ thống xác thực.
        Nên thu hồi.
      </p>
      <ul className="m-0 flex list-none flex-col gap-2 p-0">
        {unbound.map((u) => (
          <li key={u.app_id} className="flex flex-wrap items-center gap-x-4 gap-y-2">
            <code className="font-mono text-[13px] text-ink-900">{u.app_id}</code>
            <span className="text-sm">
              <SecretStatusText secret={u.secret} />
            </span>
            {onRetire ? (
              <Button type="button" size="sm" icon={<KeyRound aria-hidden="true" focusable="false" strokeWidth={1.8} />} onClick={() => onRetire(u.app_id)}>
                Thu hồi khoá
              </Button>
            ) : null}
          </li>
        ))}
      </ul>
    </section>
  );
}

// --- outcome -----------------------------------------------------------------------------------

export function MiniAppOutcomeNotices({
  communeId,
  outcome,
  onUpdate,
  onRetired,
}: {
  communeId: string;
  outcome: MiniAppOutcome;
  onUpdate: (o: MiniAppOutcome) => void;
  onRetired?: (appId: string) => void;
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
      onRetired?.(outcome.appId);
    } catch (err) {
      setError(toError(err, "thu hồi khoá bí mật")?.text ?? null);
    } finally {
      setBusy(false);
    }
  }

  const blocks: ReactNode[] = [];
  if (outcome.secretRetired === false) {
    blocks.push(
      <Notice key="retire-retry" tone="legal" icon={TriangleAlert} title={SECRET_NOT_RETIRED}>
        <p>
          App ID {outcome.appId} đã gỡ nên khoá cũ không đăng nhập được cho ai, nhưng khoá vẫn còn lưu ở hệ thống xác
          thực. {secretRetirementText(outcome.retirementError)}
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
  if (outcome.kind === "remove") {
    blocks.push(
      <Notice key="remove" tone="neutral">
        <p>
          App ID {outcome.appId} đã được gỡ khỏi xã vĩnh viễn và không gắn lại được. Dân mở Mini App này không đăng nhập
          được nữa; mã QR của xã từ nay mở Mini App dùng chung.
        </p>
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
  if (outcome.kind === "retire") {
    blocks.push(
      <Notice key="retired" tone="info">
        <p>Đã thu hồi khoá bí mật của App ID {outcome.appId}.</p>
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
  onSecretSet: (set: MiniAppSecretSet) => void;
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
      onSecretSet(await setMiniAppSecret(commune.id, newAppId, { secret, reason }));
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
          App ID mới được gắn và App ID {appId} bị gỡ vĩnh viễn trong cùng một lần lưu; khoá bí mật của App ID cũ tự thu
          hồi. Từ lúc đó dân mở bản app cũ không đăng nhập được.
        </p>
        <Notice tone="legal" icon={TriangleAlert} title="Lưu ý:">
          App ID {appId} sau khi đổi không gắn lại được nữa, kể cả cho chính xã này. Mã QR đã in theo App ID cũ không
          dùng được nữa và cần in lại.
        </Notice>
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

/** The removal dialog body, rendered alone in tests: its wording IS the behaviour. */
export function RemoveWarning({ appId }: { appId: string }) {
  return (
    <Notice tone="legal" icon={TriangleAlert} title="Lưu ý:">
      <p className="m-0">{REMOVAL_IS_PERMANENT}</p>
      <p className="m-0">
        Từ lúc gỡ, dân mở Mini App {appId} không đăng nhập được nữa, khoá bí mật của App ID tự thu hồi, và mã QR của xã
        chuyển sang mở Mini App dùng chung.
      </p>
    </Notice>
  );
}

function RemoveDialog({
  commune,
  appId,
  onClose,
  onDone,
}: {
  commune: CommuneDetail;
  appId: string;
  onClose: () => void;
  onDone: (change: MiniAppChange, reason: string) => void;
}) {
  const toError = useFieldError();
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<FieldError | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (reason.trim() === "") return setError({ field: "reason", text: "Hãy ghi lý do." });
    setBusy(true);
    setError(null);
    try {
      onDone(await removeMiniApp(commune.id, appId, { reason }), reason);
    } catch (err) {
      setError(toError(err, "gỡ App ID"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog
      open
      title={`Gỡ vĩnh viễn App ID ${appId} khỏi ${commune.name}?`}
      tone="danger"
      icon={TriangleAlert}
      onClose={() => !busy && onClose()}
    >
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <RemoveWarning appId={appId} />
        <ReasonField value={reason} onChange={setReason} disabled={busy} error={error} />
        <FormMessage text={formLevel(error, "reason")} />
        <DialogActions>
          <Button type="submit" variant="danger-solid" disabled={busy} aria-busy={busy}>
            <BusyLabel busy={busy} label="Gỡ App ID" busyLabel="Đang lưu…" />
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
  onDone: (set: MiniAppSecretSet, reason: string) => void;
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
      onDone(await setMiniAppSecret(commune.id, appId, { secret, reason }), reason);
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

/** DELETE …/secret for a removed App ID still holding a live secret (`unbound_secrets`). */
function RetireSecretDialog({
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
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<FieldError | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    if (reason.trim() === "") return setError({ field: "reason", text: "Hãy ghi lý do." });
    setBusy(true);
    setError(null);
    try {
      // `retired: false` = nothing was live any more: the same end state, so both answers settle it.
      await retireMiniAppSecret(commune.id, appId, { reason });
      onDone(reason);
    } catch (err) {
      setError(toError(err, "thu hồi khoá bí mật"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open title={`Thu hồi khoá bí mật của App ID ${appId}?`} icon={KeyRound} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <p>
          App ID {appId} đã gỡ khỏi xã nên không ai đăng nhập qua khoá này. Thu hồi xoá hiệu lực của khoá ở hệ thống xác
          thực; dân đang dùng Mini App hiện tại của xã không bị ảnh hưởng.
        </p>
        <ReasonField value={reason} onChange={setReason} disabled={busy} error={error} />
        <FormMessage text={formLevel(error, "reason")} />
        <DialogActions>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            <BusyLabel busy={busy} label="Thu hồi khoá" busyLabel="Đang thu hồi…" />
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
        hint="Dãy chữ số Zalo cấp cho Mini App của xã. App ID đã gỡ trước đây không gắn lại được."
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
