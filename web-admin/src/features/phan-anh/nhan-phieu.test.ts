import { describe, expect, it } from "vitest";

import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import {
  buocLuongChinh,
  cauGiaiThichTrangThai,
  conBuocKeTiep,
  LINH_VUC_PHAN_ANH,
  linhVucPhanAnh,
  lopHan,
  LUONG_CHINH,
  MOI_TRANG_THAI,
  nhanBoPhan,
  nhanCanBoXuLy,
  nhanHan,
  nhanHienCongKhai,
  nhanKenh,
  nhanLinhVuc,
  nhanNguoiGui,
  nhanThoiDiem,
  nhanTrangThai,
  phanLoaiDuoc,
  RE_NHANH,
  trangThaiHan,
} from "./nhan-phieu";

function phieu(sua: Partial<petitions_phieuPhanAnhRa> = {}): petitions_phieuPhanAnhRa {
  return {
    code: "PA-2026-0021",
    channel: "zalo-mini-app",
    status: "dang-phan-loai",
    field: "",
    field_label: "",
    content: "Rác tồn đọng ở đầu ngõ.",
    address: "Tổ 6",
    // Số điện thoại giả đã thống nhất, ở dạng máy chủ che (luật 3, bất biến 5).
    reporter_name: "Nguyễn V. A.",
    reporter_phone: "09****0000",
    anonymous: false,
    clock_from: "2026-09-09T07:20:00Z",
    booked_at: "2026-09-09T07:21:00Z",
    acknowledge_due: null,
    resolve_due: null,
    // Hạn BẮT BUỘC PHÂN LOẠI — trần 1 ngày làm việc, ADR 0035 §C (câu mở #26,
    // chốt 22/09/2026). `null` là trạng thái THẬT: phiếu đã phân loại rồi thì trần
    // ấy không còn nghĩa gì. Nó là hạn THỨ BA của một phiếu, cạnh hạn tiếp nhận và
    // hạn xử lý xong — cả ba đều LƯU một lần tại hành vi ấn định, không tính lại.
    classify_due: null,
    // BỐN TRƯỜNG CỦA ĐƯỜNG XỬ LÝ PHÍA CÁN BỘ (`service-petitions`, 23/09/2026). Chuỗi rỗng
    // là trạng thái THẬT — "chưa phân công bộ phận nào", "chưa có kết quả" — chứ không
    // phải thiếu dữ liệu, nên hợp đồng khai `string` chứ không `string | null`.
    //
    // `public` là cờ CÔNG KHAI phiếu ra kênh công dân, mặc định tắt: một phiếu mang họ tên
    // và số điện thoại người gửi (luật 3), nên công khai phải là một hành vi có người bấm.
    unit: "",
    assignee: "",
    result: "",
    public: false,
    ...sua,
  };
}

describe("nhãn trạng thái và kênh", () => {
  it("chín trạng thái của vòng đời đều có nhãn tiếng Việt", () => {
    for (const ma of [
      "da-tiep-nhan",
      "dang-phan-loai",
      "da-chuyen-xu-ly",
      "dang-xu-ly",
      "da-xu-ly",
      "cho-dan-xac-nhan",
      "da-dong",
      "khong-tiep-nhan",
      "chuyen-cap-tren",
    ]) {
      expect(nhanTrangThai(ma)).not.toContain(ma);
    }
  });

  it("bốn kênh đều có nhãn", () => {
    expect(nhanKenh("zalo-mini-app")).toBe("Zalo Mini App");
    expect(nhanKenh("can-bo-nhap-ho")).toBe("Cán bộ nhập hộ");
  });

  it("mã LẠ hiện nguyên mã VÀ nói rõ nó chưa có nhãn — không im lặng, không ô trống", () => {
    // Hợp đồng khai hai trường này là `string` trơn, không kèm `enum`, nên `tsc` không canh được
    // một trạng thái thứ mười. Nhánh dự phòng vì thế phải NÓI RA chứ không giấu đi.
    expect(nhanTrangThai("trang-thai-moi")).toContain("trang-thai-moi");
    expect(nhanTrangThai("trang-thai-moi")).toContain("chưa có nhãn");
  });
});

