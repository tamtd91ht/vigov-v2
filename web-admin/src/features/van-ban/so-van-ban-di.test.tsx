import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { TRANG_DAU } from "@/features/cau-hinh/ngan-xep-con-tro";
import type { BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_vanBanDiRa,
  page_Result_documents_vanBanDiRa,
} from "@/lib/api/schema.gen";

import { CANH_BAO_GO_KHONG_TRA_SO } from "./nhan-van-ban";
import {
  BAN_DI_TRONG,
  BangVanBanDi,
  BieuMauVanBanDi,
  ManSoVanBanDi,
  type ThaoTacDi,
} from "./so-van-ban-di";

/**
 * ⚠ QUYỂN SỔ NÀY KHÔNG CÓ ĐẶC TẢ NÀO, nên tệp này canh một điều mà màn hình có đặc tả không cần:
 * MÀN HÌNH KHÔNG ĐƯỢC BỊA THÊM GÌ. Cụ thể là không có trạng thái, không có vòng đời "nháp → đã ký
 * → đã phát hành", không có hạn xử lý — ba thứ `vanBanDiRa` cố ý không có
 * (`service-documents/internal/http/van_ban_di.go:40`), và ba thứ mà một người dựng màn hình rất
 * dễ thêm vào vì "sổ đến có thì sổ đi cũng nên có".
 *
 * Vế phủ định là vế chịu lực ở đây: thêm một cột `Trạng thái` không làm hỏng thứ gì đang chạy, nó
 * chỉ bắt mọi xã đi theo một quy trình không ai quyết định.
 */

const KHONG_LAM_GI: ThaoTacDi = { them: () => {}, sua: () => {}, go: () => {} };

const TRA_LOAI: BangTraDanhMuc = { pha: "xong", ten: new Map([["cong-van", "Công văn"]]) };

function dong(sua: Partial<documents_vanBanDiRa> = {}): documents_vanBanDiRa {
  return {
    id: "01JVBDI00000000000000001",
    number: 12,
    year: 2026,
    document_date: "2026-09-22",
    document_type: "cong-van",
    summary: "Trả lời đơn kiến nghị về tuyến mương thoát nước",
    recipient: "UBND huyện Thăng Bình",
    signer: "Chủ tịch UBND xã",
    created_by: "CB-00123",
    created_at: "2026-09-22T02:00:00Z",
    updated_at: "2026-09-22T02:00:00Z",
    ...sua,
  };
}

function trang(items: documents_vanBanDiRa[]): KetQua<page_Result_documents_vanBanDiRa> {
  return { ok: true, duLieu: { items, next_cursor: "", has_more: false } };
}

function veBang(kq: KetQua<page_Result_documents_vanBanDiRa> | null, coQuyenGhi = true) {
  return renderToStaticMarkup(
    <BangVanBanDi kq={kq} traLoai={TRA_LOAI} coQuyenGhi={coQuyenGhi} thaoTac={KHONG_LAM_GI} />,
  );
}

describe("bảng sổ văn bản đi — đúng sáu cột của hợp đồng, không một cột bịa thêm", () => {
  it("hiện số đi, ngày, loại, trích yếu, nơi nhận, người ký", () => {
    const html = veBang(trang([dong()]));

    expect(html).toContain("12/2026");
    expect(html).toContain("22/09/2026");
    expect(html).toContain("Công văn");
    expect(html).toContain("Trả lời đơn kiến nghị về tuyến mương thoát nước");
    expect(html).toContain("UBND huyện Thăng Bình");
    expect(html).toContain("Chủ tịch UBND xã");
  });

  it("KHÔNG có cột Trạng thái và KHÔNG có cột Hạn — hợp đồng không có hai trường ấy", () => {
    // VẾ CHỊU LỰC. `vanBanDiRa` không có `status` và không có `due_at`: một văn bản đi không có
    // vòng đời và không mang cam kết nào — cấp số CHÍNH LÀ hành vi phát hành. Vẽ một cột trạng
    // thái ở đây là bịa ra một quy trình mà mọi xã sau đó buộc phải đi theo.
    const html = veBang(trang([dong()]));

    expect(html).not.toMatch(/<th[^>]*>Trạng thái</);
    expect(html).not.toMatch(/<th[^>]*>Hạn/);
    expect(html).not.toMatch(/Nháp|Đã ký|Đã phát hành|Chờ ký/);
    expect(html).not.toMatch(/overdue|qua_han/i);
  });

  it("KHÔNG có nút chuyển xử lý — văn bản đi rời khỏi xã, nó không đi giữa các bộ phận", () => {
    expect(veBang(trang([dong()]))).not.toContain("Chuyển");
  });

  it("người ký để trống là một câu trả lời, không phải một ô trống", () => {
    expect(veBang(trang([dong({ signer: "" })]))).toContain("<td>Không ghi</td>");
  });

  it("thiếu quyền ghi: bảng VẪN hiện đủ dòng, chỉ mất cụm nút", () => {
    const html = veBang(trang([dong()]), false);

    expect(html).toContain("UBND huyện Thăng Bình");
    expect(html).not.toContain("Gỡ khỏi sổ");
    expect(html).not.toContain("nut-xoa");
  });

  it("nơi nhận KHÔNG đi vào một thuộc tính nào — nó có thể là tên một công dân", () => {
    // `van_ban_di.go:36`: "Ông Nguyễn Văn A, thôn Bình Trị" là điều một xã viết trên công văn trả
    // lời. Nó hiện trong ô cho cán bộ của chính xã ấy; nhãn trợ năng dùng SỐ ĐI (luật 3, cấm #4).
    const html = veBang(trang([dong({ recipient: "Ông Nguyễn Văn A, thôn Bình Trị" })]));

    expect(html).toContain("<td>Ông Nguyễn Văn A, thôn Bình Trị</td>");
    expect(html).not.toMatch(/aria-label="[^"]*Nguyễn Văn A/);
    expect(html).toContain('aria-label="Gỡ khỏi sổ văn bản đi số 12/2026"');
  });
});

