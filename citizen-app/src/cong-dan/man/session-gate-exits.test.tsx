import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * EVERY OUTCOME OF THE COMMUNE APP'S SESSION GATE ENDS ON AN EXIT (report of 01/10/2026: the `--demo` build,
 * on "Gửi phản ánh", stopped on a Zalo refusal and the citizen could not get out).
 *
 * What must hold, for the `--demo` build's gate (`openAtOnce`) — and the screen it drives:
 *   · every failure the opener can produce ends in `ket-qua`: a sentence, never `dang-mo` left standing;
 *   · a throwing / rejecting opener ends in `thu-lai`, never `dang-mo` forever;
 *   · "Về trang chủ" (`reset`) works at ANY time — also while an open is still running, which is the state
 *     where the old gate ignored it and every button was dead;
 *   · the result screen always carries "Về trang chủ", and "Thử lại" only where a new tap can help;
 *   · no "Đồng ý chia sẻ số điện thoại" in that build — nothing is shared there — and no "demo" word.
 *
 * `zmp-sdk` is replaced because the real module needs a Zalo runtime; every Zalo call is recorded so the
 * composed path (real `openCommuneAppSessionWithDemoIdentity` → real App.tsx table → real gate) can prove it
 * asks Zalo for nothing.
 */
const sdk = vi.hoisted(() => ({
  getAccessToken: vi.fn<() => Promise<string>>(),
  getPhoneNumber: vi.fn<() => Promise<{ token?: string }>>(),
  getUserInfo: vi.fn<() => Promise<{ userInfo: { name?: string } }>>(),
}));
vi.mock("zmp-sdk", () => sdk);

import { toCommuneAppSessionResult } from "../../App";
import { openCommuneAppSessionWithDemoIdentity } from "../../features/dang-nhap/cau-vigov";
import { openCommuneAppDemoSessionCall } from "../../features/dang-nhap/goi-may-chu";
import type { CommuneAppSessionResult, OpenCommuneAppSession } from "../api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../api/phien-vigov";

import { createSessionGate, type SessionGateState, type SessionGateStop } from "./commune-session";
import { COMMUNE_APP_SESSION, PHONE_VERIFICATION, XA_TN } from "./noi-dung";
import { SessionGateScreen } from "./TrangXa";

const COMMUNE = "Xã Thử Nghiệm";
const APP_ID = "1234567890123456789";
const ADDRESS = "https://mini.vidu.vn/api/v1/sessions";
const BANNED = /demo|trình diễn|trải nghiệm/i;

const settle = () => new Promise((r) => setTimeout(r, 0));

function gateAtOnce(open: OpenCommuneAppSession) {
  const states: Array<SessionGateState | null> = [];
  const gate = createSessionGate(open, () => COMMUNE, (s) => void states.push(s), true);
  return { gate, states };
}

beforeEach(() => {
  datPhienViGov(null);
  sdk.getAccessToken.mockReset().mockRejectedValue(Object.assign(new Error("x"), { code: -1402 }));
  sdk.getPhoneNumber.mockReset().mockRejectedValue(Object.assign(new Error("x"), { code: -1402 }));
  sdk.getUserInfo.mockReset().mockRejectedValue(Object.assign(new Error("x"), { code: -1402 }));
});

afterEach(() => {
  vi.unstubAllGlobals();
  datPhienViGov(null);
});

/* ═══════════════════════════════════ 1. THE GATE ALONE ═══════════════════════════════════ */

