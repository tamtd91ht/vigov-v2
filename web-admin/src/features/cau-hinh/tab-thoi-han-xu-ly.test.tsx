import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhSachCaLamBuRa,
  identity_danhSachCaLamViecRa,
  identity_danhSachNgayNghiLeRa,
  identity_danhSachSLARa,
  identity_dongSLARa,
  identity_phienHienTaiRa,
} from "@/lib/api/schema.gen";

import { quyetDinhGhiThoiHan } from "./quyen-tab";
import { ManThoiHanXuLy, type DuLieuTab, type ThaoTacThoiHan } from "./tab-thoi-han-xu-ly";

/**
 * NĂM ĐIỀU TỆP NÀY CANH, và cả năm đều là những thứ một lần sửa MỘT DÒNG phá được mà không phép
 * kiểm nào khác thấy:
 *
 * 1. KHỐI CẢNH BÁO CÓ MẶT KHI XÃ CHƯA KHAI XONG, VÀ NÓI RA HẬU QUẢ. Đây là phần đáng giá nhất của
 *    màn hình: một xã mới không vào sổ được văn bản đến và không nhận được phản ánh nào, mà lỗi ấy
 *    hiện ra ở một màn hình khác hẳn màn hình sửa được nó.
 *
 * 2. VÀ VẮNG MẶT KHI ĐÃ KHAI XONG — vế phủ định là vế chịu lực. Một lời báo động cũng hiện ở xã đã
 *    cấu hình đủ là một lời báo động người ta học cách bỏ qua, rồi bỏ qua nốt lần nó đúng.
 *
 * 3. KHÔNG CÓ NÚT `+ Thêm thời hạn cho một lĩnh vực` DƯỚI BẤT KỲ CÁCH VIẾT NÀO. Đặc tả vẽ nút ấy
 *    (`14-cau-hinh.md:293`) nên áp lực thêm lại nó là có thật và đến từ một tài liệu trông có thẩm
 *    quyền — nhưng tuyến sau nó cố ý chưa có (ADR 0026 điều kiện dừng #2: mã lĩnh vực từ client
 *    phải đối chiếu bộ mã tầng 1 ở `platform`, và đường đọc ấy chưa có ADR). Một nút gọi vào tuyến
 *    không tồn tại là lời hứa suông.
 *
 * 4. CÂU "THAY ĐỔI CHỈ ÁP DỤNG CHO HỒ SƠ TIẾP NHẬN SAU THỜI ĐIỂM LƯU" CÓ TRÊN TRANG. Không nói ra
 *    thì cán bộ tưởng vừa bấm Lưu là mọi hồ sơ đang chạy đổi hạn theo — và báo cáo lên trên theo
 *    cái tưởng ấy.
 *
 * 5. CÂU "TẾT / GIỖ TỔ / NGÀY LIỀN KỀ 02/9 CÒN THIẾU" ĐỨNG CẠNH NÚT GIEO NGÀY LỄ. Tuyến chỉ gieo
 *    BỐN ngày cố định theo dương lịch và `seeded: 4` đọc ra là "xong"; thiếu ba nhóm ngày kia thì
 *    mọi hạn rơi vào dịp Tết bị tính sai mà không gì báo lỗi.
 */

const KHONG_LAM_GI: ThaoTacThoiHan = {
  gieoThoiHan: () => {},
  gieoTuan: () => {},
  gieoNgayLe: () => {},
  suaThoiHan: () => {},
  themCa: () => {},
  suaCa: () => {},
  xoaCa: () => {},
  themNghi: () => {},
  suaNghi: () => {},
  xoaNghi: () => {},
  themLamBu: () => {},
  suaLamBu: () => {},
  xoaLamBu: () => {},
};

const DONG_SLA: identity_dongSLARa = {
  id: "01J0000000000000000000SLA",
  work_kind: "phan-anh",
  field: "an-ninh-trat-tu",
  is_default: false,
  acknowledge_hours: 2,
  resolve_hours: 16,
  due_soon_hours: 4,
  escalate_leader_hours: 8,
  escalate_president_hours: 16,
};