function veForm(dangMo: Parameters<typeof BieuMauVanBanDi>[0]["dangMo"], loi = "") {
  return renderToStaticMarkup(
    <BieuMauVanBanDi
      dangMo={dangMo}
      ban={BAN_DI_TRONG}
      datBan={() => {}}
      traLoai={TRA_LOAI}
      loi={loi}
      dangGui={false}
      onGui={() => {}}
      onHuy={() => {}}
    />,
  );
}

describe("biểu mẫu sổ văn bản đi", () => {
  it("cấp số: năm ô của hợp đồng, KHÔNG có ô Số đi", () => {
    // Máy chủ trả 400 nếu thân nhắc tới `number`, và ở quyển sổ này lý do nặng nhất: con số ấy
    // được in lên giấy, đóng dấu, gửi ra khỏi trụ sở.
    const html = veForm({ kieu: "them", khoaChongTrung: "khoa-cua-bai-kiem" });

    expect(html).toContain("Ngày văn bản");
    expect(html).toContain("Loại văn bản");
    expect(html).toContain("Trích yếu");
    expect(html).toContain("Nơi nhận");
    expect(html).toContain("Người ký");
    expect(html).not.toMatch(/<label[^>]*>Số đi</);
    expect(html).not.toMatch(/name="so(Di)?"/);
  });

  it("cấp số: nói trước rằng số cấp ngay khi lưu và không bao giờ cấp lại", () => {
    const html = veForm({ kieu: "them", khoaChongTrung: "k" });
    expect(html).toMatch(/cấp số đi ngay khi lưu/);
    expect(html).toMatch(/không bao giờ được cấp lại/);
  });

  it("gỡ: NGUYÊN câu 'gỡ không trả số về dãy', kèm ô lý do bắt buộc", () => {
    const html = veForm({ kieu: "go", vb: dong() });

    expect(html).toContain(CANH_BAO_GO_KHONG_TRA_SO);
    expect(html).toContain("Lý do gỡ");
    expect(html).toContain("12/2026");
  });

  it("câu từ chối của máy chủ ra nguyên văn, không kèm số hiệu HTTP", () => {
    const cauCuaMayChu =
      "Loại văn bản này không còn trong danh mục đang dùng của xã. Hãy chọn lại loại văn bản.";
    const html = veForm({ kieu: "them", khoaChongTrung: "k" }, cauCuaMayChu);

    expect(html).toContain(cauCuaMayChu);
    expect(html).not.toContain("409");
  });
});

describe("màn sổ văn bản đi", () => {
  function veMan(coQuyenGhi: boolean, thieuQuyenGhi: boolean) {
    return renderToStaticMarkup(
      <ManSoVanBanDi
        kq={trang([dong()])}
        nam={2026}
        namGoc={2026}
        datNam={() => {}}
        loaiLoc=""
        datLoaiLoc={() => {}}
        traLoai={TRA_LOAI}
        coQuyenGhi={coQuyenGhi}
        thieuQuyenGhi={thieuQuyenGhi}
        thaoTac={KHONG_LAM_GI}
        cauDaXong=""
        loiNgoaiForm=""
        nganXep={TRANG_DAU}
        diToiTrang={() => {}}
        form={null}
      />,
    );
  }

  it("đủ quyền: có nút cấp số, có bộ lọc năm và loại", () => {
    const html = veMan(true, false);

    expect(html).toContain("Cấp số văn bản đi");
    expect(html).toContain("Năm của sổ");
    expect(html).toContain("Loại văn bản");
  });

  it("bộ lọc KHÔNG có ô trạng thái và KHÔNG có ô bộ phận — tuyến đọc không nhận hai tham số ấy", () => {
    const html = veMan(true, false);

    expect(html).not.toContain("Tất cả trạng thái");
    expect(html).not.toContain("Bộ phận đang giữ");
  });

  it("thiếu quyền ghi: nói rõ thiếu quyền gì, và KHÔNG mất bảng", () => {
    const html = veMan(false, true);

    expect(html).toMatch(/không có quyền cấp số/);
    expect(html).toContain("UBND huyện Thăng Bình");
    expect(html).not.toContain("Cấp số văn bản đi");
  });
});
