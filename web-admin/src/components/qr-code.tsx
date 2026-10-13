import { encode } from "uqr";

import { cn } from "@/lib/cn";

/**
 * A QR code drawn in the browser, as SVG elements React builds — never an `<img>` from a URL and
 * never a third-party QR service: what it encodes (the commune's Mini App launch link) would reach
 * that service, and a picture the server cannot vouch for would be printed and posted in public.
 *
 * Port of `platform-admin/src/components/qr-code.tsx` (bf1fa420). The two apps share no package, and
 * a shared one for ~90 lines would be a new build edge between the vendor console and the commune
 * app (ADR 0003 keeps those apart). Keep the two in step: both encode with ECC M, border 4.
 *
 * DEPENDENCY: `uqr` (unjs, MIT, zero dependencies), pinned exactly in package.json to the version
 * platform-admin and citizen-app use, so one URL gives the same modules in all three.
 *
 * CSP: only SVG presentation ATTRIBUTES (`fill`) and classes, no `style`. The frame stays white
 * whatever the theme: a scanner needs dark modules on a light ground.
 *
 * PNG EXPORT (`downloadQrPng`, owner 13/10/2026, ADR 0070 §Sửa đổi 13/10/2026 #4): drawn from the
 * SAME `qrMatrix` the SVG draws, onto a `<canvas>`, so the file is the code on screen module for
 * module. Not through the SVG: loading it as a `blob:` image would need `img-src blob:`.
 */

/** The one encoding every drawing of a QR here uses. ECC M: still readable off a slightly dirty or
 *  glaring print. Border 4 is the quiet zone the standard asks for; `data` INCLUDES it. */
export function qrMatrix(value: string) {
  return encode(value, { ecc: "M", border: 4 });
}

/** One `M x y h1 v1 h-1 z` square per dark module. Exported for the test. */
export function qrPath(modules: readonly (readonly boolean[])[]): string {
  const parts: string[] = [];
  modules.forEach((row, y) =>
    row.forEach((dark, x) => {
      if (dark) parts.push(`M${x} ${y}h1v1h-1z`);
    }),
  );
  return parts.join("");
}

/** The part of a 2D context `drawQrModules` needs — so the test can pass a recorder, not a canvas. */
export type QrDrawTarget = {
  fillStyle: string | CanvasGradient | CanvasPattern;
  fillRect: (x: number, y: number, w: number, h: number) => void;
};

/**
 * The side length, at a WHOLE number of pixels per module and at least `minPixels`: a fractional
 * scale anti-aliases module edges into grey, which is what makes a printed code hard to scan.
 */
export function qrPixelSize(moduleCount: number, minPixels: number): number {
  return Math.ceil(minPixels / moduleCount) * moduleCount;
}

export function drawQrModules(target: QrDrawTarget, modules: readonly (readonly boolean[])[], side: number): void {
  const scale = side / modules.length;
  target.fillStyle = "#fff";
  target.fillRect(0, 0, side, side);
  target.fillStyle = "#000";
  modules.forEach((row, y) =>
    row.forEach((dark, x) => {
      if (dark) target.fillRect(x * scale, y * scale, scale, scale);
    }),
  );
}

/**
 * Saves the QR of `value` as a PNG named `fileName`. The caller chooses a name WITHOUT personal data
 * (rule 3, forbidden #4). Rejects when the browser cannot give a canvas or a blob, so the caller can
 * say so instead of a button that silently did nothing.
 */
export function downloadQrPng(value: string, fileName: string, minPixels = 1024): Promise<void> {
  const qr = qrMatrix(value);
  const side = qrPixelSize(qr.size, minPixels);
  const canvas = document.createElement("canvas");
  canvas.width = side;
  canvas.height = side;
  const ctx = canvas.getContext("2d");
  if (ctx === null) return Promise.reject(new Error("canvas 2d context unavailable"));
  drawQrModules(ctx, qr.data, side);
  return new Promise((resolve, reject) => {
    canvas.toBlob((blob) => {
      if (blob === null) {
        reject(new Error("canvas produced no PNG"));
        return;
      }
      const url = URL.createObjectURL(blob);
      const a = document.createElement("a");
      a.href = url;
      a.download = fileName;
      document.body.appendChild(a);
      a.click();
      a.remove();
      // Revoked on the next turn, not at once: some browsers start reading the URL after click() returns.
      setTimeout(() => URL.revokeObjectURL(url), 1000);
      resolve();
    }, "image/png");
  });
}

export function QrCode({ value, label, className }: { value: string; label: string; className?: string }) {
  const qr = qrMatrix(value);
  return (
    <svg
      className={cn("border-line block h-auto w-full max-w-56 rounded-[10px] border border-solid bg-white", className)}
      viewBox={`0 0 ${qr.size} ${qr.size}`}
      role="img"
      aria-label={label}
      shapeRendering="crispEdges"
    >
      <rect width={qr.size} height={qr.size} fill="#fff" />
      <path d={qrPath(qr.data)} fill="#000" />
    </svg>
  );
}
