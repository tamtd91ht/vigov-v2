import { afterEach, describe, expect, it, vi } from "vitest";

import {
  bookStaffIntake,
  chuyenCapTrenPhieu,
  chuyenXuLyPhieu,
  completeLogAttachment,
  completeVerificationPhoto,
  dongPhieu,
  listIntakeFields,
  listVerificationPhotos,
  logAttachmentDownloadLink,
  requestLogAttachmentUpload,
  requestVerificationPhotoUpload,
  staffIntakeBody,
  type StaffIntakeInput,
  duongDanNhatKyPhieu,
  duongDanSoPhanAnh,
  ghiNhatKyPhieu,
  khongTiepNhanPhieu,
  layNhatKyPhieu,
  layPhieuPhanAnh,
  phanLoaiPhieu,
  setPetitionPublication,
  tienTrangThaiPhieu,
  countCitizenReports,
  listCitizenReportPoints,
  readCitizenReportBreakdown,
  removeLogAttachment,
} from "./phieu-phan-anh";
import type { petitions_phieuCuaToiRa, petitions_phieuPhanAnhRa } from "./schema.gen";
import { UPLOAD_FORM_MISSING } from "./task-attachments";

function batFetch(tra: Response) {
  // Tham số được khai rõ để `mock.calls[0][0]` có kiểu — một `vi.fn(async () => …)`
  // không tham số làm TypeScript coi danh sách đối số là tuple rỗng.
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const KHONG_TIM_THAY = new Response(
  JSON.stringify({
    code: "not_found",
    message: "Không tìm thấy phiếu phản ánh.",
    trace_id: "01JTRACE",
  }),
  { status: 404, headers: { "Content-Type": "application/json" } },
);

describe("tra phiếu theo mã tra cứu", () => {
  it("mã hoá mã vào đường dẫn thay vì ghép thẳng", async () => {
    const gia = batFetch(
      new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }),
    );

    await layPhieuPhanAnh("PA/2026 0021");
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA%2F2026%200021");
  });

  it("đường dẫn tương đối, không host, không `tenant_id`", async () => {
    const gia = batFetch(
      new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } }),
    );

    await layPhieuPhanAnh("PA-2026-0021");
    const duong = String(gia.mock.calls[0]?.[0]);
    expect(duong.startsWith("/api/")).toBe(true);
    expect(duong).not.toMatch(/tenant/i);
  });

  it("BỐN TÌNH HUỐNG, MỘT CÂU TRẢ LỜI — và giao diện không dựng lại sự phân biệt ấy", async () => {
    // Mã không tồn tại · mã của xã khác · phiếu đã xoá mềm · phiếu thuộc lĩnh vực hạn chế mà tài
    // khoản thiếu `feedback.restricted`. Máy chủ trả CÙNG một 404 cho cả bốn: phân biệt được
    // chúng là nói cho người đang thử mã biết họ gần tới đâu, và ở ca thứ tư là nói cho một đồng
    // nghiệp biết có người vừa phản ánh về họ (luật 4, cấm #2).
    //
    // Bài test này đỏ ngay khi ai đó thêm một nhánh rẽ theo `code` để "nói rõ hơn cho cán bộ".
    batFetch(KHONG_TIM_THAY.clone());
    const maBia = await layPhieuPhanAnh("PA-2026-9999");

    batFetch(KHONG_TIM_THAY.clone());
    const linhVucHanChe = await layPhieuPhanAnh("PA-2026-0021");

    expect(maBia).toEqual(linhVucHanChe);
    expect(maBia).toEqual({ ok: false, thongBao: "Không tìm thấy phiếu phản ánh." });
  });

  it("403 thiếu `feedback.read`: câu của máy chủ đi thẳng ra màn hình", async () => {
    batFetch(
      new Response(
        JSON.stringify({
          code: "forbidden",
          message: "Bạn không có quyền thực hiện thao tác này.",
          trace_id: "01JTRACE",
        }),
        { status: 403, headers: { "Content-Type": "application/json" } },
      ),
    );

    expect(await layPhieuPhanAnh("PA-2026-0021")).toEqual({
      ok: false,
      thongBao: "Bạn không có quyền thực hiện thao tác này.",
    });
  });

  it("200: trả về NGUYÊN phản hồi đã che sẵn của máy chủ, không ghép lại gì", async () => {
    // Hai trường người gửi về ở dạng đã che. Không có phép biến đổi nào ở tầng gọi API — nếu có
    // ngày ai đó thêm một hàm "làm đẹp số điện thoại" thì bài test này đỏ.
    const than = {
      code: "PA-2026-0021",
      channel: "zalo-mini-app",
      status: "dang-phan-loai",
      field: "",
      field_label: "",
      content: "Rác tồn đọng ở đầu ngõ ba ngày chưa ai dọn.",
      address: "Tổ 6, thôn Hà Lam",
      reporter_name: "Nguyễn V. A.",
      reporter_phone: "09****5678",
      anonymous: false,
      clock_from: "2026-09-09T07:20:00Z",
      booked_at: "2026-09-09T07:21:00Z",
      acknowledge_due: "2026-09-09T09:20:00Z",
      resolve_due: null,
      public: false,
    };

    batFetch(
      new Response(JSON.stringify(than), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const kq = await layPhieuPhanAnh("PA-2026-0021");
    expect(kq).toEqual({ ok: true, duLieu: than });
  });

});

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * TÊN THAM SỐ TRUY VẤN — VÀ ĐÂY LÀ CHỖ DUY NHẤT CANH CHÚNG.
 *
 * Hợp đồng nay đã khai các tham số của `GET /api/v1/citizen-reports`, nhưng
 * `themLocVaoTruyVan` vẫn ghép tên bằng chuỗi trần, nên `tsc` không canh giúp một chữ nào ở đây: gõ
 * `hamlet_id` thay vì `hamlet` thì máy chủ bỏ qua bộ lọc và trả về CẢ QUYỂN SỔ, còn màn hình trông
 * hoàn toàn bình thường — cán bộ tin mình đang xem một thôn.
 *
 * Mỗi `expect` dưới đây vì thế là một chuỗi ĐỌC LẠI TỪ HANDLER, không phải từ trí nhớ.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