describe("lĩnh vực phản ánh", () => {
  it("chưa phân loại KHÁC HẲN nhãn chưa được xã đặt lại", () => {
    // `field` rỗng nghĩa là phiếu chưa được phân loại — đó là lý do phiếu ấy chưa có hạn xử lý.
    // `field_label` rỗng chỉ nghĩa là xã chưa đặt lại tên cho mã, và đó là ca thông thường.
    expect(nhanLinhVuc(linhVucPhanAnh("", ""))).toBe("Chưa phân loại");
    expect(nhanLinhVuc(linhVucPhanAnh("rac-thai", ""))).toBe("rac-thai");
    expect(nhanLinhVuc(linhVucPhanAnh("rac-thai", "Rác thải – Vệ sinh môi trường"))).toBe(
      "Rác thải – Vệ sinh môi trường",
    );
  });
});

describe("người gửi", () => {
  it("hiện ĐÚNG dạng đã che của máy chủ, không ghép lại và không thêm chữ số nào", () => {
    expect(nhanNguoiGui(phieu())).toBe("Nguyễn V. A. · 09****0000");
  });

  it("ẩn danh: KHÔNG hiện tên đã che, vì một cái tên đã che vẫn là một cái tên", () => {
    // Trong một xã vài nghìn người, "Nguyễn V. A." vẫn chỉ ra một người — và đúng điều cờ ẩn
    // danh bảo vệ là cán bộ đang xử lý không biết ai gửi. Máy chủ trả cả hai trường RỖNG.
    const an = phieu({ anonymous: true, reporter_name: "", reporter_phone: "" });
    expect(nhanNguoiGui(an)).toBe("Người gửi ẩn danh");
  });

  it("ẩn danh mà máy chủ lỡ gửi kèm tên: màn hình VẪN không hiện", () => {
    // Đóng khi chưa chắc. Nếu có ngày máy chủ hỏng theo chiều ấy, chỗ hỏng không được là màn
    // hình hiện tên của một người đã xin giấu tên.
    const an = phieu({ anonymous: true, reporter_name: "Nguyễn V. A.", reporter_phone: "09****0000" });
    expect(nhanNguoiGui(an)).toBe("Người gửi ẩn danh");
  });

  it("không ẩn danh mà cả hai trường rỗng: nói ra, không để ô trống", () => {
    expect(nhanNguoiGui(phieu({ reporter_name: "", reporter_phone: "" }))).toBe(
      "Không có thông tin người gửi",
    );
  });
});