describe("gate in `openAtOnce` mode — each failure the opener answers is a final screen, never `dang-mo`", () => {
  it.each<[string, CommuneAppSessionResult, SessionGateStop]>([
    ["network / server busy", { kieu: "thu-lai" }, "thu-lai"],
    ["wait a moment", { kieu: "cho-lat" }, "cho-lat"],
    ["channel paused", { kieu: "tam-ngung" }, "tam-ngung"],
    ["app not connected (also: App ID unknown)", { kieu: "chua-ket-noi" }, "chua-ket-noi"],
    ["outside Zalo", { kieu: "ngoai-zalo" }, "ngoai-zalo"],
  ])("%s → ket-qua %s, the act does not run", async (_, answer, outcome) => {
    const run = vi.fn();
    const { gate, states } = gateAtOnce(async () => answer);
    gate.require(run);
    await settle();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome });
    expect(run).not.toHaveBeenCalled();
    expect(layPhienViGov()).toBeNull();
    gate.reset(); // "Về trang chủ"
    expect(states.at(-1)).toBeNull();
  });

  it("a REJECTING opener → thu-lai, not `dang-mo` forever; the retry can open again", async () => {
    const open = vi.fn<OpenCommuneAppSession>().mockRejectedValueOnce(new Error("boom"));
    open.mockResolvedValueOnce({ kieu: "xong", token: "t", ten_xa: COMMUNE, da_xac_thuc_so: true });
    const run = vi.fn();
    const { gate, states } = gateAtOnce(open);
    gate.require(run);
    await settle();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome: "thu-lai" });
    await gate.allow(); // "Thử lại"
    expect(run).toHaveBeenCalledTimes(1);
    expect(states.at(-1)).toBeNull();
  });

  it("a THROWING opener (synchronous throw) → thu-lai", async () => {
    const { gate, states } = gateAtOnce(() => {
      throw new Error("boom");
    });
    gate.require(() => {});
    await settle();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome: "thu-lai" });
  });

  it("an opener that NEVER settles: 'Về trang chủ' still leaves — the state is cleared, not ignored", async () => {
    const { gate, states } = gateAtOnce(() => new Promise<CommuneAppSessionResult>(() => {}));
    gate.require(() => {});
    await settle();
    expect(states.at(-1)).toEqual({ kieu: "dang-mo" });
    gate.reset();
    expect(states.at(-1)).toBeNull();
  });

  it("left while running, the late answer is not shown and does not run the act; the next act gets it", async () => {
    let finish: (r: CommuneAppSessionResult) => void = () => {};
    const open = vi.fn<OpenCommuneAppSession>(() => new Promise((r) => (finish = r)));
    const run = vi.fn();
    const { gate, states } = gateAtOnce(open);
    gate.require(run);
    gate.reset();
    finish({ kieu: "chua-ket-noi" });
    await settle();
    expect(states.at(-1)).toBeNull(); // the citizen is on the home screen and stays there
    expect(run).not.toHaveBeenCalled();
    gate.require(run); // the next personal act says the final sentence without asking again
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome: "chua-ket-noi" });
    expect(open).toHaveBeenCalledTimes(1);
  });

  it("came back while the first open still runs: waits for THAT one (one exchange), then runs the act", async () => {
    let finish: (r: CommuneAppSessionResult) => void = () => {};
    const open = vi.fn<OpenCommuneAppSession>(() => new Promise((r) => (finish = r)));
    const first = vi.fn();
    const second = vi.fn();
    const { gate, states } = gateAtOnce(open);
    gate.require(first);
    gate.reset();
    gate.require(second);
    expect(states.at(-1)).toEqual({ kieu: "dang-mo" });
    finish({ kieu: "xong", token: "t", ten_xa: COMMUNE, da_xac_thuc_so: true });
    await settle();
    expect(open).toHaveBeenCalledTimes(1);
    expect(first).not.toHaveBeenCalled();
    expect(second).toHaveBeenCalledTimes(1);
    expect(states.at(-1)).toBeNull();
  });
});

/* ═══════════════ 2. THE COMPOSED `--demo` PATH: real opener → real App.tsx table → gate ═══════════════ */

describe("the `--demo` opener through the shell table and the gate — Zalo never asked, every end has an exit", () => {
  function demoOpen(readAppId: () => string | null): OpenCommuneAppSession {
    // The host the state half hands in is real; the call is pinned to the test's ADDRESS.
    return async (identityHost) =>
      toCommuneAppSessionResult(
        await openCommuneAppSessionWithDemoIdentity(identityHost, readAppId, (req) =>
          openCommuneAppDemoSessionCall(req, ADDRESS),
        ),
      );
  }

  function stubFetch(answer: { status: number } | Error) {
    vi.stubGlobal("fetch", () =>
      answer instanceof Error
        ? Promise.reject(answer)
        : Promise.resolve({ status: answer.status, ok: false, json: async () => ({}) }),
    );
  }

  it.each<[string, () => string | null, { status: number } | Error, SessionGateStop]>([
    ["App ID unknown (khong-ro-app)", () => null, { status: 201 }, "chua-ket-noi"],
    ["server does not know the body (400)", () => APP_ID, { status: 400 }, "tam-ngung"],
    ["App ID not a demo app (422)", () => APP_ID, { status: 422 }, "chua-ket-noi"],
    ["server busy (502)", () => APP_ID, { status: 502 }, "cho-lat"],
    ["session bridge off (503)", () => APP_ID, { status: 503 }, "tam-ngung"],
    ["network error", () => APP_ID, new Error("offline"), "thu-lai"],
  ])("%s → ket-qua %s", async (_, readAppId, answer, outcome) => {
    stubFetch(answer);
    const { gate, states } = gateAtOnce(demoOpen(readAppId));
    gate.require(() => {});
    await settle();
    await settle();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome });
    expect(sdk.getAccessToken).not.toHaveBeenCalled();
    expect(sdk.getPhoneNumber).not.toHaveBeenCalled();
    expect(sdk.getUserInfo).not.toHaveBeenCalled();
  });
});