describe("đường dẫn đọc sổ phản ánh", () => {
  it("không lọc gì thì KHÔNG có dấu hỏi thừa", () => {
    expect(duongDanSoPhanAnh()).toBe("/api/v1/citizen-reports");
  });

  it("bảy bộ lọc đi ra bảy tên tham số máy chủ thật sự đọc", () => {
    const duong = duongDanSoPhanAnh({
      trangThai: "dang-xu-ly",
      kenh: "zalo-mini-app",
      linhVuc: "rac-thai",
      thonID: "01JTHON",
      boPhanID: "01JBOPHAN",
      tim: "rác đầu ngõ",
      chiTreHan: true,
    });
    const truyVan = new URLSearchParams(duong.slice(duong.indexOf("?") + 1));

    expect(truyVan.get("status")).toBe("dang-xu-ly");
    expect(truyVan.get("channel")).toBe("zalo-mini-app");
    expect(truyVan.get("field")).toBe("rac-thai");
    expect(truyVan.get("hamlet")).toBe("01JTHON");
    expect(truyVan.get("unit")).toBe("01JBOPHAN");
    expect(truyVan.get("q")).toBe("rác đầu ngõ");
    expect(truyVan.get("late")).toBe("true");
  });

  it("ô `Chỉ phiếu trễ hạn` bỏ tích: tham số VẮNG MẶT HẲN, không phải `late=false`", () => {
    // Máy chủ chỉ nhận đúng chuỗi `true` và trả **400** cho mọi giá trị khác
    // (`errLocTreHanKhongHopLe`) — có chủ ý, vì một ô đã tích mà bị bỏ qua lặng lẽ sẽ hiện cả sổ.
    expect(duongDanSoPhanAnh({ chiTreHan: false })).toBe("/api/v1/citizen-reports");
  });

  it("trang đầu KHÔNG gửi `cursor` rỗng", () => {
    // `cursor=` rỗng là 400 "con trỏ không hợp lệ" — đúng vào lần mở màn hình đầu tiên.
    const duong = duongDanSoPhanAnh({ limit: 20, cursor: null });
    expect(duong).toBe("/api/v1/citizen-reports?limit=20");
  });

  it("trang sau mang đúng con trỏ máy chủ phát ra", () => {
    const duong = duongDanSoPhanAnh({ limit: 20, cursor: "eyJrIjoi" });
    expect(new URLSearchParams(duong.slice(duong.indexOf("?") + 1)).get("cursor")).toBe("eyJrIjoi");
  });

  it("tab `Giao cho tôi` gửi `scope=mine`, và KHÔNG kèm danh tính nào", () => {
    const duong = duongDanSoPhanAnh({ phamVi: "mine", trangThai: "dang-xu-ly" });
    const truyVan = new URLSearchParams(duong.slice(duong.indexOf("?") + 1));
    expect(truyVan.get("scope")).toBe("mine");
    // Các bộ lọc khác đi cùng, không bị tab nuốt mất.
    expect(truyVan.get("status")).toBe("dang-xu-ly");
    // Máy chủ lấy mã cán bộ từ PHIÊN. Một `assignee=` do client gửi là client tự khai mình là ai.
    expect(truyVan.has("assignee")).toBe(false);
  });

  it("tab `Toàn xã` (vắng hoặc `all`): KHÔNG gửi `scope` — mặc định của máy chủ", () => {
    expect(duongDanSoPhanAnh({})).toBe("/api/v1/citizen-reports");
    expect(duongDanSoPhanAnh({ phamVi: "all" })).toBe("/api/v1/citizen-reports");
  });

  it("`scope=related` không bao giờ được gửi — máy chủ trả 400 cho nó", () => {
    // Kiểu `phamVi` không cho viết `"related"` (`tsc` đỏ). Bài chạy này canh chiều còn lại: không
    // bộ lọc nào khác lọt ra thành `related`.
    for (const phamVi of [undefined, "all", "mine"] as const) {
      expect(duongDanSoPhanAnh({ phamVi, chiTreHan: true })).not.toContain("related");
    }
  });

  it("`Bị đánh giá thấp`: `rating_max=2`; unticked, the parameter is ABSENT", () => {
    const duong = duongDanSoPhanAnh({ ratingMax: 2, trangThai: "da-dong" });
    const truyVan = new URLSearchParams(duong.slice(duong.indexOf("?") + 1));
    expect(truyVan.get("rating_max")).toBe("2");
    expect(truyVan.get("status")).toBe("da-dong");
    // `rating_max=` empty is a 400 — the box unticked must send nothing at all.
    expect(duongDanSoPhanAnh({ ratingMax: undefined })).toBe("/api/v1/citizen-reports");
  });

  it("không một chỗ nào mang `tenant_id`", () => {
    // Client tự khai xã là client tự cấp quyền (luật 1, cấm #2). Xã suy từ `Host` ở rìa ngoài cùng.
    const duong = duongDanSoPhanAnh({ trangThai: "da-dong", boPhanID: "01JBOPHAN" });
    expect(duong).not.toMatch(/tenant/i);
    expect(duong.startsWith("/api/")).toBe(true);
  });
});

describe("bốn thao tác ghi", () => {
  function batGhi() {
    const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) =>
      new Response(JSON.stringify({ code: "PA-2026-0021" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", gia);
    return gia;
  }

  it("phân loại: POST …/classification, đúng MỘT trường `field`", () => {
    const gia = batGhi();
    return phanLoaiPhieu("PA-2026-0021", "rac-thai").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/classification");
      const tuyChon = gia.mock.calls[0]?.[1];
      expect(tuyChon?.method).toBe("POST");
      // KHÔNG có `due_at` và KHÔNG có `status`: hạn là cam kết của xã tính theo giờ làm việc của
      // chính xã, và hành vi này chỉ đưa phiếu sang `dang-phan-loai`.
      expect(JSON.parse(String(tuyChon?.body))).toEqual({ field: "rac-thai" });
    });
  });

  it("chuyển xử lý không chọn cán bộ: thân KHÔNG có `assignee` rỗng", () => {
    const gia = batGhi();
    return chuyenXuLyPhieu("PA-2026-0021", "01JBOPHAN").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/assignment");
      expect(JSON.parse(String(gia.mock.calls[0]?.[1]?.body))).toEqual({ unit: "01JBOPHAN" });
    });
  });

  it("chuyển xử lý có chọn cán bộ: `assignee` là MÃ CÁN BỘ, không phải id nội bộ", () => {
    // Luật nắm giữ so `can_bo_xu_ly_id` với `Principal.Ma`. Một ULID ở đây là một phiếu "đã giao"
    // mà người được giao không bao giờ tiến được.
    const gia = batGhi();
    return chuyenXuLyPhieu("PA-2026-0021", "01JBOPHAN", "CB-00123").then(() => {
      const than = JSON.parse(String(gia.mock.calls[0]?.[1]?.body)) as Record<string, unknown>;
      expect(than).toEqual({ unit: "01JBOPHAN", assignee: "CB-00123" });
      expect(Object.keys(than)).not.toContain("id");
      expect(Object.keys(than)).not.toContain("assignee_id");
    });
  });

  it("chuyển xử lý với mã cán bộ rỗng: như không chọn ai, không gửi `assignee` rỗng", () => {
    const gia = batGhi();
    return chuyenXuLyPhieu("PA-2026-0021", "01JBOPHAN", "").then(() => {
      expect(JSON.parse(String(gia.mock.calls[0]?.[1]?.body))).toEqual({ unit: "01JBOPHAN" });
    });
  });

  it("tiến trạng thái: POST …/status, KHÔNG THÂN và KHÔNG `Content-Type`", () => {
    // Một trạng thái đích đi trên dây là một client nhảy được bước. Gửi `{}` kèm `Content-Type` là
    // tuyên bố có một thân — thứ mời người sau điền vào đó đúng cái trường ấy.
    const gia = batGhi();
    return tienTrangThaiPhieu("PA-2026-0021").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/status");
      const tuyChon = gia.mock.calls[0]?.[1];
      expect(tuyChon?.body).toBeUndefined();
      expect(tuyChon?.headers).toEqual({});
    });
  });

  it("đóng phiếu: POST …/closure kèm kết quả người dân đọc được", () => {
    const gia = batGhi();
    return dongPhieu("PA-2026-0021", "Đã dọn xong điểm tập kết rác chiều 10/9.").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/closure");
      expect(JSON.parse(String(gia.mock.calls[0]?.[1]?.body))).toEqual({
        result: "Đã dọn xong điểm tập kết rác chiều 10/9.",
      });
    });
  });

  it("mã tra cứu đi vào ĐƯỜNG DẪN thì được mã hoá, ở cả bốn tuyến", () => {
    const gia = batGhi();
    return dongPhieu("PA/2026 0021", "xong").then(() => {
      expect(gia.mock.calls[0]?.[0]).toBe(
        "/api/v1/citizen-reports/PA%2F2026%200021/closure",
      );
    });
  });

  it("409 của máy chủ ra thẳng màn hình, nguyên văn", () => {
    // Ba tuyến trả 409 với đúng quy tắc nghiệp vụ đã từ chối. Viết lại câu ấy ở client là dựng bản
    // sao thứ hai của một quy tắc rồi để nó trôi.
    vi.stubGlobal(
      "fetch",
      vi.fn(async () =>
        new Response(
          JSON.stringify({
            code: "sla_chua_cau_hinh",
            message:
              "Xã chưa cấu hình thời hạn xử lý cho lĩnh vực này, nên chưa phân loại được. " +
              "Vào Cấu hình → Thời hạn xử lý để đặt số giờ, rồi phân loại lại.",
            trace_id: "01JTRACE",
          }),
          { status: 409, headers: { "Content-Type": "application/json" } },
        ),
      ),
    );

    return phanLoaiPhieu("PA-2026-0021", "an-ninh").then((kq) => {
      expect(kq.ok).toBe(false);
      expect(kq.ok === false && kq.thongBao).toContain("Cấu hình → Thời hạn xử lý");
    });
  });
});

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * PHÉP CANH CHIỀU NGƯỢC: hợp đồng KHÔNG được mọc thêm ghi chú nội bộ hay lịch sử luân chuyển.
 *
 * Phép canh này đọc KIỂU SINH RA TỪ HỢP ĐỒNG, không đọc một thân JSON do chính bài test viết ra.
 * Một ca test gửi vào 15 trường rồi đếm lại 15 trường không canh gì cả — nó chỉ chứng minh
 * `JSON.parse(JSON.stringify(x))` giữ nguyên khoá, và nó vẫn xanh nguyên vào ngày tuyến trả về
 * thêm `ghi_chu_noi_bo`.
 *
 * Ở đây thì ngược lại: thêm MỘT trường vào `petitions.phieuPhanAnhRa` là `tsc` đỏ ngay tại dòng
 * `_duKhoaPhieu` bên dưới — nên không trường nào vào được hợp đồng mà không có người đọc nó là gì.
 *
 * ⚠ TIỀN ĐỀ CŨ CỦA KHỐI NÀY ĐÃ SAI, SỬA 23/09/2026. Câu cũ viết *"tuyến này là tuyến tra theo mã
 * người dân cầm trên tay"*, và nó đúng vào ngày được viết. Nay KHÔNG còn: cả sáu tuyến trả về
 * `phieuPhanAnhRa` đều đòi `feedback.*` (đo trên `kb/20-contracts/openapi.json`), tức đây là hình
 * dạng CỦA CÁN BỘ. Công dân đọc một hình dạng KHÁC HẲN — `petitions.phieuCuaToiRa` trên
 * `my-citizen-reports`, đúng cách luật 4 bất biến 5 đòi hai bề mặt không dùng chung handler.
 *
 * HỆ QUẢ: phép canh "không được mọc trường nội bộ" chuyển sang hình dạng CÔNG DÂN, chỗ luật 4 cấm
 * #5 thật sự áp. Giữ nó ở hình dạng cán bộ là canh sai chỗ, và nó vừa chứng minh điều đó bằng cách
 * đỏ lên trước một trường HOÀN TOÀN HỢP LỆ: `assignee` — người được phân công — là thứ màn hình xử
 * lý của cán bộ tồn tại để hiện ra.
 *
 * Ở hình dạng cán bộ thì phép canh còn lại vẫn nguyên giá trị và KHÔNG được nới: mỗi trường mới
 * phải được thêm vào danh sách dưới đây BẰNG TAY, tức phải có người đọc nó là gì trước khi nó ra
 * tới màn hình.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
