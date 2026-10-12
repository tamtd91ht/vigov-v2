import { afterEach, describe, expect, it, vi } from "vitest";

import type { CommuneAppSessionResult, OpenCommuneAppSession, ReopenWithPhone } from "../api/mo-phien-vigov";
import { datPhienViGov, layPhienViGov } from "../api/phien-vigov";

import { createSessionGate, type ReadPhoneGrant, type SessionGateState } from "./commune-session";
import type { PhoneVerificationTask } from "./noi-dung";
import { createPhoneVerification, type PhoneVerificationState } from "./phone-verification";

/**
 * Customer sheet 12/10/2026 row 7 ("mỗi lần vào lại app phải request lại quyền chia sẻ số điện thoại mới xem được
 * danh sách phản ánh"); owner: reopen silently when Zalo already holds the permission. The gate's two new paths —
 * `openIfGranted` at app start, `explicitTap` on "Phản ánh của tôi" — and the fail-closed answer to every doubt.
 * The mounted app is exercised in `hieu-ung-khong-phien.test.tsx`; this file steps the pure gate.
 */

const COMMUNE = "Xã Thử Nghiệm";
const OK: CommuneAppSessionResult = { kieu: "xong", token: "tok-test", ten_xa: COMMUNE, da_xac_thuc_so: true };

afterEach(() => {
  datPhienViGov(null);
});

function harness(open: OpenCommuneAppSession | undefined, readGrant?: ReadPhoneGrant) {
  const states: (SessionGateState | null)[] = [];
  const gate = createSessionGate(open, () => COMMUNE, (s) => states.push(s), readGrant);
  return { gate, states, last: () => states[states.length - 1] };
}

const flush = () => new Promise((r) => setTimeout(r, 0));

describe("openIfGranted — the silent reopen at app start", () => {
  it("granted → the verified session opens with `ask`, no screen is shown, answers true", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const { gate, states } = harness(open, async () => true);
    expect(await gate.openIfGranted()).toBe(true);
    expect(open.mock.calls.map((c) => c[1])).toEqual(["ask"]);
    expect(layPhienViGov()).toEqual({ token: "tok-test", ten_xa: COMMUNE, phone_verified: true });
    expect(states).toEqual([]);
  });

  it("not granted → nothing called, nothing shown, false", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const { gate, states } = harness(open, async () => false);
    expect(await gate.openIfGranted()).toBe(false);
    expect(open).not.toHaveBeenCalled();
    expect(states).toEqual([]);
  });

  it("the read throws → as not granted", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const { gate, states } = harness(open, async () => {
      throw new Error("getSetting");
    });
    expect(await gate.openIfGranted()).toBe(false);
    expect(open).not.toHaveBeenCalled();
    expect(states).toEqual([]);
  });

  it("no reader injected (the QR path), or no opener → never reads, never opens", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    expect(await harness(open).gate.openIfGranted()).toBe(false);
    const read = vi.fn(async () => true);
    expect(await harness(undefined, read).gate.openIfGranted()).toBe(false);
    expect(open).not.toHaveBeenCalled();
    expect(read).not.toHaveBeenCalled();
  });

  it("a session already exists → nothing to reopen", async () => {
    datPhienViGov({ token: "tok-old", ten_xa: COMMUNE, phone_verified: true });
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    expect(await harness(open, async () => true).gate.openIfGranted()).toBe(false);
    expect(open).not.toHaveBeenCalled();
  });

  it.each<[string, CommuneAppSessionResult]>([
    ["refused", { kieu: "tu-choi" }],
    ["another commune", { ...OK, ten_xa: "Xã Khác" }],
    ["phone-less", { ...OK, da_xac_thuc_so: false }],
    ["Zalo code", { kieu: "thu-lai", zalo: { capability: "access-token", code: -1401, transient: false } }],
    ["not connected", { kieu: "chua-ket-noi" }],
  ])("%s → false, NOTHING stored, nothing shown, nothing remembered: the next tap meets today's flow", async (_, result) => {
    const open = vi.fn<OpenCommuneAppSession>(async () => result);
    // Granted for the silent try only, so the tap below takes the explanation path — no outcome was remembered.
    let granted = true;
    const { gate, states, last } = harness(open, async () => granted);
    expect(await gate.openIfGranted()).toBe(false);
    expect(layPhienViGov()).toBeNull();
    expect(states).toEqual([]);
    granted = false;
    gate.require(() => {}, "mine", { explicitTap: true });
    await flush();
    expect(last()).toEqual({ kieu: "hoi" });
  });

  it("a tap DURING the silent reopen waits on the waiting screen, then runs on the session it opened", async () => {
    let answer: (r: CommuneAppSessionResult) => void = () => {};
    const open = vi.fn<OpenCommuneAppSession>(() => new Promise((done) => (answer = done)));
    const run = vi.fn();
    const { gate, last } = harness(open, async () => true);
    const silent = gate.openIfGranted();
    await flush();
    gate.require(run, "mine", { explicitTap: true });
    expect(last()).toEqual({ kieu: "dang-mo" });
    expect(open).toHaveBeenCalledTimes(1);
    answer(OK);
    expect(await silent).toBe(true);
    expect(run).toHaveBeenCalledTimes(1);
    expect(last()).toBeNull();
    expect(open).toHaveBeenCalledTimes(1);
  });

  it("a tap during it, then 'Về trang chủ': the act is dropped, the session still kept", async () => {
    let answer: (r: CommuneAppSessionResult) => void = () => {};
    const open = vi.fn<OpenCommuneAppSession>(() => new Promise((done) => (answer = done)));
    const run = vi.fn();
    const { gate } = harness(open, async () => true);
    const silent = gate.openIfGranted();
    await flush();
    gate.require(run, "mine");
    gate.reset();
    answer(OK);
    await silent;
    expect(run).not.toHaveBeenCalled();
    expect(layPhienViGov()?.phone_verified).toBe(true);
  });
});

