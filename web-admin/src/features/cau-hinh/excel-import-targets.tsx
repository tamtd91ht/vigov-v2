"use client";

import type { ReactNode } from "react";

import type { KhoaDanhMucGhi } from "@/lib/api/danh-muc";
import type { ImportRoutes } from "@/lib/api/excel-import";
import {
  CAPITAL_PLAN_CATEGORY_IMPORT_ROUTES,
  DOCUMENT_TYPE_IMPORT_ROUTES,
  DOCUMENT_TYPE_ROWS_FIELD,
  FINANCE_CATALOGUE_ROWS_FIELD,
  IDENTITY_CATALOGUE_ROWS_FIELD,
  PETITIONS_CATALOGUE_ROWS_FIELD,
  RESIDENTIAL_UNIT_TYPE_IMPORT_ROUTES,
  TASK_BLOC_IMPORT_ROUTES,
  TASK_PRIORITY_IMPORT_ROUTES,
  TASK_TYPE_IMPORT_ROUTES,
} from "@/lib/api/lookup-catalogue-import";
import type {
  DocumentTypeImportRow,
  FinanceCatalogueImportRow,
  IdentityCatalogueImportRow,
  PetitionsCatalogueImportRow,
} from "@/lib/api/lookup-catalogue-import";
import { MAP_ASSET_TYPE_IMPORT_ROUTES, MAP_ASSET_TYPE_ROWS_FIELD } from "@/lib/api/map-asset-type-import";
import type { MapAssetTypeImportRow } from "@/lib/api/map-asset-type-import";
import { ORG_UNIT_IMPORT_ROUTES } from "@/lib/api/org-unit-import";
import { RESIDENTIAL_UNIT_IMPORT_ROUTES, RESIDENTIAL_UNIT_ROWS_FIELD } from "@/lib/api/residential-unit-import";
import type { ResidentialUnitImportRow } from "@/lib/api/residential-unit-import";
import { STAFF_IMPORT_ROUTES, STAFF_ROWS_FIELD } from "@/lib/api/staff-import";
import type { StaffImportCreatedRow, StaffImportPlannedRow } from "@/lib/api/staff-import";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_orgUnitImportPreviewOut,
  identity_orgUnitImportUnitOut,
  identity_phienHienTaiRa,
} from "@/lib/api/schema.gen";
import { QUYEN_QUAN_LY_DANH_MUC, quyetDinhTheoKhoa } from "@/lib/quyen";

import type { ImportTarget } from "./excel-import-flow";
import { ExcelImportPanel } from "./excel-import-panel";
import { loaiDonVi, nhanLoaiDonVi, nhanSoDem } from "./nhan-thon";
import { NO_EMAIL_REASON, StaffImportResult } from "./staff-import-result";
import {
  CONFIRM_IMPORT_BUTTON,
  ERRORS_HEADING,
  IMPORT_EXPLANATION,
  TEMPLATE_FILE_NAME,
  importedSentence,
  parentText,
  previewLead,
} from "./org-unit-import-flow";

/**
 * Every "⬆ Nhập từ Excel" of the configuration screen, one `ImportTarget` each (ADR 0059).
 *
 * TO WIRE ANOTHER IMPORT (another catalogue group): write its routes file
 * beside `lib/api/map-asset-type-import.ts`, add ONE target here, and — for a catalogue group — one
 * entry in `CATALOGUE_IMPORTS`. The panel, the flow and the key-per-attempt rule need no change. Every
 * Danh mục group imports today, so `PHAN_CHUA_DUNG` (`nhan-cau-hinh.ts`) no longer carries an Excel
 * item; `khoi-chua-dung.test.tsx` fails if one comes back while `CATALOGUE_IMPORTS` covers every group.
 */

