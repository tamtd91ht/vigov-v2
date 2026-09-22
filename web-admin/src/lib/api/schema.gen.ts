// TỆP SINH RA. ĐỪNG SỬA TAY — sửa tay mất sạch ở lần sinh sau (luật 9, bất biến 8).
//
// Nguồn: kb/20-contracts/openapi.json (do tools/apidoc sinh từ khai báo route trong
// */internal/). Sinh lại: npm run gen:api — trong web-admin.
//
// Hợp đồng: ViGov — REST API cho web quản trị v1

export type comms_danhSachLoaiTaiNguyenRa = {
  "items": Array<comms_loaiTaiNguyenRa>;
};

export type comms_loaiTaiNguyenRa = {
  /** ULID — what a map asset record references */
  "id": string;
  /** `ma`: slug "doanh-nghiep", the value asset records store */
  "code": string;
  /** `nhan`: the wording a commune may change without touching `ma` */
  "label": string;
  "is_default": boolean;
  "active": boolean;
  "order": number;
  "source": string;
  "tier": number;
};

export type comms_suaLoaiTaiNguyenVao = {
  "label"?: string | null;
  "order"?: number | null;
  "active"?: boolean | null;
  "is_default"?: boolean | null;
  "code"?: string | null;
  "source"?: string | null;
  "tier"?: number | null;
};

export type comms_themLoaiTaiNguyenVao = {
  "code": string;
  "label": string;
  "order"?: number;
  "is_default"?: boolean;
  "source"?: string | null;
  "tier"?: number | null;
};

export type comms_xoaLoaiTaiNguyenVao = {
  "reason": string;
};

export type documents_danhSachLoaiVanBanRa = {
  "items": Array<documents_loaiVanBanRa>;
};

export type documents_loaiVanBanRa = {
  /** ULID — what a document record would reference */
  "id": string;
  /** "cong-van" — the value a document record stores, never renumbered */
  "code": string;
  /** "Công văn" — the wording the commune may change */
  "label": string;
  "active": boolean;
  "is_default": boolean;
  "order": number;
  "source": string;
  "tier": number;
};

export type documents_suaLoaiVanBanVao = {
  "label"?: string | null;
  "order"?: number | null;
  "active"?: boolean | null;
  "is_default"?: boolean | null;
  "code"?: string | null;
  "source"?: string | null;
  "tier"?: number | null;
};

export type documents_themLoaiVanBanVao = {
  "code": string;
  "label": string;
  "order"?: number;
  "is_default"?: boolean;
  "source"?: string | null;
  "tier"?: number | null;
};

export type documents_xoaLoaiVanBanVao = {
  "reason": string;
};

export type finance_danhSachDuAnRa = {
  "items": Array<finance_duAnRa>;
  "year": number;
  "delay_threshold": number;
};

export type finance_danhSachHangMucRa = {
  "items": Array<finance_hangMucRa>;
};

export type finance_duAnRa = {
  "id": string;
  /** `ma` — issued once, never reissued */
  "code": string;
  /** `nam` — each budget year is its own set of projects (§13 rule 8) */
  "year": number;
  /** `hang_muc_id` */
  "category_id": string;
  "name": string;
  "description"?: string;
  /** `ke_hoach_von_nam`, đồng */
  "planned_amount": number;
  /** §9's rule for a blank field already applied */
  "approved_amount": number;
  /** DERIVED from the vouchers, stored nowhere */
  "disbursed_amount": number;
  "remaining_amount": number;
  "disbursed_ratio": number | null;
  "delay_score": number | null;
  "is_delayed": boolean;
  "org_unit_id"?: string;
  "assignee_id"?: string;
  "start_date"?: string;
  "completion_date"?: string;
  "disbursement_deadline": string;
};

export type finance_hangMucRa = {
  /** ULID — what a capital plan line will reference */
  "id": string;
  /** `ma`: Vietnamese without diacritics, the value a plan line stores */
  "code": string;
  /** `nhan`: what the category is called, and what a commune may re-word */
  "label": string;
  "is_default": boolean;
  "active": boolean;
  "order": number;
  "source": string;
  "tier": number;
};

