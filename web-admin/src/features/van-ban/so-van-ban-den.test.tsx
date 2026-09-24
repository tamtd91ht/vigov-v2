import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import { TRANG_DAU } from "@/features/cau-hinh/ngan-xep-con-tro";
import type { BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_vanBanDenRa,
  page_Result_documents_vanBanDenRa,
} from "@/lib/api/schema.gen";

import { GOI_Y_TIM_DEN, type MaThuTu } from "./loc-so-van-ban";
import { CANH_BAO_GO_KHONG_TRA_SO, NHAN_DUONG_TOI_CAU_HINH } from "./nhan-van-ban";
import {
  BAN_DEN_TRONG,
  BangVanBanDen,
  BieuMauVanBanDen,
  ManSoVanBanDen,
  type ThaoTacDen,
} from "./so-van-ban-den";

/**
 * Canh những QUYẾT ĐỊNH CÓ RA TỚI TRANG hay không — bổ cho các ca kiểm module thuần, vốn chỉ canh
 * quyết định bên trong hàm. Đã đo trên màn Danh mục: bôi trắng câu quan trọng nhất của một màn hình
 * vẫn để lại 167 ca xanh và `tsc` sạch.
 *
 * BỐN ĐIỀU TỆP NÀY CANH, và ba trong bốn là VẾ PHỦ ĐỊNH — thứ chỉ lộ ra ở chỗ một nút KHÔNG được
 * có mặt:
 *
 *   1. Câu "gỡ không trả số về dãy" có mặt TRƯỚC khi cán bộ bấm gỡ.
 *   2. Câu 409 của máy chủ ra NGUYÊN VĂN, và trên cùng biểu mẫu có đường dẫn tới `/cau-hinh`.
 *   3. KHÔNG có nút sửa hay xoá nào trong khối chuyển xử lý — dòng lịch sử chỉ-thêm.
 *   4. Thiếu quyền GHI thì mất nút, KHÔNG mất bảng.
 */

const KHONG_LAM_GI: ThaoTacDen = {
  them: () => {},
  sua: () => {},
  go: () => {},
  chuyen: () => {},
  xem: () => {},
};

const TRA_LOAI: BangTraDanhMuc = { pha: "xong", ten: new Map([["cong-van", "Công văn"]]) };
const TRA_BO_PHAN: BangTraDanhMuc = {
  pha: "xong",
  ten: new Map([["01JBOPHAN", "VĂN PHÒNG ĐẢNG UỶ"]]),
};

const HAN = "2026-09-25T08:00:00Z";
const TRUOC_HAN = new Date("2026-09-24T00:00:00Z");
const SAU_HAN = new Date("2026-09-30T00:00:00Z");

function dong(sua: Partial<documents_vanBanDenRa> = {}): documents_vanBanDenRa {
  return {
    id: "01JVBDEN0000000000000001",
    number: 7,
    year: 2026,
    received_date: "2026-09-22",
    reference_no: "1742-CV/BTCTU",
    document_date: "2026-09-18",
    issuing_body: "Ban Tổ chức Tỉnh uỷ",
    document_type: "cong-van",
    summary: "Về việc rà soát hồ sơ cán bộ",
    urgency: "khan",
    holding_unit: "",
    assignee: "",
    due_at: HAN,
    status: "moi-vao-so",
    created_by: "CB-00123",
    created_at: "2026-09-22T02:00:00Z",
    updated_at: "2026-09-22T02:00:00Z",
    ...sua,
  };
}

function trang(items: documents_vanBanDenRa[]): KetQua<page_Result_documents_vanBanDenRa> {
  return { ok: true, duLieu: { items, next_cursor: "", has_more: false } };
}