describe("explicitTap — 'Phản ánh của tôi' with the permission already granted", () => {
  it("granted → no explanation: the number is asked at once and the act runs", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const run = vi.fn();
    const { gate, states } = harness(open, async () => true);
    gate.require(run, "mine", { explicitTap: true });
    await flush();
    await flush();
    expect(states).not.toContainEqual({ kieu: "hoi" });
    expect(open.mock.calls.map((c) => c[1])).toEqual(["ask"]);
    expect(run).toHaveBeenCalledTimes(1);
  });

  it("not granted → the explanation exactly as before; nothing called until 'Đồng ý'", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const { gate, last } = harness(open, async () => false);
    gate.require(() => {}, "mine", { explicitTap: true });
    await flush();
    expect(last()).toEqual({ kieu: "hoi" });
    expect(open).not.toHaveBeenCalled();
  });

  it("the read throws → the explanation", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const { gate, last } = harness(open, async () => {
      throw new Error("getSetting");
    });
    gate.require(() => {}, "mine", { explicitTap: true });
    await flush();
    expect(last()).toEqual({ kieu: "hoi" });
    expect(open).not.toHaveBeenCalled();
  });

  it("WITHOUT the flag (a retry after a lost session) the explanation always comes — no reopen loop without a tap", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const read = vi.fn(async () => true);
    const { gate, last } = harness(open, read);
    gate.require(() => {}, "mine");
    await flush();
    expect(last()).toEqual({ kieu: "hoi" });
    expect(read).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
  });

  it("the citizen leaves while the read runs → not followed: nothing opens", async () => {
    let answer: (g: boolean) => void = () => {};
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const { gate, last } = harness(open, () => new Promise((done) => (answer = done)));
    gate.require(() => {}, "mine", { explicitTap: true });
    expect(last()).toEqual({ kieu: "dang-mo" });
    gate.reset();
    answer(true);
    await flush();
    expect(open).not.toHaveBeenCalled();
    expect(last()).toBeNull();
  });
});

/*
 * Owner 12/10/2026 (follow-up to row 7): "bỏ màn giải thích khi Zalo đã cho quyền số" for EVERY personal act the
 * citizen taps, not only "Phản ánh của tôi". The gate's acts — lookup from the search, the "Nhận thông báo" row, and
 * sending (which asks Zalo at once in any case) — then the rating's own machine, whose reopen is reached from a 403.
 */