function ok<T>(duLieu: T): KetQua<T> {
  return { ok: true, duLieu };
}

function phienVoi(quyen: string[]): KetQua<identity_phienHienTaiRa> {
  return ok<identity_phienHienTaiRa>({
    sid: "01J000000000000000000SID",
    expires_at: "2026-09-26T12:00:00Z",
    staff: { code: "CB001", full_name: "Cán bộ thử", position: "Chuyên viên" },
    role: null,
    permissions: quyen,
    must_change_password: false,
  });
}

const SLA_CO_DONG = ok<identity_danhSachSLARa>({ items: [DONG_SLA], problems: [] });
const SLA_RONG = ok<identity_danhSachSLARa>({ items: [], problems: [] });

const TUAN_CO_CA = ok<identity_danhSachCaLamViecRa>({
  items: [
    { id: "01J00000000000000000000CA", weekday: 1, start: "07:30:00", end: "11:30:00", note: "Buổi sáng" },
  ],
  problems: [],
});
const TUAN_RONG = ok<identity_danhSachCaLamViecRa>({ items: [], problems: [] });

const NGHI_RONG = ok<identity_danhSachNgayNghiLeRa>({ items: [] });
const LAM_BU_RONG = ok<identity_danhSachCaLamBuRa>({ items: [], problems: [] });

function ve(du: Partial<DuLieuTab> = {}, them: { coQuyenGhi?: boolean; cauDaXong?: string } = {}) {
  return renderToStaticMarkup(
    <ManThoiHanXuLy
      du={{
        thoiHan: SLA_CO_DONG,
        tuan: TUAN_CO_CA,
        nghi: NGHI_RONG,
        lamBu: LAM_BU_RONG,
        ...du,
      }}
      nam={2026}
      namGoc={2026}
      datNam={() => {}}
      coQuyenGhi={them.coQuyenGhi ?? true}
      thieuQuyen={false}
      thaoTac={KHONG_LAM_GI}
      cauDaXong={them.cauDaXong ?? ""}
      form={null}
      nhomForm={null}
      loiMayChuNgoaiForm=""
      dangGui={false}
    />,
  );
}

describe("khối 'đơn vị chưa khai xong'", () => {
  it("bảng thời hạn RỖNG → khối có mặt và nói ra hậu quả", () => {
    const html = ve({ thoiHan: SLA_RONG });

    expect(html).toContain("Đơn vị chưa khai xong phần bắt buộc");
    // HẬU QUẢ, không phải tình trạng. "Chưa có dữ liệu" đọc ra là một việc để hôm khác.
    expect(html).toContain("chưa vào sổ được văn bản đến");
    expect(html).toContain("chưa nhận được phản ánh của người dân");
    expect(html).toContain("Bảng Thời hạn xử lý đang trống");
  });

  it("giờ làm việc RỖNG → khối vẫn có mặt, dù bảng thời hạn đã đầy", () => {
    // Điều kiện `&&` thay cho `||` ở đây sẽ tạo ra trạng thái im lặng nguy hiểm nhất: xã đã gieo
    // thời hạn, tưởng xong, mà `ResolveDeadlines` vẫn từ chối vì không đếm được giờ làm việc nào.
    const html = ve({ tuan: TUAN_RONG });

    expect(html).toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).toContain("Giờ làm việc trong tuần đang trống");
    expect(html).toContain("chưa nhận được phản ánh của người dân");
  });

  it("khối mang ĐÚNG nút gieo của bảng đang thiếu, không mang nút của bảng đã đầy", () => {
    const html = ve({ thoiHan: SLA_RONG, tuan: TUAN_CO_CA });

    // Câu `problems` của máy chủ gọi đích danh nhãn nút này; đặt tên khác đi là để máy chủ chỉ vào
    // một cái nút không tồn tại trên màn hình.
    expect(html).toContain("Gieo thời hạn mặc định");
    // Lịch tuần đã có ca, nên không có nút gieo tuần TRONG KHỐI — nhưng vẫn có nút vá lại ở bảng
    // giờ làm việc phía dưới, nên đếm bằng số lần xuất hiện chứ không bằng `not.toContain`.
    expect(html.split("Gieo giờ làm việc mặc định").length - 1).toBe(1);
  });

  it("CẢ HAI bảng đã có dòng → khối VẮNG MẶT", () => {
    const html = ve();

    expect(html).not.toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).not.toContain("chưa vào sổ được văn bản đến");
  });

  it("chưa đọc xong → khối VẮNG MẶT, và trang nói đang tải", () => {
    const html = ve({ thoiHan: null, tuan: null });

    expect(html).not.toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).toContain("Đang tải bảng thời hạn xử lý…");
  });

  it("đọc hỏng → KHÔNG khẳng định xã chưa khai; hiện NGUYÊN câu máy chủ", () => {
    const html = ve({
      thoiHan: { ok: false, thongBao: "Bạn không có quyền thực hiện thao tác này." },
    });

    expect(html).not.toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).toContain("Bạn không có quyền thực hiện thao tác này.");
  });
});

