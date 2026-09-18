import type { ComponentType } from "react";

import type { KeywordId, SolutionId } from "../../content/company-profile";

/**
 * DECORATIVE GLYPHS, DRAWN IN THE BUNDLE — not image files.
 *
 * WHY NO IMAGE FILES:
 *
 *   Everything under `public/` goes into the package uploaded to Zalo, and the app ships as a
 *   SINGLE `iife` bundle whose files are listed in the committed `app-config.json`
 *   (vite.config.ts, README §"Nộp lên Zalo"). Every added file is one more thing that has to be
 *   declared there and can go missing in silence. An inline `<svg>` is part of the one file
 *   that already exists.
 *
 * WHY EVERY GLYPH IS `aria-hidden`:
 *
 *   None of them carries information. The words next to them do. A screen reader announcing
 *   "image" six times on the solutions screen is noise for the citizen who depends on it most,
 *   and a glyph that DID carry meaning would break the rule that status is never signalled by a
 *   picture or a colour alone (README §Non-negotiables #6).
 *
 * WHY `stroke="currentColor"`:
 *
 *   The glyph then inherits the contrast of the text around it. On a brand-gradient tile the
 *   colour is `--tile-glyph`, whose ratio against both ends of that gradient is pinned in
 *   accessibility.test.ts — a white glyph on brand blue would be 2.5:1 and fails the 3:1 that
 *   a graphical object owes a reader.
 *
 * Path data uses COMMAS between coordinates on purpose: phase1-collects-nothing.test.ts strips
 * spaces, dots, dashes and brackets before sweeping the source for numbers that look like a
 * phone number, and space-separated path data collapses into long digit runs that trip it.
 */

type GlyphProps = { className?: string };

const BOX = {
  viewBox: "0 0 24 24",
  fill: "none",
  stroke: "currentColor",
  strokeWidth: 1.8,
  strokeLinecap: "round" as const,
  strokeLinejoin: "round" as const,
  "aria-hidden": true,
  focusable: "false" as const,
};

/** Tin nhắn — a message bubble with a signal trail. */
export function MessagingGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M4,6 h13 a2,2 0 0 1 2,2 v6 a2,2 0 0 1 -2,2 h-6 l-4,3 v-3 h-3 a2,2 0 0 1 -2,-2 v-6 a2,2 0 0 1 2,-2 z" />
      <path d="M8,10 h7" />
      <path d="M8,13 h4" />
    </svg>
  );
}

/** Thoại có AI — a headset ring with sound waves. */
export function VoiceAiGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M5,14 v-2 a7,7 0 0 1 14,0 v2" />
      <path d="M3,14 h3 v5 h-3 z" />
      <path d="M18,14 h3 v5 h-3 z" />
      <path d="M21,19 a3,3 0 0 1 -3,3 h-3" />
      <path d="M12,9 v4" />
    </svg>
  );
}

/** CRM — a contact record card with two people. */
export function CrmGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M3,5 h18 a1,1 0 0 1 1,1 v12 a1,1 0 0 1 -1,1 h-18 a1,1 0 0 1 -1,-1 v-12 a1,1 0 0 1 1,-1 z" />
      <circle cx="9" cy="11" r="2.4" />
      <path d="M5,17 a4,4 0 0 1 8,0" />
      <path d="M15,10 h4" />
      <path d="M15,14 h4" />
    </svg>
  );
}

/** Danh thiếp số — an identity card with a scan bracket. */
export function NamecardGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M4,6 h16 a1,1 0 0 1 1,1 v10 a1,1 0 0 1 -1,1 h-16 a1,1 0 0 1 -1,-1 v-10 a1,1 0 0 1 1,-1 z" />
      <circle cx="9" cy="11" r="2" />
      <path d="M6,16 a3.4,3.4 0 0 1 6,0" />
      <path d="M15,9 h3" />
      <path d="M15,12 h3" />
      <path d="M15,15 h3" />
    </svg>
  );
}

/** Đám mây — a cloud over a server rack. */
export function CloudGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M7,13 a4,4 0 0 1 1,-7.9 a5,5 0 0 1 9,1.6 a3.2,3.2 0 0 1 -0.6,6.3 z" />
      <path d="M6,17 h12" />
      <path d="M9,20 h6" />
    </svg>
  );
}

/** CPaaS — connected nodes, the shape of a platform other systems plug into. */
export function CpaasGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <circle cx="12" cy="12" r="2.6" />
      <circle cx="5" cy="5" r="2" />
      <circle cx="19" cy="5" r="2" />
      <circle cx="5" cy="19" r="2" />
      <circle cx="19" cy="19" r="2" />
      <path d="M6.6,6.6 L10,10" />
      <path d="M17.4,6.6 L14,10" />
      <path d="M6.6,17.4 L10,14" />
      <path d="M17.4,17.4 L14,14" />
    </svg>
  );
}

/** AI — a processor die with pins. */
export function AiGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M7,7 h10 v10 h-10 z" />
      <path d="M10,10 h4 v4 h-4 z" />
      <path d="M10,7 v-3" />
      <path d="M14,7 v-3" />
      <path d="M10,20 v-3" />
      <path d="M14,20 v-3" />
      <path d="M7,10 h-3" />
      <path d="M7,14 h-3" />
      <path d="M20,10 h-3" />
      <path d="M20,14 h-3" />
    </svg>
  );
}