/** Sơ đồ tổ chức (§1) — `admin.org`, gated by `tab-so-do-to-chuc.tsx`. */
export const ORG_UNIT_IMPORT_TARGET: ImportTarget<identity_orgUnitImportUnitOut> = {
  id: "so-do",
  title: "Nhập sơ đồ tổ chức từ Excel",
  explanation: IMPORT_EXPLANATION,
  templateFileName: TEMPLATE_FILE_NAME,
  confirmButton: CONFIRM_IMPORT_BUTTON,
  errorsHeading: ERRORS_HEADING,
  rowsLabel: "Các bộ phận sẽ tạo",
  previewLead,
  importedSentence,
  columns: [
    { header: "Dòng", cell: (u) => String(u.row) },
    { header: "Tên bộ phận", cell: (u) => u.name },
    { header: "Mã", cell: (u) => u.code, mono: true },
    { header: "Bộ phận cha", cell: parentText },
    { header: "Thứ tự", cell: (u) => String(u.order) },
  ],
  rowKey: (u) => u.row,
  routes: ORG_UNIT_IMPORT_ROUTES,
  rowsField: "units" satisfies keyof identity_orgUnitImportPreviewOut,
};

/**
 * Thôn / Tổ dân phố (§2) — `admin.org`, gated by `tab-thon-to-dan-pho.tsx`. NOT a catalogue group (a
 * unit has a name, households, a head), so it is its own target like the org chart and has no entry in
 * `CATALOGUE_IMPORTS`.
 *
 * The preview shows counts through `nhanSoDem`, the same function as the tab's table: a blank cell of
 * the spreadsheet is "Chưa nhập", never "0" — the one thing a staff member checking the preview must
 * be able to see before the file is written.
 */
export const RESIDENTIAL_UNIT_IMPORT_TARGET: ImportTarget<ResidentialUnitImportRow> = {
  id: "thon-to-dan-pho",
  title: "Nhập thôn / tổ dân phố từ Excel",
  explanation:
    "Tải tệp mẫu, điền mỗi thôn hoặc tổ dân phố một dòng rồi chọn tệp để kiểm tra. Tệp mẫu có sẵn danh " +
    "sách chọn Loại và Trưởng thôn của xã. Hệ thống kiểm tra toàn bộ tệp trước, chưa ghi gì; chỉ khi " +
    "tệp không có lỗi mới nhập được, và nhập thì nhập cả tệp hoặc không nhập gì. Nhập chỉ thêm địa bàn " +
    "mới; địa bàn đang có không bị sửa. Ô Số hộ, Nhân khẩu để trống là chưa nhập, không phải 0.",
  templateFileName: "mau-nhap-thon-to-dan-pho.xlsx",
  confirmButton: "Nhập các địa bàn này",
  errorsHeading:
    "Tệp có lỗi — chưa thôn / tổ dân phố nào được tạo. Hãy sửa các dòng dưới đây rồi kiểm tra lại:",
  rowsLabel: "Các thôn / tổ dân phố sẽ tạo",
  previewLead: (n) => `Tệp hợp lệ. Sẽ tạo ${n} thôn / tổ dân phố:`,
  importedSentence: (n) =>
    n === null
      ? "Tệp đã được nhập ở lần gửi trước. Danh sách đã được tải lại."
      : `Đã nhập ${n} thôn / tổ dân phố. Danh sách đã được tải lại.`,
  columns: [
    { header: "Dòng", cell: (u) => String(u.row) },
    { header: "Tên", cell: (u) => u.name },
    { header: "Mã", cell: (u) => u.code, mono: true },
    { header: "Loại", cell: (u) => nhanLoaiDonVi(loaiDonVi(u.type_code, u.type_label)) },
    { header: "Trưởng thôn / Tổ trưởng", cell: (u) => (u.head_staff_code === "" ? "Chưa có" : u.head_staff_name) },
    { header: "Số hộ", cell: (u) => nhanSoDem(u.household_count) },
    { header: "Nhân khẩu", cell: (u) => nhanSoDem(u.population_count) },
    { header: "Thứ tự", cell: (u) => String(u.order) },
  ],
  rowKey: (u) => u.row,
  routes: RESIDENTIAL_UNIT_IMPORT_ROUTES,
  rowsField: RESIDENTIAL_UNIT_ROWS_FIELD,
};

