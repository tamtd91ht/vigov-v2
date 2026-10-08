import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it, vi } from "vitest";

import type { comms_mailSettingsOut } from "@/lib/api/schema.gen";

import { MailServerLoading, MailServerView } from "./mail-server-tab";
import {
  ENCRYPTION_MISSING,
  MAIL_DESCRIPTION,
  NOT_CONFIGURED_WARNING,
  PASSWORD_NEW_DESTINATION,
  destinationChanged,
  draftFromSettings,
  saveBody,
  saveRefusalMessage,
  testKey,
} from "./mail-settings-form";
import type { MailDraft } from "./mail-settings-form";
import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";

/**
 * "Máy chủ thư" (spec 10, ADR 0079): the password is never pre-filled and never sent blank; changing
 * the destination says in place that the password must be retyped; a platform without the encryption
 * key makes the form read-only; the two security boxes are one choice, never both off.
 */

const SAVED: comms_mailSettingsOut = {
  configured: true,
  host: "smtp.xa.gov.vn",
  port: 587,
  security: "starttls",
  username: "ubnd@xa.gov.vn",
  from_address: "ubnd@xa.gov.vn",
  from_name: "UBND xã",
  is_enabled: true,
  password_set: true,
  encryption_configured: true,
  last_test: null,
};

function view(
  saved: comms_mailSettingsOut,
  draft: MailDraft = draftFromSettings(saved),
  extra: { saveError?: string | null; testResult?: { ok: boolean; text: string } | null; recipient?: string } = {},
) {
  return renderToStaticMarkup(
    <MailServerView
      saved={saved}
      draft={draft}
      setDraft={() => {}}
      saving={false}
      saveError={extra.saveError ?? null}
      onSave={() => {}}
      recipient={extra.recipient ?? ""}
      setRecipient={() => {}}
      testing={false}
      testResult={extra.testResult ?? null}
      onTest={() => {}}
      communeName="UBND xã Tân Phú"
    />,
  );
}

/** The opening tag of the first element carrying `attr` — attribute order is React's, not the test's. */
function tagWith(html: string, attr: string): string {
  const at = html.indexOf(` ${attr}`);
  return at < 0 ? "" : html.slice(html.lastIndexOf("<", at), html.indexOf(">", at) + 1);
}
const checkbox = (html: string, name: string) => tagWith(html, `name="${name}"`);

describe("password", () => {
  it("never pre-filled — even if a response ever carried one", () => {
    const leaky = { ...SAVED, password: "khong-duoc-hien" } as unknown as comms_mailSettingsOut;
    expect(draftFromSettings(leaky).password).toBe("");
    const html = view(leaky);
    expect(html).not.toContain("khong-duoc-hien");
    const box = tagWith(html, 'id="o-smtp-mat-khau"');
    for (const a of ['type="password"', 'autoComplete="new-password"', 'value=""']) expect(box).toContain(a);
  });

  it("blank is never sent; typed is", () => {
    expect(saveBody(draftFromSettings(SAVED))).not.toHaveProperty("password");
    expect(saveBody({ ...draftFromSettings(SAVED), password: "moi" }).password).toBe("moi");
  });

  it("placeholder 'Giữ nguyên mật khẩu cũ' only when one is saved; no extra hint under the box", () => {
    expect(tagWith(view(SAVED), 'id="o-smtp-mat-khau"')).toContain('placeholder="Giữ nguyên mật khẩu cũ"');
    expect(view({ ...SAVED, password_set: false })).not.toContain("Giữ nguyên mật khẩu cũ");
    // The old hints are gone (spec 10 #4 has none).
    expect(view(SAVED)).not.toContain("Đã lưu mật khẩu");
    expect(view({ ...SAVED, password_set: false })).not.toContain("Lần lưu đầu tiên");
  });

  it("refused save with the destination changed and nothing typed → §10's sentence, in place", () => {
    const moved = { ...draftFromSettings(SAVED), host: "smtp.khac.gov.vn" };
    expect(saveRefusalMessage(SAVED, moved, "câu máy chủ")).toBe(PASSWORD_NEW_DESTINATION);
    expect(saveRefusalMessage(SAVED, { ...draftFromSettings(SAVED), port: 465 }, "x")).toBe(PASSWORD_NEW_DESTINATION);
    // Typed a password, or nothing stored, or only the name changed: the server's own sentence.
    expect(saveRefusalMessage(SAVED, { ...moved, password: "moi" }, "câu máy chủ")).toBe("câu máy chủ");
    expect(saveRefusalMessage({ ...SAVED, password_set: false }, moved, "câu máy chủ")).toBe("câu máy chủ");
    expect(saveRefusalMessage(SAVED, { ...draftFromSettings(SAVED), fromName: "Khác" }, "câu máy chủ")).toBe("câu máy chủ");
    const html = view(SAVED, moved, { saveError: PASSWORD_NEW_DESTINATION });
    expect(html).toMatch(/<p role="alert" class="text-danger[^"]*">.*Đổi máy chủ, cổng hoặc tài khoản thì phải gõ lại mật khẩu/);
  });

  it("host compared as the server normalises it (lower case, trimmed) — capitals are not a new destination", () => {
    expect(destinationChanged(SAVED, { ...draftFromSettings(SAVED), host: " SMTP.XA.GOV.VN " })).toBe(false);
  });
});