describe("bảng thời hạn xử lý", () => {
  it("KHÔNG có nút thêm thời hạn cho một lĩnh vực, dưới bất kỳ cách viết nào", () => {
    const html = ve();

    expect(html).not.toContain("Thêm thời hạn");
    expect(html).not.toContain("thêm thời hạn");
    expect(html).not.toContain("lĩnh vực mới");
    expect(html).not.toContain("Thêm lĩnh vực");
  });

  it("câu 'chỉ áp dụng cho hồ sơ tiếp nhận sau' có trên trang", () => {
    expect(ve()).toContain("Thay đổi chỉ áp dụng cho hồ sơ tiếp nhận sau thời điểm lưu.");
  });

  it("mọi con số kèm ĐƠN VỊ 'giờ làm việc', không phải giờ đồng hồ", () => {
    // 16 giờ làm việc là hai ngày làm. Đọc nhầm thành giờ đồng hồ là siết một cam kết với người
    // dân xuống còn một phần ba, mà không có gì báo lỗi.
    const html = ve();

    expect(html).toContain("2 giờ làm việc");
    expect(html).toContain("16 giờ làm việc");
  });

  it("`problems` của máy chủ hiện ra NGUYÊN VĂN, không nuốt", () => {
    const cau =
      "Loại việc “phan-anh” có dòng riêng nhưng thiếu dòng mặc định, nên lĩnh vực nào không có " +
      "dòng riêng sẽ không tính được hạn.";
    const html = ve({
      thoiHan: ok<identity_danhSachSLARa>({
        items: [DONG_SLA],
        problems: [{ kind: "missing_default_row", work_kind: "phan-anh", message: cau }],
      }),
    });

    expect(html).toContain(cau);
  });

  it("mã lĩnh vực hiện NGUYÊN MÃ — không có bảng tra tên lĩnh vực nào ở web", () => {
    // Danh mục `Lĩnh vực phản ánh` chưa có chủ (câu mở #4, ADR 0024) và chưa có tuyến nào phát ra
    // nhãn. Một bảng tra gõ tay ở web là bản sao thứ hai của một danh mục chưa ai sở hữu.
    expect(ve()).toContain("an-ninh-trat-tu");
  });

  it("dòng mặc định nói rõ nó là mặc định, không hiện một ô trống", () => {
    const html = ve({
      thoiHan: ok<identity_danhSachSLARa>({
        items: [{ ...DONG_SLA, field: "", is_default: true }],
        problems: [],
      }),
    });

    expect(html).toContain("Mặc định cho mọi lĩnh vực");
  });
});

