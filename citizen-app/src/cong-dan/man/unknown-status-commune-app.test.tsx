import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { PhieuCuaToiTomTat } from "../api/hop-dong-phan-anh"; // vi-name-ok: existing contract type

import { TRANG_THAI_CHUA_CO_NHAN, XA_TN } from "./noi-dung";
import { ChipTrangThai, PetitionCard, PetitionList, stepLabel } from "./PhanAnhAppXa";
import { STATUS_GROUP_LABEL } from "./status-groups";

/**
 * An unknown status code in the commune's own app gets the SAME neutral treatment as the shared app
 * (`nhanTrangThai`): never a guessed group, never "Đã đóng" on a ticket that may still be open, never the raw
 * code. The contract types `status` as a bare string, so a tenth status can arrive before this app is updated.
 */

const UNKNOWN = "trang-thai-moi";

function row(trang_thai: string, code: string): PhieuCuaToiTomTat {
  return {
    ma_tra_cuu: code,
    trang_thai,
    linh_vuc: "",
    nhan_linh_vuc: "",
    trich_noi_dung: "Đèn đường hỏng",
    goc_dem_han: "2026-09-28T01:00:00Z",
    han_tiep_nhan: null,
    han_xu_ly_xong: null,
    rating: null,
    rated_at: null,
  };
}

const ready = (items: PhieuCuaToiTomTat[], hasMore = false) =>
  ({ kind: "ready", items, cursor: hasMore ? "c" : "", hasMore, loadingMore: false, moreFailure: null }) as const;

describe("commune app: unknown status code", () => {
  it("the chip shows the neutral sentence and a neutral style — not a group label, not the raw code", () => {
    const html = renderToStaticMarkup(createElement(ChipTrangThai, { tt: UNKNOWN }));
    expect(html).toBe(`<span class="xa-chip-tt xa-chip-tt--unknown">${TRANG_THAI_CHUA_CO_NHAN}</span>`);
    for (const label of Object.values(STATUS_GROUP_LABEL)) expect(html).not.toContain(label);
    expect(html).not.toContain(UNKNOWN);
    expect(html).not.toContain("xa-chip-tt--da-dong");
  });

  it("a known code still shows its group chip", () => {
    const html = renderToStaticMarkup(createElement(ChipTrangThai, { tt: "khong-tiep-nhan" }));
    expect(html).toBe(`<span class="xa-chip-tt xa-chip-tt--da-dong">${STATUS_GROUP_LABEL["da-dong"]}</span>`);
  });

  it("the petition card carries the neutral chip", () => {
    const html = renderToStaticMarkup(createElement(PetitionCard, { petition: row(UNKNOWN, "PA-1"), onOpen: () => {} }));
    expect(html).toContain(TRANG_THAI_CHUA_CO_NHAN);
    expect(html).not.toContain(STATUS_GROUP_LABEL["da-dong"]);
  });

  it("the list counts it under 'Tất cả' only — no group, 'Đã đóng' included, claims it", () => {
    const html = renderToStaticMarkup(
      createElement(PetitionList, {
        state: ready([row(UNKNOWN, "PA-AAAAAAAAAAAA"), row("da-dong", "PA-BBBBBBBBBBBB")]),
        onOpenPetition: () => {},
        onLookup: () => {},
        onOpen: () => {},
        onRetry: () => {},
        onLoadMore: () => {},
      }),
    );
    expect(html).toContain(`${XA_TN.loc_tat_ca} (2)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["da-dong"]} (1)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["da-tiep-nhan"]} (0)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["dang-xu-ly"]} (0)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["da-xu-ly-xong"]} (0)`);
    // Shown in "Tất cả" (the default filter), with the neutral sentence.
    expect(html).toContain("PA-AAAAAAAAAAAA");
    expect(html).toContain(TRANG_THAI_CHUA_CO_NHAN);
  });

  it("the timeline never prints a raw unknown code", () => {
    expect(stepLabel(UNKNOWN)).toBe(TRANG_THAI_CHUA_CO_NHAN);
    expect(stepLabel("toString")).toBe(TRANG_THAI_CHUA_CO_NHAN);
    expect(stepLabel("dang-xu-ly")).toBe("Đang xử lý");
  });
});