/** Danh mục → Loại tài nguyên bản đồ (§5) — `admin.lookup`, owner `service-comms`. */
export const MAP_ASSET_TYPE_IMPORT_TARGET: ImportTarget<MapAssetTypeImportRow> = {
  id: "loai-tai-nguyen",
  title: "Nhập loại tài nguyên bản đồ từ Excel",
  explanation:
    "Tải tệp mẫu, điền mỗi loại tài nguyên bản đồ một dòng rồi chọn tệp để kiểm tra. Hệ thống kiểm " +
    "tra toàn bộ tệp trước, chưa ghi gì; chỉ khi tệp không có lỗi mới nhập được, và nhập thì nhập cả " +
    "tệp hoặc không nhập gì. Nhập chỉ thêm loại mới; loại đang có không bị sửa.",
  templateFileName: "mau-nhap-loai-tai-nguyen-ban-do.xlsx",
  confirmButton: "Nhập các loại này",
  errorsHeading:
    "Tệp có lỗi — chưa loại tài nguyên nào được tạo. Hãy sửa các dòng dưới đây rồi kiểm tra lại:",
  rowsLabel: "Các loại tài nguyên bản đồ sẽ tạo",
  previewLead: (n) => `Tệp hợp lệ. Sẽ tạo ${n} loại tài nguyên bản đồ:`,
  importedSentence: (n) =>
    n === null
      ? "Tệp đã được nhập ở lần gửi trước. Danh mục đã được tải lại."
      : `Đã nhập ${n} loại tài nguyên bản đồ. Danh mục đã được tải lại.`,
  columns: [
    { header: "Dòng", cell: (t) => String(t.row) },
    { header: "Tên hiển thị", cell: (t) => t.label },
    { header: "Mã", cell: (t) => t.code, mono: true },
    { header: "Thứ tự", cell: (t) => String(t.order) },
  ],
  rowKey: (t) => t.row,
  routes: MAP_ASSET_TYPE_IMPORT_ROUTES,
  rowsField: MAP_ASSET_TYPE_ROWS_FIELD,
};

/** "" is a blank cell of the spreadsheet — said in words, never drawn as an empty cell. */
function orBlank(v: string, blank: string): string {
  return v === "" ? blank : v;
}

/**
 * Người dùng (§3) — `admin.user`, drawn by `danh-ba-can-bo.tsx` (the tab is already gated on that key).
 * A file with any non-empty Vai trò cell ALSO needs `admin.role`: the server answers 403
 * `role_permission_required` with its own sentence, shown as it came. The explanation says it BEFORE the
 * file is filled in, so nobody fills 200 rows of roles to be refused.
 *
 * `mobile` is drawn exactly as the server sent it — MASKED (rule 3, #16).
 *
 * The 201 is the one result that is more than a sentence: `resultView` draws the temporary passwords,
 * once (`staff-import-result.tsx`).
 */
