import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { AutomationJob } from "@/lib/api/automation-jobs";

let fakeSession: PhienDaDoc = null;

vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => fakeSession,
}));

const { AutomationJobCardView, AutomationTab, AutomationTabView } = await import("./automation-tab");
const form = await import("./automation-form");

function sessionWith(permissions: readonly string[]): PhienDaDoc {
  return {
    ok: true,
    duLieu: {
      sid: "01J000000000000000000SID",
      expires_at: "2026-09-17T12:00:00Z",
      staff: { code: "CB001", full_name: "Huỳnh Văn 1", position: "Chuyên viên chuyên môn" },
      permissions: [...permissions],
    },
  } as PhienDaDoc;
}

function job(patch: Partial<AutomationJob> = {}): AutomationJob {
  return {
    job: "sla_reminders",
    schedule_kind: "interval",
    configured: true,
    enabled: true,
    interval_minutes: 15,
    min_interval_minutes: 5,
    run_hour: null,
    run_minute: null,
    weekday: null,
    timezone: "Asia/Ho_Chi_Minh",
    enabled_at: "2026-09-29T01:00:00Z",
    run_requested_at: null,
    last_runs: [],
    ...patch,
  };
}

const noop = () => {};

function card(j: AutomationJob, extra: Record<string, unknown> = {}) {
  return renderToStaticMarkup(
    <AutomationJobCardView
      job={j}
      draft={form.draftFrom(j)}
      on={j.configured && j.enabled}
      busy=""
      error={null}
      onToggle={noop}
      onDraft={noop}
      onSave={noop}
      onRunNow={noop}
      {...extra}
    />,
  );
}

afterEach(() => {
  fakeSession = null;
  vi.unstubAllGlobals();
});

describe("tab Tự động hoá — cổng quyền", () => {
  it("CA BỊ TỪ CHỐI: thiếu `admin.sla` → câu từ chối, không gọi tuyến nào", () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    fakeSession = sessionWith(["admin.lookup", "admin.org", "admin.audit"]);
    const html = renderToStaticMarkup(<AutomationTab />);
    expect(html).toContain("không có quyền cấu hình tự động hoá");
    expect(html).not.toContain(form.AUTOMATION_GUIDANCE);
    expect(fake).not.toHaveBeenCalled();
  });

  it("phiên hết hạn → câu của máy chủ, không hiện nội dung tab", () => {
    fakeSession = { ok: false, thongBao: "Phiên đã hết hạn" };
    const html = renderToStaticMarkup(<AutomationTab />);
    expect(html).toContain("Phiên đã hết hạn");
    expect(html).not.toContain(form.AUTOMATION_GUIDANCE);
  });

  it("có `admin.sla` → lời dẫn của §9 (đúng lớp chữ), và chỉ lời dẫn ấy", () => {
    fakeSession = sessionWith(["admin.sla"]);
    const html = renderToStaticMarkup(<AutomationTab />);
    expect(html).toContain(`<p class="text-ink-muted max-w-2xl text-[12.5px]">${form.AUTOMATION_GUIDANCE}</p>`);
    expect(html).toContain("Đang tải cấu hình tự động hoá");
    // Spec 09 loading: one `Skeleton h-80`, not table rows.
    expect(html).toMatch(/<span aria-hidden="true" class="[^"]*h-80/);
    expect(html).not.toContain("skeleton-rows");
  });

  it("REGRESSION (owner 08/10 'Bỏ hết, đúng prototype'): no second paragraph — the recipients sentence is gone", () => {
    fakeSession = sessionWith(["admin.sla"]);
    const html = renderToStaticMarkup(<AutomationTab />);
    expect(html).not.toContain("chuông thông báo");
    expect(html).not.toContain("Người dân không nhận tin");
    // The guidance is the only visible paragraph before the cards.
    expect(html.match(/<p class="text-ink-muted max-w-2xl/g)).toHaveLength(1);
  });
});

