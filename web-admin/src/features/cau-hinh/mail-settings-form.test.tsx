import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import type { comms_mailSettingsOut } from "@/lib/api/schema.gen";

import { MailServerView } from "./mail-server-tab";
import {
  ENCRYPTION_MISSING,
  NOT_CONFIGURED_WARNING,
  draftFromSettings,
  passwordNote,
  saveBody,
  testKey,
} from "./mail-settings-form";
import type { MailDraft } from "./mail-settings-form";

/**
 * "Máy chủ thư": the password is never pre-filled and never sent blank; changing the destination
 * says the password must be retyped; a platform without the encryption key makes the form read-only.
 */

const SAVED: comms_mailSettingsOut = {
  configured: true,
  host: "smtp.xa.gov.vn",
  port: 587,
  security: "starttls",
  username: "ubnd@xa.gov.vn",
  from_address: "ubnd@xa.gov.vn",
  from_name: "ViGov",
  is_enabled: true,
  password_set: true,
  encryption_configured: true,
};

function view(saved: comms_mailSettingsOut, draft: MailDraft = draftFromSettings(saved)) {
  return renderToStaticMarkup(
    <MailServerView
      saved={saved}
      draft={draft}
      setDraft={() => {}}
      saving={false}
      saveMessage={null}
      onSave={() => {}}
      recipient=""
      setRecipient={() => {}}
      testing={false}
      testMessage={null}
      onTest={() => {}}
    />,
  );
}

describe("password", () => {
  it("never pre-filled — even if a response ever carried one", () => {
    const leaky = { ...SAVED, password: "khong-duoc-hien" } as unknown as comms_mailSettingsOut;
    expect(draftFromSettings(leaky).password).toBe("");
    const html = view(leaky);
    expect(html).not.toContain("khong-duoc-hien");
    expect(html).toMatch(/<input id="o-smtp-mat-khau" type="password"[^>]*name="password" value=""\/>/);
  });

  it("blank is never sent; typed is", () => {
    expect(saveBody(draftFromSettings(SAVED))).not.toHaveProperty("password");
    expect(saveBody({ ...draftFromSettings(SAVED), password: "moi" }).password).toBe("moi");
  });

  it("destination changed with a stored password and nothing typed → must retype", () => {
    const d = { ...draftFromSettings(SAVED), host: "smtp.khac.gov.vn" };
    expect(passwordNote(SAVED, d)).toMatch(/phải nhập lại mật khẩu/);
    expect(passwordNote(SAVED, { ...d, password: "moi" })).toBe("");
    expect(passwordNote(SAVED, { ...draftFromSettings(SAVED), port: 465 })).toMatch(/nhập lại/);
    expect(passwordNote(SAVED, { ...draftFromSettings(SAVED), fromName: "Khác" })).toBe("");
  });

  it("nothing stored yet → the first save needs a password", () => {
    expect(passwordNote({ ...SAVED, password_set: false }, draftFromSettings(SAVED))).toMatch(/Lần lưu đầu tiên/);
  });

  it("'đã lưu mật khẩu' shows only when password_set", () => {
    expect(view(SAVED)).toContain("Đã lưu mật khẩu");
    expect(view({ ...SAVED, password_set: false })).not.toContain("Đã lưu mật khẩu");
  });
});

describe("states", () => {
  it("encryption_configured=false → banner, both forms disabled", () => {
    const html = view({ ...SAVED, encryption_configured: false });
    expect(html).toContain(ENCRYPTION_MISSING);
    expect((html.match(/<fieldset disabled="">/g) ?? []).length).toBe(2);
  });

  it("configured=false → §10's orange warning; test send disabled (nothing saved to send through)", () => {
    const html = view({ ...SAVED, configured: false, password_set: false });
    expect(html).toContain(NOT_CONFIGURED_WARNING);
    expect((html.match(/<fieldset disabled="">/g) ?? []).length).toBe(1);
  });

  it("port is a select of the four accepted ports", () => {
    const html = view(SAVED);
    for (const p of ["587", "465", "25", "2525"]) expect(html).toContain(`<option value="${p}"`);
  });
});

describe("test-send key", () => {
  it("kept for a retry to the same recipient, new for another", () => {
    const mint = vi.fn(() => `k${mint.mock.calls.length}`);
    const a = testKey(null, "a@b.vn", mint);
    expect(testKey(a, "a@b.vn", mint)).toBe(a);
    expect(testKey(a, "c@d.vn", mint).key).not.toBe(a.key);
    expect(mint).toHaveBeenCalledTimes(2);
  });
});
