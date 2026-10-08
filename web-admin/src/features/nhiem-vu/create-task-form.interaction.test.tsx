// @vitest-environment jsdom
//
// jsdom for this file: what it pins — a press of `Giao việc` that sends nothing, focus landing on
// the first invalid field, errors clearing as the clerk types, a catalogue arriving after the form
// opened — are events, focus and re-renders, none of which a string render can show. Same reasoning
// as `task-detail-dialog.test.tsx`.

import { act } from "react";
import { createRoot, type Root } from "react-dom/client";
import { afterEach, beforeAll, beforeEach, describe, expect, it, vi } from "vitest";

import type { KetQua } from "@/lib/api/goi";
import type {
  identity_danhBaChonNguoiRa,
  petitions_loaiNhiemVuRa,
  petitions_taoNhiemVuVao,
} from "@/lib/api/schema.gen";

import {
  DOCUMENT_SUMMARY_MISSING,
  NEW_TASK_DUE_INCOMPLETE,
  TASK_CONTENT_MISSING,
  TASK_TITLE_MISSING,
  TASK_TYPE_MISSING,
  TASK_TYPE_PLACEHOLDER,
} from "./nhan-nhiem-vu";
import { FormGiaoViec, type DanhMucNhiemVu } from "./so-nhiem-vu"; // vi-name-ok: existing names, imported not declared (rule 12 inv 3)

beforeAll(() => {
  (globalThis as unknown as { IS_REACT_ACT_ENVIRONMENT: boolean }).IS_REACT_ACT_ENVIRONMENT = true;
});

const DIRECTORY: KetQua<identity_danhBaChonNguoiRa> = {
  ok: true,
  duLieu: { items: [{ code: "CB-0001", full_name: "Trần Văn Lãnh", position: "Chủ tịch", department_id: "" }] },
};

function typeRow(code: string, label: string, isDefault: boolean, active = true): petitions_loaiNhiemVuRa {
  // `requires_directive` as the server derives it (true for `theo-van-ban` only).
  return {
    id: `01J${code}`,
    code,
    label,
    is_default: isDefault,
    active,
    order: 1,
    source: "he-thong",
    tier: 1,
    color: null,
    requires_directive: code === "theo-van-ban",
  };
}

function catalogue(loai: petitions_loaiNhiemVuRa[]): DanhMucNhiemVu {
  return { loai, mucUuTien: [], khoi: [], boPhan: [] };
}

const NO_DEFAULT = catalogue([typeRow("co-ban", "Nhiệm vụ cơ bản", false), typeRow("theo-van-ban", "Theo văn bản", false)]);
const BASIC_DEFAULT = catalogue([typeRow("theo-van-ban", "Theo văn bản", false), typeRow("co-ban", "Nhiệm vụ cơ bản", true)]);
const DOCUMENT_DEFAULT = catalogue([typeRow("co-ban", "Nhiệm vụ cơ bản", false), typeRow("theo-van-ban", "Theo văn bản", true)]);

let host: HTMLDivElement;
let root: Root;

beforeEach(() => {
  host = document.createElement("div");
  document.body.append(host);
  root = createRoot(host);
});

afterEach(() => {
  act(() => root.unmount());
  host.remove();
});

type Sent = { body: petitions_taoNhiemVuVao; key: string };

function draw(
  danhMuc: DanhMucNhiemVu,
  opts: { sent?: Sent[]; documents?: boolean; loi?: string | null } = {},
): void {
  const sent = opts.sent ?? [];
  act(() =>
    root.render(
      <FormGiaoViec
        danhMuc={danhMuc}
        danhBa={DIRECTORY}
        danhBaLanhDao={DIRECTORY}
        coDanhSachVanBan={opts.documents ?? false}
        dangGui={false}
        loi={opts.loi ?? null}
        huy={() => {}}
        giaoViec={(body, key) => sent.push({ body, key })}
      />,
    ),
  );
}

function byId<T extends HTMLElement>(id: string): T {
  const el = document.getElementById(id);
  if (el === null) throw new Error(`no #${id}`);
  return el as T;
}