describe("hạn xử lý", () => {
  const bayGio = new Date("2026-09-09T10:00:00Z");

  it("`acknowledge_due` null nghĩa là KHÔNG ÁP DỤNG — và không bao giờ hiện thành 0", () => {
    // Phiếu do cán bộ nhập hộ: cán bộ CHÍNH LÀ người đọc, nên khoảng ấy không tồn tại. Hiện 0 sẽ
    // làm một xã nhập hộ nhiều phiếu báo cáo thời gian tiếp nhận trung bình gần bằng không.
    const h = trangThaiHan(null, "khongApDung", bayGio);
    expect(nhanHan(h)).toBe("Không áp dụng");
    expect(nhanHan(h)).not.toContain("0");
  });

  it("`resolve_due` null nghĩa là CHƯA CÓ — nghĩa ngược lại, và câu chữ phải khác", () => {
    const h = trangThaiHan(null, "chuaCo", bayGio);
    expect(nhanHan(h)).toContain("Chưa ấn định");
    expect(nhanHan(h)).not.toBe(nhanHan(trangThaiHan(null, "khongApDung", bayGio)));
  });

  it("quá mốc thì SUY RA quá hạn — không có trường `overdue` nào trên hợp đồng", () => {
    const h = trangThaiHan("2026-09-09T09:20:00Z", "chuaCo", bayGio);
    expect(h.loai).toBe("quaHan");
    expect(nhanHan(h)).toContain("Quá hạn");
    expect(lopHan(h)).toBe("nhan-lech");
  });

  it("chưa tới mốc thì còn hạn", () => {
    const h = trangThaiHan("2026-09-09T11:20:00Z", "chuaCo", bayGio);
    expect(h.loai).toBe("conHan");
    expect(lopHan(h)).toBeUndefined();
  });

  it("KHÔNG đếm 'quá hạn mấy ngày' — đó là khoảng tính bằng giờ làm việc, do `identity` sở hữu", () => {
    // Đặc tả §8.3 vẽ "Quá hạn 3 ngày". Con số ấy cần lịch làm việc, ngày nghỉ lễ và ngày làm bù
    // của chính xã ấy (ADR 0007). Đếm bằng giờ đồng hồ ở trình duyệt sẽ ra một số khác số của
    // máy chủ vào đúng dịp lễ — và số hiện trên màn hình cán bộ là số được báo cáo lên trên.
    const h = trangThaiHan("2026-09-01T09:20:00Z", "chuaCo", bayGio);
    expect(nhanHan(h)).not.toMatch(/\d+\s*(ngày|giờ)\b/);
  });
});

describe("thời điểm", () => {
  it("hiện theo giờ Việt Nam, GHIM, không theo cài đặt của máy", () => {
    // 07:20 UTC là 14:20 giờ Việt Nam — đúng ví dụ đặc tả §8.1 in ra. Máy chạy test đặt múi giờ
    // nào cũng phải ra cùng một chuỗi, vì một hạn xử lý là cam kết của một cơ quan nhà nước.
    expect(nhanThoiDiem("2026-09-09T07:20:00Z")).toContain("14:20");
    expect(nhanThoiDiem("2026-09-09T07:20:00Z")).toContain("09/09/2026");
  });

  it("chuỗi không đọc được hiện NGUYÊN VĂN, không hiện 'Invalid Date'", () => {
    expect(nhanThoiDiem("hong")).toBe("hong");
  });
});

describe("hiển thị với người dân", () => {
  it("hai ca nói hai câu khác nhau, và ca chưa công khai giữ nguyên lời trấn an của đặc tả", () => {
    expect(nhanHienCongKhai(false)).toContain("Người gửi vẫn tra cứu được phiếu của mình");
    expect(nhanHienCongKhai(true)).not.toBe(nhanHienCongKhai(false));
  });
});

describe("chín trạng thái — danh sách ĐÓNG, khách duyệt nguyên văn 20/09/2026", () => {
  it("bảy luồng chính cộng hai rẽ nhánh, đúng chín mã, đúng thứ tự vòng đời", () => {
    // Đổi một trong chín chuỗi là DI TRÚ HỒ SƠ LƯU TRỮ (luật 7), không phải đổi tên. Bài này viết
    // lại chín chuỗi bằng tay CÓ CHỦ Ý: nó phải đỏ khi ai đó "sửa chính tả" một mã.
    expect(LUONG_CHINH).toEqual([
      "da-tiep-nhan",
      "dang-phan-loai",
      "da-chuyen-xu-ly",
      "dang-xu-ly",
      "da-xu-ly",
      "cho-dan-xac-nhan",
      "da-dong",
    ]);
    expect(RE_NHANH).toEqual(["khong-tiep-nhan", "chuyen-cap-tren"]);
    expect(MOI_TRANG_THAI).toHaveLength(9);
  });

  it("cả chín mã đều có nhãn — không mã nào rơi xuống nhánh dự phòng", () => {
    for (const ma of MOI_TRANG_THAI) {
      expect(nhanTrangThai(ma)).not.toContain("chưa có nhãn");
    }
  });
});

