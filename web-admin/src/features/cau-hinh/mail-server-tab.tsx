"use client";

import { CheckCircle2, LockKeyhole, Mail, Save, TriangleAlert } from "lucide-react";
import { useEffect, useState } from "react";
import { toast } from "sonner";

import { useCauHinhXa } from "@/components/cau-hinh-xa";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { NoAccess } from "@/components/ui/no-access";
import { Notice } from "@/components/ui/notice";
import { PendingMarker } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";

import { usePhien } from "@/features/phien/phien-hien-tai";
import { cn } from "@/lib/cn";
import { LOI_KHONG_RO } from "@/lib/api/goi";
import type { KetQua } from "@/lib/api/goi";
import { TEST_UNREACHABLE_FALLBACK, getMailSettings, saveMailSettings, sendTestMail } from "@/lib/api/mail-settings";
import type { comms_mailSettingsOut } from "@/lib/api/schema.gen";

import { ConfigField, formInputCls } from "./config-ui";
import {
  CALL_FAILED,
  ENABLE_LABEL,
  ENCRYPTION_MISSING,
  FROM_ADDRESS_PLACEHOLDER,
  HOST_PLACEHOLDER,
  MAIL_DESCRIPTION,
  MAIL_SECURITY,
  MAIL_TITLE,
  NOT_CONFIGURED_WARNING,
  PASSWORD_KEPT_PLACEHOLDER,
  PORT_HINT,
  SAVED_SENTENCE,
  SAVE_BUTTON,
  TEST_BUTTON,
  TEST_LABEL,
  TEST_RECIPIENT_PLACEHOLDER,
  TEST_SENT_TOAST,
  draftFromSettings,
  saveBody,
  saveRefusalMessage,
  testFailedToast,
  testKey,
  testSentSentence,
} from "./mail-settings-form";
import type { MailDraft } from "./mail-settings-form";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";
import { mailServerTabDecision } from "./quyen-tab";

/**
 * The part of §10 the server does not store yet (ADR 0079 #5): drawn as "?" at the spec's place, its
 * sentence taken from `PHAN_CHUA_DUNG` as-is — never a second copy here.
 */
const LAST_TEST = PHAN_CHUA_DUNG.find((p) => p.ten === "Lần thử gần nhất");

/** The save form's id: "Lưu cấu hình" sits in the action row beside the test form, outside it. */
const SAVE_FORM_ID = "form-may-chu-thu";

/**
 * "Cấu hình → Máy chủ thư" (spec 10). One key, `admin.lookup`, for read and write — the server declares
 * it on all three routes, so the tab hides as a whole without it (convenience; the server refuses).
 *
 * AFTER A SAVE THE FORM IS REBUILT FROM THE SERVER'S ANSWER, password box emptied: the answer is the
 * state, and a password left in the box after saving is one more place it sits in memory for nothing.
 */
