import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { nhanNhomRong, nhanSoMuc } from "@/features/cau-hinh/nhan-danh-muc";
import type { NhomDanhMuc } from "@/features/cau-hinh/nhom-danh-muc";
import { NhomMuc } from "@/features/cau-hinh/tab-danh-muc";

/**
 * VÌ SAO TỆP NÀY TỒN TẠI, và nó không phải chuyện phủ thêm cho đủ.
 *
 * Mọi ca test khác của màn Danh mục canh một QUYẾT ĐỊNH trong module thuần: nhóm nào rỗng, câu
 * nào hiện ra, `null` khác `0`, thứ tự máy chủ trả về không bị sắp lại. Không ca nào canh việc
 * quyết định ấy CÓ RA TỚI TRANG hay không.
 *
 * Đã đo, không phải lo xa: bôi trắng `{nhanNhomRong(nhom.nhan)}` trong `tab-danh-muc.tsx` — câu
 * quan trọng nhất trên màn hình, thứ nói với một đơn vị rằng danh mục của họ RỖNG chứ không phải
 * HỎNG — làm cả 167 ca vẫn xanh và `tsc` vẫn sạch. Một màn hình trông như hỏng trong khi đang
 * chạy đúng là một cuộc gọi hỗ trợ từ trụ sở xã.
 *
 * KHÔNG THÊM PHỤ THUỘC NÀO. `react-dom` đã nằm trong `dependencies`; `renderToStaticMarkup` chạy
 * trong Node thuần và trả về một chuỗi. Lập luận trong `vitest.config.mts` từ chối jsdom +
 * testing-library vì "bốn phụ thuộc để kiểm lại cùng một thứ qua một lớp mô phỏng trình duyệt" —
 * lập luận ấy vẫn đứng: ở đây không có trình duyệt nào được mô phỏng, và những thứ THẬT SỰ cần
 * trình duyệt (sự kiện, tiêu điểm, bố cục) vẫn nằm ngoài phạm vi.
 */

function nhomRong(): NhomDanhMuc {
  return {
    khoa: "loaiVanBan",
    nhan: "Loại văn bản",
    thuTuLaThangBac: false,
    trangThai: { pha: "chuaCoMuc" },
  };
}

describe("NhomMuc kết xuất ra trang", () => {
  it("nhóm rỗng: câu báo rỗng PHẢI có mặt trong markup, nguyên văn", () => {
    const html = renderToStaticMarkup(<NhomMuc nhom={nhomRong()} />);

    // Nguyên văn, không phải một mảnh: một câu bị cắt còn tệ hơn câu vắng mặt, vì nó vẫn đọc
    // trôi chảy mà thiếu đúng nửa nói "không thêm được mục mới".
    expect(html).toContain(nhanNhomRong("Loại văn bản"));
  });

  it("nhóm rỗng KHÔNG được dựng thành ô trống", () => {
    const html = renderToStaticMarkup(<NhomMuc nhom={nhomRong()} />);

    // Thứ đang canh là "có chữ cho người đọc", không phải "có thẻ p". Một <p></p> rỗng qua được
    // phép kiểm thẻ và trượt đúng thứ ca test này sinh ra để bắt.
    expect(html).toContain('class="trang-thai-rong"');
    expect(html).not.toContain('class="trang-thai-rong"></p>');
  });

  it("nhóm rỗng KHÔNG được dựng như một lỗi", () => {
    const html = renderToStaticMarkup(<NhomMuc nhom={nhomRong()} />);

    // `role="alert"` cắt ngang người dùng trình đọc màn hình. Một danh mục chưa có mục nào là
    // trạng thái BÌNH THƯỜNG của mọi đơn vị hôm nay — không có gì để báo động.
    expect(html).not.toContain('role="alert"');
    expect(html).not.toContain("thong-bao-loi");
  });

  it("nhóm có mục: hiện số đếm và mã, và KHÔNG hiện câu báo rỗng", () => {
    const html = renderToStaticMarkup(
      <NhomMuc
        nhom={{
          khoa: "loaiVanBan",
          nhan: "Loại văn bản",
          thuTuLaThangBac: false,
          trangThai: {
            pha: "coMuc",
            muc: [
              { id: "lvb-1", code: "quyet-dinh", label: "Quyết định", is_default: true, active: true },
              { id: "lvb-2", code: "to-trinh", label: "Tờ trình", is_default: false, active: false },
            ],
          },
        }}
      />,
    );

    expect(html).toContain(nhanSoMuc(2));
    expect(html).toContain("quyet-dinh");
    expect(html).toContain("Tờ trình");
    expect(html).not.toContain(nhanNhomRong("Loại văn bản"));
  });

  it("nhóm đọc không được: hiện ĐÚNG thông báo của máy chủ, và không hiện câu báo rỗng", () => {
    const thongBao = "Đã xảy ra lỗi. Vui lòng thử lại.";
    const html = renderToStaticMarkup(
      <NhomMuc
        nhom={{
          khoa: "loaiVanBan",
          nhan: "Loại văn bản",
          thuTuLaThangBac: false,
          trangThai: { pha: "khongDocDuoc", thongBao },
        }}
      />,
    );

    expect(html).toContain(thongBao);
    // Hai trạng thái này phải phân biệt được trên màn hình: "chưa có gì" và "không đọc được" dẫn
    // người dùng đi hai đường khác hẳn nhau — một bên chờ onboard, một bên gọi hỗ trợ.
    expect(html).not.toContain(nhanNhomRong("Loại văn bản"));
    expect(html).toContain('role="alert"');
  });
});
