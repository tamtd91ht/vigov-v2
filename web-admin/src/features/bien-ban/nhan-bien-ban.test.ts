import { readFileSync } from "node:fs";

import { describe, expect, it } from "vitest";

import type { petitions_bienBanRa, petitions_ketLuanRa } from "@/lib/api/schema.gen";

import { danhBaTheoMa } from "@/features/phan-anh/nhan-phieu";

import {
  BIEU_MAU_TRONG,
  CHUA_TACH_NHIEM_VU,
  DAU_GACH,
  dongMeta,
  duongDanBienBan,
  giaTriTuBienBan,
  idTuNeo,
  lopChipKetLuan,
  luaChonCanBo,
  nhanBadge,
  nhanCanBo,
  nhanNgayHop,
  nhanThongBao,
  nhanTienDoBienBan,
  nhanTienDoKetLuan,
  nhanTrangThaiBienBan,
  nhanTrangThaiKetLuan,
  PHAN_CHUA_DUNG,
  quyTacBienBan,
  quyTacKetLuan,
  soThuTuKetLuan,
  tachKetLuan,
  tachThanhPhan,
  thanSuaTuBieuMau,
  thanTaoTuBieuMau,
  thanThongBao,
  VI_SAO_BIEN_BAN_CON_NHIEM_VU,
  VI_SAO_KET_LUAN_KHOA,
} from "./nhan-bien-ban";

function ketLuan(sua: Partial<petitions_ketLuanRa> = {}): petitions_ketLuanRa {
  return {
    id: "01JKL1",
    ordinal: 1,
    content: "Giao bộ phận Địa chính rà soát tiến độ tuyến đường Hà Lam – Bình Trị.",
    task_count: 0,
    task_done_count: 0,
    status: "chua-giao",
    no_task: false,
    created_at: "2026-08-05T02:00:00Z",
    ...sua,
  };
}

function bienBan(sua: Partial<petitions_bienBanRa> = {}): petitions_bienBanRa {
  return {
    id: "01JBB1",
    title: "Giao ban Uỷ ban nhân dân xã tháng 8 năm 2026",
    held_on: "2026-08-05",
    reference_no: "31/BB-UBND",
    location: "Phòng họp UBND xã",
    chaired_by: "",
    conclusions: [ketLuan()],
    task_count: 3,
    task_done_count: 1,
    status: "du-thao",
    conclusion_count: 1,
    conclusion_done_count: 0,
    created_by: "CB-2026-7K3M9Q",
    created_at: "2026-08-05T02:00:00Z",
    ...sua,
  };
}

