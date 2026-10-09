import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, describe, expect, it, vi } from "vitest";

import type { PhienDaDoc } from "@/features/phien/phien-hien-tai";
import type { SystemMessage } from "@/lib/api/system-messages";

let fakeSession: PhienDaDoc = null;

vi.mock("@/features/phien/phien-hien-tai", () => ({
  usePhien: () => fakeSession,
}));

const { AddMessageFormView, SystemMessageCardView, SystemMessagesHeader, SystemMessagesTab } = await import(
  "./system-messages-tab"
);
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
    group_code: "phan-anh",
    origin: "shipped",
    description: "Hiện khi cán bộ không tiếp nhận hoặc chuyển phiếu lên cấp trên mà bỏ trống lý do",
    default_text: "Không tiếp nhận hoặc chuyển phiếu lên cấp trên thì phải ghi rõ lý do để trả lời người dân.",
    current_text: "Không tiếp nhận hoặc chuyển phiếu lên cấp trên thì phải ghi rõ lý do để trả lời người dân.",
    overridden: false,
    is_active: true,
    updated_at: null,
    ...patch,
  };
}

const noop = () => {};

/** The attribute, not the word: every button's class list carries `disabled:` variants. */
const DISABLED_ATTR = ' disabled=""';

function card(
  m: SystemMessage,
  extra: Partial<{
    draft: string;
    busy: boolean;
    error: string | null;
    deleteReason: string | null;
    deleteError: string | null;
  }> = {},
) {
  return renderToStaticMarkup(
    <SystemMessageCardView
      message={m}
      draft={form.editableText(m)}
      busy={false}
      error={null}
      onDraft={noop}
      onSave={noop}
      onRestore={noop}
      onToggle={noop}
      onDeleteStart={noop}
      onDeleteReason={noop}
      onDeleteSubmit={noop}
      onDeleteCancel={noop}
      {...extra}
    />,
  );
}

/** A sentence the commune added (ADR 0079 Q2): no default text, never overridden. */
function communeMessage(patch: Partial<SystemMessage> = {}): SystemMessage {
  return {
    code: "chung.loi-chao",
    group_code: "chung",
    origin: "commune",
    description: "Câu chào ở đầu thư trả lời",
    current_text: "Xã xin chào ông/bà.",
    overridden: false,
    is_active: true,
    updated_at: "2026-10-08T03:00:00Z",
    updated_by: "CB-00123",
    ...patch,
  };
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

  it("thứ tự nhóm theo mã nhóm tăng dần, như prototype: bao-cao, chung, giai-ngan, phan-anh", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    expect(form.SYSTEM_MESSAGE_SECTIONS.map((s) => s.group)).toEqual(["bao-cao", "chung", "giai-ngan", "phan-anh"]);
    // While loading, "Dùng chung" is not drawn: it appears only when it holds sentences.
    const titles = [...renderToStaticMarkup(<SystemMessagesTab />).matchAll(/<h3[^>]*>([^<]*)<\/h3>/g)].map((x) => x[1]);
    expect(titles).toEqual(["Báo cáo điều hành", "Theo dõi giải ngân", "Phản ánh của người dân"]);
  });

  it("'Dùng chung' do service Phản ánh giữ (ADR 0079 Q5a); nhóm lạ rơi vào nhóm chính của service, không biến mất", () => {
    const [chung, phanAnh, giaiNgan] = ["chung", "phan-anh", "giai-ngan"].map(
      (g) => form.SYSTEM_MESSAGE_SECTIONS.find((s) => s.group === g)!,
    );
    expect(chung).toMatchObject({ module: "petitions", title: "Dùng chung", primary: false });
    const all = [
      message(),
      communeMessage(),
      communeMessage({ code: "phan-anh.cam-on", group_code: "phan-anh" }),
      communeMessage({ code: "la.x", group_code: "nhom-la" }),
    ];
    expect(form.sectionMessages(chung!, all).map((m) => m.code)).toEqual(["chung.loi-chao"]);
    expect(form.sectionMessages(phanAnh!, all).map((m) => m.code)).toEqual([
      "feedback.reason_required",
      "phan-anh.cam-on",
      "la.x",
    ]);
    expect(form.sectionMessages(giaiNgan!, [communeMessage({ group_code: "giai-ngan" })]).length).toBe(1);
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

  it("`Thêm câu mới`: nút THẬT cỡ thường (không còn '?'), ml-auto, mở/đóng form", () => {
    fakeSession = sessionWith(["admin.lookup"]);
    const html = renderToStaticMarkup(<SystemMessagesTab />);
    const button = buttonOf(html, "Thêm câu mới");
    expect(button).not.toContain(DISABLED_ATTR);
    expect(button).toContain("h-8"); // size md, not sm (h-7)
    expect(button).not.toContain("h-7");
    expect(button).toContain("ml-auto");
    expect(button).toContain('aria-expanded="false"');
    expect(html).not.toContain("tính năng đang phát triển");
    expect(renderToStaticMarkup(<SystemMessagesHeader adding onToggleAdd={noop} />)).toContain('aria-expanded="true"');
  });
});

