import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhSachCaLamBuRa,
  identity_danhSachCaLamViecRa,
  identity_danhSachNgayNghiLeRa,
  identity_phienHienTaiRa,
} from "@/lib/api/schema.gen";

import { quyetDinhGhiThoiHan } from "./quyen-tab";
import {
  CalendarForm,
  EMPTY_CALENDAR_DRAFT,
  WorkingCalendarView,
  calendarGroupOf,
  type CalendarActions,
  type CalendarData,
} from "./working-calendar-tab";

/**
 * Tab "Lịch làm việc" (ADR 0079 D1/D2) — the three calendar tables that moved out of "Thời hạn xử lý",
 * with the checks that moved with them (from `tab-thoi-han-xu-ly.test.tsx`):
 *
 * 1. THE "NOT FINISHED" BLOCK SHOWS WHEN THE WEEK IS EMPTY, AND SAYS THE CONSEQUENCE — a commune that
 *    seeded its deadlines but not its hours still receives nothing (`ResolveDeadlines` refuses).
 * 2. AND IS ABSENT WHEN THE WEEK HAS ROWS — the negative half carries the weight.
 * 3. THE TẾT / GIỖ TỔ / 02/9 SENTENCE STANDS BESIDE THE HOLIDAY SEED BUTTON — the route seeds only four
 *    fixed solar days, and `seeded: 4` reads as "done".
 * 4. THE GATE WRAPS THE WRITES, NOT THE TABLES — the three reads are any-authenticated.
 * 5. DELETE KEEPS ITS REASON STEP (rule 7) and shows the server's sentence as written.
 */

const DO_NOTHING: CalendarActions = {
  seedWeek: () => {},
  seedHolidays: () => {},
  addShift: () => {},
  editShift: () => {},
  deleteShift: () => {},
  addHoliday: () => {},
  editHoliday: () => {},
  deleteHoliday: () => {},
  addSwapDay: () => {},
  editSwapDay: () => {},
  deleteSwapDay: () => {},
};

function ok<T>(duLieu: T): KetQua<T> {
  return { ok: true, duLieu };
}

function sessionWith(permissions: string[]): KetQua<identity_phienHienTaiRa> {
  return ok<identity_phienHienTaiRa>({
    sid: "01J000000000000000000SID",
    expires_at: "2026-09-26T12:00:00Z",
    staff: { code: "CB001", full_name: "Cán bộ thử", position: "Chuyên viên" },
    role: null,
    permissions,
    must_change_password: false,
  });
}

const SHIFT = { id: "01J00000000000000000000CA", weekday: 1, start: "07:30:00", end: "11:30:00", note: "Buổi sáng" };
const WEEK_WITH_SHIFT = ok<identity_danhSachCaLamViecRa>({ items: [SHIFT], problems: [] });
const WEEK_EMPTY = ok<identity_danhSachCaLamViecRa>({ items: [], problems: [] });
const HOLIDAY = { id: "01J0000000000000000000NGH", date: "2026-09-02", name: "Quốc khánh" };
const HOLIDAYS_ONE = ok<identity_danhSachNgayNghiLeRa>({ items: [HOLIDAY] });
const HOLIDAYS_EMPTY = ok<identity_danhSachNgayNghiLeRa>({ items: [] });
const SWAP_EMPTY = ok<identity_danhSachCaLamBuRa>({ items: [], problems: [] });

function render(data: Partial<CalendarData> = {}, more: { canWrite?: boolean; outsideFormError?: string } = {}) {
  return renderToStaticMarkup(
    <WorkingCalendarView
      data={{ week: WEEK_WITH_SHIFT, holidays: HOLIDAYS_ONE, swapDays: SWAP_EMPTY, ...data }}
      year={2026}
      baseYear={2026}
      setYear={() => {}}
      canWrite={more.canWrite ?? true}
      actions={DO_NOTHING}
      form={null}
      formGroup={null}
      outsideFormError={more.outsideFormError ?? ""}
      busy={false}
    />,
  );
}

