// @vitest-environment jsdom
//
// Block "Thời hạn giải quyết đơn thư" (ADR 0084 #3, ADR 0085). jsdom: the save and the reason step are
// clicks and typing, and the gate is whether a request is sent at all — neither shows in a markup string.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import {
  afterEach,
  beforeAll,
  beforeEach,
  describe,
  expect,
  it,
  vi,
} from "vitest";

import type { identity_citizenLetterDeadlineRuleOut } from "@/lib/api/schema.gen";

const api = vi.hoisted(() => ({
  read: vi.fn(),
  create: vi.fn(),
  update: vi.fn(),
  remove: vi.fn(),
}));

vi.mock("@/lib/api/citizen-letter-deadline-rules", () => ({
  readCitizenLetterDeadlineRules: api.read,
  createCitizenLetterDeadlineRule: api.create,
  updateCitizenLetterDeadlineRule: api.update,
  removeCitizenLetterDeadlineRule: api.remove,
}));
vi.mock("sonner", () => ({ toast: { success: vi.fn() } }));

import { CitizenLetterDeadlineBlock } from "./citizen-letter-deadline-block";
import {
  AMOUNT_NOT_POSITIVE,
  BLOCK_HELP,
  BLOCK_TITLE,
  NOT_SET,
} from "./citizen-letter-deadline";
// vi-name-ok: imports the existing export of nhan-thoi-han.ts unchanged (rule 12 invariant 3)
import { LOI_THIEU_LY_DO } from "./nhan-thoi-han";

beforeAll(() => {
  (
    globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }
  ).IS_REACT_ACT_ENVIRONMENT = true;
});

const COMPLAINT_RULE: identity_citizenLetterDeadlineRuleOut = {
  id: "01J000000000000000000RULE1",
  letter_type: "khieu-nai",
  deadline_kind: "giai-quyet",
  amount: 30,
  unit: "ngay-lich",
  required_unit: "ngay-lich",
  problem: null,
};

let root: Root | null = null;
let host: HTMLDivElement | null = null;

beforeEach(() => {
  api.read.mockReset();
  api.create.mockReset();
  api.update.mockReset();
  api.remove.mockReset();
});

afterEach(() => {
  act(() => root?.unmount());
  host?.remove();
  root = null;
  host = null;
});

async function mount(
  canWrite: boolean,
  rules: identity_citizenLetterDeadlineRuleOut[] = [],
): Promise<void> {
  api.read.mockResolvedValue({ ok: true, duLieu: { items: rules } });
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
  await act(async () => {
    root!.render(<CitizenLetterDeadlineBlock canWrite={canWrite} />);
  });
}

function bodyRows(): HTMLTableRowElement[] {
  return [...host!.querySelectorAll<HTMLTableRowElement>("tbody tr")].filter(
    (tr) => tr.cells.length === 4,
  );
}

function rowOf(type: string, kind: string): HTMLTableRowElement {
  const row = bodyRows().find(
    (tr) =>
      tr.cells[0]!.textContent === type && tr.cells[1]!.textContent === kind,
  );
  if (row === undefined) throw new Error(`no row ${type} / ${kind}`);
  return row;
}

function button(scope: ParentNode, text: string): HTMLButtonElement {
  const found = [...scope.querySelectorAll<HTMLButtonElement>("button")].find(
    (b) => b.textContent === text,
  );
  if (found === undefined) throw new Error(`no button ${text}`);
  return found;
}

