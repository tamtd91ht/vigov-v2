"use client";

import { ChevronDown, ChevronRight, Loader2, LockKeyhole, Pencil, Plus, RefreshCw, Star, Ban } from "lucide-react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { NUT_HUY } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { ConfirmDialog } from "@/components/ui/confirm-dialog";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { NoAccess } from "@/components/ui/no-access";
import { Notice } from "@/components/ui/notice";
import { Skeleton } from "@/components/ui/skeleton";
import { formatVietnamDateTime } from "@/features/noi-dung/nhan-noi-dung";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { cn } from "@/lib/cn";
// vi-name-ok: the existing result type and sentence of lib/api/goi.ts, imported, not a new name
import { LOI_KHONG_RO, type KetQua } from "@/lib/api/goi";
import {
  checkSmsAccount,
  createSmsAccount,
  listSmsAccounts,
  listSmsTemplates,
  retireSmsAccount,
  SMS_ERROR,
  syncSmsTemplates,
  updateSmsAccount,
  type comms_smsAccountOut,
  type comms_smsTemplateOut,
} from "@/lib/api/sms-accounts";

import { ConfigDialog } from "./config-dialog";
import { ConfigTable } from "./config-ui";
import { smsChannelTabDecision } from "./quyen-tab";
import {
  ADD_BUTTON,
  buildCreate,
  buildUpdate,
  CHECK_FAILED_TOAST,
  checkRetireReason,
  checkToastText,
  DEFAULT_BADGE,
  DEFAULT_SET_TOAST,
  draftFromAccount,
  emptyDraft,
  isCheckOk,
  KEY_KEPT_HINT,
  KEY_NEW_HINT,
  KEY_RETYPE_HINT,
  lastCheckText,
  latestSyncedAt,
  networkLabel,
  NO_ACCOUNTS,
  NO_TEMPLATES,
  paramText,
  refusalField,
  RETIRE_CONSEQUENCE,
  RETIRE_REASON_MAX,
  RETIRED_TOAST,
  SAVE_FAILED_TOAST,
  SAVED_TOAST,
  setByText,
  SMS_DESCRIPTION,
  SYNC_BUTTON,
  syncResultText,
  TEMPLATES_DESCRIPTION,
  TEMPLATES_TITLE,
  type SmsAccountDraft,
  type SmsFieldErrors,
} from "./sms-channel-form";

/**
 * "Cấu hình → Kênh SMS" (ADR 0099 #2 #3): the commune's eSMS accounts. One key, `admin.sms`, for read and
 * write — comms declares it on all seven routes, so the tab hides as a whole without it (convenience;
 * comms refuses on every request, rule 5 forbidden #1).
 *
 * NO PROTOTYPE: the cards follow "Con bot của xã" / "OA Zalo của xã" (who set the keys and when, last
 * check, a confirmation with a reason for the destructive act), the dialog follows "Máy chủ thư"
 * (write-only secret). ADR 0068 lần 6.
 *
 * SECRETS: ApiKey / SecretKey exist ONLY in the open dialog's draft, typed by staff. They are cleared from
 * state the moment the request is sent — success or refusal — so they never outlive one send in memory,
 * and no reply carries them back (comms has no field for them). No `name` attribute: nothing submits
 * them natively and the browser has no key to remember them under.
 */