describe("khối 'đơn vị chưa khai xong' ở tab Lịch làm việc", () => {
  it("giờ làm việc RỖNG → khối có mặt, nói ra hậu quả, mang nút gieo tuần", () => {
    // `&&` in place of `||` in `khoiCanhBao` would make the most dangerous silence: a commune that
    // seeded its deadlines believes it is done while `ResolveDeadlines` still refuses.
    const html = render({ week: WEEK_EMPTY });

    expect(html).toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).toContain("Giờ làm việc trong tuần đang trống");
    expect(html).toContain("chưa nhận được phản ánh của người dân");
    expect(html.split("Gieo giờ làm việc mặc định").length - 1).toBe(1);
    // The SLA half belongs to "Thời hạn xử lý": never claimed here, never a seed button for it.
    expect(html).not.toContain("Bảng Thời hạn xử lý đang trống");
    expect(html).not.toContain("Gieo thời hạn mặc định");
  });

  it("lịch tuần đã có ca → khối VẮNG MẶT; nút gieo vá lại nằm ở đầu bảng", () => {
    const html = render();

    expect(html).not.toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html.split("Gieo giờ làm việc mặc định").length - 1).toBe(1);
  });

  it("chưa đọc xong → khối VẮNG MẶT, và trang nói đang tải", () => {
    const html = render({ week: null });

    expect(html).not.toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).toContain("Đang tải giờ làm việc…");
  });

  it("đọc hỏng → KHÔNG khẳng định xã chưa khai; hiện NGUYÊN câu máy chủ", () => {
    const html = render({ week: { ok: false, thongBao: "Phiên làm việc đã hết hạn." } });

    expect(html).not.toContain("Đơn vị chưa khai xong phần bắt buộc");
    expect(html).toContain("Phiên làm việc đã hết hạn.");
  });
});

describe("gieo ngày nghỉ lễ nợ người bấm một câu", () => {
  it("câu Tết / Giỗ Tổ / ngày liền kề còn thiếu đứng sẵn, cạnh nút gieo — sự thật thường trực, không phải lời đáp", () => {
    const html = render();

    expect(html).toContain("Gieo ngày nghỉ lễ theo dương lịch");
    expect(html).toContain("Tết Nguyên đán, Giỗ Tổ Hùng Vương và ngày liền kề 02/9 CHƯA có");
  });
});

describe("cổng quyền bọc phần GHI, không bọc bảng", () => {
  it("thiếu quyền: bảng vẫn hiện đủ dòng, chỉ nút ghi vắng", () => {
    // Hiding the tables is the UI refusing what the server serves: the three reads are any-authenticated.
    const html = render({}, { canWrite: false });

    expect(html).toContain("Buổi sáng");
    expect(html).toContain("Quốc khánh");
    expect(html).not.toContain('title="Sửa');
    expect(html).not.toContain('title="Xoá');
    expect(html).not.toContain("Thêm ca làm việc");
    expect(html).not.toContain("Gieo giờ làm việc mặc định");
    expect(html).not.toContain("Gieo ngày nghỉ lễ theo dương lịch");
  });

  it("CHỈ có `admin.sla`: nút ghi có mặt ở cả ba bảng", () => {
    // Through the SAME decision `WorkingCalendarTab` calls, not a hand-typed flag.
    const decision = quyetDinhGhiThoiHan(sessionWith(["admin.sla"]));
    expect(decision).toEqual({ hien: true });

    const html = render({}, { canWrite: decision.hien });
    expect(html).toContain('title="Sửa ca Thứ Hai');
    expect(html).toContain('title="Xoá ca Thứ Hai');
    expect(html).toContain('title="Sửa ngày nghỉ');
    expect(html).toContain("Thêm ca làm việc");
    expect(html).toContain("Thêm ngày nghỉ lễ");
    expect(html).toContain("Thêm ca làm bù");
  });

  it("CA BỊ TỪ CHỐI: không có `admin.sla` (dù có mọi khoá admin khác) → không một nút ghi nào", () => {
    const decision = quyetDinhGhiThoiHan(sessionWith(["admin.lookup", "admin.org", "admin.user", "admin.role", "admin.audit"]));
    expect(decision).toEqual({ hien: false, vi: "khong-du-quyen" });

    const html = render({}, { canWrite: decision.hien });
    expect(html).not.toContain('title="Sửa');
    expect(html).not.toContain("Thêm ca làm việc");
    expect(html).not.toContain("Thêm ca làm bù");
  });

  it("không còn hộp 'chỉ xem' nào (spec 02): thiếu quyền chỉ là thiếu nút", () => {
    expect(render({}, { canWrite: false })).not.toContain("không có quyền sửa");
  });
});

describe("trạng thái rỗng của hai bảng theo năm — một dòng trong bảng", () => {
  it("năm chưa khai ngày nghỉ nào thì nói ra hệ quả, không để bảng trống", () => {
    const html = render({ holidays: HOLIDAYS_EMPTY });

    expect(html).toContain("Năm 2026 chưa khai ngày nghỉ lễ nào");
    expect(html).toMatch(/<td colSpan="3"/i);
  });

  it("năm không có ngày làm bù là BÌNH THƯỜNG, và câu chữ phải phân biệt với lịch tuần trống", () => {
    expect(render({ swapDays: SWAP_EMPTY })).toContain("Phần lớn các năm là như vậy");
  });
});

