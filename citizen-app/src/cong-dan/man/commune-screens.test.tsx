/// <reference types="vite/client" />
import { createElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { CanBoCongKhai, TinXaTomTat } from "../api/hop-dong-cong-khai";
import type { PhieuCuaToi, PhieuCuaToiTomTat } from "../api/hop-dong-phan-anh"; // vi-name-ok: existing contract types
import { BUILD_LABEL } from "../../lib/build-label";

import { ThanDanhBaXa } from "./DanhBaXa";
import { DANH_BA, NEWS_TYPE_LABEL, TIN_XA, XA_GIAO_DIEN, XA_PA, XA_TN } from "./noi-dung";
import { ONhapDoan } from "./o-nhap";
import {
  buocDaQua,
  DongThoiGian,
  FieldStep,
  PetitionBody,
  PetitionCard,
  PetitionDetail,
  PetitionList,
  SendDone,
  StepBar,
  stepsAhead,
} from "./PhanAnhAppXa";
import { CaNhanXa, dossierLookupReady, initials, TraCuuHoSoXa } from "./TienIchAppXa";
import { BaiTinXa, NewsListBody, relativeDay, todayVN } from "./TinTucAppXa";
import { NHOM_CHUC_NANG } from "./TrangXa";
import { VONG_DOI } from "./trai-nghiem";

/**
 * WAVE 1 OF THE COMMUNE APP (30/09/2026, `ui-design/prototpye-spec/PROTOTYPE.md` §6): each screen restyled to
 * the prototype, UI only. What is pinned is what a citizen meets — the markup — and, as much, what was
 * deliberately LEFT OUT because the API does not carry it yet (photos, view counts, hamlet, phone): a
 * placeholder standing for data that does not exist reads as data that failed to load.
 */

const html = (el: Parameters<typeof renderToStaticMarkup>[0]) => renderToStaticMarkup(el);
const noop = () => {};

const ROW: PhieuCuaToiTomTat = {
  ma_tra_cuu: "PA7K2QX9M4TD",
  trang_thai: "dang-xu-ly",
  linh_vuc: "giao-thong",
  nhan_linh_vuc: "Hạ tầng giao thông",
  trich_noi_dung: "Ổ gà lớn trước cổng chợ",
  goc_dem_han: "2026-09-28T01:00:00Z",
  han_tiep_nhan: null,
  han_xu_ly_xong: null,
  rating: null,
  rated_at: null,
};

const PETITION: PhieuCuaToi = {
  ma_tra_cuu: "PA7K2QX9M4TD",
  trang_thai: "dang-xu-ly",
  linh_vuc: "giao-thong",
  nhan_linh_vuc: "Hạ tầng giao thông",
  noi_dung: "Ổ gà lớn trước cổng chợ, xe máy dễ ngã.",
  dia_chi: "Trước cổng chợ",
  ho_ten_da_che: "",
  dien_thoai_da_che: "",
  an_danh: true,
  goc_dem_han: "2026-09-28T01:00:00Z",
  han_tiep_nhan: "2026-09-28T03:00:00Z",
  han_xu_ly_xong: null,
  ket_qua: "",
  ly_do: "",
  co_quan_nhan: "",
  rating: null,
  rated_at: null,
};

describe("home: tiles and headers per §6.1, the bell gone (decision 10)", () => {
  it("the two groups carry the prototype's marks and tones; 'Bản đồ' is kept; 'Xem tất cả' opens the directory", () => {
    const [gov, info] = NHOM_CHUC_NANG;
    expect(gov!.o.map((o) => [o.bieu_tuong, o.mau])).toEqual([
      ["camera-pin", "red"],
      ["file-search", "xanh"],
      ["contact-book", "luc"],
      ["map", "cam"],
    ]);
    expect(gov!.them).toEqual({ nhan: XA_GIAO_DIEN.xem_tat_ca, man: { kieu: "danh-ba" } });
    expect(info!.o.map((o) => [o.bieu_tuong, o.mau])).toEqual([
      ["newspaper", "xanh"],
      ["speaker", "cyan"],
      ["video", "cam"],
      ["calendar", "tim"],
    ]);
  });

  it("no bell on the home header any more: Zalo's corner is empty on every screen", () => {
    const src = import.meta.glob("./TrangXa.tsx", { query: "?raw", import: "default", eager: true }) as Record<string, string>;
    const code = Object.values(src)[0]!.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\{\/\*[\s\S]*?\*\/\}/g, "");
    expect(code).not.toMatch(/xa-hero__chuong|right=\{/);
    // …and "Thông báo" is reachable from Cá nhân instead (below).
  });
});

describe("Phản ánh của tôi: the card and the empty list (§6.3)", () => {
  it("card: #code in its own element, the day it was sent, the field, the content, the chip — and no photo box", () => {
    const card = html(createElement(PetitionCard, { petition: ROW, onOpen: noop }));
    expect(card).toMatch(/^<button type="button" class="xa-the xa-petition-card">/);
    expect(card).toContain('<strong class="xa-petition-card__code">#PA7K2QX9M4TD</strong>');
    expect(card).toContain("28/09/2026");
    expect(card).not.toContain("28/09/2026 08:00"); // the day, not the minute
    expect(card).toContain(">Hạ tầng giao thông<");
    expect(card).toContain('<span class="xa-petition-card__excerpt">Ổ gà lớn trước cổng chợ</span>');
    expect(card).toContain("xa-chip-tt--dang-xu-ly");
    // Decision 12: a photo box only when photos exist — none do until the photo card.
    expect(card).not.toMatch(/<img|Không ảnh/);
  });

  it("an empty list says so and what to do next; a filter with nothing in it does not add the hint", () => {
    const empty = html(
      createElement(PetitionList, {
        state: { kind: "ready", items: [], cursor: "", hasMore: false, loadingMore: false, moreFailure: null },
        onOpenPetition: noop,
        onLookup: noop,
        onOpen: noop,
        onRetry: noop,
        onLoadMore: noop,
      }),
    );
    expect(empty).toContain(`<p class="xa-status-title">${XA_PA.chua_co_phieu}</p>`);
    expect(empty).toContain(`<p class="xa-status-hint">${XA_PA.empty_list_hint}</p>`);
  });
});

describe("Chi tiết phiếu (§6.4)", () => {
  it("the header names the ticket: 'Phiếu #{code}'", () => {
    const screen = html(createElement(PetitionDetail, { code: "PA7K2QX9M4TD", onBack: noop, onSessionLost: noop, onChanged: noop }));
    expect(screen).toContain(`<h1 class="xa-dau-con__tieu-de">${XA_PA.ticket_title("PA7K2QX9M4TD")}</h1>`);
    expect(XA_PA.ticket_title("X")).toBe("Phiếu #X");
  });

  it("summary: the facts with their labels; the stored deadline or the 'not yet' sentence, never a computed one", () => {
    const body = html(createElement(PetitionBody, { petition: PETITION }));
    expect(body).toContain('<dl class="xa-facts">');
    for (const label of ["Nơi xảy ra", "Gửi lúc", XA_PA.du_kien_xong, "Người gửi"]) expect(body).toContain(`<span>${label}</span>`);
    expect(body).toContain("Chưa có. Hạn được ấn định sau khi cán bộ phân loại phiếu.");
    // No result written → no green frame.
    expect(body).not.toContain("xa-result-box");
  });

  it("'Kết quả xử lý' has its own green frame, only when the commune wrote one", () => {
    const body = html(createElement(PetitionBody, { petition: { ...PETITION, trang_thai: "da-xu-ly", ket_qua: "Đã vá ổ gà." } }));
    expect(body).toMatch(/<section class="xa-result-box"[^>]*><h2[^>]*>Kết quả xử lý của xã<\/h2><p class="xa-giu-dong">Đã vá ổ gà\.<\/p><\/section>/);
  });

  it("timeline: reached steps ticked, the current one aria-current, the rest greyed IN WORDS", () => {
    const tl = html(createElement(DongThoiGian, { petition: PETITION }));
    const reached = buocDaQua("dang-xu-ly");
    const ahead = stepsAhead("dang-xu-ly");
    expect(reached.length + ahead.length).toBe(VONG_DOI.length);
    expect(tl.match(/xa-dong-tg__buoc--qua/g)).toHaveLength(reached.length);
    expect(tl.match(/xa-dong-tg__buoc--cho/g)).toHaveLength(ahead.length);
    expect(tl.match(/— chưa tới bước này/g)).toHaveLength(ahead.length);
    expect(tl.match(/aria-current="step"/g)).toHaveLength(1);
  });

  it("steps ahead: only on the main path; none after a terminal branch, an unknown code, or the last step", () => {
    expect(stepsAhead("da-tiep-nhan")).toEqual(VONG_DOI.slice(1));
    expect(stepsAhead("da-dong")).toEqual([]);
    expect(stepsAhead("khong-tiep-nhan")).toEqual([]);
    expect(stepsAhead("chuyen-cap-tren")).toEqual([]);
    expect(stepsAhead("trang-thai-moi")).toEqual([]);
    // `buocDaQua` keeps its meaning: what HAS happened (pinned in trai-nghiem.test.ts too).
    expect(buocDaQua("da-tiep-nhan")).toEqual(["da-tiep-nhan"]);
  });
});

describe("Gửi phản ánh (§6.2)", () => {
  it("step bar: done ticked, current named in words and aria-current, the list says which step", () => {
    const bar = html(createElement(StepBar, { step: 2 }));
    expect(bar).toContain(`aria-label="${XA_TN.buoc(2, 3)}"`);
    expect(bar).toMatch(/xa-thanh-buoc__muc--xong"><span class="xa-thanh-buoc__cham"><svg/);
    expect(bar).toMatch(/xa-thanh-buoc__muc--dang" aria-current="step"><span class="xa-thanh-buoc__cham">2<\/span><span class="xa-thanh-buoc__label">Mô tả<\/span>/);
    expect(bar).toContain('xa-thanh-buoc__muc--cho"><span class="xa-thanh-buoc__cham">3</span>');
  });

  it("step 1 tiles: the tone on the tile, the icon, the name in its own element — the code never shown", () => {
    const step = html(
      createElement(FieldStep, {
        catalogue: { kind: "ready", fields: [{ code: "dien", label: "Điện", icon: "Zap", tone: "orange" }] },
        picked: "",
        fieldChanged: false,
        onPick: noop,
        onRetry: noop,
      }),
    );
    expect(step).toMatch(/<button type="button" role="radio" aria-checked="false" class="xa-o-lv xa-mau--cam"><svg[^>]*width="32"/);
    expect(step).toContain('<span class="xa-o-lv__label">Điện</span>');
    expect(step).not.toContain(">dien<");
  });

  it("step 3: the issued code, the SERVER's deadlines only, the two ways on", () => {
    const done = html(createElement(SendDone, { petition: PETITION, onFollow: noop, onHome: noop }));
    expect(done).toContain('<strong class="xa-ket-qua__code">#PA7K2QX9M4TD</strong>');
    expect(done).toContain("xa-pop");
    expect(done).toContain(XA_PA.acknowledge_by("28/09/2026 10:00"));
    // `han_xu_ly_xong` null → no "Dự kiến xử lý xong" line; never a date computed here (rule 10 #2).
    expect(done).not.toContain("Dự kiến xử lý xong trước");
    expect(done).toContain(`>${XA_PA.theo_doi}</button>`);
    expect(done).toContain(`>${XA_TN.nut_ve_trang_chu}</button>`);
    const both = html(
      createElement(SendDone, { petition: { ...PETITION, han_xu_ly_xong: "2026-10-02T09:00:00Z" }, onFollow: noop, onHome: noop }),
    );
    expect(both).toContain(XA_PA.resolve_by("02/10/2026 16:00"));
    const none = html(createElement(SendDone, { petition: { ...PETITION, han_tiep_nhan: null }, onFollow: noop, onHome: noop }));
    expect(none).not.toContain("xa-deadline-band");
  });

  it("the description box takes the focus ONLY when a caller asks — the shared app never does", () => {
    const plain = html(createElement(ONhapDoan, { id: "x", nhan: "Nội dung", gia_tri: "", toi_da: 10, onDoi: noop }));
    expect(plain.toLowerCase()).not.toContain("autofocus");
  });
});

describe("Cá nhân (§6.6)", () => {
  const personal = (name: string | null) =>
    html(
      createElement(CaNhanXa, {
        ho_ten: name,
        ten_xa: "Xã Thử",
        tinh: "Tỉnh Mẫu",
        so_phieu: null,
        co_chu: "vua",
        onDoiCoChu: noop,
        onMoPhanAnh: noop,
        onMoTraCuu: noop,
        onOpenNotifications: noop,
      }),
    );

  it("initials: family + given name; one word → one letter; empty → none", () => {
    expect(initials("Nguyễn Văn Hùng")).toBe("NH");
    expect(initials("  đào  ")).toBe("Đ");
    expect(initials("   ")).toBe("");
  });

  it("the profile shows the name and '{xã} · {tỉnh}' — no hamlet, no phone line invented", () => {
    const page = personal("Nguyễn Văn Hùng");
    expect(page).toContain('<span class="xa-avatar" aria-hidden="true">NH</span>');
    expect(page).toContain("Xã Thử · Tỉnh Mẫu");
    expect(page).not.toMatch(/\b0\d{9}\b/);
  });

  it("three text sizes stay; language is a statement; 'Của tôi' has Phản ánh · Tra cứu hồ sơ · Thông báo; no switch", () => {
    const page = personal(null);
    expect(page.match(/class="xa-chip( xa-chip--on)?" aria-pressed/g)).toHaveLength(3);
    expect(page).toContain(XA_TN.language_value);
    for (const row of [XA_TN.of_mine, XA_TN.o_tra_cuu_ho_so, XA_TN.thong_bao]) expect(page).toContain(row);
    expect(page).not.toMatch(/role="switch"|type="checkbox"/);
  });

  it("the version line comes from the build, and is absent when the build had no label", () => {
    const page = personal(null);
    if (BUILD_LABEL === "") expect(page).not.toContain("phiên bản");
    else expect(page).toContain(XA_TN.app_version(BUILD_LABEL));
  });
});

describe("Tin tức (§6.5)", () => {
  const item = (id: string, type: TinXaTomTat["type"] = null): TinXaTomTat => ({
    id,
    tieu_de: `Tin ${id}`,
    tom_tat: "Tóm tắt",
    chuyen_muc: "Kinh tế",
    ngay_dang: "2026-09-28",
    type,
  });

  it("how long ago, by DAY: today, yesterday, n days; past 30 days or in the future → the date", () => {
    expect(relativeDay("2026-09-30", "2026-09-30")).toBe(TIN_XA.today);
    expect(relativeDay("2026-09-29", "2026-09-30")).toBe(TIN_XA.yesterday);
    expect(relativeDay("2026-09-24", "2026-09-30")).toBe("6 ngày trước");
    expect(relativeDay("2026-08-31", "2026-09-30")).toBe("30 ngày trước");
    expect(relativeDay("2026-08-30", "2026-09-30")).toBe("30/08/2026");
    expect(relativeDay("2026-10-02", "2026-09-30")).toBe("02/10/2026");
    expect(relativeDay("2026-02-30", "2026-09-30")).toBe("Chưa rõ ngày");
    // Month and year boundaries count calendar days.
    expect(relativeDay("2025-12-31", "2026-01-01")).toBe(TIN_XA.yesterday);
    expect(todayVN(Date.UTC(2026, 8, 30, 18, 0))).toBe("2026-10-01"); // 01:00 on 1 Oct in Vietnam
  });

  it("every item is the same card: no featured first card, no view count", () => {
    const list = html(
      createElement(NewsListBody, {
        ds: { muc: [item("1"), item("2")], con_tro: "", con_nua: false, da_co_trang_dau: true, dang_tai: false, loi: null },
        onMo: noop,
        onTai: noop,
        empty: "",
      }),
    );
    expect(list.match(/class="xa-the xa-hang-tin"/g)).toHaveLength(2);
    expect(list).not.toMatch(/xa-noi-bat|lượt xem/);
  });

  it("the article's header names its TYPE, known from the row it was opened from; no cover band", () => {
    const article = html(createElement(BaiTinXa, { ten_mien: "xa-thu.vigov.vn", id: "7", ds: [item("7", "su-kien")], onQuayLai: noop }));
    expect(article).toContain(`<h1 class="xa-dau-con__tieu-de">${NEWS_TYPE_LABEL["su-kien"]}</h1>`);
    const unknown = html(createElement(BaiTinXa, { ten_mien: "xa-thu.vigov.vn", id: "8", onQuayLai: noop }));
    expect(unknown).toContain(`<h1 class="xa-dau-con__tieu-de">${TIN_XA.tieu_de}</h1>`);
    expect(article).not.toContain("xa-bai__bia");
  });
});

describe("Danh bạ (§6.9)", () => {
  const person: CanBoCongKhai = {
    ho_ten: "Cán Bộ Mẫu",
    bo_phan: "Lãnh đạo UBND xã",
    chuc_vu: "Chủ tịch UBND xã",
    so_co_quan: "0900000000",
    di_dong: "0900 000 000",
    co_zalo: true,
    display_order: null,
    residential_units_headed: ["Thôn Hà Lam"],
  };

  it("one full-width 'Gọi' per number, each named with the person AND the kind; no 'Nhắn Zalo'", () => {
    const page = html(createElement(ThanDanhBaXa, { trang: { kieu: "xong", can_bo: [person] }, onTai: noop }));
    expect(page.match(/<a class="xa-nut xa-staff-card__call" href="tel:0900000000"/g)).toHaveLength(2);
    expect(page).toContain(`aria-label="${XA_GIAO_DIEN.call_number(person.ho_ten, DANH_BA.so_co_quan)}"`);
    expect(page).toContain(`aria-label="${XA_GIAO_DIEN.call_number(person.ho_ten, DANH_BA.di_dong)}"`);
    expect(page).toContain("Trưởng thôn Hà Lam");
    expect(page).toContain('<span class="xa-staff-card__digits">0900 000 000</span>');
    expect(page).not.toMatch(/Nhắn Zalo|NHẮN ZALO/i);
    expect(page).toContain('<div class="xa-tim">');
  });
});

describe("Tra cứu hồ sơ (§6.8)", () => {
  it("the button is enabled only with ≥9 phone digits and exactly 4 file digits", () => {
    expect(dossierLookupReady("0900000000", "1007")).toBe(true);
    expect(dossierLookupReady("0900 000 000", " 1007 ")).toBe(true);
    expect(dossierLookupReady("09000000", "1007")).toBe(false);
    expect(dossierLookupReady("0900000000", "107")).toBe(false);
    expect(dossierLookupReady("0900000000", "10a7")).toBe(false);
    const page = html(createElement(TraCuuHoSoXa, { onQuayLai: noop }));
    expect(page).toMatch(/<button type="button" class="xa-nut" disabled="">/);
    // A disabled button alone does not say why: the missing part is in words under it.
    expect(page).toContain(XA_TN.tra_cuu_can_ma);
  });
});
