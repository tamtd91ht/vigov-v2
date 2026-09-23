import { afterEach, describe, expect, it, vi } from "vitest";

import {
  deNghiLuiHan,
  doiTrangThaiNhiemVu,
  duongDanSoNhiemVu,
  layNhiemVu,
  quyetDinhLuiHan,
  taoNhiemVu,
  xoaNhiemVu,
} from "./nhiem-vu";

/**
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * NHÓM CHỊU LỰC CỦA TỆP NÀY LÀ NHÓM TÊN THAM SỐ, và nó canh một chỗ `tsc` KHÔNG canh được.
 *
 * `petitions_get_tasks["truyVan"]` sinh ra một đối tượng RỖNG — hợp đồng chưa khai `@query` trên
 * tuyến danh sách — trong khi handler thật đọc mười cái tên. Gõ sai một tên ở `themLocVaoTruyVan`
 * thì trình biên dịch im lặng, máy chủ bỏ qua tham số ấy và trả về **cả quyển sổ**, và màn hình
 * trông hoàn toàn bình thường: cán bộ tin mình đang xem việc của riêng mình.
 *
 * Nên mười cái tên ấy được đọc lại từng cái một ở đây. Khi hợp đồng có `parameters`, nhóm này là
 * chỗ đầu tiên phải đối chiếu lại.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */

function batFetch(tra: Response) {
  // Tham số được khai rõ để `mock.calls[0][0]` có kiểu — một `vi.fn(async () => …)` không tham số
  // làm TypeScript coi danh sách đối số là tuple rỗng.
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra);
  vi.stubGlobal("fetch", gia);
  return gia;
}

function than(gia: ReturnType<typeof batFetch>): unknown {
  const raw = gia.mock.calls[0]?.[1]?.body;
  return typeof raw === "string" ? JSON.parse(raw) : undefined;
}

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

const OK_JSON = () =>
  new Response("{}", { status: 200, headers: { "Content-Type": "application/json" } });

describe("MƯỜI TÊN THAM SỐ — đọc lại từng cái một", () => {
  it("mỗi bộ lọc đi vào ĐÚNG tên máy chủ đọc", () => {
    const duong = duongDanSoNhiemVu({
      phamVi: "mine",
      trangThai: "dang-thuc-hien",
      nguonGiao: "ket-luan-hop",
      loai: "theo-van-ban",
      khoi: "khoi-uy-ban",
      mucUuTien: "khan",
      boPhanID: "01JBOPHAN",
      nguoiThucHienMa: "CB-2026-7K3M9Q",
      tim: "báo cáo tổng kết",
      chiTreHan: true,
      limit: 20,
      cursor: "01JCONTRO",
    });
    const q = new URL(duong, "https://xa.example").searchParams;

    expect(q.get("scope")).toBe("mine");
    expect(q.get("status")).toBe("dang-thuc-hien");
    expect(q.get("source")).toBe("ket-luan-hop");
    expect(q.get("type")).toBe("theo-van-ban");
    expect(q.get("bloc")).toBe("khoi-uy-ban");
    expect(q.get("priority")).toBe("khan");
    expect(q.get("unit")).toBe("01JBOPHAN");
    expect(q.get("assignee")).toBe("CB-2026-7K3M9Q");
    expect(q.get("q")).toBe("báo cáo tổng kết");
    expect(q.get("late")).toBe("true");
    expect(q.get("limit")).toBe("20");
    expect(q.get("cursor")).toBe("01JCONTRO");
  });

  it("KHÔNG GỬI `related` và KHÔNG GỬI `soon` — hai tham số máy chủ TỪ CHỐI bằng 400", () => {
    // Cả hai không phải "lọc không ăn": `errPhamViChuaHoTro` và `errLocSapDenHanChuaCo` biến
    // quyển sổ thành một trang lỗi. Kiểu `phamVi` chỉ nhận `all | mine`, nên bài này canh phần
    // `tsc` không canh: không có đường nào trong tệp đặt hai tên ấy vào truy vấn.
    const duong = duongDanSoNhiemVu({ phamVi: "all" });
    expect(duong).not.toContain("related");
    expect(duong).not.toContain("soon");
  });

  it("`che_do_xem` KHÔNG đi lên máy chủ — Kanban / Danh sách chọn CÁCH BÀY, không đổi dòng nào", () => {
    expect(duongDanSoNhiemVu({ trangThai: "moi-giao" })).not.toContain("che_do_xem");
  });

  it("`scope=all` KHÔNG gửi tham số nào — mặc định của máy chủ, ít một chỗ gõ sai", () => {
    expect(duongDanSoNhiemVu({ phamVi: "all" })).toBe("/api/v1/tasks");
    expect(duongDanSoNhiemVu()).toBe("/api/v1/tasks");
  });

  it("ô tick bỏ trống thì `late` VẮNG MẶT HẲN, không gửi `late=false`", () => {
    // Máy chủ chỉ nhận đúng chuỗi `true` (`errLocTreHanNhiemVuKhongHopLe`); `false` là 400, tức
    // ô bỏ tích sẽ làm hỏng quyển sổ thay vì bỏ lọc.
    expect(duongDanSoNhiemVu({ chiTreHan: false })).not.toContain("late");
  });

  it("con trỏ rỗng KHÔNG thành `cursor=` — 400 ngay lần mở màn hình đầu tiên", () => {
    expect(duongDanSoNhiemVu({ cursor: null })).not.toContain("cursor");
    expect(duongDanSoNhiemVu({ cursor: "" })).not.toContain("cursor");
  });

  it("đường dẫn tương đối, không host, không `tenant_id`", () => {
    const duong = duongDanSoNhiemVu({ trangThai: "moi-giao" });
    expect(duong.startsWith("/api/")).toBe(true);
    expect(duong).not.toMatch(/tenant/i);
  });
});

