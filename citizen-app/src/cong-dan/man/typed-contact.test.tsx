import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * SENDING WHEN ZALO GIVES NO NUMBER (ADR 0080, 08/10/2026) — the state half.
 *
 * What must hold (the card, decisions 1–7 of the ADR):
 *   · the invitation to share the Zalo number comes FIRST; typing a name and a number is the fallback, offered for
 *     SENDING only, and only when Zalo gave no number (declined, Zalo failed the phone step, or identity issued a
 *     phone-less session) — never outside Zalo, never for a network failure;
 *   · the fallback opens a phone-less session only on the citizen's tap ("Gửi bằng họ tên và số điện thoại");
 *   · on that path name AND number are required, the number must look Vietnamese, anonymous is off;
 *   · the body carries the typed name and number, `anonymous: false`;
 *   · 429 `unverified_daily_limit` shows the server's sentence and keeps the draft;
 *   · a 201 with `contact_unverified` says, under the code, that no notification will come.
 *
 * The session source is the REAL one (`api/phien-vigov.ts`); only the address map is replaced, so `guiPhanAnh`
 * has somewhere to send in this test.
 */
vi.mock("../api/dia-chi-vigov", () => ({
  diaChiViGov: (_service: string, path: string) => `https://vigov.vidu.example${path}`,
}));

import { guiPhanAnh } from "../api/goi-vigov"; // vi-name-ok: existing export, imported not renamed (rule 12 #3)
import { docPhieu, type PhieuCuaToi } from "../api/hop-dong-phan-anh"; // vi-name-ok: existing exports, imported not renamed (rule 12 #3)
import { taoLanGui } from "../api/lan-gui"; // vi-name-ok: existing export, imported not renamed (rule 12 #3)
import type { CommuneAppSessionResult, OpenCommuneAppSession, SessionPhone } from "../api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../api/phien-vigov"; // vi-name-ok: existing exports, imported not renamed (rule 12 #3)

import {
  createSessionGate,
  offersManualSend,
  runsWithoutPhone,
  sessionGateMessage,
  type SessionGateState,
} from "./commune-session";
import { kiemPhanAnh, KetQuaGui } from "./GuiPhanAnhScreen"; // vi-name-ok: existing exports, imported not renamed (rule 12 #3)
import { LOI_GUI, PHONE_VERIFICATION, TYPED_CONTACT, XA_PA } from "./noi-dung"; // vi-name-ok: existing exports, imported not renamed (rule 12 #3)
import { SendDone, sendBody, sendOutcome } from "./PhanAnhAppXa";
import { PhoneVerificationPanel, phoneVerificationOffersManual } from "./phone-verification";
import { SessionGateScreen } from "./TrangXa";
import { isPlausiblePhone, kiemNhapPhieu, type NhapPhieu } from "./trai-nghiem"; // vi-name-ok: existing exports, imported not renamed (rule 12 #3)

const COMMUNE = "Xã Thử Nghiệm";
const VERIFIED: CommuneAppSessionResult = { kieu: "xong", token: "tok-test", ten_xa: COMMUNE, da_xac_thuc_so: true };
const PHONE_LESS: CommuneAppSessionResult = { ...VERIFIED, da_xac_thuc_so: false };
const ZALO_PHONE = { capability: "phone", code: -1401, transient: false } as const;
const ZALO_ACCESS = { capability: "access-token", code: -1401, transient: false } as const;

afterEach(() => {
  datPhienViGov(null);
  vi.unstubAllGlobals();
});

function harness(open: OpenCommuneAppSession) {
  const states: (SessionGateState | null)[] = [];
  const gate = createSessionGate(open, () => COMMUNE, (s) => states.push(s));
  return { gate, last: () => states[states.length - 1] };
}

/* ─────────────────────────────── the gate: ask first, fallback second ─────────────────────────────── */