function type(el: HTMLInputElement, value: string): void {
  act(() => {
    Object.getOwnPropertyDescriptor(
      HTMLInputElement.prototype,
      "value",
    )!.set!.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function click(el: HTMLElement): Promise<void> {
  await act(async () => {
    el.click();
  });
}

describe("Thời hạn giải quyết đơn thư", () => {
  it("vẽ đủ sáu dòng (4 loại đơn × loại hạn được phép), dòng chưa đặt ghi 'Không đặt hạn'", async () => {
    await mount(true, [COMPLAINT_RULE]);

    expect(host!.textContent).toContain(BLOCK_TITLE);
    expect(host!.textContent).toContain(BLOCK_HELP);
    const rows = bodyRows().map((tr) => [
      tr.cells[0]!.textContent,
      tr.cells[1]!.textContent,
      tr.cells[2]!.textContent,
    ]);
    expect(rows).toEqual([
      ["Kiến nghị, phản ánh", "Hạn xử lý đơn", NOT_SET],
      ["Khiếu nại", "Hạn xử lý đơn", NOT_SET],
      ["Khiếu nại", "Hạn giải quyết (từ ngày thụ lý)", "30 ngày"],
      ["Tố cáo", "Hạn xử lý đơn", NOT_SET],
      ["Tố cáo", "Hạn giải quyết (từ ngày thụ lý)", NOT_SET],
      ["Đề nghị", "Hạn xử lý đơn", NOT_SET],
    ]);
    // Kiến nghị, phản ánh and Đề nghị have no "Hạn giải quyết" row (ADR 0085 câu 3).
    expect(
      bodyRows().filter((tr) => tr.cells[0]!.textContent === "Đề nghị"),
    ).toHaveLength(1);
  });

  it("lưu số ngày cho dòng chưa đặt → POST với đơn vị khoá theo loại, kèm Idempotency-Key", async () => {
    api.create.mockResolvedValue({
      ok: true,
      duLieu: { ...COMPLAINT_RULE, id: "NEW" },
    });
    await mount(true);

    await click(
      rowOf(
        "Kiến nghị, phản ánh",
        "Hạn xử lý đơn",
      ).querySelector<HTMLButtonElement>('button[title="Sửa thời hạn"]')!,
    );
    const row = rowOf("Kiến nghị, phản ánh", "Hạn xử lý đơn");
    expect(row.cells[2]!.textContent).toContain("ngày làm việc");
    type(row.querySelector<HTMLInputElement>('input[name="amount"]')!, "15");
    await click(button(row, "Lưu"));

    expect(api.create).toHaveBeenCalledTimes(1);
    expect(api.create.mock.calls[0]![0]).toEqual({
      letter_type: "kien-nghi-phan-anh",
      deadline_kind: "xu-ly-don",
      amount: 15,
      unit: "ngay-lam-viec",
    });
    expect(typeof api.create.mock.calls[0]![1]).toBe("string");
    expect(api.read).toHaveBeenCalledTimes(2); // read again after the write
  });

  it("lưu dòng đã đặt → PATCH chỉ số ngày", async () => {
    api.update.mockResolvedValue({
      ok: true,
      duLieu: { ...COMPLAINT_RULE, amount: 45 },
    });
    await mount(true, [COMPLAINT_RULE]);

    const label = "Hạn giải quyết (từ ngày thụ lý)";
    await click(
      rowOf("Khiếu nại", label).querySelector<HTMLButtonElement>(
        'button[title="Sửa thời hạn"]',
      )!,
    );
    const row = rowOf("Khiếu nại", label);
    type(row.querySelector<HTMLInputElement>('input[name="amount"]')!, "45");
    await click(button(row, "Lưu"));

    expect(api.update).toHaveBeenCalledWith(COMPLAINT_RULE.id, { amount: 45 });
    expect(api.create).not.toHaveBeenCalled();
  });

  it.each(["0", "-3", ""])(
    "từ chối số ngày không dương (%j) tại chỗ, không gửi gì",
    async (value) => {
      await mount(true);

      await click(
        rowOf("Tố cáo", "Hạn xử lý đơn").querySelector<HTMLButtonElement>(
          'button[title="Sửa thời hạn"]',
        )!,
      );
      type(
        rowOf("Tố cáo", "Hạn xử lý đơn").querySelector<HTMLInputElement>(
          'input[name="amount"]',
        )!,
        value,
      );
      await click(button(rowOf("Tố cáo", "Hạn xử lý đơn"), "Lưu"));

      expect(host!.textContent).toContain(AMOUNT_NOT_POSITIVE);
      expect(api.create).not.toHaveBeenCalled();
      expect(api.update).not.toHaveBeenCalled();
    },
  );

  it("xoá hỏi lý do: trống thì từ chối, có lý do thì xoá mềm với lý do ấy", async () => {
    api.remove.mockResolvedValue({ ok: true, duLieu: null });
    await mount(true, [COMPLAINT_RULE]);

    const label = "Hạn giải quyết (từ ngày thụ lý)";
    // Only a set row offers removal.
    expect(
      rowOf("Tố cáo", "Hạn xử lý đơn").querySelector(
        'button[title="Xoá thời hạn"]',
      ),
    ).toBeNull();
    await click(
      rowOf("Khiếu nại", label).querySelector<HTMLButtonElement>(
        'button[title="Xoá thời hạn"]',
      )!,
    );
    expect(api.remove).not.toHaveBeenCalled();

    const reason = rowOf("Khiếu nại", label).querySelector<HTMLInputElement>(
      'input[name="reason"]',
    )!;
    expect(reason).not.toBeNull();
    await click(button(rowOf("Khiếu nại", label), "Xoá"));
    expect(host!.textContent).toContain(LOI_THIEU_LY_DO);
    expect(api.remove).not.toHaveBeenCalled();

    type(reason, "Chờ pháp chế đối chiếu");
    await click(button(rowOf("Khiếu nại", label), "Xoá"));
    expect(api.remove).toHaveBeenCalledWith(
      COMPLAINT_RULE.id,
      "Chờ pháp chế đối chiếu",
    );
  });

  it("dòng có `problem` hiện nguyên câu máy chủ", async () => {
    const problem =
      "hạn đơn thư: đơn vị này không được dùng cho loại đơn và loại hạn đã chọn";
    await mount(true, [{ ...COMPLAINT_RULE, unit: "ngay-lam-viec", problem }]);

    expect(host!.querySelector('[role="alert"]')?.textContent).toBe(problem);
  });

  it("không có admin.sla → khối ẩn và không gửi yêu cầu đọc nào", async () => {
    await mount(false, [COMPLAINT_RULE]);

    expect(host!.innerHTML).toBe("");
    expect(api.read).not.toHaveBeenCalled();
  });
});