function veBang(
  kq: KetQua<page_Result_documents_vanBanDenRa> | null,
  bayGio: Date,
  quyen: { ghi: boolean; chuyen: boolean } = { ghi: true, chuyen: true },
) {
  return renderToStaticMarkup(
    <BangVanBanDen
      kq={kq}
      bayGio={bayGio}
      traLoai={TRA_LOAI}
      traBoPhan={TRA_BO_PHAN}
      coQuyenGhi={quyen.ghi}
      coQuyenChuyen={quyen.chuyen}
      thaoTac={KHONG_LAM_GI}
    />,
  );
}

describe("bảng sổ văn bản đến — quá hạn SUY RA lúc vẽ", () => {
  it("cùng một dòng: trước hạn không có chữ Quá hạn, sau hạn thì có", () => {
    // CẢ HAI VẾ trên CÙNG một dữ liệu. Chỉ khác đồng hồ truyền vào — đó chính là hình dạng của một
    // giá trị được SUY RA. Một trường lưu sẵn sẽ cho ra hai lần kết xuất giống hệt nhau.
    expect(veBang(trang([dong()]), TRUOC_HAN)).not.toContain("Quá hạn");
    expect(veBang(trang([dong()]), SAU_HAN)).toContain("Quá hạn");
  });

  it("KHÔNG có chữ `overdue` dưới mọi cách viết trong HTML", () => {
    // Vế phủ định của luật 10 bất biến 3, đo trên chính chuỗi kết xuất: một lớp CSS, một thuộc
    // tính `data-overdue` hay một `aria-label` mang chữ ấy đều là dấu hiệu ai đó vừa dựng lại một
    // trường máy chủ cố ý không có.
    const html = veBang(trang([dong()]), SAU_HAN);
    expect(html).not.toMatch(/overdue|qua_han|is_?late/i);
  });

  it("hạn hiện kèm GIỜ, không chỉ ngày — cam kết đếm bằng giờ làm việc", () => {
    // `due_at` là một MỐC (RFC 3339), không phải một ngày: hạn đếm bằng giờ làm việc (ADR 0007),
    // nên cắt nó xuống còn ngày là nới rộng cam kết một cách lặng lẽ. Giờ hiện theo múi giờ Việt
    // Nam đã ghim, không theo cài đặt của máy cán bộ (`nhanThoiDiem`): 08:00Z là 15:00 giờ ta.
    expect(veBang(trang([dong()]), TRUOC_HAN)).toMatch(/Hạn xử lý 15:00 25\/09\/2026/);
  });
});

describe("cổng quyền bọc phần GHI, không bọc bảng", () => {
  it("thiếu cả hai quyền ghi: bảng VẪN hiện đủ dòng, chỉ mất cụm nút", () => {
    // Đây là quy tắc của cả màn: ẩn bảng là giao diện từ chối điều máy chủ đang phục vụ. Tuyến đọc
    // đòi `document.read` và máy chủ đã trả 200 — nghĩa là tài khoản này ĐƯỢC xem quyển sổ.
    const html = veBang(trang([dong()]), TRUOC_HAN, { ghi: false, chuyen: false });

    expect(html).toContain("Về việc rà soát hồ sơ cán bộ");
    expect(html).toContain("1742-CV/BTCTU");
    expect(html).not.toContain("Gỡ khỏi sổ");
    expect(html).not.toContain("Chuyển xử lý");
  });

  it("có `document.create` nhưng thiếu `document.route`: có Sửa và Gỡ, KHÔNG có Chuyển xử lý", () => {
    // VẾ CHỊU LỰC, và là ca một lần gộp hai khoá thành một sẽ phá: chuyển văn bản là hành vi quyết
    // định AI CHỊU TRÁCH NHIỆM, quyền riêng của nó là `document.route` (đặc tả §7 quy tắc 4). Một
    // xã giao việc nhập cho văn thư và giữ việc phân luồng cho lãnh đạo là cách làm bình thường.
    const html = veBang(trang([dong()]), TRUOC_HAN, { ghi: true, chuyen: false });

    expect(html).toContain("Gỡ khỏi sổ");
    expect(html).not.toContain("Chuyển xử lý");
  });

  it("có `document.route` nhưng thiếu `document.create`: chỉ có Chuyển xử lý", () => {
    const html = veBang(trang([dong()]), TRUOC_HAN, { ghi: false, chuyen: true });

    expect(html).toContain("Chuyển xử lý");
    expect(html).not.toContain("Gỡ khỏi sổ");
    expect(html).not.toContain("nut-xoa");
  });
});

