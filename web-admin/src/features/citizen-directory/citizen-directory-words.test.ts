import { describe, expect, it } from "vitest";

import { NO_CITIZEN_CONTACT_FILTER } from "@/lib/api/citizen-contacts";

import {
  CITIZEN_DIRECTORY_SUBTITLE,
  DELETE_REASON_REQUIRED,
  EMPTY_CONTACT_DRAFT,
  EMPTY_DIRECTORY,
  EMPTY_FILTERED,
  IMPORT_EMPTY_FILE,
  IMPORT_PREVIEW_UNEXPLAINED,
  PHONE_TAKEN_TOAST,
  SAVE_FAILED_TOAST,
  TAGS_WARNING,
  ZALO_COLUMN_TOOLTIP,
  canSubmitContact,
  deleteReasonProblem,
  emptySentence,
  importButtonLabel,
  importPreviewHeading,
  importPreviewReport,
  importedToast,
  parseTags,
  saveFailureToast,
  tabLabel,
  tagsCellText,
  toggleGroup,
} from "./citizen-directory-words";

/** Chapter 16's sentences and the pure decisions of the screen — pinned verbatim. */

describe("sentences (chapter 16, ADR 0097)", () => {
  it("the chapter's subtitle (D3/D13), the Zalo tooltip (D13), the fixed tags warning (D4)", () => {
    expect(CITIZEN_DIRECTORY_SUBTITLE).toBe(
      "Số điện thoại để xã liên lạc với bà con, phân theo nhóm và gắn nhãn. Cột Zalo cho biết số đã đăng nhập Mini App của xã.",
    );
    expect(ZALO_COLUMN_TOOLTIP).toBe("Đã đăng nhập Mini App của xã bằng số này");
    expect(TAGS_WARNING).toBe("Không tạo nhãn về sức khoẻ, dân tộc, tôn giáo, chính trị");
  });

  it("tab label: the bracket only with a server figure, formatted vi-VN", () => {
    expect(tabLabel("Danh sách", null)).toBe("Danh sách");
    expect(tabLabel("Danh sách", 0)).toBe("Danh sách (0)");
    expect(tabLabel("Nhóm", 1234)).toBe("Nhóm (1.234)");
  });

  it("empty sentences: the directory's only when nothing narrows the list", () => {
    expect(emptySentence(NO_CITIZEN_CONTACT_FILTER)).toBe(EMPTY_DIRECTORY);
    expect(emptySentence({ ...NO_CITIZEN_CONTACT_FILTER, keyword: "An" })).toBe(EMPTY_FILTERED);
    expect(emptySentence({ ...NO_CITIZEN_CONTACT_FILTER, hasOwnAppSession: true })).toBe(EMPTY_FILTERED);
    expect(EMPTY_DIRECTORY).toBe("Danh bạ chưa có ai. Bấm “Thêm người dân” để bắt đầu.");
  });

  it("tags cell: joined by a comma, or the dash", () => {
    expect(tagsCellText(["tổ trưởng", "hộ kinh doanh"])).toBe("tổ trưởng, hộ kinh doanh");
    expect(tagsCellText([])).toBe("—");
  });
});

describe("the dialog's decisions", () => {
  it("tags: split on commas, trimmed, blanks dropped (prototype)", () => {
    expect(parseTags(" hộ nghèo, tổ trưởng ,, hộ kinh doanh ")).toEqual(["hộ nghèo", "tổ trưởng", "hộ kinh doanh"]);
    expect(parseTags("")).toEqual([]);
  });

  it("groups toggle in ticking order", () => {
    expect(toggleGroup([], "a")).toEqual(["a"]);
    expect(toggleGroup(["a", "b"], "a")).toEqual(["b"]);
  });

  it("submit: name and ≥ 8 characters of number when adding; when EDITING also the revealed number", () => {
    const ok = { ...EMPTY_CONTACT_DRAFT, fullName: "Nguyễn Văn A", phone: "0900000001" };
    expect(canSubmitContact(ok, false, false)).toBe(true);
    expect(canSubmitContact({ ...ok, fullName: " " }, false, false)).toBe(false);
    expect(canSubmitContact({ ...ok, phone: "0900" }, false, false)).toBe(false);
    expect(canSubmitContact(ok, true, false)).toBe(false);
    expect(canSubmitContact(ok, true, true)).toBe(true);
  });

  it("a refused save: the chapter's sentence for phone_taken, its other sentence for the rest", () => {
    expect(saveFailureToast("phone_taken")).toBe(PHONE_TAKEN_TOAST);
    expect(saveFailureToast("invalid_request")).toBe(SAVE_FAILED_TOAST);
    expect(saveFailureToast(undefined)).toBe(SAVE_FAILED_TOAST);
    expect(PHONE_TAKEN_TOAST).toBe("Số điện thoại này đã có trong danh bạ.");
  });

  it("a delete reason is mandatory — whitespace is not a reason", () => {
    expect(deleteReasonProblem("")).toBe(DELETE_REASON_REQUIRED);
    expect(deleteReasonProblem("   ")).toBe(DELETE_REASON_REQUIRED);
    expect(deleteReasonProblem("Nhập trùng")).toBeNull();
  });
});

describe("the import's report", () => {
  const err = { row: 4, column: "Số điện thoại", message: "Số này đã có trong danh bạ." };

  it("a clean preview → a report of n rows; an empty one → a sentence; unexplained refusal → a sentence", () => {
    expect(importPreviewReport({ valid: true, rowCount: 3, errors: [] })).toEqual({
      report: { total_rows: 3, created: 0, committed: false, errors: [], codes: [] },
    });
    expect(importPreviewReport({ valid: true, rowCount: 0, errors: [] })).toEqual({ problem: IMPORT_EMPTY_FILE });
    expect(importPreviewReport({ valid: false, rowCount: 0, errors: [] })).toEqual({ problem: IMPORT_PREVIEW_UNEXPLAINED });
    expect(importPreviewReport({ valid: false, rowCount: 5, errors: [err] })).toEqual({
      report: { total_rows: 5, created: 0, committed: false, errors: [err], codes: [] },
    });
  });

  it("headings, the button and the toast name the unit `người dân`", () => {
    expect(importPreviewHeading({ total_rows: 3, created: 0, committed: false, errors: [], codes: [] })).toBe("3 dòng hợp lệ, sẵn sàng nhập");
    expect(importPreviewHeading({ total_rows: 5, created: 0, committed: false, errors: [err], codes: [] })).toBe("1 dòng sai trên tổng số 5 dòng");
    expect(importPreviewHeading({ total_rows: 0, created: 0, committed: false, errors: [err], codes: [] })).toBe("1 dòng sai");
    expect(importPreviewHeading({ total_rows: 0, created: 0, committed: false, errors: [{ ...err, row: 0 }], codes: [] })).toBe("Tệp chưa nhập được");
    expect(importButtonLabel(null)).toBe("Nhập");
    expect(importButtonLabel({ total_rows: 3, created: 0, committed: false, errors: [], codes: [] })).toBe("Nhập 3 người dân");
    expect(importedToast(3)).toBe("Đã nhập 3 người dân.");
    expect(importedToast(null)).toContain("đã được nhập ở lần gửi trước");
  });
});