export type finance_suaHangMucVao = {
  "label"?: string | null;
  "order"?: number | null;
  "active"?: boolean | null;
  "is_default"?: boolean | null;
  "code"?: string | null;
  "source"?: string | null;
  "tier"?: number | null;
};

export type finance_themHangMucVao = {
  "code": string;
  "label": string;
  "order"?: number;
  "is_default"?: boolean;
  "source"?: string | null;
  "tier"?: number | null;
};

export type finance_xoaHangMucVao = {
  "reason": string;
};

export type httpx_Error = {
  "code": string;
  /** safe for a citizen to read: never personal data, never internals */
  "message": string;
  "trace_id": string;
};

export type identity_boPhanRa = {
  /** ULID — what other records reference */
  "id": string;
  /** slug */
  "code": string;
  "name": string;
  "parent_id": string;
};

export type identity_caLamBuRa = {
  /** ULID — what a later edit would reference */
  "id": string;
  "date": string;
  "start": string;
  "end": string;
  "name": string;
};

export type identity_caLamViecRa = {
  /** ULID — what a later edit would reference */
  "id": string;
  "weekday": number;
  "start": string;
  "end": string;
  "note": string;
};

export type identity_canBoGon = {
  /** cb.Ma — the business code, the one the audit trail shows */
  "code": string;
  /** shown in the header of the admin screens */
  "full_name": string;
  "position": string;
};

export type identity_canBoTomTat = {
  /** ULID — what GET /api/v1/staff/{id} takes */
  "id": string;
  /** cb.Ma — the code the audit trail quotes */
  "code": string;
  "full_name": string;
  "email": string;
  "position": string;
  "department_id": string;
  "role_id": string;
  "phone": string;
  "mobile": string;
  "has_account": boolean;
  "active": boolean;
  "last_login_at": string | null;
  "created_at": string;
};

export type identity_capQuyenRa = {
  /** vaiTroCotRa.ID — a role OF THIS COMMUNE */
  "role_id": string;
  "permission": string;
};

export type identity_danhSachBoPhanRa = {
  "items": Array<identity_boPhanRa>;
};

export type identity_danhSachCaLamBuRa = {
  "items": Array<identity_caLamBuRa>;
  "problems": Array<identity_vanDeLamBuRa>;
};

export type identity_danhSachCaLamViecRa = {
  "items": Array<identity_caLamViecRa>;
  "problems": Array<identity_vanDeLichRa>;
};

export type identity_danhSachKhoiNhiemVuRa = {
  "items": Array<identity_khoiNhiemVuRa>;
};

export type identity_danhSachLoaiDonViDanCuRa = {
  "items": Array<identity_loaiDonViDanCuRa>;
};

export type identity_danhSachNgayNghiLeRa = {
  "items": Array<identity_ngayNghiLeRa>;
};

export type identity_danhSachThonToDanPhoRa = {
  "items": Array<identity_thonToDanPhoRa>;
};

export type identity_danhSachVaiTroRa = {
  "items": Array<identity_vaiTroMucRa>;
};

export type identity_datVaiTroVao = {
  "role_id": string;
};

export type identity_khoiNhiemVuRa = {
  /** ULID */
  "id": string;
  /** slug: "khoi-uy-ban" — the value a task record in `petitions` holds */
  "code": string;
  /** "Khối Uỷ ban" */
  "label": string;
  /** the row a form pre-selects; at most one per commune */
  "is_default": boolean;
  "active": boolean;
};

export type identity_loaiDonViDanCuRa = {
  /** ULID — what a later reference would point at */
  "id": string;
  /** slug: "thon" — the value thon_to_dan_pho.loai holds */
  "code": string;
  /** "Thôn" — what a person reads */
  "label": string;
  "is_default": boolean;
  "active": boolean;
};

export type identity_maTranQuyenRa = {
  "groups": Array<identity_nhomQuyenRa>;
  "roles": Array<identity_vaiTroCotRa>;
  "grants": Array<identity_capQuyenRa>;
};

