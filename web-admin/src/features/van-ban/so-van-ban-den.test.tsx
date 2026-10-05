import { readFileSync } from "node:fs";

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
import {
  CANH_BAO_GO_KHONG_TRA_SO,
  INTAKE_TITLE,
  MORE_FIELDS_TOGGLE,
  NHAN_DUONG_TOI_CAU_HINH,
  OVERDUE_ONLY_LABEL,
  SAVE_AND_NEXT_LABEL,
} from "./nhan-van-ban";
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

function veBang(kq: KetQua<page_Result_documents_vanBanDenRa> | null, bayGio: Date) {
  return renderToStaticMarkup(
    <BangVanBanDen kq={kq} bayGio={bayGio} traBoPhan={TRA_BO_PHAN} thaoTac={KHONG_LAM_GI} />,
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

describe("bảng theo prototype — bảy cột, không cột thao tác (ADR 0068 lần 5)", () => {
  it("đúng bảy cột của prototype, đúng thứ tự", () => {
    const html = veBang(trang([dong()]), TRUOC_HAN);
    const heads = [...html.matchAll(/<th scope="col">([^<]*)<\/th>/g)].map((m) => m[1]);
    expect(heads).toEqual(["Số đến", "Ngày đến", "Cơ quan ban hành", "Trích yếu", "Đang giữ", "Hạn xử lý", "Trạng thái"]);
  });

  it("không có nút ghi nào trên dòng — Sửa, Gỡ và Chuyển xử lý nằm trong ngăn chi tiết", () => {
    // VẾ PHỦ ĐỊNH: the row is read-only whatever the account may do; the detail carries the gates.
    const html = veBang(trang([dong()]), TRUOC_HAN);
    expect(html).not.toContain("Gỡ khỏi sổ");
    expect(html).not.toContain("Chuyển xử lý");
    expect(html).not.toMatch(/aria-label="Sửa /);
    expect(html).not.toContain("nut-xoa");
    expect(html).toContain("Về việc rà soát hồ sơ cán bộ");
  });

  it("dòng quá hạn tô nền — cùng phép so SUY RA với ô hạn, không phải một cờ", () => {
    expect(veBang(trang([dong()]), TRUOC_HAN)).not.toContain("bg-danger-50");
    expect(veBang(trang([dong()]), SAU_HAN)).toContain("bg-danger-50");
  });

  it("độ khẩn từ Khẩn trở lên hiện dưới trích yếu; Thường và không ghi thì không", () => {
    expect(veBang(trang([dong({ urgency: "hoa-toc" })]), TRUOC_HAN)).toContain("Hoả tốc");
    expect(veBang(trang([dong({ urgency: "thuong" })]), TRUOC_HAN)).not.toContain(">Thường<");
    expect(veBang(trang([dong({ urgency: "" })]), TRUOC_HAN)).not.toContain("Không ghi độ khẩn");
  });
});

describe("dữ liệu cá nhân không đi vào thuộc tính nào", () => {
  it("trích yếu và cơ quan ban hành hiện trong ô, KHÔNG nằm trong `aria-label` hay `title`", () => {
    // Luật 3, cấm #4: nhãn trợ năng, `title`, URL và tên tệp là những chỗ một giá trị bị mang đi —
    // vào cây trợ năng, vào nhật ký trình duyệt, vào ảnh chụp màn hình gửi cho hỗ trợ. Nhãn của
    // từng nút vì thế dùng SỐ ĐẾN, thứ vốn sinh ra để đọc qua điện thoại.
    const html = veBang(trang([dong()]), TRUOC_HAN);

    expect(html).toContain("<td>Ban Tổ chức Tỉnh uỷ</td>");
    expect(html).not.toMatch(/aria-label="[^"]*rà soát hồ sơ/);
    expect(html).not.toMatch(/aria-label="[^"]*Ban Tổ chức/);
    expect(html).not.toMatch(/title="[^"]*Ban Tổ chức/);
    expect(html).toContain('aria-label="Xem chi tiết văn bản đến số 7/2026"');
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

  it("rỗng theo bộ lọc: cùng câu của sổ, thêm gợi ý đổi bộ lọc; không lọc thì không có gợi ý", () => {
    const base = { bayGio: TRUOC_HAN, traBoPhan: TRA_BO_PHAN, thaoTac: KHONG_LAM_GI };
    const filtered = renderToStaticMarkup(<BangVanBanDen kq={trang([])} {...base} filtered />);
    const plain = renderToStaticMarkup(<BangVanBanDen kq={trang([])} {...base} />);

    expect(filtered).toContain("Chưa có văn bản đến nào");
    expect(filtered).toContain("Thử đổi hoặc bỏ bớt bộ lọc.");
    expect(plain).not.toContain("Thử đổi hoặc bỏ bớt bộ lọc.");
  });

  it("tải hỏng: nút Tải lại gọi lại đúng lượt đọc của màn; không truyền thì không có nút", () => {
    const base = { bayGio: TRUOC_HAN, traBoPhan: TRA_BO_PHAN, thaoTac: KHONG_LAM_GI };
    const failed = { ok: false as const, thongBao: "Máy chủ đang bận." };

    expect(renderToStaticMarkup(<BangVanBanDen kq={failed} {...base} onReload={() => {}} />)).toContain("Tải lại");
    expect(renderToStaticMarkup(<BangVanBanDen kq={failed} {...base} />)).not.toContain("Tải lại");
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
    const html = veBang(trang([dong()]), TRUOC_HAN);

    // Lối cho bàn phím có mặt với mọi tài khoản đọc được sổ: xem là quyền ĐỌC.
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
        traBoPhan={TRA_BO_PHAN}
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

describe("sau một lần chuyển thành công, ngăn và sổ ĐỌC LẠI — nối dây trong SoVanBanDen", () => {
  /**
   * VÌ SAO CA NÀY ĐỌC MÃ NGUỒN THAY VÌ BẤM NÚT: môi trường test là Node không DOM (xem
   * `vitest.config.mts`), nên `useEffect` và `useCallback` của `SoVanBanDen` không chạy được ở đây.
   * Đây là GIỚI HẠN của ca, nói thẳng: nó canh HÌNH DẠNG của đường nối, không canh hành vi lúc chạy.
   * Một phép thử bằng trình duyệt thật (Playwright) mới canh được trọn.
   *
   * VÌ SAO VẪN ĐÁNG VIẾT: bỏ dòng `datLanDocNgan(...)` khỏi nhánh thành công của `guiChuyen` thì
   * `tsc` sạch, `eslint` sạch (`thucHien` vẫn dùng `datLanDocNgan`, nên `no-unused-vars` không kêu),
   * và mọi ca khác xanh — đã đo 24/09/2026 trên một bản sao. (Bỏ `lanDocNgan` khỏi mảng phụ thuộc
   * của effect thì `no-unused-vars` ĐÃ bắt, nên không viết ca cho vế ấy.) Hậu quả: cán bộ vừa chuyển
   * vẫn nhìn dòng thời gian CŨ, không thấy lần chuyển mình vừa ghi, và bấm lần nữa — một dòng lịch
   * sử thứ hai KHÔNG sửa, KHÔNG xoá được (trigger `lich_su_chuyen_chi_them`; tuyến ấy cố ý không có
   * Idempotency-Key, `van-ban.test.ts`).
   */
  const nguon = readFileSync(new URL("./so-van-ban-den.tsx", import.meta.url), "utf8")
    .replace(/\{\/\*[\s\S]*?\*\/\}/g, "")
    .replace(/\/\*[\s\S]*?\*\//g, "")
    .replace(/^\s*\/\/.*$/gm, "");

  it("nhánh thành công của guiChuyen đọc lại CẢ ngăn LẪN sổ; nhánh lỗi thì không", () => {
    const than = /const guiChuyen = useCallback\(([\s\S]*?)\n {2}\}, \[/.exec(nguon)?.[1];
    expect(than, "không tìm thấy guiChuyen trong so-van-ban-den.tsx").toBeDefined();
    // Khối chuyển gửi id của văn bản ĐANG MỞ trong ngăn và bản nháp của CHÍNH khối ấy — cùng hàm
    // `guiChuyenVanBan` biểu mẫu trang gọi trước lần dời (e3732fd^), không một bản sao thứ hai.
    expect(than).toMatch(/guiChuyenVanBan\(xem\.id, banChuyen\)/);

    const [loi, thanhCong] = (than as string).split(/if \(!k\.ok\) \{[\s\S]*?return;\s*\}/);
    expect(thanhCong, "không tách được nhánh lỗi khỏi nhánh thành công").toBeDefined();
    expect(thanhCong).toMatch(/datLanDocNgan\(/);
    expect(thanhCong).toMatch(/datLanDoc\(/);
    // Nhánh lỗi không đọc lại: câu từ chối phải đứng yên dưới tay người đang sửa bản nháp.
    expect(loi).not.toMatch(/datLanDoc/);
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
    expect(GOI_Y_TIM_DEN).toMatch(/trích yếu/i);
    expect(GOI_Y_TIM_DEN).toMatch(/số ký hiệu/);
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

/* ---- khung prototype (ADR 0068 lần 5) -------------------------------------------------------- */

describe("khung prototype — đầu trang, hàng lọc, hộp thoại", () => {
  it("nút đầu trang: [Vào sổ văn bản đến] hoặc [Quét & OCR ?]; thiếu quyền ghi thì chỉ còn nút “?”", () => {
    const full = veMan({ ghi: true, thieuGhi: false, chuyen: true });
    expect(full.indexOf("Vào sổ văn bản đến")).toBeLessThan(full.indexOf("hoặc"));
    expect(full.indexOf("hoặc")).toBeLessThan(full.indexOf("Quét &amp; OCR"));

    const denied = veMan({ ghi: false, thieuGhi: true, chuyen: false });
    expect(denied).not.toContain("hoặc");
    expect(denied).toContain("Quét &amp; OCR");
  });

  it("frame đặt nút vào đầu trang và thân vào khung tab — sổ không tự vẽ đầu trang", () => {
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
        coQuyenGhi
        thieuQuyenGhi={false}
        thaoTac={KHONG_LAM_GI}
        cauDaXong=""
        loiNgoaiForm=""
        nganXep={TRANG_DAU}
        diToiTrang={() => {}}
        form={null}
        frame={(actions, body) => (
          <>
            <header data-test="dau-trang">{actions}</header>
            <div data-test="than">{body}</div>
          </>
        )}
      />,
    );
    const header = /<header data-test="dau-trang">([\s\S]*?)<\/header>/.exec(html)?.[1] ?? "";
    expect(header).toContain("Vào sổ văn bản đến");
    expect(header).not.toContain("Về việc rà soát hồ sơ cán bộ");
  });

  it("một hàng lọc theo thứ tự prototype: phạm vi → tìm → trạng thái → bộ phận → quá hạn … → Nhập Excel · Xuất sổ", () => {
    const html = veMan({ ghi: true, thieuGhi: false, chuyen: true });
    const order = [
      "Toàn xã",
      'id="tim-van-ban-den"',
      'id="loc-trang-thai-den"',
      'id="loc-bo-phan-den"',
      OVERDUE_ONLY_LABEL,
      'id="loc-loai-den"',
      'id="nam-so-van-ban-den"',
      'id="thu-tu-so-den"',
      "Nhập hàng loạt từ Excel",
      "Xuất sổ 2026",
    ].map((k) => html.indexOf(k));
    for (const i of order) expect(i).toBeGreaterThan(-1);
    expect(order).toEqual([...order].sort((a, b) => a - b));
    // No "Bộ lọc" panel any more (ADR 0068 §12 replaced for this screen).
    expect(html).not.toMatch(/>\s*Bộ lọc\s*</);
  });

  it("“Giao cho tôi”, “Liên quan đến tôi”, “Nhập hàng loạt”, “Xuất sổ” vô hiệu, có “?”; ô quá hạn bấm được", () => {
    const html = veMan({ ghi: true, thieuGhi: false, chuyen: true });
    for (const name of ["Lọc Giao cho tôi / Liên quan đến tôi", "Nhập hàng loạt từ Excel", "Xuất sổ văn bản đến"]) {
      expect(html).toContain(`${name} — tính năng đang phát triển`);
    }
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*>[\s\S]{0,200}?Giao cho tôi/);
    expect(html).toMatch(/<input id="loc-tre-han-den" type="checkbox"(?![^>]*disabled)[^>]*>/);
  });

  it("hộp vào sổ: tiêu đề prototype, bốn ô bắt buộc trước, phần thêm gập, Lưu & nhập tiếp, Ctrl + Enter", () => {
    const html = veForm({ kieu: "them", khoaChongTrung: "k" });
    expect(html).toContain("<dialog");
    expect(html).toContain('aria-labelledby="tieu-de-bieu-mau-van-ban-den"');
    expect(html).toContain(INTAKE_TITLE);
    expect(html).toContain(SAVE_AND_NEXT_LABEL.replace("&", "&amp;"));
    expect(html).toContain("Ctrl + Enter");
    expect(html).toContain(MORE_FIELDS_TOGGLE);
    expect(html).toContain('aria-expanded="false"');
    for (const id of ["o-ngay-den", "o-loai-van-ban", "o-co-quan", "o-trich-yeu"]) expect(html).toContain(`id="${id}"`);
    // Optional fields stay folded until asked for.
    expect(html).not.toContain('id="o-so-ky-hieu"');
    expect(html).not.toContain('id="o-do-khan"');
  });

  it("hộp sửa: không có Lưu & nhập tiếp; giá trị phụ đang có thì phần thêm mở sẵn", () => {
    const html = veForm({ kieu: "sua", vb: dong() }, "", { ...BAN_DEN_TRONG, soKyHieu: "1742-CV/BTCTU" });
    expect(html).toContain("Sửa văn bản đến");
    expect(html).not.toContain(SAVE_AND_NEXT_LABEL.replace("&", "&amp;"));
    expect(html).toContain('id="o-so-ky-hieu"');
    expect(html).toContain('value="1742-CV/BTCTU"');
  });

  it("sau Lưu & nhập tiếp: câu đã lưu hiện ngay trong hộp đang mở", () => {
    const html = renderToStaticMarkup(
      <BieuMauVanBanDen
        dangMo={{ kieu: "them", khoaChongTrung: "k2" }}
        ban={BAN_DEN_TRONG}
        datBan={() => {}}
        traLoai={TRA_LOAI}
        loi=""
        dangGui={false}
        savedNotice="Đã vào sổ số đến 7/2026."
        onGui={() => {}}
        onHuy={() => {}}
      />,
    );
    expect(html).toMatch(/role="status"[^>]*>[\s\S]*Đã vào sổ số đến 7\/2026\./);
  });

  it("hộp gỡ là hộp thoại, lý do bắt buộc vẫn có", () => {
    const html = veForm({ kieu: "go", vb: dong() });
    expect(html).toContain("<dialog");
    expect(html).toContain("Gỡ văn bản đến số 7/2026 khỏi sổ?");
    expect(html).toMatch(/<input id="o-ly-do-go-den"[^>]*required/);
  });
});
