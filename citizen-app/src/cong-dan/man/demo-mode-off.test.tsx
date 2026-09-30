import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

/**
 * THE OTHER HALF of `demo-mode.test.tsx`: the same pieces with the REAL constant — `false` in every build
 * except `deploy.mjs --vao-thang --demo`, and in every test run. Without `--demo` nothing may change: no
 * band, no sample name, no fake number, and a phone-step failure stops the act exactly as before.
 */

import type { CommuneAppSessionResult } from "../api/mo-phien-vigov";
import { datPhienViGov } from "../api/phien-vigov";
import { DEMO_BUILD } from "../../lib/demo-build";

import { createSessionGate, type SessionGateState } from "./commune-session";
import { DemoBand, demoPhonePrefill, withDemoName } from "./demo-mode";
import { createPhoneVerification, type PhoneVerificationState } from "./phone-verification";

afterEach(() => datPhienViGov(null));

describe("not a demo build (every build without `--demo`)", () => {
  it("the constant is false in tests, as in every ordinary build", () => {
    expect(DEMO_BUILD).toBe(false);
  });

  it("no band", () => {
    expect(renderToStaticMarkup(createElement(DemoBand))).toBe("");
  });

  it("no sample name: the entry state passes through untouched", () => {
    expect(withDemoName({ kind: "needs-consent" })).toEqual({ kind: "needs-consent" });
    expect(withDemoName({ kind: "settled", name: null })).toEqual({ kind: "settled", name: null });
    const zalo = { capability: "name", code: -1402, transient: false } as const;
    expect(withDemoName({ kind: "settled", name: null, zalo })).toEqual({ kind: "settled", name: null, zalo });
  });

  it("no fake number, even if a caller claims there is no session", () => {
    expect(demoPhonePrefill(true)).toBe("");
  });

  it("gate without the demo argument: Zalo's refusal is the error sentence, the act does not run", async () => {
    const states: Array<SessionGateState | null> = [];
    const open = vi.fn(
      async (): Promise<CommuneAppSessionResult> => ({
        kieu: "thu-lai",
        zalo: { capability: "phone", code: -1402, transient: false },
      }),
    );
    const gate = createSessionGate(open, () => "Xã Thử Nghiệm", (s) => void states.push(s));
    const run = vi.fn();
    gate.require(run);
    await gate.allow();
    expect(run).not.toHaveBeenCalled();
    expect(states.at(-1)).toEqual({
      kieu: "ket-qua",
      outcome: "thu-lai",
      zalo: { capability: "phone", code: -1402, transient: false },
    });
    expect(gate.demoWithoutSession()).toBe(false);
    gate.showDemoNotice();
    expect(states.at(-1)).toMatchObject({ kieu: "ket-qua" });
  });

  it("rating path: Zalo's refusal keeps the ordinary outcome, never the demo state", async () => {
    datPhienViGov({ token: "t1", ten_xa: "Xã Thử Nghiệm" });
    const states: Array<PhoneVerificationState | null> = [];
    const m = createPhoneVerification(async () => ({ kieu: "ngoai-zalo" }), (s) => void states.push(s));
    m.onPhoneRequired(() => {});
    await m.allow();
    expect(states.at(-1)).toEqual({ kieu: "ket-qua", outcome: "ngoai-zalo" });
  });
});