export const STAFF_IMPORT_TARGET: ImportTarget<StaffImportPlannedRow, StaffImportCreatedRow> = {
  id: "can-bo",
  title: "Nhập danh sách cán bộ từ Excel",
  explanation:
    "Tải tệp mẫu, điền mỗi cán bộ một dòng rồi chọn tệp để kiểm tra. Tệp mẫu có sẵn danh sách chọn Bộ " +
    "phận và Vai trò của xã; mã cán bộ do hệ thống tự sinh. Hệ thống kiểm tra toàn bộ tệp trước, chưa " +
    "ghi gì; chỉ khi tệp không có lỗi mới nhập được, và nhập thì nhập cả tệp hoặc không nhập gì. Dòng có " +
    "thư điện tử công vụ được cấp tài khoản đăng nhập kèm mật khẩu tạm, hiện ĐÚNG MỘT LẦN ngay sau khi " +
    "nhập — hãy chuẩn bị sẵn trước khi bấm nhập. Tệp có cột Vai trò không trống thì tài khoản của bạn " +
    "cần thêm quyền Phân quyền.",
  templateFileName: "mau-nhap-can-bo.xlsx",
  confirmButton: "Nhập các cán bộ này",
  errorsHeading: "Tệp có lỗi — chưa cán bộ nào được tạo. Hãy sửa các dòng dưới đây rồi kiểm tra lại:",
  rowsLabel: "Các cán bộ sẽ tạo",
  previewLead: (n) => `Tệp hợp lệ. Sẽ tạo ${n} cán bộ:`,
  importedSentence: (n) =>
    n === null
      ? "Tệp đã được nhập ở lần gửi trước. Danh sách đã được tải lại."
      : `Đã nhập ${n} cán bộ. Danh sách đã được tải lại.`,
  columns: [
    { header: "Dòng", cell: (p) => String(p.row) },
    { header: "Họ và tên", cell: (p) => p.full_name },
    { header: "Thư điện tử công vụ", cell: (p) => orBlank(p.email, "Chưa có") },
    { header: "Chức vụ", cell: (p) => orBlank(p.position, "Chưa có") },
    { header: "Bộ phận", cell: (p) => (p.org_unit_code === "" ? "Chưa phân bộ phận" : p.org_unit_name) },
    { header: "Vai trò", cell: (p) => (p.role_code === "" ? "Chưa gán vai trò" : p.role_name) },
    { header: "Điện thoại cơ quan", cell: (p) => orBlank(p.office_phone, "Chưa có") },
    { header: "Di động cá nhân", cell: (p) => orBlank(p.mobile, "Chưa có") },
    {
      header: "Tài khoản",
      cell: (p) => (p.issues_account ? "Sẽ cấp tài khoản" : `Không cấp — ${NO_EMAIL_REASON}`),
    },
  ],
  rowKey: (p) => p.row,
  routes: STAFF_IMPORT_ROUTES,
  rowsField: STAFF_ROWS_FIELD,
  resultView: (created, onClose) => <StaffImportResult created={created} onClose={onClose} />,
};

/** The four fields every lookup-catalogue import row has (`row`, `code`, `label`, `order`). */
type LookupImportRow = { readonly row: number; readonly code: string; readonly label: string; readonly order: number };

/**
 * One Danh mục group whose rows are the plain four fields. The map-asset-type target above predates this
 * and keeps its own words; the six below differ only in the noun, the file name, the routes and — for
 * Mức ưu tiên alone — one extra sentence of explanation.
 */
function lookupImportTarget<R extends LookupImportRow>(spec: {
  id: string;
  noun: string;
  templateFileName: string;
  routes: ImportRoutes;
  rowsField: string;
  /** One more sentence for a group whose template differs from the four plain columns. */
  explanationNote?: string;
}): ImportTarget<R> {
  const { noun } = spec;
  return {
    id: spec.id,
    title: `Nhập ${noun} từ Excel`,
    explanation:
      `Tải tệp mẫu, điền mỗi ${noun} một dòng rồi chọn tệp để kiểm tra. Hệ thống kiểm tra toàn bộ tệp ` +
      "trước, chưa ghi gì; chỉ khi tệp không có lỗi mới nhập được, và nhập thì nhập cả tệp hoặc không " +
      "nhập gì. Nhập chỉ thêm mục mới; mục đang có không bị sửa." +
      (spec.explanationNote === undefined ? "" : ` ${spec.explanationNote}`),
    templateFileName: spec.templateFileName,
    confirmButton: "Nhập các mục này",
    errorsHeading: `Tệp có lỗi — chưa ${noun} nào được tạo. Hãy sửa các dòng dưới đây rồi kiểm tra lại:`,
    rowsLabel: `Các ${noun} sẽ tạo`,
    previewLead: (n) => `Tệp hợp lệ. Sẽ tạo ${n} ${noun}:`,
    importedSentence: (n) =>
      n === null
        ? "Tệp đã được nhập ở lần gửi trước. Danh mục đã được tải lại."
        : `Đã nhập ${n} ${noun}. Danh mục đã được tải lại.`,
    columns: [
      { header: "Dòng", cell: (t) => String(t.row) },
      { header: "Tên hiển thị", cell: (t) => t.label },
      { header: "Mã", cell: (t) => t.code, mono: true },
      { header: "Thứ tự", cell: (t) => String(t.order) },
    ],
    rowKey: (t) => t.row,
    routes: spec.routes,
    rowsField: spec.rowsField,
  };
}

