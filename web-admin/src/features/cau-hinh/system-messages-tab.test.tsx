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

/** The attribute, not the word: every button's class list carries `disabled:` variants. */
const DISABLED_ATTR = ' disabled=""';

function card(m: SystemMessage, extra: Partial<{ draft: string; busy: boolean; error: string | null }> = {}) {
  return renderToStaticMarkup(
    <SystemMessageCardView
      message={m}
      draft={m.current_text}
      busy={false}
      error={null}
      onDraft={noop}
      onSave={noop}
      onRestore={noop}
      {...extra}
    />,
  );
}

/** The `<button …>…label</button>` element, to read its attributes. */
function buttonOf(html: string, label: string): string {
  const m = html.match(new RegExp(`<button[^>]*>(?:(?!<button).)*?${label}</button>`));
  expect(m).not.toBeNull();
  return m![0];
}

afterEach(() => {
  fakeSession = null;
  vi.unstubAllGlobals();
});

describe("tab Lời hệ thống — cổng quyền", () => {
  it("CA BỊ TỪ CHỐI: thiếu `admin.lookup` → câu từ chối, không gọi tuyến nào, không có nút nào", () => {
    const fake = vi.fn();
    vi.stubGlobal("fetch", fake);
    fakeSession = sessionWith(["admin.audit", "admin.user", "asset.read"]);
    const html = renderToStaticMarkup(<SystemMessagesTab />);
    expect(html).toContain("không có quyền sửa lời hệ thống");
    expect(html).not.toContain("Phản ánh của người dân");
    expect(html).not.toContain("Thêm câu mới");
    expect(fake).not.toHaveBeenCalled();
  });

  it("có `admin.lookup` → ba nhóm theo spec 07: Phản ánh của người dân, Theo dõi giải ngân, Báo cáo điều hành", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    const html = renderToStaticMarkup(<SystemMessagesTab />);
    expect(html).toMatch(/<h3[^>]*class="text-navy m-0 mb-2.5 text-\[12.5px\] font-bold"[^>]*>Phản ánh của người dân<\/h3>/);
    expect(html).toContain(">Theo dõi giải ngân</h3>");
    expect(html).toContain(">Báo cáo điều hành</h3>");
  });

  it("thứ tự nhóm theo mã nhóm tăng dần, như prototype: bao-cao, giai-ngan, phan-anh", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    expect(form.SYSTEM_MESSAGE_SECTIONS.map((s) => s.group)).toEqual(["bao-cao", "giai-ngan", "phan-anh"]);
    const titles = [...renderToStaticMarkup(<SystemMessagesTab />).matchAll(/<h3[^>]*>([^<]*)<\/h3>/g)].map((x) => x[1]);
    expect(titles).toEqual(["Báo cáo điều hành", "Theo dõi giải ngân", "Phản ánh của người dân"]);
  });

  it("câu hướng dẫn: nguyên văn câu chủ đầu tư chốt 08/10 (spec, bỏ phần Mini App), kiểu chữ của spec", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    const sentence =
      "Những câu dưới đây là lời hệ thống hiện ra trên trang quản trị. Câu đi kèm phần mềm có thể sửa lời " +
      "nhưng không xoá được — xoá đi thì lúc từ chối, hệ thống không còn gì để nói.";
    expect(form.SYSTEM_MESSAGES_GUIDANCE).toBe(sentence);
    expect(renderToStaticMarkup(<SystemMessagesTab />)).toContain(
      `<p class="text-ink-muted m-0 max-w-2xl text-[12.5px]">${sentence}</p>`,
    );
  });

  it("đang tải: khung ba thanh dùng chung (ConfigLoading) ở mỗi nhóm", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    vi.stubGlobal("fetch", vi.fn(() => new Promise(() => {})));
    const html = renderToStaticMarkup(<SystemMessagesTab />);
    expect(html).toContain("Đang tải lời hệ thống…");
    expect(html.match(/bg-muted h-11 w-full rounded-md/g)?.length).toBe(9);
  });

  it("KHÔNG còn ghi chú của nhóm Báo cáo (chủ đầu tư 08/10: 'Bỏ hết, đúng prototype')", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    const html = renderToStaticMarkup(<SystemMessagesTab />);
    expect(html).not.toMatch(/chưa hiện ở đâu|Màn Báo cáo chưa xuất tệp/);
  });

  it("`Thêm câu mới`: nút cỡ thường, VÔ HIỆU, có dấu '?' (ADR 0068 §14) — không bao giờ là nút bấm được", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    const html = renderToStaticMarkup(<SystemMessagesTab />);
    const button = buttonOf(html, "Thêm câu mới");
    expect(button).toContain(DISABLED_ATTR);
    expect(button).toContain("h-8"); // size md, not sm (h-7)
    expect(button).not.toContain("h-7");
    expect(html).toContain('aria-label="Thêm câu mới — tính năng đang phát triển. Bấm để xem mô tả"');
  });
});