describe("ô chọn năm của lịch nghỉ lễ và làm bù (VALIDATE 08/10/2026)", () => {
  it("là select theo mẫu chung (selectCls, h-9), nhãn 11.5px — không còn ô `.chon-nam` cũ", () => {
    const html = render();
    expect(html).not.toContain("chon-nam");
    const select = html.match(/<select id="nam-lich-lam-viec"[^>]*class="([^"]*)"/);
    expect(select).not.toBeNull();
    const cls = select![1]!.split(" ");
    expect(cls).toContain("h-9");
    expect(cls).toContain("rounded-md");
    expect(cls).toContain("text-[12.5px]");
    expect(html).toMatch(/<label for="nam-lich-lam-viec" class="[^"]*text-\[11\.5px\][^"]*">Năm của lịch nghỉ lễ và làm bù<\/label>/);
  });

  it("giữ đúng các năm của `danhSachNam` quanh năm gốc, năm đang chọn được chọn", () => {
    const html = render();
    expect(html).toMatch(/<option value="2026" selected="">2026<\/option>/);
    expect(html).toContain('<option value="2025">2025</option>');
    expect(html).toContain('<option value="2027">2027</option>');
  });
});

describe("biểu mẫu là hàng xám phía trên bảng", () => {
  it("biểu mẫu thuộc đúng bảng của nó", () => {
    expect(calendarGroupOf({ kieu: "themCa" })).toBe("week");
    expect(calendarGroupOf({ kieu: "xoaNghi", ngay: HOLIDAY })).toBe("holidays");
    expect(calendarGroupOf({ kieu: "suaLamBu", ca: { ...SHIFT, date: "2026-02-28", name: "x" } })).toBe("swapDays");
    expect(calendarGroupOf(null)).toBeNull();
  });

  it("biểu mẫu mở NGAY TRÊN bảng của nó, không phải dưới", () => {
    const html = renderToStaticMarkup(
      <WorkingCalendarView
        data={{ week: WEEK_WITH_SHIFT, holidays: HOLIDAYS_ONE, swapDays: SWAP_EMPTY }}
        year={2026}
        baseYear={2026}
        setYear={() => {}}
        canWrite
        actions={DO_NOTHING}
        form={<form aria-label="FORM-MARK" />}
        formGroup="week"
        outsideFormError=""
        busy={false}
      />,
    );
    expect(html.indexOf("FORM-MARK")).toBeGreaterThan(-1);
    expect(html.indexOf("FORM-MARK")).toBeLessThan(html.indexOf('aria-label="Giờ làm việc trong tuần"'));
  });

  it("xoá: giữ bước lý do, có câu cảnh báo giữ chỗ vĩnh viễn, lỗi tại chỗ và lỗi máy chủ là hai vùng", () => {
    const html = renderToStaticMarkup(
      <CalendarForm
        open={{ kieu: "xoaCa", ca: SHIFT }}
        draft={EMPTY_CALENDAR_DRAFT}
        setDraft={() => {}}
        localError="Hãy nêu lý do xoá."
        serverError="Ca này đã được dùng để tính hạn."
        busy={false}
        onSubmit={() => {}}
        onCancel={() => {}}
      />,
    );
    expect(html).toContain('name="lyDo"');
    expect(html).toContain("Lý do xoá");
    expect(html).toContain("bị giữ lại vĩnh viễn");
    expect(html).toContain("Xác nhận xoá");
    expect(html).toContain("Hãy nêu lý do xoá.");
    expect(html).toContain("Ca này đã được dùng để tính hạn.");
    expect((html.match(/role="alert"/g) ?? []).length).toBe(2);
  });

  it("thêm ca: nút 'Thêm' và 'Huỷ'; ô thứ là ô chọn theo ISO (1 = thứ Hai)", () => {
    const html = renderToStaticMarkup(
      <CalendarForm
        open={{ kieu: "themCa" }}
        draft={EMPTY_CALENDAR_DRAFT}
        setDraft={() => {}}
        localError=""
        serverError=""
        busy={false}
        onSubmit={() => {}}
        onCancel={() => {}}
      />,
    );
    expect(html).toContain(">Thêm<");
    expect(html).toContain(">Huỷ<");
    expect(html).toMatch(/<option value="1"[^>]*>Thứ Hai<\/option>/);
    expect(html).toContain('type="time"');
  });
});