describe("gate — the invitation to share comes first, the typed path only when Zalo gave no number", () => {
  it("send: `hoi` first and NOTHING opened; the typed path is not even offered before the citizen answers", () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => VERIFIED);
    const { gate, last } = harness(open);
    gate.require(() => {}, "submit");
    expect(last()).toEqual({ kieu: "hoi" });
    expect(open).not.toHaveBeenCalled();
  });

  it("declined → `tu-choi` with the typed path; the tap opens a PHONE-LESS session (`skip`) and runs the send once", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => PHONE_LESS);
    const run = vi.fn();
    const { gate, last } = harness(open);
    gate.require(run, "submit");
    gate.decline();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "tu-choi" });
    expect(offersManualSend("tu-choi", "submit")).toBe(true);
    expect(open).not.toHaveBeenCalled();
    await gate.manual();
    expect(open.mock.calls.map((c) => c[1])).toEqual<SessionPhone[]>(["skip"]);
    expect(run).toHaveBeenCalledTimes(1);
    expect(layPhienViGov()).toEqual({ token: "tok-test", ten_xa: COMMUNE, phone_verified: false });
    // The next send runs at once on that session — the citizen is not asked again in this open.
    gate.require(run, "submit");
    expect(run).toHaveBeenCalledTimes(2);
    expect(open).toHaveBeenCalledTimes(1);
  });

  it("agreed, Zalo's dialog refused → `tu-choi`, the send is KEPT for the typed path", async () => {
    const open = vi
      .fn<OpenCommuneAppSession>()
      .mockResolvedValueOnce({ kieu: "tu-choi" })
      .mockResolvedValueOnce(PHONE_LESS);
    const run = vi.fn();
    const { gate, last } = harness(open);
    gate.require(run, "submit");
    await gate.allow();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "tu-choi" });
    await gate.manual();
    expect(open.mock.calls.map((c) => c[1])).toEqual<SessionPhone[]>(["ask", "skip"]);
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("agreed, identity issued a PHONE-LESS session (the commune app's retry) → `no-phone` first, then the tap runs it with NO second open", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => PHONE_LESS);
    const run = vi.fn();
    const { gate, last } = harness(open);
    gate.require(run, "submit");
    await gate.allow();
    // Told before the form asks them to type: the citizen agreed to share, and Zalo gave nothing.
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "no-phone" });
    expect(run).not.toHaveBeenCalled();
    expect(sessionGateMessage("no-phone", "submit")).toBe(TYPED_CONTACT.zalo_gave_no_number("phản ánh chưa được gửi"));
    await gate.manual();
    expect(open).toHaveBeenCalledTimes(1);
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("Zalo failed the PHONE step → the typed path is offered; a failed ACCESS step or a network failure → not", () => {
    expect(offersManualSend("thu-lai", "submit", ZALO_PHONE)).toBe(true);
    expect(offersManualSend("thu-lai", "submit", ZALO_ACCESS)).toBe(false);
    expect(offersManualSend("thu-lai", "submit")).toBe(false);
  });

  it("never outside Zalo (no session, no commune — rule 1), never for another commune or a closed channel", () => {
    for (const outcome of ["ngoai-zalo", "khac-xa", "chua-ket-noi", "tam-ngung", "cho-lat", "chua-mo"] as const) {
      expect(offersManualSend(outcome, "submit"), outcome).toBe(false);
    }
  });

  it("only SENDING has the typed path; looking up runs on a phone-less session; 'Phản ánh của tôi' does not", async () => {
    for (const task of ["lookup", "mine", "rate"] as const) {
      expect(offersManualSend("tu-choi", task), task).toBe(false);
    }
    expect(runsWithoutPhone("submit")).toBe(true);
    expect(runsWithoutPhone("lookup")).toBe(true);
    expect(runsWithoutPhone("mine")).toBe(false);

    const open = vi.fn<OpenCommuneAppSession>(async () => PHONE_LESS);
    const lookup = vi.fn();
    const mine = vi.fn();
    const { gate, last } = harness(open);
    gate.require(lookup, "lookup");
    await gate.allow();
    expect(lookup).toHaveBeenCalledTimes(1);
    // A phone-less session, an act that needs the number, Zalo gave none in this open → said, not asked again.
    gate.require(mine, "mine");
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "chua-xac-thuc-so" });
    expect(mine).not.toHaveBeenCalled();
    expect(open).toHaveBeenCalledTimes(1);
  });

  it("`manual` does nothing for an act other than sending (no session opened behind the citizen's back)", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => PHONE_LESS);
    const { gate } = harness(open);
    gate.require(() => {}, "mine");
    gate.decline();
    await gate.manual();
    expect(open).not.toHaveBeenCalled();
    expect(layPhienViGov()).toBeNull();
  });
});

