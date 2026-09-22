import type { ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import { docYeuCauCuaToi, guiYeuCau } from "../../api/goi-may-chu";
import {
  conLaiGhiChu,
  demKyTu,
  docCauLoi,
  docDanhSach,
  docMaYeuCau,
  MA_HOP_LE,
  MA_TRANG_THAI,
  SO_QUAN_TAM_TOI_DA,
  thanYeuCau,
  TRAN_GHI_CHU,
  TRUONG_GUI_DI,
  type YeuCauMoi,
} from "../../api/hop-dong-yeu-cau";
import { CHIEN_DICH } from "../../content/chien-dich";
import { SOLUTIONS } from "../../content/company-profile";
import { NhaCungCapPhien } from "../dang-nhap/kho-phien";
import { QUY_MO } from "../goi-y-giai-phap/anh-xa";

import { MAN_TU_VAN, MAN_YEU_CAU } from "./index";
import { GIAI_THICH_TRANG_THAI, ngayDoc, NHAN_LOAI, NHAN_TRANG_THAI } from "./trang-thai";
import { TuVanBaoGiaScreen } from "./TuVanBaoGiaScreen";
import { DongYeuCau, ThanDanhSach, YeuCauCuaToiScreen } from "./YeuCauCuaToiScreen";
import { YEU_CAU_CUA_TOI } from "./noi-dung";

const ve = (node: ReactElement) => renderToStaticMarkup(node);
const chuCua = (markup: string) =>
  markup.replace(/<[^>]+>/g, " ").replace(/&amp;/g, "&").replace(/\s+/g, " ");

const MAU: YeuCauMoi = {
  loai: "consult",
  quan_tam: ["messaging", "voice-ai"],
  quy_mo: "10-50",
  ghi_chu: "  Cho tôi xin báo giá tổng đài.  ",
  nguon: "vp-quay",
};

const DIA_CHI_GIA = "https://vi-du.test/api/v1/requests";

afterEach(() => {
  vi.unstubAllGlobals();
});

/** Một `fetch` giả trả về đúng một câu trả lời. Ghi lại lời gọi để soi thân và tiêu đề. */
function fetchGia(status: number, than: unknown) {
  const da_goi: { dia_chi: string; tuy_chon: RequestInit }[] = [];
  vi.stubGlobal("fetch", (dia_chi: string, tuy_chon: RequestInit) => {
    da_goi.push({ dia_chi, tuy_chon });
    return Promise.resolve({
      status,
      ok: status >= 200 && status < 300,
      json: () => Promise.resolve(than),
    });
  });
  return da_goi;
}

/* ============================================================================================
   THÂN YÊU CẦU — ĐỊNH DANH ĐẾN TỪ PHIÊN, KHÔNG TỪ THÂN
   ============================================================================================ */

describe("thân yêu cầu không mang một trường nào nói 'tôi là ai'", () => {
  /**
   * ⚠ CA ĐẮT NHẤT CỦA TỆP NÀY (luật 4, bất biến 2).
   *
   *   Máy chủ lấy người dùng TỪ PHIÊN và có ca kiểm nhồi `nguoi_dung_id`/`userId`/`phone` vào thân
   *   để chứng minh chúng bị bỏ qua. Nửa này canh phần của mình: một trường định danh THÊM vào
   *   `thanYeuCau` sẽ không làm máy chủ sai — nó chỉ gửi số điện thoại của một người thật lên dây
   *   mà không ai yêu cầu, và không có gì đỏ lên, vì phản hồi vẫn là 201.
   *
   *   Nên ca này khẳng định TẬP KHOÁ ĐÚNG BẰNG bảng khai, không phải "không chứa những tên xấu":
   *   một danh sách tên xấu luôn thiếu đúng cái tên người sau nghĩ ra.
   */
  it("tập khoá gửi đi ĐÚNG BẰNG bảng khai, không thừa một khoá nào", () => {
    const khoa = Object.keys(JSON.parse(thanYeuCau(MAU)) as Record<string, unknown>).sort();
    expect(khoa).toEqual(TRUONG_GUI_DI.map((t) => t.khoa).sort());
  });

  it("không một hình dạng nào của 'tôi là ai' lọt vào thân", () => {
    // Vế thứ hai, cố ý thừa: nếu ai đó đổi cả bảng khai LẪN thân cùng lúc, ca trên vẫn xanh.
    // Ca này thì không — nó đọc thẳng chuỗi JSON đi lên dây.
    const than = thanYeuCau(MAU).toLowerCase();
    for (const xau of [
      "nguoi_dung_id",
      "nguoidungid",
      "userid",
      "user_id",
      "phone",
      "so_dien_thoai",
      "sodienthoai",
      "token",
      "zalo",
    ]) {
      expect(than, `thân yêu cầu mang trường định danh "${xau}"`).not.toContain(xau);
    }
  });

  it("cắt khoảng trắng thừa của ghi chú trước khi gửi", () => {
    const than = JSON.parse(thanYeuCau(MAU)) as Record<string, unknown>;
    expect(than["note"]).toBe("Cho tôi xin báo giá tổng đài.");
  });
});

/* ============================================================================================
   TRẦN GHI CHÚ — ĐẾM THEO KÝ TỰ, KHÔNG THEO BYTE
   ============================================================================================ */

describe("ghi chú đếm theo KÝ TỰ, không theo byte", () => {
  /**
   * ⚠ ĐÂY LÀ CHỖ SAI IM LẶNG NHẤT CỦA CẢ TUYẾN NÀY.
   *
   *   Máy chủ đếm bằng `utf8.RuneCountInString`. Một ký tự tiếng Việt có dấu tốn tới 3 byte, nên
   *   2000 ký tự tiếng Việt vượt xa 2000 byte. Đếm theo byte ở client thì ô ghi chú báo "còn 0 ký
   *   tự" khi người dùng mới gõ chừng một phần ba mức thật — một cái trần thấp hơn trần thật ba
   *   lần, mà KHÔNG CÓ GÌ ĐỎ LÊN, vì máy chủ vẫn nhận mọi thứ client chịu gửi.
   */
  it("2000 ký tự tiếng Việt vẫn vừa trần, dù chúng vượt 2000 BYTE", () => {
    const dai = "ệ".repeat(TRAN_GHI_CHU);
    expect(demKyTu(dai)).toBe(TRAN_GHI_CHU);
    expect(conLaiGhiChu(dai), "đang đếm theo BYTE chứ không theo ký tự").toBe(0);

    // Bằng chứng rằng hai cách đếm KHÁC NHAU thật — không có dòng này thì ca trên cũng xanh với
    // một chuỗi ASCII, và nó sẽ xanh vì lý do sai.
    const so_byte = new TextEncoder().encode(dai).length;
    expect(so_byte, "chuỗi thử không còn vượt byte — ca trên mất nội dung").toBeGreaterThan(
      TRAN_GHI_CHU,
    );
  });

  it("vượt một ký tự thì còn lại âm, và màn hình chặn nút gửi", () => {
    expect(conLaiGhiChu("ệ".repeat(TRAN_GHI_CHU + 1))).toBe(-1);
  });

  it("đếm theo rune chứ không theo đơn vị mã UTF-16", () => {
    // Một emoji là MỘT ký tự với `Array.from`, nhưng `.length` đếm hai. Người dùng gõ emoji vào
    // ô ghi chú là chuyện bình thường, và một cái trần tụt đi một nửa vì chúng là một cái trần sai.
    expect(demKyTu("👍")).toBe(1);
    expect("👍".length).toBe(2);
  });
});

/* ============================================================================================
   MÃ GỬI ĐI PHẢI KHỚP KHUÔN CỦA MÁY CHỦ
   ============================================================================================ */

describe("mọi mã ứng dụng có thể gửi đều khớp khuôn máy chủ", () => {
  /**
   * `MA_HOP_LE` là bản chép của `maNgan` bên máy chủ. Một bản chép chỉ đáng giá khi có ca cho nó
   * ăn MỌI mã có thật trong ứng dụng — không có ca này thì nó chỉ lặp lại một luật mà không ai
   * đối chiếu, và ngày ai đó thêm một `SolutionId` viết hoa thì lỗi hiện ra ở người dùng dưới
   * dạng một câu 400 không ai hiểu.
   */
  it("id của mọi dòng giải pháp khớp `^[a-z0-9][a-z0-9_-]{0,63}$`", () => {
    expect(SOLUTIONS.length).toBeGreaterThan(0);
    for (const gp of SOLUTIONS) {
      expect(MA_HOP_LE.test(gp.id), `id sản phẩm "${gp.id}" máy chủ sẽ trả 400`).toBe(true);
    }
  });

  it("mọi mã quy mô và mọi mã chiến dịch cũng vậy", () => {
    for (const q of QUY_MO) {
      expect(MA_HOP_LE.test(q.ma), `mã quy mô "${q.ma}" máy chủ sẽ trả 400`).toBe(true);
    }
    const ma_chien_dich = Object.keys(CHIEN_DICH);
    expect(ma_chien_dich.length).toBeGreaterThan(0);
    for (const ma of ma_chien_dich) {
      expect(MA_HOP_LE.test(ma), `mã chiến dịch "${ma}" máy chủ sẽ trả 400`).toBe(true);
    }
  });

  it("và khuôn ấy KHÔNG kêu oan, cũng KHÔNG bỏ lọt", () => {
    for (const xau of ["Viết Hoa", "có-dấu", "", "-bat-dau-bang-gach", "a".repeat(65)]) {
      expect(MA_HOP_LE.test(xau), `khuôn bỏ lọt mã xấu: "${xau}"`).toBe(false);
    }
  });

  it("số dòng sản phẩm không vượt trần của máy chủ", () => {
    expect(SOLUTIONS.length).toBeLessThanOrEqual(SO_QUAN_TAM_TOI_DA);
  });
});

/* ============================================================================================
   ĐỌC PHẢN HỒI — FAIL CLOSED, KHÔNG SUY DIỄN
   ============================================================================================ */

describe("đọc phản hồi của máy chủ", () => {
  it("danh sách rỗng, `null`, hay thân lạ đều ra một mảng rỗng, không ném", () => {
    for (const than of [null, {}, { items: null }, { items: "x" }, 7]) {
      expect(docDanhSach(than)).toEqual([]);
    }
  });

  it("một hàng mang mã trạng thái TA CHƯA BIẾT bị bỏ qua, KHÔNG bị quy về `moi`", () => {
    // Quy nó về `moi` là hiện một trạng thái SAI cho một yêu cầu có thật — người dùng đọc "Đã
    // tiếp nhận" trong khi máy chủ đang nói điều khác, và không ai hỏi lại vì nó trông đúng.
    const ra = docDanhSach({
      items: [
        { requestId: "A", kind: "consult", status: "moi", createdAt: "2026-09-22T03:00:00Z" },
        { requestId: "B", kind: "consult", status: "trang_thai_moi_cua_may_chu", createdAt: "" },
        { requestId: "C", kind: "loai_la", status: "moi", createdAt: "" },
      ],
    });
    expect(ra.map((m) => m.ma)).toEqual(["A"]);
  });

  it("mã yêu cầu rỗng hoặc thiếu thì trả `null`, không trả chuỗi 'undefined'", () => {
    expect(docMaYeuCau({ requestId: "YC-1" })).toBe("YC-1");
    for (const than of [null, {}, { requestId: "" }, { requestId: 7 }]) {
      expect(docMaYeuCau(than)).toBeNull();
    }
  });

  it("câu lỗi đọc nguyên văn, và thân không có câu thì trả `null`", () => {
    expect(docCauLoi({ message: "Bạn đã yêu cầu gọi lại 3 lần trong 24 giờ qua." })).toBe(
      "Bạn đã yêu cầu gọi lại 3 lần trong 24 giờ qua.",
    );
    for (const than of [null, {}, { message: "   " }, { message: 7 }]) {
      expect(docCauLoi(than)).toBeNull();
    }
  });
});

/* ============================================================================================
   TẦNG GỌI MẠNG — NĂM NHÁNH, VÀ TIÊU ĐỀ XÁC THỰC
   ============================================================================================ */

describe("gửi yêu cầu: mỗi mã trạng thái một việc phải làm tiếp", () => {
  it("chưa khai host thì KHÔNG gọi mạng — fail closed, không đoán địa chỉ", async () => {
    const da_goi = fetchGia(201, {});
    expect(await guiYeuCau("bearer", MAU, "")).toEqual({ kieu: "chua-khai-host" });
    expect(da_goi.length, "đã gọi mạng dù chưa khai host").toBe(0);
  });

  it("chưa có phiên thì KHÔNG gọi mạng — một yêu cầu không thuộc về ai là một hàng không ai tra ra", async () => {
    const da_goi = fetchGia(201, {});
    expect(await guiYeuCau("", MAU, DIA_CHI_GIA)).toEqual({ kieu: "chua-dang-nhap" });
    expect(da_goi.length).toBe(0);
  });

  it("201 trả về mã yêu cầu, và lời gọi mang đúng tiêu đề Bearer", async () => {
    const da_goi = fetchGia(201, { requestId: "YC-7", kind: "consult", status: "moi" });
    expect(await guiYeuCau("phieu-abc", MAU, DIA_CHI_GIA)).toEqual({
      kieu: "xong",
      ma_yeu_cau: "YC-7",
    });
    const tieu_de = da_goi[0]!.tuy_chon.headers as Record<string, string>;
    expect(tieu_de["Authorization"]).toBe("Bearer phieu-abc");
    expect(da_goi[0]!.tuy_chon.method).toBe("POST");
    // VÀ PHIẾU PHIÊN KHÔNG NẰM TRONG THÂN. Nó là một tiêu đề, không phải một trường dữ liệu.
    expect(String(da_goi[0]!.tuy_chon.body)).not.toContain("phieu-abc");
  });

  it("401 → mời đăng nhập lại, không phải một câu lỗi chung chung", async () => {
    fetchGia(401, { message: "Bạn chưa đăng nhập." });
    expect(await guiYeuCau("het-han", MAU, DIA_CHI_GIA)).toEqual({ kieu: "chua-dang-nhap" });
  });

  it("429 và 503 giữ NGUYÊN VĂN câu của máy chủ — client không viết lại", async () => {
    // ⚠ VIẾT LẠI CÂU ẤY LÀ DỰNG CHỖ THỨ HAI GIỮ CÙNG MỘT CÂU. Câu của máy chủ mang những con số
    // của máy chủ (trần 3 lượt/24 giờ), và ngày trần đổi thì chỗ thứ hai vẫn nói số cũ.
    const cau_429 = "Bạn đã yêu cầu gọi lại 3 lần trong 24 giờ qua. Vui lòng gọi hotline nếu cần gấp.";
    fetchGia(429, { message: cau_429 });
    expect(await guiYeuCau("p", MAU, DIA_CHI_GIA)).toEqual({ kieu: "tu-choi", cau: cau_429 });

    const cau_503 = "Chức năng gọi lại đang tạm ngưng. Vui lòng gọi hotline để được hỗ trợ ngay.";
    fetchGia(503, { message: cau_503 });
    expect(await guiYeuCau("p", MAU, DIA_CHI_GIA)).toEqual({ kieu: "tu-choi", cau: cau_503 });
  });

  it("400 cũng là `tu-choi`, và thân lỗi không phải JSON thì câu rỗng chứ không ném", async () => {
    vi.stubGlobal("fetch", () =>
      Promise.resolve({
        status: 400,
        ok: false,
        json: () => Promise.reject(new Error("not json")),
      }),
    );
    expect(await guiYeuCau("p", MAU, DIA_CHI_GIA)).toEqual({ kieu: "tu-choi", cau: "" });
  });

  it("mất mạng thì nói việc cần làm tiếp, không để màn hình treo ở 'đang gửi'", async () => {
    vi.stubGlobal("fetch", () => Promise.reject(new Error("mạng hỏng")));
    expect(await guiYeuCau("p", MAU, DIA_CHI_GIA)).toEqual({ kieu: "khong-goi-duoc" });
  });
});

describe("đọc danh sách: không một tham số nào nói 'của ai'", () => {
  it("gọi GET với Bearer, và URL không mang một tham số truy vấn nào", async () => {
    // ⚠ Luật 4, cấm #1: `GET /requests?phone=…` là đổi một tham số để đọc dữ liệu của người khác.
    // Việc URL KHÔNG CÓ tham số nào chính là tính năng, nên nó có một ca.
    const da_goi = fetchGia(200, { items: [] });
    await docYeuCauCuaToi("phieu-xyz", DIA_CHI_GIA);
    expect(da_goi[0]!.dia_chi).toBe(DIA_CHI_GIA);
    expect(da_goi[0]!.dia_chi, "URL mang một tham số truy vấn").not.toContain("?");
    expect(da_goi[0]!.tuy_chon.method).toBe("GET");
    expect((da_goi[0]!.tuy_chon.headers as Record<string, string>)["Authorization"]).toBe(
      "Bearer phieu-xyz",
    );
  });

  it("401 → mời đăng nhập lại; mất mạng → một câu nói việc cần làm", async () => {
    fetchGia(401, {});
    expect(await docYeuCauCuaToi("p", DIA_CHI_GIA)).toEqual({ kieu: "chua-dang-nhap" });
    vi.stubGlobal("fetch", () => Promise.reject(new Error("x")));
    expect(await docYeuCauCuaToi("p", DIA_CHI_GIA)).toEqual({ kieu: "khong-goi-duoc" });
  });
});

/* ============================================================================================
   NHÃN TRẠNG THÁI — MỘT CHỖ, KHÔNG CAM KẾT THỜI GIAN
   ============================================================================================ */

describe("bốn mã trạng thái, bốn nhãn tiếng Việt, một chỗ", () => {
  it("mỗi mã có một nhãn và một câu giải thích, không mã nào rỗng", () => {
    expect(MA_TRANG_THAI.length).toBe(4);
    for (const ma of MA_TRANG_THAI) {
      expect(NHAN_TRANG_THAI[ma], `mã "${ma}" không có nhãn`).not.toBe("");
      expect(GIAI_THICH_TRANG_THAI[ma], `mã "${ma}" không có câu giải thích`).not.toBe("");
    }
  });

  it("không nhãn nào in một mã kỹ thuật ra màn hình", () => {
    for (const ma of MA_TRANG_THAI) {
      expect(NHAN_TRANG_THAI[ma], `nhãn của "${ma}" in chính cái mã ra`).not.toContain(ma);
    }
  });

  it("không nhãn và không câu giải thích nào mang một cam kết thời gian", () => {
    // Một thời hạn in ra màn hình là một cam kết của bên phát hành, không phải một câu trấn an —
    // và người phát hiện ra nó sai là người đang chờ.
    const tat_ca = [
      ...Object.values(NHAN_TRANG_THAI),
      ...Object.values(GIAI_THICH_TRANG_THAI),
      ...Object.values(NHAN_LOAI),
    ].join("\n");
    expect(tat_ca).not.toMatch(/trong \d+ (giây|phút|giờ|ngày)/);
  });

  it("ngày giờ lạ thì trả chuỗi rỗng, không trả 'Invalid Date'", () => {
    expect(ngayDoc("")).toBe("");
    expect(ngayDoc("khong-phai-ngay")).toBe("");
    expect(ngayDoc("2026-09-22T03:00:00Z")).not.toBe("");
  });
});

/* ============================================================================================
   MÀN HÌNH — CHƯA ĐĂNG NHẬP THÌ KHÔNG CÓ BIỂU MẪU
   ============================================================================================ */

describe("chưa đăng nhập thì không vẽ biểu mẫu, và không vẽ một danh sách rỗng", () => {
  /**
   * Bộ test dựng bằng `react-dom/server`, không có nhà cung cấp phiên — tức là đúng trạng thái
   * "chưa đăng nhập". Đó cũng là lý do `kho-phien.tsx` trả "chưa đăng nhập" thay vì ném: một
   * `throw` ở đây là một màn trắng trên máy thật.
   */
  it("màn Tư vấn mời đăng nhập, và KHÔNG có một ô nhập nào", () => {
    const markup = ve(<TuVanBaoGiaScreen />);
    expect(markup, "biểu mẫu vẽ ra khi chưa đăng nhập — người dùng sẽ gõ rồi mất chữ").not.toMatch(
      /<(form|input|textarea|select)\b/,
    );
    const chu = chuCua(markup);
    expect(chu).toContain("Bạn cần đăng nhập trước");
    // HOTLINE Ở LẠI: đường duy nhất chạy được khi chưa đăng nhập.
    expect(markup).toContain('href="tel:');
  });

  it("màn Yêu cầu mời đăng nhập, KHÔNG nói 'bạn chưa gửi yêu cầu nào'", () => {
    // Một danh sách rỗng khi chưa đăng nhập đọc ra thành một câu SAI, và người đọc nó sẽ gửi lại.
    const chu = chuCua(ve(<YeuCauCuaToiScreen />));
    expect(chu).toContain(YEU_CAU_CUA_TOI.can_dang_nhap_tieu_de);
    expect(chu, "nói 'chưa gửi yêu cầu nào' trong khi chỉ là chưa đăng nhập").not.toContain(
      YEU_CAU_CUA_TOI.rong,
    );
  });

  it("mỗi màn có ĐÚNG MỘT `<h1>`, và mọi glyph đều bị giấu khỏi trình đọc màn hình", () => {
    for (const [ten, markup] of [
      ["tu-van", ve(<TuVanBaoGiaScreen />)],
      ["yeu-cau", ve(<YeuCauCuaToiScreen />)],
    ] as const) {
      expect((markup.match(/<h1\b/g) ?? []).length, `${ten} không có đúng một <h1>`).toBe(1);
      const svgs = markup.match(/<svg[\s>][^>]*>/g) ?? [];
      expect(svgs.length, `${ten} không vẽ một glyph nào`).toBeGreaterThan(0);
      for (const svg of svgs) {
        expect(svg, `${ten} vẽ một <svg> trình đọc màn hình sẽ đọc ra`).toContain(
          'aria-hidden="true"',
        );
      }
    }
  });

  it("thanh tiêu đề và `<h1>` của mỗi màn là MỘT chuỗi, không hai", () => {
    for (const man of [MAN_TU_VAN, MAN_YEU_CAU]) {
      expect(man.cho.kieu).toBe("ngoai-tab");
      expect(man.cho.tabSangLen).toBe("home");
      expect(chuCua(ve(<man.component />))).toContain(man.headerTitle);
    }
  });
});

/* ============================================================================================
   ĐÃ ĐĂNG NHẬP — TRẠNG THÁI DUY NHẤT CỦA CẢ ỨNG DỤNG CÓ MỘT Ô NHẬP
   ============================================================================================ */

describe("đã đăng nhập: biểu mẫu hiện ra, và nó có ĐÚNG MỘT ô nhập", () => {
  /**
   * ⚠ KHÔNG CÓ KHỐI NÀY THÌ TRẠNG THÁI CÓ BIỂU MẪU KHÔNG CA KIỂM NÀO CHẠM TỚI.
   *
   *   `SCREEN_MARKUP` của `screens.test.tsx` dựng màn này KHÔNG có nhà cung cấp phiên, tức luôn
   *   ở nhánh "mời đăng nhập" — nhánh không có ô nhập nào. Mọi bất biến của nhánh CÓ biểu mẫu
   *   (đúng một `<h1>`, glyph bị giấu, đúng một ô nhập, không dữ liệu cá nhân viết cứng) vì thế
   *   nằm ngoài tầm với của lượt quét ấy, và đây là nơi chúng được đo.
   */
  const daDangNhap = () =>
    ve(
      <NhaCungCapPhien phien_ban_dau={{ token: "phieu-thu", het_han: "2026-09-29T03:00:00Z" }}>
        <TuVanBaoGiaScreen />
      </NhaCungCapPhien>,
    );

  it("vẽ ĐÚNG MỘT `<textarea>`, và KHÔNG một `<form>`/`<input>`/`<select>` nào", () => {
    const markup = daDangNhap();
    expect((markup.match(/<textarea\b/g) ?? []).length, "số ô ghi chú vừa đổi").toBe(1);
    expect(markup, "một ô nhập thứ hai vừa xuất hiện").not.toMatch(/<(form|input|select)\b/);
  });

  it("ô ghi chú có `<label>` thật nối đúng `id`, và một dòng đếm ngược đọc được", () => {
    const markup = daDangNhap();
    const id = /<textarea[^>]*\bid="([^"]+)"/.exec(markup)?.[1];
    expect(id, "ô ghi chú không còn `id` — `<label htmlFor>` không nối vào đâu").toBeDefined();
    expect(markup, "nhãn không trỏ vào ô").toContain(`for="${id!}"`);
    expect(markup, "dòng đếm ngược không được nối bằng aria-describedby").toContain(
      `aria-describedby="${id!}-dem"`,
    );
    expect(chuCua(markup)).toContain(`Còn ${TRAN_GHI_CHU} ký tự`);
  });

  it("vẫn đúng một `<h1>`, và mọi glyph vẫn bị giấu khỏi trình đọc màn hình", () => {
    const markup = daDangNhap();
    expect((markup.match(/<h1\b/g) ?? []).length).toBe(1);
    for (const svg of markup.match(/<svg[\s>][^>]*>/g) ?? []) {
      expect(svg).toContain('aria-hidden="true"');
    }
  });

  it("hiện đủ chữ của mọi dòng sản phẩm và mọi mức quy mô, và KHÔNG hiện một mã nào", () => {
    const chu = chuCua(daDangNhap());
    for (const gp of SOLUTIONS) {
      expect(chu, `thiếu lựa chọn sản phẩm: ${gp.product ?? gp.headline}`).toContain(
        gp.product ?? gp.headline,
      );
      expect(chu, `mã sản phẩm "${gp.id}" lọt lên màn hình`).not.toContain(gp.id);
    }
    for (const q of QUY_MO) {
      expect(chu, `thiếu mức quy mô: ${q.nhan}`).toContain(q.nhan);
    }
  });

  it("nói ra TRẦN GỌI LẠI ngay trên màn, không để người dùng gặp nó dưới dạng một câu từ chối", () => {
    expect(chuCua(daDangNhap())).toContain("tối đa 3 lần trong 24 giờ");
  });

  it("KHÔNG in một cam kết thời gian nào, và KHÔNG hứa chắc có tin ZNS", () => {
    const chu = chuCua(daDangNhap());
    expect(chu, "màn in một cam kết thời gian").not.toMatch(/trong \d+ (giây|phút)/);
    expect(chu, "hứa chắc rằng sẽ có tin ZNS").not.toMatch(/[Bb]ạn sẽ nhận được (một )?tin/);
  });

  it("nói ra rằng phiên không được ghi xuống máy — người dùng không nên tự phát hiện", () => {
    expect(chuCua(daDangNhap())).toContain("không ghi phiên đăng nhập xuống máy bạn");
  });
});

