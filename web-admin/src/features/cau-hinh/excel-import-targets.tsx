"use client";

import type { ReactNode } from "react";

import type { KhoaDanhMucGhi } from "@/lib/api/danh-muc";
import { MAP_ASSET_TYPE_IMPORT_ROUTES, MAP_ASSET_TYPE_ROWS_FIELD } from "@/lib/api/map-asset-type-import";
import type { MapAssetTypeImportRow } from "@/lib/api/map-asset-type-import";
import { ORG_UNIT_IMPORT_ROUTES } from "@/lib/api/org-unit-import";
import { RESIDENTIAL_UNIT_IMPORT_ROUTES, RESIDENTIAL_UNIT_ROWS_FIELD } from "@/lib/api/residential-unit-import";
import type { ResidentialUnitImportRow } from "@/lib/api/residential-unit-import";
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
 * TO WIRE ANOTHER IMPORT (staff, another catalogue group): write its routes file
 * beside `lib/api/map-asset-type-import.ts`, add ONE target here, and — for a catalogue group — one
 * entry in `CATALOGUE_IMPORTS`. The panel, the flow and the key-per-attempt rule need no change. Then
 * update the `Excel` item of `PHAN_CHUA_DUNG` (`nhan-cau-hinh.ts`) in the same change: it names every
 * group that can import today, and `khoi-chua-dung.test.tsx` checks it against `CATALOGUE_IMPORTS`.
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

/** What the Danh mục tab needs to offer an import for one group. */
export type CatalogueImport = {
  /** The key the three routes declare — gated in the UI, checked by the server. */
  readonly permission: string;
  readonly panel: (p: { onImported: () => void; onClose: () => void }) => ReactNode;
};

/**
 * The catalogue groups that can import from Excel TODAY — one entry per group whose owning service
 * has published its three import routes (ADR 0059 §3). A group absent here draws no import button.
 */
export const CATALOGUE_IMPORTS: Partial<Record<KhoaDanhMucGhi, CatalogueImport>> = {
  loaiTaiNguyenBanDo: {
    permission: QUYEN_QUAN_LY_DANH_MUC,
    panel: (p) => <ExcelImportPanel target={MAP_ASSET_TYPE_IMPORT_TARGET} {...p} />,
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
