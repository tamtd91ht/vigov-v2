import { afterEach, describe, expect, it } from "vitest";

import {
  type CommuneAppSessionResult,
  openCommuneAppSession,
  reopenSessionWithPhone,
  type ZaloFailure,
} from "../api/mo-phien-vigov";
import { datPhienViGov } from "../api/phien-vigov";

import { createSessionGate, sessionGateMessage, type SessionGateState } from "./commune-session";
import {
  confirmCommuneZaloFailed,
  PHONE_VERIFICATION,
  PHONE_VERIFICATION_TASK,
  SEND_LOCATION_WORDS,
  XAC_NHAN_XA,
  XA_TN,
  zaloFailureSentence,
} from "./noi-dung";
import { COMMUNE_LOCATION_WORDS } from "./PhanAnhAppXa";
import { phoneVerificationMessage } from "./phone-verification";
import { locateOnce, sceneLocationFailureText } from "./scene-location";
import { afterNameAsk } from "./trai-nghiem";
import { buocSauMoPhien } from "./XacNhanXa";

/**
 * A PERMISSION ZALO REFUSED IS SAID AS THAT — which capability, and Zalo's own code (user rule 30/09/2026:
 * the test build is the build that ships). Before this, every such refusal read "chưa … vì mạng yếu", an
 * endless retry for something no retry can fix.
 *
 * What must hold, on every path of the state half (session code, phone, location, name):
 *   · the sentence names the capability and carries "mã lỗi <code>" verbatim
 *   · a code the SDK does not call "try again later" says it is NOT the network
 *   · no `zalo` → the old sentence, word for word (nothing measured, nothing invented)
 */

const PHONE: ZaloFailure = { capability: "phone", code: -1401, transient: false };
const LOCATION: ZaloFailure = { capability: "location", code: -1403, transient: false };
const NAME: ZaloFailure = { capability: "name", code: -2002, transient: false };
const ACCESS_SLOW: ZaloFailure = { capability: "access-token", code: -1408, transient: true };
const NOT_NETWORK = "Đây không phải lỗi mạng";

afterEach(() => {
  datPhienViGov(null);
});

describe("the Zalo sentence", () => {
  it("phone, not granted: names it, gives the code, says a retry will not change it", () => {
    const s = zaloFailureSentence(PHONE);
    expect(s).toContain("Zalo chưa cho phép ứng dụng lấy số điện thoại (mã lỗi -1401).");
    expect(s).toContain(NOT_NETWORK);
  });

  it("location and name", () => {
    expect(zaloFailureSentence(LOCATION)).toContain("Zalo chưa cho phép ứng dụng lấy vị trí (mã lỗi -1403).");
    expect(zaloFailureSentence(NAME)).toContain("Zalo chưa cho phép ứng dụng lấy tên Zalo (mã lỗi -2002).");
  });

  it("a 'try again later' code does not claim a missing permission", () => {
    const s = zaloFailureSentence(ACCESS_SLOW);
    expect(s).toContain("mã lỗi -1408");
    expect(s).not.toContain("chưa cho phép");
    expect(s).not.toContain(NOT_NETWORK);
  });

  it("name -1401 (index.d.ts:3213: the user declined the name) says both readings, never only 'Zalo refused'", () => {
    const s = zaloFailureSentence({ capability: "name", code: -1401, transient: false });
    expect(s).toContain("chưa đồng ý");
    expect(s).toContain("mã lỗi -1401");
  });
});

describe("phone — shared app (403 reopen) and commune app (session gate)", () => {
  it("the reopen sentence names Zalo's refusal and what to do next", () => {
    const s = phoneVerificationMessage("thu-lai", "submit", PHONE);
    expect(s).toContain("mã lỗi -1401");
    expect(s).toContain(PHONE_VERIFICATION_TASK.submit);
    expect(s).toContain("Bộ phận tiếp nhận");
    expect(s).not.toContain("mạng yếu");
  });

  it("the gate sentence too; without `zalo` both are the old sentences word for word", () => {
    expect(sessionGateMessage("thu-lai", "mine", PHONE)).toContain("mã lỗi -1401");
    expect(sessionGateMessage("thu-lai", "mine")).toBe(PHONE_VERIFICATION.retry(PHONE_VERIFICATION_TASK.mine));
    expect(phoneVerificationMessage("thu-lai", "lookup")).toBe(PHONE_VERIFICATION.retry(PHONE_VERIFICATION_TASK.lookup));
  });

  it("-201 is still the refusal sentence, with no code", () => {
    const s = sessionGateMessage("tu-choi", "submit");
    expect(s).toBe(PHONE_VERIFICATION.refused(PHONE_VERIFICATION_TASK.submit));
    expect(s).not.toContain("mã lỗi");
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

  it("stays on the confirmation with Zalo's sentence", () => {
    const step = buocSauMoPhien(xa, { kieu: "thu-lai", zalo: ACCESS_SLOW });
    expect(step).toEqual({
      trang: { kieu: "hoi", xa, cau_loi: confirmCommuneZaloFailed(zaloFailureSentence(ACCESS_SLOW), true) },
    });
    expect(JSON.stringify(step)).toContain("mã lỗi -1408");
  });

  it("without `zalo`: the old sentence", () => {
    expect(buocSauMoPhien(xa, { kieu: "thu-lai" })).toEqual({ trang: { kieu: "hoi", xa, cau_loi: XAC_NHAN_XA.thu_lai } });
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
      expect(s).toContain("mã lỗi -1403");
      expect(s).toContain("ô trên");
      expect(sceneLocationFailureText(words, "thu-lai", null)).toBe(words.failures["thu-lai"]);
      expect(sceneLocationFailureText(words, "tu-choi", null)).toBe(words.failures["tu-choi"]);
    }
  });
});

describe("name — commune app entry", () => {
  it("Zalo's refusal after the citizen agreed is kept, and said", () => {
    const state = afterNameAsk({ kieu: "khong-lay-duoc", zalo: NAME });
    expect(state).toEqual({ kind: "settled", name: null, zalo: NAME });
    expect(XA_TN.name_zalo_failed(zaloFailureSentence(NAME))).toContain("mã lỗi -2002");
  });

  it("the citizen's own refusal and an unmeasured failure stay quiet", () => {
    expect(afterNameAsk({ kieu: "tu-choi" })).toEqual({ kind: "settled", name: null });
    expect(afterNameAsk({ kieu: "khong-lay-duoc" })).toEqual({ kind: "settled", name: null });
  });
});