export function MailServerTab() {
  const phien = usePhien();
  // The sender-name placeholder is the signed-in commune (ADR 0068 §13), read from the
  // server-built context like every other commune value — never a product name.
  const commune = useCauHinhXa();
  const decision = phien === null ? null : mailServerTabDecision(phien);
  const allowed = decision !== null && decision.hien;

  const [loaded, setLoaded] = useState<KetQua<comms_mailSettingsOut> | null>(null);
  const [draft, setDraft] = useState<MailDraft | null>(null);
  const [saving, setSaving] = useState(false);
  const [saveError, setSaveError] = useState<string | null>(null);

  const [recipient, setRecipient] = useState("");
  const [attempt, setAttempt] = useState<{ key: string; recipient: string } | null>(null);
  const [testing, setTesting] = useState(false);
  const [testResult, setTestResult] = useState<{ ok: boolean; text: string } | null>(null);

  useEffect(() => {
    if (!allowed) return;
    let gone = false;
    getMailSettings().then((r) => {
      if (gone) return;
      setLoaded(r);
      if (r.ok) setDraft(draftFromSettings(r.duLieu));
    });
    return () => {
      gone = true;
    };
  }, [allowed]);

  if (phien === null) return <p role="status">Đang kiểm tra quyền truy cập…</p>;
  if (decision !== null && !decision.hien) {
    return decision.vi === "khong-doc-duoc" ? (
      <p className="thong-bao-loi" role="alert">
        {decision.thongBao}
      </p>
    ) : (
      // Shared `NoAccess` (spec v2 §8b) + this tab's own sentence, verbatim, as its caption.
      <div className="khung-thieu-quyen flex min-w-0 flex-col items-center pb-10 [&>.trang-thai-rong]:m-0 [&>.trang-thai-rong]:max-w-md [&>.trang-thai-rong]:border-0 [&>.trang-thai-rong]:bg-transparent [&>.trang-thai-rong]:px-4 [&>.trang-thai-rong]:py-0 [&>.trang-thai-rong]:text-center [&>.trang-thai-rong]:text-[13px] [&>.trang-thai-rong]:text-ink-500">
        <NoAccess className="pb-4" />
        <p className="trang-thai-rong">
          Tài khoản của bạn không có quyền cấu hình máy chủ thư, nên tab này không hiển thị.
        </p>
      </div>
    );
  }
  if (loaded === null) return <MailServerLoading />;
  if (!loaded.ok) {
    return <ErrorState role="alert" title="Chưa tải được cấu hình máy chủ thư" message={loaded.thongBao} />;
  }
  if (draft === null) return null;
  const saved = loaded.duLieu;

  async function save() {
    if (draft === null || saving) return;
    const sent = draft;
    setSaving(true);
    setSaveError(null);
    const r = await saveMailSettings(saveBody(sent));
    setSaving(false);
    if (!r.ok) {
      // In place (ADR 0079: form errors stay at the form). The typed password stays in the box: the
      // refusal may be about another field.
      setSaveError(saveRefusalMessage(saved, sent, r.thongBao));
      return;
    }
    setLoaded(r);
    setDraft(draftFromSettings(r.duLieu));
    toast.success(SAVED_SENTENCE);
  }

  async function test() {
    const to = recipient.trim();
    if (to === "" || testing) return;
    const a = testKey(attempt, to, () => crypto.randomUUID());
    setAttempt(a);
    setTesting(true);
    setTestResult(null);
    const r = await sendTestMail(to, a.key);
    setTesting(false);
    if (!r.ok) {
      // `LOI_KHONG_RO` / `TEST_UNREACHABLE_FALLBACK` are what `sendTestMail` returns when service-comms
      // itself gave no answer — not a sentence from the commune's mail server.
      const unreachable = r.thongBao === LOI_KHONG_RO || r.thongBao === TEST_UNREACHABLE_FALLBACK;
      const text = unreachable ? CALL_FAILED : r.thongBao;
      toast.error(unreachable ? CALL_FAILED : testFailedToast(r.thongBao));
      setTestResult({ ok: false, text });
      return;
    }
    // Done: the next click is a new send, with a new key.
    setAttempt(null);
    toast.success(TEST_SENT_TOAST);
    setTestResult({ ok: true, text: testSentSentence(to) });
  }

  return (
    <MailServerView
      saved={saved}
      draft={draft}
      setDraft={setDraft}
      saving={saving}
      saveError={saveError}
      onSave={() => void save()}
      recipient={recipient}
      setRecipient={setRecipient}
      testing={testing}
      testResult={testResult}
      onTest={() => void test()}
      communeName={commune.displayName}
    />
  );
}

/** First load (spec 10): one `Skeleton h-80`; the sentence stays the live region. */
export function MailServerLoading() {
  return (
    <div className="max-w-3xl">
      <p role="status" className="an-thi-giac">
        Đang tải cấu hình máy chủ thư…
      </p>
      <Skeleton className="h-80 w-full rounded-[12px]" />
    </div>
  );
}

/** A tick box of the muted box (spec 10 #5): native checkbox, 16px like the prototype's shadcn Checkbox. */
function TickBox({
  name,
  checked,
  onChange,
  children,
}: {
  name: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
  children: string;
}) {
  return (
    <label className="text-ink m-0 flex cursor-pointer items-center gap-2 font-normal">
      <input
        type="checkbox"
        name={name}
        className="accent-brand m-0 size-4 shrink-0"
        checked={checked}
        onChange={(e) => onChange(e.target.checked)}
      />
      {children}
    </label>
  );
}

/**
 * Pure rendering, exported so the tests read the markup: the password input's `value` must be
 * empty whatever was saved, the encryption banner must disable the form, and the two security boxes
 * must never be both off.
 */