describe("explicitTap — every personal act a tap starts, with the permission granted or not", () => {
  it.each<PhoneVerificationTask>(["lookup", "notify", "mine"])("%s, granted → no explanation: asked at once, the act runs", async (task) => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const run = vi.fn();
    const { gate, states } = harness(open, async () => true);
    gate.require(run, task, { explicitTap: true });
    await flush();
    await flush();
    expect(states).not.toContainEqual({ kieu: "hoi" });
    expect(open.mock.calls.map((c) => c[1])).toEqual(["ask"]);
    expect(run).toHaveBeenCalledTimes(1);
  });

  it.each<PhoneVerificationTask>(["lookup", "notify", "mine"])("%s, NOT granted → the explanation; nothing opened", async (task) => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const run = vi.fn();
    const { gate, last } = harness(open, async () => false);
    gate.require(run, task, { explicitTap: true });
    await flush();
    expect(last()).toEqual({ kieu: "hoi" });
    expect(open).not.toHaveBeenCalled();
    expect(run).not.toHaveBeenCalled();
  });

  it("sending: Zalo is asked at once whether granted or not — never the explanation, never the reader", async () => {
    for (const granted of [true, false]) {
      datPhienViGov(null);
      const open = vi.fn<OpenCommuneAppSession>(async () => OK);
      const read = vi.fn(async () => granted);
      const run = vi.fn();
      const { gate, states } = harness(open, read);
      gate.require(run, "submit", { explicitTap: true });
      await flush();
      expect(states, String(granted)).not.toContainEqual({ kieu: "hoi" });
      expect(open, String(granted)).toHaveBeenCalledTimes(1);
      expect(run, String(granted)).toHaveBeenCalledTimes(1);
    }
  });

  it("a lost-session retry (no flag) of a lookup still meets the explanation — the loop guard holds for every act", async () => {
    const open = vi.fn<OpenCommuneAppSession>(async () => OK);
    const read = vi.fn(async () => true);
    const { gate, last } = harness(open, read);
    gate.require(() => {}, "lookup");
    await flush();
    expect(last()).toEqual({ kieu: "hoi" });
    expect(read).not.toHaveBeenCalled();
    expect(open).not.toHaveBeenCalled();
  });
});

describe("the rating's phone machine (`createPhoneVerification` + reader) — commune app only", () => {
  const VERIFIED = { kieu: "xong", token: "tok-moi", ten_xa: COMMUNE, da_xac_thuc_so: true } as const;
  function machine(readGrant?: () => Promise<boolean>) {
    const reopen = vi.fn<ReopenWithPhone>(async () => VERIFIED);
    const states: (PhoneVerificationState | null)[] = [];
    const m = createPhoneVerification(reopen, (s) => states.push(s), readGrant);
    return { m, reopen, states, last: () => states[states.length - 1] };
  }

  // The 403 arrives on a session already open in this app (phone-less, e.g. after a lookup) — the reopen replaces it.
  const phoneLess = () => datPhienViGov({ token: "tok-cu", ten_xa: COMMUNE, phone_verified: false });

  it("granted → no explanation: reopened at once, the rating re-sent ONCE", async () => {
    phoneLess();
    const rerun = vi.fn();
    const { m, reopen, states } = machine(async () => true);
    m.onPhoneRequired(rerun);
    await flush();
    await flush();
    expect(states).not.toContainEqual({ kieu: "hoi" });
    expect(reopen).toHaveBeenCalledTimes(1);
    expect(rerun).toHaveBeenCalledTimes(1);
    expect(layPhienViGov()?.phone_verified).toBe(true);
  });

  it("granted, and the re-sent rating still gets 403 → the final sentence, no second reopen (no loop)", async () => {
    phoneLess();
    const rerun = vi.fn();
    const { m, reopen, last } = machine(async () => true);
    m.onPhoneRequired(rerun);
    await flush();
    await flush();
    m.onPhoneRequired(rerun);
    await flush();
    expect(last()).toEqual({ kieu: "ket-qua", outcome: "van-chua-xac-thuc" });
    expect(reopen).toHaveBeenCalledTimes(1);
    expect(rerun).toHaveBeenCalledTimes(1);
  });

  it("not granted, or the read throws → the explanation; nothing reopened until 'Đồng ý'", async () => {
    for (const read of [async () => false, async () => Promise.reject(new Error("getSetting"))]) {
      phoneLess();
      const { m, reopen, last } = machine(read);
      m.onPhoneRequired(() => {});
      await flush();
      expect(last()).toEqual({ kieu: "hoi" });
      expect(reopen).not.toHaveBeenCalled();
    }
  });

  it("no reader (the shared app) → the explanation at once, never a read", () => {
    phoneLess();
    const { m, reopen, last } = machine();
    m.onPhoneRequired(() => {});
    expect(last()).toEqual({ kieu: "hoi" });
    expect(reopen).not.toHaveBeenCalled();
  });

  it("the citizen starts something else while the read runs (`reset`) → not followed", async () => {
    phoneLess();
    let answer: (g: boolean) => void = () => {};
    const { m, reopen, last } = machine(() => new Promise((done) => (answer = done)));
    m.onPhoneRequired(() => {});
    expect(last()).toEqual({ kieu: "dang-xac-nhan" });
    m.reset();
    answer(true);
    await flush();
    expect(reopen).not.toHaveBeenCalled();
    expect(last()).toBeNull();
  });
});
