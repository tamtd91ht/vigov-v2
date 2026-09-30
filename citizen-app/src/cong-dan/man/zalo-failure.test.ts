import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it } from "vitest";

import {
  type CommuneAppSessionResult,
  openCommuneAppSession,
  reopenSessionWithPhone,
  type ZaloFailure,
} from "../api/mo-phien-vigov";
import { datPhienViGov } from "../api/phien-vigov";

import { createSessionGate, sessionGateMessage, type SessionGateState } from "./commune-session";
import { KhoiTrangThai } from "./khung-xa";
import {
  confirmCommuneZaloFailed,
  PHONE_VERIFICATION,
  PHONE_VERIFICATION_TASK,
  SEND_LOCATION_WORDS,
  XAC_NHAN_XA,
  XA_TN,
  zaloFailureSentence,
  zaloSupportCode,
} from "./noi-dung";
import { COMMUNE_LOCATION_WORDS } from "./PhanAnhAppXa";
import { PhoneVerificationPanel, phoneVerificationMessage } from "./phone-verification";
import { locateOnce, SceneLocationControl, sceneLocationFailureText } from "./scene-location";
import { afterNameAsk } from "./trai-nghiem";
import { buocSauMoPhien, ManXacNhanXa } from "./XacNhanXa";

/**
 * A PERMISSION ZALO REFUSED IS SAID AS THAT — which capability, in a plain sentence; Zalo's own code on a
 * separate secondary line "Mã hỗ trợ: <code>" (user decisions 30/09/2026: the test build is the build that
 * ships; the code leaves the main sentence). Before this, every such refusal read "chưa … vì mạng yếu", an
 * endless retry for something no retry can fix.
 *
 * What must hold, on every path of the state half (session code, phone, location, name):
 *   · the sentence names the capability and carries NO digit of the code
 *   · the secondary line carries exactly "Mã hỗ trợ: <code>", and is rendered under the sentence
 *   · a code the SDK does not call "try again later" says it is NOT the network
 *   · no `zalo` → the old sentence, word for word, and no secondary line
 */

const PHONE: ZaloFailure = { capability: "phone", code: -1401, transient: false };
const LOCATION: ZaloFailure = { capability: "location", code: -1403, transient: false };
const NAME: ZaloFailure = { capability: "name", code: -2002, transient: false };
const ACCESS_SLOW: ZaloFailure = { capability: "access-token", code: -1408, transient: true };
const NOT_NETWORK = "Đây không phải lỗi mạng";
const SUPPORT_LABEL = "Mã hỗ trợ";
const NO_DIGIT = /\d/;

/** The text of every `<p>` in rendered markup, in order. */
function paragraphs(html: string): string[] {
  return [...html.matchAll(/<p\b[^>]*>(.*?)<\/p>/g)].map((m) => m[1]!);
}

/** Rendered with a code: exactly one support line, exactly that code, after the sentence; no other line has it. */
function expectSupportLine(html: string, code: number, sentence: string) {
  const all = paragraphs(html);
  const support = all.filter((p) => p.startsWith(SUPPORT_LABEL));
  expect(support).toEqual([`${SUPPORT_LABEL}: ${code}`]);
  for (const p of all.filter((p) => !p.startsWith(SUPPORT_LABEL))) expect(p).not.toContain(String(Math.abs(code)));
  expect(all.indexOf(sentence)).toBeGreaterThanOrEqual(0);
  expect(all.indexOf(support[0]!)).toBeGreaterThan(all.indexOf(sentence));
}

afterEach(() => {
  datPhienViGov(null);
});