export type identity_ngayNghiLeRa = {
  /** ULID — what a later edit would reference */
  "id": string;
  "date": string;
  "name": string;
};

export type identity_nhomQuyenRa = {
  "name": string;
  "permissions": Array<identity_quyenMucRa>;
};

export type identity_phanHoiDangNhap = {
  "sid": string;
  "expires_at": string;
  "staff": identity_canBoGon;
};

export type identity_phienHienTaiRa = {
  "sid": string;
  "expires_at": string;
  "staff": identity_canBoGon;
  "role": identity_vaiTroGon | null;
  "permissions": Array<string>;
};

export type identity_quyenMucRa = {
  /** "task.extend" */
  "code": string;
  /** "Duyệt gia hạn" */
  "label": string;
};

export type identity_suaCanBoVao = {
  "full_name": string | null;
  "position": string | null;
  "email": string | null;
  "org_unit_id": string | null;
  "office_phone": string | null;
  "mobile": string | null;
};

export type identity_thanDangNhap = {
  "email": string;
  "password": string;
};

export type identity_themCanBoVao = {
  "full_name": string;
  "position": string;
  "email": string;
  "org_unit_id": string;
  "office_phone": string;
  "mobile": string;
};

export type identity_thonToDanPhoRa = {
  /** ULID — what other records reference */
  "id": string;
  /** slug: "thon-binh-an" — immutable once issued */
  "code": string;
  /** "Thôn Bình An" — a unit has a NAME, unlike a catalogue row's label */
  "name": string;
  "type_code": string;
  "type_label": string;
  "household_count": number | null;
  "population_count": number | null;
  "active": boolean;
};

export type identity_thongTinXa = {
  "name": string;
  "host": string;
  "province": string;
};

export type identity_vaiTroCotRa = {
  /** ULID — what `grants` and `staff.role_id` reference */
  "id": string;
  "code": string;
  "name": string;
  "is_leader": boolean;
  /** mọi cán bộ giữ vai trò này — đúng con số tab Người dùng hiện */
  "staff_count": number;
  /** trong số đó, ai có tài khoản và đang mở: bật/tắt một ô chỉ đổi quyền của những người này */
  "active_account_count": number;
};

export type identity_vaiTroGon = {
  /** vai_tro.ma — the stable slug a client keys on */
  "code": string;
  /** vai_tro.ten — what a person reads */
  "name": string;
  "is_leader": boolean;
};

export type identity_vaiTroMucRa = {
  /** ULID — what nguoi_dung.vai_tro_id references */
  "id": string;
  /** slug */
  "code": string;
  "name": string;
  "is_leader": boolean;
};

export type identity_vanDeLamBuRa = {
  "kind": string;
  "date": string;
  "session_ids": Array<string>;
  "message": string;
};

export type identity_vanDeLichRa = {
  "kind": string;
  "weekday": number | null;
  "session_ids": Array<string>;
  "message": string;
};