describe("ngày họp là một NGÀY LỊCH, không phải một mốc thời gian", () => {
  it("`2026-08-05` thành `5/8/2026`, không số 0 đứng đầu", () => {
    expect(nhanNgayHop("2026-08-05")).toBe("5/8/2026");
    expect(nhanNgayHop("2026-12-31")).toBe("31/12/2026");
  });

  it("KHÔNG đi qua `new Date` — nguồn tệp không có phép dựng ngày nào", () => {
    // Đây là bài kiểm về MÃ NGUỒN vì hậu quả không hiện ra ở máy chạy test: `new Date("2026-08-05")`
    // là nửa đêm UTC, và chỉ trình duyệt ở múi giờ âm mới vẽ ra ngày 4/8. Một bài kiểm chạy ở UTC
    // sẽ xanh vĩnh viễn trong khi màn hình của một xã hiện sai ngày họp.
    const nguon = readFileSync(new URL("./nhan-bien-ban.ts", import.meta.url), "utf8")
      .replace(/\/\*[\s\S]*?\*\//g, "")
      .split("\n")
      .filter((d) => !/^\s*\/\//.test(d))
      .join("\n");

    expect(nguon).not.toMatch(/new\s+Date\s*\(/);
    expect(nguon).not.toMatch(/Date\s*\.\s*parse/);
    expect(nguon).not.toMatch(/toLocaleDateString/);
  });

  it("chuỗi không đọc được thì là dấu gạch, KHÔNG phải một ngày bịa ra", () => {
    expect(nhanNgayHop("")).toBe(DAU_GACH);
    expect(nhanNgayHop("05/08/2026")).toBe(DAU_GACH);
    expect(nhanNgayHop("2026-08-05T00:00:00Z")).toBe(DAU_GACH);
    expect(nhanNgayHop("2026-13-05")).toBe(DAU_GACH);
  });
});

describe("dòng meta: phần nào thiếu thì bỏ", () => {
  it("đủ ba phần thì nối bằng ` · `", () => {
    expect(dongMeta(bienBan())).toBe("5/8/2026 · 31/BB-UBND · Phòng họp UBND xã");
  });

  it("thiếu số hiệu thì BỎ HẲN đoạn ấy, không để một chỗ trống giữa hai dấu chấm", () => {
    expect(dongMeta(bienBan({ reference_no: "" }))).toBe("5/8/2026 · Phòng họp UBND xã");
  });

  it("chỉ còn ngày họp thì dòng meta chỉ có ngày họp", () => {
    expect(dongMeta(bienBan({ reference_no: "", location: "" }))).toBe("5/8/2026");
  });

  it("ngày họp không đọc được thì nó cũng bị bỏ khỏi dòng meta", () => {
    // Một dấu gạch đứng lẫn giữa số hiệu và địa điểm trông như một trường rỗng chứ không như một
    // giá trị hỏng.
    expect(dongMeta(bienBan({ held_on: "" }))).toBe("31/BB-UBND · Phòng họp UBND xã");
  });
});

describe("hai bộ đếm, và chúng đến từ MÁY CHỦ", () => {
  it("badge dùng `task_count`/`task_done_count` của biên bản, không cộng lại từ mảng kết luận", () => {
    // Biên bản có MỘT kết luận với 0 nhiệm vụ, nhưng máy chủ nói 1/3. Cộng lại ở client sẽ ra
    // `0/0` — một con số thứ hai của cùng một sự thật, và là con số lãnh đạo đọc.
    expect(nhanBadge(bienBan())).toBe("1/3 nhiệm vụ xong");
  });

  it("con số CHÍNH là kết luận hoàn thành, đọc từ `conclusion_*` của máy chủ — không đếm mảng", () => {
    // Mảng có MỘT kết luận "chua-giao", nhưng máy chủ nói 2/5: màn vẽ 2/5.
    expect(nhanTienDoBienBan(bienBan({ conclusion_done_count: 2, conclusion_count: 5 }))).toBe(
      "2/5 kết luận hoàn thành",
    );
  });

  it("`Chưa tách thành nhiệm vụ nào` KHÁC `0/3 nhiệm vụ đã hoàn thành`", () => {
    expect(nhanTienDoKetLuan(ketLuan({ task_count: 0, task_done_count: 0 }))).toBe(
      CHUA_TACH_NHIEM_VU,
    );
    expect(nhanTienDoKetLuan(ketLuan({ task_count: 3, task_done_count: 0 }))).toBe(
      "0/3 nhiệm vụ đã hoàn thành",
    );
    expect(nhanTienDoKetLuan(ketLuan({ task_count: 1, task_done_count: 1 }))).toBe(
      "1/1 nhiệm vụ đã hoàn thành",
    );
  });
});

describe("số thứ tự kết luận — số ĐÃ CẤP, không phải vị trí trong danh sách", () => {
  it("lấy đúng `ordinal` máy chủ trả, kể cả khi nó không liên tục", () => {
    // Xoá mềm kết luận ② thì kết luận kế tiếp mang số ④. Khoảng trống ấy ĐÚNG (luật 7, bất biến
    // 3): số đã cấp thì không bao giờ cấp lại, và biên bản giấy đã in mang con số cũ.
    expect(soThuTuKetLuan(ketLuan({ ordinal: 4 }))).toBe(4);
    expect(soThuTuKetLuan(ketLuan({ ordinal: 1 }))).toBe(1);
  });
});

describe("tách ô nhập nhiều dòng", () => {
  it("mỗi dòng một mục, bỏ dòng trắng, giữ nguyên thứ tự người gõ", () => {
    expect(tachThanhPhan("Ông A\n\n  Bà B  \n")).toEqual(["Ông A", "Bà B"]);
    expect(tachKetLuan("Kết luận một\nKết luận hai\n")).toEqual([
      "Kết luận một",
      "Kết luận hai",
    ]);
  });

  it("ô trống cho ra danh sách rỗng — §7.3: biên bản không có kết luận vẫn lưu được", () => {
    expect(tachKetLuan("")).toEqual([]);
    expect(tachKetLuan("   \n  ")).toEqual([]);
  });
});

describe("phần chưa dựng được", () => {
  it("mỗi mục có tên và lý do, không mục nào rỗng", () => {
    expect(PHAN_CHUA_DUNG.length).toBeGreaterThan(0);
    for (const p of PHAN_CHUA_DUNG) {
      expect(p.ten.trim()).not.toBe("");
      expect(p.viSao.trim()).not.toBe("");
    }
  });

  /**
   * CA NÀY ĐÃ ĐỔI HAI LẦN TRONG MỘT NGÀY, và cả hai lần đều đổi theo HÀNH VI chứ không bị gỡ:
   *
   *   bản 1  đòi "nút `Tách thành nhiệm vụ`" nằm trong danh sách — đúng tới khi nút được dựng
   *   bản 2  đòi "ĐIỀN SẴN" nằm trong danh sách — đúng được vài giờ, tới khi `tieuDeCoSan` ra đời
   *   bản 3  (đây) đòi HẠN GỢI Ý, và đòi lý do nói rõ đó là CÂU CHỜ KHÁCH chứ không phải việc nợ
   *
   * Nhịp ấy chính là điều ca này tồn tại để giữ: mỗi lần một mảnh §3 được dựng, danh sách
   * chưa-dựng-được phải co lại theo. Ai dựng nốt hạn gợi ý mà quên xoá mục này, hoặc xoá mục mà
   * chưa dựng, thì đúng ca này đỏ.
   */
  it("thứ CÒN THIẾU của §3 là HẠN GỢI Ý, và lý do nói rõ đó là câu chờ khách", () => {
    const muc = PHAN_CHUA_DUNG.find((p) => p.ten.includes("HẠN GỢI Ý"));

    expect(muc).toBeDefined();
    // ĐIỀN SẴN đã dựng xong (`tieuDeCoSan`), nên nó KHÔNG được còn nằm trong danh sách như một
    // thứ chưa có — một danh sách kể tên thứ đã dựng là danh sách khiến người sau dựng lần hai.
    expect(PHAN_CHUA_DUNG.some((p) => p.ten.includes("ĐIỀN SẴN"))).toBe(false);
    // Lý do phải dẫn ĐÚNG CHỖ máy chủ đã từ chối tự suy ngày, kèm số đo. Thiếu nó thì mục này
    // đọc ra như một việc chưa ai làm, và người sau sẽ làm — ở client, nơi không ai kiểm được.
    expect(muc?.viSao).toContain("bien_ban_hop_ghi.go");
    expect(muc?.viSao).toContain("ba trên bốn lần");
  });

  /**
   * NHỊP CO LẠI, LẦN THỨ TƯ (25/09/2026): hợp đồng đã có tuyến chi tiết, sửa/gỡ, danh sách nhiệm vụ
   * của một kết luận, danh bạ chọn người và thứ tự theo ngày họp — năm mục ấy RỜI danh sách. Ba mục
   * vẫn đúng (tệp đính kèm, hạn gợi ý, CSS) thì Ở LẠI. Một danh sách kể tên thứ đã dựng là danh sách
   * khiến người sau dựng lần hai.
   */
  it("năm mục đã dựng RỜI danh sách; tệp đính kèm, hạn gợi ý, CSS Ở LẠI", () => {
    const ten = PHAN_CHUA_DUNG.map((p) => p.ten).join("\n");
    expect(ten).not.toContain("Ô chọn `Chủ trì`");
    expect(ten).not.toContain("KHI ĐỌC LẠI");
    expect(ten).not.toContain("Sửa, xoá biên bản");
    expect(ten).not.toContain("Danh sách nhiệm vụ đã tách");
    expect(ten).not.toContain("Thứ tự thẻ theo NGÀY HỌP");

    expect(ten).toContain("Tệp đính kèm");
    expect(ten).toContain("HẠN GỢI Ý");
    expect(ten).toContain("Lớp CSS");
  });
});

describe("trạng thái biên bản và kết luận — chữ cho mã của MÁY CHỦ", () => {
  it("biên bản: dự thảo / đã ký; mã lạ hiện nguyên văn", () => {
    expect(nhanTrangThaiBienBan(bienBan({ status: "du-thao" }))).toBe("Dự thảo");
    expect(nhanTrangThaiBienBan(bienBan({ status: "da-ky" }))).toBe("Đã ký");
    expect(nhanTrangThaiBienBan(bienBan({ status: "ma-la" }))).toBe("ma-la");
  });

  it("kết luận: bốn mã suy ra, và “không phát sinh” đi TRƯỚC `status`", () => {
    expect(nhanTrangThaiKetLuan(ketLuan({ status: "chua-giao" }))).toBe("Chưa giao");
    expect(nhanTrangThaiKetLuan(ketLuan({ status: "dang-thuc-hien" }))).toBe("Đang thực hiện");
    expect(nhanTrangThaiKetLuan(ketLuan({ status: "qua-han" }))).toBe("Quá hạn");
    expect(nhanTrangThaiKetLuan(ketLuan({ status: "hoan-thanh" }))).toBe("Hoàn thành");
    expect(nhanTrangThaiKetLuan(ketLuan({ status: "hoan-thanh", no_task: true }))).toBe(
      "Không phát sinh nhiệm vụ",
    );
  });

  /**
   * CA DUY NHẤT PHÂN BIỆT ĐƯỢC "ĐỌC `status`" VỚI "SUY TỪ BỘ ĐẾM", và nó là ca thật: 1/3 việc xong,
   * một việc chưa xong đã trễ. Máy chủ biết việc trễ (so với hạn, `dieuKienTreHan`), client thì
   * KHÔNG — dây không mang số việc trễ. Client suy từ `task_count`/`task_done_count` sẽ vẽ
   * "Đang thực hiện" màu xám cho đúng kết luận lãnh đạo cần thấy màu đỏ. Các ca trên dùng bộ đếm 0/0
   * nên một phép suy "còn việc → đang thực hiện" đặt trước `switch` vẫn xanh qua chúng.
   */
  it("1/3 việc xong mà máy chủ nói quá hạn thì hiện QUÁ HẠN, chip đỏ — không suy lại từ bộ đếm", () => {
    const kl = ketLuan({ task_count: 3, task_done_count: 1, status: "qua-han" });
    expect(nhanTrangThaiKetLuan(kl)).toBe("Quá hạn");
    expect(lopChipKetLuan(kl)).toBe("chip chip-cham");
  });

  it("quá hạn là chip đỏ; hoàn thành và không phát sinh là chip xanh", () => {
    expect(lopChipKetLuan(ketLuan({ status: "qua-han" }))).toBe("chip chip-cham");
    expect(lopChipKetLuan(ketLuan({ status: "hoan-thanh" }))).toBe("chip chip-hoat-dong");
    expect(lopChipKetLuan(ketLuan({ no_task: true }))).toBe("chip chip-hoat-dong");
    expect(lopChipKetLuan(ketLuan({ status: "chua-giao" }))).toBe("chip chip-ngung");
  });
});

describe("quy tắc nút — tiện dụng, FAIL CLOSED theo trạng thái", () => {
  const NHAP = bienBan({ status: "du-thao", task_count: 0 });
  const DA_KY = bienBan({ status: "da-ky" });

  it("kết luận của bản nháp, chưa có nhiệm vụ: sửa/gỡ bấm được, đánh dấu có, tách có", () => {
    const qt = quyTacKetLuan(NHAP, ketLuan());
    expect(qt.sua).toEqual({ hien: true, viSaoTat: null });
    expect(qt.go).toEqual({ hien: true, viSaoTat: null });
    expect(qt.danhDau.hien).toBe(true);
    expect(qt.boDau.hien).toBe(false);
    expect(qt.tach.hien).toBe(true);
  });

  it("kết luận đã có nhiệm vụ: sửa/gỡ TẮT kèm lý do; đánh dấu ẨN", () => {
    const qt = quyTacKetLuan(NHAP, ketLuan({ task_count: 1 }));
    expect(qt.sua).toEqual({ hien: true, viSaoTat: VI_SAO_KET_LUAN_KHOA });
    expect(qt.go).toEqual({ hien: true, viSaoTat: VI_SAO_KET_LUAN_KHOA });
    expect(qt.danhDau.hien).toBe(false);
  });

  it("đang đánh dấu “không phát sinh”: bỏ dấu có, tách ẨN", () => {
    const qt = quyTacKetLuan(NHAP, ketLuan({ no_task: true }));
    expect(qt.boDau.hien).toBe(true);
    expect(qt.danhDau.hien).toBe(false);
    expect(qt.tach.hien).toBe(false);
  });

  it("biên bản đã ký: sửa/gỡ/đánh dấu/bỏ dấu ẨN; tách vẫn có", () => {
    const qt = quyTacKetLuan(DA_KY, ketLuan());
    expect(qt.sua.hien || qt.go.hien || qt.danhDau.hien || qt.boDau.hien).toBe(false);
    expect(qt.tach.hien).toBe(true);
    expect(quyTacKetLuan(DA_KY, ketLuan({ no_task: true })).boDau.hien).toBe(false);
  });

  it("mã trạng thái LẠ: không mở nút sửa/gỡ nào, cũng không mở nút ghi Thông báo nào", () => {
    const la = bienBan({ status: "ma-la" });
    const qt = quyTacBienBan(la, true);
    expect(qt.sua.hien || qt.xoa.hien || qt.ky.hien || qt.themKetLuan.hien).toBe(false);
    expect(qt.ghiThongBao.hien || qt.boSung.hien).toBe(false);
    expect(quyTacKetLuan(la, ketLuan()).sua.hien).toBe(false);
  });

  it("ký: chỉ bản nháp VÀ chỉ khi có quyền — ca thiếu quyền ẩn nút", () => {
    expect(quyTacBienBan(NHAP, false).ky.hien).toBe(false);
    expect(quyTacBienBan(NHAP, true).ky.hien).toBe(true);
    expect(quyTacBienBan(DA_KY, true).ky.hien).toBe(false);
  });

  it("gỡ biên bản: TẮT kèm lý do khi còn nhiệm vụ trỏ về", () => {
    expect(quyTacBienBan(bienBan({ task_count: 2 }), false).xoa).toEqual({
      hien: true,
      viSaoTat: VI_SAO_BIEN_BAN_CON_NHIEM_VU,
    });
  });

  it("ghi Thông báo: chỉ đã ký VÀ chưa có Thông báo; bổ sung: chỉ đã ký", () => {
    expect(quyTacBienBan(DA_KY, false).ghiThongBao.hien).toBe(true);
    expect(
      quyTacBienBan(
        bienBan({ status: "da-ky", notice: { reference_no: "12/TB", issued_on: "2026-08-07" } }),
        false,
      ).ghiThongBao.hien,
    ).toBe(false);
    expect(quyTacBienBan(DA_KY, false).boSung.hien).toBe(true);
    expect(quyTacBienBan(NHAP, false).boSung.hien).toBe(false);
    expect(quyTacBienBan(NHAP, false).ghiThongBao.hien).toBe(false);
  });
});

const DANH_BA = danhBaTheoMa([
  { code: "CB-2026-7K3M9Q", full_name: "Nguyễn Văn An", position: "Chủ tịch UBND", department_id: "" },
  { code: "CB-2026-1A2B3C", full_name: "Trần Thị Bình", position: "", department_id: "" },
]);

describe("biểu mẫu → thân yêu cầu", () => {
  const GT = {
    ...BIEU_MAU_TRONG,
    ten: "  Giao ban tháng 8  ",
    ngay: "2026-08-05",
    chuTri: "CB-2026-7K3M9Q",
    thuKy: "CB-2026-1A2B3C",
    thanhPhanCanBo: ["CB-2026-7K3M9Q"],
    thanhPhanKhac: "Đại diện thôn Hà Lam\n\nCB-2026-7K3M9Q\n",
    ketLuan: "Kết luận một\nKết luận hai",
  };

  it("tạo: gửi MÃ cán bộ cho chủ trì/thư ký, thành phần = đã chọn + chữ tự do, bỏ trùng", () => {
    const than = thanTaoTuBieuMau(GT, null);
    expect(than.title).toBe("Giao ban tháng 8");
    expect(than.held_on).toBe("2026-08-05");
    expect(than.chaired_by).toBe("CB-2026-7K3M9Q");
    expect(than.minutes_taker).toBe("CB-2026-1A2B3C");
    expect(than.attendees).toEqual(["CB-2026-7K3M9Q", "Đại diện thôn Hà Lam"]);
    expect(than.conclusions).toEqual(["Kết luận một", "Kết luận hai"]);
    expect(than.supplements_id).toBeUndefined();
  });

  it("tạo bổ sung: mang `supplements_id` của biên bản gốc", () => {
    expect(thanTaoTuBieuMau(GT, "01JBBGOC").supplements_id).toBe("01JBBGOC");
  });

  it("tạo: trường rỗng VẮNG, trừ `chaired_by` (hợp đồng khai string thường)", () => {
    const than = thanTaoTuBieuMau({ ...BIEU_MAU_TRONG, ten: "x", ngay: "2026-08-05" }, null);
    expect(JSON.parse(JSON.stringify(than))).toEqual({
      title: "x",
      held_on: "2026-08-05",
      chaired_by: "",
    });
  });

  it("sửa: chỉ trường ĐÃ ĐỔI đi lên; không đổi gì thì `null`", () => {
    const ban = bienBan({
      chaired_by: "CB-2026-7K3M9Q",
      minutes_taker: "CB-2026-1A2B3C",
      attendees: ["CB-2026-7K3M9Q", "Đại diện thôn Hà Lam"],
      content: "Toàn văn.",
    });
    const gt = giaTriTuBienBan(ban, DANH_BA);
    expect(thanSuaTuBieuMau(gt, ban)).toBeNull();
    expect(thanSuaTuBieuMau({ ...gt, diaDiem: "" }, ban)).toEqual({ location: "" });
    expect(thanSuaTuBieuMau({ ...gt, thuKy: "" }, ban)).toEqual({ minutes_taker: "" });
  });

  it("sửa: thành phần tách hai ngăn theo danh bạ; danh bạ chưa tải thì gửi lại NGUYÊN chuỗi", () => {
    const ban = bienBan({ attendees: ["CB-2026-7K3M9Q", "Đại diện thôn Hà Lam"] });
    const coDanhBa = giaTriTuBienBan(ban, DANH_BA);
    expect(coDanhBa.thanhPhanCanBo).toEqual(["CB-2026-7K3M9Q"]);
    expect(coDanhBa.thanhPhanKhac).toBe("Đại diện thôn Hà Lam");
    const khongDanhBa = giaTriTuBienBan(ban, null);
    expect(khongDanhBa.thanhPhanCanBo).toEqual([]);
    expect(thanSuaTuBieuMau(khongDanhBa, ban)).toBeNull();
  });

  it("Thông báo: hai ô trống là KHÔNG ghi; có một nửa thì gửi cả cặp để máy chủ nói nửa nào thiếu", () => {
    expect(thanThongBao("", "")).toBeNull();
    expect(thanThongBao(" 12/TB-UBND ", "2026-08-07")).toEqual({
      reference_no: "12/TB-UBND",
      issued_on: "2026-08-07",
    });
    expect(thanThongBao("12/TB-UBND", "")).toEqual({ reference_no: "12/TB-UBND", issued_on: "" });
  });
});

describe("đọc ra mã cán bộ, Thông báo, neo", () => {
  it("mã cán bộ → họ tên · chức vụ; không trong danh bạ → mã kèm câu; rỗng → Không ghi", () => {
    expect(nhanCanBo("CB-2026-7K3M9Q", DANH_BA)).toBe("Nguyễn Văn An · Chủ tịch UBND");
    expect(nhanCanBo("CB-2026-1A2B3C", DANH_BA)).toBe("Trần Thị Bình");
    expect(nhanCanBo("CB-X", DANH_BA)).toBe("CB-X (không có trong danh bạ cán bộ đang hoạt động)");
    expect(nhanCanBo("CB-X", null)).toBe("CB-X");
    expect(nhanCanBo("", DANH_BA)).toBe("Không ghi");
    expect(nhanCanBo(undefined, DANH_BA)).toBe("Không ghi");
  });

  it("ô chọn giữ giá trị đang lưu dù người ấy đã rời danh bạ — không xoá chủ trì lặng lẽ", () => {
    const ds = [{ code: "CB-A", full_name: "A", position: "", department_id: "" }];
    expect(luaChonCanBo(ds, "CB-DA-NGHI").map((l) => l.ma)).toEqual(["CB-DA-NGHI", "CB-A"]);
    expect(luaChonCanBo(ds, "CB-A").map((l) => l.ma)).toEqual(["CB-A"]);
  });

  it("Thông báo: ngày là NGÀY LỊCH", () => {
    expect(nhanThongBao({ reference_no: "12/TB-UBND", issued_on: "2026-08-07" })).toBe(
      "Số 12/TB-UBND, ngày 7/8/2026",
    );
    expect(nhanThongBao(undefined)).toBe("Chưa ghi");
  });

  it("neo: chỉ nhận `#bien-ban-<chữ và số>`; đường dẫn từ màn khác đi sau dấu `#`", () => {
    expect(idTuNeo("#bien-ban-01JBB1")).toBe("01JBB1");
    expect(idTuNeo("#bien-ban-")).toBeNull();
    expect(idTuNeo("#bien-ban-01J/../x")).toBeNull();
    expect(idTuNeo("")).toBeNull();
    expect(duongDanBienBan("01JBB1")).toBe("/nhiem-vu/bien-ban#bien-ban-01JBB1");
  });
});