/** Điện thoại — a handset. */
export function PhoneGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M5,4 h4 l2,5 l-2.5,1.5 a11,11 0 0 0 5,5 l1.5,-2.5 l5,2 v4 a1,1 0 0 1 -1,1 a16,16 0 0 1 -15,-15 a1,1 0 0 1 1,-1 z" />
    </svg>
  );
}

/** Thư điện tử — an envelope. */
export function MailGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M3,6 h18 a1,1 0 0 1 1,1 v10 a1,1 0 0 1 -1,1 h-18 a1,1 0 0 1 -1,-1 v-10 a1,1 0 0 1 1,-1 z" />
      <path d="M2.5,7 L12,13 L21.5,7" />
    </svg>
  );
}

/** Trang web — a globe with meridians. */
export function GlobeGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <circle cx="12" cy="12" r="9" />
      <path d="M3,12 h18" />
      <path d="M12,3 a14,14 0 0 1 0,18 a14,14 0 0 1 0,-18 z" />
    </svg>
  );
}

/** Địa chỉ — a map pin. */
export function PinGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M12,21 c4.5,-5 7,-8.4 7,-11 a7,7 0 1 0 -14,0 c0,2.6 2.5,6 7,11 z" />
      <circle cx="12" cy="10" r="2.6" />
    </svg>
  );
}

/** Chứng chỉ — a shield with a check. */
export function ShieldGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M12,3 l8,3 v6 c0,4.5 -3.4,7.8 -8,9 c-4.6,-1.2 -8,-4.5 -8,-9 v-6 z" />
      <path d="M8.5,12 L11,14.5 L15.5,9.5" />
    </svg>
  );
}

/** Triết lý thương hiệu — a handshake reduced to two clasped arms. */
export function HandshakeGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M2,10 l3,-3 h4 l3,3" />
      <path d="M22,10 l-3,-3 h-4 l-3,3" />
      <path d="M12,10 l3,3 a1.6,1.6 0 0 1 -2.4,2 l-2.6,-2.4" />
      <path d="M5,7 v7 h3" />
      <path d="M19,7 v7 h-3" />
    </svg>
  );
}

/** Tầm nhìn — a compass rose. */
export function CompassGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <circle cx="12" cy="12" r="9" />
      <path d="M15.5,8.5 L13.5,13.5 L8.5,15.5 L10.5,10.5 z" />
    </svg>
  );
}

/** Sứ mệnh — a target with an arrow at the centre. */
export function TargetGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <circle cx="12" cy="12" r="8.5" />
      <circle cx="12" cy="12" r="4.5" />
      <circle cx="12" cy="12" r="1" />
    </svg>
  );
}

/** Đơn vị thành viên — a building block. */
export function BuildingGlyph({ className }: GlyphProps) {
  return (
    <svg {...BOX} className={className}>
      <path d="M4,20 v-13 l7,-4 v17" />
      <path d="M11,20 v-9 l9,3 v6 z" />
      <path d="M7,10 h1" />
      <path d="M7,14 h1" />
      <path d="M15,15 h1" />
    </svg>
  );
}

/**
 * The backdrop drawn behind the dark panels: a constellation of linked nodes over a soft grid.
 *
 * It is one `<svg>` scaled to its container rather than a repeated background image, so it
 * costs no file and no request. `preserveAspectRatio="none"` lets it stretch to whatever the
 * panel is; nothing in it has to stay circular to read as a network.
 */
export function NetworkBackdrop({ className }: GlyphProps) {
  return (
    <svg
      className={className}
      viewBox="0 0 320 180"
      preserveAspectRatio="none"
      aria-hidden={true}
      focusable="false"
    >
      <g stroke="currentColor" strokeWidth="1" fill="none" opacity="0.55">
        <path d="M18,150 L70,96 L140,124 L196,58 L266,86 L310,34" />
        <path d="M70,96 L96,32 L196,58" />
        <path d="M140,124 L188,166 L266,86" />
        <path d="M18,150 L96,32" />
      </g>
      <g fill="currentColor" opacity="0.75">
        <circle cx="18" cy="150" r="3" />
        <circle cx="70" cy="96" r="4" />
        <circle cx="96" cy="32" r="3" />
        <circle cx="140" cy="124" r="3.5" />
        <circle cx="196" cy="58" r="4.5" />
        <circle cx="266" cy="86" r="3" />
        <circle cx="188" cy="166" r="2.5" />
        <circle cx="310" cy="34" r="2.5" />
      </g>
    </svg>
  );
}

/** Glyph per solution line. A lookup, so a new line in the content file fails to compile
 *  until someone decides what it looks like — rather than rendering an empty square. */
export const SOLUTION_GLYPHS: Record<SolutionId, ComponentType<GlyphProps>> = {
  messaging: MessagingGlyph,
  "voice-ai": VoiceAiGlyph,
  crm: CrmGlyph,
  namecard: NamecardGlyph,
};

export const KEYWORD_GLYPHS: Record<KeywordId, ComponentType<GlyphProps>> = {
  cloud: CloudGlyph,
  cpaas: CpaasGlyph,
  ai: AiGlyph,
  crm: CrmGlyph,
  messaging: MessagingGlyph,
};
