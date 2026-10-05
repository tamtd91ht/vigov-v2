import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";

/**
 * EVERY OUTCOME OF THE COMMUNE APP'S SESSION GATE ENDS ON AN EXIT (report of 01/10/2026: on "Gửi phản ánh",
 * the gate stopped on a Zalo refusal and the citizen could not get out).
 *
 * What must hold, for the gate and the screen it drives:
 *   · every failure the opener can produce ends in `ket-qua`: a sentence, never `dang-mo` left standing;
 *   · a throwing / rejecting opener ends in `thu-lai`, never `dang-mo` forever;
 *   · "Về trang chủ" (`reset`) works at ANY time — also while an open is still running, which is the state
 *     where the old gate ignored it and every button was dead;
 *   · the result screen always carries "Về trang chủ", and a retry only where a new tap can help.
 */
import type { CommuneAppSessionResult, OpenCommuneAppSession } from "../api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../api/phien-vigov";

import { createSessionGate, type SessionGateState, type SessionGateStop } from "./commune-session";
import { COMMUNE_APP_SESSION, PHONE_VERIFICATION, XA_TN } from "./noi-dung";
import { SessionGateScreen } from "./TrangXa";

const COMMUNE = "Xã Thử Nghiệm";

const settle = () => new Promise((r) => setTimeout(r, 0));

function makeGate(open: OpenCommuneAppSession) {
  const states: Array<SessionGateState | null> = [];
  const gate = createSessionGate(open, () => COMMUNE, (s) => void states.push(s));
  return { gate, states };
}

/** The act asks first; the citizen's "Đồng ý chia sẻ số điện thoại" is what calls the opener (policy 3.3.4). */
function requireAndAllow(gate: ReturnType<typeof makeGate>["gate"], run: () => void) {
  gate.require(run);
  void gate.allow();
}

beforeEach(() => {
  datPhienViGov(null);
});

afterEach(() => {
  datPhienViGov(null);
});

/* ═══════════════════════════════════ 1. THE GATE ALONE ═══════════════════════════════════ */

describe("gate — asks first, never opens on its own", () => {
  it("`require` without a session shows the explanation and calls nothing", () => {
    const open = vi.fn<OpenCommuneAppSession>();
    const { gate, states } = makeGate(open);
    gate.require(() => {});
    expect(states.at(-1)).toEqual({ kieu: "hoi" });
    expect(open).not.toHaveBeenCalled();
  });
});

describe("gate — each failure the opener answers is a final screen, never `dang-mo`", () => {
  it.each<[string, CommuneAppSessionResult, SessionGateStop]>([
    ["network / server busy", { kieu: "thu-lai" }, "thu-lai"],
    ["wait a moment", { kieu: "cho-lat" }, "cho-lat"],
    ["channel paused", { kieu: "tam-ngung" }, "tam-ngung"],
    ["app not connected (also: App ID unknown)", { kieu: "chua-ket-noi" }, "chua-ket-noi"],
    ["outside Zalo", { kieu: "ngoai-zalo" }, "ngoai-zalo"],
  ])("%s → ket-qua %s, the act does not run", async (_, answer, outcome) => {
    const run = vi.fn();
    const { gate, states } = makeGate(async () => answer);
    requireAndAllow(gate, run);
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
    const { gate, states } = makeGate(open);
    requireAndAllow(gate, run);
    await settle();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome: "thu-lai" });
    await gate.allow(); // the retry button
    expect(run).toHaveBeenCalledTimes(1);
    expect(states.at(-1)).toBeNull();
  });

  it("a THROWING opener (synchronous throw) → thu-lai", async () => {
    const { gate, states } = makeGate(() => {
      throw new Error("boom");
    });
    requireAndAllow(gate, () => {});
    await settle();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome: "thu-lai" });
  });

  it("an opener that NEVER settles: 'Về trang chủ' still leaves — the state is cleared, not ignored", async () => {
    const { gate, states } = makeGate(() => new Promise<CommuneAppSessionResult>(() => {}));
    requireAndAllow(gate, () => {});
    await settle();
    expect(states.at(-1)).toEqual({ kieu: "dang-mo" });
    gate.reset();
    expect(states.at(-1)).toBeNull();
  });

  it("left while running, the late answer is not shown and does not run the act; the next act gets it", async () => {
    let finish: (r: CommuneAppSessionResult) => void = () => {};
    const open = vi.fn<OpenCommuneAppSession>(() => new Promise((r) => (finish = r)));
    const run = vi.fn();
    const { gate, states } = makeGate(open);
    requireAndAllow(gate, run);
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
    const { gate, states } = makeGate(open);
    requireAndAllow(gate, first);
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

/* ═══════════════════════════════════ 2. THE SCREEN ═══════════════════════════════════ */

function screen(state: SessionGateState) {
  return renderToStaticMarkup(
    createElement(SessionGateScreen, { state, task: "submit", onAllow() {}, onDecline() {}, onClose() {} }),
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

describe("SessionGateScreen `dang-mo` — a sentence and an exit, never only a status line", () => {
  it("carries 'Về trang chủ' under the working words", () => {
    const html = screen({ kieu: "dang-mo" });
    expect(html).toContain(COMMUNE_APP_SESSION.working);
    expect(html).toContain(`>${XA_TN.nut_ve_trang_chu}</button>`);
  });
});

describe("SessionGateScreen `hoi` — says where the number goes FOR THIS APP (ADR 0066)", () => {
  // The commune app's login goes straight to ViGov identity. The shared app's sentence names ViHAT Group's
  // server; showing it here told the citizen and Zalo's reviewer a route the number does not take.
  it("shows the commune app's sentence, never the shared app's", () => {
    const html = screen({ kieu: "hoi" });
    expect(html).toContain(COMMUNE_APP_SESSION.zalo_asks);
    expect(html).not.toContain(PHONE_VERIFICATION.zalo_asks);
    expect(html).not.toContain("ViHAT");
  });
});

describe("SessionGateScreen `ket-qua` — 'Về trang chủ' always, and no retry that cannot help", () => {
  it.each(ALL)("%s: 'Về trang chủ' always; the retry is the phone-sharing button", (outcome) => {
    const html = screen({ kieu: "ket-qua", outcome });
    expect(html).toContain(`>${XA_TN.nut_ve_trang_chu}</button>`);
    expect(html.includes(`>${PHONE_VERIFICATION.allow}</button>`)).toBe(RETRYABLE.has(outcome));
  });

  it("Zalo refused with a NON-transient code: the sentence says pressing again changes nothing, so no retry", () => {
    const zalo = { capability: "access-token" as const, code: -1402, transient: false };
    const html = screen({ kieu: "ket-qua", outcome: "thu-lai", zalo });
    expect(html).toContain(`>${XA_TN.nut_ve_trang_chu}</button>`);
    expect(html).not.toContain(`>${PHONE_VERIFICATION.allow}</button>`);
  });

  it("Zalo refused with a transient code: the retry stays", () => {
    const zalo = { capability: "access-token" as const, code: -1408, transient: true };
    expect(screen({ kieu: "ket-qua", outcome: "thu-lai", zalo })).toContain(`>${PHONE_VERIFICATION.allow}</button>`);
  });
});
