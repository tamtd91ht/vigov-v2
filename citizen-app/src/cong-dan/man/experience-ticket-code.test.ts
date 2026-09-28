import { afterEach, describe, expect, it, vi } from "vitest";

import { maPhieuTraiNghiem } from "./trai-nghiem";

/**
 * The commune app's experience lookup code must come from a CSPRNG (rule 4 invariant 4, rule 13 forbidden #2).
 * It used to default to the non-cryptographic generator, and `security_guard` missed it because the
 * default was written as a bare reference, not a call.
 */
describe("maPhieuTraiNghiem — lookup code from a CSPRNG", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("the default path draws from crypto.getRandomValues and never from Math.random", () => {
    const weak = vi.spyOn(Math, "random");
    const strong = vi.spyOn(globalThis.crypto, "getRandomValues");
    expect(maPhieuTraiNghiem()).toMatch(/^TN-[A-HJ-NP-Z2-9]{8}$/);
    expect(weak).not.toHaveBeenCalled();
    expect(strong).toHaveBeenCalledTimes(1);
  });

  it("maps each byte onto the 32-symbol alphabet (injectable for a pinned result)", () => {
    expect(maPhieuTraiNghiem((n) => new Uint8Array(n))).toBe("TN-AAAAAAAA");
    expect(maPhieuTraiNghiem((n) => new Uint8Array(n).fill(31))).toBe("TN-99999999");
    // 32 wraps to the first symbol: 256 is a multiple of 32, so the modulo is unbiased.
    expect(maPhieuTraiNghiem(() => Uint8Array.from([0, 1, 7, 8, 23, 24, 32, 255]))).toBe("TN-ABHJZ2A9");
  });

  it("the source no longer names the weak generator as a default", () => {
    const src = import.meta.glob("./trai-nghiem.ts", { query: "?raw", import: "default", eager: true }) as Record<
      string,
      string
    >;
    const code = Object.values(src)[0]!;
    expect(code.length).toBeGreaterThan(0);
    expect(code).not.toMatch(/Math\s*\.\s*random/);
  });
});