describe("một câu hệ thống", () => {
  it("thẻ theo spec: mã, 'Đi kèm phần mềm', mô tả, ô sửa trực tiếp chứa câu đang dùng — không bảng mặc định/đang dùng, không nút 'Sửa lời'", () => {
    const m = message();
    const html = card(m);
    expect(html).toContain('<article class="border-line rounded-[10px] border border-solid bg-white p-3"');
    expect(html).toContain(">feedback.reason_required</code>");
    expect(html).toContain(form.SHIPPED_BADGE);
    expect(html).toContain('<p class="text-ink-muted m-0 mb-2 text-[11.5px]">');
    expect(html).toMatch(new RegExp(`<textarea[^>]*rows="2"[^>]*>${m.current_text}</textarea>`));
    expect(html).not.toContain("Câu mặc định của phần mềm");
    expect(html).not.toContain("<dl");
    expect(html).not.toContain("Sửa lời</button>");
    expect("EDIT_BUTTON" in form).toBe(false);
  });

  it("chưa sửa: không có nhãn 'Đã sửa lời', không có Khôi phục; 'Lưu' VÔ HIỆU khi câu chưa đổi", () => {
    const html = card(message());
    expect(html).not.toContain(form.OVERRIDDEN_BADGE);
    expect(html).not.toContain(form.RESTORE_BUTTON);
    expect(buttonOf(html, "Lưu")).toContain(DISABLED_ATTR);
  });

  it("'Lưu' BẬT khi nội dung đã khác câu đang dùng (khác chỉ ở dấu cách hai đầu thì vẫn tắt)", () => {
    const m = message();
    expect(buttonOf(card(m, { draft: "Xin ghi lý do." }), "Lưu")).not.toContain(DISABLED_ATTR);
    expect(buttonOf(card(m, { draft: `  ${m.current_text} ` }), "Lưu")).toContain(DISABLED_ATTR);
  });

  it("đã sửa: nhãn tangerine 'Đã sửa lời' và nút Khôi phục lời gốc bấm được ngay", () => {
    const html = card(message({ overridden: true, current_text: "Xin ghi lý do để trả lời người dân." }));
    expect(html).toContain(">Đã sửa lời</span>");
    expect(html).toContain(">Xin ghi lý do để trả lời người dân.</textarea>");
    const restore = buttonOf(html, form.RESTORE_BUTTON);
    expect(restore).toContain('type="button"');
    expect(restore).not.toContain(DISABLED_ATTR);
  });

  it("HỒI QUY (chủ đầu tư 08/10 'Bỏ hết, đúng prototype'): không dòng 'Sửa lần cuối', không ghi chú 'chưa có chức năng nào', không bước xác nhận khôi phục", () => {
    const html =
      card(
        message({
          overridden: true,
          updated_by: "CB-00123",
          updated_at: "2026-09-22T07:05:00Z",
        }),
      ) + card(message({ code: "feedback.unknown_field" }));
    expect(html).not.toContain("Sửa lần cuối");
    expect(html).not.toContain("CB-00123");
    expect(html).not.toContain("Chưa có chức năng nào dùng câu này");
    expect(html).not.toContain("Xác nhận khôi phục");
    expect(html).not.toContain("Dùng lại câu mặc định của phần mềm");
  });

  it("câu đã sửa lời: 'Tắt' là nút '?' vô hiệu; không có 'Xoá' (mọi câu đi kèm phần mềm)", () => {
    const html = card(message({ overridden: true }));
    expect(buttonOf(html, "Tắt")).toContain(DISABLED_ATTR);
    expect(html).toContain('aria-label="Tắt câu hệ thống — tính năng đang phát triển. Bấm để xem mô tả"');
    expect(html).not.toContain("Xoá");
  });

  it("HỒI QUY (ADR 0079 lô 3): câu CHƯA có lời của xã thì KHÔNG có 'Tắt' — không có gì để tắt", () => {
    const html = card(message({ overridden: false }));
    expect(html).not.toMatch(/>Tắt<\/button>/);
    expect(html).not.toContain("Tắt câu hệ thống");
  });

  it("HỒI QUY: 'Đi kèm phần mềm' và 'Đã sửa lời' là thuộc tính — viên chữ không biểu tượng, như prototype", () => {
    const html = card(message({ overridden: true }));
    expect(html).toMatch(/<span class="[^"]*bg-background text-ink border-line">Đi kèm phần mềm<\/span>/);
    expect(html).toMatch(
      /<span class="[^"]*bg-tangerine\/12 text-tangerine border-tangerine\/25">Đã sửa lời<\/span>/,
    );
    // The card's only icon is Khôi phục's RotateCcw: no tone icon on either pill.
    expect(html.match(/<svg/g)?.length).toBe(1);
  });

  it("ô sửa giới hạn 1000 ký tự", () => {
    expect(card(message())).toContain('maxLength="1000"');
  });

  it("lỗi hiện TẠI CHỖ dưới ô sửa, nguyên văn câu máy chủ, ô sửa được đánh dấu lỗi", () => {
    const sentence = "Nội dung câu không được chứa dấu < hoặc >.";
    const html = card(message(), { draft: "", error: sentence });
    expect(html).toContain('aria-invalid="true"');
    expect(html).toContain(
      'role="alert" class="text-danger m-0 mt-1.5 text-[12px] font-medium">Nội dung câu không được chứa dấu &lt; hoặc &gt;.</p>',
    );
  });

  it("không dùng lại tên lớp cũ", () => {
    const html = card(message({ overridden: true }));
    expect(html).not.toMatch(/the-loi-he-thong|o-nhap|form-danh-muc|cum-nut|ma-muc|ghi-chu|thong-bao-loi/);
  });
});