describe("danh sách yêu cầu nói ra mọi nhánh, kể cả nhánh rỗng", () => {
  it("rỗng thì có một câu và một lối đi tiếp, không phải một màn trắng", () => {
    const chu = chuCua(
      ve(<ThanDanhSach ket_qua={{ kieu: "xong", danh_sach: [] }} onDangNhapLai={() => {}} onGuiMoi={() => {}} />),
    );
    expect(chu).toContain(YEU_CAU_CUA_TOI.rong);
    expect(chu).toContain(YEU_CAU_CUA_TOI.nut_gui_moi);
  });

  it("một dòng in đủ: loại · tình trạng bằng CHỮ · mã · thời điểm", () => {
    const chu = chuCua(
      ve(
        <DongYeuCau
          yeu_cau={{
            ma: "YC-42",
            loai: "callback",
            trang_thai: "dang_xu_ly",
            tao_luc: "2026-09-22T03:00:00Z",
          }}
        />,
      ),
    );
    expect(chu).toContain(NHAN_LOAI.callback);
    expect(chu, "tình trạng không nói bằng chữ").toContain(NHAN_TRANG_THAI.dang_xu_ly);
    expect(chu).toContain(GIAI_THICH_TRANG_THAI.dang_xu_ly);
    expect(chu).toContain("YC-42");
    // MÃ KỸ THUẬT KHÔNG BAO GIỜ HIỆN RA — người dùng đọc nhãn, không đọc `dang_xu_ly`.
    expect(chu, "mã trạng thái kỹ thuật lọt lên màn hình").not.toContain("dang_xu_ly");
  });

  it("mỗi nhánh hỏng có một câu nói việc cần làm tiếp, không một mã lỗi nào", () => {
    for (const kieu of ["chua-dang-nhap", "chua-khai-host", "khong-goi-duoc"] as const) {
      const chu = chuCua(
        ve(<ThanDanhSach ket_qua={{ kieu }} onDangNhapLai={() => {}} onGuiMoi={() => {}} />),
      );
      expect(chu.trim(), `nhánh ${kieu} không nói gì cả`).not.toBe("");
      expect(chu, `nhánh ${kieu} in một mã lỗi`).not.toMatch(/\b(401|429|503|undefined|null)\b/);
    }
  });
});
