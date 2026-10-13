"use client";

import { KeyRound, UserPlus, UsersRound } from "lucide-react";
import { useEffect, useState, type FormEvent } from "react";

import { Dialog, DialogActions } from "@/components/dialog";
import { FormMessage, hintClass, TextField } from "@/components/form-parts";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { useGuardedError } from "@/features/operator/use-guarded-error";
import {
  createCommuneAdminAccount,
  listCommuneStaffAccounts,
  resetStaffPassword,
  type CommuneDetail,
  type StaffAccount,
} from "@/lib/api";

import {
  checkCreateCommuneAdmin,
  checkReset,
  COMMUNE_ADMIN_NOTE,
  ownerAccountError,
  STAFF_DISABLED_RESET,
  STAFF_RESET_NOTE,
  staffStatus,
  type OwnerError,
} from "./owner-model";
import { at, formLevel, ReasonField, StatusChips, TemporaryPasswordFields, useOwnerError } from "./owner-parts";

/**
 * A commune's sign-in accounts on its detail page — OWNER ONLY (ADR 0048 §Sửa đổi 13/10/2026 #2, #4, #5).
 *
 * THE LIST IS AN AUDITED READ OF PERSONAL DATA: identity writes an entry in the commune's own
 * audit_log for every read. So this component is mounted ONLY for the owner (the detail page decides),
 * and it reads once, when mounted — never in the background, never for a non-owner.
 *
 * Every act lands in the COMMUNE's trail with the owner's VH- code (§Sửa đổi #5). The commune can see it.
 */

export const CREATE_ADMIN_LABEL = "Thêm tài khoản quản trị";
export const CREATE_ADMIN_INACTIVE =
  "Xã đang ngừng hoạt động nên không tạo được tài khoản. Bật hoạt động trở lại trước, nếu việc đó đúng.";

