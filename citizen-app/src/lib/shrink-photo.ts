/**
 * SHRINK A PHOTO ON THE PHONE before it is checked against a size limit and uploaded (owner report 12/10/2026, a
 * real Zalo device: "Chụp ảnh" → the photo is "too large" and never sent).
 *
 * WHY: "Chụp ảnh" is zmp-sdk `chooseImage` (`features/tinh-nang/zalo-api.ts` `takeScenePhoto`), a hidden
 * `<input type="file">` that takes NO compression option. A modern camera original is 6–15 MB, over the commune's
 * scene-photo limit (10 MB) and the consultation-photo limit (5 MB, `vihat-miniapp`). The picker path
 * (`openMediaPicker`, `compressLevel`) compresses on Zalo's side; a picked photo still over the target goes
 * through here too.
 *
 * WHY HERE (`lib/`): both halves need it — the commune's scene photos (`cong-dan/man/scene-photos.tsx`) and the
 * ViHAT intro's consultation photos (`features/yeu-cau/consult-photos.ts`) — and they may not import each other
 * (`ranh-gioi-hai-nua.test.ts` §3a). `lib/` is the neutral shell both may import. This file makes no network call
 * and touches no storage (`phase1-collects-nothing.test.ts`): it turns bytes in memory into other bytes in memory.
 *
 * RE-ENCODING DROPS EXIF, GPS included — the canvas writes a bare JPEG. Nothing relies on it: the scene location
 * travels separately (the location step of "Gửi phản ánh"), and both servers strip EXIF anyway before storing
 * (`core/imaging/imaging.go`, used by `service-petitions/internal/app/petition_photo.go`; the ViHAT privacy policy's
 * "Máy chủ bỏ thông tin vị trí chụp"). The ONE thing EXIF carries that matters is ORIENTATION, which the server
 * would otherwise apply; so the decode below applies it first, or the stored photo would lie on its side.
 *
 * ⚠ NEVER LOSES THE PHOTO: any failure (a format the webview cannot decode — HEIC on most Android —, no canvas,
 *   out of memory) returns the ORIGINAL bytes, and the caller's own checks (type sniff, size) speak as before.
 */

/** A decoded picture, already upright, that the codec can draw at any size. `release` frees its pixels. */
export type DecodedPhoto = { readonly width: number; readonly height: number; release(): void };

/** The two platform steps, injected so the ladder below is testable in Node (no canvas there). */
export type PhotoCodec<P extends DecodedPhoto = DecodedPhoto> = {
  decode(file: Blob): Promise<P>;
  /** A JPEG of `photo` drawn at `width`×`height`, or `null` when the platform produced nothing. */
  encodeJpeg(photo: P, width: number, height: number, quality: number): Promise<Blob | null>;
};

export type ShrinkStep = { readonly maxSide: number; readonly quality: number };

/**
 * Tried in order; the first result under the target wins.
 *
 *   2560 px   the size `service-petitions` stores anyway (`photoMaxSide`, petition_photo.go) — so the first step
 *             loses no detail the commune would have kept. A 12 MP photo at 0.85 is typically 1–2 MB.
 *   0.85→0.7  quality before size: a busy scene (foliage, a crowd) can stay large at 0.85.
 *   2048·1600 only if 0.7 is still over: still enough to read a sign or a house number.
 */
export const SHRINK_LADDER: readonly ShrinkStep[] = [
  { maxSide: 2560, quality: 0.85 },
  { maxSide: 2560, quality: 0.7 },
  { maxSide: 2048, quality: 0.7 },
  { maxSide: 1600, quality: 0.7 },
];

/** `width`×`height` scaled so the longer side is at most `maxSide`, aspect kept, NEVER enlarged. PURE. */
export function fitWithin(width: number, height: number, maxSide: number): { width: number; height: number } {
  const longest = Math.max(width, height);
  if (longest <= maxSide) return { width: Math.round(width), height: Math.round(height) };
  const scale = maxSide / longest;
  return { width: Math.max(1, Math.round(width * scale)), height: Math.max(1, Math.round(height * scale)) };
}

/**
 * `file` itself when it is already at most `targetBytes` (no re-encode: the server re-encodes anyway, and a photo
 * that passes needs no second loss of quality), else the first ladder result under the target — or, if none gets
 * there, the smallest result when it is smaller than the original. Never throws; any failure returns `file`.
 */
