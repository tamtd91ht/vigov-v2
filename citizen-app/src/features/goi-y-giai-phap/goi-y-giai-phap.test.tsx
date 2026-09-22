import type { ReactElement } from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { SOLUTIONS } from "../../content/company-profile";

import {
  ANH_XA_VIEC,
  CAU_HOI_NGANH,
  CAU_HOI_QUY_MO,
  CAU_HOI_VIEC,
  giaiPhapGoiY,
  type MaViec,
  NGANH,
  nhanCua,
  QUY_MO,
  VIEC,
} from "./anh-xa";
import { BuocChon, GoiYGiaiPhapScreen, KetQuaGoiY, LOI } from "./GoiYGiaiPhapScreen";
import { MAN_GOI_Y_GIAI_PHAP } from "./index";

const ve = (node: ReactElement) => renderToStaticMarkup(node);
/**
 * Chữ người đọc thấy, từ một chuỗi HTML.
 *
 * GIẢI MÃ THỰC THỂ HTML, KHÔNG CHỈ BÓC THẺ: `renderToStaticMarkup` thoát `&` thành `&amp;`, nên
 * một câu đã công bố như "Messaging & Voice" không bao giờ khớp nếu chỉ bóc thẻ — và ca kiểm sẽ
 * đỏ vì một lý do không liên quan gì tới thứ nó hỏi.
 */