type KhoaPhieu = keyof petitions_phieuPhanAnhRa;

const KHOA_PHIEU_MONG_DOI = [
  "code",
  "channel",
  "status",
  "field",
  "field_label",
  "content",
  "address",
  "reporter_name",
  "reporter_phone",
  "anonymous",
  "clock_from",
  "booked_at",
  "acknowledge_due",
  "resolve_due",
  "public",
  // NĂM TRƯỜNG CỦA ĐƯỜNG XỬ LÝ PHÍA CÁN BỘ, thêm 23/09/2026 — và chúng vào đây SAU KHI đã đối
  // chiếu từng tuyến trả về hình dạng này: cả sáu đều đòi `feedback.*` (`citizen-reports` và bốn
  // tuyến ghi của nó). Tuyến công dân `my-citizen-reports` trả một hình dạng KHÁC, nên không
  // trường nào dưới đây tới được màn hình người dân — đó là điều luật 4 cấm #5 đòi, và là lý do
  // danh sách này tồn tại thay vì một dòng `Partial<>`.
  //
  // `classify_due` là hạn THỨ BA của một phiếu (trần bắt buộc phân loại, ADR 0035 §C), cạnh
  // `acknowledge_due` và `resolve_due`. `unit` · `assignee` · `result` là bộ phận đang giữ,
  // người được phân công, và kết quả đóng phiếu — thứ công dân ĐƯỢC đọc ở dạng đã lọc qua tuyến
  // riêng của họ, không phải qua hình dạng này.
  "classify_due",
  "unit",
  "assignee",
  "result",
  // BA TRƯỜNG CỦA HAI NHÁNH RẼ, thêm 25/09/2026 (27aed64): lý do không tiếp nhận / chuyển cấp
  // trên, cơ quan nhận, lúc rẽ nhánh. Cả ba là thứ công dân ĐƯỢC đọc (tuyến riêng của họ cũng trả
  // `reason`/`receiving_body`), không phải ghi chú nội bộ; chỉ có mặt khi phiếu ở hai nhánh ấy.
  "reason",
  "receiving_body",
  "branch_ended_at",
  // Thêm 26/09/2026 (4ce8933): phiếu có công dân đứng sau để xác nhận không. Chỉ là cờ, không mang
  // `cong_dan_id` — cho biết CÓ tài khoản, không cho biết AI.
  "has_citizen",
  // Added in dde0f8f3 (ADR 0080): true when the petition came from a session whose phone
  // number was never verified, so the contact on it is unconfirmed. A flag about the channel, not a
  // staff note or routing history (rule 4, forbidden #5); it carries no identity of its own.
  "contact_unverified",
  // Added 28/09/2026 (ADR 0050 points 2 and 8), all five read by the drawer and the lookup view:
  // `publication_status` is the staff moderation of the public page (`public` is now derived from
  // it); `rating` · `rating_comment` · `rated_at` are the CITIZEN'S verdict — `rating_comment` is the
  // citizen's own words and follows `content` (not masked, never logged); `reopen_count` counts the
  // reopenings a low rating caused. None is a staff note or routing history (rule 4, forbidden #5).
  "publication_status",
  "rating",
  "rating_comment",
  "rated_at",
  "reopen_count",
  // Added 29/09/2026 (b5d17bb): the scene coordinates the citizen optionally sent from the Mini App,
  // six decimals. Personal data (rule 3 lists coordinates) that follows `address`: returned under
  // `feedback.read`, anonymous petitions included — the flag hides the reporter, not the place. The
  // screens show them as TEXT only; no map tile or outbound link sends them anywhere
  // (`features/phan-anh/nhan-phieu.ts`, `sceneCoordinates`).
  "lat",
  "lng",
  // ADR 0083 (08/10/2026, temporary): an explicit marker that the petition came in with no account — derived in
  // one function of service-petitions, never a stored flag. Not personal data; staff see a label from it.
  "accountless",
  // ADR 0088 §1 (09/10/2026): the thôn / tổ dân phố recorded ON the petition at intake or classification
  // (id + name). Not personal data — a public administrative unit; never derived from the coordinates.
  "residential_unit_id",
  "residential_unit_name",
  // ADR 0087 (09/10/2026): the duplicate link, staff responses only — `merged_into` is the main
  // petition's code, `merged_petitions` the codes linked to this one, `merged_at` / `merged_by` who
  // linked it (a staff business code, rule 6 inv 8). Codes, not content: no citizen response carries them.
  "merged_into",
  "merged_petitions",
  "merged_at",
  "merged_by",
] as const satisfies readonly KhoaPhieu[];

/** Hợp đồng mọc thêm một trường mà danh sách trên không có → đỏ ngay tại đây. */
type DuKhoaPhieu =
  Exclude<KhoaPhieu, (typeof KHOA_PHIEU_MONG_DOI)[number]> extends never ? true : never;
const _duKhoaPhieu: DuKhoaPhieu = true;
void _duKhoaPhieu;