describe("dữ liệu cá nhân không đi vào thuộc tính nào", () => {
  it("trích yếu và cơ quan ban hành hiện trong ô, KHÔNG nằm trong `aria-label` hay `title`", () => {
    // Luật 3, cấm #4: nhãn trợ năng, `title`, URL và tên tệp là những chỗ một giá trị bị mang đi —
    // vào cây trợ năng, vào nhật ký trình duyệt, vào ảnh chụp màn hình gửi cho hỗ trợ. Nhãn của
    // từng nút vì thế dùng SỐ ĐẾN, thứ vốn sinh ra để đọc qua điện thoại.
    const html = veBang(trang([dong()]), TRUOC_HAN);

    expect(html).toContain("<td>Ban Tổ chức Tỉnh uỷ</td>");
    expect(html).not.toContain('aria-label="Sửa văn bản đến số 7/2026"'.replace("7/2026", "Về việc"));
    expect(html).not.toMatch(/aria-label="[^"]*rà soát hồ sơ/);
    expect(html).not.toMatch(/title="[^"]*Ban Tổ chức/);
    expect(html).toContain('aria-label="Sửa văn bản đến số 7/2026"');
  });
});

describe("bảng ở ba trạng thái không phải 'có dữ liệu'", () => {
  it("chưa đọc xong: nói đang tải, KHÔNG nói sổ rỗng", () => {
    expect(veBang(null, TRUOC_HAN)).toContain("Đang tải");
  });

  it("máy chủ từ chối: câu của máy chủ ra NGUYÊN VĂN, không có số hiệu HTTP", () => {
    const cauCuaMayChu = "Tài khoản của bạn không có quyền xem văn bản.";
    const html = veBang({ ok: false, thongBao: cauCuaMayChu }, TRUOC_HAN);

    expect(html).toContain(cauCuaMayChu);
    expect(html).not.toContain("403");
  });

  it("sổ rỗng: câu nói việc phải làm tiếp, không phải một lời báo động", () => {
    const html = veBang(trang([]), TRUOC_HAN);

    expect(html).toContain("Chưa có văn bản đến nào");
    expect(html).not.toContain('role="alert"');
  });
});

/* ---- biểu mẫu ------------------------------------------------------------------------------ */

function veForm(
  dangMo: Parameters<typeof BieuMauVanBanDen>[0]["dangMo"],
  loi = "",
  ban = BAN_DEN_TRONG,
) {
  return renderToStaticMarkup(
    <BieuMauVanBanDen
      dangMo={dangMo}
      ban={ban}
      datBan={() => {}}
      traLoai={TRA_LOAI}
      loi={loi}
      dangGui={false}
      onGui={() => {}}
      onHuy={() => {}}
    />,
  );
}

