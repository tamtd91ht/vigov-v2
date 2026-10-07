import { renderToStaticMarkup } from "react-dom/server";
import { describe, expect, it } from "vitest";

import type { petitions_deNghiLuiHanRa, petitions_nhatKyNhiemVuRa } from "@/lib/api/schema.gen";

import { handoverOf } from "./nhat-ky-nhiem-vu";
import { TimelineAttachments, removeFileLabel } from "./task-attachments-ui";
import { EXTENSION_HISTORY_TITLE, TaskExtensionHistoryView, extensionStatusText } from "./task-extension-block";
import { elapsedText, statusEnteredAt } from "./task-status-pipeline";

/**
 * Review round 1 (07/10/2026), what the timeline gives the drawer: the time in the current status
 * (F-2), the hand-over "A → B" (G-3); and the two read/write paths added with their clients — extension
 * history (F-3) and file removal (G-2).
 */

function row(p: Partial<petitions_nhatKyNhiemVuRa>): petitions_nhatKyNhiemVuRa {
  return { id: "r", at: "2026-09-01T00:00:00Z", actor_code: "CB-1", status: "moi-giao", unit: "", assignee: "", note: "", attachments: [], ...p };
}

describe("F-2 — time in the current status, from the timeline (prototype `TaskStatusPipeline.tsx:62-86`)", () => {
  const task = { status: "dang-thuc-hien", created_at: "2026-08-01T00:00:00Z" };
  const rows = [
    row({ id: "3", at: "2026-09-10T00:00:00Z", status: "dang-thuc-hien" }),
    row({ id: "2", at: "2026-09-05T00:00:00Z", status: "dang-thuc-hien" }),
    row({ id: "1", at: "2026-09-01T00:00:00Z", status: "da-tiep-nhan" }),
  ];

  it("the oldest row of the newest run in the current status", () => {
    expect(statusEnteredAt(task, rows, false)).toBe("2026-09-05T00:00:00Z");
  });

  it("the whole page in the current status: older rows unknown → null; the full history → its first row", () => {
    expect(statusEnteredAt(task, rows.slice(0, 2), false)).toBeNull();
    expect(statusEnteredAt(task, rows.slice(0, 2), true)).toBe("2026-09-05T00:00:00Z");
  });

  it("no rows at all: `created_at` once the history is known complete, else unknown", () => {
    expect(statusEnteredAt(task, [], true)).toBe("2026-08-01T00:00:00Z");
    expect(statusEnteredAt(task, [], false)).toBeNull();
  });

  it("`n ngày m giờ` · `n giờ` · `vừa xong`; unknown stays unknown", () => {
    const now = new Date("2026-09-15T06:00:00Z");
    expect(elapsedText("2026-09-05T00:00:00Z", now)).toBe("10 ngày 6 giờ");
    expect(elapsedText("2026-09-15T03:00:00Z", now)).toBe("3 giờ");
    expect(elapsedText("2026-09-15T05:59:00Z", now)).toBe("vừa xong");
    expect(elapsedText(null, now)).toBeNull();
    expect(elapsedText("2026-09-16T00:00:00Z", now)).toBeNull();
  });
});

describe("G-3 — hand-over `A → B`, never an arrow with nothing before it", () => {
  const unit = (id: string) => (id === "" ? "Chưa giao bộ phận" : `BP ${id}`);
  const staff = (code: string) => (code === "" ? "Chưa phân công" : `Tên ${code}`);

  it("the previous holder is the nearest OLDER assignment row; only the side that changed", () => {
    const rows = [
      row({ id: "3", unit: "B", assignee: "CB-2" }),
      row({ id: "2" }),
      row({ id: "1", unit: "A", assignee: "CB-2" }),
    ];
    expect(handoverOf(rows, 0, unit, staff)).toEqual({ unit: { from: "BP A", to: "BP B" }, assignee: null });
  });

  it("no older assignment row loaded: the new holder with `from: null`", () => {
    expect(handoverOf([row({ unit: "A", assignee: "CB-1" })], 0, unit, staff)).toEqual({
      unit: { from: null, to: "BP A" },
      assignee: { from: null, to: "Tên CB-1" },
    });
  });

  it("a row that does not touch the assignment: no hand-over", () => {
    expect(handoverOf([row({})], 0, unit, staff)).toBeNull();
  });
});

describe("F-3 — Lịch sử gia hạn (GET /api/v1/tasks/{ma}/extensions)", () => {
  const item: petitions_deNghiLuiHanRa = {
    id: "1",
    requested_by: "CB-1",
    new_due_at: "2026-07-20T23:59:59+07:00",
    reason: "Chờ số liệu",
    status: "da-duyet",
    requested_at: "2026-06-18T02:00:00Z",
    decided_at: "2026-06-19T02:00:00Z",
    decision_note: "Đồng ý",
  };

  it("drawn only when there is a request; the server's codes in words; no arrow without a `from`", () => {
    expect(renderToStaticMarkup(<TaskExtensionHistoryView load={null} />)).toBe("");
    expect(renderToStaticMarkup(<TaskExtensionHistoryView load={[]} />)).toBe("");
    const html = renderToStaticMarkup(<TaskExtensionHistoryView load={[item, { ...item, id: "2", status: "tu-choi" }]} />);
    expect(html).toContain(`>${EXTENSION_HISTORY_TITLE}</h3>`);
    expect(html).toContain("Lùi tới 20/7/2026");
    expect(html).not.toContain("→");
    expect(html).toContain(">đã duyệt</span>");
    expect(html).toContain(">từ chối</span>");
    expect(html).toContain("Ghi chú quyết định: Đồng ý");
    expect(extensionStatusText("cho-duyet")).toBe("chờ duyệt");
    expect(extensionStatusText("la")).toBe("la");
  });

  it("a failed read says the server's sentence — never silence", () => {
    expect(renderToStaticMarkup(<TaskExtensionHistoryView load="Không đủ quyền." />)).toContain(
      'role="alert">Không đủ quyền.</p>',
    );
  });
});

describe("G-2 — a file's remove control (ADR 0076 #4b): only with `onRemove`, opening the reason dialog", () => {
  const file = { id: "f1", file_name: "bien-ban.pdf", mime_type: "application/pdf", size_bytes: 10, status: "da-luu" } as never;

  it("DENIED (no `onRemove`): no remove control; ALLOWED: `Gỡ tệp {name}`, `aria-haspopup=dialog`", () => {
    expect(renderToStaticMarkup(<TimelineAttachments taskCode="NV19" attachments={[file]} />)).not.toContain(
      removeFileLabel("bien-ban.pdf"),
    );
    const html = renderToStaticMarkup(<TimelineAttachments taskCode="NV19" attachments={[file]} onRemove={() => {}} />);
    expect(html).toContain(`aria-label="${removeFileLabel("bien-ban.pdf")}" aria-haspopup="dialog"`);
    expect(html).toContain("lucide-trash");
  });
});