describe("mã nhiệm vụ đi vào đường dẫn", () => {
  it("mã hoá thay vì ghép thẳng — §7.1 cho phép tự nhập mã", async () => {
    const gia = batFetch(OK_JSON());
    await layNhiemVu("NV 19/2026");
    expect(gia.mock.calls[0]?.[0]).toBe("/api/v1/tasks/NV%2019%2F2026");
  });

  it("id đề nghị cũng được mã hoá, và cả hai chỗ thay đúng khuôn của hợp đồng", async () => {
    const gia = batFetch(OK_JSON());
    await quyetDinhLuiHan("NV19", "01JDENGHI", true);
    expect(gia.mock.calls[0]?.[0]).toBe(
      "/api/v1/tasks/NV19/extensions/01JDENGHI/decision",
    );
  });
});

describe("thân của sáu tuyến ghi", () => {
  it("tạo nhiệm vụ: BA trường bắt buộc, trường rỗng VẮNG MẶT HẲN, kèm `Idempotency-Key`", async () => {
    const gia = batFetch(
      new Response("{}", { status: 201, headers: { "Content-Type": "application/json" } }),
    );

    await taoNhiemVu(
      { auto_code: true, type: "theo-van-ban", title: "Báo cáo tổng kết", bloc: "", note: "" } as never,
      "khoa-gia-cua-bai-kiem",
    );

    expect(than(gia)).toEqual({
      auto_code: true,
      type: "theo-van-ban",
      title: "Báo cáo tổng kết",
    });

    const dau = gia.mock.calls[0]?.[1]?.headers as Record<string, string>;
    expect(dau["Idempotency-Key"]).toBe("khoa-gia-cua-bai-kiem");
  });

  it("tạo nhiệm vụ: DỰNG TỪNG TRƯỜNG — `status` và `created_by` KHÔNG lọt lên máy chủ", async () => {
    // Ba trường ấy là những thứ hợp đồng CỐ Ý không nhận: trạng thái đầu đời là `moi-giao` và
    // không đâu khác; tác giả là chủ thể của phiên, và một yêu cầu tự khai được tác giả là một
    // yêu cầu giả được vết kiểm toán (luật 6).
    const gia = batFetch(
      new Response("{}", { status: 201, headers: { "Content-Type": "application/json" } }),
    );

    await taoNhiemVu(
      {
        auto_code: true,
        type: "co-ban",
        title: "Rà soát tuyến đường",
        status: "hoan-thanh",
        created_by: "CB-GIA",
        original_due_at: "2020-01-01T00:00:00+07:00",
      } as never,
      "khoa-gia",
    );

    const gui = than(gia) as Record<string, unknown>;
    expect(gui).not.toHaveProperty("status");
    expect(gui).not.toHaveProperty("created_by");
    expect(gui).not.toHaveProperty("original_due_at");
  });

  it("đổi trạng thái: TRẠNG THÁI ĐÍCH ĐI TRÊN DÂY, ghi chú rỗng thì vắng mặt", async () => {
    const gia = batFetch(OK_JSON());
    await doiTrangThaiNhiemVu("NV19", "da-tiep-nhan", "");
    expect(than(gia)).toEqual({ status: "da-tiep-nhan" });

    vi.unstubAllGlobals();
    const gia2 = batFetch(OK_JSON());
    await doiTrangThaiNhiemVu("NV19", "hoan-thanh", "đã nghiệm thu");
    expect(than(gia2)).toEqual({ status: "hoan-thanh", note: "đã nghiệm thu" });
  });

  it("xoá: DELETE có THÂN mang lý do, và 204 không thân vẫn là thành công", async () => {
    // Lý do bắt buộc (luật 7, bất biến 1). Đưa nó vào query string sẽ đẩy chữ tự do về một hồ sơ
    // của cơ quan nhà nước vào mọi log truy cập.
    const gia = batFetch(new Response(null, { status: 204 }));
    const kq = await xoaNhiemVu("NV19", "trùng với NV18");

    expect(gia.mock.calls[0]?.[1]?.method).toBe("DELETE");
    expect(than(gia)).toEqual({ reason: "trùng với NV18" });
    // Một hàm luôn gọi `.json()` sẽ biến lần xoá thành công này thành "không đọc được".
    expect(kq.ok).toBe(true);
  });

  it("đề nghị lùi hạn: hai trường, 201", async () => {
    const gia = batFetch(
      new Response("{}", { status: 201, headers: { "Content-Type": "application/json" } }),
    );
    await deNghiLuiHan("NV19", "2026-12-20T23:59:59+07:00", "chờ ý kiến cấp trên");
    expect(than(gia)).toEqual({
      new_due_at: "2026-12-20T23:59:59+07:00",
      reason: "chờ ý kiến cấp trên",
    });
  });

  it("quyết định lùi hạn: ĐÚNG HAI GIÁ TRỊ `approve` / `reject`", async () => {
    // Một `status` trên dây sẽ cho client gửi `cho-duyet` và ghi một quyết định không quyết định
    // gì, hoặc bịa ra mã thứ tư.
    const gia = batFetch(OK_JSON());
    await quyetDinhLuiHan("NV19", "01JDENGHI", true);
    expect(than(gia)).toEqual({ decision: "approve" });

    vi.unstubAllGlobals();
    const gia2 = batFetch(OK_JSON());
    await quyetDinhLuiHan("NV19", "01JDENGHI", false, "không đủ lý do");
    expect(than(gia2)).toEqual({ decision: "reject", note: "không đủ lý do" });
  });
});