describe("gỡ khỏi sổ — câu cảnh báo có mặt TRƯỚC khi bấm", () => {
  it("biểu mẫu gỡ mang NGUYÊN câu 'gỡ không trả số về dãy'", () => {
    // Câu này là toàn bộ lý do biểu mẫu gỡ tồn tại dưới dạng một bước riêng thay vì một nút bấm
    // thẳng: số đã cấp không bao giờ quay lại dãy (luật 7, bất biến 3), và cán bộ phải đọc điều đó
    // TRƯỚC, không phải sau.
    const html = veForm({ kieu: "go", vb: dong() });

    expect(html).toContain(CANH_BAO_GO_KHONG_TRA_SO);
    expect(html).toContain("Lý do gỡ");
    expect(html).toContain("Xác nhận gỡ khỏi sổ");
    // Số của văn bản sắp gỡ có mặt trên biểu mẫu: gỡ nhầm dòng là mất một số không lấy lại được.
    expect(html).toContain("7/2026");
  });

  it("biểu mẫu SỬA không mang câu ấy — một lời cảnh báo hiện ở mọi nơi là lời cảnh báo bị bỏ qua", () => {
    expect(veForm({ kieu: "sua", vb: dong() })).not.toContain(CANH_BAO_GO_KHONG_TRA_SO);
  });
});

describe("409 chưa cấu hình thời hạn — nguyên văn, kèm đường tới chỗ sửa được", () => {
  const CAU_409 =
    "Xã chưa cấu hình thời hạn xử lý cho văn bản đến, nên chưa vào sổ được. " +
    "Vào Cấu hình → Thời hạn xử lý để đặt số giờ xử lý, rồi vào sổ lại.";

  it("câu máy chủ ra nguyên văn, và biểu mẫu có đường dẫn `/cau-hinh`", () => {
    // Đây là lỗi một xã MỚI chắc chắn gặp ở lần vào sổ đầu tiên, và nó hiện ra ở một màn hình khác
    // hẳn màn hình sửa được nó. Câu của máy chủ chỉ đúng chỗ ấy; đường dẫn đưa cán bộ tới đó.
    const html = veForm({ kieu: "them", khoaChongTrung: "khoa-cua-bai-kiem" }, CAU_409);

    expect(html).toContain(CAU_409);
    expect(html).toContain('href="/cau-hinh"');
    expect(html).toContain(NHAN_DUONG_TOI_CAU_HINH);
    expect(html).not.toContain("409");
    expect(html).not.toContain("sla_chua_cau_hinh");
  });

  it("đường dẫn ấy có mặt NGAY CẢ KHI chưa có lỗi nào", () => {
    // Có chủ ý: để biết một câu từ chối có phải câu `sla_chua_cau_hinh` hay không thì phải DÒ CHỮ
    // trong câu máy chủ viết — một bản sao thứ hai của quy tắc nghiệp vụ ở client, và nó im lặng
    // hỏng vào ngày máy chủ sửa câu chữ. Một câu đúng ở mọi lúc thì không có ngày nào sai.
    const html = veForm({ kieu: "them", khoaChongTrung: "khoa-cua-bai-kiem" });
    expect(html).toContain('href="/cau-hinh"');
  });

  it("biểu mẫu vào sổ KHÔNG có ô Số đến và KHÔNG có ô Hạn xử lý", () => {
    // VẾ CHỊU LỰC. Máy chủ trả 400 nếu thân NHẮC TỚI `number`, `status` hay `due_at` — một ô ở đây
    // là mời cán bộ gõ một giá trị rồi xem nó biến mất, và ở ô số thì tệ hơn: họ tin mình vừa cấp
    // lại số 7.
    const html = veForm({ kieu: "them", khoaChongTrung: "k" });

    expect(html).not.toMatch(/<label[^>]*>Số đến</);
    expect(html).not.toMatch(/name="so(VaoSo)?"/);
    expect(html).not.toMatch(/name="han/i);
    expect(html).not.toMatch(/name="trangThai"/);
  });
});

describe("chuyển xử lý rời khỏi biểu mẫu của trang", () => {
  it("biểu mẫu sửa không mang ô nào của khối chuyển — khối ấy nay ở ngăn chi tiết", () => {
    // Khối chuyển và các ca kiểm của nó ở `ngan-van-ban-den.test.tsx`. Ca này canh vế còn lại: một
    // bản sao thứ hai của ba ô chuyển mà còn sót trong biểu mẫu trang là hai đường ghi một dòng
    // lịch sử không sửa được.
    const html = veForm({ kieu: "sua", vb: dong() });

    expect(html).not.toContain("Lý do chuyển");
    expect(html).not.toContain("Chuyển và ghi vết");
    expect(html).not.toMatch(/chưa có đường đọc/);
  });
});

describe("mở ngăn chi tiết từ một dòng", () => {
  it("ô Số đến là một nút mở ngăn, nhãn trợ năng dùng SỐ ĐẾN, không dùng trích yếu", () => {
    const html = veBang(trang([dong()]), TRUOC_HAN, { ghi: false, chuyen: false });

    // Lối cho bàn phím có mặt kể cả khi tài khoản không có quyền ghi nào: xem là quyền ĐỌC.
    expect(html).toContain('aria-label="Xem chi tiết văn bản đến số 7/2026"');
    expect(html).toContain('id="xem-van-ban-den-01JVBDEN0000000000000001"');
    expect(html).toContain('aria-expanded="false"');
    expect(html).not.toMatch(/aria-label="[^"]*rà soát/);
  });

  it("dòng đang mở mang aria-expanded=true, dòng khác thì không", () => {
    const html = renderToStaticMarkup(
      <BangVanBanDen
        kq={trang([dong(), dong({ id: "01JVBDEN0000000000000002", number: 8 })])}
        bayGio={TRUOC_HAN}
        traLoai={TRA_LOAI}
        traBoPhan={TRA_BO_PHAN}
        coQuyenGhi={false}
        coQuyenChuyen={false}
        thaoTac={KHONG_LAM_GI}
        idDangXem="01JVBDEN0000000000000002"
      />,
    );

    expect(html).toMatch(/aria-expanded="false"[^>]*aria-label="Xem chi tiết văn bản đến số 7\/2026"/);
    expect(html).toMatch(/aria-expanded="true"[^>]*aria-label="Xem chi tiết văn bản đến số 8\/2026"/);
  });

  it("ngăn do bên gọi dựng ra tới trang", () => {
    const html = renderToStaticMarkup(
      <ManSoVanBanDen
        kq={trang([dong()])}
        bayGio={TRUOC_HAN}
        nam={2026}
        namGoc={2026}
        datNam={() => {}}
        trangThai=""
        datTrangThai={() => {}}
        loaiLoc=""
        datLoaiLoc={() => {}}
        boPhanLoc=""
        datBoPhanLoc={() => {}}
        tim=""
        datTim={() => {}}
        thuTu=""
        datThuTu={() => {}}
        traLoai={TRA_LOAI}
        traBoPhan={TRA_BO_PHAN}
        coQuyenGhi={false}
        thieuQuyenGhi={false}
        coQuyenChuyen={false}
        thaoTac={KHONG_LAM_GI}
        cauDaXong=""
        loiNgoaiForm=""
        nganXep={TRANG_DAU}
        diToiTrang={() => {}}
        ngan={<p>ngăn-giả-của-bài-kiểm</p>}
        form={null}
      />,
    );

    expect(html).toContain("ngăn-giả-của-bài-kiểm");
  });
});