describe("the Zalo sentence", () => {
  it("phone, not granted: names it, says a retry will not change it, carries no code", () => {
    const s = zaloFailureSentence(PHONE);
    expect(s).toContain("Zalo chưa cho phép ứng dụng lấy số điện thoại.");
    expect(s).toContain(NOT_NETWORK);
    expect(s).toContain("bấm lại ngay cũng không khác");
    expect(s).not.toMatch(NO_DIGIT);
  });

  it("location and name", () => {
    expect(zaloFailureSentence(LOCATION)).toContain("Zalo chưa cho phép ứng dụng lấy vị trí.");
    expect(zaloFailureSentence(NAME)).toContain("Zalo chưa cho phép ứng dụng lấy tên Zalo.");
    expect(zaloFailureSentence(LOCATION)).not.toMatch(NO_DIGIT);
    expect(zaloFailureSentence(NAME)).not.toMatch(NO_DIGIT);
  });

  it("a 'try again later' code does not claim a missing permission", () => {
    const s = zaloFailureSentence(ACCESS_SLOW);
    expect(s).not.toMatch(NO_DIGIT);
    expect(s).not.toContain("chưa cho phép");
    expect(s).not.toContain(NOT_NETWORK);
  });

  it("name -1401 (index.d.ts:3213: the user declined the name) says both readings, never only 'Zalo refused'", () => {
    const s = zaloFailureSentence({ capability: "name", code: -1401, transient: false });
    expect(s).toContain("chưa đồng ý");
    expect(s).toContain("Zalo chưa cho phép");
    expect(s).not.toMatch(NO_DIGIT);
  });

  it("the support line is exactly the code; no failure, no line", () => {
    expect(zaloSupportCode(PHONE)).toBe("Mã hỗ trợ: -1401");
    expect(zaloSupportCode(ACCESS_SLOW)).toBe("Mã hỗ trợ: -1408");
    expect(zaloSupportCode(null)).toBeNull();
    expect(zaloSupportCode(undefined)).toBeNull();
  });
});

describe("phone — shared app (403 reopen) and commune app (session gate)", () => {
  it("the reopen sentence names Zalo's refusal and what to do next, with no code", () => {
    const s = phoneVerificationMessage("thu-lai", "submit", PHONE);
    expect(s).toContain("Zalo chưa cho phép ứng dụng lấy số điện thoại");
    expect(s).not.toMatch(NO_DIGIT);
    expect(s).toContain(PHONE_VERIFICATION_TASK.submit);
    expect(s).toContain("Bộ phận tiếp nhận");
    expect(s).not.toContain("mạng yếu");
  });

  it("the gate sentence too; without `zalo` both are the old sentences word for word", () => {
    const s = sessionGateMessage("thu-lai", "mine", PHONE);
    expect(s).toContain("Zalo chưa cho phép ứng dụng lấy số điện thoại");
    expect(s).not.toMatch(NO_DIGIT);
    expect(sessionGateMessage("thu-lai", "mine")).toBe(PHONE_VERIFICATION.retry(PHONE_VERIFICATION_TASK.mine));
    expect(phoneVerificationMessage("thu-lai", "lookup")).toBe(PHONE_VERIFICATION.retry(PHONE_VERIFICATION_TASK.lookup));
  });

  it("-201 is still the refusal sentence, with no code", () => {
    const s = sessionGateMessage("tu-choi", "submit");
    expect(s).toBe(PHONE_VERIFICATION.refused(PHONE_VERIFICATION_TASK.submit));
    expect(s).not.toContain(SUPPORT_LABEL);
  });

  it("the panel renders the support line under the sentence; without `zalo`, no line", () => {
    const panel = (zalo?: ZaloFailure) =>
      renderToStaticMarkup(
        createElement(PhoneVerificationPanel, {
          state: zalo === undefined ? { kieu: "ket-qua", outcome: "thu-lai" } : { kieu: "ket-qua", outcome: "thu-lai", zalo },
          task: "submit",
          onAllow: () => {},
          onDecline: () => {},
        }),
      );
    expectSupportLine(panel(PHONE), -1401, phoneVerificationMessage("thu-lai", "submit", PHONE));
    expect(panel()).not.toContain(SUPPORT_LABEL);
  });

  it("the commune gate's status block renders it the same way; `null` renders nothing", () => {
    const sentence = sessionGateMessage("thu-lai", "mine", PHONE);
    const block = (support_code: string | null) =>
      renderToStaticMarkup(createElement(KhoiTrangThai, { bieu_tuong: "alert", loi: true, cau: sentence, support_code }));
    expectSupportLine(block(zaloSupportCode(PHONE)), -1401, sentence);
    expect(block(null)).not.toContain(SUPPORT_LABEL);
  });

  it("the reopen keeps Zalo's answer on its way to the screen", async () => {
    datPhienViGov({ token: "tok-test", ten_xa: "Xã Thử Nghiệm" });
    expect(await reopenSessionWithPhone(async () => ({ kieu: "thu-lai", zalo: PHONE }))).toEqual({
      kieu: "thu-lai",
      zalo: PHONE,
    });
  });

  it("the gate state carries it, and a tap is still offered (the sentence, not the button, says it will not help)", async () => {
    const states: (SessionGateState | null)[] = [];
    const open = async (): Promise<CommuneAppSessionResult> => ({ kieu: "thu-lai", zalo: PHONE });
    expect(await openCommuneAppSession(open, "Xã Thử Nghiệm")).toEqual({ kieu: "thu-lai", zalo: PHONE });
    const gate = createSessionGate(open, () => "Xã Thử Nghiệm", (s) => states.push(s));
    gate.require(() => undefined);
    await gate.allow();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome: "thu-lai", zalo: PHONE });
  });
});

