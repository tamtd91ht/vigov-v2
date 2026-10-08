"use client";

import { Mail, Save, Send, TriangleAlert } from "lucide-react";
import { useEffect, useState } from "react";

import { useCauHinhXa } from "@/components/cau-hinh-xa";
import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { NoAccess } from "@/components/ui/no-access";
import { Notice } from "@/components/ui/notice";
import { Skeleton } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";

import { usePhien } from "@/features/phien/phien-hien-tai";
import type { KetQua } from "@/lib/api/goi";
import { getMailSettings, saveMailSettings, sendTestMail } from "@/lib/api/mail-settings";
import type { comms_mailSettingsOut } from "@/lib/api/schema.gen";

import {
  ENCRYPTION_MISSING,
  MAIL_DESCRIPTION,
  MAIL_PORTS,
  MAIL_SECURITY,
  MAIL_TITLE,
  NOT_CONFIGURED_WARNING,
  PASSWORD_SAVED,
  PORT_HINT,
  SAVED_SENTENCE,
  SAVE_BUTTON,
  TEST_BUTTON,
  TEST_LABEL,
  TEST_SAVED_ONLY,
  draftFromSettings,
  passwordNote,
  saveBody,
  testKey,
  testSentSentence,
} from "./mail-settings-form";
import type { MailDraft } from "./mail-settings-form";
import { mailServerTabDecision } from "./quyen-tab";

/**
 * "Cấu hình → Máy chủ thư" (§10). One key, `admin.lookup`, for read and write — the server declares
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
  const [saveMessage, setSaveMessage] = useState<{ ok: boolean; text: string } | null>(null);

  const [recipient, setRecipient] = useState("");
  const [attempt, setAttempt] = useState<{ key: string; recipient: string } | null>(null);
  const [testing, setTesting] = useState(false);
  const [testMessage, setTestMessage] = useState<{ ok: boolean; text: string } | null>(null);

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
  if (loaded === null)
    return (
      // FIRST LOAD (spec §8b): the sentence stays the live region; the eye gets form-shaped bars.
      <div className="page--form flex flex-col gap-3 rounded-card border border-line bg-surface p-4">
        <p role="status" className="an-thi-giac">
          Đang tải cấu hình máy chủ thư…
        </p>
        <Skeleton className="h-5 w-48" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-full" />
        <Skeleton className="h-10 w-2/3" />
      </div>
    );
  if (!loaded.ok) {
    return <ErrorState role="alert" title="Chưa tải được cấu hình máy chủ thư" message={loaded.thongBao} />;
  }
  if (draft === null) return null;

  async function save() {
    if (draft === null || saving) return;
    setSaving(true);
    setSaveMessage(null);
    const r = await saveMailSettings(saveBody(draft));
    setSaving(false);
    if (!r.ok) {
      // The typed password stays in the box: the refusal may be about another field.
      setSaveMessage({ ok: false, text: r.thongBao });
      return;
    }
    setLoaded(r);
    setDraft(draftFromSettings(r.duLieu));
    setSaveMessage({ ok: true, text: SAVED_SENTENCE });
  }

  async function test() {
    const to = recipient.trim();
    if (to === "" || testing) return;
    const a = testKey(attempt, to, () => crypto.randomUUID());
    setAttempt(a);
    setTesting(true);
    setTestMessage(null);
    const r = await sendTestMail(to, a.key);
    setTesting(false);
    if (!r.ok) {
      setTestMessage({ ok: false, text: r.thongBao });
      return;
    }
    // Done: the next click is a new send, with a new key.
    setAttempt(null);
    setTestMessage({ ok: true, text: testSentSentence(to) });
  }

  return (
    <MailServerView
      saved={loaded.duLieu}
      draft={draft}
      setDraft={setDraft}
      saving={saving}
      saveMessage={saveMessage}
      onSave={() => void save()}
      recipient={recipient}
      setRecipient={setRecipient}
      testing={testing}
      testMessage={testMessage}
      onTest={() => void test()}
      communeName={commune.displayName}
    />
  );
}

/**
 * Pure rendering, exported so the tests read the markup: the password input's `value` must be
 * empty whatever was saved, and the encryption banner must disable the form.
 */