export async function shrinkPhoto<P extends DecodedPhoto>(
  file: Blob,
  targetBytes: number,
  codec: PhotoCodec<P>,
): Promise<Blob> {
  if (file.size <= targetBytes) return file;
  let decoded: P;
  try {
    decoded = await codec.decode(file);
  } catch {
    return file;
  }
  let best: Blob | null = null;
  try {
    if (!(decoded.width > 0 && decoded.height > 0)) return file;
    const tried = new Set<string>();
    for (const step of SHRINK_LADDER) {
      const { width, height } = fitWithin(decoded.width, decoded.height, step.maxSide);
      // A photo already under 2560 px gives the same pixels at every size step: encoding them again is time a
      // low-end phone spends for an identical answer.
      const key = `${width}x${height}@${step.quality}`;
      if (tried.has(key)) continue;
      tried.add(key);
      let out: Blob | null;
      try {
        out = await codec.encodeJpeg(decoded, width, height, step.quality);
      } catch {
        // Out of memory at one size can still succeed at a smaller one.
        out = null;
      }
      if (out === null || out.size === 0) continue;
      if (best === null || out.size < best.size) best = out;
      if (out.size <= targetBytes) break;
    }
  } catch {
    return file;
  } finally {
    try {
      decoded.release();
    } catch {
      // Freeing is best effort; the photo is what matters.
    }
  }
  return best !== null && best.size < file.size ? best : file;
}

/* ═══════════════════════════════ THE BROWSER CODEC ═══════════════════════════════ */

type BrowserPhoto = DecodedPhoto & { readonly source: CanvasImageSource };

/** `createImageBitmap` with EXIF orientation applied — or `null` when the webview lacks it or refuses the option. */
async function decodeAsBitmap(file: Blob): Promise<BrowserPhoto | null> {
  if (typeof createImageBitmap !== "function") return null;
  try {
    const bitmap = await createImageBitmap(file, { imageOrientation: "from-image" });
    return { width: bitmap.width, height: bitmap.height, source: bitmap, release: () => bitmap.close() };
  } catch {
    // An older webview rejects "from-image" as an unknown value; the <img> path below still orients.
    return null;
  }
}

/** The fallback: an `<img>` on an object URL. Modern webviews draw an `<img>` upright by its EXIF orientation. */
function decodeAsImage(file: Blob): Promise<BrowserPhoto> {
  if (typeof document === "undefined" || typeof Image === "undefined" || typeof URL.createObjectURL !== "function") {
    return Promise.reject(new Error("no image decoder"));
  }
  const url = URL.createObjectURL(file);
  return new Promise<BrowserPhoto>((resolve, reject) => {
    const img = new Image();
    img.onload = () =>
      resolve({
        width: img.naturalWidth,
        height: img.naturalHeight,
        source: img,
        release: () => {
          img.src = "";
          URL.revokeObjectURL(url);
        },
      });
    img.onerror = () => {
      URL.revokeObjectURL(url);
      reject(new Error("not decodable"));
    };
    img.src = url;
  });
}

function canvasToJpeg(canvas: HTMLCanvasElement, quality: number): Promise<Blob | null> {
  return new Promise((resolve) => canvas.toBlob((blob) => resolve(blob), "image/jpeg", quality));
}

/** The real codec: `createImageBitmap`/`<img>` to decode, a 2D canvas (or `OffscreenCanvas`) to encode. */
export const BROWSER_PHOTO_CODEC: PhotoCodec<BrowserPhoto> = {
  async decode(file) {
    return (await decodeAsBitmap(file)) ?? (await decodeAsImage(file));
  },
  async encodeJpeg(photo, width, height, quality) {
    const useDocument = typeof document !== "undefined";
    if (!useDocument && typeof OffscreenCanvas === "undefined") return null;
    const canvas = useDocument ? document.createElement("canvas") : new OffscreenCanvas(width, height);
    canvas.width = width;
    canvas.height = height;
    const ctx = canvas.getContext("2d") as CanvasRenderingContext2D | OffscreenCanvasRenderingContext2D | null;
    if (ctx === null) return null;
    // White under the picture: a PNG's transparent pixels would otherwise turn BLACK in a JPEG. The server
    // flattens onto white too (core/imaging), so the commune sees the same picture either way.
    ctx.fillStyle = "#ffffff";
    ctx.fillRect(0, 0, width, height);
    ctx.imageSmoothingEnabled = true;
    ctx.imageSmoothingQuality = "high";
    ctx.drawImage(photo.source, 0, 0, width, height);
    try {
      return useDocument
        ? await canvasToJpeg(canvas as HTMLCanvasElement, quality)
        : await (canvas as OffscreenCanvas).convertToBlob({ type: "image/jpeg", quality });
    } finally {
      // Free the backing store now, not at the next GC: a 2560 px canvas is ~26 MB, and a low-end phone running
      // four ladder steps should not hold four of them.
      canvas.width = 0;
      canvas.height = 0;
    }
  },
};