export function SmsChannelTab() {
  const phien = usePhien();
  const decision = phien === null ? null : smsChannelTabDecision(phien);
  const allowed = decision !== null && decision.hien;

  const [accounts, setAccounts] = useState<KetQua<comms_smsAccountOut[]> | null>(null);
  /** comms' `encryption_not_configured` sentence, once any call answered it: every "Lưu" stays disabled. */
  const [encryptionMissing, setEncryptionMissing] = useState<string | null>(null);
  const [dialog, setDialog] = useState<{ mode: "add" } | { mode: "edit"; account: comms_smsAccountOut } | null>(null);
  const [retiring, setRetiring] = useState<comms_smsAccountOut | null>(null);
  /** One account action in flight at a time: two overlapping writes on "mặc định" would race. */
  const [busy, setBusy] = useState<{ id: string; kind: "check" | "default" } | null>(null);

  const reload = useCallback(async () => {
    setAccounts(await listSmsAccounts());
  }, []);

  useEffect(() => {
    if (!allowed) return;
    let gone = false;
    listSmsAccounts().then((r) => {
      if (!gone) setAccounts(r);
    });
    return () => {
      gone = true;
    };
  }, [allowed]);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p className="text-danger m-0 text-[12.5px] font-medium" role="alert">
        {decision.thongBao}
      </p>
    ) : (
      <div className="flex min-w-0 flex-col items-center pb-10">
        <NoAccess className="pb-4" />
        <p className="text-ink-muted m-0 max-w-md px-4 text-center text-[13px]">
          Tài khoản của bạn không có quyền cấu hình kênh SMS, nên tab này không hiển thị.
        </p>
      </div>
    );
  }
  if (accounts === null) {
    return (
      <>
        <p role="status" className="an-thi-giac">
          Đang tải tài khoản SMS của xã…
        </p>
        <Skeleton className="h-48 w-full" />
      </>
    );
  }
  if (!accounts.ok) return <ErrorState role="alert" title="Chưa tải được tài khoản SMS của xã" message={accounts.thongBao} />;
  const list = accounts.duLieu;

  /** The shared handling of a refused account action: encryption, a vanished account, a concurrent change. */
  async function refused(r: { thongBao: string; code?: string }, fallback: string) {
    if (r.code === SMS_ERROR.encryptionMissing) setEncryptionMissing(r.thongBao);
    toast.error(r.thongBao === LOI_KHONG_RO ? fallback : r.thongBao);
    if (r.code === SMS_ERROR.notFound || r.code === SMS_ERROR.changed) await reload();
  }

  async function check(a: comms_smsAccountOut) {
    if (busy !== null) return;
    setBusy({ id: a.id, kind: "check" });
    const r = await checkSmsAccount(a.id);
    setBusy(null);
    if (!r.ok) {
      await refused(r, CHECK_FAILED_TOAST);
      return;
    }
    const text = checkToastText(r.duLieu.result, r.duLieu.balance);
    if (isCheckOk(r.duLieu.result)) toast.success(text);
    else toast.error(text);
    await reload();
  }

  async function makeDefault(a: comms_smsAccountOut) {
    if (busy !== null) return;
    setBusy({ id: a.id, kind: "default" });
    const r = await updateSmsAccount(a.id, { is_default: true });
    setBusy(null);
    if (!r.ok) {
      await refused(r, SAVE_FAILED_TOAST);
      return;
    }
    toast.success(DEFAULT_SET_TOAST);
    await reload();
  }

  const addButton = (
    <Button
      type="button"
      variant="primary"
      icon={<Plus aria-hidden="true" focusable="false" />}
      onClick={() => setDialog({ mode: "add" })}
    >
      {ADD_BUTTON}
    </Button>
  );

  return (
    <div className="flex min-w-0 flex-col gap-4" data-sms-channel="">
      <section className={SECTION} aria-labelledby="sms-channel-title">
        <div className="flex flex-wrap items-start gap-3">
          <div className="min-w-0 flex-1">
            <h3 id="sms-channel-title" className={TITLE}>
              Tài khoản eSMS của xã
            </h3>
            <p className={DESCRIPTION}>{SMS_DESCRIPTION}</p>
          </div>
          {list.length > 0 && addButton}
        </div>
        {encryptionMissing !== null && (
          <Notice role="alert" icon={LockKeyhole} className="mt-3" data-encryption-missing="">
            {encryptionMissing}
          </Notice>
        )}
        {list.length === 0 && (
          <div className="mt-4 flex flex-col items-start gap-3" data-sms-empty="">
            <p className="text-ink-muted m-0 text-[12.5px]">{NO_ACCOUNTS}</p>
            {addButton}
          </div>
        )}
      </section>

      {list.map((a) => (
        <SmsAccountCard
          key={a.id}
          account={a}
          busy={busy !== null && busy.id === a.id ? busy.kind : null}
          locked={busy !== null}
          onCheck={() => void check(a)}
          onEdit={() => setDialog({ mode: "edit", account: a })}
          onMakeDefault={() => void makeDefault(a)}
          onRetire={() => setRetiring(a)}
          onEncryptionMissing={setEncryptionMissing}
        />
      ))}

      {dialog !== null && (
        <SmsAccountDialog
          account={dialog.mode === "edit" ? dialog.account : null}
          firstAccount={list.length === 0}
          encryptionMissing={encryptionMissing}
          onEncryptionMissing={setEncryptionMissing}
          onClose={(saved) => {
            setDialog(null);
            if (saved) void reload();
          }}
        />
      )}

      {retiring !== null && (
        <RetireDialog
          account={retiring}
          onClose={(done) => {
            setRetiring(null);
            if (done) void reload();
          }}
        />
      )}
    </div>
  );
}