export function MailServerView({
  saved,
  draft,
  setDraft,
  saving,
  saveError,
  onSave,
  recipient,
  setRecipient,
  testing,
  testResult,
  onTest,
  communeName,
}: {
  saved: comms_mailSettingsOut;
  draft: MailDraft;
  setDraft: (d: MailDraft) => void;
  saving: boolean;
  /** The refused save's sentence, shown in place; null when none. */
  saveError: string | null;
  onSave: () => void;
  recipient: string;
  setRecipient: (s: string) => void;
  testing: boolean;
  /** This session's test send, if any. Not persisted by the server (see `LAST_TEST`). */
  testResult: { ok: boolean; text: string } | null;
  onTest: () => void;
  /** The signed-in commune's `displayName`, verbatim — the sender-name placeholder. */
  communeName: string;
}) {
  const readOnly = !saved.encryption_configured;
  // There is no other mail path (no platform fallback in service-comms), so mail goes nowhere unless
  // the commune's server is saved AND switched on. Read from the SAVED state: ticking the box without
  // saving changes nothing on the server.
  const noMailPath = !saved.configured || !saved.is_enabled;
  const testDisabled = readOnly || testing || !saved.configured;
  return (
    // Spec 10 (prototype `EmailSettingPanel.tsx:75`): one white card, max 3xl, NO shadow.
    <section className="border-line max-w-3xl rounded-[12px] border border-solid bg-white p-4" aria-labelledby="tieu-de-may-chu-thu">
      <h3 id="tieu-de-may-chu-thu" className="text-navy m-0 flex items-center gap-1.5 text-[13px] font-bold">
        <Mail aria-hidden="true" focusable="false" className="size-4 shrink-0" />
        {MAIL_TITLE}
      </h3>
      <p className="text-ink-muted m-0 mt-1 text-[12px]">{MAIL_DESCRIPTION}</p>

      {readOnly && (
        // ADR 0009 server state: without the platform key nothing can be saved or sent.
        <Notice role="alert" icon={LockKeyhole} className="mt-2">
          {ENCRYPTION_MISSING}
        </Notice>
      )}
      {/* ONLY the warning: the owner decided there is no platform mail fallback (ADR 0079 lô 2 Q1 #8,
          `platform_fallback` always false), and with fallback=false the prototype draws only this line
          (`EmailSettingPanel.tsx:85-95`) — no "Đang dùng máy chủ thư của nền tảng.", no "?" for it. */}
      {noMailPath && (
        <>
          <p className="text-tangerine m-0 mt-2 flex items-center gap-1.5 text-[11.5px]">
            <TriangleAlert aria-hidden="true" focusable="false" className="size-3.5 shrink-0" />
            {NOT_CONFIGURED_WARNING}
          </p>
        </>
      )}

      <form
        id={SAVE_FORM_ID}
        className="m-0"
        aria-label="Cấu hình máy chủ thư"
        onSubmit={(e) => {
          e.preventDefault();
          onSave();
        }}
      >
        {/* One `disabled` on the fieldset disables every control inside — the read-only state
            cannot forget a field. */}
        <fieldset disabled={readOnly || saving} className="m-0 min-w-0 border-0 p-0">
          <div className="mt-3 grid gap-3 sm:grid-cols-2">
            <ConfigField label="Máy chủ SMTP" htmlFor="o-smtp-host" className="sm:col-span-2">
              <input
                id="o-smtp-host"
                name="host"
                className={formInputCls}
                value={draft.host}
                placeholder={HOST_PLACEHOLDER}
                autoComplete="off"
                onChange={(e) => setDraft({ ...draft, host: e.target.value })}
              />
            </ConfigField>

            <ConfigField label="Cổng" htmlFor="o-smtp-port">
              {/* Free number (spec 10 #4); the server keeps the list of accepted ports and refuses
                  any other with its own sentence. Empty box = 0, which it refuses too. */}
              <input
                id="o-smtp-port"
                name="port"
                type="number"
                inputMode="numeric"
                min={1}
                max={65535}
                className={formInputCls}
                value={draft.port === 0 ? "" : String(draft.port)}
                onChange={(e) => setDraft({ ...draft, port: e.target.value === "" ? 0 : Number(e.target.value) })}
                aria-describedby="giai-thich-cong-smtp"
              />
              <p id="giai-thich-cong-smtp" className="text-ink-muted m-0 mt-1 text-[11px]">
                {PORT_HINT}
              </p>
            </ConfigField>

            <ConfigField label="Tài khoản" htmlFor="o-smtp-tai-khoan">
              <input
                id="o-smtp-tai-khoan"
                name="username"
                className={formInputCls}
                value={draft.username}
                autoComplete="off"
                onChange={(e) => setDraft({ ...draft, username: e.target.value })}
              />
            </ConfigField>

            <ConfigField label="Mật khẩu" htmlFor="o-smtp-mat-khau">
              {/* WRITE-ONLY: `value` is the draft's, which starts "" and is never filled from the
                  server. `new-password` keeps the browser from offering the staff member's own
                  login password here. */}
              <input
                id="o-smtp-mat-khau"
                name="password"
                type="password"
                autoComplete="new-password"
                className={formInputCls}
                value={draft.password}
                placeholder={saved.password_set ? PASSWORD_KEPT_PLACEHOLDER : undefined}
                onChange={(e) => setDraft({ ...draft, password: e.target.value })}
              />
            </ConfigField>

            <ConfigField label="Địa chỉ gửi" htmlFor="o-smtp-dia-chi-gui">
              <input
                id="o-smtp-dia-chi-gui"
                name="from_address"
                type="email"
                className={formInputCls}
                value={draft.fromAddress}
                placeholder={FROM_ADDRESS_PLACEHOLDER}
                onChange={(e) => setDraft({ ...draft, fromAddress: e.target.value })}
              />
            </ConfigField>

            <ConfigField label="Tên hiển thị của người gửi" htmlFor="o-smtp-ten-gui" className="sm:col-span-2">
              <input
                id="o-smtp-ten-gui"
                name="from_name"
                className={formInputCls}
                value={draft.fromName}
                placeholder={communeName}
                onChange={(e) => setDraft({ ...draft, fromName: e.target.value })}
              />
            </ConfigField>
          </div>

          {/* TOKEN TRAP: the spec's `bg-surface` is the page colour, `bg-background` in this app. */}
          {/* `flex-col gap-2`, not the spec's `space-y-2`: Tailwind v4 emits space-y under :where(), so the
              rows' own `m-0` (needed against legacy label margins) would win and pack them at 0 gap. */}
          <div className="border-line bg-background mt-3 flex flex-col gap-2 rounded-[10px] border border-solid p-3 text-[12.5px]">
            <TickBox name="is_enabled" checked={draft.isEnabled} onChange={(on) => setDraft({ ...draft, isEnabled: on })}>
              {ENABLE_LABEL}
            </TickBox>
            {/* ONE field, two boxes: ticking a box selects it; un-ticking the selected one is ignored,
                so exactly one is always on — never plaintext (rule 13). */}
            {MAIL_SECURITY.map((s) => (
              <TickBox
                key={s.value}
                name={`security-${s.value}`}
                checked={draft.security === s.value}
                onChange={() => setDraft({ ...draft, security: s.value })}
              >
                {s.label}
              </TickBox>
            ))}
          </div>
        </fieldset>
      </form>

      <div className="mt-3 flex flex-wrap items-end gap-2">
        <Button
          type="submit"
          form={SAVE_FORM_ID}
          variant="primary"
          disabled={readOnly || saving}
          icon={saving ? undefined : <Save aria-hidden="true" focusable="false" />}
          aria-busy={saving}
        >
          <BusyLabel busy={saving} label={SAVE_BUTTON} busyText={BUSY_SAVING} />
        </Button>

        {/* From 640px: right-aligned, the box `w-56` (spec 10 #6). Below: the full row, the box
            shrinking — 224px + the button do not fit a 320px screen's card. */}
        <form
          className="m-0 flex w-full min-w-0 items-end gap-2 sm:ml-auto sm:w-auto"
          aria-label="Gửi thư thử"
          onSubmit={(e) => {
            e.preventDefault();
            onTest();
          }}
        >
          <ConfigField label={TEST_LABEL} htmlFor="o-gui-thu-toi" className="flex-1 sm:flex-none">
            <input
              id="o-gui-thu-toi"
              name="recipient"
              type="email"
              className={cn(formInputCls, "sm:w-56")}
              value={recipient}
              placeholder={TEST_RECIPIENT_PLACEHOLDER}
              disabled={readOnly || !saved.configured}
              onChange={(e) => setRecipient(e.target.value)}
            />
          </ConfigField>
          <Button
            type="submit"
            variant="outline"
            icon={<Mail aria-hidden="true" focusable="false" />}
            disabled={testDisabled || recipient.trim() === ""}
            aria-busy={testing}
          >
            {TEST_BUTTON}
          </Button>
        </form>
      </div>

      {saveError !== null && (
        <p role="alert" className="text-danger m-0 mt-2 flex items-center gap-1.5 text-[11.5px]">
          <TriangleAlert aria-hidden="true" focusable="false" className="size-3.5 shrink-0" />
          {saveError}
        </p>
      )}

      {testResult !== null && (
        <p
          role={testResult.ok ? "status" : "alert"}
          className={cn(
            "m-0 mt-2 flex items-center gap-1.5 text-[11.5px]",
            testResult.ok ? "text-leaf" : "text-danger",
          )}
        >
          {testResult.ok ? (
            <CheckCircle2 aria-hidden="true" focusable="false" className="size-3.5 shrink-0" />
          ) : (
            <TriangleAlert aria-hidden="true" focusable="false" className="size-3.5 shrink-0" />
          )}
          {testResult.text}
        </p>
      )}

      {/* Spec 10 #7's persisted "Lần thử gần nhất …": the server does not store it yet (ADR 0079 #5). */}
      {LAST_TEST !== undefined && (
        <p className="text-ink-muted m-0 mt-2 flex items-center gap-1.5 text-[11.5px]" data-pending="">
          <span>{LAST_TEST.ten}</span>
          <PendingMarker info={LAST_TEST} />
        </p>
      )}
    </section>
  );
}
