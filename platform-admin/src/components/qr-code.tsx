import { encode } from "uqr";

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
 */

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

export function QrCode({ value, label }: { value: string; label: string }) {
  // ECC M: still readable off a slightly dirty or glaring screen; the URI is short enough that
  // the code stays small. Border 4 is the quiet zone the standard asks for.
  const qr = encode(value, { ecc: "M", border: 4 });
  return (
    <svg
      className="block h-auto w-full max-w-56 rounded-control border border-line bg-white"
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
