// @vitest-environment jsdom
//
// jsdom: the drawer's `Xem` is a click that awaits a read and swaps text in place — an event and a
// state change, which a static render cannot show. The picker and handover lines are static markup.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { renderToStaticMarkup } from "react-dom/server";
import { afterEach, beforeAll, describe, expect, it, vi } from "vitest";

import { filterStaffOptions, staffOptions } from "@/components/staff-combobox-logic";
import { nhanLuaChonCanBo } from "@/features/phan-anh/nhan-phieu";
import type { StaffEmailResult } from "@/lib/api/danh-ba-chon-nguoi";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_boPhanRa,
  identity_canBoChonNguoiRa,
  identity_danhBaChonNguoiRa,
  petitions_nhiemVuRa,
} from "@/lib/api/schema.gen";

import { BANG_NHAN_MAC_DINH, taskStaffOptionLabel } from "./nhan-nhiem-vu";
import { StaffEmailReveal, STAFF_EMAIL_REVEAL_LABEL } from "./staff-email";
import { assignmentFormFromTask } from "./task-assignment";
import { TaskAssignmentForm } from "./task-assignment-block";
import { TaskPersonPicker } from "./task-person-picker";

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const UNIT = "01JBOPHANA";
const STAFF_A = "CB-2026-AAAAAA";
const STAFF_B = "CB-2026-BBBBBB";
const STAFF: identity_canBoChonNguoiRa[] = [
  { code: STAFF_A, full_name: "Nguyễn Văn A", position: "Chuyên viên", department_id: UNIT, email_masked: "n***@example.vn" },
  { code: STAFF_B, full_name: "Trần Thị B", position: "", department_id: UNIT, email_masked: null },
];

/* ── The drawer's `Xem` ──────────────────────────────────────────────────────────────────────── */

let root: Root | null = null;
let host: HTMLDivElement | null = null;

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

function mount(canReveal: boolean, reveal: (code: string) => Promise<StaffEmailResult>): HTMLDivElement {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  act(() => root!.render(<StaffEmailReveal code={STAFF_A} masked="n***@example.vn" canReveal={canReveal} reveal={reveal} />));
  return host;
}

function button(el: HTMLElement): HTMLButtonElement | null {
  return el.querySelector(`button[aria-label="${STAFF_EMAIL_REVEAL_LABEL}"]`);
}

async function click(el: HTMLElement): Promise<void> {
  await act(async () => {
    button(el)!.click();
  });
}

describe("drawer `Xem` (ADR 0082 #3, #10)", () => {
  it("success: asks for THIS code once, the full address replaces the masked one in place, the button goes", async () => {
    const reveal = vi.fn(async () => ({ ok: true, duLieu: { email: "nguyen.van.a@example.vn" } }) as const);
    const el = mount(true, reveal);
    expect(el.textContent).toContain("n***@example.vn");
    expect(reveal).not.toHaveBeenCalled(); // never on render: every call is an audited read
    await click(el);
    expect(reveal).toHaveBeenCalledTimes(1);
    expect(reveal).toHaveBeenCalledWith(STAFF_A);
    expect(el.textContent).toContain("nguyen.van.a@example.vn");
    expect(el.textContent).not.toContain("n***@example.vn");
    expect(button(el)).toBeNull();
    expect(el.querySelector('[role="alert"]')).toBeNull();
  });

  it("403: the button goes, the masked address stays, no message", async () => {
    const el = mount(true, async () => ({ ok: false, thongBao: "Bạn không có quyền.", forbidden: true }));
    await click(el);
    expect(button(el)).toBeNull();
    expect(el.textContent).toContain("n***@example.vn");
    expect(el.textContent).not.toContain("Bạn không có quyền.");
  });

  it("404: the server's short sentence inline, the masked address stays", async () => {
    const el = mount(true, async () => ({ ok: false, thongBao: "Không tìm thấy cán bộ." }));
    await click(el);
    expect(el.querySelector('[role="alert"]')?.textContent).toBe("Không tìm thấy cán bộ.");
    expect(el.textContent).toContain("n***@example.vn");
    expect(button(el)).toBeNull();
  });

  it("DENIED — no `task.read`: the masked address only, no button, nothing asked", () => {
    const reveal = vi.fn(async (): Promise<StaffEmailResult> => ({ ok: false, thongBao: "x" }));
    const el = mount(false, reveal);
    expect(el.textContent).toBe("n***@example.vn");
    expect(button(el)).toBeNull();
    expect(reveal).not.toHaveBeenCalled();
  });
});

