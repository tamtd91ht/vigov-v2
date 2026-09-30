"use client";

import { useEffect, useState } from "react";

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
      <p className="trang-thai-rong">
        Tài khoản của bạn không có quyền cấu hình máy chủ thư, nên tab này không hiển thị.
      </p>
    );
  }
  if (loaded === null) return <p role="status">Đang tải cấu hình máy chủ thư…</p>;
  if (!loaded.ok) {
    return (
      <p className="thong-bao-loi" role="alert">
        {loaded.thongBao}
      </p>
    );
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
}) {
  const readOnly = !saved.encryption_configured;
  const note = passwordNote(saved, draft);
  return (
    <section className="tab-danh-muc" aria-labelledby="tieu-de-may-chu-thu">
      <h2 id="tieu-de-may-chu-thu">{MAIL_TITLE}</h2>
      <p className="ghi-chu">{MAIL_DESCRIPTION}</p>

      {readOnly && (
        <p className="khoi-chua-khai" role="alert">
          {ENCRYPTION_MISSING}
        </p>
      )}
      {!saved.configured && <p className="canh-bao-pham-vi">{NOT_CONFIGURED_WARNING}</p>}

      <form
        className="form-danh-muc"
        aria-label="Cấu hình máy chủ thư"
        onSubmit={(e) => {
          e.preventDefault();
          onSave();
        }}
      >
        {/* One `disabled` on the fieldset disables every control inside — the read-only state
            cannot forget a field. */}
        <fieldset disabled={readOnly || saving}>
          <div className="o-nhap">
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

          <fieldset className="o-nhap">
            <legend>Bảo mật kết nối</legend>
            {MAIL_SECURITY.map((s) => (
              <label key={s.value}>
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

          <div className="o-nhap">
            <label htmlFor="o-smtp-ten-gui">Tên hiển thị của người gửi</label>
            <input
              id="o-smtp-ten-gui"
              name="from_name"
              value={draft.fromName}
              placeholder="ViGov"
              onChange={(e) => setDraft({ ...draft, fromName: e.target.value })}
            />
          </div>

          <div className="o-nhap">
            <label>
              <input
                type="checkbox"
                name="is_enabled"
                checked={draft.isEnabled}
                onChange={(e) => setDraft({ ...draft, isEnabled: e.target.checked })}
              />{" "}
              Dùng máy chủ thư này cho xã
            </label>
          </div>

          <div className="cum-nut">
            <button type="submit" className="nut-chinh">
              {SAVE_BUTTON}
            </button>
          </div>
        </fieldset>
        {saveMessage !== null &&
          (saveMessage.ok ? (
            <p role="status">{saveMessage.text}</p>
          ) : (
            <p className="thong-bao-loi" role="alert">
              {saveMessage.text}
            </p>
          ))}
      </form>

      <form
        className="form-danh-muc"
        aria-label="Gửi thư thử"
        onSubmit={(e) => {
          e.preventDefault();
          onTest();
        }}
      >
        <fieldset disabled={readOnly || testing || !saved.configured}>
          <div className="o-nhap">
            <label htmlFor="o-gui-thu-toi">{TEST_LABEL}</label>
            <input
              id="o-gui-thu-toi"
              name="recipient"
              type="email"
              value={recipient}
              onChange={(e) => setRecipient(e.target.value)}
              aria-describedby="giai-thich-gui-thu"
            />
            <p className="ghi-chu" id="giai-thich-gui-thu">
              {TEST_SAVED_ONLY}
            </p>
          </div>
          <button type="submit" className="nut-phu" disabled={recipient.trim() === ""}>
            {TEST_BUTTON}
          </button>
        </fieldset>
        {testMessage !== null &&
          (testMessage.ok ? (
            <p role="status">{testMessage.text}</p>
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