// Same card as the sections of Kênh Zalo (`zalo-channel-tab.tsx` SECTION / TITLE / DESCRIPTION).
const SECTION = "border-line shadow-card rounded-card m-0 min-w-0 border border-solid bg-white p-5";
const TITLE = "text-navy m-0 text-[14px] font-bold";
const DESCRIPTION = "text-ink-muted m-0 mt-1 max-w-2xl text-[12.5px]";
const FIELD_LABEL = "text-navy m-0 block text-[13px] leading-5 font-semibold";
const HINT = "text-ink-muted m-0 text-[12px]";
const ERROR = "text-danger m-0 text-[12px] font-medium";
const DANGER_OUTLINE = "border-danger/30 text-danger hover:not-disabled:bg-danger/5 bg-white";
const inputCls = cn(controlClass, "bg-white");

function spin(on: boolean, icon: ReactNode): ReactNode {
  return on ? <Loader2 aria-hidden="true" focusable="false" className="animate-spin" /> : icon;
}

function SmsAccountCard({
  account: a,
  busy,
  locked,
  onCheck,
  onEdit,
  onMakeDefault,
  onRetire,
  onEncryptionMissing,
}: {
  account: comms_smsAccountOut;
  busy: "check" | "default" | null;
  /** Another account action is in flight. */
  locked: boolean;
  onCheck: () => void;
  onEdit: () => void;
  onMakeDefault: () => void;
  onRetire: () => void;
  onEncryptionMissing: (sentence: string) => void;
}) {
  const titleId = `sms-account-${a.id}-title`;
  return (
    <section className={SECTION} aria-labelledby={titleId} data-sms-account={a.id}>
      <div className="flex flex-wrap items-start gap-3">
        <div className="min-w-0 flex-1">
          <h3 id={titleId} className={cn(TITLE, "break-words")}>
            {a.label}
          </h3>
          <p className="text-ink m-0 mt-1 text-[12.5px]">
            Brandname: <span className="font-semibold" data-brandname="">{a.brandname}</span>
          </p>
        </div>
        {a.is_default && (
          <Badge tone="info" icon={Star} data-default-badge="">
            {DEFAULT_BADGE}
          </Badge>
        )}
      </div>

      <div className="mt-3 flex flex-col gap-1">
        <p className={HINT} data-set-by="">
          {setByText(formatVietnamDateTime(a.set_at), a.set_by)}
        </p>
        {a.last_check !== null && (
          <p
            className={cn("m-0 text-[12px]", isCheckOk(a.last_check.result) ? "text-leaf" : "text-danger")}
            data-last-check=""
          >
            {lastCheckText(formatVietnamDateTime(a.last_check.at), a.last_check.result, a.last_check.balance)}
          </p>
        )}
      </div>

      <div className="mt-4 flex flex-wrap items-center gap-2">
        <Button
          type="button"
          variant="outline"
          className="bg-white"
          disabled={locked}
          aria-busy={busy === "check"}
          icon={spin(busy === "check", <RefreshCw aria-hidden="true" focusable="false" />)}
          onClick={onCheck}
        >
          Kiểm tra kết nối
        </Button>
        <Button
          type="button"
          variant="outline"
          className="bg-white"
          disabled={locked}
          icon={<Pencil aria-hidden="true" focusable="false" />}
          onClick={onEdit}
        >
          Sửa
        </Button>
        {!a.is_default && (
          <Button
            type="button"
            variant="outline"
            className="bg-white"
            disabled={locked}
            aria-busy={busy === "default"}
            icon={spin(busy === "default", <Star aria-hidden="true" focusable="false" />)}
            onClick={onMakeDefault}
          >
            Đặt mặc định
          </Button>
        )}
        <Button
          type="button"
          variant="outline"
          className={DANGER_OUTLINE}
          disabled={locked}
          icon={<Ban aria-hidden="true" focusable="false" />}
          onClick={onRetire}
        >
          Ngừng dùng
        </Button>
      </div>

      <SmsTemplatesPanel accountId={a.id} label={a.label} onEncryptionMissing={onEncryptionMissing} />
    </section>
  );
}