describe("session code after the commune is confirmed (shared app)", () => {
  const xa = { ten: "Xã Thử Nghiệm", tinh: "Tỉnh Thử" };
  const render = (step: ReturnType<typeof buocSauMoPhien>) => {
    if (!("trang" in step)) throw new Error("expected the confirmation page");
    return renderToStaticMarkup(
      createElement(ManXacNhanXa, { trang: step.trang, nguon: "qr", onXacNhan: () => {}, onKhongPhai: () => {} }),
    );
  };

  it("stays on the confirmation with Zalo's sentence, and the code on its own line", () => {
    const step = buocSauMoPhien(xa, { kieu: "thu-lai", zalo: ACCESS_SLOW });
    const sentence = confirmCommuneZaloFailed(zaloFailureSentence(ACCESS_SLOW), true);
    expect(step).toEqual({ trang: { kieu: "hoi", xa, cau_loi: sentence, support_code: "Mã hỗ trợ: -1408" } });
    expect(sentence).not.toMatch(NO_DIGIT);
    expectSupportLine(render(step), -1408, sentence);
  });

  it("without `zalo`: the old sentence, no support line", () => {
    const step = buocSauMoPhien(xa, { kieu: "thu-lai" });
    expect(step).toEqual({ trang: { kieu: "hoi", xa, cau_loi: XAC_NHAN_XA.thu_lai } });
    expect(render(step)).not.toContain(SUPPORT_LABEL);
  });
});

describe("location — both apps", () => {
  it("the tap keeps Zalo's answer", async () => {
    expect(await locateOnce(async () => ({ kind: "thu-lai", zalo: LOCATION }))).toEqual({
      location: null,
      failure: "thu-lai",
      zalo: LOCATION,
    });
  });

  it("the line names it and still offers the address box; without `zalo`, the old line", () => {
    for (const words of [SEND_LOCATION_WORDS, COMMUNE_LOCATION_WORDS]) {
      const s = sceneLocationFailureText(words, "thu-lai", LOCATION);
      expect(s).toContain("Zalo chưa cho phép ứng dụng lấy vị trí");
      expect(s).not.toMatch(NO_DIGIT);
      expect(s).toContain("ô trên");
      expect(sceneLocationFailureText(words, "thu-lai", null)).toBe(words.failures["thu-lai"]);
      expect(sceneLocationFailureText(words, "tu-choi", null)).toBe(words.failures["tu-choi"]);
    }
  });

  it("the control renders the support line under the sentence in both looks; without `zalo`, none", () => {
    for (const [words, look] of [
      [SEND_LOCATION_WORDS, "shared"],
      [COMMUNE_LOCATION_WORDS, "commune"],
    ] as const) {
      const control = (zalo: ZaloFailure | null) =>
        renderToStaticMarkup(
          createElement(SceneLocationControl, {
            words,
            look,
            locating: false,
            location: null,
            failure: "thu-lai",
            zalo,
            onLocate: () => {},
          }),
        );
      expectSupportLine(control(LOCATION), -1403, sceneLocationFailureText(words, "thu-lai", LOCATION));
      expect(control(null)).not.toContain(SUPPORT_LABEL);
    }
  });
});

describe("name — commune app entry", () => {
  it("Zalo's refusal after the citizen agreed is kept, and said without the code", () => {
    const state = afterNameAsk({ kieu: "khong-lay-duoc", zalo: NAME });
    expect(state).toEqual({ kind: "settled", name: null, zalo: NAME });
    const s = XA_TN.name_zalo_failed(zaloFailureSentence(NAME));
    expect(s).toContain("Zalo chưa cho phép ứng dụng lấy tên Zalo");
    expect(s).not.toMatch(NO_DIGIT);
    expect(zaloSupportCode(NAME)).toBe("Mã hỗ trợ: -2002");
  });

  it("the citizen's own refusal and an unmeasured failure stay quiet", () => {
    expect(afterNameAsk({ kieu: "tu-choi" })).toEqual({ kind: "settled", name: null });
    expect(afterNameAsk({ kieu: "khong-lay-duoc" })).toEqual({ kind: "settled", name: null });
  });
});