/** Types into a React-controlled box: the native setter, then the event React listens to. */
function typeInto(id: string, value: string): void {
  const el = byId<HTMLInputElement | HTMLTextAreaElement>(id);
  const proto = el instanceof HTMLTextAreaElement ? HTMLTextAreaElement.prototype : HTMLInputElement.prototype;
  act(() => {
    Object.getOwnPropertyDescriptor(proto, "value")!.set!.call(el, value);
    el.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

function choose(id: string, value: string): void {
  const el = byId<HTMLSelectElement>(id);
  act(() => {
    el.value = value;
    el.dispatchEvent(new Event("change", { bubbles: true }));
  });
}

function submitButton(): HTMLButtonElement {
  const b = host.querySelector<HTMLButtonElement>('button[type="submit"]');
  if (b === null) throw new Error("no submit button");
  return b;
}

function press(): void {
  act(() => submitButton().click());
}

/** The error line a field points at through `aria-describedby`, or `null`. */
function errorOf(id: string): string | null {
  const el = byId(id);
  const describedBy = el.getAttribute("aria-describedby");
  if (el.getAttribute("aria-invalid") !== "true" || describedBy === null) return null;
  return document.getElementById(describedBy)?.textContent ?? null;
}

describe("Giao việc mới — loại mặc định của xã (người dùng chốt 07/10/2026)", () => {
  it("chọn sẵn dòng `is_default` đang dùng", () => {
    draw(BASIC_DEFAULT);
    expect(byId<HTMLSelectElement>("giao-loai").value).toBe("co-ban");
    expect(host.textContent).not.toContain(TASK_TYPE_PLACEHOLDER);
  });

  it("danh mục về SAU lúc form mở: chọn sẵn ngay khi về", () => {
    draw(catalogue([]));
    expect(byId<HTMLSelectElement>("giao-loai").value).toBe("");
    draw(BASIC_DEFAULT);
    expect(byId<HTMLSelectElement>("giao-loai").value).toBe("co-ban");
  });

  it("loại cán bộ đã chọn KHÔNG bị mặc định đè, kể cả khi danh mục đọc lại", () => {
    draw(BASIC_DEFAULT);
    choose("giao-loai", "theo-van-ban");
    draw(catalogue([...BASIC_DEFAULT.loai]));
    expect(byId<HTMLSelectElement>("giao-loai").value).toBe("theo-van-ban");
  });

  it("không dòng mặc định: `— Chọn loại —`; bấm thì báo DƯỚI ô loại, không gọi API", () => {
    const sent: Sent[] = [];
    draw(NO_DEFAULT, { sent });
    expect(byId<HTMLSelectElement>("giao-loai").value).toBe("");
    typeInto("giao-tieu-de", "Rà soát hộ nghèo");
    press();
    expect(sent).toHaveLength(0);
    expect(errorOf("giao-loai")).toBe(TASK_TYPE_MISSING);
    expect(document.activeElement?.id).toBe("giao-loai");
  });

  it("dòng mặc định đã NGỪNG dùng thì bỏ qua", () => {
    draw(catalogue([typeRow("co-ban", "Nhiệm vụ cơ bản", true, false), typeRow("theo-van-ban", "Theo văn bản", false)]));
    expect(byId<HTMLSelectElement>("giao-loai").value).toBe("");
  });

  it("mặc định `theo-van-ban`: form mở ra đã có nhóm `sản phẩm đầu ra` và `Ghi chú`", () => {
    draw(DOCUMENT_DEFAULT, { documents: true });
    expect(byId<HTMLSelectElement>("giao-loai").value).toBe("theo-van-ban");
    expect(document.getElementById("giao-nhom-van-ban-san-pham-dau-ra")).not.toBeNull();
    expect(document.getElementById("giao-ghi-chu")).not.toBeNull();
  });
});

describe("Giao việc mới — bấm khi thiếu: lỗi dưới ô, tiêu điểm tới ô thiếu đầu tiên (07/10/2026)", () => {
  it("nút bấm được trước lần bấm, và chưa có lỗi nào hiện", () => {
    draw(NO_DEFAULT);
    expect(submitButton().disabled).toBe(false);
    expect(host.querySelector('[aria-invalid="true"]')).toBeNull();
    expect(host.textContent).not.toContain(TASK_TYPE_MISSING);
    expect(host.textContent).not.toContain(TASK_TITLE_MISSING);
  });

  it("bấm khi trống hết: không gọi API, lỗi dưới loại VÀ tên, tiêu điểm ở ô LOẠI (đầu tiên)", () => {
    const sent: Sent[] = [];
    draw(NO_DEFAULT, { sent });
    press();
    expect(sent).toHaveLength(0);
    expect(errorOf("giao-loai")).toBe(TASK_TYPE_MISSING);
    expect(errorOf("giao-tieu-de")).toBe(TASK_TITLE_MISSING);
    expect(document.activeElement?.id).toBe("giao-loai");
    // The error line sits right under its field, inside the field's own block.
    expect(byId("giao-loai").parentElement?.contains(byId("giao-loai-loi"))).toBe(true);
  });

  it("sửa từng ô thì lỗi ô ấy tự biến mất; đủ rồi thì gửi", () => {
    const sent: Sent[] = [];
    draw(NO_DEFAULT, { sent });
    press();
    choose("giao-loai", "co-ban");
    expect(errorOf("giao-loai")).toBeNull();
    expect(byId("giao-loai").hasAttribute("aria-invalid")).toBe(false);
    expect(errorOf("giao-tieu-de")).toBe(TASK_TITLE_MISSING);
    typeInto("giao-tieu-de", "Rà soát hộ nghèo");
    expect(errorOf("giao-tieu-de")).toBeNull();
    press();
    expect(sent).toHaveLength(1);
    expect(sent[0]!.body.type).toBe("co-ban");
    expect(sent[0]!.body.title).toBe("Rà soát hộ nghèo");
  });

  it("loại `theo-van-ban`: câu lỗi tên nói đúng tên ô", () => {
    draw(DOCUMENT_DEFAULT);
    press();
    expect(errorOf("giao-tieu-de")).toBe(TASK_CONTENT_MISSING);
    expect(document.activeElement?.id).toBe("giao-tieu-de");
  });

  it("hạn gõ dở: không gửi, lỗi dưới ô hạn, tiêu điểm vào ô hạn", () => {
    const sent: Sent[] = [];
    draw(BASIC_DEFAULT, { sent });
    typeInto("giao-tieu-de", "Rà soát hộ nghèo");
    const due = byId<HTMLInputElement>("giao-han");
    // A half-typed `datetime-local` reads "" with `validity.badInput` — what the browser reports.
    Object.defineProperty(due, "validity", { configurable: true, value: { badInput: true } });
    typeInto("giao-han", "");
    press();
    expect(sent).toHaveLength(0);
    expect(errorOf("giao-han")).toBe(NEW_TASK_DUE_INCOMPLETE);
    expect(document.activeElement?.id).toBe("giao-han");
  });

  it("dòng văn bản trống: lỗi dưới ĐÚNG dòng ấy, tiêu điểm vào ô trích yếu của nó", () => {
    const sent: Sent[] = [];
    draw(DOCUMENT_DEFAULT, { sent, documents: true });
    typeInto("giao-tieu-de", "Báo cáo sơ kết");
    act(() => byId("giao-them-van-ban-chi-dao-dang-uy").click());
    const row = host.querySelector<HTMLTextAreaElement>('textarea[id^="giao-van-ban-"]')!;
    act(() => submitButton().focus());
    press();
    expect(sent).toHaveLength(0);
    expect(errorOf(row.id)).toBe(DOCUMENT_SUMMARY_MISSING);
    expect(document.activeElement).toBe(row);
    typeInto(row.id, "Công văn số 416-CV/ĐU");
    expect(errorOf(row.id)).toBeNull();
    press();
    expect(sent).toHaveLength(1);
  });

  it("lỗi máy chủ vẫn là MỘT câu cạnh nút, `role=\"alert\"`", () => {
    draw(BASIC_DEFAULT, { loi: "Mã nhiệm vụ đã có." });
    const alert = host.querySelector('[role="alert"]');
    expect(alert?.textContent).toBe("Mã nhiệm vụ đã có.");
    expect(alert?.className).toContain("thong-bao-loi");
  });

  it("đang gửi thì nút khoá — chỉ lúc ấy (và lúc danh bạ còn tải)", () => {
    act(() =>
      root.render(
        <FormGiaoViec
          danhMuc={BASIC_DEFAULT}
          danhBa={DIRECTORY}
          danhBaLanhDao={DIRECTORY}
          dangGui
          loi={null}
          huy={() => {}}
          giaoViec={vi.fn()}
        />,
      ),
    );
    expect(submitButton().disabled).toBe(true);
  });
});