/**
 * "Mẫu tin SMS" of one account — collapsed by default and read only when opened: a commune with several
 * accounts would otherwise send one templates read per card on every visit to the tab.
 */
function SmsTemplatesPanel({
  accountId,
  label,
  onEncryptionMissing,
}: {
  accountId: string;
  label: string;
  onEncryptionMissing: (sentence: string) => void;
}) {
  const [open, setOpen] = useState(false);
  const [items, setItems] = useState<KetQua<comms_smsTemplateOut[]> | null>(null);
  const [syncing, setSyncing] = useState(false);
  const panelId = `sms-account-${accountId}-templates`;

  useEffect(() => {
    if (!open || items !== null) return;
    let gone = false;
    listSmsTemplates(accountId).then((r) => {
      if (!gone) setItems(r);
    });
    return () => {
      gone = true;
    };
  }, [open, items, accountId]);

  async function sync() {
    if (syncing) return;
    setSyncing(true);
    const r = await syncSmsTemplates(accountId);
    setSyncing(false);
    if (!r.ok) {
      if (r.code === SMS_ERROR.encryptionMissing) onEncryptionMissing(r.thongBao);
      toast.error(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    toast.success(syncResultText(r.duLieu));
    setItems({ ok: true, duLieu: r.duLieu.items });
  }

  const synced = items !== null && items.ok ? latestSyncedAt(items.duLieu) : null;

  return (
    <div className="border-line mt-4 border-0 border-t border-solid pt-3">
      <button
        type="button"
        className="text-navy m-0 flex cursor-pointer items-center gap-1.5 border-0 bg-transparent p-0 text-[13px] font-semibold"
        aria-expanded={open}
        aria-controls={panelId}
        onClick={() => setOpen(!open)}
      >
        {open ? (
          <ChevronDown aria-hidden="true" focusable="false" className="size-4" />
        ) : (
          <ChevronRight aria-hidden="true" focusable="false" className="size-4" />
        )}
        {TEMPLATES_TITLE}
      </button>
      {open && (
        <div id={panelId} className="mt-3 flex min-w-0 flex-col gap-3" data-sms-templates="">
          <div className="flex flex-wrap items-start gap-3">
            <p className={cn(DESCRIPTION, "mt-0 min-w-0 flex-1")}>{TEMPLATES_DESCRIPTION}</p>
            <div className="flex flex-col items-end gap-1">
              <Button
                type="button"
                variant="outline"
                className="bg-white"
                disabled={syncing}
                aria-busy={syncing}
                icon={spin(syncing, <RefreshCw aria-hidden="true" focusable="false" />)}
                onClick={() => void sync()}
              >
                {SYNC_BUTTON}
              </Button>
              {synced !== null && (
                <p className={HINT} data-synced-at="">
                  {`Nạp lúc ${formatVietnamDateTime(synced)}`}
                </p>
              )}
            </div>
          </div>
          {items === null ? (
            <>
              <p role="status" className="an-thi-giac">
                Đang tải mẫu tin SMS…
              </p>
              <Skeleton className="h-16 w-full" />
            </>
          ) : !items.ok ? (
            <p className={ERROR} role="alert">
              {items.thongBao}
            </p>
          ) : items.duLieu.length === 0 ? (
            <p className="text-ink-muted m-0 text-[12.5px]" data-templates-empty="">
              {NO_TEMPLATES}
            </p>
          ) : (
            <ConfigTable label={`Mẫu tin SMS của tài khoản ${label}`} caption="Nội dung mẫu, tham số và nhà mạng đã đăng ký">
              <thead>
                <tr>
                  <th scope="col">Nội dung mẫu</th>
                  <th scope="col">Tham số</th>
                  <th scope="col">Nhà mạng</th>
                </tr>
              </thead>
              <tbody>
                {items.duLieu.map((t, i) => (
                  <tr key={`${i}-${t.content}`} data-sms-template="">
                    <td className="!whitespace-normal min-w-56 break-words">{t.content}</td>
                    <td data-params="">{t.params.length > 0 ? t.params.map(paramText).join(", ") : "—"}</td>
                    <td className="!whitespace-normal" data-networks="">
                      <span className="flex flex-col gap-0.5">
                        {t.networks.map((n) => (
                          <span key={`${n.network_id}-${n.template_id}`}>
                            {networkLabel(n.network_id)}
                            <span className="text-ink-muted font-mono text-[11.5px]">{` · ${n.template_id}`}</span>
                          </span>
                        ))}
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </ConfigTable>
          )}
        </div>
      )}
    </div>
  );
}

/**
 * "Thêm tài khoản" / "Sửa". The Idempotency-Key of the POST is made ONCE, when the dialog opens, and kept
 * across a retry inside it: comms releases a refused key (`core/idem`), so the corrected retry may use it,
 * while a double click never adds two accounts.
 */
function SmsAccountDialog({
  account,
  firstAccount,
  encryptionMissing,
  onEncryptionMissing,
  onClose,
}: {
  /** null = add. */
  account: comms_smsAccountOut | null;
  firstAccount: boolean;
  encryptionMissing: string | null;
  onEncryptionMissing: (sentence: string) => void;
  onClose: (saved: boolean) => void;
}) {
  const editing = account !== null;
  const [draft, setDraft] = useState<SmsAccountDraft>(() =>
    account !== null ? draftFromAccount(account) : emptyDraft(firstAccount),
  );
  const [idempotencyKey] = useState(() => crypto.randomUUID());
  const [busy, setBusy] = useState(false);
  const [errors, setErrors] = useState<SmsFieldErrors>({});
  /** A send happened and the keys were cleared: say why the boxes are empty again. */
  const [keysCleared, setKeysCleared] = useState(false);
  const idPrefix = editing ? `sms-account-${account.id}` : "sms-account-new";

  async function submit() {
    if (busy || encryptionMissing !== null) return;
    let request: Promise<KetQua<comms_smsAccountOut>>;
    if (account === null) {
      const built = buildCreate(draft);
      if (!built.ok) {
        setErrors(built.errors);
        return;
      }
      request = createSmsAccount(built.body, idempotencyKey);
    } else {
      const built = buildUpdate(account, draft);
      if (!built.ok) {
        setErrors(built.errors);
        return;
      }
      if (built.empty) {
        onClose(false);
        return;
      }
      request = updateSmsAccount(account.id, built.body);
    }
    const sentKeys = draft.apiKey !== "" || draft.secretKey !== "";
    // The keys leave state now: the request already holds its own copy of the body.
    setDraft((d) => ({ ...d, apiKey: "", secretKey: "" }));
    setErrors({});
    setBusy(true);
    const r = await request;
    setBusy(false);
    if (!r.ok) {
      if (sentKeys) setKeysCleared(true);
      if (r.code === SMS_ERROR.encryptionMissing) onEncryptionMissing(r.thongBao);
      if (r.code === SMS_ERROR.notFound) {
        toast.error(r.thongBao);
        onClose(true);
        return;
      }
      const text = r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao;
      setErrors({ [refusalField(r.code)]: text });
      return;
    }
    toast.success(SAVED_TOAST);
    onClose(true);
  }

  const keyHint = editing ? KEY_KEPT_HINT : KEY_NEW_HINT;
  const saveDisabled = busy || encryptionMissing !== null;

  return (
    <ConfigDialog
      title={editing ? "Sửa tài khoản eSMS" : "Thêm tài khoản eSMS"}
      description={editing ? account.label : undefined}
      onDismiss={() => {
        if (!busy) onClose(false);
      }}
    >
      <form
        aria-label={editing ? "Sửa tài khoản eSMS" : "Thêm tài khoản eSMS"}
        className="m-0 flex flex-col gap-4"
        autoComplete="off"
        onSubmit={(e) => {
          e.preventDefault();
          void submit();
        }}
      >
        {encryptionMissing !== null && (
          <Notice role="alert" icon={LockKeyhole} data-encryption-missing="">
            {encryptionMissing}
          </Notice>
        )}
        <fieldset disabled={busy} className="m-0 flex min-w-0 flex-col gap-4 border-0 p-0">
          <Field id={`${idPrefix}-label`} label="Tên gợi nhớ" error={errors.label}>
            <input
              id={`${idPrefix}-label`}
              type="text"
              className={inputCls}
              maxLength={100}
              value={draft.label}
              aria-invalid={errors.label !== undefined}
              onChange={(e) => setDraft({ ...draft, label: e.target.value })}
            />
          </Field>
          <div className="grid gap-4 sm:grid-cols-2">
            {/* WRITE-ONLY: never filled from the server; no `name`; `autoComplete="off"`. */}
            <Field id={`${idPrefix}-api-key`} label="ApiKey" error={errors.keys}>
              <input
                id={`${idPrefix}-api-key`}
                type="password"
                autoComplete="off"
                spellCheck={false}
                className={inputCls}
                placeholder={editing ? "••••••••" : undefined}
                value={draft.apiKey}
                aria-invalid={errors.keys !== undefined}
                aria-describedby={`${idPrefix}-key-hint`}
                onChange={(e) => setDraft({ ...draft, apiKey: e.target.value })}
              />
            </Field>
            <Field id={`${idPrefix}-secret-key`} label="SecretKey">
              <input
                id={`${idPrefix}-secret-key`}
                type="password"
                autoComplete="off"
                spellCheck={false}
                className={inputCls}
                placeholder={editing ? "••••••••" : undefined}
                value={draft.secretKey}
                aria-invalid={errors.keys !== undefined}
                aria-describedby={`${idPrefix}-key-hint`}
                onChange={(e) => setDraft({ ...draft, secretKey: e.target.value })}
              />
            </Field>
          </div>
          <p id={`${idPrefix}-key-hint`} className={cn(HINT, "-mt-2")} data-key-hint="">
            {keysCleared ? `${keyHint} ${KEY_RETYPE_HINT}` : keyHint}
          </p>
          <Field id={`${idPrefix}-brandname`} label="Brandname" error={errors.brandname}>
            <input
              id={`${idPrefix}-brandname`}
              type="text"
              spellCheck={false}
              className={inputCls}
              maxLength={32}
              value={draft.brandname}
              aria-invalid={errors.brandname !== undefined}
              onChange={(e) => setDraft({ ...draft, brandname: e.target.value })}
            />
          </Field>
          <label className="text-ink m-0 flex cursor-pointer items-center gap-2 text-[13px] font-normal">
            <input
              type="checkbox"
              className="accent-brand m-0 size-4 shrink-0"
              checked={draft.isDefault}
              onChange={(e) => setDraft({ ...draft, isDefault: e.target.checked })}
            />
            Đặt làm mặc định
          </label>
        </fieldset>
        {errors.form !== undefined && (
          <p className={ERROR} role="alert" data-form-error="">
            {errors.form}
          </p>
        )}
        <div className="flex flex-wrap justify-end gap-2">
          <Button type="submit" variant="primary" disabled={saveDisabled} aria-busy={busy}>
            Lưu
          </Button>
          <Button type="button" variant="outline" disabled={busy} onClick={() => onClose(false)}>
            {NUT_HUY}
          </Button>
        </div>
      </form>
    </ConfigDialog>
  );
}

function Field({ id, label, error, children }: { id: string; label: string; error?: string; children: ReactNode }) {
  return (
    <div className="flex min-w-0 flex-col gap-1.5">
      <label htmlFor={id} className={FIELD_LABEL}>
        {label}
      </label>
      {children}
      {error !== undefined && (
        <p className={ERROR} role="alert" data-field-error="">
          {error}
        </p>
      )}
    </div>
  );
}

/** "Ngừng dùng": a required reason, then DELETE — a soft delete that keeps the account as history (rule 7). */
function RetireDialog({ account, onClose }: { account: comms_smsAccountOut; onClose: (done: boolean) => void }) {
  const [reason, setReason] = useState("");
  const [error, setError] = useState<string | null>(null);
  const [busy, setBusy] = useState(false);

  async function retire() {
    const checked = checkRetireReason(reason);
    if (!checked.ok) {
      setError(checked.text);
      return;
    }
    setError(null);
    setBusy(true);
    const r = await retireSmsAccount(account.id, checked.reason);
    setBusy(false);
    if (!r.ok) {
      if (r.code === SMS_ERROR.notFound) {
        toast.error(r.thongBao);
        onClose(true);
        return;
      }
      // Kept in the box: the reason typed stays, the server's sentence next to it.
      setError(r.thongBao === LOI_KHONG_RO ? SAVE_FAILED_TOAST : r.thongBao);
      return;
    }
    toast.success(RETIRED_TOAST);
    onClose(true);
  }

  return (
    <ConfigDialog
      title="Ngừng dùng tài khoản SMS?"
      hideHeader
      onDismiss={() => {
        if (!busy) onClose(false);
      }}
    >
      <ConfirmDialog
        as="form"
        aria-label="Ngừng dùng tài khoản SMS"
        icon={Ban}
        tone="danger"
        title={`Ngừng dùng tài khoản “${account.label}”?`}
        className="m-0 p-0 shadow-none"
        onSubmit={(e) => {
          e.preventDefault();
          if (!busy) void retire();
        }}
        actions={
          <>
            <Button type="submit" variant="primary" disabled={busy} aria-busy={busy}>
              Ngừng dùng
            </Button>
            <Button type="button" variant="outline" disabled={busy} onClick={() => onClose(false)}>
              {NUT_HUY}
            </Button>
          </>
        }
      >
        <p className="m-0" data-retire-consequence="">
          {RETIRE_CONSEQUENCE}
        </p>
        <div className="mt-3 flex flex-col gap-1.5">
          <label htmlFor={`sms-account-${account.id}-retire-reason`} className={FIELD_LABEL}>
            Lý do ngừng dùng
          </label>
          {/* One line: comms refuses control characters, a newline included. */}
          <input
            id={`sms-account-${account.id}-retire-reason`}
            type="text"
            className={inputCls}
            maxLength={RETIRE_REASON_MAX}
            value={reason}
            disabled={busy}
            aria-invalid={error !== null}
            onChange={(e) => setReason(e.target.value)}
          />
          {error !== null && (
            <p className={ERROR} role="alert">
              {error}
            </p>
          )}
        </div>
      </ConfirmDialog>
    </ConfigDialog>
  );
}