/* ---- cả màn hình --------------------------------------------------------------------------- */

function veMan(
  quyen: { ghi: boolean; thieuGhi: boolean; chuyen: boolean },
  loc: { tim?: string; thuTu?: MaThuTu } = {},
) {
  return renderToStaticMarkup(
    <ManSoVanBanDen
      kq={trang([dong()])}
      bayGio={TRUOC_HAN}
      nam={2026}
      namGoc={2026}
      datNam={() => {}}
      trangThai=""
      datTrangThai={() => {}}
      loaiLoc=""
      datLoaiLoc={() => {}}
      boPhanLoc=""
      datBoPhanLoc={() => {}}
      tim={loc.tim ?? ""}
      datTim={() => {}}
      thuTu={loc.thuTu ?? ""}
      datThuTu={() => {}}
      traLoai={TRA_LOAI}
      traBoPhan={TRA_BO_PHAN}
      coQuyenGhi={quyen.ghi}
      thieuQuyenGhi={quyen.thieuGhi}
      coQuyenChuyen={quyen.chuyen}
      thaoTac={KHONG_LAM_GI}
      cauDaXong=""
      loiNgoaiForm=""
      nganXep={TRANG_DAU}
      diToiTrang={() => {}}
      form={null}
    />,
  );
}

describe("màn sổ văn bản đến", () => {
  it("đủ quyền: có nút vào sổ, có bộ lọc, có bảng", () => {
    const html = veMan({ ghi: true, thieuGhi: false, chuyen: true });

    expect(html).toContain("Vào sổ văn bản đến");
    expect(html).toContain("Trạng thái");
    expect(html).toContain("Bộ phận đang giữ");
    expect(html).toContain("Về việc rà soát hồ sơ cán bộ");
  });

  it("thiếu quyền ghi: nói rõ thiếu quyền gì, và KHÔNG mất bảng", () => {
    const html = veMan({ ghi: false, thieuGhi: true, chuyen: false });

    expect(html).toMatch(/không có quyền vào sổ/);
    expect(html).toMatch(/vẫn xem được/);
    expect(html).not.toContain("Vào sổ văn bản đến");
    expect(html).toContain("Về việc rà soát hồ sơ cán bộ");
  });

  it("chưa đọc xong quyền: KHÔNG vẽ nút ghi, và cũng KHÔNG nói thiếu quyền", () => {
    // Ba trạng thái, không hai. Nói "bạn không có quyền" trong lúc còn đang đọc là nói một điều
    // chưa biết đúng hay sai — và cán bộ sẽ đi hỏi xin một quyền họ đang có.
    const html = veMan({ ghi: false, thieuGhi: false, chuyen: false });

    expect(html).not.toContain("Vào sổ văn bản đến");
    expect(html).not.toMatch(/không có quyền/);
  });

  it("có ô tìm văn bản, gợi ý nói ĐÚNG hai cột máy chủ tìm ở sổ đến", () => {
    // `store/van_ban_den.go:188`: trích yếu và số, ký hiệu. Gợi ý nhắc "cơ quan ban hành" sẽ là một
    // lời hứa máy chủ không giữ — cán bộ gõ đúng tên cơ quan và nhận một sổ rỗng.
    const html = veMan({ ghi: true, thieuGhi: false, chuyen: true }, { tim: "rà soát" });

    expect(html).toContain('role="search"');
    expect(html).toContain("Tìm văn bản");
    expect(html).toContain(`placeholder="${GOI_Y_TIM_DEN}"`);
    expect(GOI_Y_TIM_DEN).toMatch(/Trích yếu/);
    expect(GOI_Y_TIM_DEN).toMatch(/số, ký hiệu/);
    expect(GOI_Y_TIM_DEN).not.toMatch(/cơ quan|nơi nhận/i);
    // Ô giữ chữ đang tìm sau khi vẽ lại — không thì cán bộ không biết bảng đang lọc theo gì.
    expect(html).toContain('value="rà soát"');
  });

  it("có ô thứ tự; chú thích bảng nói đúng thứ tự đang xem", () => {
    const macDinh = veMan({ ghi: true, thieuGhi: false, chuyen: true });
    const cuTruoc = veMan({ ghi: true, thieuGhi: false, chuyen: true }, { thuTu: "so-tang" });

    expect(macDinh).toContain("Thứ tự");
    expect(macDinh).toContain("Số mới nhất trước");
    expect(macDinh).toContain("Số cũ nhất trước");
    expect(macDinh).toMatch(/<caption[^>]*>[^<]*số mới nhất trước/);
    expect(cuTruoc).toMatch(/<caption[^>]*>[^<]*số cũ nhất trước/);
  });
});
