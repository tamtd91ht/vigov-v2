import { describe, expect, it } from "vitest";

import type {
  identity_danhSachBoPhanRa,
  identity_danhSachVaiTroRa,
} from "@/lib/api/schema.gen";

import { nhanBoPhan, nhanVaiTro } from "./nhan-can-bo";
import { bangTraTuKetQua, traTen } from "./tra-danh-muc";

const BP_LANH_DAO = "01J0000000000000000000BP1";
const VT_CHU_TICH = "01J0000000000000000000VT1";

/** Thân 200 của hai tuyến danh mục — kiểu lấy từ hợp đồng, không gõ tay hình dạng ở đây. */
const BO_PHAN_RA: identity_danhSachBoPhanRa = {
  items: [
    { id: BP_LANH_DAO, code: "lanh-dao", name: "LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ", parent_id: "" },
  ],
};

const VAI_TRO_RA: identity_danhSachVaiTroRa = {
  items: [{ id: VT_CHU_TICH, code: "chu-tich-ubnd", name: "Chủ tịch UBND", is_leader: true }],
};

const DANH_MUC_BO_PHAN = bangTraTuKetQua({ ok: true, duLieu: BO_PHAN_RA });
const DANH_MUC_VAI_TRO = bangTraTuKetQua({ ok: true, duLieu: VAI_TRO_RA });

describe("tra được id ra tên", () => {
  it("bộ phận: id trên dòng danh bạ ra đúng tên trong danh mục", () => {
    expect(traTen(DANH_MUC_BO_PHAN, BP_LANH_DAO)).toEqual({
      loai: "coTen",
      ten: "LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ",
    });
    expect(nhanBoPhan(traTen(DANH_MUC_BO_PHAN, BP_LANH_DAO))).toBe(
      "LÃNH ĐẠO ỦY BAN NHÂN DÂN XÃ",
    );
  });

  it("vai trò: cũng một phép tra, khác danh mục", () => {
    expect(nhanVaiTro(traTen(DANH_MUC_VAI_TRO, VT_CHU_TICH))).toBe("Chủ tịch UBND");
  });

  it("ULID không bao giờ lọt ra màn hình", () => {
    // Một ULID hiện cho cán bộ là một chuỗi vô nghĩa với họ. Mọi nhãn ở dưới đều là câu chữ.
    const moiNhan = [
      nhanBoPhan(traTen(DANH_MUC_BO_PHAN, BP_LANH_DAO)),
      nhanBoPhan(traTen(DANH_MUC_BO_PHAN, "01J0KHONG_CO_TRONG_DANH_MUC")),
      nhanVaiTro(traTen(DANH_MUC_VAI_TRO, "01J0KHONG_CO_TRONG_DANH_MUC")),
    ];
    for (const nhan of moiNhan) expect(nhan).not.toMatch(/01J0/);
  });
});

describe("id không tra được — một trạng thái THẬT, không phải lỗi lập trình", () => {
  // Dòng danh bạ trỏ tới một bộ phận đã bị xoá mềm: danh mục lọc `deleted_at IS NULL` nên id ấy
  // không còn trong danh mục, trong khi `department_id` trên dòng vẫn còn nguyên.
  const KHONG_CO = "01J0000000000000000000XXX";

  it("nói ra bằng một câu, KHÔNG để ô trống", () => {
    expect(traTen(DANH_MUC_BO_PHAN, KHONG_CO)).toEqual({ loai: "khongTraDuoc" });
    expect(nhanBoPhan(traTen(DANH_MUC_BO_PHAN, KHONG_CO))).toBe(
      "Không tra được trong danh mục",
    );
  });

  it("KHÁC HẲN 'chưa phân bộ phận' — hai ca, hai câu, hai việc phải làm", () => {
    // Ô trống trông y hệt "người này chưa được phân bộ phận". Ca đầu là việc của người quản lý
    // nhân sự; ca sau là một dòng dữ liệu lệch chỉ người quản trị sửa được.
    const chuaGan = nhanBoPhan(traTen(DANH_MUC_BO_PHAN, ""));
    const khongTra = nhanBoPhan(traTen(DANH_MUC_BO_PHAN, KHONG_CO));

    expect(chuaGan).toBe("Chưa phân bộ phận");
    expect(chuaGan).not.toBe(khongTra);
    expect(chuaGan).not.toBe("");
    expect(khongTra).not.toBe("");
  });

  it("một mục có tên rỗng cũng là 'không tra được', không phải một ô trống", () => {
    const tenRong: identity_danhSachBoPhanRa = {
      items: [{ id: KHONG_CO, code: "x", name: "", parent_id: "" }],
    };
    const danhMuc = bangTraTuKetQua({ ok: true, duLieu: tenRong });
    expect(traTen(danhMuc, KHONG_CO)).toEqual({ loai: "khongTraDuoc" });
  });
});