describe("danh sách việc", () => {
  it("đọc hỏng → câu của máy chủ", () => {
    const html = renderToStaticMarkup(<AutomationTabView loaded={{ ok: false, thongBao: "Đã xảy ra lỗi. Vui lòng thử lại." }} />);
    expect(html).toContain('role="alert">Đã xảy ra lỗi. Vui lòng thử lại.</p>');
  });

  it("ba việc, đúng thứ tự máy chủ trả, đúng tên của §9", () => {
    const html = renderToStaticMarkup(
      <AutomationTabView
        loaded={{
          ok: true,
          duLieu: [
            job(),
            job({ job: "escalation", schedule_kind: "daily", interval_minutes: null, run_hour: 7, run_minute: 0 }),
            job({ job: "weekly_digest", schedule_kind: "weekly", interval_minutes: null, run_hour: 7, run_minute: 30, weekday: 1 }),
          ],
        }}
      />,
    );
    const a = html.indexOf("Nhắc việc sắp đến hạn và đã quá hạn");
    const b = html.indexOf("Leo thang việc trễ hạn");
    const c = html.indexOf("Bản tin đầu tuần cho lãnh đạo");
    expect(a).toBeGreaterThan(-1);
    expect(b).toBeGreaterThan(a);
    expect(c).toBeGreaterThan(b);
  });
});