export type page_Result_identity_canBoTomTat = {
  "items": Array<identity_canBoTomTat>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

export type petitions_danhSachLoaiNhiemVuRa = {
  "items": Array<petitions_loaiNhiemVuRa>;
};

export type petitions_danhSachMucUuTienRa = {
  "items": Array<petitions_mucUuTienRa>;
};

export type petitions_guiPhanAnhVao = {
  "content": string;
  "address": string;
  "reporter_name": string;
  "reporter_phone": string;
  "anonymous": boolean;
  "citizen_id": string | null;
  "cong_dan_id": string | null;
  "field": string | null;
  "linh_vuc": string | null;
  "channel": string | null;
  "code": string | null;
  "status": string | null;
  "clock_from": string | null;
  "acknowledge_due": string | null;
  "resolve_due": string | null;
};

export type petitions_loaiNhiemVuRa = {
  /** ULID — what a task record references */
  "id": string;
  /** slug: "theo-van-ban" */
  "code": string;
  "label": string;
  "is_default": boolean;
  "active": boolean;
  "order": number;
  "source": string;
  "tier": number;
};

export type petitions_mucUuTienRa = {
  /** ULID — what a task record references */
  "id": string;
  /** slug: "khan" */
  "code": string;
  "label": string;
  "is_default": boolean;
  "active": boolean;
  "order": number;
  "source": string;
  "tier": number;
};

export type petitions_phieuCuaToiRa = {
  "code": string;
  /** `zalo-mini-app` | `zalo-oa` | `web-xa` | `can-bo-nhap-ho` */
  "channel": string;
  /** one of the nine (ADR 0027) */
  "status": string;
  "field": string;
  "field_label": string;
  "content": string;
  "address": string;
  "reporter_name": string;
  "reporter_phone": string;
  "anonymous": boolean;
  "clock_from": string;
  "acknowledge_due": string | null;
  "resolve_due": string | null;
};

export type petitions_phieuPhanAnhRa = {
  "code": string;
  /** `zalo-mini-app` | `zalo-oa` | `web-xa` | `can-bo-nhap-ho` */
  "channel": string;
  /** one of the nine (ADR 0027) */
  "status": string;
  "field": string;
  "field_label": string;
  "content": string;
  "address": string;
  "reporter_name": string;
  "reporter_phone": string;
  "anonymous": boolean;
  "clock_from": string;
  "booked_at": string;
  "acknowledge_due": string | null;
  "resolve_due": string | null;
  "public": boolean;
};

export type petitions_suaLoaiNhiemVuVao = {
  "label"?: string | null;
  "order"?: number | null;
  "active"?: boolean | null;
  "is_default"?: boolean | null;
  "code"?: string | null;
  "source"?: string | null;
  "tier"?: number | null;
};

export type petitions_suaMucUuTienVao = {
  "label"?: string | null;
  "order"?: number | null;
  "active"?: boolean | null;
  "is_default"?: boolean | null;
  "code"?: string | null;
  "source"?: string | null;
  "tier"?: number | null;
};

export type petitions_themLoaiNhiemVuVao = {
  "code": string;
  "label": string;
  "order"?: number;
  "is_default"?: boolean;
  "source"?: string | null;
  "tier"?: number | null;
};

export type petitions_themMucUuTienVao = {
  "code": string;
  "label": string;
  "order"?: number;
  "is_default"?: boolean;
  "source"?: string | null;
  "tier"?: number | null;
};

export type petitions_xoaLoaiNhiemVuVao = {
  "reason": string;
};

export type petitions_xoaMucUuTienVao = {
  "reason": string;
};

/** GET /api/v1/capital-plan-categories — Danh mục hạng mục kế hoạch vốn của xã — dùng cho ô phân loại dòng kế hoạch và bộ lọc */
export type finance_get_capital_plan_categories = {
  duongDan: "/api/v1/capital-plan-categories";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: finance_danhSachHangMucRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/capital-plan-categories — Thêm một hạng mục kế hoạch vốn của riêng xã vào danh mục */
export type finance_post_capital_plan_categories = {
  duongDan: "/api/v1/capital-plan-categories";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: finance_themHangMucVao;
  phanHoi: {
    201: finance_hangMucRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/capital-plan-categories/{id} — Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một hạng mục kế hoạch vốn */
export type finance_patch_capital_plan_categories_by_id = {
  duongDan: "/api/v1/capital-plan-categories/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_suaHangMucVao;
  phanHoi: {
    200: finance_hangMucRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/capital-plan-categories/{id} — Xoá mềm một mục danh mục do xã tự thêm, kèm lý do bắt buộc */
export type finance_delete_capital_plan_categories_by_id = {
  duongDan: "/api/v1/capital-plan-categories/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_xoaHangMucVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/citizen-reports/{maTraCuu} — Một phiếu phản ánh, tra theo mã tra cứu đã trả cho người dân */
export type petitions_get_citizen_reports_by_maTraCuu = {
  duongDan: "/api/v1/citizen-reports/{maTraCuu}";
  phuongThuc: "GET";
  thamSo: {
    "maTraCuu": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: petitions_phieuPhanAnhRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/communes/current — Thông tin xã ứng với tên miền đang gọi, cho màn hình đăng nhập */
export type identity_get_communes_current = {
  duongDan: "/api/v1/communes/current";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_thongTinXa;
  };
};

/** GET /api/v1/document-types — Danh mục loại văn bản của xã — dùng cho ô chọn loại khi vào sổ, bộ lọc và nhãn trên mọi văn bản */
export type documents_get_document_types = {
  duongDan: "/api/v1/document-types";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: documents_danhSachLoaiVanBanRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/document-types — Thêm một loại văn bản của riêng xã vào danh mục */
export type documents_post_document_types = {
  duongDan: "/api/v1/document-types";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: documents_themLoaiVanBanVao;
  phanHoi: {
    201: documents_loaiVanBanRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/document-types/{id} — Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một loại văn bản */
export type documents_patch_document_types_by_id = {
  duongDan: "/api/v1/document-types/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: documents_suaLoaiVanBanVao;
  phanHoi: {
    200: documents_loaiVanBanRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/document-types/{id} — Xoá mềm một mục danh mục do xã tự thêm, kèm lý do bắt buộc */
export type documents_delete_document_types_by_id = {
  duongDan: "/api/v1/document-types/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: documents_xoaLoaiVanBanVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/investment-projects — Danh sách dự án đầu tư của xã theo năm ngân sách, kèm số đã giải ngân suy ra từ chứng từ */
export type finance_get_investment_projects = {
  duongDan: "/api/v1/investment-projects";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "category"?: string;
    "year": string;
  };
  than: never;
  phanHoi: {
    200: finance_danhSachDuAnRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/investment-projects/{id} — Chi tiết một dự án đầu tư: kế hoạch vốn, đã giải ngân, tỷ lệ và điểm chậm */
export type finance_get_investment_projects_by_id = {
  duongDan: "/api/v1/investment-projects/{id}";
  phuongThuc: "GET";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: finance_duAnRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/map-asset-types — Danh mục loại tài nguyên bản đồ của xã — dùng cho ô chọn nhóm trên bản đồ kinh tế số, bộ lọc và nhãn của tài nguyên đã lưu */
export type comms_get_map_asset_types = {
  duongDan: "/api/v1/map-asset-types";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: comms_danhSachLoaiTaiNguyenRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/map-asset-types — Thêm một loại tài nguyên bản đồ của riêng xã vào danh mục */
export type comms_post_map_asset_types = {
  duongDan: "/api/v1/map-asset-types";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: comms_themLoaiTaiNguyenVao;
  phanHoi: {
    201: comms_loaiTaiNguyenRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/map-asset-types/{id} — Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một loại tài nguyên bản đồ */
export type comms_patch_map_asset_types_by_id = {
  duongDan: "/api/v1/map-asset-types/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: comms_suaLoaiTaiNguyenVao;
  phanHoi: {
    200: comms_loaiTaiNguyenRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/map-asset-types/{id} — Xoá mềm một mục danh mục do xã tự thêm, kèm lý do bắt buộc */
export type comms_delete_map_asset_types_by_id = {
  duongDan: "/api/v1/map-asset-types/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: comms_xoaLoaiTaiNguyenVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/my-citizen-reports — Công dân gửi một phiếu phản ánh — trả MÃ TRA CỨU ngay khi tiếp nhận */
export type petitions_post_my_citizen_reports = {
  duongDan: "/api/v1/my-citizen-reports";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: petitions_guiPhanAnhVao;
  phanHoi: {
    201: petitions_phieuCuaToiRa;
    400: httpx_Error;
    401: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
    503: httpx_Error;
  };
};

/** GET /api/v1/my-citizen-reports/{maTraCuu} — Phiếu phản ánh CỦA CHÍNH NGƯỜI GỬI, tra theo mã tra cứu — dùng trong Zalo Mini App */
export type petitions_get_my_citizen_reports_by_maTraCuu = {
  duongDan: "/api/v1/my-citizen-reports/{maTraCuu}";
  phuongThuc: "GET";
  thamSo: {
    "maTraCuu": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: petitions_phieuCuaToiRa;
    401: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/org-units — Danh mục bộ phận của xã — cây tổ chức, dùng cho ô phân công, luồng văn bản và bộ lọc */
export type identity_get_org_units = {
  duongDan: "/api/v1/org-units";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_danhSachBoPhanRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/public-holidays — Ngày nghỉ lễ của xã trong một năm — ngày xã KHÔNG làm việc, gồm cả lễ quốc gia lẫn lễ địa phương */
export type identity_get_public_holidays = {
  duongDan: "/api/v1/public-holidays";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "year": string;
  };
  than: never;
  phanHoi: {
    200: identity_danhSachNgayNghiLeRa;
    400: httpx_Error;
    401: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/residential-unit-types — Danh mục loại đơn vị dân cư của xã — thôn / tổ dân phố, dùng cho ô chọn Loại và bộ lọc */
export type identity_get_residential_unit_types = {
  duongDan: "/api/v1/residential-unit-types";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_danhSachLoaiDonViDanCuRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/residential-units — Danh sách thôn / tổ dân phố của xã — kèm nhãn loại đơn vị, dùng cho ô chọn địa bàn và bộ lọc */
export type identity_get_residential_units = {
  duongDan: "/api/v1/residential-units";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_danhSachThonToDanPhoRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/role-permissions — Ma trận phân quyền của xã — nhóm quyền, vai trò kèm số cán bộ, và các ô đã cấp */
export type identity_get_role_permissions = {
  duongDan: "/api/v1/role-permissions";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_maTranQuyenRa;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/roles — Danh mục vai trò của xã — dùng cho ô chọn vai trò, cột danh bạ và ma trận phân quyền */
export type identity_get_roles = {
  duongDan: "/api/v1/roles";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_danhSachVaiTroRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/sessions — Đăng nhập bằng email và mật khẩu, mở một phiên làm việc */
export type identity_post_sessions = {
  duongDan: "/api/v1/sessions";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_thanDangNhap;
  phanHoi: {
    201: identity_phanHoiDangNhap;
    400: httpx_Error;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/sessions/current — Phiên làm việc hiện tại của chính người gọi, kèm vai trò và danh sách quyền để ẩn/hiện menu */
export type identity_get_sessions_current = {
  duongDan: "/api/v1/sessions/current";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_phienHienTaiRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/sessions/{sid} — Kết thúc phiên làm việc của chính mình */
export type identity_delete_sessions_by_sid = {
  duongDan: "/api/v1/sessions/{sid}";
  phuongThuc: "DELETE";
  thamSo: {
    "sid": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    204: void;
    401: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/staff — Danh sách cán bộ của xã — gồm cả người có tài khoản đăng nhập và người chỉ có trong danh bạ */
export type identity_get_staff = {
  duongDan: "/api/v1/staff";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "limit"?: number;
    "cursor"?: string;
    "sort"?: "code" | "created_at";
    "order"?: "asc" | "desc";
  };
  than: never;
  phanHoi: {
    200: page_Result_identity_canBoTomTat;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/staff — Thêm một cán bộ vào danh bạ của xã — mã cán bộ do hệ thống sinh, không có ô nhập */
export type identity_post_staff = {
  duongDan: "/api/v1/staff";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_themCanBoVao;
  phanHoi: {
    201: identity_canBoTomTat;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/staff/{id} — Chi tiết một cán bộ trong xã */
export type identity_get_staff_by_id = {
  duongDan: "/api/v1/staff/{id}";
  phuongThuc: "GET";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_canBoTomTat;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/staff/{id} — Sửa hồ sơ một cán bộ — họ tên, chức vụ, thư điện tử, bộ phận, hai số điện thoại */
export type identity_patch_staff_by_id = {
  duongDan: "/api/v1/staff/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_suaCanBoVao;
  phanHoi: {
    200: identity_canBoTomTat;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/staff/{id}/lockout — Khoá tài khoản một cán bộ đã nghỉ hưu hoặc chuyển công tác — người này vẫn còn trong danh bạ */
export type identity_post_staff_by_id_lockout = {
  duongDan: "/api/v1/staff/{id}/lockout";
  phuongThuc: "POST";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_canBoTomTat;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/staff/{id}/lockout — Mở khoá tài khoản một cán bộ */
export type identity_delete_staff_by_id_lockout = {
  duongDan: "/api/v1/staff/{id}/lockout";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_canBoTomTat;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** PUT /api/v1/staff/{id}/role — Đổi vai trò của một cán bộ — chuỗi rỗng nghĩa là gỡ vai trò */
export type identity_put_staff_by_id_role = {
  duongDan: "/api/v1/staff/{id}/role";
  phuongThuc: "PUT";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_datVaiTroVao;
  phanHoi: {
    200: identity_canBoTomTat;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/swap-working-days — Ngày làm bù của xã trong một năm — ngày xã CÓ làm việc dù lịch tuần nói không, kèm giờ làm của chính ngày đó */
export type identity_get_swap_working_days = {
  duongDan: "/api/v1/swap-working-days";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "year": string;
  };
  than: never;
  phanHoi: {
    200: identity_danhSachCaLamBuRa;
    400: httpx_Error;
    401: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/task-blocs — Danh mục khối nhiệm vụ của xã — Khối Uỷ ban / Khối Đảng / Khác, dùng cho ô chọn khối và bộ lọc nhiệm vụ */
export type identity_get_task_blocs = {
  duongDan: "/api/v1/task-blocs";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_danhSachKhoiNhiemVuRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/task-priorities — Danh mục mức ưu tiên nhiệm vụ của xã, theo đúng thứ tự thang — dùng cho ô chọn và bộ lọc */
export type petitions_get_task_priorities = {
  duongDan: "/api/v1/task-priorities";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: petitions_danhSachMucUuTienRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/task-priorities — Thêm một mức ưu tiên nhiệm vụ của riêng xã vào danh mục */
export type petitions_post_task_priorities = {
  duongDan: "/api/v1/task-priorities";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: petitions_themMucUuTienVao;
  phanHoi: {
    201: petitions_mucUuTienRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/task-priorities/{id} — Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một mức ưu tiên nhiệm vụ */
export type petitions_patch_task_priorities_by_id = {
  duongDan: "/api/v1/task-priorities/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: petitions_suaMucUuTienVao;
  phanHoi: {
    200: petitions_mucUuTienRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/task-priorities/{id} — Xoá mềm một mục danh mục do xã tự thêm, kèm lý do bắt buộc */
export type petitions_delete_task_priorities_by_id = {
  duongDan: "/api/v1/task-priorities/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: petitions_xoaMucUuTienVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/task-types — Danh mục loại nhiệm vụ của xã — dùng cho ô chọn loại trên biểu mẫu nhiệm vụ và bộ lọc */
export type petitions_get_task_types = {
  duongDan: "/api/v1/task-types";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: petitions_danhSachLoaiNhiemVuRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/task-types — Thêm một loại nhiệm vụ của riêng xã vào danh mục */
export type petitions_post_task_types = {
  duongDan: "/api/v1/task-types";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: petitions_themLoaiNhiemVuVao;
  phanHoi: {
    201: petitions_loaiNhiemVuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/task-types/{id} — Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một loại nhiệm vụ */
export type petitions_patch_task_types_by_id = {
  duongDan: "/api/v1/task-types/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: petitions_suaLoaiNhiemVuVao;
  phanHoi: {
    200: petitions_loaiNhiemVuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/task-types/{id} — Xoá mềm một mục danh mục do xã tự thêm, kèm lý do bắt buộc */
export type petitions_delete_task_types_by_id = {
  duongDan: "/api/v1/task-types/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: petitions_xoaLoaiNhiemVuVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/working-hours — Lịch làm việc thông thường của xã — mỗi dòng là một CA, nghỉ trưa là khoảng hở giữa hai ca */
export type identity_get_working_hours = {
  duongDan: "/api/v1/working-hours";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_danhSachCaLamViecRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};
