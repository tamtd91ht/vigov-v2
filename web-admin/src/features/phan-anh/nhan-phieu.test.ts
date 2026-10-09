import { readFileSync } from "node:fs";
import { fileURLToPath } from "node:url";

import { describe, expect, it } from "vitest";

import type { petitions_phieuPhanAnhRa } from "@/lib/api/schema.gen";

import {
  buocLuongChinh,
  buocReNhanh,
  CO_QUAN_TOI_DA,
  coCongDanXacNhan,
  demKyTu,
  dongDuocTrenManHinh,
  GHI_CHU_TOI_DA,
  laDongPhanCong,
  loiCoQuan,
  loiGhiChuNhatKy,
  loiGhiChuNoiBo,
  loiLyDo,
  NHAN_THAO_TAC_NHAT_KY,
  nhanThaoTacNhatKy,
  type MaThaoTacNhatKy,
  LY_DO_TOI_DA,
  LY_DO_TOI_THIEU,
  reNhanhDuoc,
  cauGiaiThichTrangThai,
  conBuocKeTiep,
  danhBaTheoMa,
  LINH_VUC_PHAN_ANH,
  linhVucPhanAnh,
  lopHan,
  LUONG_CHINH,
  MOI_KENH,
  MOI_TRANG_THAI,
  nhanBoPhan,
  nhanCanBoXuLy,
  nhanHan,
  nhanKenh,
  nhanLinhVuc,
  nhanLuaChonCanBo,
  nhanNguoiGui,
  isContactUnverified,
  nhanThoiDiem,
  nhanTrangThai,
  PHAN_CHUA_DUNG,
  petitionPendingPart,
  phanLoaiDuoc,
  initialClassifyField,
  RE_NHANH,
  trangThaiHan,
  CITIZEN_LOG_ACTOR,
  congThaoTac,
  LOW_RATING_MAX,
  LOW_RATING_REOPENED,
  logActorLabel,
  NEVER_PUBLIC_HINT,
  publicationView,
  ratingStars,
  ratingView,
  reopenLine,
  SCENE_LOCATION_LABEL,
  LOCATION_SECTION_TITLE,
  sceneCoordinates,
  statusStrip,
  cardSenderLabel,
  dateTimeLabel,
  petitionDeepLinkCode,
  afterPhotoType,
  afterPhotoUploadOpen,
  clockFromBounds,
  clockFromRfc3339,
  INTAKE_DESCRIPTION,
  intakeContentError,
  onTimePercent,
  PETITION_INTAKE_PERMISSION,
  ratingAverage,
  toLocalInputValue,
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

  it("contact_unverified: only an explicit true counts (ADR 0080 #5), never has_citizen/channel", () => {
    expect(isContactUnverified(phieu({ contact_unverified: true }))).toBe(true);
    expect(isContactUnverified(phieu({ contact_unverified: false }))).toBe(false);
    expect(isContactUnverified(phieu({ contact_unverified: null }))).toBe(false);
    expect(isContactUnverified(phieu({ contact_unverified: undefined }))).toBe(false);
    expect(isContactUnverified(phieu({ channel: "zalo-mini-app", has_citizen: false }))).toBe(false);
  });

  it("không ẩn danh mà cả hai trường rỗng: nói ra (the prototype's words), không để ô trống", () => {
    expect(nhanNguoiGui(phieu({ reporter_name: "", reporter_phone: "" }))).toBe("Không rõ người gửi");
    expect(cardSenderLabel(phieu({ reporter_name: "", reporter_phone: "" }))).toBe("Không rõ người gửi");
  });

  it("the card says “Gửi ẩn danh”, the drawer “Người gửi ẩn danh” — and neither adds anything", () => {
    const an = phieu({ anonymous: true, reporter_name: "", reporter_phone: "" });
    expect(cardSenderLabel(an)).toBe("Gửi ẩn danh");
    expect(nhanNguoiGui(an)).toBe("Người gửi ẩn danh");
    expect(cardSenderLabel(phieu())).toBe(nhanNguoiGui(phieu()));
  });

  it("`?id=` deep link: a single plain token only; anything else is ignored, never sent", () => {
    expect(petitionDeepLinkCode("PA-2026-0021")).toBe("PA-2026-0021");
    expect(petitionDeepLinkCode("  PA-1 ")).toBe("PA-1");
    for (const bad of [undefined, "", "../x", "PA 1", "a".repeat(65), ["PA-1", "PA-2"]]) {
      expect(petitionDeepLinkCode(bad as string | readonly string[] | undefined)).toBeNull();
    }
  });

  it("drawer header time: date first, Vietnam time", () => {
    expect(dateTimeLabel("2026-09-09T07:21:00Z")).toBe("09/09/2026 14:21");
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

describe("hiển thị với người dân — `publication_status` (§8.3, ADR 0050 điểm 8)", () => {
  it("three stored values, three labels — the requirement's wording, and the spec's reassurance kept", () => {
    const cho = publicationView(phieu({ publication_status: "cho-duyet" }));
    const cong = publicationView(phieu({ publication_status: "cong-khai", public: true }));
    const an = publicationView(phieu({ publication_status: "an" }));
    expect([cho.label, cong.label, an.label]).toEqual([
      "Chưa cho hiện công khai",
      "Đang hiện công khai",
      "Không cho hiện công khai",
    ]);
    expect(cho.hint).toBe("Chỉ cán bộ trong xã xem được. Người gửi vẫn tra cứu được phiếu của mình.");
  });

  it("buttons: publish unless public, hide unless hidden", () => {
    const cho = publicationView(phieu({ publication_status: "cho-duyet" }));
    expect([cho.canPublish, cho.canHide]).toEqual([true, true]);
    const cong = publicationView(phieu({ publication_status: "cong-khai", public: true }));
    expect([cong.canPublish, cong.canHide]).toEqual([false, true]);
    const an = publicationView(phieu({ publication_status: "an" }));
    expect([an.canPublish, an.canHide]).toEqual([true, false]);
  });

  it("`can-bo` (staff conduct): NEVER a publish button, and the hint is the server's sentence", () => {
    for (const status of ["cho-duyet", "an"]) {
      const v = publicationView(phieu({ field: "can-bo", publication_status: status }));
      expect(v.canPublish, status).toBe(false);
      expect(v.hint, status).toBe(NEVER_PUBLIC_HINT);
    }
    // Somehow not hidden (an old row): hiding it stays possible.
    expect(publicationView(phieu({ field: "can-bo", publication_status: "cho-duyet" })).canHide).toBe(
      true,
    );
  });

  it("field ABSENT (older server): derived from `public`, never a fourth state", () => {
    expect(publicationView(phieu({ public: true })).status).toBe("cong-khai");
    expect(publicationView(phieu({ public: false })).status).toBe("cho-duyet");
    expect(publicationView(phieu({ public: false, publication_status: "" })).status).toBe("cho-duyet");
  });

  it("unknown value: raw code said out loud, NO button — never a guess at what a click does", () => {
    const v = publicationView(phieu({ publication_status: "tam-an" }));
    expect(v.label).toContain("tam-an");
    expect(v.label).toContain("chưa có nhãn");
    expect([v.canPublish, v.canHide]).toEqual([false, false]);
  });

  it("the gate is `feedback.assign` — the same argument as `Chuyển xử lý`, nothing else", () => {
    expect(congThaoTac(false, true, false).moderate).toBe(true);
    expect(congThaoTac(true, false, true).moderate).toBe(false);
  });
});

describe("đánh giá của người dân — rating, rating_comment, rated_at, reopen_count", () => {
  it("no rating: `none`, whatever reopen_count says", () => {
    expect(ratingView(phieu()).kind).toBe("none");
    expect(ratingView(phieu({ rating: null, reopen_count: 2 })).kind).toBe("none");
  });

  it("rated 4: stars, score, Vietnam time, comment as sent — not low", () => {
    const v = ratingView(
      phieu({
        rating: 4,
        rated_at: "2026-09-28T03:05:00Z",
        rating_comment: "Đã dọn sạch.",
        reopen_count: 0,
      }),
    );
    expect(v).toEqual({
      kind: "rated",
      stars: "★★★★☆",
      score: "4/5",
      at: "10:05 28/09/2026",
      comment: "Đã dọn sạch.",
      low: false,
      reopened: false,
    });
  });

  it("rated ≤2 AND reopened at least once: the requirement's red sentence applies", () => {
    for (const rating of [1, 2]) {
      const v = ratingView(phieu({ rating, reopen_count: 1 }));
      expect(v.kind === "rated" && v.low && v.reopened, String(rating)).toBe(true);
    }
    expect(LOW_RATING_REOPENED).toBe("Đánh giá thấp — phiếu đã tự mở lại để xử lý tiếp.");
    expect(LOW_RATING_MAX).toBe(2);
  });

  it("rated ≤2 but reopen_count 0 or absent: NOT claimed as reopened", () => {
    for (const reopen_count of [0, null, undefined]) {
      const v = ratingView(phieu({ rating: 2, reopen_count }));
      expect(v.kind === "rated" && v.reopened, String(reopen_count)).toBe(false);
    }
  });

  it("3 stars is not low, even with an earlier reopening on record", () => {
    const v = ratingView(phieu({ rating: 3, reopen_count: 1 }));
    expect(v.kind === "rated" && (v.low || v.reopened)).toBe(false);
  });

  it("missing time or comment: `null` / empty, never 'Invalid Date' or 'undefined'", () => {
    const v = ratingView(phieu({ rating: 5 }));
    expect(v.kind === "rated" && v.at).toBeNull();
    expect(v.kind === "rated" && v.comment).toBe("");
  });

  it("stars clamp to 0..5", () => {
    expect(ratingStars(0)).toBe("☆☆☆☆☆");
    expect(ratingStars(5)).toBe("★★★★★");
    expect(ratingStars(9)).toBe("★★★★★");
  });

  it("reopen line: only when the server counts at least one reopening", () => {
    expect(reopenLine(2)).toBe("Đã mở lại 2 lần do người dân chấm điểm thấp");
    for (const n of [0, null, undefined]) expect(reopenLine(n), String(n)).toBeNull();
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

  it("all nine statuses carry the prototype's sentence, VERBATIM (ADR 0027 Bổ sung 2026-10-02)", () => {
    // Typed again from `../vigov-require` `apps/admin/src/lib/feedback-display.ts:136-146` — NOT read
    // from the table under test. Eight equal the citizen app's `TRANG_THAI`; `da-dong` is the staff's
    // full sentence (row 5: the citizen reads only "Phiếu đã đóng.").
    expect(MOI_TRANG_THAI.map((ma) => [ma, cauGiaiThichTrangThai(ma)])).toEqual([
      ["da-tiep-nhan", "Phiếu vừa vào sổ, chưa phân cho ai."],
      ["dang-phan-loai", "Đang xem phiếu thuộc lĩnh vực nào, có tiếp nhận không."],
      ["da-chuyen-xu-ly", "Đã giao cho bộ phận, chưa bắt tay làm."],
      ["dang-xu-ly", "Bộ phận đang xử lý tại hiện trường."],
      ["da-xu-ly", "Đã làm xong, chờ báo lại cho người dân."],
      ["cho-dan-xac-nhan", "Đã báo người dân, chờ họ xác nhận và chấm điểm."],
      ["da-dong", "Phiếu đã đóng. Phải có ảnh sau xử lý mới đóng được."],
      ["khong-tiep-nhan", "Không thuộc thẩm quyền hoặc không đủ căn cứ. Đã ghi lý do."],
      ["chuyen-cap-tren", "Vượt thẩm quyền của xã, đã chuyển lên cấp trên."],
    ]);
  });

  it("the eight shared sentences equal the citizen app's table, character for character", () => {
    const citizen = readFileSync(
      fileURLToPath(new URL("../../../../citizen-app/src/cong-dan/man/noi-dung.ts", import.meta.url)),
      "utf8",
    );
    for (const ma of MOI_TRANG_THAI.filter((m) => m !== "da-dong")) {
      expect(citizen, ma).toContain(`"${ma}": { giai_thich: "${cauGiaiThichTrangThai(ma)}" }`);
    }
  });

  it("an unknown code has NO sentence — never one the web made up", () => {
    expect(cauGiaiThichTrangThai("mot-ma-moi")).toBeNull();
    expect(cauGiaiThichTrangThai("constructor")).toBeNull();
    expect(cauGiaiThichTrangThai("")).toBeNull();
  });
});

describe("khi nào vẽ nút nào", () => {
  it("phân loại chỉ có nghĩa ở `da-tiep-nhan` — lần thứ hai máy chủ trả 409", () => {
    expect(phanLoaiDuoc("da-tiep-nhan")).toBe(true);
    for (const ma of MOI_TRANG_THAI.filter((m) => m !== "da-tiep-nhan")) {
      expect(phanLoaiDuoc(ma)).toBe(false);
    }
  });

  it("ô phân loại mở sẵn lĩnh vực đã chọn lúc nhập hộ (PA-03); mã ngoài danh mục thì để trống", () => {
    expect(initialClassifyField("giao-thong")).toBe("giao-thong");
    expect(initialClassifyField("ma-khong-co")).toBe("");
    expect(initialClassifyField("")).toBe("");
    expect(initialClassifyField(undefined)).toBe("");
    expect(initialClassifyField(null)).toBe("");
  });

  it("bước cuối luồng chính và hai rẽ nhánh thì không còn bước kế tiếp", () => {
    expect(conBuocKeTiep("dang-xu-ly")).toBe(true);
    expect(conBuocKeTiep("da-dong")).toBe(false);
    for (const ma of RE_NHANH) expect(conBuocKeTiep(ma)).toBe(false);
  });
});

describe("bộ phận (ULID) và cán bộ (MÃ CÁN BỘ) đang giữ phiếu — hai cách tra khác nhau", () => {
  it("id bộ phận không tra được thì nói ra, KHÔNG in id lên màn hình cán bộ", () => {
    const ten = new Map([["01JBOPHAN", "VĂN PHÒNG ĐẢNG ỦY"]]);
    expect(nhanBoPhan("01JBOPHAN", ten)).toBe("VĂN PHÒNG ĐẢNG ỦY");
    expect(nhanBoPhan("", ten)).toBe("Chưa chuyển bộ phận");
    expect(nhanBoPhan("01JKHAC", ten)).not.toContain("01JKHAC");
  });

  const DANH_BA = danhBaTheoMa([
    { code: "CB-00123", full_name: "Trần Thị B", position: "Công chức Văn phòng", department_id: "01JBOPHAN", email_masked: null },
    { code: "CB-00124", full_name: "", position: "", department_id: "", email_masked: null },
  ]);

  it("chưa phân công cán bộ cụ thể: câu nói ra điều đó, bất kể danh bạ", () => {
    expect(nhanCanBoXuLy("", DANH_BA)).toBe("Chưa phân công cán bộ cụ thể");
    expect(nhanCanBoXuLy("", null)).toBe("Chưa phân công cán bộ cụ thể");
  });

  it("có trong danh bạ: hiện HỌ TÊN thay cho mã", () => {
    expect(nhanCanBoXuLy("CB-00123", DANH_BA)).toBe("Trần Thị B");
  });

  it("không có trong danh bạ (ví dụ tài khoản đã khoá): hiện MÃ kèm câu trung tính", () => {
    const nhan = nhanCanBoXuLy("CB-00999", DANH_BA);
    expect(nhan).toContain("CB-00999");
    expect(nhan).toContain("không có trong danh bạ");
    expect(nhan).not.toContain("undefined");
  });

  it("danh bạ chưa tải hoặc tải hỏng: hiện đúng mã, không bao giờ `undefined`", () => {
    expect(nhanCanBoXuLy("CB-00123", null)).toBe("CB-00123");
  });

  it("họ tên rỗng trong danh bạ: hiện mã, không một ô trống", () => {
    expect(nhanCanBoXuLy("CB-00124", DANH_BA)).toBe("CB-00124");
  });

  it("dòng ô chọn: `Họ tên · Chức danh`, bỏ chức danh khi rỗng", () => {
    expect(
      nhanLuaChonCanBo({ code: "CB-00123", full_name: "Trần Thị B", position: "Công chức", department_id: "", email_masked: null }),
    ).toBe("Trần Thị B · Công chức");
    expect(
      nhanLuaChonCanBo({ code: "CB-00123", full_name: "Trần Thị B", position: "", department_id: "", email_masked: null }),
    ).toBe("Trần Thị B");
  });
});

describe("phần chưa dựng được — không còn liệt kê những gì ĐÃ dựng", () => {
  it("ô chọn cán bộ, họ tên người đang giữ phiếu và tab `Giao cho tôi` đã rời danh sách", () => {
    const tatCa = PHAN_CHUA_DUNG.map((p) => `${p.ten}\n${p.viSao}`).join("\n");
    expect(tatCa).not.toContain("Giao cho tôi` / `Liên quan");
    expect(tatCa).not.toMatch(/Họ tên — email của cán bộ đang giữ/);
    expect(tatCa).not.toMatch(/ô chọn `Cán bộ xử lý` của §8\.5 cũng không dựng/);
    // Niềm tin sai cũ — "`assignee` là ULID nội bộ" — không còn ra tới màn hình.
    expect(tatCa).not.toMatch(/ULID nội bộ/i);
    expect(tatCa).not.toContain("admin.user");
  });

  it("`Liên quan đến tôi` VẪN nằm trong danh sách — máy chủ trả 400 cho `scope=related`", () => {
    expect(PHAN_CHUA_DUNG.some((p) => p.ten.includes("Liên quan đến tôi"))).toBe(true);
    expect(PHAN_CHUA_DUNG.some((p) => p.ten.includes("Giao cho tôi"))).toBe(false);
  });
});

describe("hai nhánh rẽ — điểm rời, giới hạn, ô trên thanh bước", () => {
  it("chỉ rời được từ `dang-phan-loai`", () => {
    for (const ma of MOI_TRANG_THAI) {
      expect(reNhanhDuoc(ma), ma).toBe(ma === "dang-phan-loai");
    }
  });

  it("hai ô rẽ nhánh sáng ĐÚNG ô của trạng thái, và không ô nào khi phiếu trên luồng chính", () => {
    expect(buocReNhanh("khong-tiep-nhan").map((o) => o.vaiTro)).toEqual(["dangODay", "chuaToi"]);
    expect(buocReNhanh("chuyen-cap-tren").map((o) => o.vaiTro)).toEqual(["chuaToi", "dangODay"]);
    expect(buocReNhanh("dang-phan-loai").every((o) => o.vaiTro === "chuaToi")).toBe(true);
    expect(buocReNhanh("khong-tiep-nhan").map((o) => o.nhan)).toEqual([
      "Không tiếp nhận",
      "Chuyển cấp trên",
    ]);
  });

  it("giới hạn đúng số của máy chủ", () => {
    expect([LY_DO_TOI_THIEU, LY_DO_TOI_DA, CO_QUAN_TOI_DA]).toEqual([10, 2000, 200]);
  });

  it("đếm KÝ TỰ chữ Việt như rune của Go, trên chuỗi đã cắt khoảng trắng", () => {
    expect(demKyTu("  Đường ngập  ")).toBe(10);
    // Một biểu tượng ngoài mặt phẳng cơ bản là HAI đơn vị UTF-16 nhưng MỘT rune.
    expect("😀".length).toBe(2);
    expect(demKyTu("😀")).toBe(1);
  });

  it("lý do: 9 ký tự bị từ chối, 10 được; 2000 được, 2001 bị từ chối", () => {
    expect(loiLyDo("Ngập ước!")).not.toBeNull(); // 9
    expect(loiLyDo("Ngập nước!")).toBeNull(); // 10
    expect(loiLyDo("   Ngập ước!   ")).not.toBeNull(); // khoảng trắng không được đếm
    expect(loiLyDo("ữ".repeat(2000))).toBeNull();
    expect(loiLyDo("ữ".repeat(2001))).not.toBeNull();
  });

  it("cơ quan tiếp nhận: bắt buộc (rỗng hay toàn khoảng trắng bị từ chối), tối đa 200", () => {
    expect(loiCoQuan("")).not.toBeNull();
    expect(loiCoQuan("   ")).not.toBeNull();
    expect(loiCoQuan("Công an xã")).toBeNull();
    expect(loiCoQuan("ở".repeat(200))).toBeNull();
    expect(loiCoQuan("ở".repeat(201))).not.toBeNull();
  });
});

describe("Đóng phiếu — hai điểm đóng, điểm `da-xu-ly` suy từ kênh nhập hộ", () => {
  function p(status: string, channel: string): petitions_phieuPhanAnhRa {
    return { status, channel } as petitions_phieuPhanAnhRa;
  }

  it("`cho-dan-xac-nhan` đóng được ở mọi kênh", () => {
    for (const k of MOI_KENH) expect(dongDuocTrenManHinh(p("cho-dan-xac-nhan", k)), k).toBe(true);
  });

  it("`da-xu-ly` chỉ đóng được khi kênh là `can-bo-nhap-ho`", () => {
    for (const k of MOI_KENH) {
      expect(dongDuocTrenManHinh(p("da-xu-ly", k)), k).toBe(k === "can-bo-nhap-ho");
    }
  });

  it("mọi trạng thái khác: không", () => {
    for (const ma of MOI_TRANG_THAI.filter((m) => m !== "cho-dan-xac-nhan" && m !== "da-xu-ly")) {
      expect(dongDuocTrenManHinh(p(ma, "can-bo-nhap-ho")), ma).toBe(false);
    }
  });
});

describe("phần chưa dựng được — hai ô rẽ nhánh đã rời danh sách", () => {
  it("không còn mục nào về hai nhánh rẽ", () => {
    const tatCa = PHAN_CHUA_DUNG.map((x) => `${x.ten} ${x.viSao}`).join(" ");
    expect(tatCa).not.toMatch(/rẽ nhánh|Không tiếp nhận|Chuyển cấp trên/);
  });
});

describe("`has_citizen` QUYẾT ĐỊNH khi có mặt; vắng thì quay về luật kênh", () => {
  function p(status: string, channel: string, has_citizen?: boolean | null): petitions_phieuPhanAnhRa {
    return { status, channel, has_citizen } as petitions_phieuPhanAnhRa;
  }

  it("cờ `false` thắng kênh công dân: `da-xu-ly` đóng được", () => {
    // Phiếu `zalo-mini-app` mà không có tài khoản — máy chủ cho đóng, luật kênh cũ thì không.
    for (const k of MOI_KENH) {
      expect(coCongDanXacNhan(p("da-xu-ly", k, false)), k).toBe(false);
      expect(dongDuocTrenManHinh(p("da-xu-ly", k, false)), k).toBe(true);
    }
  });

  it("cờ `true` thắng kênh nhập hộ: `da-xu-ly` KHÔNG đóng được — phải chờ dân xác nhận", () => {
    for (const k of MOI_KENH) {
      expect(coCongDanXacNhan(p("da-xu-ly", k, true)), k).toBe(true);
      expect(dongDuocTrenManHinh(p("da-xu-ly", k, true)), k).toBe(false);
    }
  });

  it("cờ vắng (`undefined` hoặc `null`): luật kênh `can-bo-nhap-ho` như trước", () => {
    for (const co of [undefined, null]) {
      for (const k of MOI_KENH) {
        expect(dongDuocTrenManHinh(p("da-xu-ly", k, co)), `${k}/${co}`).toBe(k === "can-bo-nhap-ho");
      }
    }
  });

  it("cờ không mở điểm đóng nào khác: trạng thái khác vẫn không đóng được", () => {
    for (const ma of MOI_TRANG_THAI.filter((m) => m !== "cho-dan-xac-nhan" && m !== "da-xu-ly")) {
      expect(dongDuocTrenManHinh(p(ma, "zalo-mini-app", false)), ma).toBe(false);
    }
  });
});

describe("nhật ký xử lý — nhãn mười mã thao tác", () => {
  /** Danh sách đóng của máy chủ, gõ lại từ hợp đồng — KHÔNG sinh từ bảng nhãn đang kiểm. */
  const LOG_ACTIONS = [
    "phan-loai",
    "phan-cong",
    "chuyen-trang-thai",
    "dong-phieu",
    "khong-tiep-nhan",
    "chuyen-cap-tren",
    "ghi-chu",
    // `domain.LogActionCitizenRating` / `LogActionReopenByRating` (ADR 0050 point 2).
    "danh-gia",
    "mo-lai-theo-danh-gia",
    // `domain.LogActionTaskCreated` — a task booked from the petition (POST …/tasks).
    "tao-nhiem-vu",
    // `domain.LogActionStaffIntake` — the staff intake (ADR 0028 Bổ sung 2026-10-02 row 6).
    "nhap-ho",
  ] as const satisfies readonly MaThaoTacNhatKy[];

  // Mức KIỂU: hợp mọc thêm một mã mà danh sách trên không có → `tsc` đỏ tại đây.
  type DuMa = Exclude<MaThaoTacNhatKy, (typeof LOG_ACTIONS)[number]> extends never ? true : never;
  const _duMa: DuMa = true;
  void _duMa;

  it("bảng nhãn có ĐÚNG mười một khoá, không hơn", () => {
    expect(Object.keys(NHAN_THAO_TAC_NHAT_KY).sort()).toEqual([...LOG_ACTIONS].sort());
  });

  it("nhãn nguyên văn chuyên gia nghiệp vụ chốt", () => {
    expect(LOG_ACTIONS.map(nhanThaoTacNhatKy)).toEqual([
      "Phân loại",
      "Chuyển xử lý",
      "Chuyển trạng thái",
      "Đóng phiếu",
      "Không tiếp nhận",
      "Chuyển cấp trên",
      "Ghi chú",
      "Người dân đánh giá",
      "Mở lại do đánh giá thấp",
      "Tạo nhiệm vụ",
      "Nhập hộ phản ánh",
    ]);
  });

  it("actor `cong-dan` reads “Người dân”; staff codes as is without a directory; empty is a dash", () => {
    expect(CITIZEN_LOG_ACTOR).toBe("cong-dan");
    expect(logActorLabel("cong-dan", null)).toBe("Người dân");
    expect(logActorLabel("CB-00123", null)).toBe("CB-00123");
    expect(logActorLabel("", null)).toBe("—");
  });

  // PA-06: the log names the officer, and keeps the code (rule 6, invariant 8).
  const LOG_DIRECTORY = danhBaTheoMa([
    { code: "CB-00123", full_name: "Trần Thị B", position: "Công chức", department_id: "", email_masked: null },
    { code: "CB-00124", full_name: "", position: "", department_id: "", email_masked: null },
    // A directory row whose code collides with the citizen marker must never be looked up.
    { code: "cong-dan", full_name: "Không được hiện", position: "", department_id: "", email_masked: null },
  ]);

  it("staff code known to the directory reads `Họ tên (CB-…)`", () => {
    expect(logActorLabel("CB-00123", LOG_DIRECTORY)).toBe("Trần Thị B (CB-00123)");
  });

  it("falls back to the bare code: not in the directory (inactive account), or no name on file", () => {
    expect(logActorLabel("CB-00999", LOG_DIRECTORY)).toBe("CB-00999");
    expect(logActorLabel("CB-00124", LOG_DIRECTORY)).toBe("CB-00124");
  });

  it("the citizen marker is never looked up in the directory", () => {
    expect(logActorLabel("cong-dan", LOG_DIRECTORY)).toBe("Người dân");
  });

  it("mã lạ: hiện nguyên mã và NÓI RA là chưa có nhãn, không đoán", () => {
    expect(nhanThaoTacNhatKy("mo-lai")).toBe("mo-lai (mã thao tác chưa có nhãn trên màn hình này)");
  });

  it("chỉ dòng `phan-cong` mang bộ phận / phụ trách", () => {
    for (const ma of LOG_ACTIONS) expect(laDongPhanCong(ma), ma).toBe(ma === "phan-cong");
  });
});

describe("giới hạn ghi chú — 2000 ký tự của máy chủ", () => {
  it("ô nhật ký: BẮT BUỘC có chữ; 2000 được, 2001 không", () => {
    expect(GHI_CHU_TOI_DA).toBe(2000);
    expect(loiGhiChuNhatKy("")).not.toBeNull();
    expect(loiGhiChuNhatKy("  \n ")).not.toBeNull();
    expect(loiGhiChuNhatKy("ệ".repeat(2000))).toBeNull();
    expect(loiGhiChuNhatKy("ệ".repeat(2001))).not.toBeNull();
  });

  it("ghi chú nội bộ của sáu thao tác: TRỐNG là hợp lệ; 2001 không", () => {
    expect(loiGhiChuNoiBo("")).toBeNull();
    expect(loiGhiChuNoiBo("ệ".repeat(2000))).toBeNull();
    expect(loiGhiChuNoiBo("ệ".repeat(2001))).not.toBeNull();
  });
});

describe("phần chưa dựng được — nhật ký xử lý đã rời danh sách", () => {
  it("không còn mục nào về nhật ký, còn mọi mục khác vẫn nguyên", () => {
    const tatCa = PHAN_CHUA_DUNG.map((x) => `${x.ten} ${x.viSao}`).join(" ");
    expect(tatCa).not.toMatch(/Nhật ký xử lý|nhat_ky_phan_anh/);
    // Chín mục (02/10/2026, ADR 0068 §14): bảy mục cũ, trong đó hai mục tách đôi để mỗi dấu "?" mở
    // đúng một mô tả (tab Bản đồ nhiệt / tab Báo cáo; ô thôn / nút đính ảnh của modal nhập hộ). Bỏ
    // nhầm một mục khác cùng lúc là đỏ ở đây. Mười từ 06/10/2026: khối `Có thể trùng với phiếu khác`
    // của prototype (ADR 0068 lần 5) có chỗ giữ riêng.
    // Thirteen from 09/10/2026 (prototype round, owner decisions D1–D5): the composer's attachment, the
    // log row's `Nội bộ` pill and the log file's remove button have no route behind them.
    expect(PHAN_CHUA_DUNG.length).toBe(13);
    // Both photo halves are built now (ADR 0047: the "after" row replaces G8).
    expect(PHAN_CHUA_DUNG.some((p) => p.ten.startsWith("Ảnh sau khi xử lý"))).toBe(false);
    expect(PHAN_CHUA_DUNG.some((p) => p.ten.includes("Ảnh trước"))).toBe(false);
  });
});

describe("phần chưa dựng được — đánh giá và kiểm duyệt công khai đã rời danh sách", () => {
  const tatCa = () => PHAN_CHUA_DUNG.map((x) => `${x.ten} ${x.viSao}`).join(" ");

  it("no entry claims the low-rated filter or the public toggle is missing", () => {
    expect(tatCa()).not.toContain("Lọc `Bị đánh giá thấp`");
    expect(tatCa()).not.toContain("diem_hai_long");
    expect(tatCa()).not.toContain("Cho hiện công khai");
    expect(tatCa()).not.toMatch(/không có tuyến ghi nó/);
  });

  it("§8.6 stays listed, with the reason — a decision, not a missing route", () => {
    const muc = PHAN_CHUA_DUNG.find((p) => p.ten.includes("§8.6"));
    expect(muc?.viSao).toContain("cán bộ không ghi đánh giá thay người dân");
  });

  it("the KPI cards left the list; only the heat-map (§9) and Báo cáo (§10) tabs remain, with why", () => {
    expect(PHAN_CHUA_DUNG.some((p) => p.ten.includes("KPI") || p.ten.includes("§3"))).toBe(false);
    // One entry per tab — each "?" opens its own reason: the basemap and all-points read still being
    // built, and no count by field / unit / hamlet for the whole commune.
    expect(petitionPendingPart("heatMapTab").ten).toBe("Bản đồ nhiệt");
    expect(petitionPendingPart("heatMapTab").viSao).toContain("lưu ngay trên máy chủ của hệ thống");
    expect(petitionPendingPart("heatMapTab").viSao).toContain("toàn bộ điểm phản ánh");
    expect(petitionPendingPart("reportTab").ten).toBe("Báo cáo");
    expect(petitionPendingPart("reportTab").viSao).toContain("theo lĩnh vực, theo bộ phận hay theo thôn");
    expect(petitionPendingPart("reportTab").viSao).toContain("Bốn thẻ số liệu");
  });

  it("02/10/2026: the three built entries are gone, the others kept, the one added is the intake's hamlet/photos", () => {
    const ten = PHAN_CHUA_DUNG.map((p) => p.ten);
    expect(ten).not.toContain("Modal `Nhập hộ phản ánh` (§11)");
    expect(ten).not.toContain("Câu giải thích trạng thái, tám trong chín (§8.2)");
    expect(ten).not.toContain("Ảnh sau khi xử lý (§8.4)");
    expect(ten).toEqual([
      "Bản đồ hiện trường và tên thôn",
      "Bản đồ nhiệt",
      "Báo cáo",
      // 06/10/2026 (ADR 0068 lần 5): the prototype's duplicates block.
      "Có thể trùng với phiếu khác",
      "Liên quan đến tôi",
      "Biểu mẫu `Ghi nhận đánh giá của người dân` (§8.6)",
      "Email của cán bộ trong ô `Đang giao cho` và ô chọn cán bộ (§8.3, §8.5)",
      "Thôn, tổ dân phố",
      "Đính ảnh hiện trường",
      // 09/10/2026: three prototype controls with no route behind them.
      "Đính kèm ảnh, tệp khi chuyển trạng thái",
      "Nội bộ",
      "Gỡ tệp đính kèm",
      "`⚠ Quá hạn 3 ngày` — số ngày trễ (§8.3, §7)",
    ]);
    // The overdue NUMBER stays unbuilt on purpose (ADR 0007 decision 10a): the entry still says why.
    const tre = PHAN_CHUA_DUNG.find((p) => p.ten.startsWith("`⚠ Quá hạn 3 ngày`"));
    expect(tre?.viSao).toContain("giờ làm việc");
  });
});

describe("vị trí hiện trường — toạ độ đã về, bản đồ chưa (§8.4)", () => {
  const tatCa = () => PHAN_CHUA_DUNG.map((x) => `${x.ten} ${x.viSao}`).join(" ");

  it("no entry still claims the contract lacks `lat`/`lng`", () => {
    expect(tatCa()).not.toContain("hợp đồng không trả `lat`/`lng`");
  });

  it("the heatmap and the mini-map say the self-hosted basemap is chosen and still being built (ADR 0072 §Sửa đổi 09/10/2026)", () => {
    // Staff-readable (ADR 0068 §14: the sentence opens behind a "?"); the ADR and the technical reason
    // live in the code comment above each entry, never on screen.
    for (const id of ["heatMapTab", "sceneMap"]) {
      const viSao = petitionPendingPart(id).viSao;
      expect(viSao, id).toContain("lưu ngay trên máy chủ của hệ thống");
      expect(viSao, id).toContain("đang chờ dựng tệp nền");
      // The provider question is decided: no entry may still say it is open.
      expect(viSao, id).not.toContain("chưa được quyết");
      expect(viSao, id).not.toMatch(/ADR|PMTiles/);
    }
    expect(petitionPendingPart("sceneMap").viSao).toContain("tên thôn");
    // Owner decision D2: the drawer's section is titled "Vị trí", as in the prototype; the lookup view
    // keeps its own row label.
    expect(LOCATION_SECTION_TITLE).toBe("Vị trí");
    expect(SCENE_LOCATION_LABEL).toBe("Vị trí hiện trường");
  });

  it("six decimals, latitude first", () => {
    expect(sceneCoordinates({ lat: 21.028511, lng: 105.804817 })).toBe("21.028511, 105.804817");
    // Padded, not trimmed: the precision stays readable as six places.
    expect(sceneCoordinates({ lat: 21.5, lng: 105 })).toBe("21.500000, 105.000000");
    expect(sceneCoordinates({ lat: -0.0000004, lng: 0 })).toBe("-0.000000, 0.000000");
  });

  it("absent, null, half, or not finite: no coordinates at all", () => {
    expect(sceneCoordinates({})).toBeNull();
    expect(sceneCoordinates({ lat: null, lng: null })).toBeNull();
    expect(sceneCoordinates({ lat: 21.028511 })).toBeNull();
    expect(sceneCoordinates({ lat: 21.028511, lng: null })).toBeNull();
    expect(sceneCoordinates({ lat: Number.NaN, lng: 105.804817 })).toBeNull();
    expect(sceneCoordinates({ lat: 21.028511, lng: Number.POSITIVE_INFINITY })).toBeNull();
  });
});

/**
 * Owner decision D1 (09/10/2026): a chip is pressable ONLY for a move the server performs for staff, and
 * only with that act's key. Pinned in both directions — the developer's account holds every key, so a
 * chip leaking past its gate is invisible while building.
 */
describe("status strip — which chips are pressable (D1)", () => {
  const ALL = congThaoTac(true, true, true);
  const NONE = congThaoTac(false, false, false);
  const p = (status: string, more: Record<string, unknown> = {}) =>
    ({
      code: "PA-1",
      channel: "zalo-mini-app",
      status,
      field: "rac-thai",
      field_label: "",
      content: "",
      address: "",
      reporter_name: "",
      reporter_phone: "",
      anonymous: true,
      clock_from: "2026-09-09T07:20:00Z",
      booked_at: "2026-09-09T07:21:00Z",
      acknowledge_due: null,
      resolve_due: null,
      classify_due: null,
      unit: "",
      assignee: "",
      result: "",
      public: false,
      ...more,
    }) as Parameters<typeof statusStrip>[0];
  const nextOf = (v: ReturnType<typeof statusStrip>) =>
    [...v.main, ...v.branches].filter((s) => s.role === "next").map((s) => `${s.code}:${s.act}`);

  it("every key: exactly the server's staff moves, each with its own act", () => {
    expect(nextOf(statusStrip(p("da-tiep-nhan"), ALL))).toEqual(["dang-phan-loai:classify"]);
    expect(nextOf(statusStrip(p("dang-phan-loai"), ALL))).toEqual([
      "da-chuyen-xu-ly:assign",
      "khong-tiep-nhan:reject",
      "chuyen-cap-tren:refer",
    ]);
    expect(nextOf(statusStrip(p("da-chuyen-xu-ly"), ALL))).toEqual(["dang-xu-ly:advance"]);
    expect(nextOf(statusStrip(p("dang-xu-ly"), ALL))).toEqual(["da-xu-ly:advance"]);
    // A citizen stands behind it: close only from `cho-dan-xac-nhan`.
    expect(nextOf(statusStrip(p("da-xu-ly"), ALL))).toEqual(["cho-dan-xac-nhan:advance"]);
    expect(nextOf(statusStrip(p("da-xu-ly", { has_citizen: false }), ALL))).toEqual([
      "cho-dan-xac-nhan:advance",
      "da-dong:close",
    ]);
    expect(nextOf(statusStrip(p("cho-dan-xac-nhan"), ALL))).toEqual(["da-dong:close"]);
    for (const end of ["da-dong", "khong-tiep-nhan", "chuyen-cap-tren"]) {
      expect(nextOf(statusStrip(p(end), ALL)), end).toEqual([]);
    }
  });

  it("no reopening chip for staff: only the citizen's 1–2 stars reopen (ADR 0050 point 2)", () => {
    for (const s of ["da-xu-ly", "cho-dan-xac-nhan", "da-dong"]) {
      const step = statusStrip(p(s, { has_citizen: false }), ALL).main.find((x) => x.code === "dang-xu-ly");
      expect(step?.role, s).toBe("blocked");
    }
  });

  it("DENIED — without the act's key the chip is blocked WITH the key's name; advance has no UI gate", () => {
    const classify = statusStrip(p("da-tiep-nhan"), NONE).main.find((s) => s.code === "dang-phan-loai");
    expect(classify?.role).toBe("blocked");
    expect(classify?.reason).toContain("feedback.classify");
    const screening = statusStrip(p("dang-phan-loai"), congThaoTac(true, false, false));
    expect(screening.main.find((s) => s.code === "da-chuyen-xu-ly")?.reason).toContain("feedback.assign");
    expect(nextOf(screening)).toEqual(["khong-tiep-nhan:reject", "chuyen-cap-tren:refer"]);
    const closing = statusStrip(p("cho-dan-xac-nhan"), congThaoTac(true, true, false));
    expect(closing.main.find((s) => s.code === "da-dong")?.reason).toContain("feedback.resolve");
    // The holder rule is the server's: advance stays pressable with no key at all.
    expect(nextOf(statusStrip(p("dang-xu-ly"), NONE))).toEqual(["da-xu-ly:advance"]);
  });

  it("the no-permission sentence: only when there IS a move and none is open to this account", () => {
    expect(statusStrip(p("da-tiep-nhan"), NONE).noPermission).toBe(true);
    expect(statusStrip(p("cho-dan-xac-nhan"), NONE).noPermission).toBe(true);
    expect(statusStrip(p("dang-xu-ly"), NONE).noPermission).toBe(false);
    expect(statusStrip(p("da-dong"), NONE).noPermission).toBe(false);
    expect(statusStrip(p("da-tiep-nhan"), ALL).noPermission).toBe(false);
  });

  it("branch row: only the branch the petition is on, or one it can move to", () => {
    expect(statusStrip(p("dang-xu-ly"), ALL).branches).toEqual([]);
    expect(statusStrip(p("dang-phan-loai"), ALL).branches.map((s) => s.code)).toEqual([
      "khong-tiep-nhan",
      "chuyen-cap-tren",
    ]);
    // Without the key the branches are still SHOWN (they are moves of the lifecycle), but blocked.
    expect(statusStrip(p("dang-phan-loai"), NONE).branches.map((s) => s.role)).toEqual(["blocked", "blocked"]);
    expect(statusStrip(p("chuyen-cap-tren"), ALL).branches.map((s) => `${s.code}:${s.role}`)).toEqual([
      "chuyen-cap-tren:current",
    ]);
  });
});

describe("staff intake — the words and the clock (§11)", () => {
  it("the modal's description is the sentence the owner approved on 30/09/2026, verbatim", () => {
    expect(INTAKE_DESCRIPTION).toBe(
      "Dùng khi người dân gọi điện, ghé trụ sở, hoặc gặp trưởng thôn ngoài địa bàn. Phiếu nhập ở đây đi " +
        "cùng quy trình với phiếu gửi từ Zalo. Vì đã biết lĩnh vực ngay, hạn xử lý được ấn định luôn — hãy " +
        "ghi đúng thời điểm người dân phản ánh.",
    );
    // The old sentence (ADR 0028 E/F made it false) never comes back.
    expect(INTAKE_DESCRIPTION).not.toContain("cùng thời hạn");
  });

  it("the key is the seeded `feedback.create`", () => {
    expect(PETITION_INTAKE_PERMISSION).toBe("feedback.create");
  });

  it("`Dân phản ánh lúc` is read on the COMMUNE'S clock (+07:00), whatever the browser's zone", () => {
    expect(clockFromRfc3339("2026-10-02T08:30")).toBe("2026-10-02T08:30:00+07:00");
    expect(clockFromRfc3339("2026-10-02T08:30:15")).toBe("2026-10-02T08:30:15+07:00");
    expect(clockFromRfc3339("")).toBe("");
    expect(clockFromRfc3339("   ")).toBe("");
    // Not the input's shape: refused, never guessed.
    expect(clockFromRfc3339("02/10/2026 08:30")).toBeNull();
    expect(clockFromRfc3339("2026-10-02T08:30Z")).toBeNull();
  });

  it("the picker's bounds are seven days back to now, in Vietnam time (tests run under TZ=UTC)", () => {
    const now = new Date("2026-10-02T01:30:00Z"); // 08:30 in Hà Nội
    expect(toLocalInputValue(now)).toBe("2026-10-02T08:30");
    expect(toLocalInputValue(new Date("2026-10-01T17:05:00Z"))).toBe("2026-10-02T00:05");
    expect(clockFromBounds(now)).toEqual({ min: "2026-09-25T08:30", max: "2026-10-02T08:30" });
  });

  it("content is required: blank or spaces refuse before sending", () => {
    expect(intakeContentError("")).not.toBeNull();
    expect(intakeContentError("  \n ")).not.toBeNull();
    expect(intakeContentError("Rác")).toBeNull();
  });
});

describe("KPI arithmetic (§3) — two divisions, never a 0 for 'no value'", () => {
  it("average = sum / sample, ONE decimal, Vietnamese comma", () => {
    expect(ratingAverage(13, 3)).toBe("4,3/5");
    expect(ratingAverage(8, 2)).toBe("4,0/5");
    expect(ratingAverage(1, 1)).toBe("1,0/5");
  });

  it("sample 0 → no value (the card shows —), NEVER 0,0/5", () => {
    expect(ratingAverage(0, 0)).toBeNull();
  });

  it("fields absent or null (a server before 02/10/2026) → no value", () => {
    expect(ratingAverage(undefined, undefined)).toBeNull();
    expect(ratingAverage(null, 3)).toBeNull();
    expect(ratingAverage(12, null)).toBeNull();
  });

  it("on-time rate: on_time / on_time_sample, one decimal; sample 0 → no value, never 0%", () => {
    expect(onTimePercent(1, 3)).toBe("33,3%");
    expect(onTimePercent(0, 21)).toBe("0,0%");
    expect(onTimePercent(0, 0)).toBeNull();
  });
});

describe("verification photos — when, and which files (§8.4)", () => {
  it("upload is offered on every status but the three endings — the server's own rule", () => {
    for (const ma of MOI_TRANG_THAI) {
      expect(afterPhotoUploadOpen(ma), ma).toBe(!["da-dong", "khong-tiep-nhan", "chuyen-cap-tren"].includes(ma));
    }
    // An unknown status keeps the button: the server's 409 is the answer (a deny list, like the server's).
    expect(afterPhotoUploadOpen("mot-ma-moi")).toBe(true);
  });

  it("JPEG / PNG / WebP declared; anything else, or an empty file, refused before upload", () => {
    expect(afterPhotoType({ name: "a.jpg", type: "image/jpeg", size: 10 })).toEqual({ ok: true, contentType: "image/jpeg" });
    expect(afterPhotoType({ name: "a.webp", type: "", size: 10 })).toEqual({ ok: true, contentType: "image/webp" });
    expect(afterPhotoType({ name: "a.pdf", type: "application/pdf", size: 10 }).ok).toBe(false);
    expect(afterPhotoType({ name: "a.heic", type: "", size: 10 }).ok).toBe(false);
    expect(afterPhotoType({ name: "a.png", type: "image/png", size: 0 }).ok).toBe(false);
  });
});