describe("một việc", () => {
  it("chưa lưu lần nào (configured:false) → ĐANG TẮT, nền trang (bg-background), không có phần nhịp", () => {
    const html = card(job({ configured: false, enabled: false, enabled_at: null }));
    expect(html).toContain('role="switch" aria-checked="false"');
    expect(html).toContain("Đang tắt</label>");
    // TOKEN TRAP: the spec's off `bg-surface` is #f4f8fb = `bg-background` here; `bg-surface` is white.
    expect(html).toMatch(/<section class="[^"]*bg-background/);
    expect(html).not.toMatch(/<section class="[^"]*bg-white/);
    expect(html).not.toContain("Cứ mỗi");
    expect(html).not.toContain(`${form.RUN_NOW_BUTTON}</button>`);
    expect(html).toContain("Chưa chạy lần nào");
  });

  it("đang bật → thẻ trắng, công tắc bật, 'Cứ mỗi' từ mức tối thiểu, Chạy ngay; chưa đổi thì không có Lưu nhịp", () => {
    const html = card(job({ min_interval_minutes: 10 }));
    expect(html).toMatch(/<section class="border-line shadow-card rounded-card border border-solid p-4 bg-white"/);
    expect(html).toContain('role="switch" aria-checked="true"');
    expect(html).toContain("Đang bật</label>");
    expect(html).toContain('class="text-ink-muted m-0 mb-1 block text-[11px] font-semibold">Cứ mỗi</label>');
    expect(html).toContain('<option value="15" selected="">15 phút</option>');
    expect(html).toMatch(/<select id="tu-dong-hoa-sla_reminders-nhip" class="[^"]* min-w-0 /);
    expect(html).toContain('<option value="180">3 giờ</option>');
    expect(html).toContain('<option value="1440">1 ngày</option>');
    // Below `min_interval_minutes` is not offered.
    expect(html).not.toContain('<option value="5">');
    expect(html).toContain(`${form.RUN_NOW_BUTTON}</button>`);
    expect(html).not.toContain(form.SAVE_CADENCE_BUTTON);
    // The old cadence sentence and state badge are gone.
    expect(html).not.toContain("một lần");
  });

  it("nhịp đã đổi mà chưa lưu → hiện nút 'Lưu nhịp' cỡ sm", () => {
    const j = job();
    const html = card(j, { draft: { ...form.draftFrom(j), interval: "30" } });
    expect(html).toMatch(/<button class="nut-chinh [^"]* h-7 [^"]* min-h-0" type="submit"[^>]*>Lưu nhịp<\/button>/);
  });

  it("việc theo tuần: ô 'Vào' (1 = Thứ Hai … 7 = Chủ nhật) và ô 'Lúc'", () => {
    const html = card(job({ job: "weekly_digest", schedule_kind: "weekly", interval_minutes: null, run_hour: 7, run_minute: 30, weekday: 1 }));
    expect(html).toContain(">Vào</label>");
    // The legacy 12rem select floor is undone, as `formSelectCls` does.
    expect(html).toMatch(/<select id="tu-dong-hoa-weekly_digest-thu" class="[^"]* min-w-0 /);
    expect(html).toContain('<option value="1" selected="">Thứ Hai</option>');
    expect(html).toContain('<option value="7">Chủ nhật</option>');
    expect(html).toContain('type="time"');
    expect(html).toContain('value="07:30"');
    expect(html).toContain(">Lúc</label>");
  });

  it("việc hằng ngày: chỉ ô 'Lúc'", () => {
    const html = card(job({ job: "escalation", schedule_kind: "daily", interval_minutes: null, run_hour: 7, run_minute: 0 }));
    expect(html).toContain(">Lúc</label>");
    expect(html).not.toContain(">Vào</label>");
    expect(html).not.toContain("Cứ mỗi");
  });

  it("chưa có lượt chạy nào → 'Chưa chạy lần nào', không có bảng", () => {
    const html = card(job());
    expect(html).toContain('<p class="text-ink-muted m-0 mt-1 text-[11px]">Chưa chạy lần nào</p>');
    expect(html).not.toContain("<table");
  });

  it("lượt chạy theo từng loại việc: dòng 'Chạy lần cuối' lấy lượt MỚI NHẤT, bảng gọn trong thẻ", () => {
    const html = card(
      job({
        last_runs: [
          {
            work_kind: "nhiem-vu",
            run_id: "01JRUN1",
            trigger: "schedule",
            claimed_at: "2026-09-29T01:15:00Z",
            outcome: "succeeded",
            records_examined: 42,
            notices_delivered: 3,
            records_without_recipient: 1,
            recorded_at: "2026-09-29T01:15:05Z",
          },
          {
            work_kind: "van-ban-den",
            run_id: "01JRUN2",
            trigger: "request",
            claimed_at: "2026-09-29T01:16:00Z",
            outcome: null,
            records_examined: null,
            notices_delivered: null,
            records_without_recipient: null,
            recorded_at: null,
          },
        ],
      }),
    );
    expect(html).toContain("Chạy lần cuối 08:16 29/09/2026");
    expect(html).toContain('<div class="border-line mt-3 border-t pt-3">');
    expect(html).toContain("<td>Nhiệm vụ</td><td>08:15 29/09/2026</td><td>Theo lịch</td><td>Hoàn thành</td><td>42</td><td>3</td><td>1</td>");
    expect(html).toContain("<td>Văn bản đến</td><td>08:16 29/09/2026</td><td>Chạy ngay</td><td>Chưa báo kết quả</td><td>—</td><td>—</td><td>—</td>");
  });

  it("câu từ chối của máy chủ (409 đang tắt / người khác vừa lưu) hiện nguyên văn, tại chỗ", () => {
    const sentence = "Cấu hình việc này vừa được người khác lưu. Hãy tải lại trang rồi thử lại.";
    const html = card(job(), { error: sentence });
    expect(html).toContain(`<p class="thong-bao-loi m-0 mt-2" role="alert">${sentence}</p>`);
  });

  it("yêu cầu chạy ngay chưa ai nhận → nói thật, từ dữ liệu", () => {
    const html = card(job({ run_requested_at: "2026-09-29T03:00:00Z" }));
    expect(html).toContain("Chưa có lượt chạy nào nhận yêu cầu này.");
  });

  it("không dùng lớp cũ của bảng/biểu mẫu danh mục", () => {
    const html = card(job());
    for (const legacy of ["o-nhap", "form-danh-muc", "the-loi-he-thong", "bang-danh-muc", "cum-nut"]) {
      expect(html).not.toContain(legacy);
    }
  });

  it("không chèn HTML thô: một khoá việc lạ chứa thẻ vẫn chỉ là chữ", () => {
    const html = card(job({ job: "<b>x</b>" }));
    expect(html).toContain("&lt;b&gt;x&lt;/b&gt;");
    expect(html).not.toContain("<b>x</b>");
  });
});
