import { describe, expect, it } from "vitest";

import { BROWSER_PHOTO_CODEC, type DecodedPhoto, fitWithin, type PhotoCodec, SHRINK_LADDER, shrinkPhoto } from "./shrink-photo";

/**
 * THE PHONE-SIDE SHRINK (owner report 12/10/2026: a camera photo was "too large" and never sent). Node has no
 * canvas, so the decode and encode steps are injected: what is pinned here is the decision — when to re-encode,
 * which sizes, when to stop, and that a failure never loses the photo. How the real webview decodes, orients and
 * encodes is a device question (see the report), not something a Node test can answer.
 */

const MB = 1024 * 1024;
const bytes = (size: number) => new Blob([new Uint8Array(size)], { type: "image/jpeg" });

type Call = { width: number; height: number; quality: number };

/** A codec whose encode answers `sizeFor(call)` bytes, recording every call. */
function fakeCodec(
  width: number,
  height: number,
  sizeFor: (c: Call) => number | null | Error,
): { codec: PhotoCodec; calls: Call[]; released: () => number } {
  const calls: Call[] = [];
  let released = 0;
  const photo: DecodedPhoto = { width, height, release: () => void released++ };
  return {
    calls,
    released: () => released,
    codec: {
      decode: async () => photo,
      encodeJpeg: async (_p, w, h, quality) => {
        const call = { width: w, height: h, quality };
        calls.push(call);
        const size = sizeFor(call);
        if (size instanceof Error) throw size;
        return size === null ? null : bytes(size);
      },
    },
  };
}

describe("fitWithin — the longest side, never enlarged, aspect kept", () => {
  it("a 4000×3000 photo fits 2560 on its LONG side", () => {
    expect(fitWithin(4000, 3000, 2560)).toEqual({ width: 2560, height: 1920 });
  });
  it("portrait: the height is the long side", () => {
    expect(fitWithin(3000, 4000, 2560)).toEqual({ width: 1920, height: 2560 });
    expect(fitWithin(3024, 4032, 1600)).toEqual({ width: 1200, height: 1600 });
  });
  it("a photo already inside the box is NOT enlarged", () => {
    expect(fitWithin(1200, 900, 2560)).toEqual({ width: 1200, height: 900 });
    expect(fitWithin(2560, 1440, 2560)).toEqual({ width: 2560, height: 1440 });
  });
  it("a very thin panorama keeps at least one pixel", () => {
    expect(fitWithin(100000, 10, 2560)).toEqual({ width: 2560, height: 1 });
  });
});

describe("shrinkPhoto — re-encode ONLY when over the target", () => {
  it("a photo already under the target is returned untouched, without decoding", async () => {
    const small = bytes(MB);
    let decoded = 0;
    const codec: PhotoCodec = {
      decode: async () => {
        decoded++;
        throw new Error("must not decode");
      },
      encodeJpeg: async () => null,
    };
    expect(await shrinkPhoto(small, 4 * MB, codec)).toBe(small);
    expect(decoded).toBe(0);
  });

  it("the ladder STOPS at the first result under the target", async () => {
    const { codec, calls, released } = fakeCodec(4000, 3000, (c) => (c.quality === 0.85 ? 5 * MB : 2 * MB));
    const out = await shrinkPhoto(bytes(12 * MB), 4 * MB, codec);
    expect(out.size).toBe(2 * MB);
    expect(calls).toEqual([
      { width: 2560, height: 1920, quality: 0.85 },
      { width: 2560, height: 1920, quality: 0.7 },
    ]);
    expect(released()).toBe(1);
  });

  it("the first step is enough for an ordinary photo — one encode", async () => {
    const { codec, calls } = fakeCodec(4000, 3000, () => 1.5 * MB);
    expect((await shrinkPhoto(bytes(8 * MB), 4 * MB, codec)).size).toBe(1.5 * MB);
    expect(calls).toHaveLength(1);
  });

  it("walks every step down to 1600 px when needed; none under → the SMALLEST result if smaller than the original", async () => {
    const sizes = [9, 8, 6, 5];
    let i = 0;
    const { codec, calls } = fakeCodec(4000, 3000, () => sizes[i++]! * MB);
    const out = await shrinkPhoto(bytes(12 * MB), 4 * MB, codec);
    expect(calls.map((c) => [c.width, c.quality])).toEqual(SHRINK_LADDER.map((s) => [s.maxSide, s.quality]));
    expect(out.size).toBe(5 * MB);
  });

  it("a photo already under 2560 px is not encoded twice at the same pixels and quality", async () => {
    const { codec, calls } = fakeCodec(1500, 1000, () => 6 * MB);
    await shrinkPhoto(bytes(7 * MB), 4 * MB, codec);
    expect(calls).toEqual([
      { width: 1500, height: 1000, quality: 0.85 },
      { width: 1500, height: 1000, quality: 0.7 },
    ]);
  });
});

describe("shrinkPhoto — a failure NEVER loses the photo", () => {
  it("cannot decode (HEIC on Android, junk) → the ORIGINAL bytes", async () => {
    const original = bytes(12 * MB);
    const codec: PhotoCodec = { decode: async () => Promise.reject(new Error("no")), encodeJpeg: async () => null };
    expect(await shrinkPhoto(original, 4 * MB, codec)).toBe(original);
  });

  it("every encode fails or answers nothing → the original, and the decoded pixels are still freed", async () => {
    const original = bytes(12 * MB);
    const failing = fakeCodec(4000, 3000, () => new Error("out of memory"));
    expect(await shrinkPhoto(original, 4 * MB, failing.codec)).toBe(original);
    expect(failing.released()).toBe(1);
    const empty = fakeCodec(4000, 3000, () => null);
    expect(await shrinkPhoto(original, 4 * MB, empty.codec)).toBe(original);
  });

  it("an out-of-memory at the big size can still succeed at a smaller one", async () => {
    const { codec } = fakeCodec(8000, 6000, (c) => (c.width === 2560 ? new Error("oom") : 3 * MB));
    expect((await shrinkPhoto(bytes(20 * MB), 4 * MB, codec)).size).toBe(3 * MB);
  });

  it("a re-encode LARGER than the original is never chosen", async () => {
    const original = bytes(5 * MB);
    const { codec } = fakeCodec(4000, 3000, () => 6 * MB);
    expect(await shrinkPhoto(original, 4 * MB, codec)).toBe(original);
  });

  it("a decoder that reports no size → the original", async () => {
    const original = bytes(12 * MB);
    const { codec, calls } = fakeCodec(0, 0, () => MB);
    expect(await shrinkPhoto(original, 4 * MB, codec)).toBe(original);
    expect(calls).toHaveLength(0);
  });

  it("the REAL codec in a runtime with no image decoder (Node) falls back to the original, never throws", async () => {
    const original = bytes(12 * MB);
    expect(await shrinkPhoto(original, 4 * MB, BROWSER_PHOTO_CODEC)).toBe(original);
  });
});
