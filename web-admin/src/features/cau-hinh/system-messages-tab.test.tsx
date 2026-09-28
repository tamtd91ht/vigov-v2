import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { SystemMessage } from "@/lib/api/system-messages";

let fakeSession: PhienDaDoc = null;

vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => fakeSession,
}));

const { SystemMessageCardView, SystemMessagesTab } = await import("./system-messages-tab");
const form = await import("./system-message-form");

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

function message(patch: Partial<SystemMessage> = {}): SystemMessage {
  return {
    code: "feedback.reason_required",
    description: "Hiện khi cán bộ không tiếp nhận hoặc chuyển phiếu lên cấp trên mà bỏ trống lý do",
    default_text: "Không tiếp nhận hoặc chuyển phiếu lên cấp trên thì phải ghi rõ lý do để trả lời người dân.",
    current_text: "Không tiếp nhận hoặc chuyển phiếu lên cấp trên thì phải ghi rõ lý do để trả lời người dân.",
    overridden: false,
    updated_at: null,
    ...patch,
  };
}

const noop = () => {};

function card(m: SystemMessage, mode: "view" | "edit" | "confirm-restore" = "view", extra = {}) {
  return renderToStaticMarkup(
    <SystemMessageCardView
      message={m}
      mode={mode}
      draft="Câu đang gõ"
      busy={false}
      notice={null}
      onEdit={noop}
      onDraft={noop}
      onSave={noop}
      onCancel={noop}
      onAskRestore={noop}
      onRestore={noop}
      {...extra}
    />,
  );
}

afterEach(() => {
  fakeSession = null;
  vi.unstubAllGlobals();
});

describe("tab Lời hệ thống — cổng quyền", () => {
  it("CA BỊ TỪ CHỐI: thiếu `admin.lookup` → câu từ chối, không gọi tuyến nào", () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    fakeSession = sessionWith(["admin.audit", "admin.user", "asset.read"]);
    const html = renderToStaticMarkup(<SystemMessagesTab />);
    expect(html).toContain("không có quyền sửa lời hệ thống");
    expect(html).not.toContain("Phản ánh");
    expect(fake).not.toHaveBeenCalled();
  });

  it("có `admin.lookup` → hai phần Phản ánh và Thu – Chi, kèm ghi chú 32 câu report.*", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    const html = renderToStaticMarkup(<SystemMessagesTab />);
    expect(html).toContain(">Phản ánh</h3>");
    expect(html).toContain(">Thu – Chi</h3>");
    expect(html).toContain(form.REPORT_MESSAGES_NOTE);
    expect(form.REPORT_MESSAGES_NOTE).toMatch(/32 câu/);
    expect(form.REPORT_MESSAGES_NOTE).toMatch(/ADR 0024/);
  });

  it("KHÔNG có nút `+ Thêm câu mới` và không có `Tắt` — danh mục đóng, câu không tắt được", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    const html =
      renderToStaticMarkup(<SystemMessagesTab />) + card(message({ overridden: true }));
    expect(html).not.toContain("Thêm câu mới");
    expect(html).not.toMatch(/>\s*Tắt\s*</);
  });
});

describe("một câu hệ thống", () => {
  it("hiện mô tả, câu mặc định, câu đang dùng; chưa sửa thì không có nhãn và không có Khôi phục", () => {
    const html = card(message());
    expect(html).toContain("feedback.reason_required");
    expect(html).toContain("Câu mặc định của phần mềm");
    expect(html).toContain("Câu đang dùng");
    expect(html).not.toContain(form.OVERRIDDEN_BADGE);
    expect(html).not.toContain(form.RESTORE_BUTTON);
    expect(html).toContain(form.EDIT_BUTTON);
  });

  it("xã đã sửa: nhãn 'Đang dùng câu của xã', người và lúc sửa, nút Khôi phục câu mặc định", () => {
    const html = card(
      message({
        overridden: true,
        current_text: "Xin ghi lý do để trả lời người dân.",
        updated_by: "CB-00123",
        updated_at: "2026-09-22T07:05:00Z",
      }),
    );
    expect(html).toContain(form.OVERRIDDEN_BADGE);
    expect(html).toContain("Xin ghi lý do để trả lời người dân.");
    expect(html).toContain("Sửa lần cuối: CB-00123, 14:05 22/09/2026");
    expect(html).toContain(form.RESTORE_BUTTON);
  });

  it.each(["feedback.after_photo_required", "feedback.unknown_field"])(
    "%s: 'chưa có chức năng nào dùng câu này'",
    (code) => {
      expect(card(message({ code }))).toContain(form.NOT_RAISED_NOTE);
    },
  );

  it("câu đang được dùng thật thì KHÔNG mang ghi chú ấy", () => {
    expect(card(message({ code: "feedback.never_public" }))).not.toContain(form.NOT_RAISED_NOTE);
  });

  it("chế độ sửa: ô nhập có giới hạn 1000 ký tự, nút Lưu và Huỷ", () => {
    const html = card(message(), "edit");
    expect(html).toContain('maxLength="1000"');
    expect(html).toContain(">Câu đang gõ</textarea>");
    expect(html).toContain(`>${form.SAVE_BUTTON}</button>`);
    expect(html).toContain(`>${form.CANCEL_BUTTON}</button>`);
  });

  it("chế độ xác nhận khôi phục: hỏi lại trước khi xoá câu của xã", () => {
    const html = card(message({ overridden: true }), "confirm-restore");
    expect(html).toContain(form.RESTORE_CONFIRM);
    expect(html).toContain(`>${form.RESTORE_CONFIRM_BUTTON}</button>`);
  });

  it("câu từ chối 400 của máy chủ hiện nguyên văn ở dòng báo lỗi", () => {
    const sentence = "Nội dung câu không được chứa dấu < hoặc >.";
    const html = card(message(), "edit", { notice: { ok: false, text: sentence } });
    expect(html).toContain('<p class="thong-bao-loi" role="alert">Nội dung câu không được chứa dấu &lt; hoặc &gt;.</p>');
  });
});