export function MailServerView({
  saved,
  draft,
  setDraft,
  saving,
  saveMessage,
  onSave,
  recipient,
  setRecipient,
  testing,
  testMessage,
  onTest,
  communeName,
}: {
  saved: comms_mailSettingsOut;
  draft: MailDraft;
  setDraft: (d: MailDraft) => void;
  saving: boolean;
  saveMessage: { ok: boolean; text: string } | null;
  onSave: () => void;
  recipient: string;
  setRecipient: (s: string) => void;
  testing: boolean;
  testMessage: { ok: boolean; text: string } | null;
  onTest: () => void;
  /** The signed-in commune's `displayName`, verbatim — the sender-name placeholder. */
  communeName: string;
}) {
  const readOnly = !saved.encryption_configured;
  const note = passwordNote(saved, draft);
  return (
    // The prototype's `EmailSettingPanel` (ADR 0068 lần 5): ONE white card, 768px wide, its title inside
    // (icon + 13px bold), the fields in two columns with host and sender name full width, the switches
    // in one muted box, then "Lưu cấu hình" and — right-aligned — the test send.
    <section
      className="tab-danh-muc flex max-w-3xl min-w-0 flex-col gap-3 rounded-card border border-line bg-surface p-4 [&>*]:my-0"
      aria-labelledby="tieu-de-may-chu-thu"
    >
      <div className="min-w-0">
        <h2 id="tieu-de-may-chu-thu" className="m-0 flex items-center gap-1.5 text-[13px] font-bold text-ink-900">
          <Mail aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0 text-brand-600" />
          {MAIL_TITLE}
        </h2>
        <p className="ghi-chu m-0 mt-1 text-xs text-ink-500">{MAIL_DESCRIPTION}</p>
      </div>

      {readOnly && (
        <p className="khoi-chua-khai" role="alert">
          {ENCRYPTION_MISSING}
        </p>
      )}
      {!saved.configured && (
        <Notice tone="neutral" icon={TriangleAlert} className="canh-bao-pham-vi">
          {NOT_CONFIGURED_WARNING}
        </Notice>
      )}

      <form
        className="form-danh-muc m-0 border-0 bg-transparent p-0"
        aria-label="Cấu hình máy chủ thư"
        onSubmit={(e) => {
          e.preventDefault();
          onSave();
        }}
      >
        {/* One `disabled` on the fieldset disables every control inside — the read-only state
            cannot forget a field. */}
        <fieldset disabled={readOnly || saving}>
          {/* Layout lives on this inner div, NOT on the fieldset: the tab's test counts the bare
              `<fieldset disabled="">` tags. Two columns from 640px for the short paired fields. */}
          <div className="grid min-w-0 gap-3 sm:grid-cols-2 [&>*]:m-0 [&>.cum-nut]:col-span-full">
          <div className="o-nhap sm:col-span-2">
            <label htmlFor="o-smtp-host">Máy chủ SMTP</label>
            <input
              id="o-smtp-host"
              name="host"
              value={draft.host}
              placeholder="smtp.danang.gov.vn"
              autoComplete="off"
              onChange={(e) => setDraft({ ...draft, host: e.target.value })}
            />
          </div>

          <div className="o-nhap">
            <label htmlFor="o-smtp-port">Cổng</label>
            <select
              id="o-smtp-port"
              name="port"
              value={String(draft.port)}
              onChange={(e) => setDraft({ ...draft, port: Number(e.target.value) })}
              aria-describedby="giai-thich-cong-smtp"
            >
              {/* The saved port is kept selectable even if it is not one of the four — shown as sent. */}
              {(MAIL_PORTS.includes(draft.port) ? MAIL_PORTS : [draft.port, ...MAIL_PORTS]).map((p) => (
                <option key={p} value={String(p)}>
                  {p}
                </option>
              ))}
            </select>
            <p className="ghi-chu" id="giai-thich-cong-smtp">
              {PORT_HINT}
            </p>
          </div>

          <div className="o-nhap">
            <label htmlFor="o-smtp-tai-khoan">Tài khoản</label>
            <input
              id="o-smtp-tai-khoan"
              name="username"
              value={draft.username}
              autoComplete="off"
              onChange={(e) => setDraft({ ...draft, username: e.target.value })}
            />
          </div>

          <div className="o-nhap">
            <label htmlFor="o-smtp-mat-khau">Mật khẩu</label>
            {/* WRITE-ONLY: `value` is the draft's, which starts "" and is never filled from the
                server. `new-password` keeps the browser from offering the staff member's own
                login password here. */}
            <input
              id="o-smtp-mat-khau"
              name="password"
              type="password"
              autoComplete="new-password"
              value={draft.password}
              onChange={(e) => setDraft({ ...draft, password: e.target.value })}
              aria-describedby="giai-thich-mat-khau-smtp"
            />
            <p className="ghi-chu" id="giai-thich-mat-khau-smtp">
              {saved.password_set ? PASSWORD_SAVED : ""}
              {note !== "" && (
                <>
                  {saved.password_set ? " " : ""}
                  <strong>{note}</strong>
                </>
              )}
            </p>
          </div>

          <div className="o-nhap">
            <label htmlFor="o-smtp-dia-chi-gui">Địa chỉ gửi</label>
            <input
              id="o-smtp-dia-chi-gui"
              name="from_address"
              type="email"
              value={draft.fromAddress}
              placeholder="ubnd@xa.danang.gov.vn"
              onChange={(e) => setDraft({ ...draft, fromAddress: e.target.value })}
            />
          </div>

          <div className="o-nhap sm:col-span-2">
            <label htmlFor="o-smtp-ten-gui">Tên hiển thị của người gửi</label>
            <input
              id="o-smtp-ten-gui"
              name="from_name"
              value={draft.fromName}
              placeholder={communeName}
              onChange={(e) => setDraft({ ...draft, fromName: e.target.value })}
            />
          </div>

          {/* The prototype's muted box of switches: "use this server", then the connection security.
              Security stays ONE choice of three (the contract's enum), not two independent boxes. */}
          <div className="col-span-full flex min-w-0 flex-col gap-1 rounded-[10px] border border-line bg-surface-muted p-3">
            <label className="inline-flex min-h-10 cursor-pointer items-center gap-2 text-[13px]">
              <input
                type="checkbox"
                name="is_enabled"
                checked={draft.isEnabled}
                onChange={(e) => setDraft({ ...draft, isEnabled: e.target.checked })}
              />{" "}
              Dùng máy chủ thư này cho xã
            </label>
            <fieldset className="o-nhap m-0 flex min-w-0 flex-wrap gap-x-6 gap-y-1 border-0 p-0">
              <legend className="p-0 text-xs font-semibold text-ink-700">Bảo mật kết nối</legend>
              {MAIL_SECURITY.map((s) => (
                <label key={s.value} className="inline-flex min-h-10 cursor-pointer items-center gap-2 text-[13px]">
                  <input
                    type="radio"
                    name="security"
                    value={s.value}
                    checked={draft.security === s.value}
                    onChange={() => setDraft({ ...draft, security: s.value })}
                  />{" "}
                  {s.label}
                </label>
              ))}
            </fieldset>
          </div>

          <div className="cum-nut flex">
            <Button
              type="submit"
              variant="primary"
              icon={saving ? undefined : <Save aria-hidden="true" focusable="false" strokeWidth={1.8} />}
              aria-busy={saving}
            >
              <BusyLabel busy={saving} label={SAVE_BUTTON} busyText={BUSY_SAVING} />
            </Button>
          </div>
          </div>
        </fieldset>
        {saveMessage !== null &&
          (saveMessage.ok ? (
            <p role="status" className="mt-3 text-sm font-medium text-success-600">
              {saveMessage.text}
            </p>
          ) : (
            <p className="thong-bao-loi" role="alert">
              {saveMessage.text}
            </p>
          ))}
      </form>

      <form
        className="form-danh-muc m-0 border-0 bg-transparent p-0"
        aria-label="Gửi thư thử"
        onSubmit={(e) => {
          e.preventDefault();
          onTest();
        }}
      >
        <fieldset disabled={readOnly || testing || !saved.configured}>
          {/* Right-aligned, as the prototype's: the address box then the outline "Gửi thử". */}
          <div className="flex min-w-0 flex-wrap items-end justify-end gap-2">
            <div className="o-nhap m-0 min-w-0 flex-[0_1_16rem]">
              <label htmlFor="o-gui-thu-toi">{TEST_LABEL}</label>
              <input
                id="o-gui-thu-toi"
                name="recipient"
                type="email"
                value={recipient}
                placeholder="ten@xa.danang.gov.vn"
                onChange={(e) => setRecipient(e.target.value)}
                aria-describedby="giai-thich-gui-thu"
              />
            </div>
            <Button
              type="submit"
              variant="outline"
              icon={<Send aria-hidden="true" focusable="false" strokeWidth={1.8} />}
              disabled={recipient.trim() === ""}
              aria-busy={testing}
            >
              {TEST_BUTTON}
            </Button>
            <p className="ghi-chu m-0 w-full text-right text-xs text-ink-500" id="giai-thich-gui-thu">
              {TEST_SAVED_ONLY}
            </p>
          </div>
        </fieldset>
        {testMessage !== null &&
          (testMessage.ok ? (
            <p role="status" className="mt-3 text-sm font-medium text-success-600">
              {testMessage.text}
            </p>
          ) : (
            // For 502 this is the server's sentence per SMTP failure, saying what to check.
            <p className="thong-bao-loi" role="alert">
              {testMessage.text}
            </p>
          ))}
      </form>
    </section>
  );
}