export function StaffAccountTable({
  accounts,
  now,
  onReset,
}: {
  accounts: readonly StaffAccount[];
  now: Date;
  onReset: (a: StaffAccount) => void;
}) {
  if (accounts.length === 0) {
    return <EmptyState icon={UsersRound} tone="neutral" title="Xã chưa có tài khoản đăng nhập nào." className="py-6" />;
  }
  return (
    <TableScroll aria-label="Tài khoản cán bộ của xã" className="shadow-none">
      <table className={DATA_TABLE_CLASS}>
        <thead>
          <tr>
            <th scope="col">Mã</th>
            <th scope="col">Họ và tên</th>
            <th scope="col">Email</th>
            <th scope="col">Vai trò</th>
            <th scope="col">Trạng thái</th>
            <th scope="col" className="text-right">
              <span className="sr-only">Thao tác</span>
            </th>
          </tr>
        </thead>
        <tbody>
          {accounts.map((a) => (
            <tr key={a.user_id}>
              <td className="font-mono text-[13px] text-ink-900">{a.staff_code}</td>
              <td className="text-ink-900">{a.full_name}</td>
              <td className="break-all text-ink-700">{a.email}</td>
              <td>
                <span className="flex flex-wrap items-center gap-1.5 text-ink-700">
                  {a.role_name || a.role_code || "—"}
                  {a.is_admin ? <Badge tone="info">Quản trị</Badge> : null}
                </span>
              </td>
              <td>
                <StatusChips chips={staffStatus(a, now)} />
              </td>
              <td className="text-right">
                {/* A disabled account is the commune's decision; the owner does not hand it a password. */}
                <div className="flex flex-col items-end gap-1">
                  <Button
                    type="button"
                    size="sm"
                    disabled={a.disabled}
                    icon={<KeyRound aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                    onClick={() => onReset(a)}
                  >
                    Đặt lại mật khẩu
                  </Button>
                  {a.disabled ? <span className="text-xs text-ink-muted">{STAFF_DISABLED_RESET}</span> : null}
                </div>
              </td>
            </tr>
          ))}
        </tbody>
      </table>
    </TableScroll>
  );
}

export function CommuneStaffAccountsSection({ commune }: { commune: CommuneDetail }) {
  const guarded = useGuardedError();
  const [accounts, setAccounts] = useState<StaffAccount[] | null>(null);
  const [loadError, setLoadError] = useState<string | null>(null);
  const [creating, setCreating] = useState(false);
  const [resetting, setResetting] = useState<StaffAccount | null>(null);
  const [done, setDone] = useState<string | null>(null);

  useEffect(() => {
    let alive = true;
    listCommuneStaffAccounts(commune.id).then(
      (items) => alive && setAccounts(items),
      (err: unknown) => {
        if (!alive) return;
        const message = guarded(err, "xem tài khoản cán bộ của xã", (e) => ownerAccountError(e)?.text ?? null);
        if (message !== null) setLoadError(message);
      },
    );
    return () => {
      alive = false;
    };
    // The commune's id only: re-reading on every detail change would write a trail entry each time.
  }, [commune.id, guarded]);

  return (
    <>
      {accounts !== null ? (
        <StaffAccountTable
          accounts={accounts}
          now={new Date()}
          onReset={(a) => {
            setDone(null);
            setResetting(a);
          }}
        />
      ) : loadError !== null ? (
        <ErrorState role="alert" title="Chưa tải được tài khoản cán bộ" message={loadError} />
      ) : (
        <div>
          <p role="status" className="sr-only">
            Đang tải tài khoản cán bộ…
          </p>
          <SkeletonRows rows={3} />
        </div>
      )}

      {done ? (
        <p role="status" className="m-0 text-sm font-medium text-success-600">
          {done}
        </p>
      ) : null}

      <CreateAdminControl
        active={commune.active}
        onOpen={() => {
          setDone(null);
          setCreating(true);
        }}
      />

      {creating ? (
        <CreateCommuneAdminDialog
          commune={commune}
          onClose={() => setCreating(false)}
          onDone={(a) => {
            setAccounts((list) => (list ? [...list, a] : [a]));
            setDone(`Đã tạo tài khoản quản trị ${a.staff_code}. ${COMMUNE_ADMIN_NOTE}`);
            setCreating(false);
          }}
        />
      ) : null}
      {resetting ? (
        <ResetStaffPasswordDialog
          key={resetting.user_id}
          commune={commune}
          account={resetting}
          onClose={() => setResetting(null)}
          onDone={(a) => {
            setAccounts((list) => (list ? list.map((x) => (x.user_id === a.user_id ? a : x)) : list));
            setDone(`Đã đặt lại mật khẩu tạm cho ${a.staff_code}. ${STAFF_RESET_NOTE}`);
            setResetting(null);
          }}
        />
      ) : null}
    </>
  );
}

/** The create button, disabled with its reason when the commune is inactive (the server refuses it too). */
export function CreateAdminControl({ active, onOpen }: { active: boolean; onOpen: () => void }) {
  return (
    <div className="flex flex-col items-start gap-1.5 border-t border-line pt-4">
      <Button
        type="button"
        variant="secondary"
        disabled={!active}
        icon={<UserPlus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
        onClick={onOpen}
      >
        {CREATE_ADMIN_LABEL}
      </Button>
      {!active ? <p className={hintClass}>{CREATE_ADMIN_INACTIVE}</p> : null}
    </div>
  );
}

function CreateCommuneAdminDialog({
  commune,
  onClose,
  onDone,
}: {
  commune: CommuneDetail;
  onClose: () => void;
  onDone: (a: StaffAccount) => void;
}) {
  const toError = useOwnerError(ownerAccountError);
  const [email, setEmail] = useState("");
  const [fullName, setFullName] = useState("");
  const [password, setPassword] = useState("");
  const [confirm, setConfirm] = useState("");
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<OwnerError | null>(null);

  async function submit(event: FormEvent<HTMLFormElement>) {
    event.preventDefault();
    if (busy) return;
    const local = checkCreateCommuneAdmin({ email, fullName, password, confirm });
    if (local) return setError(local);
    setBusy(true);
    setError(null);
    try {
      onDone(
        await createCommuneAdminAccount(commune.id, {
          email: email.trim(),
          fullName: fullName.trim(),
          temporaryPassword: password,
        }),
      );
    } catch (err) {
      setError(toError(err, "tạo tài khoản quản trị của xã"));
    } finally {
      setPassword("");
      setConfirm("");
      setBusy(false);
    }
  }

  return (
    <Dialog open title={`${CREATE_ADMIN_LABEL} cho ${commune.name}`} icon={UserPlus} onClose={() => !busy && onClose()}>
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <Notice tone="info">
          <p>{COMMUNE_ADMIN_NOTE}</p>
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
          label="Họ tên"
          name="full_name"
          type="text"
          autoComplete="off"
          maxLength={200}
          value={fullName}
          onChange={(e) => setFullName(e.target.value)}
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
        <FormMessage text={formLevel(error, "email", "name", "password", "confirm")} />
        <DialogActions>
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            {busy ? "Đang thêm…" : CREATE_ADMIN_LABEL}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}

function ResetStaffPasswordDialog({
  commune,
  account,
  onClose,
  onDone,
}: {
  commune: CommuneDetail;
  account: StaffAccount;
  onClose: () => void;
  onDone: (a: StaffAccount) => void;
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
      onDone(await resetStaffPassword(commune.id, account.user_id, { temporaryPassword: password, reason: reason.trim() }));
    } catch (err) {
      setError(toError(err, "đặt lại mật khẩu tài khoản cán bộ"));
    } finally {
      setPassword("");
      setConfirm("");
      setBusy(false);
    }
  }

  return (
    <Dialog
      open
      title={`Đặt lại mật khẩu cho ${account.staff_code} (${account.full_name}) của ${commune.name}?`}
      icon={KeyRound}
      onClose={() => !busy && onClose()}
    >
      <form className="flex flex-col gap-4" method="post" onSubmit={submit} noValidate>
        <Notice tone="legal" title="Lưu ý:">
          <p>{STAFF_RESET_NOTE}</p>
          <p>Việc đặt lại được ghi vào nhật ký của chính xã, kèm mã người vận hành của bạn.</p>
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
          <Button type="button" variant="secondary" onClick={onClose} disabled={busy}>
            Huỷ
          </Button>
          <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
            {busy ? "Đang lưu…" : "Đặt lại mật khẩu"}
          </Button>
        </DialogActions>
      </form>
    </Dialog>
  );
}