describe("phạm vi của phiếu trả về cho cán bộ", () => {
  it("không trường nào trong hợp đồng mang tên của ghi chú nội bộ hay lịch sử luân chuyển", () => {
    // Phép kiểm chạy được, đứng cạnh phép kiểm ở mức KIỂU bên trên. Nó bắt chiều còn lại: một
    // trường mới được thêm vào CẢ hợp đồng LẪN danh sách trên mà không ai đọc lại nó là gì.
    for (const khoa of KHOA_PHIEU_MONG_DOI) {
      expect(khoa).not.toMatch(/noi_bo|internal|note|history|luan_chuyen|nhat_ky/i);
    }
  });
});

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * PHÉP CANH CỦA LUẬT 4 — ĐẶT ĐÚNG CHỖ: hình dạng CÔNG DÂN đọc.
 *
 * `petitions.phieuCuaToiRa` là thứ `GET /api/v1/my-citizen-reports/{maTraCuu}` trả cho chính
 * người đã gửi phiếu. Luật 4 cấm #5 áp ở ĐÂY, không ở hình dạng của cán bộ: ghi chú nội bộ, lịch
 * sử luân chuyển và người được phân công nằm ngoài phạm vi công dân.
 *
 * SO SÁNH HAI HÌNH DẠNG LÀ ĐIỀU ĐÁNG CANH NHẤT, chứ không phải đếm trường: hai bề mặt dùng chung
 * một hình dạng là đúng thứ luật 4 bất biến 5 cấm, và nó xảy ra bằng một lần ai đó "tái dùng cho
 * gọn". Nếu hai kiểu này có ngày trùng nhau thì `tsc` không đỏ — nên phép kiểm dưới đây khẳng
 * định bốn trường CHỈ CÓ ở phía cán bộ vẫn vắng mặt ở phía công dân.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
type KhoaPhieuCuaToi = keyof petitions_phieuCuaToiRa;

describe("phạm vi của phiếu trả về cho CÔNG DÂN", () => {
  it("không trường nào mang tên ghi chú nội bộ, lịch sử luân chuyển hay người phân công", () => {
    const khoa: readonly KhoaPhieuCuaToi[] = [
      "code", "channel", "status", "field", "field_label", "content", "address",
      "reporter_name", "reporter_phone", "anonymous", "clock_from",
      "acknowledge_due", "resolve_due", "result",
    ];
    for (const k of khoa) {
      expect(k).not.toMatch(/noi_bo|internal|note|history|luan_chuyen|assignee|nhat_ky|unit/i);
    }
  });

  it("BỐN trường chỉ-của-cán-bộ KHÔNG có mặt ở hình dạng công dân", () => {
    // VẾ CHỊU LỰC của cả tệp này. `tsc` đỏ khi một trong bốn tên dưới đây trở thành khoá hợp lệ
    // của `phieuCuaToiRa` — tức đúng ngày ai đó cho hai bề mặt dùng chung một hình dạng, hoặc
    // thêm thẳng một trường nội bộ vào hình dạng công dân.
    //
    // `classify_due` nằm trong bốn tên ấy có chủ ý: trần bắt buộc phân loại là một con số NỘI BỘ
    // để xã tự chấm mình, không phải một lời hứa đã nói với người dân. Hai hạn đã hứa —
    // `acknowledge_due` và `resolve_due` — thì CÓ mặt, và đó là điểm khác nhau.
    type ChiCuaCanBo = "unit" | "assignee" | "classify_due" | "public";
    type KhongDuocLo = Extract<KhoaPhieuCuaToi, ChiCuaCanBo> extends never ? true : never;
    const _khongLo: KhongDuocLo = true;
    void _khongLo;
    expect(_khongLo).toBe(true);
  });
});