/** Danh mục → Loại đơn vị dân cư (§5) — `admin.lookup`, owner `service-identity`. */
export const RESIDENTIAL_UNIT_TYPE_IMPORT_TARGET = lookupImportTarget<IdentityCatalogueImportRow>({
  id: "loai-don-vi-dan-cu",
  noun: "loại đơn vị dân cư",
  templateFileName: "mau-nhap-loai-don-vi-dan-cu.xlsx",
  routes: RESIDENTIAL_UNIT_TYPE_IMPORT_ROUTES,
  rowsField: IDENTITY_CATALOGUE_ROWS_FIELD,
});

/** Danh mục → Khối nhiệm vụ (§5) — `admin.lookup`, owner `service-identity`. */
export const TASK_BLOC_IMPORT_TARGET = lookupImportTarget<IdentityCatalogueImportRow>({
  id: "khoi-nhiem-vu",
  noun: "khối nhiệm vụ",
  templateFileName: "mau-nhap-khoi-nhiem-vu.xlsx",
  routes: TASK_BLOC_IMPORT_ROUTES,
  rowsField: IDENTITY_CATALOGUE_ROWS_FIELD,
});

/** Danh mục → Loại văn bản (§5) — `admin.lookup`, owner `service-documents`. */
export const DOCUMENT_TYPE_IMPORT_TARGET = lookupImportTarget<DocumentTypeImportRow>({
  id: "loai-van-ban",
  noun: "loại văn bản",
  templateFileName: "mau-nhap-loai-van-ban.xlsx",
  routes: DOCUMENT_TYPE_IMPORT_ROUTES,
  rowsField: DOCUMENT_TYPE_ROWS_FIELD,
});

/** Danh mục → Loại nhiệm vụ (§5) — `admin.lookup`, owner `service-petitions`. */
export const TASK_TYPE_IMPORT_TARGET = lookupImportTarget<PetitionsCatalogueImportRow>({
  id: "loai-nhiem-vu",
  noun: "loại nhiệm vụ",
  templateFileName: "mau-nhap-loai-nhiem-vu.xlsx",
  routes: TASK_TYPE_IMPORT_ROUTES,
  rowsField: PETITIONS_CATALOGUE_ROWS_FIELD,
});

/**
 * Danh mục → Mức ưu tiên nhiệm vụ (§5) — `admin.lookup`, owner `service-petitions`. Its order IS the
 * rank, so the template has NO Thứ tự column: the server appends imported levels after the commune's
 * last level, in file order. The preview's Thứ tự column is that server-computed rank; the explanation
 * says so before the file is filled in, so nobody expects to set the rank from the spreadsheet.
 */
export const TASK_PRIORITY_IMPORT_TARGET = lookupImportTarget<PetitionsCatalogueImportRow>({
  id: "muc-uu-tien-nhiem-vu",
  noun: "mức ưu tiên nhiệm vụ",
  templateFileName: "mau-nhap-muc-uu-tien-nhiem-vu.xlsx",
  routes: TASK_PRIORITY_IMPORT_ROUTES,
  rowsField: PETITIONS_CATALOGUE_ROWS_FIELD,
  explanationNote:
    "Tệp mẫu không có cột Thứ tự: các mức nhập vào được xếp sau mức cuối cùng đang có của xã, theo " +
    "đúng thứ tự các dòng trong tệp.",
});

