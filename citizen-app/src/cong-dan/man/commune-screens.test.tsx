/// <reference types="vite/client" />
import { createElement, isValidElement, type ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { BaiTinXa as BaiTinXaData, CanBoCongKhai, TinXaTomTat } from "../api/hop-dong-cong-khai";
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
import {
  ArticleCover,
  BaiTinXa,
  type CoverState,
  EventDetails,
  eventTimeLabel,
  FeaturedNewsView,
  HangTin,
  NewsArticle,
  NewsListBody,
  NewsThumb,
  publishedLabel,
  relativeDay,
  todayVN,
  videoOpenFailed,
  WatchVideo,
} from "./TinTucAppXa";
import { BaiTin, TheTin } from "./TinTucXaScreen"; // vi-name-ok: existing shared-app components
import { NHOM_CHUC_NANG, screenKey } from "./TrangXa";
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

  it("Truyền thanh and Video read the commune's published items by type, like Sự kiện — not a static empty screen", () => {
    const src = import.meta.glob("./TrangXa.tsx", { query: "?raw", import: "default", eager: true }) as Record<string, string>;
    const code = Object.values(src)[0]!.replace(/\/\*[\s\S]*?\*\//g, "").replace(/\/\/[^\n]*/g, "");
    const block = code.slice(code.indexOf('case "truyen-thanh":'), code.indexOf('case "ban-do":'));
    expect(block).toContain('case "video":');
    expect(block).toMatch(/<NewsOfType[\s\S]*type=\{newsType\}/);
    expect(block).not.toContain("ManChuaCoDuLieu");
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

  it("every item is the same card: no featured first card", () => {
    const list = html(
      createElement(NewsListBody, {
        ds: { muc: [item("1"), item("2")], con_tro: "", con_nua: false, da_co_trang_dau: true, dang_tai: false, loi: null },
        onMo: noop,
        onTai: noop,
        empty: "",
      }),
    );
    expect(list.match(/class="xa-the xa-hang-tin"/g)).toHaveLength(2);
    expect(list).not.toMatch(/xa-noi-bat/);
  });

  /**
   * THE VIEW COUNT (owner, 02/10/2026): "{n} lượt xem" after the day, with an aria-hidden eye, on the list cards and
   * the article in BOTH apps; never on the home "Tin mới" card or the Video list. An older server sends no count →
   * nothing at all, never a made-up "0". A count of 0 (a new article) is a real count → "0 lượt xem".
   */
  describe("view count", () => {
    const VIEWS = '<span class="view-count"><svg class="xa-bt" width="16" height="16"';
    const page = (muc: TinXaTomTat[], showViews?: boolean) =>
      html(
        createElement(NewsListBody, {
          ds: { muc, con_tro: "", con_nua: false, da_co_trang_dau: true, dang_tai: false, loi: null },
          onMo: noop,
          onTai: noop,
          empty: "",
          ...(showViews === undefined ? {} : { showViews }),
        }),
      );

    it("vi-VN grouping: 1.234, 0 → '0 lượt xem'", () => {
      expect(TIN_XA.view_count(1234)).toBe("1.234 lượt xem");
      expect(TIN_XA.view_count(1234567)).toBe("1.234.567 lượt xem");
      expect(TIN_XA.view_count(0)).toBe("0 lượt xem");
    });

    it("commune app list: after the day, eye aria-hidden at 16px; 0 shows; absent shows nothing", () => {
      const list = page([{ ...item("1"), viewCount: 1234 }, { ...item("2"), viewCount: 0 }, item("3")]);
      expect(list).toMatch(
        /<span class="xa-phu news-meta">[^<]*<span class="view-count"><svg class="xa-bt" width="16" height="16"[^>]*aria-hidden="true"[^>]*>[\s\S]*?<\/svg>1\.234 lượt xem<\/span><\/span>/,
      );
      expect(list).toContain("</svg>0 lượt xem</span>");
      expect(list.match(/lượt xem/g)).toHaveLength(2);
      // The card without a count keeps exactly its old line.
      expect(list.match(/<span class="xa-phu">/g)).toHaveLength(1);
    });

    it("NOT on the Video list, NOT on the home screen's compact card; YES on 'Tin liên quan'", () => {
      expect(page([{ ...item("1"), viewCount: 5 }], false)).not.toContain("lượt xem");
      expect(html(createElement(HangTin, { tin: { ...item("1"), viewCount: 5 }, onMo: noop, compact: true }))).not.toContain(
        "lượt xem",
      );
      const tin = { ...item("1"), viewCount: 9 };
      const related = html(
        createElement(NewsArticle, {
          bai: { ...item("0"), noi_dung: "Đoạn." },
          coverFailed: false,
          onCoverFail: noop,
          ds: [tin],
          onMo: noop,
        }),
      );
      expect(related).toContain("9 lượt xem");
    });

    it("the Video list and the home card are wired that way in the source", () => {
      const src = import.meta.glob(["./TinTucAppXa.tsx", "./TrangXa.tsx"], { query: "?raw", import: "default", eager: true }) as Record<
        string,
        string
      >;
      expect(src["./TinTucAppXa.tsx"]).toContain('showViews={props.type !== "video"}');
      expect(src["./TrangXa.tsx"]).not.toMatch(/<HangTin[^>]*showViews/);
    });

    it("commune app article: the meta line ends with the count; absent → the old line exactly", () => {
      const bai: BaiTinXaData = { ...item("1"), ngay_dang: "2026-09-25", noi_dung: "Đoạn.", viewCount: 1234 };
      const withCount = html(createElement(NewsArticle, { bai, coverFailed: false, onCoverFail: noop, ds: [] }));
      expect(withCount).toMatch(/<p class="xa-phu news-meta">Kinh tế · 25\/09\/2026<span class="view-count"><svg[^>]*>[\s\S]*?<\/svg>1\.234 lượt xem<\/span><\/p>/);
      const { viewCount: _drop, ...noCount } = bai;
      const without = html(createElement(NewsArticle, { bai: noCount, coverFailed: false, onCoverFail: noop, ds: [] }));
      expect(without).toContain('<p class="xa-phu">Kinh tế · 25/09/2026</p>');
      expect(without).not.toContain("lượt xem");
      expect(html(createElement(NewsArticle, { bai: { ...bai, viewCount: 0 }, coverFailed: false, onCoverFail: noop, ds: [] }))).toContain(
        "0 lượt xem",
      );
    });

    it("shared app: list card and article both carry it; absent → nothing", () => {
      const card = html(createElement(TheTin, { tin: { ...item("1"), viewCount: 1234 }, onMo: noop }));
      expect(card).toContain('<span class="cd-the-cua-toi__dong news-meta">');
      expect(card).toContain(VIEWS);
      expect(card).toContain("1.234 lượt xem");
      expect(html(createElement(TheTin, { tin: item("1"), onMo: noop }))).not.toContain("lượt xem");
      const article = html(createElement(BaiTin, { bai: { ...item("1"), noi_dung: "Đoạn.", viewCount: 0 } }));
      expect(article).toContain("0 lượt xem");
      expect(html(createElement(BaiTin, { bai: { ...item("1"), noi_dung: "Đoạn." } }))).not.toContain("lượt xem");
    });
  });

  it("the article's header names its TYPE, known from the row it was opened from; no cover while it loads", () => {
    const article = html(createElement(BaiTinXa, { ten_mien: "xa-thu.vigov.vn", id: "7", ds: [item("7", "su-kien")], onQuayLai: noop }));
    expect(article).toContain(`<h1 class="xa-dau-con__tieu-de">${NEWS_TYPE_LABEL["su-kien"]}</h1>`);
    const unknown = html(createElement(BaiTinXa, { ten_mien: "xa-thu.vigov.vn", id: "8", onQuayLai: noop }));
    expect(unknown).toContain(`<h1 class="xa-dau-con__tieu-de">${TIN_XA.tieu_de}</h1>`);
    expect(article).not.toContain("xa-bai__cover");
  });
});

/**
 * THE NEWS COVER (owner, 01/10/2026, ADR 0047 §6): commune app only. Card: the picture in the slot when the item
 * has one, the newspaper icon when it has none or the picture fails — the slot never goes. Detail: a 16:9 band
 * above the title when it has one; none or failed → no band at all. The URL is fake and points nowhere.
 */
describe("Tin tức: cover picture (card and detail)", () => {
  const COVER = "https://media.vigov.example/t_TENANT/cover-1280.jpg";
  const plain: TinXaTomTat = { id: "1", tieu_de: "Tin một", tom_tat: "", chuyen_muc: "Kinh tế", ngay_dang: "2026-09-28", type: "tin-tuc" };
  const withCover: TinXaTomTat = { ...plain, imageUrl: COVER };
  const article: BaiTinXaData = { ...withCover, noi_dung: "Đoạn một." };

  /** The one `<img>` a hookless element tree holds, found without rendering it. */
  function findImg(node: unknown): ReactElement<Record<string, unknown>> | null {
    if (!isValidElement(node)) return null;
    const el = node as ReactElement<Record<string, unknown>>;
    if (el.type === "img") return el;
    const kids = el.props.children;
    for (const k of Array.isArray(kids) ? kids : [kids]) {
      const hit = findImg(k);
      if (hit !== null) return hit;
    }
    return null;
  }

  it("card with a cover: the picture fills the slot, decorative and lazy; no icon beside it", () => {
    const card = html(createElement(HangTin, { tin: withCover, onMo: noop, today: "2026-09-30" }));
    expect(card).toContain(
      `<span class="xa-hang-tin__o" aria-hidden="true"><img class="xa-hang-tin__image" src="${COVER}" alt="" loading="lazy" decoding="async"/></span>`,
    );
    expect(card).not.toContain("<svg");
  });

  it("card without a cover: the newspaper icon, as before — the slot stays", () => {
    const card = html(createElement(HangTin, { tin: plain, onMo: noop, today: "2026-09-30" }));
    expect(card).toContain('<span class="xa-hang-tin__o" aria-hidden="true"><svg');
    expect(card).not.toContain("<img");
  });

  it("a failed picture falls back to the icon, and the image's onError is what reports the failure", () => {
    const failed = html(createElement(NewsThumb, { imageUrl: COVER, failed: true, onFail: noop }));
    expect(failed).toContain("<svg");
    expect(failed).not.toContain("<img");
    let reported = 0;
    const img = findImg(NewsThumb({ imageUrl: COVER, failed: false, onFail: () => reported++ }));
    expect(img?.props.alt).toBe("");
    (img!.props.onError as () => void)();
    expect(reported).toBe(1);
  });

  it("the home screen's card is the compact one (88×72); lists keep 96×80", async () => {
    // Read from disk, not imported — the reason and the variable specifier are `accessibility.test.ts:8-19`'s.
    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    expect(html(createElement(HangTin, { tin: plain, onMo: noop, compact: true }))).toMatch(
      /^<button type="button" class="xa-the xa-hang-tin xa-hang-tin--compact">/,
    );
    expect(html(createElement(HangTin, { tin: plain, onMo: noop }))).toMatch(/^<button type="button" class="xa-the xa-hang-tin">/);
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");
    expect(css).toMatch(/\.xa-hang-tin__o \{[^}]*width: 96px;[^}]*height: 80px;/);
    expect(css).toMatch(/\.xa-hang-tin--compact \.xa-hang-tin__o \{[^}]*width: 88px;[^}]*height: 72px;/);
    expect(css).toMatch(/\.xa-bai__cover \{[^}]*aspect-ratio: 16 \/ 9;/);
    const src = Object.values(import.meta.glob("./TrangXa.tsx", { query: "?raw", import: "default", eager: true }))[0] as string;
    expect(src).toContain("<HangTin tin={t} compact ");
  });

  it("detail with a cover: one decorative band ABOVE the title", () => {
    // Server rendering adds a `<link rel="preload">` for a non-lazy image (React's own, as in
    // `commune-app-look.test.tsx`); the article itself is what is pinned.
    const page = html(createElement(NewsArticle, { bai: article, coverFailed: false, onCoverFail: noop, ds: [] })).replace(
      /^<link rel="preload"[^>]*\/>/,
      "",
    );
    expect(page).toMatch(
      new RegExp(`^<article class="xa-bai"><div class="xa-bai__cover"><img class="xa-bai__cover-image" src="${COVER.replace(/[.]/g, "\\.")}" alt="" decoding="async"/></div><h2 class="xa-bai__tieu-de">`),
    );
    expect(page.match(/<img /g)).toHaveLength(1);
  });

  it("detail without a cover, or with one that failed: NO band at all", () => {
    for (const bai of [{ ...plain, noi_dung: "Đoạn một." }, article]) {
      const page = html(createElement(NewsArticle, { bai, coverFailed: bai === article, onCoverFail: noop, ds: [] }));
      expect(page).toMatch(/^<article class="xa-bai"><h2 class="xa-bai__tieu-de">/);
      expect(page).not.toMatch(/xa-bai__cover|<img/);
    }
  });

  it("the band's onError reaches the article's onCoverFail", () => {
    let reported = 0;
    const onCoverFail = () => reported++;
    const tree = NewsArticle({ bai: article, coverFailed: false, onCoverFail, ds: [] });
    const cover = (tree.props as { children: unknown[] }).children.find(
      (c) => isValidElement(c) && c.type === ArticleCover,
    ) as ReactElement<{ onFail: () => void }>;
    expect(cover.props.onFail).toBe(onCoverFail);
    const img = findImg(ArticleCover({ imageUrl: COVER, failed: false, onFail: onCoverFail }));
    (img!.props.onError as () => void)();
    expect(reported).toBe(1);
    expect(ArticleCover({ imageUrl: undefined, failed: false, onFail: noop })).toBeNull();
  });
});

