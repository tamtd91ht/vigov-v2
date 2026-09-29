import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/session/current-session";
import type { AutomationJob } from "@/lib/api/automation-jobs";

let fakeSession: PhienDaDoc = null;

vi.mock("@/features/session/current-session", () => ({
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
      busy=""
      notice={null}
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

  it("có `admin.sla` → lời dẫn của §9 và câu 'chỉ gửi cán bộ, qua chuông'", () => {
    fakeSession = sessionWith(["admin.sla"]);
    const html = renderToStaticMarkup(<AutomationTab />);
    expect(html).toContain(form.AUTOMATION_GUIDANCE);
    expect(html).toContain(form.AUTOMATION_RECIPIENTS);
    expect(form.AUTOMATION_RECIPIENTS).toMatch(/chuông thông báo/);
    expect(html).toContain("Đang tải cấu hình tự động hoá");
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
  it("chưa lưu lần nào (configured:false) → ĐANG TẮT, nhịp gợi ý điền sẵn, không có Chạy ngay", () => {
    const html = card(job({ configured: false, enabled: false, enabled_at: null }));
    expect(html).toContain("Đang tắt — xã chưa lưu cấu hình việc này");
    expect(html).toContain('value="15"');
    expect(html).not.toContain(`>${form.RUN_NOW_BUTTON}</button>`);
    expect(html).toContain(form.RUN_NOW_NEEDS_ON);
    // The switch is drawn off.
    expect(html).toMatch(/role="switch"(?![^>]*checked)/);
  });

  it("đang bật → nhịp bằng lời, giờ Việt Nam, nút Chạy ngay", () => {
    const html = card(job());
    expect(html).toContain("Cứ 15 phút một lần (Giờ Việt Nam)");
    expect(html).toContain(`>${form.RUN_NOW_BUTTON}</button>`);
    expect(html).toContain("Ít nhất 5 phút giữa hai lượt quét.");
  });

  it("việc theo tuần: ô chọn thứ (1 = Thứ Hai … 7 = Chủ nhật) và ô giờ", () => {
    const html = card(job({ job: "weekly_digest", schedule_kind: "weekly", interval_minutes: null, run_hour: 7, run_minute: 30, weekday: 1 }));
    expect(html).toContain('<option value="1" selected="">Thứ Hai</option>');
    expect(html).toContain('<option value="7">Chủ nhật</option>');
    expect(html).toContain('type="time"');
    expect(html).toContain('value="07:30"');
    expect(html).toContain("Lúc (Giờ Việt Nam)");
  });

  it("chưa có lượt chạy nào → nói đúng câu ấy, không có bảng", () => {
    const html = card(job());
    expect(html).toContain(form.NO_RUNS_YET);
    expect(html).not.toContain("<table");
  });

  it("lượt chạy theo từng loại việc: giờ nhận, cách chạy, kết quả, ba con số", () => {
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
    expect(html).toContain("<td>Nhiệm vụ</td><td>08:15 29/09/2026</td><td>Theo lịch</td><td>Hoàn thành</td><td>42</td><td>3</td><td>1</td>");
    expect(html).toContain("<td>Văn bản đến</td><td>08:16 29/09/2026</td><td>Chạy ngay</td><td>Chưa báo kết quả</td><td>—</td><td>—</td><td>—</td>");
    expect(html).not.toContain(form.NO_RUNS_YET);
  });

  it("câu từ chối của máy chủ (409 đang tắt / người khác vừa lưu) hiện nguyên văn", () => {
    const sentence = "Cấu hình việc này vừa được người khác lưu. Hãy tải lại trang rồi thử lại.";
    const html = card(job(), { notice: { ok: false, text: sentence } });
    expect(html).toContain(`<p class="thong-bao-loi" role="alert">${sentence}</p>`);
  });

  it("yêu cầu chạy ngay chưa ai nhận → nói thật, từ dữ liệu", () => {
    const html = card(job({ run_requested_at: "2026-09-29T03:00:00Z" }));
    expect(html).toContain("Chưa có lượt chạy nào nhận yêu cầu này.");
  });

  it("không chèn HTML thô: một khoá việc lạ chứa thẻ vẫn chỉ là chữ", () => {
    const html = card(job({ job: "<b>x</b>" }));
    expect(html).toContain("&lt;b&gt;x&lt;/b&gt;");
    expect(html).not.toContain("<b>x</b>");
  });
});