/* ── The create form's picker ────────────────────────────────────────────────────────────────── */

function picker(value: string): string {
  return renderToStaticMarkup(
    <TaskPersonPicker
      id="giao-nguoi-thuc-hien"
      label="Người thực hiện"
      emptyLabel="— Chưa chọn —"
      value={value}
      directory={STAFF}
      labelClassName=""
      onChange={() => {}}
    />,
  );
}

describe("picker lines (prototype `PersonPicker.tsx:83-86,158-167`)", () => {
  it("option: line 1 `Họ tên · Chức danh`, line 2 the masked address in 11px muted; no address → one line", () => {
    const html = picker("");
    expect(html).toMatch(
      /data-value="CB-2026-AAAAAA"[\s\S]*?<span class="text-navy block truncate">Nguyễn Văn A · Chuyên viên<\/span><span class="text-ink-muted block truncate text-\[11px\]">n\*\*\*@example\.vn<\/span>/,
    );
    expect(html).toMatch(/data-value="CB-2026-BBBBBB"[\s\S]*?<span class="text-navy min-w-0 truncate">Trần Thị B<\/span><\/li>/);
    // No reveal in the picker (ADR 0082 #10).
    expect(html).not.toContain("Xem email");
  });

  it("chosen box: `Họ tên — email_masked`; an officer without an address keeps `Họ tên · chức vụ`", () => {
    expect(picker(STAFF_A)).toMatch(/<span class="text-navy">Nguyễn Văn A<\/span><span class="text-ink-muted"> — n\*\*\*@example\.vn<\/span>/);
    expect(picker(STAFF_B)).toMatch(/<span class="text-navy min-w-0 flex-1 truncate">Trần Thị B<\/span>/);
  });

  it("typing never matches the address", () => {
    const options = staffOptions(STAFF, "");
    expect(filterStaffOptions(options, "example")).toEqual([]);
    expect(filterStaffOptions(options, "n***")).toEqual([]);
    expect(filterStaffOptions(options, "nguyen").map((o) => o.value)).toEqual([STAFF_A]);
  });
});

/* ── The drawer's handover select ────────────────────────────────────────────────────────────── */

describe("handover select line (prototype `HandoverFields.tsx:87-92`)", () => {
  it("`Họ tên — email_masked · Chức danh`, and the shared Phản ánh builder is unchanged", () => {
    expect(taskStaffOptionLabel(STAFF[0]!)).toBe("Nguyễn Văn A — n***@example.vn · Chuyên viên");
    expect(taskStaffOptionLabel(STAFF[1]!)).toBe("Trần Thị B");
    expect(taskStaffOptionLabel({ ...STAFF[1]!, position: "Văn thư" })).toBe("Trần Thị B · Văn thư");
    expect(nhanLuaChonCanBo(STAFF[0]!)).toBe("Nguyễn Văn A · Chuyên viên");
  });

  it("the select draws it", () => {
    const task = { code: "NV19", unit: UNIT, assignee: "", status: "dang-thuc-hien" } as petitions_nhiemVuRa;
    const units: identity_boPhanRa[] = [{ id: UNIT, code: "vp", name: "VĂN PHÒNG", parent_id: "", order: 0, staff_count: 0 }];
    const directory: KetQua<identity_danhBaChonNguoiRa> = { ok: true, duLieu: { items: STAFF } };
    const html = renderToStaticMarkup(
      <TaskAssignmentForm
        task={task}
        units={units}
        directory={directory}
        labels={BANG_NHAN_MAC_DINH}
        form={assignmentFormFromTask(task)}
        change={() => {}}
        sending={false}
        submit={() => {}}
      />,
    );
    expect(html).toContain('<option value="CB-2026-AAAAAA">Nguyễn Văn A — n***@example.vn · Chuyên viên</option>');
    expect(html).toContain('<option value="CB-2026-BBBBBB">Trần Thị B</option>');
  });
});