describe("gieo ngày nghỉ lễ nợ người bấm một câu", () => {
  it("câu Tết / Giỗ Tổ / ngày liền kề còn thiếu đứng cạnh kết quả gieo", () => {
    const html = ve({}, { cauDaXong: "Năm 2026: đã thêm 4 ngày nghỉ lễ theo dương lịch." });

    expect(html).toContain("Năm 2026: đã thêm 4 ngày nghỉ lễ theo dương lịch.");
    expect(html).toContain("Tết Nguyên đán");
    expect(html).toContain("Giỗ Tổ Hùng Vương");
    expect(html).toContain("ngày liền kề 02/9");
    expect(html).toContain("mọi thời hạn rơi vào dịp Tết đều bị tính sai");
  });

  it("câu ấy đứng sẵn cả khi chưa bấm gieo — nó là sự thật thường trực, không phải một lời đáp", () => {
    expect(ve()).toContain("Tết Nguyên đán, Giỗ Tổ Hùng Vương và ngày liền kề 02/9 CHƯA có");
  });
});

describe("cổng quyền bọc phần GHI, không bọc bảng", () => {
  it("thiếu quyền: bảng vẫn hiện đủ dòng, chỉ nút ghi vắng", () => {
    // Ẩn cả bảng là giao diện từ chối điều máy chủ đang phục vụ. Ba tuyến đọc lịch là
    // `any-authenticated`, và `GET /api/v1/sla` thì máy chủ tự trả 403 — không cần cổng thứ hai.
    const html = ve({}, { coQuyenGhi: false });

    expect(html).toContain("16 giờ làm việc");
    expect(html).toContain("Buổi sáng");
    expect(html).not.toContain(">Sửa<");
    expect(html).not.toContain(">Xoá<");
    expect(html).not.toContain("Thêm ca làm việc");
    expect(html).not.toContain("Gieo thời hạn mặc định");
  });

  it("CHỈ có `admin.sla` (không `admin.lookup`, không khoá admin nào khác): dùng được tab", () => {
    // Đi qua ĐÚNG phép quyết định mà `TabThoiHanXuLy` gọi, không qua một cờ gõ tay trong test.
    const phien = phienVoi(["admin.sla"]);
    const quyet = quyetDinhGhiThoiHan(phien);
    expect(quyet).toEqual({ hien: true });

    const html = ve({}, { coQuyenGhi: quyet.hien });
    expect(html).toContain(">Sửa<");
    expect(html).toContain("Thêm ca làm việc");
  });

  it("không có `admin.sla` (dù có mọi khoá admin khác): KHÔNG một nút ghi nào", () => {
    const phien = phienVoi(["admin.lookup", "admin.org", "admin.user", "admin.role", "admin.audit"]);
    const quyet = quyetDinhGhiThoiHan(phien);
    expect(quyet).toEqual({ hien: false, vi: "khong-du-quyen" });

    const html = ve({}, { coQuyenGhi: quyet.hien });
    expect(html).not.toContain(">Sửa<");
    expect(html).not.toContain("Thêm ca làm việc");
  });

  it("có quyền: nút ghi có mặt ở cả bốn bảng", () => {
    const html = ve();

    expect(html).toContain(">Sửa<");
    expect(html).toContain(">Xoá<");
    expect(html).toContain("Thêm ca làm việc");
    expect(html).toContain("Thêm ngày nghỉ lễ");
    expect(html).toContain("Thêm ca làm bù");
  });
});

describe("trạng thái rỗng của hai bảng theo năm", () => {
  it("năm chưa khai ngày nghỉ nào thì nói ra hệ quả, không để trang trống", () => {
    const html = ve({ nghi: NGHI_RONG });

    expect(html).toContain("Năm 2026 chưa khai ngày nghỉ lễ nào");
  });

  it("năm không có ngày làm bù là BÌNH THƯỜNG, và câu chữ phải phân biệt với lịch tuần trống", () => {
    const html = ve({ lamBu: LAM_BU_RONG });

    expect(html).toContain("Phần lớn các năm là như vậy");
  });
});