describe("hai nhánh rẽ — …/rejection và …/referral", () => {
  const PHIEU_SAU = new Response(JSON.stringify({ code: "PA-2026-0021", status: "khong-tiep-nhan" }), {
    status: 200,
    headers: { "Content-Type": "application/json" },
  });
  const LY_DO = "  Nội dung thuộc thẩm quyền ngành điện, không thuộc xã.  ";

  it("rejection: POST đúng tuyến, thân ĐÚNG MỘT khoá `reason`, đã cắt khoảng trắng", async () => {
    const gia = batFetch(PHIEU_SAU.clone());
    const kq = await khongTiepNhanPhieu("PA-2026-0021", LY_DO);

    expect(kq.ok).toBe(true);
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/rejection");
    const tuyChon = gia.mock.calls[0]?.[1];
    expect(tuyChon?.method).toBe("POST");
    expect(JSON.parse(String(tuyChon?.body))).toEqual({ reason: LY_DO.trim() });
  });

  it("referral: thân ĐÚNG HAI khoá `reason` và `receiving_body`, cả hai đã cắt", async () => {
    const gia = batFetch(PHIEU_SAU.clone());
    await chuyenCapTrenPhieu("PA-2026-0021", LY_DO, "  Công ty điện lực  ");

    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/referral");
    const than = JSON.parse(String(gia.mock.calls[0]?.[1]?.body)) as Record<string, unknown>;
    expect(Object.keys(than).sort()).toEqual(["reason", "receiving_body"]);
    expect(than).toEqual({ reason: LY_DO.trim(), receiving_body: "Công ty điện lực" });
  });

  it("không gửi Idempotency-Key — hai tuyến không đòi (như …/closure)", async () => {
    const gia = batFetch(PHIEU_SAU.clone());
    await khongTiepNhanPhieu("PA-2026-0021", LY_DO);
    const header = new Headers(gia.mock.calls[0]?.[1]?.headers);
    expect(header.has("Idempotency-Key")).toBe(false);
  });

  it("lý do KHÔNG lên URL và KHÔNG ra console, kể cả khi máy chủ từ chối", async () => {
    const soi = [
      vi.spyOn(console, "log"),
      vi.spyOn(console, "info"),
      vi.spyOn(console, "warn"),
      vi.spyOn(console, "error"),
      vi.spyOn(console, "debug"),
    ];
    const gia = batFetch(
      new Response(
        JSON.stringify({ code: "petition_state", message: "Phiếu đã chuyển trạng thái.", trace_id: "x" }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );
    await chuyenCapTrenPhieu("PA-2026-0021", LY_DO, "Công an xã");

    const duong = decodeURIComponent(String(gia.mock.calls[0]?.[0]));
    expect(duong).not.toContain("thẩm quyền");
    expect(duong).not.toContain("Công an");
    expect(duong).not.toContain("?");
    for (const s of soi) expect(s).not.toHaveBeenCalled();
  });

  it("409 `petition_state`: câu của máy chủ ra NGUYÊN VĂN", async () => {
    batFetch(
      new Response(
        JSON.stringify({
          code: "petition_state",
          message: "Chỉ từ chối tiếp nhận hoặc chuyển cấp trên được phiếu đang ở bước phân loại.",
          trace_id: "01JTRACE",
        }),
        { status: 409, headers: { "Content-Type": "application/json" } },
      ),
    );
    expect(await khongTiepNhanPhieu("PA-2026-0021", LY_DO)).toEqual({
      ok: false,
      thongBao: "Chỉ từ chối tiếp nhận hoặc chuyển cấp trên được phiếu đang ở bước phân loại.",
    });
  });
});

/**
 * NHẬT KÝ XỬ LÝ — GET/POST …/log-entries.
 *
 * Ba điều canh ở đây, mỗi điều là một cách hỏng không làm đỏ màn hình nào:
 *   1. trang đầu KHÔNG gửi `cursor=` rỗng (máy chủ trả 400 đúng lần mở phiếu đầu tiên);
 *   2. POST mang `Idempotency-Key` ĐÚNG khoá được truyền vào, và lần gửi lại mang LẠI khoá ấy —
 *      khoá sinh trong hàm là hai dòng nhật ký giống hệt nhau sau một lần lỗi mạng;
 *   3. thân ĐÚNG MỘT khoá `note`, đã cắt khoảng trắng — không `status`, không `actor_code`: người ghi
 *      là PHIÊN, không phải thân yêu cầu.
 */
describe("nhật ký xử lý — …/log-entries", () => {
  const DONG = {
    id: "01JDONG",
    at: "2026-09-26T03:00:00Z",
    actor_code: "CB-00123",
    action: "ghi-chu",
    status: "dang-xu-ly",
    unit: "",
    assignee: "",
    note: "Đã gọi đội vệ sinh.",
  };

  it("trang đầu: đúng tuyến, mã được mã hoá, KHÔNG có `cursor` rỗng", () => {
    expect(duongDanNhatKyPhieu("PA/2026 0021", { limit: 20, cursor: null })).toBe(
      "/api/v1/citizen-reports/PA%2F2026%200021/log-entries?limit=20",
    );
    expect(duongDanNhatKyPhieu("PA-2026-0021", { cursor: "" })).toBe(
      "/api/v1/citizen-reports/PA-2026-0021/log-entries",
    );
  });

  it("trang sau mang đúng con trỏ máy chủ phát ra, không `tenant`", async () => {
    const gia = batFetch(
      new Response(JSON.stringify({ items: [DONG], next_cursor: "", has_more: false }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );
    const kq = await layNhatKyPhieu("PA-2026-0021", { limit: 20, cursor: "eyJrIjoi" });
    const duong = String(gia.mock.calls[0]?.[0]);
    expect(duong.startsWith("/api/v1/citizen-reports/PA-2026-0021/log-entries?")).toBe(true);
    const truyVan = new URLSearchParams(duong.slice(duong.indexOf("?") + 1));
    expect(truyVan.get("cursor")).toBe("eyJrIjoi");
    expect(truyVan.get("limit")).toBe("20");
    expect(duong).not.toMatch(/tenant/i);
    expect(gia.mock.calls[0]?.[1]?.method).toBe("GET");
    expect(kq).toEqual({ ok: true, duLieu: { items: [DONG], next_cursor: "", has_more: false } });
  });

  it("404 (mã không có HOẶC lĩnh vực hạn chế): một câu, nguyên văn", async () => {
    batFetch(KHONG_TIM_THAY.clone());
    expect(await layNhatKyPhieu("PA-2026-0021")).toEqual({
      ok: false,
      thongBao: "Không tìm thấy phiếu phản ánh.",
    });
  });

  function batGhiNhatKy(trangThai = 201, than: unknown = DONG) {
    const gia = vi.fn(
      async (_duongDan: string, _tuyChon?: RequestInit) =>
        new Response(JSON.stringify(than), {
          status: trangThai,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", gia);
    return gia;
  }

  it("POST: mong 201, thân ĐÚNG MỘT khoá `note` đã cắt, kèm Idempotency-Key", async () => {
    const gia = batGhiNhatKy();
    const kq = await ghiNhatKyPhieu("PA-2026-0021", "  Đã gọi đội vệ sinh.\nChờ xe rác.  ", "k-nhap");
    expect(kq).toEqual({ ok: true, duLieu: DONG });
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA-2026-0021/log-entries");
    const tuyChon = gia.mock.calls[0]?.[1];
    expect(tuyChon?.method).toBe("POST");
    // Xuống dòng GIỮ NGUYÊN — chỉ cắt hai đầu.
    expect(JSON.parse(String(tuyChon?.body))).toEqual({ note: "Đã gọi đội vệ sinh.\nChờ xe rác." });
    expect(new Headers(tuyChon?.headers).get("Idempotency-Key")).toBe("k-nhap");
  });

  it("gửi lại CÙNG bản nháp: CÙNG khoá — hàm không tự sinh khoá", async () => {
    const gia = batGhiNhatKy(500, { code: "internal", message: "Lỗi máy chủ.", trace_id: "x" });
    await ghiNhatKyPhieu("PA-2026-0021", "Ghi chú", "k-co-dinh");
    await ghiNhatKyPhieu("PA-2026-0021", "Ghi chú", "k-co-dinh");
    const k0 = new Headers(gia.mock.calls[0]?.[1]?.headers).get("Idempotency-Key");
    const k1 = new Headers(gia.mock.calls[1]?.[1]?.headers).get("Idempotency-Key");
    expect(k0).toBe("k-co-dinh");
    expect(k1).toBe(k0);
  });

  it("403 của luật nghiệp vụ: câu của máy chủ ra NGUYÊN VĂN", async () => {
    const cau = "Bạn không được phân công phiếu này và không có quyền xử lý phản ánh của xã.";
    batGhiNhatKy(403, { code: "forbidden", message: cau, trace_id: "x" });
    expect(await ghiNhatKyPhieu("PA-2026-0021", "Ghi chú", "k")).toEqual({ ok: false, thongBao: cau });
  });

  it("200 thay vì 201 là KHÔNG thành công — mong đúng mã của hợp đồng", async () => {
    batGhiNhatKy(200);
    expect((await ghiNhatKyPhieu("PA-2026-0021", "Ghi chú", "k")).ok).toBe(false);
  });
});

/**
 * GHI CHÚ NỘI BỘ của sáu thao tác — `note` chỉ đi khi có chữ.
 *
 * `note: ""` trên dây là một dòng nhật ký với ghi chú rỗng — trông như cán bộ định viết rồi mất chữ.
 */
describe("ghi chú nội bộ trên sáu thao tác", () => {
  function batGhi() {
    const gia = vi.fn(
      async (_duongDan: string, _tuyChon?: RequestInit) =>
        new Response(JSON.stringify({ code: "PA-2026-0021" }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
    );
    vi.stubGlobal("fetch", gia);
    return gia;
  }
  const than = (gia: ReturnType<typeof batGhi>, i = 0) =>
    JSON.parse(String(gia.mock.calls[i]?.[1]?.body)) as Record<string, unknown>;

  const LY_DO = "Nội dung thuộc thẩm quyền ngành điện.";

  it("có chữ: `note` đi kèm, đã cắt khoảng trắng, ở cả năm tuyến có thân", async () => {
    const gia = batGhi();
    await phanLoaiPhieu("PA-1", "rac-thai", "  Đã xem hiện trường.  ");
    await chuyenXuLyPhieu("PA-1", "01JBOPHAN", "CB-00123", "Giao anh B.");
    await dongPhieu("PA-1", "Đã dọn xong.", "Xe rác tới lúc 9h.");
    await khongTiepNhanPhieu("PA-1", LY_DO, "Đã báo lãnh đạo.");
    await chuyenCapTrenPhieu("PA-1", LY_DO, "Công ty điện lực", "Gửi công văn số 12.");

    expect(than(gia, 0)).toEqual({ field: "rac-thai", note: "Đã xem hiện trường." });
    expect(than(gia, 1)).toEqual({ unit: "01JBOPHAN", assignee: "CB-00123", note: "Giao anh B." });
    expect(than(gia, 2)).toEqual({ result: "Đã dọn xong.", note: "Xe rác tới lúc 9h." });
    expect(than(gia, 3)).toEqual({ reason: LY_DO, note: "Đã báo lãnh đạo." });
    expect(than(gia, 4)).toEqual({
      reason: LY_DO,
      receiving_body: "Công ty điện lực",
      note: "Gửi công văn số 12.",
    });
  });

  it("trống hoặc toàn khoảng trắng: KHÔNG có khoá `note` nào", async () => {
    const gia = batGhi();
    for (const rong of [undefined, "", "   \n  "]) {
      await phanLoaiPhieu("PA-1", "rac-thai", rong);
      await chuyenXuLyPhieu("PA-1", "01JBOPHAN", undefined, rong);
      await dongPhieu("PA-1", "Đã dọn xong.", rong);
      await khongTiepNhanPhieu("PA-1", LY_DO, rong);
      await chuyenCapTrenPhieu("PA-1", LY_DO, "Công an xã", rong);
    }
    for (let i = 0; i < gia.mock.calls.length; i++) {
      expect(Object.keys(than(gia, i)), String(i)).not.toContain("note");
    }
    expect(gia.mock.calls.length).toBe(15);
  });

  it("tiến trạng thái có ghi chú: thân `{note}` DUY NHẤT, không trạng thái đích nào", async () => {
    const gia = batGhi();
    await tienTrangThaiPhieu("PA-1", "  Đã tới hiện trường.  ");
    const tuyChon = gia.mock.calls[0]?.[1];
    expect(JSON.parse(String(tuyChon?.body))).toEqual({ note: "Đã tới hiện trường." });
    expect(new Headers(tuyChon?.headers).get("Content-Type")).toBe("application/json");
  });

  it("tiến trạng thái KHÔNG ghi chú: vẫn KHÔNG thân, KHÔNG `Content-Type`", async () => {
    const gia = batGhi();
    await tienTrangThaiPhieu("PA-1", "   ");
    await tienTrangThaiPhieu("PA-1");
    for (const i of [0, 1]) {
      expect(gia.mock.calls[i]?.[1]?.body).toBeUndefined();
      expect(gia.mock.calls[i]?.[1]?.headers).toEqual({});
    }
  });
});

describe("kiểm duyệt công khai — PUT …/publication", () => {
  function batPut(status = 200, than: unknown = { code: "PA-2026-0021", publication_status: "cong-khai" }) {
    const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) =>
      new Response(JSON.stringify(than), {
        status,
        headers: { "Content-Type": "application/json" },
      }),
    );
    vi.stubGlobal("fetch", gia);
    return gia;
  }

  it("PUT the right route, body EXACTLY `{status}`, path code encoded, no Idempotency-Key", async () => {
    const gia = batPut();
    const kq = await setPetitionPublication("PA/2026 0021", "cong-khai");
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/citizen-reports/PA%2F2026%200021/publication");
    const tuyChon = gia.mock.calls[0]?.[1];
    expect(tuyChon?.method).toBe("PUT");
    expect(JSON.parse(String(tuyChon?.body))).toEqual({ status: "cong-khai" });
    expect(new Headers(tuyChon?.headers).has("Idempotency-Key")).toBe(false);
    // 200 carries the petition as it now stands — the screen re-renders from it.
    expect(kq.ok && kq.duLieu.publication_status).toBe("cong-khai");
  });

  it("hide sends `an`", async () => {
    const gia = batPut(200, { code: "PA-2026-0021", publication_status: "an" });
    await setPetitionPublication("PA-2026-0021", "an");
    expect(JSON.parse(String(gia.mock.calls[0]?.[1]?.body))).toEqual({ status: "an" });
  });

  it("409 `never_public`: the commune's configured sentence reaches the screen VERBATIM", async () => {
    // `feedback.never_public` is a system message the commune may reword; the server sends the
    // commune's sentence. A reworded one here, so no client copy of the default could pass this.
    const cau = "Xã không công khai phản ánh liên quan đến cán bộ, công chức.";
    batPut(409, { code: "never_public", message: cau, trace_id: "01JTRACE" });
    const kq = await setPetitionPublication("PA-2026-0021", "cong-khai");
    expect(kq.ok).toBe(false);
    expect(kq.ok === false && kq.thongBao).toBe(cau);
  });

  it("403 without `feedback.assign`: the server's sentence, not a client-side one", async () => {
    const cau = "Tài khoản không có quyền thực hiện thao tác này.";
    batPut(403, { code: "forbidden", message: cau, trace_id: "01JTRACE" });
    const kq = await setPetitionPublication("PA-2026-0021", "an");
    expect(kq.ok === false && kq.thongBao).toBe(cau);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * 02/10/2026 — staff intake, log attachments, verification photos, the close gate
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

type Call = [string, RequestInit | undefined];

function answer(status: number, body: unknown) {
  const fake = vi.fn(async (_path: string, _init?: RequestInit) =>
    new Response(JSON.stringify(body), { status, headers: { "Content-Type": "application/json" } }),
  );
  vi.stubGlobal("fetch", fake);
  return fake;
}

function call(fake: ReturnType<typeof answer>, i = 0): Call {
  return fake.mock.calls[i] as unknown as Call;
}

/** The commune's own origin, no cache, no tenant in any form (rule 1, forbidden #2). */
function expectCommon(path: string, init: RequestInit | undefined) {
  expect(path.startsWith("/api/v1/")).toBe(true);
  expect(path).not.toMatch(/tenant/i);
  expect(init?.credentials).toBe("same-origin");
  expect(init?.cache).toBe("no-store");
  expect(String(init?.body ?? "")).not.toMatch(/tenant/i);
}

const INTAKE: StaffIntakeInput = {
  field: "rac-thai",
  content: "  Rác tồn đọng ở đầu ngõ.  ",
  address: "",
  reporterName: "",
  reporterPhone: "",
  anonymous: false,
  clockFrom: "",
};

describe("GET /api/v1/citizen-report-intake-fields", () => {
  it("path, GET, no-store, no tenant; the list verbatim", async () => {
    const items = [{ code: "rac-thai", label: "Rác thải – Vệ sinh môi trường", icon: null, tone: null }];
    const fake = answer(200, { items });
    const r = await listIntakeFields();
    const [path, init] = call(fake);
    expect(path).toBe("/api/v1/citizen-report-intake-fields");
    expect(init?.method).toBe("GET");
    expectCommon(path, init);
    expect(r).toEqual({ ok: true, duLieu: { items } });
  });

  it("503 `field_catalogue_unavailable`: the server's sentence, never a built-in list", async () => {
    const cau = "Chưa đọc được danh mục lĩnh vực. Vui lòng thử lại sau ít phút.";
    answer(503, { code: "field_catalogue_unavailable", message: cau, trace_id: "01JT" });
    expect(await listIntakeFields()).toEqual({ ok: false, thongBao: cau });
  });
});

describe("POST /api/v1/citizen-reports — staff intake", () => {
  it("path, POST, Idempotency-Key, no-store, no tenant; the minimal body", async () => {
    const fake = answer(201, { code: "PA-2026-0042" });
    const r = await bookStaffIntake(INTAKE, "khoa-1");
    const [path, init] = call(fake);
    expect(path).toBe("/api/v1/citizen-reports");
    expect(init?.method).toBe("POST");
    expectCommon(path, init);
    expect(new Headers(init?.headers).get("Idempotency-Key")).toBe("khoa-1");
    // Blank optionals ABSENT, content trimmed; no channel, no citizen, no hamlet, no deadline.
    expect(JSON.parse(String(init?.body))).toEqual({ field: "rac-thai", content: "Rác tồn đọng ở đầu ngõ." });
    expect(r).toEqual({ ok: true, duLieu: { code: "PA-2026-0042" } });
  });

  it("every optional field when set — and nothing the server decides", () => {
    const body = staffIntakeBody({
      field: "an-ninh",
      content: "Đánh nhau ở chợ.",
      address: " Chợ thôn Hà Lam ",
      reporterName: " Nguyễn Văn Hùng ",
      reporterPhone: "0900000000",
      anonymous: true,
      clockFrom: "2026-10-02T08:30:00+07:00",
    });
    expect(body).toEqual({
      field: "an-ninh",
      content: "Đánh nhau ở chợ.",
      address: "Chợ thôn Hà Lam",
      reporter_name: "Nguyễn Văn Hùng",
      reporter_phone: "0900000000",
      anonymous: true,
      clock_from: "2026-10-02T08:30:00+07:00",
    });
    for (const k of ["channel", "citizen_id", "cong_dan_id", "code", "status", "hamlet", "thon_id", "acknowledge_due", "resolve_due", "lat", "lng", "linh_vuc"]) {
      expect(Object.keys(body), k).not.toContain(k);
    }
  });

  it("`anonymous` unticked is ABSENT, not `false`", () => {
    expect(Object.keys(staffIntakeBody(INTAKE))).not.toContain("anonymous");
  });

  it("a replay of the same key answers `{code, replayed}` — the code is still read", async () => {
    answer(201, { code: "PA-2026-0042", replayed: true });
    expect(await bookStaffIntake(INTAKE, "khoa-1")).toEqual({
      ok: true,
      duLieu: { code: "PA-2026-0042", replayed: true },
    });
  });

  it.each([
    [400, "clock_from_out_of_range", "Thời điểm người dân phản ánh không được sớm hơn 7 ngày trước lúc vào sổ, và không được muộn hơn lúc vào sổ. Hãy kiểm tra lại ô \"Dân phản ánh lúc\"."],
    [503, "intake_not_configured", "Chưa ấn định được thời hạn xử lý theo cấu hình của xã nên phiếu CHƯA được vào sổ. Hãy kiểm tra bảng thời hạn xử lý và lịch làm việc ở màn hình Cấu hình."],
    [503, "field_catalogue_unavailable", "Chưa kiểm tra được lĩnh vực nên phiếu CHƯA được vào sổ. Vui lòng thử lại sau ít phút."],
    [409, "request_in_progress", "Yêu cầu trước đó với cùng mã này đang được xử lý. Vui lòng thử lại sau giây lát."],
  ])("%i `%s`: the server's sentence VERBATIM", async (status, code, message) => {
    answer(status, { code, message, trace_id: "01JT" });
    expect(await bookStaffIntake(INTAKE, "k")).toEqual({ ok: false, thongBao: message });
  });
});

describe("POST …/log-entries — files ride on the note", () => {
  it("stored ids go as `attachments`", async () => {
    const fake = answer(201, {});
    await ghiNhatKyPhieu("PA-1", " Đã tới hiện trường. ", "k", ["01JFILE1", "01JFILE2"]);
    expect(JSON.parse(String(call(fake)[1]?.body))).toEqual({
      note: "Đã tới hiện trường.",
      attachments: ["01JFILE1", "01JFILE2"],
    });
  });

  it("no file: `attachments` ABSENT — the request it always was", async () => {
    const fake = answer(201, {});
    await ghiNhatKyPhieu("PA-1", "Ghi chú.", "k");
    expect(Object.keys(JSON.parse(String(call(fake)[1]?.body)))).toEqual(["note"]);
  });
});

describe("log attachments — declare · complete · download", () => {
  const FORM = { url: "https://kho.example.test/tmp", fields: { key: "t_01J/x", policy: "p" }, expires_at: "2026-10-02T03:15:00Z" };
  const FILE = { id: "01JFILE1", file_name: "bien-ban.pdf", mime_type: "application/pdf", size_bytes: 1200, status: "pending" };

  it("declare: path (code encoded), POST, Idempotency-Key, field-by-field body, 201 kept with its status", async () => {
    const fake = answer(201, { attachment: FILE, upload: FORM });
    const r = await requestLogAttachmentUpload(
      "PA/1",
      { file_name: "bien-ban.pdf", content_type: "application/pdf", size: 1200, extra: "x" } as never,
      "k-1",
    );
    const [path, init] = call(fake);
    expect(path).toBe("/api/v1/citizen-reports/PA%2F1/log-attachments");
    expect(init?.method).toBe("POST");
    expectCommon(path, init);
    expect(new Headers(init?.headers).get("Idempotency-Key")).toBe("k-1");
    expect(JSON.parse(String(init?.body))).toEqual({ file_name: "bien-ban.pdf", content_type: "application/pdf", size: 1200 });
    expect(r.ok).toBe(true);
  });

  it("a replayed 201 without the form is a refusal, said in one sentence", async () => {
    answer(201, { code: "01JFILE1", replayed: true });
    const r = await requestLogAttachmentUpload("PA-1", { file_name: "a.pdf", content_type: "application/pdf", size: 1 }, "k");
    expect(r).toEqual({ ok: false, status: 201, message: UPLOAD_FORM_MISSING });
  });

  it("completion: path, POST, status kept (422 / 503)", async () => {
    const fake = answer(422, { code: "attachment_rejected", message: "Tệp bị từ chối vì phát hiện mã độc.", trace_id: "t" });
    const r = await completeLogAttachment("PA-1", "01J/FILE");
    const [path, init] = call(fake);
    expect(path).toBe("/api/v1/citizen-reports/PA-1/log-attachments/01J%2FFILE/completion");
    expect(init?.method).toBe("POST");
    expectCommon(path, init);
    expect(r).toEqual({ ok: false, status: 422, message: "Tệp bị từ chối vì phát hiện mã độc." });
  });

  it("download: path, GET, no-store; the signed link returned as is", async () => {
    const fake = answer(200, { url: "https://kho.example.test/f?sig=1", expires_at: "2026-10-02T03:15:00Z" });
    const r = await logAttachmentDownloadLink("PA-1", "01JFILE1");
    const [path, init] = call(fake);
    expect(path).toBe("/api/v1/citizen-reports/PA-1/log-attachments/01JFILE1/download");
    expect(init?.method).toBe("GET");
    expectCommon(path, init);
    expect(r.ok && r.data.url).toBe("https://kho.example.test/f?sig=1");
  });

  it("no answer at all is status 0", async () => {
    vi.stubGlobal("fetch", vi.fn(async () => Promise.reject(new TypeError("network"))));
    const r = await logAttachmentDownloadLink("PA-1", "x");
    expect(r.ok === false && r.status).toBe(0);
  });
});

describe("verification photos — list · declare · complete", () => {
  it("list: path, GET, no-store, no tenant", async () => {
    const fake = answer(200, { items: [] });
    const r = await listVerificationPhotos("PA 1");
    const [path, init] = call(fake);
    expect(path).toBe("/api/v1/citizen-reports/PA%201/verification-photos");
    expect(init?.method).toBe("GET");
    expectCommon(path, init);
    expect(r).toEqual({ ok: true, data: { items: [] } });
  });

  it("list 503 keeps its status (the column's own sentence)", async () => {
    answer(503, { code: "storage_not_configured", message: "x", trace_id: "t" });
    const r = await listVerificationPhotos("PA-1");
    expect(r.ok === false && r.status).toBe(503);
  });

  it("declare: POST, Idempotency-Key, body {content_type, size} only", async () => {
    const fake = answer(201, {
      photo: { id: "01JP", content_type: "image/jpeg", size_bytes: 10, status: "pending", created_at: "x" },
      upload: { url: "https://kho.example.test/tmp", fields: { key: "k" }, expires_at: "x" },
    });
    await requestVerificationPhotoUpload("PA-1", { content_type: "image/jpeg", size: 10, kind: "sau" } as never, "k-9");
    const [path, init] = call(fake);
    expect(path).toBe("/api/v1/citizen-reports/PA-1/verification-photos");
    expect(init?.method).toBe("POST");
    expectCommon(path, init);
    expect(new Headers(init?.headers).get("Idempotency-Key")).toBe("k-9");
    expect(JSON.parse(String(init?.body))).toEqual({ content_type: "image/jpeg", size: 10 });
  });

  it.each([
    ["photo_limit", "Phiếu đã có đủ số ảnh sau xử lý tối đa."],
    ["petition_state", "Phiếu đã kết thúc nên không thêm ảnh sau xử lý được nữa."],
  ])("declare 409 `%s`: status and the server's sentence", async (code, message) => {
    answer(409, { code, message, trace_id: "t" });
    expect(await requestVerificationPhotoUpload("PA-1", { content_type: "image/png", size: 1 }, "k")).toEqual({
      ok: false,
      status: 409,
      message,
    });
  });

  it("completion: path, POST", async () => {
    const fake = answer(200, { id: "01JP", content_type: "image/jpeg", size_bytes: 10, status: "stored", created_at: "x" });
    const r = await completeVerificationPhoto("PA-1", "01JP");
    const [path, init] = call(fake);
    expect(path).toBe("/api/v1/citizen-reports/PA-1/verification-photos/01JP/completion");
    expect(init?.method).toBe("POST");
    expectCommon(path, init);
    expect(r.ok && r.data.status).toBe("stored");
  });
});

describe("POST …/closure — the commune's verification-photo gate", () => {
  it("409 `after_photo_required`: flagged, and the COMMUNE'S sentence verbatim", async () => {
    // A reworded sentence: no client copy of the default could pass this.
    const cau = "Xã yêu cầu có ảnh nghiệm thu trước khi đóng phiếu.";
    answer(409, { code: "after_photo_required", message: cau, trace_id: "t" });
    expect(await dongPhieu("PA-1", "Đã dọn xong.")).toEqual({ ok: false, thongBao: cau, afterPhotoRequired: true });
  });

  it("another 409 is NOT the gate", async () => {
    answer(409, { code: "petition_state", message: "Phiếu đã chuyển trạng thái.", trace_id: "t" });
    expect(await dongPhieu("PA-1", "x")).toEqual({
      ok: false,
      thongBao: "Phiếu đã chuyển trạng thái.",
      afterPhotoRequired: false,
    });
  });

  it("the same code on another status is NOT the gate either", async () => {
    answer(400, { code: "after_photo_required", message: "x", trace_id: "t" });
    const r = await dongPhieu("PA-1", "x");
    expect(r.ok === false && r.afterPhotoRequired).toBe(false);
  });

  it("200: the petition; POST, JSON body, no-store, no Idempotency-Key", async () => {
    const fake = answer(200, { code: "PA-1", status: "da-dong" });
    const r = await dongPhieu("PA-1", "Đã dọn xong.");
    const [, init] = call(fake);
    expect(init?.method).toBe("POST");
    expect(init?.cache).toBe("no-store");
    expect(new Headers(init?.headers).get("Content-Type")).toBe("application/json");
    expect(new Headers(init?.headers).has("Idempotency-Key")).toBe(false);
    expect(r.ok && r.duLieu.status).toBe("da-dong");
  });
});

/* ══════════════════════════════════════════════════════════════════════════════════════════
 * 09/10/2026 — count, points, breakdown, act files, hamlet, log-file removal
 * ══════════════════════════════════════════════════════════════════════════════════════════ */

describe("GET count / points / breakdown", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("count: the list's filters, the total verbatim", async () => {
    const fake = answer(200, { total: 57 });
    const r = await countCitizenReports({ trangThai: "dang-xu-ly", tim: "ngõ" });
    const [path, init] = call(fake);
    expectCommon(path, init);
    expect(path).toBe("/api/v1/citizen-report-counts?status=dang-xu-ly&q=ng%C3%B5");
    expect(r).toEqual({ ok: true, duLieu: { total: 57 } });
  });

  it("points: 422 `too_many_points` is flagged (narrow the filters); any other refusal is not", async () => {
    answer(422, { code: "too_many_points", message: "Quá nhiều điểm. Hãy thu hẹp bộ lọc." });
    expect(await listCitizenReportPoints({})).toEqual({
      ok: false,
      message: "Quá nhiều điểm. Hãy thu hẹp bộ lọc.",
      tooManyPoints: true,
    });
    answer(422, { code: "something_else", message: "Khác." });
    expect((await listCitizenReportPoints({})).ok === false).toBe(true);
    expect(await listCitizenReportPoints({})).toMatchObject({ tooManyPoints: false });
    const fake = answer(200, { items: [{ lat: 15.7, lng: 108.3, status: "dang-xu-ly" }] });
    const ok = await listCitizenReportPoints({ thonID: "01JTHON1" });
    expect(call(fake)[0]).toBe("/api/v1/citizen-report-points?hamlet=01JTHON1");
    expectCommon(...call(fake));
    expect(ok.ok).toBe(true);
  });

  it("breakdown: 503 / 409 come back as the server's sentence verbatim", async () => {
    answer(503, { code: "working_hours_unavailable", message: "Chưa đo được giờ làm việc lúc này." });
    expect(await readCitizenReportBreakdown({ from: "a", to: "b" })).toEqual({
      ok: false,
      thongBao: "Chưa đo được giờ làm việc lúc này.",
    });
  });
});

describe("act bodies — files and the hamlet (ADR 0088)", () => {
  afterEach(() => vi.unstubAllGlobals());
  const body = (fake: ReturnType<typeof answer>) => JSON.parse(String(call(fake)[1]?.body ?? "null")) as unknown;

  it("no file: the body is byte-for-byte what it was (no `attachments` key)", async () => {
    let fake = answer(200, {});
    await phanLoaiPhieu("PA-1", "rac-thai", "");
    expect(body(fake)).toEqual({ field: "rac-thai" });
    fake = answer(200, {});
    await tienTrangThaiPhieu("PA-1", "", []);
    expect(call(fake)[1]?.body).toBeUndefined();
  });

  it("every act carries the STORED ids it was given", async () => {
    const ids = ["01JF1", "01JF2"];
    let fake = answer(200, {});
    await phanLoaiPhieu("PA-1", "rac-thai", "", { attachments: ids });
    expect(body(fake)).toEqual({ field: "rac-thai", attachments: ids });
    fake = answer(200, {});
    await chuyenXuLyPhieu("PA-1", "01JBP", undefined, "", ids);
    expect(body(fake)).toEqual({ unit: "01JBP", attachments: ids });
    fake = answer(200, {});
    await tienTrangThaiPhieu("PA-1", "Đã tới", ids);
    expect(body(fake)).toEqual({ note: "Đã tới", attachments: ids });
    fake = answer(200, {});
    await dongPhieu("PA-1", "Đã dọn.", "", ids);
    expect(body(fake)).toEqual({ result: "Đã dọn.", attachments: ids });
    fake = answer(200, {});
    await khongTiepNhanPhieu("PA-1", "Không thuộc thẩm quyền xã.", "", ids);
    expect(body(fake)).toEqual({ reason: "Không thuộc thẩm quyền xã.", attachments: ids });
    fake = answer(200, {});
    await chuyenCapTrenPhieu("PA-1", "Thuộc thẩm quyền tỉnh.", "Sở GTVT", "", ids);
    expect(body(fake)).toEqual({ reason: "Thuộc thẩm quyền tỉnh.", receiving_body: "Sở GTVT", attachments: ids });
  });

  it("classification: `residential_unit_id` only when picked (absent keeps the stored one)", async () => {
    let fake = answer(200, {});
    await phanLoaiPhieu("PA-1", "rac-thai", "", { residentialUnitId: "01JTHON1" });
    expect(body(fake)).toEqual({ field: "rac-thai", residential_unit_id: "01JTHON1" });
    fake = answer(200, {});
    await phanLoaiPhieu("PA-1", "rac-thai", "", { residentialUnitId: "" });
    expect(body(fake)).toEqual({ field: "rac-thai" });
  });

  it("staff intake: `residential_unit_id` only when picked", () => {
    expect(staffIntakeBody({ ...INTAKE, residentialUnitId: "01JTHON1" }).residential_unit_id).toBe("01JTHON1");
    expect("residential_unit_id" in staffIntakeBody({ ...INTAKE, residentialUnitId: "" })).toBe(false);
    expect("residential_unit_id" in staffIntakeBody(INTAKE)).toBe(false);
  });
});

describe("DELETE …/log-attachments/{id} — soft removal, reason required", () => {
  afterEach(() => vi.unstubAllGlobals());

  it("path, DELETE, the trimmed reason in the BODY; 204 is success", async () => {
    const fake = vi.fn(async (_p: string, _i?: RequestInit) => new Response(null, { status: 204 }));
    vi.stubGlobal("fetch", fake);
    const r = await removeLogAttachment("PA 1", "01J/F", "  Tải nhầm tệp  ");
    const [path, init] = fake.mock.calls[0] as unknown as Call;
    expectCommon(path, init);
    expect(path).toBe("/api/v1/citizen-reports/PA%201/log-attachments/01J%2FF");
    expect(init?.method).toBe("DELETE");
    expect(JSON.parse(String(init?.body))).toEqual({ reason: "Tải nhầm tệp" });
    expect(r).toEqual({ ok: true, duLieu: undefined });
  });

  it("409 `legal_hold` / 403: the server's sentence", async () => {
    answer(409, { code: "legal_hold", message: "Tệp đang bị phong toả." });
    expect(await removeLogAttachment("PA-1", "01JF", "x")).toEqual({ ok: false, thongBao: "Tệp đang bị phong toả." });
  });
});