describe("gate screen — the typed path is explained before it is chosen", () => {
  const render = (state: SessionGateState, task: "submit" | "mine") =>
    renderToStaticMarkup(
      createElement(SessionGateScreen, { state, task, onAllow() {}, onDecline() {}, onManual() {}, onClose() {} }),
    );

  it("`tu-choi` on a send: the refusal, then what the typed path means, then the 48px button", () => {
    const html = render({ kieu: "ket-qua", outcome: "tu-choi" }, "submit");
    expect(html).toContain(TYPED_CONTACT.offer);
    expect(html).toContain(`<button type="button" class="xa-nut">${TYPED_CONTACT.button}</button>`);
    expect(html.indexOf(TYPED_CONTACT.offer)).toBeLessThan(html.indexOf(TYPED_CONTACT.button));
  });

  it("not on 'Phản ánh của tôi', and not outside Zalo", () => {
    expect(render({ kieu: "ket-qua", outcome: "tu-choi" }, "mine")).not.toContain(TYPED_CONTACT.button);
    expect(render({ kieu: "ket-qua", outcome: "ngoai-zalo" }, "submit")).not.toContain(TYPED_CONTACT.button);
  });

  it("the shared phone panel offers it only where the send screen passes `onManual`", () => {
    const state = { kieu: "ket-qua", outcome: "tu-choi" } as const;
    const withIt = renderToStaticMarkup(
      createElement(PhoneVerificationPanel, { state, task: "submit", onAllow() {}, onDecline() {}, onManual() {} }),
    );
    const without = renderToStaticMarkup(
      createElement(PhoneVerificationPanel, { state, task: "lookup", onAllow() {}, onDecline() {} }),
    );
    expect(withIt).toContain(TYPED_CONTACT.button);
    expect(without).not.toContain(TYPED_CONTACT.button);
    expect(phoneVerificationOffersManual("van-chua-xac-thuc")).toBe(true);
    expect(phoneVerificationOffersManual("ngoai-zalo")).toBe(false);
    expect(phoneVerificationOffersManual("thu-lai")).toBe(false);
    expect(phoneVerificationOffersManual("thu-lai", ZALO_PHONE)).toBe(true);
  });

  it("the explanation no longer says the number is needed to SEND — and keeps the privacy promise", () => {
    expect(PHONE_VERIFICATION.why).not.toMatch(/Để gửi phản ánh/);
    expect(PHONE_VERIFICATION.why).toMatch(/chỉ bạn xem được phản ánh bạn đã gửi/);
  });
});

/* ─────────────────────────────── the form on the typed path ─────────────────────────────── */

const FORM: NhapPhieu = {
  linh_vuc: "rac-thai",
  noi_dung: "Rác tồn đọng đầu ngõ 12",
  dia_chi: "",
  ho_ten: "",
  dien_thoai: "",
  an_danh: true,
};
const WORDS = { thieu: "thiếu", thieu_nguoi_gui: "thiếu người gửi", qua_dai: () => "dài" };
const TYPED = { name_missing: "name", phone_missing: "phone", phone_invalid: "invalid" };