describe("form thêm câu (spec 07)", () => {
  function addForm(extra: Partial<{ draft: ReturnType<typeof form.emptyAddDraft>; error: string | null; sending: boolean }> = {}) {
    return renderToStaticMarkup(
      <AddMessageFormView
        draft={form.emptyAddDraft()}
        sending={false}
        error={null}
        onDraft={noop}
        onSubmit={noop}
        onCancel={noop}
        {...extra}
      />,
    );
  }

  it("khung xám của spec, đúng prototype: Mã câu (gợi ý cố định 'chung.loi-chao') · Giải thích, rồi Nội dung 2 dòng, 'Thêm câu' / 'Huỷ'", () => {
    const html = addForm();
    expect(html).toContain('class="border-line bg-background mb-4 rounded-[10px] border border-solid p-3"');
    expect(html).toContain('class="grid gap-3 sm:grid-cols-[14rem_minmax(0,1fr)]"');
    expect(html).toContain(">Mã câu</label>");
    expect(html).toContain('placeholder="chung.loi-chao"');
    expect(html).toContain(">Giải thích câu này dùng ở đâu</label>");
    expect(html).toContain(">Nội dung</label>");
    expect(html).toMatch(/<textarea[^>]*rows="2"/);
    expect(buttonOf(html, "Thêm câu")).toContain('type="submit"');
    expect(buttonOf(html, "Huỷ")).toContain('type="button"');
    // Field order of the prototype: code, description, then content.
    expect(html.indexOf(">Mã câu<")).toBeLessThan(html.indexOf(">Giải thích câu này dùng ở đâu<"));
    expect(html.indexOf(">Giải thích câu này dùng ở đâu<")).toBeLessThan(html.indexOf(">Nội dung<"));
  });

  it("HỒI QUY (VALIDATE vòng 4, 'đúng prototype'): KHÔNG có ô chọn nhóm — nhóm lấy từ tiền tố của mã", () => {
    const html = addForm();
    expect(html).not.toContain("<select");
    expect(html).not.toContain(">Nhóm</label>");
    expect("ADD_GROUP_LABEL" in form).toBe(false);
    // The placeholder is fixed, whatever is typed.
    expect(addForm({ draft: { ...form.emptyAddDraft(), code: "giai-ngan.x" } })).toContain('placeholder="chung.loi-chao"');
  });

  it("lỗi hiện TẠI CHỖ trong form, nguyên văn", () => {
    expect(addForm({ error: form.ADD_MISSING })).toContain(
      'role="alert" class="text-danger m-0 mt-2 text-[12px] font-medium">Cần cả mã và nội dung câu.</p>',
    );
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

  it("câu đã sửa lời: 'Tắt' là nút THẬT (không '?'); không có 'Xoá' (mọi câu đi kèm phần mềm)", () => {
    const html = card(message({ overridden: true }));
    const off = buttonOf(html, "Tắt");
    expect(off).not.toContain(DISABLED_ATTR);
    expect(off).toContain('type="button"');
    expect(html).not.toContain("tính năng đang phát triển");
    expect(html).not.toContain("Xoá");
  });

  it("câu đi kèm phần mềm đang tắt: mờ (opacity-60), viên 'Đang tắt — dùng lời gốc', nút 'Bật lại'; ô sửa giữ LỜI CỦA XÃ chứ không phải lời gốc đang dùng", () => {
    const m = message({
      overridden: true,
      is_active: false,
      override_text: "Xin ghi lý do.",
      current_text: "Không tiếp nhận hoặc chuyển phiếu lên cấp trên thì phải ghi rõ lý do để trả lời người dân.",
    });
    const html = card(m);
    expect(html).toMatch(/<article class="[^"]*opacity-60"/);
    expect(html).toMatch(/<span class="[^"]*bg-ink-muted\/12 text-ink border-line">Đang tắt — dùng lời gốc<\/span>/);
    expect(buttonOf(html, "Bật lại")).not.toContain(DISABLED_ATTR);
    expect(html).not.toMatch(/>Tắt<\/button>/);
    expect(html).toContain(">Xin ghi lý do.</textarea>");
    // Unchanged words: nothing to save.
    expect(buttonOf(html, "Lưu")).toContain(DISABLED_ATTR);
  });

  it("câu đang bật thì KHÔNG mờ, không có viên 'Đang tắt'", () => {
    const html = card(message({ overridden: true }));
    expect(html).not.toContain("opacity-60");
    expect(html).not.toContain("Đang tắt");
  });
});

describe("câu xã tự thêm (ADR 0079 Q2)", () => {
  it("viên 'Xã tự thêm' màu brand, chỉ chữ; không 'Đi kèm phần mềm', không 'Đã sửa lời', không 'Khôi phục lời gốc'", () => {
    const html = card(communeMessage());
    expect(html).toMatch(/<span class="[^"]*bg-brand\/12 text-brand border-brand\/25">Xã tự thêm<\/span>/);
    expect(html).not.toContain(form.SHIPPED_BADGE);
    expect(html).not.toContain(form.OVERRIDDEN_BADGE);
    expect(html).not.toContain(form.RESTORE_BUTTON);
  });

  it("'Tắt' thật; 'Xoá' viền đỏ ml-auto có Trash2 — chỉ trên câu xã tự thêm", () => {
    const html = card(communeMessage());
    expect(buttonOf(html, "Tắt")).not.toContain(DISABLED_ATTR);
    const del = buttonOf(html, "Xoá");
    expect(del).toContain("text-danger");
    expect(del).toContain("ml-auto");
    expect(del).toContain("lucide-trash-2");
    expect(del).not.toContain(DISABLED_ATTR);
    expect(card(message())).not.toContain(">Xoá</button>");
  });

  it("đang tắt: mờ, viên 'Đang tắt' (không nói 'dùng lời gốc' — câu xã thêm không có lời gốc), nút 'Bật lại'", () => {
    const html = card(communeMessage({ is_active: false }));
    expect(html).toMatch(/<article class="[^"]*opacity-60"/);
    expect(html).toMatch(/<span class="[^"]*bg-ink-muted\/12 text-ink border-line">Đang tắt<\/span>/);
    expect(html).not.toContain("dùng lời gốc");
    expect(buttonOf(html, "Bật lại")).not.toContain(DISABLED_ATTR);
  });

  it("ADR 0079 lô 3 Q7(b) THẮNG Q5b: không có dòng 'Chưa có chức năng nào dùng câu này' trên câu xã tự thêm", () => {
    expect(card(communeMessage())).not.toContain("Chưa có chức năng nào dùng câu này");
  });

  it("mô tả trống thì không vẽ dòng mô tả", () => {
    expect(card(communeMessage({ description: "" }))).not.toContain('class="text-ink-muted m-0 mb-2 text-[11.5px]"');
  });

  it("bước LÝ DO XOÁ (luật 7): ô lý do có nhãn, nút Xoá đỏ + Huỷ, nằm NGOÀI form sửa lời; 'Xoá' trên thẻ khoá lại", () => {
    const html = card(communeMessage(), { deleteReason: "" });
    expect(html).toContain(">Lý do xoá câu chung.loi-chao</label>");
    expect(html).toMatch(/<input[^>]*id="loi-he-thong-ly-do-xoa-chung.loi-chao"[^>]*maxLength="200"/);
    expect(html).toMatch(/<form[^>]*aria-label="Xoá câu chung.loi-chao"/);
    // The edit form closes before the delete step opens: no nested <form>.
    const edit = html.indexOf('aria-label="Sửa lời câu chung.loi-chao"');
    const step = html.indexOf('aria-label="Xoá câu chung.loi-chao"');
    expect(html.slice(edit, step)).toContain("</form>");
    expect(buttonOf(html, "Huỷ")).toContain('type="button"');
    expect(html.match(/<button[^>]*>(?:(?!<button).)*?Xoá<\/button>/g)?.length).toBe(2);
    expect(html).toMatch(/<button class="[^"]*bg-destructive\/10[^"]*" type="submit"[^>]*>Xoá<\/button>/);
    expect(html).toMatch(/<button[^>]*disabled=""[^>]*aria-expanded="true"/);
  });

  it("lỗi của bước xoá hiện tại chỗ, nguyên văn", () => {
    const html = card(communeMessage(), { deleteReason: "", deleteError: form.DELETE_REASON_MISSING });
    expect(html).toContain(`role="alert" class="text-danger col-span-full m-0 text-[12px] font-medium">${form.DELETE_REASON_MISSING}</p>`);
  });

  it("câu đi kèm phần mềm KHÔNG bao giờ có bước xoá, kể cả khi trạng thái lệch", () => {
    expect(card(message({ overridden: true }), { deleteReason: "" })).not.toContain("Lý do xoá");
  });

  it("chủ dự án 09/10/2026 (thay ADR 0079 lô 3): câu CHƯA sửa lời cũng có 'Tắt' thật — như prototype :193-194", () => {
    const html = card(message({ overridden: false }));
    const off = buttonOf(html, "Tắt");
    expect(off).not.toContain(DISABLED_ATTR);
    expect(off).toContain('type="button"');
    // Never reworded: still no "Khôi phục lời gốc", nothing to go back to.
    expect(html).not.toContain(form.RESTORE_BUTTON);
  });

  it("câu CHƯA sửa lời đang tắt: mờ, viên 'Đang tắt — dùng lời gốc', nút 'Bật lại'; không 'Đã sửa lời'", () => {
    const html = card(message({ overridden: false, is_active: false }));
    expect(html).toMatch(/<article class="[^"]*opacity-60"/);
    expect(html).toContain(form.SWITCHED_OFF_BADGE);
    expect(html).not.toContain(form.OVERRIDDEN_BADGE);
    expect(buttonOf(html, "Bật lại")).not.toContain(DISABLED_ATTR);
    expect(html).not.toMatch(/>Tắt<\/button>/);
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
