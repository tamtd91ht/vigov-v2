/**
 * The parts of `/nguoi-dan` the server cannot serve yet, drawn at the PROTOTYPE's position as disabled
 * controls with a "?" (ADR 0068 §14, lần 5 #5: never faked, never dropped silently). The sentences are
 * read by `tools/tien_do_san_pham.py` for the product progress count — the `@screens` line names the
 * menu route the block is drawn on.
 */

import type { PendingFeatureInfo } from "@/components/ui/pending-feature";

// @screens /nguoi-dan
export const PHAN_CHUA_DUNG: readonly PendingFeatureInfo[] = [ // vi-name-ok: the name every screen's unbuilt-parts block carries; `tools/tien_do_san_pham.py` reads it by this name
  // Chapter 16 §3: "dải nút nhãn kèm số người; một nhãn một lúc" — bấm để lọc. The tag strip is real
  // (`GET /api/v1/citizen-contact-tags`), but neither `POST /citizen-contact-searches` nor the list's
  // query has a tag field (`identity_citizenContactSearchIn`), so a click has nothing to send.
  {
    ten: "Lọc theo nhãn",
    viSao:
      "Dải nhãn hiện các nhãn đang dùng và số người mang mỗi nhãn. Máy chủ chưa có bộ lọc theo nhãn trên " +
      "tuyến tìm người dân, nên bấm vào một nhãn chưa lọc được bảng.",
  },
];