describe("`role_id` rỗng là CHƯA GÁN VAI TRÒ", () => {
  it("nói đúng chuyện của nó, không mượn câu của cột bộ phận", () => {
    expect(traTen(DANH_MUC_VAI_TRO, "")).toEqual({ loai: "chuaGan" });
    expect(nhanVaiTro(traTen(DANH_MUC_VAI_TRO, ""))).toBe("Chưa gán vai trò");
    expect(nhanVaiTro(traTen(DANH_MUC_VAI_TRO, ""))).not.toBe(
      nhanBoPhan(traTen(DANH_MUC_BO_PHAN, "")),
    );
  });

  it("trả lời được NGAY cả khi danh mục chưa đọc xong hoặc đọc hỏng", () => {
    // "Chưa gán" đã biết chắc từ chính dòng danh bạ; không cần danh mục nào để khẳng định.
    expect(traTen({ pha: "dangDoc" }, "")).toEqual({ loai: "chuaGan" });
    expect(traTen({ pha: "loi", thongBao: "Phiên làm việc đã hết hạn." }, "")).toEqual({
      loai: "chuaGan",
    });
  });
});

describe("chưa có danh mục — không vu cho mọi dòng cùng một lỗi dữ liệu", () => {
  it("đang đọc thì nói đang đọc, không nói 'không tra được'", () => {
    const ket = traTen({ pha: "dangDoc" }, BP_LANH_DAO);
    expect(ket).toEqual({ loai: "dangDoc" });
    expect(nhanBoPhan(ket)).toBe("Đang tải…");
  });

  it("đọc hỏng thì nói là chưa đọc được DANH MỤC, không nói dòng dữ liệu sai", () => {
    const bang = bangTraTuKetQua({ ok: false, thongBao: "Phiên làm việc đã hết hạn." });
    const ket = traTen(bang, BP_LANH_DAO);

    expect(ket).toEqual({ loai: "khongCoDanhMuc" });
    expect(nhanBoPhan(ket)).toBe("Chưa đọc được danh mục bộ phận");
    expect(nhanVaiTro(ket)).toBe("Chưa đọc được danh mục vai trò");
    // Không được trùng câu của ca "id có thật nhưng danh mục không có" — hai sự cố khác nhau,
    // và chỉ một trong hai là do dữ liệu của xã.
    expect(nhanBoPhan(ket)).not.toBe(nhanBoPhan({ loai: "khongTraDuoc" }));
  });

  it("`null` (chưa gọi xong) không bao giờ thành 'đã đọc và rỗng'", () => {
    // Một xã vừa onboard có danh mục RỖNG thật, và đó là `{pha:"xong", ten: Map(0)}` — khác hẳn
    // `{pha:"dangDoc"}`. Gộp hai cái là báo "không tra được" cho toàn bảng trong lúc đang tải.
    expect(bangTraTuKetQua(null)).toEqual({ pha: "dangDoc" });
    expect(bangTraTuKetQua({ ok: true, duLieu: { items: [] } })).toEqual({
      pha: "xong",
      ten: new Map(),
    });
  });
});

describe("năm ca, năm câu — không hai ca nào nói giống nhau", () => {
  it("cột Bộ phận", () => {
    const cau = new Set([
      nhanBoPhan({ loai: "dangDoc" }),
      nhanBoPhan({ loai: "chuaGan" }),
      nhanBoPhan({ loai: "coTen", ten: "VĂN PHÒNG ĐẢNG ỦY" }),
      nhanBoPhan({ loai: "khongTraDuoc" }),
      nhanBoPhan({ loai: "khongCoDanhMuc" }),
    ]);
    expect(cau.size).toBe(5);
  });

  it("cột Vai trò", () => {
    const cau = new Set([
      nhanVaiTro({ loai: "dangDoc" }),
      nhanVaiTro({ loai: "chuaGan" }),
      nhanVaiTro({ loai: "coTen", ten: "Kế toán" }),
      nhanVaiTro({ loai: "khongTraDuoc" }),
      nhanVaiTro({ loai: "khongCoDanhMuc" }),
    ]);
    expect(cau.size).toBe(5);
  });
});