describe("mười hai lĩnh vực — §5", () => {
  it("đủ mười hai mã, và `can-bo` CÓ trong ô chọn", () => {
    expect(LINH_VUC_PHAN_ANH).toHaveLength(12);
    // Lĩnh vực hạn chế ở đường ĐỌC, nhưng chốt lĩnh vực VÀO nó là hành vi được phép — đúng tình
    // huống cán bộ đọc phiếu rồi nhận ra nó nói về một đồng nghiệp. Bỏ mục ấy là bịt đường phân
    // loại đúng.
    expect(LINH_VUC_PHAN_ANH.map((l) => l.ma)).toContain("can-bo");
  });
});

describe("StatusStepper §8.2", () => {
  it("phiếu trên luồng chính: đúng MỘT ô `đang ở đây`, các ô trước là `đã qua`", () => {
    const o = buocLuongChinh("dang-xu-ly");
    expect(o.filter((x) => x.vaiTro === "dangODay").map((x) => x.ma)).toEqual(["dang-xu-ly"]);
    expect(o.filter((x) => x.vaiTro === "daQua")).toHaveLength(3);
  });

  it("phiếu ở RẼ NHÁNH: KHÔNG ô nào sáng — không đoán bừa một vị trí trên luồng chính", () => {
    // Một ô tô màu sai nói với cán bộ rằng phiếu còn đang chạy trên luồng chính.
    for (const ma of RE_NHANH) {
      expect(buocLuongChinh(ma).some((x) => x.vaiTro === "dangODay")).toBe(false);
    }
  });

  it("mã lạ (hợp đồng mọc thêm trạng thái) cũng không làm sáng ô đầu tiên", () => {
    expect(buocLuongChinh("mot-ma-moi").every((x) => x.vaiTro === "chuaToi")).toBe(true);
  });

  it("chỉ MỘT trong chín trạng thái có câu giải thích — tám câu còn lại không được bịa", () => {
    const coCau = MOI_TRANG_THAI.filter((ma) => cauGiaiThichTrangThai(ma) !== null);
    expect(coCau).toEqual(["dang-phan-loai"]);
    expect(cauGiaiThichTrangThai("dang-phan-loai")).toBe(
      "Đang xem phiếu thuộc lĩnh vực nào, có tiếp nhận không.",
    );
  });
});

describe("khi nào vẽ nút nào", () => {
  it("phân loại chỉ có nghĩa ở `da-tiep-nhan` — lần thứ hai máy chủ trả 409", () => {
    expect(phanLoaiDuoc("da-tiep-nhan")).toBe(true);
    for (const ma of MOI_TRANG_THAI.filter((m) => m !== "da-tiep-nhan")) {
      expect(phanLoaiDuoc(ma)).toBe(false);
    }
  });

  it("bước cuối luồng chính và hai rẽ nhánh thì không còn bước kế tiếp", () => {
    expect(conBuocKeTiep("dang-xu-ly")).toBe(true);
    expect(conBuocKeTiep("da-dong")).toBe(false);
    for (const ma of RE_NHANH) expect(conBuocKeTiep(ma)).toBe(false);
  });
});

describe("bộ phận và cán bộ đang giữ phiếu — hai ULID, hai câu trả lời khác nhau", () => {
  it("id không tra được thì nói ra, KHÔNG in id lên màn hình cán bộ", () => {
    const ten = new Map([["01JBOPHAN", "VĂN PHÒNG ĐẢNG ỦY"]]);
    expect(nhanBoPhan("01JBOPHAN", ten)).toBe("VĂN PHÒNG ĐẢNG ỦY");
    expect(nhanBoPhan("", ten)).toBe("Chưa chuyển bộ phận");
    expect(nhanBoPhan("01JKHAC", ten)).not.toContain("01JKHAC");
  });

  it("cán bộ xử lý: không bao giờ in ULID ra màn hình", () => {
    expect(nhanCanBoXuLy("")).toBe("Chưa phân công cán bộ cụ thể");
    expect(nhanCanBoXuLy("01JCANBO")).not.toContain("01JCANBO");
  });
});
