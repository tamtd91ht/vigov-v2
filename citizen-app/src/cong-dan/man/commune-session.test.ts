import { afterEach, describe, expect, it, vi } from "vitest";

import {
  type CommuneAppSessionResult,
  communeAppReopen,
  type OpenCommuneAppSession,
  openCommuneAppSession,
} from "../api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../api/phien-vigov";

import { createSessionGate, type SessionGateState, sessionGateMessage, type SessionGateStop } from "./commune-session";

/**
 * THE COMMUNE APP'S SESSION — opened at the first personal act, after the explanation, for the commune on
 * the header and no other.
 */

const COMMUNE = "Xã Thử Nghiệm";
const OK: CommuneAppSessionResult = { kieu: "xong", token: "tok-test", ten_xa: COMMUNE, da_xac_thuc_so: true };

afterEach(() => {
  datPhienViGov(null);
});

describe("openCommuneAppSession — stores only a usable session of the commune on screen", () => {
  it("same commune, bearer, verified phone → stored", async () => {
    expect(await openCommuneAppSession(async () => OK, COMMUNE)).toEqual({ kieu: "da-mo" });
    expect(layPhienViGov()).toEqual({ token: "tok-test", ten_xa: COMMUNE });
  });

  it("ANOTHER commune → khac-xa and NOTHING stored (a petition would reach the wrong commune)", async () => {
    expect(await openCommuneAppSession(async () => ({ ...OK, ten_xa: "Xã Khác" }), COMMUNE)).toEqual({ kieu: "khac-xa" });
    expect(layPhienViGov()).toBeNull();
  });

  it("no commune on screen → khac-xa, never a default", async () => {
    expect(await openCommuneAppSession(async () => OK, " ")).toEqual({ kieu: "khac-xa" });
    expect(layPhienViGov()).toBeNull();
  });

  it("phone not verified → chua-xac-thuc-so, not stored", async () => {
    expect(await openCommuneAppSession(async () => ({ ...OK, da_xac_thuc_so: false }), COMMUNE)).toEqual({
      kieu: "chua-xac-thuc-so",
    });
    expect(layPhienViGov()).toBeNull();
  });

  it("empty bearer → chua-mo; a throwing opener → thu-lai", async () => {
    expect(await openCommuneAppSession(async () => ({ ...OK, token: "" }), COMMUNE)).toEqual({ kieu: "chua-mo" });
    expect(
      await openCommuneAppSession(async () => {
        throw new Error("boom");
      }, COMMUNE),
    ).toEqual({ kieu: "thu-lai" });
    expect(layPhienViGov()).toBeNull();
  });

  it.each(["tu-choi", "chua-ket-noi", "tam-ngung", "cho-lat", "thu-lai", "ngoai-zalo"] as const)(
    "%s passes through, nothing stored",
    async (kieu) => {
      expect(await openCommuneAppSession(async () => ({ kieu }), COMMUNE)).toEqual({ kieu });
      expect(layPhienViGov()).toBeNull();
    },
  );
});

describe("communeAppReopen — the same opener serves the 403 `chua_xac_thuc_so` path", () => {
  it.each<[CommuneAppSessionResult, string]>([
    [OK, "xong"],
    [{ kieu: "tu-choi" }, "tu-choi"],
    [{ kieu: "thu-lai" }, "thu-lai"],
    [{ kieu: "cho-lat" }, "thu-lai"],
    [{ kieu: "ngoai-zalo" }, "ngoai-zalo"],
    [{ kieu: "chua-ket-noi" }, "chua-mo"],
    [{ kieu: "tam-ngung" }, "chua-mo"],
  ])("%j → %s", async (result, kieu) => {
    expect((await communeAppReopen(async () => result)()).kieu).toBe(kieu);
  });
});

function harness(open: OpenCommuneAppSession | undefined) {
  const states: (SessionGateState | null)[] = [];
  const gate = createSessionGate(open, () => COMMUNE, (s) => states.push(s));
  return { gate, states, last: () => states[states.length - 1] };
}

describe("createSessionGate — explain first, prompt only on the tap", () => {
  it("no session: require explains and calls NOTHING; allow opens, then runs the act exactly once", async () => {
    const open = vi.fn(async () => OK);
    const run = vi.fn();
    const { gate, last } = harness(open);
    gate.require(run);
    expect(last()).toEqual({ kieu: "hoi" });
    expect(open).not.toHaveBeenCalled();
    expect(run).not.toHaveBeenCalled();
    await gate.allow();
    expect(open).toHaveBeenCalledTimes(1);
    expect(run).toHaveBeenCalledTimes(1);
    expect(last()).toBeNull();
    // A second personal act reuses the session: no second dialog.
    gate.require(run);
    expect(open).toHaveBeenCalledTimes(1);
    expect(run).toHaveBeenCalledTimes(2);
  });

  it("decline → the refusal sentence, nothing called; asking again is allowed", async () => {
    const open = vi.fn(async () => OK);
    const { gate, last } = harness(open);
    gate.require(() => {});
    gate.decline();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "tu-choi" });
    expect(open).not.toHaveBeenCalled();
    gate.require(() => {});
    expect(last()).toEqual({ kieu: "hoi" });
  });

  it("a FINAL outcome (other commune) is not asked again, and the act never runs", async () => {
    const open = vi.fn(async (): Promise<CommuneAppSessionResult> => ({ ...OK, ten_xa: "Xã Khác" }));
    const run = vi.fn();
    const { gate, last } = harness(open);
    gate.require(run);
    await gate.allow();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "khac-xa" });
    gate.require(run);
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "khac-xa" });
    expect(open).toHaveBeenCalledTimes(1);
    expect(run).not.toHaveBeenCalled();
  });

  it("a retryable outcome keeps the act: the next tap on allow opens again and runs it", async () => {
    const open = vi
      .fn<OpenCommuneAppSession>()
      .mockResolvedValueOnce({ kieu: "cho-lat" })
      .mockResolvedValueOnce(OK);
    const run = vi.fn();
    const { gate, last } = harness(open);
    gate.require(run);
    await gate.allow();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "cho-lat" });
    await gate.allow();
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("no opener (tests, outside the shell) → chua-ket-noi at once, no network", () => {
    const { gate, last } = harness(undefined);
    gate.require(() => {});
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "chua-ket-noi" });
  });
});

describe("messages — plain Vietnamese, never a code, always a next step", () => {
  const STOPS: SessionGateStop[] = [
    "tu-choi",
    "thu-lai",
    "cho-lat",
    "tam-ngung",
    "chua-ket-noi",
    "khac-xa",
    "chua-xac-thuc-so",
    "chua-mo",
    "ngoai-zalo",
  ];
  it.each(STOPS)("%s", (outcome) => {
    const text = sessionGateMessage(outcome, "submit");
    expect(text).toContain("phản ánh chưa được gửi");
    expect(text).not.toMatch(/\b(4\d\d|5\d\d)\b|app_not_configured|chua_xac_thuc_so|undefined/);
    // A next step: an instruction ("Hãy …") or the always-available office/phone route ("vẫn có thể đến …").
    expect(text).toMatch(/Hãy |vẫn có thể đến/);
  });
});
