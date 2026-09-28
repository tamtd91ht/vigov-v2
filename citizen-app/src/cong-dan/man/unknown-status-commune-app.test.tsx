import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { TRANG_THAI_CHUA_CO_NHAN, XA_TN } from "./noi-dung";
import { ChipTrangThai, DanhSachPhieuTN, ThePhieuTN } from "./PhanAnhAppXa";
import { STATUS_GROUP_LABEL } from "./status-groups";
import { duocDanhGia, type PhieuTN, taoPhieuTraiNghiem } from "./trai-nghiem";

/**
 * An unknown status code in the commune's own app gets the SAME neutral treatment as the shared app
 * (`nhanTrangThai`): never a guessed group, never "Đã đóng" on a ticket that may still be open, never the raw
 * code. The contract types `status` as a bare string, so a tenth status can arrive before this app is updated.
 */

const UNKNOWN = "trang-thai-moi";

function ticket(trang_thai: string, ma: string): PhieuTN {
  const p = taoPhieuTraiNghiem(
    { linh_vuc: "Điện", noi_dung: "Đèn đường hỏng", dia_chi: "", ho_ten: "", dien_thoai: "", an_danh: true },
    "2026-09-28T01:00:00Z",
    ma,
  );
  return { ...p, trang_thai };
}

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

  it("the ticket card carries the neutral chip", () => {
    const html = renderToStaticMarkup(createElement(ThePhieuTN, { phieu: ticket(UNKNOWN, "TN-AAAAAAAA"), onMo: () => {} }));
    expect(html).toContain(TRANG_THAI_CHUA_CO_NHAN);
    expect(html).not.toContain(STATUS_GROUP_LABEL["da-dong"]);
  });

  it("the list counts it under 'Tất cả' only — no group, 'Đã đóng' included, claims it", () => {
    const html = renderToStaticMarkup(
      createElement(DanhSachPhieuTN, {
        phieu: [ticket(UNKNOWN, "TN-AAAAAAAA"), ticket("da-dong", "TN-BBBBBBBB")],
        onMo: () => {},
        onTraCuu: () => {},
      }),
    );
    expect(html).toContain(`${XA_TN.loc_tat_ca} (2)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["da-dong"]} (1)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["da-tiep-nhan"]} (0)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["dang-xu-ly"]} (0)`);
    expect(html).toContain(`${STATUS_GROUP_LABEL["da-xu-ly-xong"]} (0)`);
    // Shown in "Tất cả" (the default filter), with the neutral sentence.
    expect(html).toContain("TN-AAAAAAAA");
    expect(html).toContain(TRANG_THAI_CHUA_CO_NHAN);
  });

  it("an unknown code is not ratable — rating needs the 'Đã xử lý xong' group", () => {
    expect(duocDanhGia(ticket(UNKNOWN, "TN-AAAAAAAA"))).toBe(false);
  });
});