const chuCua = (markup: string) =>
  markup
    .replace(/<[^>]+>/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#x27;|&#39;/g, "'")
    .replace(/\s+/g, " ");

/* ============================================================================================
   BẢNG ÁNH XẠ — ĐÂY LÀ CA ĐẮT NHẤT CỦA CẢ TỆP
   ============================================================================================ */

describe("bảng ánh xạ trỏ vào danh mục sản phẩm THẬT", () => {
  /**
   * ⚠ KHÔNG CÓ CA NÀY THÌ MỘT ID GÕ SAI LÀ MỘT LỖI HOÀN TOÀN IM LẶNG.
   *
   *   `giaiPhapGoiY` lọc `SOLUTIONS`. Một id không tồn tại không ném, không cảnh báo, không vẽ
   *   một thẻ rỗng — nó chỉ biến mất khỏi kết quả. Ngày ai đó đổi danh mục sản phẩm (đổi tên một
   *   `SolutionId`, gỡ một dòng), màn này lặng lẽ gợi ý thiếu, hoặc gợi ý RỖNG, và mọi ca "màn có
   *   vẽ ra không" ở dưới vẫn xanh.
   */
  it("mọi id trong bảng ánh xạ đều tồn tại trong SOLUTIONS", () => {
    const co_that = new Set(SOLUTIONS.map((giai_phap) => giai_phap.id));
    expect(co_that.size, "danh mục sản phẩm rỗng — ca này sẽ xanh vì lý do sai").toBeGreaterThan(0);

    for (const [viec, ids] of Object.entries(ANH_XA_VIEC)) {
      expect(ids.length, `việc "${viec}" không ánh xạ tới dòng sản phẩm nào`).toBeGreaterThan(0);
      for (const id of ids) {
        expect(
          co_that,
          `bảng ánh xạ trỏ tới "${id}", một id KHÔNG có trong SOLUTIONS — việc "${viec}" sẽ gợi ý thiếu`,
        ).toContain(id);
      }
    }
  });

  it("mọi lựa chọn ở bước ba đều có một dòng trong bảng ánh xạ", () => {
    // Mặt kia của ca trên: một lựa chọn hiện ra trên màn mà bảng không có dòng cho nó là một nút
    // dẫn tới một kết quả rỗng.
    for (const mot of VIEC) {
      expect(Object.keys(ANH_XA_VIEC), `bước ba có lựa chọn "${mot.ma}" mà bảng không khai`).toContain(
        mot.ma,
      );
      expect(giaiPhapGoiY(mot.ma).length, `lựa chọn "${mot.ma}" cho ra gợi ý rỗng`).toBeGreaterThan(0);
    }
  });

  it("giữ đúng thứ tự SOLUTIONS công bố, không đảo theo bảng ánh xạ", () => {
    // `da-kenh` ánh xạ tới `["voice-ai", "messaging"]`, nhưng `SOLUTIONS` công bố `messaging`
    // trước. Người dùng vừa xem màn Giải pháp phải gặp lại đúng thứ tự ấy.
    const ids = giaiPhapGoiY("da-kenh").map((giai_phap) => giai_phap.id);
    const thu_tu_cong_bo = SOLUTIONS.filter((giai_phap) => ids.includes(giai_phap.id)).map(
      (giai_phap) => giai_phap.id,
    );
    expect(ids).toEqual(thu_tu_cong_bo);
  });

  it("không dựng một danh sách sản phẩm thứ hai — chữ đọc thẳng từ SOLUTIONS", () => {
    for (const mot of VIEC) {
      const chu = chuCua(
        ve(<KetQuaGoiY tra_loi={{ viec: mot.ma }} viec={mot.ma} onXemGiaiPhap={() => {}} />),
      );
      for (const giai_phap of giaiPhapGoiY(mot.ma)) {
        expect(chu, `gợi ý cho "${mot.ma}" thiếu câu đã công bố`).toContain(giai_phap.headline);
      }
    }
  });
});

/* ============================================================================================
   BỘ CHỌN BA BƯỚC
   ============================================================================================ */

describe("ba bước, mỗi bước 3–4 lựa chọn, tất cả là nút", () => {
  it("mỗi bước có từ ba tới bốn lựa chọn", () => {
    for (const [ten, danh_sach] of [
      ["lĩnh vực", NGANH],
      ["quy mô", QUY_MO],
      ["việc đang cần", VIEC],
    ] as const) {
      expect(danh_sach.length, `bước "${ten}" có ${danh_sach.length} lựa chọn`).toBeGreaterThanOrEqual(3);
      expect(danh_sach.length, `bước "${ten}" có ${danh_sach.length} lựa chọn`).toBeLessThanOrEqual(4);
      expect(new Set(danh_sach.map((mot) => mot.ma)).size, `bước "${ten}" có hai mã trùng`).toBe(
        danh_sach.length,
      );
    }
  });

  /**
   * ⚠ ĐÂY LÀ CÁI BẪY CHÍNH CỦA MÀN NÀY, VÀ NÓ ĐƯỢC CANH Ở HAI CHỖ.
   *
   *   `phase1-collects-nothing.test.ts` quét MÃ NGUỒN và cấm `<form|input|textarea|select>` ở mọi
   *   tệp. Ca dưới quét BẢN DỰNG RA của đúng màn này. Hai hình dạng vì một lý do: phép quét mã
   *   nguồn không thấy được một `<select>` sinh ra từ một biến, và phép dựng không thấy được một
   *   nhánh chưa chạy tới. Một bộ chọn là đúng chỗ cả hai loại lọt qua.
   */
  it("mỗi lựa chọn là một `<button>`, không một ô nhập nào", () => {
    for (const [thu, cau_hoi, danh_sach] of [
      [1, CAU_HOI_NGANH, NGANH],
      [2, CAU_HOI_QUY_MO, QUY_MO],
      [3, CAU_HOI_VIEC, VIEC],
    ] as const) {
      const markup = ve(
        <BuocChon thu={thu} cau_hoi={cau_hoi} lua_chon={danh_sach} onChon={() => {}} />,
      );
      expect(markup, `bước ${thu} dựng ra một ô nhập`).not.toMatch(/<(form|input|textarea|select)\b/);
      const nut = markup.match(/<button\b/g) ?? [];
      expect(nut.length, `bước ${thu} có ${nut.length} nút cho ${danh_sach.length} lựa chọn`).toBe(
        danh_sach.length,
      );
      const chu = chuCua(markup);
      expect(chu, `bước ${thu} không hiện câu hỏi`).toContain(cau_hoi);
      for (const mot of danh_sach) {
        expect(chu, `bước ${thu} thiếu lựa chọn: ${mot.nhan}`).toContain(mot.nhan);
      }
      // Số bước nói bằng CHỮ, không bằng riêng màu của một chuỗi chấm tròn.
      expect(chu, `bước ${thu} không nói mình là bước thứ mấy`).toContain(LOI.buoc(thu, 3));
    }
  });

  it("mã nội bộ của một lựa chọn không bao giờ hiện ra màn hình", () => {
    // `ban-le`, `50-200`, `quan-he-khach` là khoá của mã, không phải chữ của người đọc. Một khoá
    // lọt lên màn là một màn hình nói bằng ngôn ngữ của lập trình viên.
    const chu = chuCua(ve(<BuocChon thu={1} cau_hoi={CAU_HOI_NGANH} lua_chon={NGANH} onChon={() => {}} />));
    for (const mot of NGANH) {
      expect(chu, `mã nội bộ "${mot.ma}" hiện ra màn hình`).not.toContain(mot.ma);
    }
  });

  it("màn mở ra ở bước một, và chưa có đường lui nào", () => {
    const chu = chuCua(ve(<GoiYGiaiPhapScreen />));
    expect(chu).toContain(LOI.tieu_de);
    expect(chu).toContain(CAU_HOI_NGANH);
    // Hai nút lui chỉ xuất hiện khi CÓ thứ để lui về. Ở bước một chúng là hai nút không làm gì,
    // và một nút bấm không phản hồi là cách nhanh nhất để người lớn tuổi kết luận app hỏng.
    expect(chu, "bước một đã bày nút quay lại").not.toContain(LOI.nut_quay_lai);
    expect(chu, "bước một đã bày nút làm lại").not.toContain(LOI.nut_lam_lai);
  });

  it("nhanCua đọc chữ từ chính danh sách vẽ ra nó, và trả null khi chưa chọn", () => {
    expect(nhanCua(QUY_MO, undefined)).toBeNull();
    expect(nhanCua(QUY_MO, "50-200")).toBe(QUY_MO.find((mot) => mot.ma === "50-200")!.nhan);
  });
});

/* ============================================================================================
   KẾT QUẢ — HAI HÀNH ĐỘNG, KHÔNG MỘT LỜI HỨA NÀO VƯỢT GIAI ĐOẠN A
   ============================================================================================ */

describe("màn kết quả kết thúc bằng đúng hai hành động", () => {
  const viec: MaViec = "da-kenh";
  const markup = ve(
    <KetQuaGoiY
      tra_loi={{ nganh: "ban-le", quy_mo: "10-50", viec }}
      viec={viec}
      onXemGiaiPhap={() => {}}
    />,
  );

  it("đọc lại cả ba câu trả lời, kể cả hai câu không lọc gì", () => {
    // Hai bước đầu KHÔNG đổi danh sách gợi ý, và màn hình nói ra điều đó. Chúng vẫn phải xuất
    // hiện ở câu tóm tắt — nếu không thì chúng là hai bước không làm gì, tức hai bước nói dối.
    const chu = chuCua(markup);
    expect(chu).toContain(nhanCua(NGANH, "ban-le")!);
    expect(chu).toContain(nhanCua(QUY_MO, "10-50")!);
    expect(chu).toContain(nhanCua(VIEC, viec)!);
    expect(chu, "màn không nói ra rằng gợi ý chọn theo việc đang cần").toContain(LOI.co_so_goi_y);
  });

  it("có đúng một neo `tel:`, tới đúng hotline đã công bố", () => {
    const neo = markup.match(/href="tel:([^"]+)"/g) ?? [];
    expect(neo).toHaveLength(1);
    expect(markup).toContain('href="tel:');
  });

  it("KHÔNG hứa báo giá, đặt lịch hay đăng ký — giai đoạn A chưa có tuyến nào nhận", () => {
    const chu = chuCua(markup).toLowerCase();
    for (const tu of ["báo giá", "đặt lịch", "đăng ký", "để lại thông tin", "gửi yêu cầu"]) {
      expect(chu, `màn kết quả hứa "${tu}" trong khi chưa có tuyến nào nhận`).not.toContain(tu);
    }
  });

  it("không một ô nhập nào, kể cả ở màn kết quả", () => {
    expect(markup).not.toMatch(/<(form|input|textarea|select)\b/);
  });
});

/* ============================================================================================
   CHỖ CỦA MÀN TRONG SỔ MÀN HÌNH
   ============================================================================================ */

describe("màn đứng ngoài thanh tab, và nói rõ ô nào sáng", () => {
  it("khai `ngoai-tab` và trỏ về màn chủ", () => {
    expect(MAN_GOI_Y_GIAI_PHAP.cho.kieu).toBe("ngoai-tab");
    expect(MAN_GOI_Y_GIAI_PHAP.cho.tabSangLen).toBe("home");
  });

  it("thanh tiêu đề và `<h1>` của màn là MỘT chuỗi, không hai", () => {
    // Hai chỗ giữ một cái tên là hai chỗ sẽ lệch, và lần lệch ấy hiện ra thành thanh tiêu đề nói
    // một đằng, tiêu đề màn một nẻo — với người lớn tuổi thì đó là "tôi đang ở đâu".
    expect(MAN_GOI_Y_GIAI_PHAP.headerTitle).toBe(LOI.tieu_de);
    expect(chuCua(ve(<GoiYGiaiPhapScreen />))).toContain(MAN_GOI_Y_GIAI_PHAP.headerTitle);
  });
});