describe("typed contact — name AND number required, the number looks Vietnamese, anonymous off", () => {
  it("anonymous does not excuse the name or the number on this path", () => {
    expect(kiemNhapPhieu(FORM, WORDS)).toEqual({});
    expect(kiemNhapPhieu(FORM, WORDS, TYPED)).toEqual({ ho_ten: "name", dien_thoai: "phone" });
    expect(kiemNhapPhieu({ ...FORM, ho_ten: "Nguyễn Văn An", dien_thoai: "12345" }, WORDS, TYPED)).toEqual({
      dien_thoai: "invalid",
    });
    expect(kiemNhapPhieu({ ...FORM, ho_ten: "Nguyễn Văn An", dien_thoai: "0900 000 000" }, WORDS, TYPED)).toEqual({});
  });

  it("a plausible Vietnamese number: 10–11 digits from 0, or the same after +84", () => {
    for (const ok of ["0900000000", "09000000000", "+84900000000", "0900.000.000", "090-000-0000"]) {
      expect(isPlausiblePhone(ok), ok).toBe(true);
    }
    for (const bad of ["", "900000000", "12345", "090000000", "090000000000", "+8490000", "09a0000000", "84900000000"]) {
      expect(isPlausiblePhone(bad), bad).toBe(false);
    }
  });

  it("the commune app's body carries the TYPED name and number, `anonymous: false` even from an anonymous draft", () => {
    const form = { ...FORM, ho_ten: "Nguyễn Văn An", dien_thoai: "0900000000" };
    const typed = JSON.parse(sendBody(form, null, true)) as Record<string, unknown>;
    expect(typed).toMatchObject({ reporter_name: "Nguyễn Văn An", reporter_phone: "0900000000", anonymous: false });
    // Off the path the citizen's own switch still holds.
    const usual = JSON.parse(sendBody(form, null)) as Record<string, unknown>;
    expect(usual).toMatchObject({ reporter_name: "", reporter_phone: "", anonymous: true });
  });

  it("the shared app's check says the same in its own sentences", () => {
    const pa = { noi_dung: "x", dia_chi: "", ho_ten: "", dien_thoai: "", an_danh: true, field: "rac-thai" };
    expect(kiemPhanAnh(pa)).toBeNull();
    expect(kiemPhanAnh(pa, true)).toBe(TYPED_CONTACT.name_missing);
    expect(kiemPhanAnh({ ...pa, ho_ten: "A" }, true)).toBe(TYPED_CONTACT.phone_missing);
    expect(kiemPhanAnh({ ...pa, ho_ten: "A", dien_thoai: "123" }, true)).toBe(TYPED_CONTACT.phone_invalid);
    expect(kiemPhanAnh({ ...pa, ho_ten: "A", dien_thoai: "0900000000" }, true)).toBeNull();
  });
});

/* ─────────────────────────────── 429 and the code screen ─────────────────────────────── */

const SERVER_LIMIT_SENTENCE =
  "Hôm nay bạn đã gửi nhiều phản ánh khi chưa xác nhận số điện thoại nên phản ánh này CHƯA được ghi nhận.";

function stubFetch(status: number, body: unknown) {
  vi.stubGlobal("fetch", async () => ({ status, ok: status >= 200 && status < 300, json: async () => body }));
}