describe("câu từ chối của máy chủ đi NGUYÊN VĂN ra ngoài", () => {
  it("409 hoàn thành cha còn con: DANH SÁCH MÃ giữ nguyên, không bị nuốt thành 'có lỗi xảy ra'", async () => {
    // Danh sách mã là TOÀN BỘ phần có ích của câu này. Viết lại nó ở client là dựng bản sao thứ
    // hai của một quy tắc nghiệp vụ, và bản sao ấy trôi mà không bài kiểm nào đỏ.
    const cau =
      "còn 3 việc con (NV20, NV21, NV22) — hoàn thành hết việc con rồi mới hoàn thành việc cha";
    batFetch(
      new Response(JSON.stringify({ code: "task_tree", message: cau, trace_id: "01JTRACE" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      }),
    );

    expect(await doiTrangThaiNhiemVu("NV19", "hoan-thanh")).toEqual({
      ok: false,
      thongBao: cau,
    });
  });

  it("409 xoá cha còn con: CON SỐ giữ nguyên", async () => {
    const cau = "còn 3 việc con chưa xoá — xử lý hoặc xoá các việc con trước";
    batFetch(
      new Response(JSON.stringify({ code: "task_tree", message: cau, trace_id: "01JTRACE" }), {
        status: 409,
        headers: { "Content-Type": "application/json" },
      }),
    );

    expect(await xoaNhiemVu("NV19", "nhập trùng")).toEqual({ ok: false, thongBao: cau });
  });

  it("403 ADR 0038 — không phải lãnh đạo giao việc: câu của máy chủ ra thẳng màn hình", async () => {
    const cau = "Nhiệm vụ này chưa ghi lãnh đạo giao việc nên chưa ai duyệt được đề nghị lùi hạn.";
    batFetch(
      new Response(JSON.stringify({ code: "forbidden", message: cau, trace_id: "01JTRACE" }), {
        status: 403,
        headers: { "Content-Type": "application/json" },
      }),
    );

    expect(await quyetDinhLuiHan("NV19", "01JDENGHI", true)).toEqual({
      ok: false,
      thongBao: cau,
    });
  });

  it("404 KHÔNG được dựng lại sự phân biệt ba nguyên nhân", async () => {
    // Mã không tồn tại · mã của xã khác · nhiệm vụ đã xoá mềm. Phân biệt được chúng là nói cho
    // người đang thử mã biết những số nào tồn tại trong một quyển sổ họ không đọc được.
    const traLoi = () =>
      new Response(
        JSON.stringify({
          code: "not_found",
          message: "Không tìm thấy nhiệm vụ.",
          trace_id: "01JTRACE",
        }),
        { status: 404, headers: { "Content-Type": "application/json" } },
      );

    batFetch(traLoi());
    const maBia = await layNhiemVu("NV99");
    vi.unstubAllGlobals();
    batFetch(traLoi());
    const daXoaMem = await layNhiemVu("NV19");

    expect(maBia).toEqual(daXoaMem);
    expect(maBia).toEqual({ ok: false, thongBao: "Không tìm thấy nhiệm vụ." });
  });
});