/** Danh mục → Hạng mục kế hoạch vốn (§5) — `admin.lookup`, owner `service-finance`. */
export const CAPITAL_PLAN_CATEGORY_IMPORT_TARGET = lookupImportTarget<FinanceCatalogueImportRow>({
  id: "hang-muc-ke-hoach-von",
  noun: "hạng mục kế hoạch vốn",
  templateFileName: "mau-nhap-hang-muc-ke-hoach-von.xlsx",
  routes: CAPITAL_PLAN_CATEGORY_IMPORT_ROUTES,
  rowsField: FINANCE_CATALOGUE_ROWS_FIELD,
});

/** What the Danh mục tab needs to offer an import for one group. */
export type CatalogueImport = {
  /** The key the three routes declare — gated in the UI, checked by the server. */
  readonly permission: string;
  readonly panel: (p: { onImported: () => void; onClose: () => void }) => ReactNode;
};

/**
 * The catalogue groups that can import from Excel — one entry per group whose owning service has
 * published its three import routes (ADR 0059 §3); since 5a576de that is all seven. A group absent
 * here draws no import button.
 */
export const CATALOGUE_IMPORTS: Partial<Record<KhoaDanhMucGhi, CatalogueImport>> = {
  loaiTaiNguyenBanDo: {
    permission: QUYEN_QUAN_LY_DANH_MUC,
    panel: (p) => <ExcelImportPanel target={MAP_ASSET_TYPE_IMPORT_TARGET} {...p} />,
  },
  loaiVanBan: {
    permission: QUYEN_QUAN_LY_DANH_MUC,
    panel: (p) => <ExcelImportPanel target={DOCUMENT_TYPE_IMPORT_TARGET} {...p} />,
  },
  loaiDonViDanCu: {
    permission: QUYEN_QUAN_LY_DANH_MUC,
    panel: (p) => <ExcelImportPanel target={RESIDENTIAL_UNIT_TYPE_IMPORT_TARGET} {...p} />,
  },
  khoiNhiemVu: {
    permission: QUYEN_QUAN_LY_DANH_MUC,
    panel: (p) => <ExcelImportPanel target={TASK_BLOC_IMPORT_TARGET} {...p} />,
  },
  loaiNhiemVu: {
    permission: QUYEN_QUAN_LY_DANH_MUC,
    panel: (p) => <ExcelImportPanel target={TASK_TYPE_IMPORT_TARGET} {...p} />,
  },
  mucUuTienNhiemVu: {
    permission: QUYEN_QUAN_LY_DANH_MUC,
    panel: (p) => <ExcelImportPanel target={TASK_PRIORITY_IMPORT_TARGET} {...p} />,
  },
  hangMucKeHoachVon: {
    permission: QUYEN_QUAN_LY_DANH_MUC,
    panel: (p) => <ExcelImportPanel target={CAPITAL_PLAN_CATEGORY_IMPORT_TARGET} {...p} />,
  },
};

/**
 * The import a group offers THIS account, or `null`: the group has an import route AND the session
 * holds that route's key. FAIL CLOSED — a session not read yet, or read with an error, offers nothing.
 * Convenience, not a control: the server checks the key on all three routes.
 */
export function catalogueImportFor(
  khoa: KhoaDanhMucGhi | null,
  phien: KetQua<identity_phienHienTaiRa> | null,
): CatalogueImport | null {
  if (khoa === null || phien === null) return null;
  const entry = CATALOGUE_IMPORTS[khoa];
  if (entry === undefined) return null;
  return quyetDinhTheoKhoa(phien, entry.permission).hien ? entry : null;
}
