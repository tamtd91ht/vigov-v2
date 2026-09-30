import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * THE OTHER HALF of `demo-mode.test.tsx`: the same pieces with the REAL constant — `false` in every build
 * except `deploy.mjs --vao-thang --demo`, and in every test run. Without `--demo` nothing may change from
 * before the flag existed: the Zalo name is asked for, the explanation comes before Zalo's phone dialog, the
 * phone field starts empty — and no screen carries a demo word (owner 01/10/2026: in no build at all).
 */

import type { CommuneAppSessionResult } from "../api/mo-phien-vigov";
import { datPhienViGov } from "../api/phien-vigov";
import { DEMO_BUILD } from "../../lib/demo-build";

import { createSessionGate, type SessionGateState } from "./commune-session";
import { blankForm, CommuneSendScreen } from "./PhanAnhAppXa";
import { TrangXa } from "./TrangXa";

const BANNED = /demo|trình diễn|trải nghiệm/i;

afterEach(() => datPhienViGov(null));

describe("not a `--demo` build (every build without the flag)", () => {
  it("the constant is false in tests, as in every ordinary build", () => {
    expect(DEMO_BUILD).toBe(false);
  });

  it("the phone field starts empty — no fixed number", () => {
    expect(blankForm("Trần Thị Bình").dien_thoai).toBe(""); // invented test data
  });

  it("the gate explains FIRST and asks Zalo nothing before the tap (policy 3.3.4)", async () => {
    const states: Array<SessionGateState | null> = [];
    const open = vi.fn(async (): Promise<CommuneAppSessionResult> => ({ kieu: "tu-choi" }));
    const gate = createSessionGate(open, () => "Xã Thử Nghiệm", (s) => void states.push(s));
    gate.require(() => {});
    expect(states.at(-1)).toEqual({ kieu: "hoi" });
    expect(open).not.toHaveBeenCalled();
    await gate.allow();
    expect(open).toHaveBeenCalledTimes(1);
  });

  it("Zalo's refusal is the ordinary error sentence; the act does not run", async () => {
    const states: Array<SessionGateState | null> = [];
    const zalo = { capability: "phone", code: -1402, transient: false } as const;
    const gate = createSessionGate(
      async (): Promise<CommuneAppSessionResult> => ({ kieu: "thu-lai", zalo }),
      () => "Xã Thử Nghiệm",
      (s) => void states.push(s),
    );
    const run = vi.fn();
    gate.require(run);
    await gate.allow();
    expect(run).not.toHaveBeenCalled();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome: "thu-lai", zalo });
  });

  it("no demo word on the loading screen or the send screen", () => {
    expect(renderToStaticMarkup(createElement(TrangXa, { ten_mien: "thu.vigov.vn" }))).not.toMatch(BANNED);
    const send = renderToStaticMarkup(
      createElement(CommuneSendScreen, {
        ten_xa: "Xã Thử Nghiệm",
        ho_ten: null,
        onBack: () => {},
        onSessionLost: () => {},
        onSent: () => {},
        onOpenPetition: () => {},
      }),
    );
    expect(send).not.toMatch(BANNED);
  });
});
