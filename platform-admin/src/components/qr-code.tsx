import { encode } from "uqr";

import { cn } from "@/lib/cn";

/**
 * A QR code drawn in the browser, as SVG elements React builds — never an `<img>` from a URL and
 * never a third-party QR service. What it encodes here is a TOTP `otpauth://` URI, i.e. the
 * second-factor SECRET: sending it to any server to be turned into a picture would hand that
 * server the operator's second factor.
 *
 * DEPENDENCY: `uqr` (unjs, MIT, zero dependencies, pinned exactly in package.json). Writing a
 * Reed-Solomon QR encoder here would be a few hundred lines of untested code on the one screen
 * where a wrong module means an operator cannot enrol. The manual-entry key is always shown
 * beside it, so a scanner that cannot read the code is never a dead end.
 *
 * CSP: only SVG presentation ATTRIBUTES (`fill`) and classes, no `style` — the document policy has
 * no 'unsafe-inline' for styles (`lib/csp.ts`). The frame stays white whatever the theme: a scanner
 * needs dark modules on a light ground.
 *
 * PNG EXPORT (`downloadQrPng`, owner 13/10/2026: the commune's launch QR gets "Tải ảnh PNG"): drawn
 * from the SAME `qrMatrix` the SVG draws, onto a `<canvas>`, so there is one encoding path and the
 * file is the code on screen module for module. Not through the SVG: loading it as a `blob:` image
 * is outside `img-src 'self' data:`. It is a separate function the caller opts into — the sign-in
 * screen's TOTP QR never calls it, because a downloaded file of the second-factor secret would
 * outlive the enrolment in somebody's Downloads folder.
 */

/** The one encoding every drawing of a QR here uses. ECC M: still readable off a slightly dirty or
 *  glaring screen or print, and the URI is short enough that the code stays small. Border 4 is the
 *  quiet zone the standard asks for; `data` INCLUDES it. */
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
export type QrDrawTarget = { fillStyle: string | CanvasGradient | CanvasPattern; fillRect: (x: number, y: number, w: number, h: number) => void };

/**
 * Paints the matrix white-ground, black-module, at a WHOLE number of pixels per module (at least
 * `minPixels` wide): a fractional scale anti-aliases module edges into grey, which is what makes a
 * printed code hard to scan. Returns the side length actually drawn, for the canvas size.
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
      className={cn("block h-auto w-full max-w-56 rounded-control border border-line bg-white", className)}
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