/* ═══════════════════════════════════ 3. THE SCREEN ═══════════════════════════════════ */

function screen(state: SessionGateState, atOnce: boolean) {
  return renderToStaticMarkup(
    createElement(SessionGateScreen, { state, task: "submit", atOnce, onAllow() {}, onDecline() {}, onClose() {} }),
  );
}

const ALL: SessionGateStop[] = [
  "thu-lai",
  "cho-lat",
  "tam-ngung",
  "chua-ket-noi",
  "ngoai-zalo",
  "tu-choi",
  "khac-xa",
  "chua-mo",
  "chua-xac-thuc-so",
];
const RETRYABLE = new Set<SessionGateStop>(["thu-lai", "cho-lat", "tam-ngung"]);

describe("SessionGateScreen in the `--demo` build — a sentence and an exit, never only a status line", () => {
  it("`dang-mo` carries 'Về trang chủ' under the working words", () => {
    const html = screen({ kieu: "dang-mo" }, true);
    expect(html).toContain(COMMUNE_APP_SESSION.working);
    expect(html).toContain(`>${XA_TN.nut_ve_trang_chu}</button>`);
  });

  it.each(ALL)("%s: 'Về trang chủ' always; 'Thử lại' exactly where a tap can help; no phone-sharing button", (outcome) => {
    const html = screen({ kieu: "ket-qua", outcome }, true);
    expect(html).toContain(`>${XA_TN.nut_ve_trang_chu}</button>`);
    expect(html.includes(`>${COMMUNE_APP_SESSION.retry}</button>`)).toBe(RETRYABLE.has(outcome));
    expect(html).not.toContain(`>${PHONE_VERIFICATION.allow}</button>`);
    expect(html).not.toContain(`“${PHONE_VERIFICATION.allow}”`); // no sentence naming a button that is not there
    expect(html).not.toMatch(BANNED);
  });
});

describe("SessionGateScreen `hoi` — says where the number goes FOR THIS APP (ADR 0066)", () => {
  // The commune app's login goes straight to ViGov identity. The shared app's sentence names ViHAT Group's
  // server; showing it here told the citizen and Zalo's reviewer a route the number does not take.
  it("shows the commune app's sentence, never the shared app's", () => {
    const html = screen({ kieu: "hoi" }, false);
    expect(html).toContain(COMMUNE_APP_SESSION.zalo_asks);
    expect(html).not.toContain(PHONE_VERIFICATION.zalo_asks);
    expect(html).not.toContain("ViHAT");
  });
});

describe("SessionGateScreen in every other build — unchanged words, and no retry that cannot help", () => {
  it.each(ALL)("%s: 'Về trang chủ' always; the retry is the phone-sharing button", (outcome) => {
    const html = screen({ kieu: "ket-qua", outcome }, false);
    expect(html).toContain(`>${XA_TN.nut_ve_trang_chu}</button>`);
    expect(html.includes(`>${PHONE_VERIFICATION.allow}</button>`)).toBe(RETRYABLE.has(outcome));
  });

  it("Zalo refused with a NON-transient code: the sentence says pressing again changes nothing, so no retry", () => {
    const zalo = { capability: "access-token" as const, code: -1402, transient: false };
    const html = screen({ kieu: "ket-qua", outcome: "thu-lai", zalo }, false);
    expect(html).toContain(`>${XA_TN.nut_ve_trang_chu}</button>`);
    expect(html).not.toContain(`>${PHONE_VERIFICATION.allow}</button>`);
  });

  it("Zalo refused with a transient code: the retry stays", () => {
    const zalo = { capability: "access-token" as const, code: -1408, transient: true };
    expect(screen({ kieu: "ket-qua", outcome: "thu-lai", zalo }, false)).toContain(`>${PHONE_VERIFICATION.allow}</button>`);
  });
});
