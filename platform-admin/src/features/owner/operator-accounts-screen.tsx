"use client";

import { KeyRound, Lock, ShieldCheck, UserPlus, UsersRound } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FormMessage, labelClass, TextField } from "@/components/form-parts";
import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { useOperator } from "@/features/operator/operator-context";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import {
  ALL_OPS_KEYS,
  createOperatorAccount,
  listOperatorAccounts,
  OPS_KEY_LABELS,
  resetOperatorPassword,
  setOperatorPermissions,
  type OperatorAccount,
} from "@/lib/api";

import {
  checkCreateOperator,
  checkReason,
  checkReset,
  OPERATOR_CREATE_NOTE,
  OPERATOR_RESET_NOTE,
  operatorStatus,
  ownerAccountError,
  PERMISSIONS_REDUCED_NOTE,
  SESSIONS_REVOKED_NOTE,
  type OwnerError,
} from "./owner-model";
import { at, formLevel, ReasonField, StatusChips, TemporaryPasswordFields, useOwnerError } from "./owner-parts";

/**
 * `/phan-quyen` — the operator accounts, OWNER ONLY (ADR 0048 §Sửa đổi 13/10/2026 #2, #3, #6).
 *
 * The page renders nothing that asks the server anything until the operator context says
 * `is_owner: true`: a non-owner gets a sentence, and no request is made (each would be a 403). The
 * owner row carries no control — no screen removes or transfers the owner (§Sửa đổi #1).
 */

export const OWNER_ONLY_TEXT =
  "Chỉ tài khoản owner của khu vận hành quản lý được tài khoản vận hành và quyền. Tài khoản của bạn không phải owner.";
export const OWNER_ROW_LABEL = "Owner — toàn quyền";
export const CREATE_OPERATOR_LABEL = "Tạo tài khoản vận hành";

type Action = { kind: "permissions" | "password"; account: OperatorAccount } | { kind: "create" };

/** One key as a chip: the key itself, its meaning as the tooltip. A key outside the closed set shows as sent. */
function KeyChips({ keys }: { keys: readonly string[] }) {
  if (keys.length === 0) return <span className="text-ink-500">Chưa có quyền</span>;
  return (
    <ul className="m-0 flex list-none flex-wrap gap-1.5 p-0">
      {keys.map((k) => (
        <li key={k} title={OPS_KEY_LABELS[k] ?? k} className="rounded-full bg-[#f1f4f8] px-2 py-0.5 font-mono text-xs text-ink-700">
          {k}
        </li>
      ))}
    </ul>
  );
}