/**
 * THE HOME SCREEN'S FEATURED CARD (owner, UI-1 02/10/2026): the newest item, large, ONLY with its own cover loaded.
 * Everything short of that is the compact card as before — the reason wave 1 removed `.xa-noi-bat` still holds.
 */
describe("Trang chủ: the featured card (UI-1)", () => {
  const COVER = "https://media.vigov.example/t_TENANT/cover-1280.jpg";
  const plain: TinXaTomTat = { id: "1", tieu_de: "Tin một", tom_tat: "", chuyen_muc: "", ngay_dang: "2026-09-30", type: "tin-tuc" };
  const withCover: TinXaTomTat = { ...plain, imageUrl: COVER, viewCount: 42 };
  const view = (tin: TinXaTomTat, cover: CoverState) =>
    html(createElement(FeaturedNewsView, { tin, cover, onFail: noop, onMo: noop, today: "2026-09-30" }));

  it("large only when the cover LOADED: the 16:9 band edge to edge, then the title and the day — one button", () => {
    // React's server render may add a `<link rel="preload">` for a non-lazy image (as in the article test above).
    const card = view(withCover, "loaded").replace(/^<link rel="preload"[^>]*\/>/, "");
    expect(card).toMatch(
      new RegExp(
        `^<button type="button" class="xa-the xa-featured-news"><span class="xa-featured-news__cover" aria-hidden="true"><img class="xa-featured-news__image" src="${COVER.replace(/[.]/g, "\\.")}" alt="" decoding="async"/></span><span class="xa-featured-news__body"><strong class="xa-hang-tin__tieu-de xa-cat-2">Tin một</strong><span class="xa-phu">${TIN_XA.today}</span></span></button>$`,
      ),
    );
    // No view count, like the compact home card (6a29447b).
    expect(card).not.toContain("lượt xem");
  });

  it("not loaded yet, failed, or no cover at all → the ordinary compact card, nothing in its place", () => {
    for (const [tin, cover] of [
      [withCover, "pending"],
      [withCover, "failed"],
      [plain, "loaded"],
    ] as const) {
      const card = view(tin, cover);
      expect(card).toMatch(/^<button type="button" class="xa-the xa-hang-tin xa-hang-tin--compact">/);
      expect(card).not.toMatch(/xa-featured-news|lượt xem/);
    }
  });

  it("the band's onError is what turns it back into the compact card", () => {
    let failed = 0;
    const tree = FeaturedNewsView({ tin: withCover, cover: "loaded", onFail: () => failed++, onMo: noop, today: "2026-09-30" });
    const kids = (tree.props as { children: unknown[] }).children;
    const coverSpan = kids.find((c) => isValidElement(c) && (c.props as { className?: string }).className === "xa-featured-news__cover") as ReactElement<{
      children: ReactElement<{ onError: () => void }>;
    }>;
    coverSpan.props.children.props.onError();
    expect(failed).toBe(1);
  });

  it("the home screen features the FIRST item only; the second stays compact; lists keep one card shape", () => {
    const src = Object.values(import.meta.glob("./TrangXa.tsx", { query: "?raw", import: "default", eager: true }))[0] as string;
    expect(src).toMatch(/i === 0 \? \(\s*<FeaturedNews tin=\{t\}/);
    expect(src).not.toMatch(/<FeaturedNews[^>]*showViews/);
    const tinSrc = Object.values(import.meta.glob("./TinTucAppXa.tsx", { query: "?raw", import: "default", eager: true }))[0] as string;
    const listBody = tinSrc.slice(tinSrc.indexOf("export function NewsListBody("), tinSrc.indexOf("export function useTinXa("));
    expect(listBody).not.toContain("FeaturedNews");
  });
});

/**
 * THE ENTRY MOTION (owner, UI-1 02/10/2026): the commune app has no router, so "a new screen" is a new
 * `screenKey`. The frame carries `xa-frame--enter` only while the key is new (`TrangXa.tsx`).
 */
describe("navigation: what counts as a new screen", () => {
  it("each tab, each article, each petition is its own screen; a child screen is its kind", () => {
    expect(screenKey({ kieu: "tab", tab: "trang-chu" })).toBe("tab:trang-chu");
    expect(screenKey({ kieu: "tab", tab: "tin-tuc" })).not.toBe(screenKey({ kieu: "tab", tab: "trang-chu" }));
    expect(screenKey({ kieu: "bai", id: "1", tu: "trang-chu" })).toBe(screenKey({ kieu: "bai", id: "1", tu: "tin-tuc" }));
    expect(screenKey({ kieu: "bai", id: "1", tu: "trang-chu" })).not.toBe(screenKey({ kieu: "bai", id: "2", tu: "trang-chu" }));
    expect(screenKey({ kieu: "phieu", ma: "A", tu: "phan-anh" })).not.toBe(screenKey({ kieu: "phieu", ma: "B", tu: "phan-anh" }));
    expect(screenKey({ kieu: "danh-ba" })).toBe("danh-ba");
  });

  it("the entry animates the scrolling body, not the frame — so no transform sits on the leave-app dialog's ancestors", async () => {
    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");
    expect(css).toMatch(/\.xa-frame--enter \.xa-trang,\s*\.cd-man \{\s*animation: xa-enter 200ms ease-out backwards;/);
    expect(css).not.toMatch(/\.xa-frame--enter\s*\{/);
    // ≤ 250ms, and the press feedback ≤ 150ms.
    expect(css).toMatch(/transition: transform 120ms ease-out;/);
    const src = Object.values(import.meta.glob("./TrangXa.tsx", { query: "?raw", import: "default", eager: true }))[0] as string;
    // No key was added to remount the tab bodies (their loaded state must survive a tab switch).
    expect(src).not.toMatch(/<(TrangChuXa|PetitionList|DanhSachTinXa|CaNhanXa)[^>]*\bkey=/);
  });
});

/**
 * THE REST OF WAVE 1 ON THE ARTICLE (G1, ADR 0047 §6): the time it was published and, on an event, its window
 * and place. Every one optional — absent means the line or block is simply not there. Vietnam time, pinned +07.
 */
describe("Tin tức: published time and the event block (G1)", () => {
  const base: BaiTinXaData = {
    id: "1",
    tieu_de: "Tin một",
    tom_tat: "",
    chuyen_muc: "Kinh tế",
    ngay_dang: "2026-09-25",
    type: "tin-tuc",
    noi_dung: "Đoạn một.",
  };
  const render = (bai: BaiTinXaData) => html(createElement(NewsArticle, { bai, coverFailed: false, onCoverFail: noop, ds: [] }));

  it("published at: 'HH:mm ngày dd/MM/yyyy' in Vietnam time when it falls on ngay_dang; otherwise the day alone", () => {
    expect(publishedLabel({ ...base, publishedAt: "2026-09-25T02:07:00Z" })).toBe("09:07 ngày 25/09/2026");
    // 18:30 UTC on the 24th is 01:30 on the 25th in Vietnam — the day is Vietnam's, not the phone's or UTC's.
    expect(publishedLabel({ ...base, publishedAt: "2026-09-24T18:30:00Z" })).toBe("01:30 ngày 25/09/2026");
    // A backdated item: the instant is another day than the one displayed — no time is claimed.
    expect(publishedLabel({ ...base, ngay_dang: "2026-09-20", publishedAt: "2026-09-25T02:07:00Z" })).toBe("20/09/2026");
    expect(publishedLabel(base)).toBe("25/09/2026");
    expect(render({ ...base, publishedAt: "2026-09-25T02:07:00Z" })).toContain('<p class="xa-phu">Kinh tế · 09:07 ngày 25/09/2026</p>');
    expect(render(base)).toContain('<p class="xa-phu">Kinh tế · 25/09/2026</p>');
  });

  it("the event window in words: start only, same day, two days; no start → no time", () => {
    expect(eventTimeLabel("2026-10-05T01:00:00Z", undefined)).toBe("08:00 ngày 05/10/2026");
    expect(eventTimeLabel("2026-10-05T01:00:00Z", "2026-10-05T04:00:00Z")).toBe("08:00 – 11:00 ngày 05/10/2026");
    expect(eventTimeLabel("2026-10-05T01:00:00Z", "2026-10-06T10:00:00Z")).toBe("08:00 ngày 05/10/2026 – 17:00 ngày 06/10/2026");
    expect(eventTimeLabel(undefined, "2026-10-06T10:00:00Z")).toBeNull();
    expect(eventTimeLabel(undefined, undefined)).toBeNull();
  });

  it("an event article: the block under the meta line, labelled in words, each row only when set", () => {
    const ev: BaiTinXaData = { ...base, type: "su-kien", eventStartsAt: "2026-10-05T01:00:00Z", eventEndsAt: "2026-10-05T04:00:00Z", eventPlace: "Nhà văn hoá thôn" };
    const page = render(ev);
    expect(page).toContain(
      `<section class="xa-bai__event" aria-label="${XA_TN.event_details}"><dl class="xa-bai__event-list">` +
        `<div><dt>Thời gian</dt><dd>08:00 – 11:00 ngày 05/10/2026</dd></div>` +
        `<div><dt>Địa điểm</dt><dd>Nhà văn hoá thôn</dd></div></dl></section><div class="xa-ke"></div>`,
    );
    const placeOnly = render({ ...base, type: "su-kien", eventPlace: "Sân UBND" });
    expect(placeOnly).toContain("<dt>Địa điểm</dt><dd>Sân UBND</dd>");
    expect(placeOnly).not.toContain("Thời gian");
    const timeOnly = render({ ...base, type: "su-kien", eventStartsAt: "2026-10-05T01:00:00Z" });
    expect(timeOnly).toContain("<dt>Thời gian</dt><dd>08:00 ngày 05/10/2026</dd>");
    expect(timeOnly).not.toContain("Địa điểm");
  });

  it("absent fields render NOTHING: no event block, no time, no video button", () => {
    const page = render({ ...base, type: "su-kien" });
    expect(page).not.toMatch(/xa-bai__event|Thời gian|Địa điểm|Xem video/);
    expect(html(createElement(EventDetails, { tin: base }))).toBe("");
  });

  it("the event block uses the purple tone pairs `accessibility.test.ts` already measures, and sets no font size", async () => {
    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");
    expect(css).toMatch(/\.xa-bai__event \{[^}]*background: var\(--xa-purple-50\);/);
    expect(css).toMatch(/\.xa-bai__event-list dt \{[^}]*color: var\(--xa-purple-ink\);/);
    expect(css).toMatch(/\.xa-bai__event-list dd \{[^}]*color: var\(--ink\);/);
    for (const m of css.matchAll(/\.xa-bai__event[^{]*\{([^}]*)\}/g)) expect(m[1]).not.toContain("font-size");
  });
});

/**
 * "XEM VIDEO" (owner, 01/10/2026, ADR 0047 §6): on the detail of a `video` item with a link, in the commune's own
 * app only. The tap hands the commune's link, unchanged, to the opener the shell injects (`moRaNgoai("video", …)`);
 * a failed open says what to do next in one sentence. The link is fake and points nowhere.
 */
describe("Tin tức: the 'Xem video' button", () => {
  const CLIP = "https://video.example.vn/xem/abc123?t=10";
  const video: BaiTinXaData = {
    id: "9",
    tieu_de: "Hướng dẫn nộp hồ sơ trực tuyến",
    tom_tat: "",
    chuyen_muc: "Hành chính",
    ngay_dang: "2026-09-25",
    type: "video",
    videoUrl: CLIP,
    noi_dung: "Đoạn một.",
  };
  const render = (extra: Record<string, unknown>) =>
    html(createElement(NewsArticle, { bai: video, coverFailed: false, onCoverFail: noop, ds: [], ...extra }));
  const BUTTON = `<div class="xa-bai__video"><button type="button" class="xa-nut">${XA_TN.watch_video}</button></div>`;

  it("present: one red primary button in plain words, after the meta line and before the body", () => {
    const page = render({ onWatchVideo: noop });
    expect(XA_TN.watch_video).toBe("Xem video");
    expect(page).toContain(`<p class="xa-phu">Hành chính · 25/09/2026</p>${BUTTON}<div class="xa-ke"></div>`);
    expect(page.match(/Xem video/g)).toHaveLength(1);
    expect(page).not.toContain('role="alert"');
  });

  it("absent: no opener → no button, even with a link (tests, the shared app)", () => {
    expect(render({})).not.toMatch(/xa-bai__video|Xem video/);
    // And `BaiTinXa` with no `openVideo`, or still loading, draws none either.
    const detail = html(createElement(BaiTinXa, { ten_mien: "xa-thu.vigov.vn", id: "9", ds: [video], onQuayLai: noop }));
    expect(detail).not.toContain("Xem video");
  });

  it("a failed tap: one sentence under the button, saying what to do next — words, no code", () => {
    const page = render({ onWatchVideo: noop, videoFailed: true });
    expect(page).toContain(
      `<div class="xa-bai__video"><button type="button" class="xa-nut">Xem video</button>` +
        `<p class="xa-error-box" role="alert">${XA_TN.watch_video_failed}</p></div>`,
    );
    expect(XA_TN.watch_video_failed).toMatch(/bấm “Xem video” lần nữa/);
    expect(XA_TN.watch_video_failed).not.toMatch(/\d{3}|lỗi|error|demo|trải nghiệm/i);
  });

  it("the tap reaches the caller's handler: article → WatchVideo → the button's onClick", () => {
    let taps = 0;
    const onWatchVideo = () => taps++;
    const tree = NewsArticle({ bai: video, coverFailed: false, onCoverFail: noop, ds: [], onWatchVideo });
    const button = (tree.props as { children: unknown[] }).children.find(
      (c) => isValidElement(c) && c.type === WatchVideo,
    ) as ReactElement<{ onTap: () => void }>;
    expect(button.props.onTap).toBe(onWatchVideo);
    const el = WatchVideo({ failed: false, onTap: onWatchVideo });
    const inner = (el.props as { children: unknown[] }).children[0] as ReactElement<{ onClick: () => void }>;
    inner.props.onClick();
    expect(taps).toBe(1);
  });

  it("one tap calls the injected opener with EXACTLY the commune's link, once; failed = it did not open", async () => {
    const calls: string[] = [];
    const opener = (answer: boolean) => async (url: string) => {
      calls.push(url);
      return answer;
    };
    expect(await videoOpenFailed(opener(true), CLIP)).toBe(false);
    expect(calls).toEqual([CLIP]);
    expect(await videoOpenFailed(opener(false), CLIP)).toBe(true);
    // A rejected opener is a failure too, never an unhandled rejection under a button.
    expect(await videoOpenFailed(() => Promise.reject(new Error("x")), CLIP)).toBe(true);
    expect(calls).toEqual([CLIP, CLIP]);
  });

  it("wired in the commune's own app only, through the declared destination 'video'", () => {
    const raw = import.meta.glob(["../../App.tsx", "./TinTucXaScreen.tsx", "./TinTucAppXa.tsx"], {
      query: "?raw",
      import: "default",
      eager: true,
    }) as Record<string, string>;
    const app = raw["../../App.tsx"]!;
    const own = app.slice(app.indexOf("export function AppRieng("), app.indexOf("function AppChung("));
    expect(own).toMatch(/openVideo=\{openCommuneVideo\}/);
    expect(app).toMatch(/const openCommuneVideo: OpenVideo = \(url\) => moRaNgoai\("video", url\);/);
    expect(app.slice(app.indexOf("function AppChung("))).not.toMatch(/openVideo|openCommuneVideo/);
    // The shared app's news screen is untouched (owner, 01/10/2026: commune app only).
    expect(raw["./TinTucXaScreen.tsx"]).not.toMatch(/Xem video|watch_video|openVideo/);
    // And the state half itself opens nothing: it only calls what it was given. Comments stripped — they may NAME
    // the door they do not use (same expression as `ranh-gioi-hai-nua.test.ts`).
    const code = raw["./TinTucAppXa.tsx"]!.replace(/\/\*[\s\S]*?\*\//g, " ").replace(/(?<!:)\/\/[^\n]*/g, " ");
    expect(code.length).toBeGreaterThan(1000);
    expect(code).not.toMatch(/moRaNgoai|moTrangWeb|zmp-sdk|window\s*\.\s*open/);
  });

  it("the block sets spacing only — the button's size and colour are `xa-nut`'s, already measured", async () => {
    const nodeFs = "node:fs";
    const { readFileSync } = (await import(/* @vite-ignore */ nodeFs)) as {
      readFileSync: (path: URL, encoding: "utf8") => string;
    };
    const css = readFileSync(new URL("../../styles.css", import.meta.url), "utf8");
    const block = css.match(/\.xa-bai__video \{([^}]*)\}/);
    expect(block, "no .xa-bai__video rule").not.toBeNull();
    expect(block![1]).not.toMatch(/font-size|color|background|height/);
    expect(css).toMatch(/\.xa-nut \{[^}]*min-height: calc\(var\(--tap-min\) \+ 4px\);[^}]*font-size: var\(--text-body\);/);
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