describe("429 `unverified_daily_limit` — the server's sentence, nothing recorded, the draft kept", () => {
  it("the call layer carries the server's sentence; the screen shows it, with no 'Gửi lại'", async () => {
    datPhienViGov({ token: "tok-test", ten_xa: COMMUNE, phone_verified: false });
    stubFetch(429, { code: "unverified_daily_limit", message: `  ${SERVER_LIMIT_SENTENCE}  `, trace_id: "t" });
    const kq = await guiPhanAnh(taoLanGui("{}"));
    expect(kq).toEqual({ kieu: "unverified-daily-limit", message: SERVER_LIMIT_SENTENCE });
    expect(sendOutcome(kq)).toEqual({ kind: "failed", failure: "unverified-daily-limit", message: SERVER_LIMIT_SENTENCE });
    expect(LOI_GUI["unverified-daily-limit"].co_the_gui_lai).toBe(false);
  });

  it("no sentence from the server → our own; another 429 on the send route stays a server fault", async () => {
    datPhienViGov({ token: "tok-test", ten_xa: COMMUNE, phone_verified: false });
    stubFetch(429, { code: "unverified_daily_limit" });
    expect(await guiPhanAnh(taoLanGui("{}"))).toEqual({ kieu: "unverified-daily-limit", message: null });
    expect(sendOutcome({ kieu: "unverified-daily-limit", message: null })).toEqual({
      kind: "failed",
      failure: "unverified-daily-limit",
    });
    stubFetch(429, { code: "rate_limited" });
    expect(await guiPhanAnh(taoLanGui("{}"))).toEqual({ kieu: "loi-may-chu" });
  });
});

const PETITION_OUT = {
  code: "PA7K2QX9M4TD",
  status: "da-tiep-nhan",
  field: "rac-thai",
  field_label: "Rác thải",
  content: "Rác tồn đọng",
  address: "",
  reporter_name: "Nguyễn V. A.",
  reporter_phone: "09****0000",
  anonymous: false,
  clock_from: "2026-10-08T01:30:00Z",
  acknowledge_due: null,
  resolve_due: null,
  result: "",
};

describe("201 with `contact_unverified` — the code screen says no notification will come", () => {
  it("read from the server's explicit field; absent or false is an ordinary petition; junk is out of shape", () => {
    expect(docPhieu({ ...PETITION_OUT, contact_unverified: true })?.contact_unverified).toBe(true);
    expect(docPhieu(PETITION_OUT)).not.toHaveProperty("contact_unverified");
    expect(docPhieu({ ...PETITION_OUT, contact_unverified: false })).not.toHaveProperty("contact_unverified");
    expect(docPhieu({ ...PETITION_OUT, contact_unverified: null })).not.toHaveProperty("contact_unverified");
    expect(docPhieu({ ...PETITION_OUT, contact_unverified: "yes" })).toBeNull();
  });

  it("commune app: the notice stands under the code; an ordinary petition has none", () => {
    const unverified = docPhieu({ ...PETITION_OUT, contact_unverified: true }) as PhieuCuaToi;
    const html = renderToStaticMarkup(createElement(SendDone, { petition: unverified, onFollow() {}, onHome() {} }));
    expect(html).toContain(XA_PA.done_no_notice);
    expect(html.indexOf(PETITION_OUT.code)).toBeLessThan(html.indexOf(XA_PA.done_no_notice));
    const usual = renderToStaticMarkup(
      createElement(SendDone, { petition: docPhieu(PETITION_OUT) as PhieuCuaToi, onFollow() {}, onHome() {} }),
    );
    expect(usual).not.toContain(XA_PA.done_no_notice);
  });

  it("shared app: the same notice, in its own voice", () => {
    const unverified = docPhieu({ ...PETITION_OUT, contact_unverified: true }) as PhieuCuaToi;
    expect(renderToStaticMarkup(createElement(KetQuaGui, { phieu: unverified, onGuiKhac() {} }))).toContain(
      TYPED_CONTACT.done_no_notice,
    );
  });

  it("the notice says what to do next, in plain words, no code", () => {
    for (const s of [XA_PA.done_no_notice, TYPED_CONTACT.done_no_notice]) {
      expect(s).toMatch(/KHÔNG nhận được thông báo/);
      expect(s).toMatch(/tra cứu theo mã/);
      expect(s).toMatch(/tài khoản Zalo/);
      expect(s).not.toMatch(/contact_unverified|\b4\d\d\b/);
    }
  });
});