export function OperatorAccountTable({
  accounts,
  now,
  onAction,
}: {
  accounts: readonly OperatorAccount[];
  now: Date;
  onAction: (a: Action) => void;
}) {
  if (accounts.length === 0) {
    return <EmptyState icon={UsersRound} tone="neutral" title="Chưa có tài khoản vận hành nào." className="py-6" />;
  }
  const icon = (I: typeof KeyRound) => <I aria-hidden="true" focusable="false" strokeWidth={1.8} />;
  return (
    <TableScroll sticky aria-label="Tài khoản vận hành">
      <table className={DATA_TABLE_CLASS}>
        <caption className="sr-only">Tài khoản vận hành và quyền</caption>
        <thead>
          <tr>
            <th scope="col">Mã</th>
            <th scope="col">Email</th>
            <th scope="col">Tên</th>
            <th scope="col">Quyền</th>
            <th scope="col">Trạng thái</th>
            <th scope="col">Thao tác</th>
          </tr>
        </thead>
        <tbody>
          {accounts.map((a) => (
            <tr key={a.code}>
              <td className="font-mono text-[13px] text-ink-900">{a.code}</td>
              <td className="break-all text-ink-700">{a.email}</td>
              <td className="text-ink-900">{a.display_name}</td>
              <td>{a.is_owner ? <span className="text-ink-700">Toàn quyền</span> : <KeyChips keys={a.permission_keys} />}</td>
              <td>
                <StatusChips chips={operatorStatus(a, now)} />
              </td>
              <td>
                {a.is_owner ? (
                  <span className="text-ink-500">{OWNER_ROW_LABEL}</span>
                ) : (
                  <div className="flex flex-wrap gap-2">
                    <Button type="button" size="sm" icon={icon(ShieldCheck)} onClick={() => onAction({ kind: "permissions", account: a })}>
                      Phân quyền
                    </Button>
                    <Button type="button" size="sm" icon={icon(KeyRound)} onClick={() => onAction({ kind: "password", account: a })}>
                      Đặt lại mật khẩu
                    </Button>
                  </div>
                )}
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

/** The closed `ops.*` set as checkboxes, each with its meaning and its key. */
export function PermissionKeyChecklist({
  selected,
  onChange,
  disabled,
  legend,
}: {
  selected: readonly string[];
  onChange: (keys: string[]) => void;
  disabled: boolean;
  legend: string;
}) {
  return (
    <fieldset className="m-0 flex min-w-0 flex-col gap-1 border-0 p-0">
      <legend className={labelClass}>{legend}</legend>
      {ALL_OPS_KEYS.map((k) => (
        <label key={k} className="flex min-h-9 cursor-pointer items-start gap-2.5 py-1 text-sm text-ink-900">
          <input
            type="checkbox"
            name="permission_keys"
            value={k}
            checked={selected.includes(k)}
            disabled={disabled}
            // Kept in the closed set's order, so the body is stable whatever order they were ticked in.
            onChange={(e) =>
              onChange(e.target.checked ? ALL_OPS_KEYS.filter((x) => x === k || selected.includes(x)) : selected.filter((x) => x !== k))
            }
            className="m-0 mt-0.5 size-5 shrink-0 cursor-pointer accent-brand-600"
          />
          <span className="flex min-w-0 flex-col">
            <span>{OPS_KEY_LABELS[k] ?? k}</span>
            <code className="font-mono text-xs text-ink-500">{k}</code>
          </span>
        </label>
      ))}
    </fieldset>
  );
}

export function OperatorAccountsGate({
  status,
  isOwner,
  message,
}: {
  status: "loading" | "ready" | "error";
  isOwner: boolean;
  message?: string;
}) {
  if (status === "loading") {
    return (
      <Card>
        <p role="status" className="sr-only">
          Đang kiểm tra quyền…
        </p>
        <SkeletonRows rows={3} columns={2} />
      </Card>
    );
  }
  if (status === "error") {
    return (
      <Card as="section">
        <ErrorState role="alert" title="Chưa đọc được quyền của tài khoản" message={message} />
      </Card>
    );
  }
  if (!isOwner) {
    return (
      <Card as="section">
        <EmptyState icon={Lock} tone="neutral" title="Chỉ owner dùng được màn này" description={OWNER_ONLY_TEXT} />
      </Card>
    );
  }
  return <OperatorAccountsWorkspace />;
}

export function OperatorAccountsScreen() {
  const op = useOperator();
  return (
    <OperatorAccountsGate
      status={op.status}
      isOwner={op.status === "ready" && op.operator.is_owner === true}
      message={op.status === "error" ? op.message : undefined}
    />
  );
}

function OperatorAccountsWorkspace() {
  const guarded = useGuardedError();
  const [accounts, setAccounts] = useState<OperatorAccount[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [action, setAction] = useState<Action | null>(null);
  const [done, setDone] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    listOperatorAccounts().then(
      (items) => alive && setAccounts(items),
      (err: unknown) => {
        if (!alive) return;
        const message = guarded(err, "xem tài khoản vận hành", (e) => ownerAccountError(e)?.text ?? null);
        if (message !== null) setLoadError(message);
      },
    );
    return () => {
      alive = false;
    };
  }, [guarded]);

  function replace(updated: OperatorAccount) {
    setAccounts((list) => (list ? list.map((a) => (a.code === updated.code ? updated : a)) : list));
  }

  const close = () => setAction(null);

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <p className="m-0 text-sm text-ink-700">
          Tài khoản mới không có quyền nào cho tới khi được cấp. Owner không gỡ hay chuyển được từ màn này.
        </p>
        <Button
          type="button"
          variant="primary"
          icon={<UserPlus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
          onClick={() => {
            setDone(null);
            setAction({ kind: "create" });
          }}
        >
          {CREATE_OPERATOR_LABEL}
        </Button>
      </div>

      {done ? (
        <p role="status" className="m-0 text-sm font-medium text-success-600">
          {done}
        </p>
      ) : null}

      {accounts !== null ? (
        <OperatorAccountTable
          accounts={accounts}
          now={new Date()}
          onAction={(a) => {
            setDone(null);
            setAction(a);
          }}
        />
      ) : loadError !== null ? (
        <Card as="section">
          <ErrorState role="alert" title="Chưa tải được danh sách tài khoản vận hành" message={loadError} />
        </Card>
      ) : (
        <Card>
          <p role="status" className="sr-only">
            Đang tải tài khoản vận hành…
          </p>
          <SkeletonRows rows={4} />
        </Card>
      )}

      {action?.kind === "create" ? (
        <CreateOperatorDialog
          onClose={close}
          onDone={(a) => {
            setAccounts((list) => (list ? [...list, a] : [a]));
            setDone(`Đã tạo tài khoản ${a.code}. ${OPERATOR_CREATE_NOTE}`);
            close();
          }}
        />
      ) : null}
      {action?.kind === "permissions" ? (
        <PermissionsDialog
          key={"p" + action.account.code}
          account={action.account}
          onClose={close}
          onDone={(a) => {
            replace(a);
            setDone(`Đã lưu quyền của ${a.code}.`);
            close();
          }}
        />
      ) : null}
      {action?.kind === "password" ? (
        <ResetOperatorPasswordDialog
          key={"r" + action.account.code}
          account={action.account}
          onClose={close}
          onDone={(a) => {
            replace(a);
            setDone(`Đã đặt lại mật khẩu tạm cho ${a.code}. ${OPERATOR_RESET_NOTE}`);
            close();
          }}
        />
      ) : null}
    </div>
  );
}

// --- dialogs -------------------------------------------------------------------------------------

function CreateOperatorDialog({ onClose, onDone }: { onClose: () => void; onDone: (a: OperatorAccount) => void }) {
  const toError = useOwnerError(ownerAccountError);
  const [email, setEmail] = useState("");
  const [displayName, setDisplayName] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [keys, setKeys] = useState<string[]>([]);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<OwnerError | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const local = checkCreateOperator({ email, displayName, password, confirm });
    if (local) return setError(local);
    setBusy(true);
    setError(null);
    try {
      onDone(await createOperatorAccount({ email: email.trim(), displayName: displayName.trim(), temporaryPassword: password, permissionKeys: keys }));
    } catch (err) {
      setError(toError(err, "tạo tài khoản vận hành"));
    } finally {
      setPassword("");
      setConfirm("");
      setBusy(false);
    }
  }

  return (
    <Dialog open title={CREATE_OPERATOR_LABEL} icon={UserPlus} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <Notice tone="info">
          <p>{OPERATOR_CREATE_NOTE}</p>
        </Notice>
        <TextField
          label="Email đăng nhập"
          name="email"
          type="email"
          autoComplete="off"
          autoCapitalize="none"
          spellCheck={false}
          value={email}
          onChange={(e) => setEmail(e.target.value)}
          disabled={busy}
          error={at(error, "email")}
        />
        <TextField
          label="Tên hiển thị"
          name="display_name"
          type="text"
          autoComplete="off"
          maxLength={200}
          value={displayName}
          onChange={(e) => setDisplayName(e.target.value)}
          disabled={busy}
          error={at(error, "name")}
        />
        <TemporaryPasswordFields
          password={password}
          confirm={confirm}
          onPassword={setPassword}
          onConfirm={setConfirm}
          disabled={busy}
          error={error}
        />
        <PermissionKeyChecklist selected={keys} onChange={setKeys} disabled={busy} legend="Quyền ban đầu (có thể để trống)" />
        <FormMessage text={formLevel(error, "email", "name", "password", "confirm")} />
        <DialogActions>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            {busy ? "Đang tạo…" : "Tạo tài khoản"}
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

function PermissionsDialog({
  account,
  onClose,
  onDone,
}: {
  account: OperatorAccount;
  onClose: () => void;
  onDone: (a: OperatorAccount) => void;
}) {
  const toError = useOwnerError(ownerAccountError);
  // Only keys of the closed set are offered; one the server knows and this console does not is kept
  // as it is, so saving never silently revokes it.
  const [keys, setKeys] = useState<string[]>(account.permission_keys.filter((k) => ALL_OPS_KEYS.includes(k)));
  const unknown = account.permission_keys.filter((k) => !ALL_OPS_KEYS.includes(k));
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<OwnerError | null>(null);
  const reduced = account.permission_keys.some((k) => !keys.includes(k) && ALL_OPS_KEYS.includes(k));

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const local = checkReason(reason);
    if (local) return setError(local);
    setBusy(true);
    setError(null);
    try {
      onDone(await setOperatorPermissions(account.code, { permissionKeys: [...keys, ...unknown], reason: reason.trim() }));
    } catch (err) {
      setError(toError(err, "phân quyền tài khoản vận hành"));
    } finally {
      setBusy(false);
    }
  }

  return (
    <Dialog open title={`Phân quyền cho ${account.code} (${account.display_name})?`} icon={ShieldCheck} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <PermissionKeyChecklist selected={keys} onChange={setKeys} disabled={busy} legend="Quyền của tài khoản" />
        {reduced ? (
          <Notice tone="legal" title="Lưu ý:">
            <p>{PERMISSIONS_REDUCED_NOTE}</p>
          </Notice>
        ) : null}
        <ReasonField value={reason} onChange={setReason} disabled={busy} error={error} />
        <FormMessage text={formLevel(error, "reason")} />
        <DialogActions>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : "Lưu quyền"}
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

function ResetOperatorPasswordDialog({
  account,
  onClose,
  onDone,
}: {
  account: OperatorAccount;
  onClose: () => void;
  onDone: (a: OperatorAccount) => void;
}) {
  const toError = useOwnerError(ownerAccountError);
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [reason, setReason] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<OwnerError | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const local = checkReset({ password, confirm, reason });
    if (local) return setError(local);
    setBusy(true);
    setError(null);
    try {
      onDone(await resetOperatorPassword(account.code, { temporaryPassword: password, reason: reason.trim() }));
    } catch (err) {
      setError(toError(err, "đặt lại mật khẩu tài khoản vận hành"));
    } finally {
      setPassword("");
      setConfirm("");
      setBusy(false);
    }
  }

  return (
    <Dialog open title={`Đặt lại mật khẩu cho ${account.code} (${account.display_name})?`} icon={KeyRound} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <Notice tone="legal" title="Lưu ý:">
          <p>{OPERATOR_RESET_NOTE}</p>
          <p>{SESSIONS_REVOKED_NOTE}</p>
        </Notice>
        <TemporaryPasswordFields
          password={password}
          confirm={confirm}
          onPassword={setPassword}
          onConfirm={setConfirm}
          disabled={busy}
          error={error}
        />
        <ReasonField value={reason} onChange={setReason} disabled={busy} error={error} />
        <FormMessage text={formLevel(error, "password", "confirm", "reason")} />
        <DialogActions>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : "Đặt lại mật khẩu"}
          </Button>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