describe("security — one server field, two exclusive boxes (rule 13)", () => {
  it("STARTTLS saved → its box on, TLS box off; TLS saved → the reverse; never both, never neither", () => {
    const a = view(SAVED);
    expect(checkbox(a, "security-starttls")).toContain('checked=""');
    expect(checkbox(a, "security-tls")).not.toContain("checked");
    const b = view({ ...SAVED, security: "tls" });
    expect(checkbox(b, "security-tls")).toContain('checked=""');
    expect(checkbox(b, "security-starttls")).not.toContain("checked");
  });

  it("labels verbatim from spec 10 #5; no legend, no radio", () => {
    const html = view(SAVED);
    expect(html).toContain("STARTTLS (thường dùng với cổng 587)");
    expect(html).toContain("TLS ngay từ đầu (thường dùng với cổng 465)");
    expect(html).toContain("Dùng máy chủ thư này cho xã");
    expect(html).not.toContain('type="radio"');
    expect(html).not.toContain("<legend");
    expect(checkbox(html, "security-tls")).toContain('type="checkbox"');
    expect(checkbox(html, "security-tls")).toContain('class="accent-brand m-0 size-4 shrink-0"');
    // Rows spaced by gap, not space-y (which loses to the rows' m-0 in Tailwind v4).
    expect(html).toContain('class="border-line bg-background mt-3 flex flex-col gap-2 rounded-[10px]');
    expect(html).not.toContain("space-y-");
  });
});

describe("states", () => {
  it("encryption_configured=false → banner kept, form and test send disabled", () => {
    const html = view({ ...SAVED, encryption_configured: false }, undefined, { recipient: "a@b.vn" });
    expect(html).toContain(ENCRYPTION_MISSING);
    expect((html.match(/<fieldset disabled=""/g) ?? []).length).toBe(1);
    expect(tagWith(html, 'form="form-may-chu-thu"')).toContain('disabled=""');
    expect(tagWith(html, 'id="o-gui-thu-toi"')).toContain('disabled=""');
  });

  it("not configured or not enabled → the orange warning; enabled → none", () => {
    for (const s of [{ ...SAVED, configured: false, password_set: false }, { ...SAVED, is_enabled: false }]) {
      const html = view(s);
      expect(html).toContain(NOT_CONFIGURED_WARNING);
      expect(html).toMatch(/<p class="text-tangerine m-0 mt-2 flex items-center gap-1.5 text-\[11.5px\]">/);
      // No platform fallback (ADR 0079 lô 2 Q1 #8): ONLY the warning — no fallback line, no "?" for it.
      expect(html).not.toContain("Đường thư dự phòng");
      expect(html).not.toContain("Đang dùng máy chủ thư của nền tảng");
      expect(html).not.toContain("dự phòng");
    }
    const on = view(SAVED);
    expect(on).not.toContain(NOT_CONFIGURED_WARNING);
  });

  it("description: only the true sentence — no platform fallback exists in service-comms", () => {
    const html = view(SAVED);
    expect(html).toContain(MAIL_DESCRIPTION);
    expect(html).not.toContain("máy chủ thư của nền tảng nếu có");
    expect(html).not.toContain("của tỉnh");
  });

  it("configured=false → test send disabled (nothing saved to send through)", () => {
    const html = view({ ...SAVED, configured: false, password_set: false }, undefined, { recipient: "a@b.vn" });
    expect(tagWith(html, 'id="o-gui-thu-toi"')).toContain('disabled=""');
  });

  it("Gửi thử disabled while the recipient is empty", () => {
    const testButton = (html: string) => html.match(/<button[^>]*>(?:(?!<button).)*Gửi thử<\/button>/)?.[0] ?? "";
    expect(testButton(view(SAVED))).toContain('disabled=""');
    expect(testButton(view(SAVED, undefined, { recipient: "a@b.vn" }))).not.toContain('disabled=""');
  });

  it("shell: white card, max 3xl, no shadow; one action row; no TEST_SAVED_ONLY note", () => {
    const html = view(SAVED);
    expect(html).toMatch(/<section class="border-line max-w-3xl rounded-\[12px\] border border-solid bg-white p-4"/);
    expect(tagWith(html, 'aria-labelledby="tieu-de-may-chu-thu"')).not.toMatch(/shadow/);
    expect(html).toMatch(/<h3 id="tieu-de-may-chu-thu" class="text-navy m-0 flex items-center gap-1.5 text-\[13px\] font-bold">/);
    expect(html).not.toContain("ĐÃ LƯU");
    expect(html).toContain('class="mt-3 flex flex-wrap items-end gap-2"');
    expect(html).not.toMatch(/o-nhap|nut-phu-|page--form|form-danh-muc/);
  });

  it("port is a number box (default 587 from the server), with the spec's hint", () => {
    const html = view(SAVED);
    const port = tagWith(html, 'id="o-smtp-port"');
    expect(port).toContain('type="number"');
    expect(port).toContain('value="587"');
    expect(html).not.toContain("<select");
    expect(html).toContain("587 dùng STARTTLS, 465 dùng TLS ngay từ đầu.");
  });

  it("this session's test result: leaf + check on success, danger on failure; persisted last test is a '?'", () => {
    const ok = view(SAVED, undefined, { testResult: { ok: true, text: "gửi được" } });
    expect(ok).toMatch(/<p role="status" class="m-0 mt-2 flex items-center gap-1.5 text-\[11.5px\] text-leaf">/);
    const bad = view(SAVED, undefined, { testResult: { ok: false, text: "hỏng" } });
    expect(bad).toMatch(/<p role="alert" class="m-0 mt-2 flex items-center gap-1.5 text-\[11.5px\] text-danger">/);
    // Prototype words (`EmailSettingPanel.tsx:277`): "Lần thử gần nhất".
    expect(view(SAVED)).toMatch(/aria-label="Lần thử gần nhất — tính năng đang phát triển/);
    expect(view(SAVED)).toContain("<span>Lần thử gần nhất</span>");
  });

  it("registry: 'Lần thử gần nhất' is an entry; no entry for a platform mail fallback (decided: none)", () => {
    expect(PHAN_CHUA_DUNG.filter((p) => p.ten === "Lần thử gần nhất")).toHaveLength(1);
    expect(PHAN_CHUA_DUNG.some((p) => /Lần thử gửi thư gần nhất|dự phòng|máy chủ thư của nền tảng/i.test(`${p.ten} ${p.viSao}`))).toBe(false);
  });

  it("loading is one Skeleton h-80", () => {
    const html = renderToStaticMarkup(<MailServerLoading />);
    expect(html).toContain("h-80");
    expect(html).toContain('role="status"');
  });
});

describe("placeholders name no commune and no vendor (rule 1 inv 10)", () => {
  it("sender name = the commune's displayName; host/address/recipient neutral", () => {
    const html = view(SAVED);
    expect(tagWith(html, 'id="o-smtp-ten-gui"')).toContain('placeholder="UBND xã Tân Phú"');
    expect(html).not.toContain('placeholder="ViGov"');
    expect(html.replace(/<[^>]*>/g, " ")).not.toMatch(/vigov/i);
    for (const banned of ["danang", "vihatgroup", "Thăng Bình"]) expect(html).not.toContain(banned);
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
