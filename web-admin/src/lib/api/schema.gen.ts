// TỆP SINH RA. ĐỪNG SỬA TAY — sửa tay mất sạch ở lần sinh sau (luật 9, bất biến 8).
//
// Nguồn: kb/20-contracts/openapi.json (do tools/apidoc sinh từ khai báo route trong
// */internal/). Sinh lại: npm run gen:api — trong web-admin.
//
// Hợp đồng: ViGov — REST API cho web quản trị v1

export type comms_danhMucRa = {
  "id": string;
  "name": string;
  "slug": string;
  "parent_id": string;
  "order": number;
  "created_at": string;
};

export type comms_danhSachDanhMucRa = {
  "items": Array<comms_danhMucRa>;
};

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

export type comms_noiDungRa = {
  "id": string;
  "type": string;
  "category_id": string;
  "title": string;
  "summary": string;
  "body"?: string | null;
  "image_url": string;
  "has_image": boolean;
  "published_on": string;
  "view_count": number;
  "status": string;
  "source": string;
  "source_url": string;
  "source_ref": string;
  "hand_edited": boolean;
  "author_code": string;
  "created_at": string;
  "updated_at": string;
};

export type comms_phatHanhThongBaoVao = {
  "title": string;
  "body": string;
  "org_unit_ids"?: Array<string>;
  "recipient_codes"?: Array<string>;
  "pinned"?: boolean;
  "ack_required"?: boolean;
  "email_requested"?: boolean;
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

export type comms_suaNoiDungVao = {
  "type"?: string | null;
  "category_id"?: string | null;
  "title"?: string | null;
  "summary"?: string | null;
  "body"?: string | null;
  "image_url"?: string | null;
  "publish"?: boolean | null;
};

export type comms_themDanhMucVao = {
  "name": string;
  "slug": string;
  "parent_id"?: string;
  "order"?: number;
};

export type comms_themLoaiTaiNguyenVao = {
  "code": string;
  "label": string;
  "order"?: number;
  "is_default"?: boolean;
  "source"?: string | null;
  "tier"?: number | null;
};

export type comms_themNoiDungVao = {
  "type": string;
  "title": string;
  "category_id"?: string;
  "summary"?: string;
  "body"?: string;
  "image_url"?: string;
  "publish"?: boolean;
};

export type comms_thongBaoRa = {
  "id": string;
  "title": string;
  "body": string;
  "status": string;
  "pinned": boolean;
  "ack_required": boolean;
  "email_requested": boolean;
  "email_status": string;
  "author_code": string;
  "recipient_count": number;
  "ack_count": number;
  "issued_at": string | null;
  "created_at": string;
};

export type comms_xoaLoaiTaiNguyenVao = {
  "reason": string;
};

export type documents_capSoVanBanDiVao = {
  /** YYYY-MM-DD */
  "document_date": string;
  "document_type": string;
  "summary": string;
  "recipient": string;
  "signer"?: string;
  "number"?: number | null;
};

export type documents_chuyenVanBanVao = {
  "to_unit": string;
  "assignee"?: string;
  "reason": string;
};

export type documents_danhSachLichSuChuyenRa = {
  "items": Array<documents_lichSuChuyenRa>;
};

export type documents_danhSachLoaiVanBanRa = {
  "items": Array<documents_loaiVanBanRa>;
};

export type documents_goVanBanVao = {
  "reason": string;
};

export type documents_lichSuChuyenRa = {
  "id": string;
  "document_id": string;
  /** RFC 3339 — the instant of the act */
  "routed_at": string;
  /** staff business code of who routed it */
  "routed_by": string;
  /** the state the document moved INTO by this act */
  "status": string;
  /** empty on the first routing: nobody held it */
  "from_unit"?: string;
  "to_unit": string;
  /** empty = "Để bộ phận tự phân công" */
  "assignee"?: string;
  "reason": string;
  /** RFC 3339 — when the row was written */
  "created_at": string;
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

export type documents_suaVanBanDenVao = {
  "received_date"?: string | null;
  "reference_no"?: string | null;
  "document_date"?: string | null;
  "issuing_body"?: string | null;
  "document_type"?: string | null;
  "summary"?: string | null;
  "urgency"?: string | null;
  "number"?: number | null;
  "status"?: string | null;
  "due_at"?: string | null;
};

export type documents_suaVanBanDiVao = {
  "document_date"?: string | null;
  "document_type"?: string | null;
  "summary"?: string | null;
  "recipient"?: string | null;
  "signer"?: string | null;
  "number"?: number | null;
};

export type documents_themLoaiVanBanVao = {
  "code": string;
  "label": string;
  "order"?: number;
  "is_default"?: boolean;
  "source"?: string | null;
  "tier"?: number | null;
};

export type documents_themVanBanDenVao = {
  /** YYYY-MM-DD */
  "received_date": string;
  "reference_no"?: string;
  "document_date"?: string;
  "issuing_body": string;
  "document_type": string;
  "summary": string;
  "urgency"?: string;
  "number"?: number | null;
  "status"?: string | null;
  "due_at"?: string | null;
};

export type documents_vanBanDenRa = {
  "id": string;
  "number": number;
  "year": number;
  /** YYYY-MM-DD, the day it arrived */
  "received_date": string;
  /** "1742-CV/BTCTU" */
  "reference_no"?: string;
  /** YYYY-MM-DD, the day it was signed */
  "document_date"?: string;
  "issuing_body": string;
  /** the catalogue CODE */
  "document_type": string;
  "summary": string;
  "urgency"?: string;
  /** identity's `bo_phan.id` */
  "holding_unit"?: string;
  /** a STAFF BUSINESS CODE */
  "assignee"?: string;
  /** RFC 3339 — the commitment, fixed once at booking */
  "due_at": string;
  /** `moi-vao-so` … — Vietnamese without diacritics (ADR 0011) */
  "status": string;
  "created_by": string;
  "created_at": string;
  "updated_at": string;
};

export type documents_vanBanDiRa = {
  "id": string;
  "number": number;
  "year": number;
  /** YYYY-MM-DD, the day it was signed and issued */
  "document_date": string;
  /** the catalogue CODE */
  "document_type": string;
  "summary": string;
  "recipient": string;
  "signer"?: string;
  "created_by": string;
  "created_at": string;
  "updated_at": string;
};

export type documents_xoaLoaiVanBanVao = {
  "reason": string;
};

export type finance_bangDayDuRa = {
  "sheet": finance_bangRa;
  "columns": Array<finance_cotRa>;
  "lines": Array<finance_dongRa>;
  "summary": finance_tomTatRa;
};

export type finance_bangRa = {
  "id": string;
  /** "NS-2026-CHI-01" — the business code the audit trail is filed under */
  "code": string;
  "year": number;
  /** `thu` | `chi` */
  "kind": string;
  "revision": number;
  "title": string;
  "unit": string;
  "unit_label": string;
  "unit_warning"?: string;
  /** YYYY-MM-DD */
  "cumulative_to"?: string;
  "source_file"?: string;
  /** RFC 3339 */
  "loaded_at"?: string;
};

export type finance_chiSoNamRa = {
  "year": number;
  /** Thu đạt dự toán */
  "revenue_achievement": finance_chiSoRa;
  /** Chi đạt dự toán */
  "expenditure_achievement": finance_chiSoRa;
  /** Cân đối thu - chi */
  "balance": finance_soTienRa;
  "revenue_totals": Array<finance_oTongRa>;
};

export type finance_chiSoRa = {
  "name": string;
  "basis_points": number | null;
  "unavailable_reason"?: string;
};

export type finance_chungTuRa = {
  "id": string;
  "project_id": string;
  /** YYYY-MM-DD */
  "payment_date": string;
  /** đồng, always > 0 (open question #30) */
  "amount": number;
  "description": string;
  "counterparty"?: string;
  "voucher_no"?: string;
  "funding_source_id"?: string;
  "status": string;
  /** `nguoi_nhap_id` — a staff business code */
  "entered_by": string;
  /** `nguoi_xac_nhan_id` */
  "confirmed_by"?: string;
  /** `nguoi_khoa_id` */
  "locked_by"?: string;
  /** RFC 3339 */
  "locked_at"?: string;
  "unlocked_by"?: string;
  "unlocked_at"?: string;
  "unlock_reason"?: string;
  "unlock_count": number;
};

export type finance_cotRa = {
  "id": string;
  "name": string;
  "order": number;
  /** `so` | `phan_tram` */
  "type": string;
  "formula"?: string;
  "role"?: string;
};

export type finance_cotVao = {
  "name": string;
  "order": number;
  /** `so` | `phan_tram` */
  "type": string;
  /** required for `phan_tram`, refused on `so` */
  "formula"?: string;
  /** one of the six; empty for an ordinary column */
  "role"?: string;
};

export type finance_danhSachDotRa = {
  "line_id": string;
  "method": string;
  /** newest first: date DESC, then recording time DESC */
  "entries": Array<finance_dotRa>;
};

export type finance_danhSachDuAnRa = {
  "items": Array<finance_duAnRa>;
  "year": number;
  "delay_threshold": number;
  "delay_threshold_source": string;
};

export type finance_danhSachHangMucRa = {
  "items": Array<finance_hangMucRa>;
};

export type finance_dongRa = {
  "id": string;
  "parent_id"?: string;
  "no": string;
  "name": string;
  "order": number;
  "method": string;
  "level": number;
  "is_headline": boolean;
  /** columnID -> đồng, or null for an empty cell */
  "values": Record<string, number | null>;
  "unavailable_reasons"?: Record<string, string>;
};

export type finance_dotRa = {
  "id": string;
  "line_id": string;
  /** YYYY-MM-DD */
  "date": string;
  "content": string;
  "counterparty"?: string;
  "document_no"?: string;
  /** RFC 3339; absent on the 201 of a create */
  "recorded_at"?: string;
  "values": Record<string, number | null>;
  "unavailable_reasons"?: Record<string, string>;
};

export type finance_duAnGhiRa = {
  "id": string;
  "code": string;
  "year": number;
  "category_id": string;
  "name": string;
  "description"?: string;
  "planned_amount": number;
  "approved_amount": number;
  "approved_amount_set": boolean;
  "org_unit_id"?: string;
  "assignee_id"?: string;
  "start_date"?: string;
  "completion_date"?: string;
  "disbursement_deadline": string;
  "funding_allocations"?: Array<finance_phanBoRa>;
  "funding_allocated_total": number;
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
  "delay_threshold": number;
  "delay_threshold_source": string;
};

export type finance_ghiDotVao = {
  /** YYYY-MM-DD */
  "date": string;
  "content": string;
  "counterparty"?: string;
  "document_no"?: string;
  "values": Record<string, number | null>;
};

export type finance_goChungTuVao = {
  "reason": string;
};

export type finance_goVao = {
  "reason": string;
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

export type finance_moKhoaVao = {
  "reason": string;
};

export type finance_oTongRa = {
  "column_id": string;
  "name": string;
  "role"?: string;
  "value": number | null;
  "unavailable_reason"?: string;
};

export type finance_phanBoRa = {
  "id": string;
  "funding_source_id": string;
  "amount": number;
};

export type finance_phanBoVao = {
  "funding_source_id": string;
  /** đồng */
  "amount": number;
};

export type finance_soTienRa = {
  /** đồng */
  "amount": number | null;
  "unavailable_reason"?: string;
};

export type finance_suaBangVao = {
  "title"?: string | null;
  /** YYYY-MM-DD, or "" to clear */
  "cumulative_to"?: string | null;
  /** `dong` | `nghin-dong` | `trieu-dong` */
  "unit"?: string | null;
  "year"?: number | null;
  "kind"?: string | null;
  "code"?: string | null;
  "columns"?: Array<finance_cotVao>;
};

export type finance_suaChungTuVao = {
  "payment_date"?: string | null;
  "amount"?: number | null;
  "description"?: string | null;
  "counterparty"?: string | null;
  "voucher_no"?: string | null;
  "funding_source_id"?: string | null;
  "project_id"?: string | null;
  "status"?: string | null;
};

export type finance_suaDongVao = {
  "no"?: string | null;
  "name"?: string | null;
  "order"?: number | null;
  "values"?: Record<string, number | null>;
  "sheet_id"?: string | null;
  "parent_id"?: string | null;
  "method"?: string | null;
  "level"?: number | null;
  "is_headline"?: boolean | null;
};

export type finance_suaDuAnVao = {
  "category_id"?: string | null;
  "name"?: string | null;
  "description"?: string | null;
  "planned_amount"?: number | null;
  "approved_amount"?: number | null;
  "org_unit_id"?: string | null;
  "assignee_id"?: string | null;
  "start_date"?: string | null;
  "completion_date"?: string | null;
  "disbursement_deadline"?: string | null;
  "code"?: string | null;
  "year"?: number | null;
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

export type finance_taoBangVao = {
  "year": number;
  "kind": string;
  "title": string;
  /** `dong` | `nghin-dong` | `trieu-dong` — display only, figures stay đồng */
  "unit": string;
  /** YYYY-MM-DD */
  "cumulative_to"?: string;
  "columns": Array<finance_cotVao>;
  /** present only so it can be refused */
  "code"?: string | null;
};

export type finance_themChungTuVao = {
  "project_id": string;
  /** YYYY-MM-DD */
  "payment_date": string;
  /** đồng */
  "amount": number;
  "description": string;
  "counterparty"?: string;
  "voucher_no"?: string;
  "funding_source_id"?: string;
  "status"?: string | null;
};

export type finance_themDongVao = {
  "sheet_id": string;
  "parent_id"?: string;
  "no"?: string;
  "name": string;
  "order": number;
  "method"?: string | null;
  "level"?: number | null;
  "is_headline"?: boolean | null;
};

export type finance_themDuAnVao = {
  "code": string;
  "year": number;
  "category_id": string;
  "name": string;
  "description"?: string;
  /** `ke_hoach_von_nam`, đồng */
  "planned_amount": number;
  /** 0 = "same as this year's plan" (§9) */
  "approved_amount"?: number;
  "org_unit_id"?: string;
  "assignee_id"?: string;
  /** YYYY-MM-DD */
  "start_date"?: string;
  /** YYYY-MM-DD */
  "completion_date"?: string;
  /** YYYY-MM-DD */
  "disbursement_deadline"?: string;
  "funding_allocations"?: Array<finance_phanBoVao>;
};

export type finance_themHangMucVao = {
  "code": string;
  "label": string;
  "order"?: number;
  "is_default"?: boolean;
  "source"?: string | null;
  "tier"?: number | null;
};

export type finance_tomTatRa = {
  "headline_line_id"?: string;
  "unavailable_reason"?: string;
  "cells": Array<finance_oTongRa>;
  "indicator": finance_chiSoRa;
};

export type finance_xoaDuAnVao = {
  "reason": string;
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

export type identity_boPhanDaGhiRa = {
  "id": string;
  "code": string;
  "name": string;
  "parent_id": string;
  "order": number;
};

export type identity_boPhanRa = {
  /** ULID — what other records reference */
  "id": string;
  /** slug */
  "code": string;
  "name": string;
  "parent_id": string;
  "order": number;
  "staff_count": number;
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

export type identity_canBoChonNguoiRa = {
  "code": string;
  "full_name": string;
  "position": string;
  /** "" when the person sits in no unit */
  "department_id": string;
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
  "has_zalo": boolean;
  "published": boolean;
  "display_order": number | null;
  "consent_recorded_at": string | null;
};

export type identity_capQuyenRa = {
  /** vaiTroCotRa.ID — a role OF THIS COMMUNE */
  "role_id": string;
  "permission": string;
};

export type identity_capTaiKhoanRa = {
  "staff": identity_canBoTomTat;
  /** ⚠ BÍ MẬT ĐI RA, CÓ CHỦ Ý — Mật khẩu tạm dùng MỘT LẦN, khách chốt 22/09/2026 (câu mở #9): quản trị viên đọc lại cho cán bộ, máy chủ không giữ bản trần và không trả lại lần thứ hai. Đổi ở lần đăng nhập đầu là bắt buộc. */
  "temporary_password": string;
};

export type identity_cotPhanQuyenRa = {
  "role_id": string;
  /** sắp theo thứ tự chữ; `[]` khi vai trò không giữ quyền nào */
  "permissions": Array<string>;
};

export type identity_danhBaChonNguoiRa = {
  "items": Array<identity_canBoChonNguoiRa>;
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

export type identity_danhSachSLARa = {
  "items": Array<identity_dongSLARa>;
  "problems": Array<identity_vanDeSLARa>;
};

export type identity_danhSachThonToDanPhoRa = {
  "items": Array<identity_thonToDanPhoRa>;
};

export type identity_danhSachVaiTroRa = {
  "items": Array<identity_vaiTroMucRa>;
};

export type identity_datCongKhaiVao = {
  "published": boolean | null;
  "consent_confirmed": boolean;
  "display_order": number | null;
};

export type identity_datVaiTroVao = {
  "role_id": string;
};

export type identity_doiMatKhauVao = {
  "current_password": string;
  "new_password": string;
};

export type identity_dongSLARa = {
  /** ULID — what PATCH /api/v1/sla/{id} references */
  "id": string;
  "work_kind": string;
  "field": string;
  "is_default": boolean;
  "acknowledge_hours": number;
  "resolve_hours": number;
  "due_soon_hours": number;
  "escalate_leader_hours": number;
  "escalate_president_hours": number;
};

export type identity_gieoLichRa = {
  "seeded": number;
  "kept": number;
  "skipped": number;
};

export type identity_gieoNgayNghiLeVao = {
  "year": number | null;
};

export type identity_gieoSLARa = {
  "seeded": number;
  "kept": number;
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
  "order": number;
  "source": string;
  "tier": number;
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
  "order": number;
  "source": string;
  "tier": number;
};

export type identity_luuPhanQuyenVao = {
  "permissions": Array<string> | null;
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
  "must_change_password": boolean;
};

export type identity_phienHienTaiRa = {
  "sid": string;
  "expires_at": string;
  "staff": identity_canBoGon;
  "role": identity_vaiTroGon | null;
  "permissions": Array<string>;
  "must_change_password": boolean;
};

export type identity_quyenMucRa = {
  /** "task.extend" */
  "code": string;
  /** "Duyệt gia hạn" */
  "label": string;
};

export type identity_suaBoPhanVao = {
  "name"?: string | null;
  "parent_id"?: string | null;
  "order"?: number | null;
  "code"?: string | null;
};

export type identity_suaCaLamViecVao = {
  "weekday": number | null;
  "start": string | null;
  "end": string | null;
  "note": string | null;
};

export type identity_suaCanBoVao = {
  "full_name": string | null;
  "position": string | null;
  "email": string | null;
  "org_unit_id": string | null;
  "office_phone": string | null;
  "mobile": string | null;
  "has_zalo"?: boolean | null;
};

export type identity_suaDanhMucVao = {
  "label"?: string | null;
  "order"?: number | null;
  "active"?: boolean | null;
  "is_default"?: boolean | null;
  "code"?: string | null;
  "source"?: string | null;
  "tier"?: number | null;
};

export type identity_suaNgayLamBuVao = {
  "date": string | null;
  "start": string | null;
  "end": string | null;
  "name": string | null;
};

export type identity_suaNgayNghiLeVao = {
  "date": string | null;
  "name": string | null;
};

export type identity_suaSLAVao = {
  "acknowledge_hours": number | null;
  "resolve_hours": number | null;
  "due_soon_hours": number | null;
  "escalate_leader_hours": number | null;
  "escalate_president_hours": number | null;
};

export type identity_thanDangNhap = {
  "email": string;
  "password": string;
};

export type identity_themBoPhanVao = {
  "name": string;
  "parent_id"?: string;
  "order"?: number | null;
  "code"?: string;
};

export type identity_themCaLamViecVao = {
  "weekday": number;
  "start": string;
  "end": string;
  "note": string;
};

export type identity_themCanBoVao = {
  "full_name": string;
  "position": string;
  "email": string;
  "org_unit_id": string;
  "office_phone": string;
  "mobile": string;
};

export type identity_themDanhMucVao = {
  "code": string;
  "label": string;
  "order"?: number;
  "is_default"?: boolean;
  "source"?: string | null;
  "tier"?: number | null;
};

export type identity_themNgayLamBuVao = {
  "date": string;
  "start": string;
  "end": string;
  "name": string;
};

export type identity_themNgayNghiLeVao = {
  "date": string;
  "name": string;
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

export type identity_timCanBoVao = {
  "q": string;
  "unit": string;
  "published": boolean | null;
  "limit": number | null;
  "cursor": string;
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

export type identity_vanDeSLARa = {
  "kind": string;
  "work_kind": string;
  "message": string;
};

export type identity_xoaCanBoVao = {
  "reason": string;
};

export type identity_xoaDanhMucVao = {
  "reason": string;
};

export type identity_xoaLichVao = {
  "reason": string;
};

export type page_Result_comms_noiDungRa = {
  "items": Array<comms_noiDungRa>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

export type page_Result_comms_thongBaoRa = {
  "items": Array<comms_thongBaoRa>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

export type page_Result_documents_vanBanDenRa = {
  "items": Array<documents_vanBanDenRa>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

export type page_Result_documents_vanBanDiRa = {
  "items": Array<documents_vanBanDiRa>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

export type page_Result_identity_canBoTomTat = {
  "items": Array<identity_canBoTomTat>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

export type page_Result_petitions_bienBanRa = {
  "items": Array<petitions_bienBanRa>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

export type page_Result_petitions_nhiemVuRa = {
  "items": Array<petitions_nhiemVuRa>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

export type page_Result_petitions_phieuPhanAnhRa = {
  "items": Array<petitions_phieuPhanAnhRa>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

export type petitions_bienBanRa = {
  "id": string;
  "title": string;
  "held_on": string;
  "reference_no": string;
  "location": string;
  "chaired_by": string;
  "conclusions": Array<petitions_ketLuanRa>;
  "task_count": number;
  "task_done_count": number;
  "created_by": string;
  "created_at": string;
};

export type petitions_chuyenCapTrenVao = {
  "reason": string;
  "receiving_body": string;
};

export type petitions_danhSachLoaiNhiemVuRa = {
  "items": Array<petitions_loaiNhiemVuRa>;
};

export type petitions_danhSachMucUuTienRa = {
  "items": Array<petitions_mucUuTienRa>;
};

export type petitions_danhSachTrangThaiNhiemVuRa = {
  "items": Array<petitions_trangThaiNhiemVuRa>;
};

export type petitions_deNghiLuiHanRa = {
  "id": string;
  "requested_by": string;
  "decided_by"?: string;
  "new_due_at": string;
  "reason": string;
  "status": string;
  "requested_at": string;
  "decided_at": string | null;
};

export type petitions_deNghiLuiHanVao = {
  "new_due_at": string;
  "reason": string;
};

export type petitions_doiTrangThaiVao = {
  "status": string;
  "note"?: string;
};

export type petitions_dongPhieuVao = {
  "result": string;
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

export type petitions_ketLuanRa = {
  "id": string;
  "ordinal": number;
  "content": string;
  "task_count": number;
  "task_done_count": number;
  "created_at": string;
};

export type petitions_khongTiepNhanVao = {
  "reason": string;
};

export type petitions_loaiNhiemVuRa = {
  /** ULID of THIS lookup row */
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

export type petitions_nhiemVuRa = {
  "code": string;
  "type": string;
  "bloc": string;
  "priority": string;
  "title": string;
  "description": string;
  "status": string;
  "source": string;
  "source_id": string;
  "unit": string;
  "assignee": string;
  "assigner": string;
  "lead_unit": string;
  "monitor": string;
  "due_at": string | null;
  "original_due_at": string | null;
  "completed_at": string | null;
  "progress": number;
  "result_summary": string;
  "note": string;
  "leader_approved": boolean;
  "superior_acknowledged": boolean;
  "parent": string;
  "created_by": string;
  "created_at": string;
  "documents"?: Array<petitions_nhiemVuVanBanRa> | null;
};

export type petitions_nhiemVuVanBanRa = {
  "id": string;
  "group": string;
  "reference": string;
  "date": string;
  "summary": string;
  "position": number;
};

export type petitions_phanCongVao = {
  "unit": string;
  "assignee"?: string;
};

export type petitions_phanLoaiVao = {
  "field": string;
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
  "result": string;
  "reason"?: string;
  "receiving_body"?: string;
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
  "classify_due": string | null;
  "unit": string;
  "assignee": string;
  "result": string;
  "reason"?: string;
  "receiving_body"?: string;
  "branch_ended_at"?: string | null;
  "public": boolean;
};

export type petitions_quyetDinhLuiHanVao = {
  "decision": string;
  "note"?: string;
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

export type petitions_suaNhiemVuVao = {
  "bloc"?: string | null;
  "title"?: string | null;
  "description"?: string | null;
  "priority"?: string | null;
  "progress"?: number | null;
  "result_summary"?: string | null;
  "note"?: string | null;
  "leader_approved"?: boolean | null;
  "superior_acknowledged"?: boolean | null;
  "parent"?: string | null;
  "documents"?: Array<petitions_vanBanNhiemVuVao> | null;
};

export type petitions_suaTrangThaiNhiemVuVao = {
  "label"?: string | null;
  "order"?: number | null;
  "code"?: string | null;
  "active"?: boolean | null;
};

export type petitions_tachKetLuanVao = {
  "code"?: string;
  "auto_code": boolean;
  "type": string;
  "bloc"?: string;
  "title": string;
  "description"?: string;
  "priority"?: string;
  "unit"?: string;
  "assignee"?: string;
  "assigner"?: string;
  "lead_unit"?: string;
  "monitor"?: string;
  "due_at"?: string | null;
  "parent"?: string;
};

export type petitions_taoBienBanVao = {
  "title": string;
  "held_on": string;
  "reference_no"?: string;
  "location"?: string;
  "chaired_by": string;
  "content"?: string;
  "attendees"?: Array<string>;
  "conclusions"?: Array<string>;
};

export type petitions_taoNhiemVuVao = {
  "code"?: string;
  "auto_code": boolean;
  "type": string;
  "bloc"?: string;
  "title": string;
  "description"?: string;
  "priority"?: string;
  "source"?: string;
  "source_id"?: string;
  "unit"?: string;
  "assignee"?: string;
  "assigner"?: string;
  "lead_unit"?: string;
  "monitor"?: string;
  "due_at"?: string | null;
  "parent"?: string;
  "documents"?: Array<petitions_vanBanNhiemVuVao>;
};

export type petitions_themKetLuanVao = {
  "content": string;
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

export type petitions_trangThaiNhiemVuRa = {
  "code": string;
  "label": string;
  "order": number;
  "role": string;
  "default_label": string;
  "default_order": number;
  "customised": boolean;
};

export type petitions_vanBanNhiemVuVao = {
  "id"?: string;
  "group": string;
  "reference"?: string;
  "date"?: string;
  "summary": string;
};

export type petitions_xoaLoaiNhiemVuVao = {
  "reason": string;
};

export type petitions_xoaMucUuTienVao = {
  "reason": string;
};

export type petitions_xoaNhiemVuVao = {
  "reason": string;
};

/** GET /api/v1/announcements — Sổ thông báo nội bộ của xã — một trang thẻ, mới nhất ở trên, kèm bộ đếm xác nhận */
export type comms_get_announcements = {
  duongDan: "/api/v1/announcements";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "limit"?: number;
    "cursor"?: string;
    "sort"?: "created_at";
    "order"?: "asc" | "desc";
  };
  than: never;
  phanHoi: {
    200: page_Result_comms_thongBaoRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/announcements — Phát hành một thông báo nội bộ tới các cán bộ được chọn đích danh */
export type comms_post_announcements = {
  duongDan: "/api/v1/announcements";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: comms_phatHanhThongBaoVao;
  phanHoi: {
    201: comms_thongBaoRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
    501: httpx_Error;
  };
};

/** DELETE /api/v1/budget-entries/{id} — Gỡ mềm một đợt thu, chi, kèm lý do bắt buộc */
export type finance_delete_budget_entries_by_id = {
  duongDan: "/api/v1/budget-entries/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_goVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/budget-indicators — Ba chỉ số ngân sách của một năm: thu đạt dự toán, chi đạt dự toán, cân đối thu - chi */
export type finance_get_budget_indicators = {
  duongDan: "/api/v1/budget-indicators";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "year": string;
  };
  than: never;
  phanHoi: {
    200: finance_chiSoNamRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/budget-lines — Thêm một khoản mục vào cây của bảng ngân sách */
export type finance_post_budget_lines = {
  duongDan: "/api/v1/budget-lines";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: finance_themDongVao;
  phanHoi: {
    201: finance_dongRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
    503: httpx_Error;
  };
};

/** PATCH /api/v1/budget-lines/{id} — Sửa số thứ tự, tên, cách tính (manual/entries) hoặc các ô số của một khoản mục chưa có dòng con */
export type finance_patch_budget_lines_by_id = {
  duongDan: "/api/v1/budget-lines/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_suaDongVao;
  phanHoi: {
    200: finance_dongRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/budget-lines/{id} — Gỡ mềm một khoản mục chưa có dòng con, kèm lý do bắt buộc */
export type finance_delete_budget_lines_by_id = {
  duongDan: "/api/v1/budget-lines/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_goVao;
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

/** GET /api/v1/budget-lines/{id}/entries — Các đợt thu, chi đã ghi của một khoản mục, mới nhất trước, kèm số tiền theo từng cột số */
export type finance_get_budget_lines_by_id_entries = {
  duongDan: "/api/v1/budget-lines/{id}/entries";
  phuongThuc: "GET";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: finance_danhSachDotRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/budget-lines/{id}/entries — Ghi một đợt thu, chi vào một khoản mục lá */
export type finance_post_budget_lines_by_id_entries = {
  duongDan: "/api/v1/budget-lines/{id}/entries";
  phuongThuc: "POST";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_ghiDotVao;
  phanHoi: {
    201: finance_dotRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
    503: httpx_Error;
  };
};

/** POST /api/v1/budget-lines/{id}/headline — Đánh dấu một khoản mục là dòng tổng của bảng — số tóm tắt và chỉ số đọc từ dòng này */
export type finance_post_budget_lines_by_id_headline = {
  duongDan: "/api/v1/budget-lines/{id}/headline";
  phuongThuc: "POST";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: finance_dongRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/budget-sheets — Bảng thu hoặc chi của một năm ngân sách: cột, cây khoản mục, số liệu và ô tóm tắt */
export type finance_get_budget_sheets = {
  duongDan: "/api/v1/budget-sheets";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "kind"?: string;
    "year"?: string;
  };
  than: never;
  phanHoi: {
    200: finance_bangDayDuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/budget-sheets — Tạo bảng thu hoặc chi cho một năm ngân sách, kèm bộ cột của biểu */
export type finance_post_budget_sheets = {
  duongDan: "/api/v1/budget-sheets";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: finance_taoBangVao;
  phanHoi: {
    201: finance_bangRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
    503: httpx_Error;
  };
};

/** PATCH /api/v1/budget-sheets/{id} — Sửa tiêu đề, đơn vị tính hiển thị hoặc mốc luỹ kế của một bảng ngân sách */
export type finance_patch_budget_sheets_by_id = {
  duongDan: "/api/v1/budget-sheets/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_suaBangVao;
  phanHoi: {
    200: finance_bangRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/budget-sheets/{id} — Gỡ mềm cả bảng ngân sách của một năm, kèm lý do bắt buộc */
export type finance_delete_budget_sheets_by_id = {
  duongDan: "/api/v1/budget-sheets/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_goVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
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

/** GET /api/v1/citizen-reports — Danh sách phiếu phản ánh của xã — phân trang theo con trỏ, lọc theo trạng thái · lĩnh vực · thôn · bộ phận · kênh · trễ hạn · phạm vi (`scope=mine`: phiếu đang giao cho chính người gọi, mã lấy từ phiên) */
export type petitions_get_citizen_reports = {
  duongDan: "/api/v1/citizen-reports";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "limit"?: number;
    "cursor"?: string;
    "sort"?: "booked_at" | "created_at";
    "order"?: "asc" | "desc";
    "channel"?: string;
    "field"?: string;
    "hamlet"?: string;
    "late"?: string;
    "q"?: string;
    "scope"?: string;
    "status"?: string;
    "unit"?: string;
  };
  than: never;
  phanHoi: {
    200: page_Result_petitions_phieuPhanAnhRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
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

/** POST /api/v1/citizen-reports/{maTraCuu}/assignment — Chuyển phiếu phản ánh cho một bộ phận xử lý, kèm cán bộ phụ trách nếu đã biết */
export type petitions_post_citizen_reports_by_maTraCuu_assignment = {
  duongDan: "/api/v1/citizen-reports/{maTraCuu}/assignment";
  phuongThuc: "POST";
  thamSo: {
    "maTraCuu": string;
  };
  truyVan: {
  };
  than: petitions_phanCongVao;
  phanHoi: {
    200: petitions_phieuPhanAnhRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/citizen-reports/{maTraCuu}/classification — Chốt lĩnh vực cho phiếu phản ánh — hành vi ẤN ĐỊNH hạn xử lý xong theo cấu hình của xã */
export type petitions_post_citizen_reports_by_maTraCuu_classification = {
  duongDan: "/api/v1/citizen-reports/{maTraCuu}/classification";
  phuongThuc: "POST";
  thamSo: {
    "maTraCuu": string;
  };
  truyVan: {
  };
  than: petitions_phanLoaiVao;
  phanHoi: {
    200: petitions_phieuPhanAnhRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/citizen-reports/{maTraCuu}/closure — Đóng phiếu phản ánh kèm kết quả xử lý người dân đọc được */
export type petitions_post_citizen_reports_by_maTraCuu_closure = {
  duongDan: "/api/v1/citizen-reports/{maTraCuu}/closure";
  phuongThuc: "POST";
  thamSo: {
    "maTraCuu": string;
  };
  truyVan: {
  };
  than: petitions_dongPhieuVao;
  phanHoi: {
    200: petitions_phieuPhanAnhRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/citizen-reports/{maTraCuu}/referral — Chuyển phiếu phản ánh lên/sang cơ quan có thẩm quyền — kèm lý do và cơ quan tiếp nhận */
export type petitions_post_citizen_reports_by_maTraCuu_referral = {
  duongDan: "/api/v1/citizen-reports/{maTraCuu}/referral";
  phuongThuc: "POST";
  thamSo: {
    "maTraCuu": string;
  };
  truyVan: {
  };
  than: petitions_chuyenCapTrenVao;
  phanHoi: {
    200: petitions_phieuPhanAnhRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/citizen-reports/{maTraCuu}/rejection — Không tiếp nhận phiếu phản ánh — kèm lý do người dân đọc được */
export type petitions_post_citizen_reports_by_maTraCuu_rejection = {
  duongDan: "/api/v1/citizen-reports/{maTraCuu}/rejection";
  phuongThuc: "POST";
  thamSo: {
    "maTraCuu": string;
  };
  truyVan: {
  };
  than: petitions_khongTiepNhanVao;
  phanHoi: {
    200: petitions_phieuPhanAnhRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/citizen-reports/{maTraCuu}/status — Chuyển phiếu phản ánh sang bước kế tiếp của luồng chính (máy trạng thái quyết định bước nào) */
export type petitions_post_citizen_reports_by_maTraCuu_status = {
  duongDan: "/api/v1/citizen-reports/{maTraCuu}/status";
  phuongThuc: "POST";
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
    409: httpx_Error;
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

/** GET /api/v1/content-categories — Danh mục tin nội bộ của Mini App — cây phẳng, dùng cho ô chọn ở §7 và bộ lọc ở §6 */
export type comms_get_content_categories = {
  duongDan: "/api/v1/content-categories";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: comms_danhSachDanhMucRa;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/content-categories — Thêm một danh mục tin của riêng xã vào cây danh mục Mini App */
export type comms_post_content_categories = {
  duongDan: "/api/v1/content-categories";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: comms_themDanhMucVao;
  phanHoi: {
    201: comms_danhMucRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/content-items — Sổ nội dung Mini App của xã — một trang của bảng §6, lọc theo loại, danh mục và tiêu đề */
export type comms_get_content_items = {
  duongDan: "/api/v1/content-items";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "limit"?: number;
    "cursor"?: string;
    "sort"?: "created_at";
    "order"?: "asc" | "desc";
    "category": string;
    "q": string;
    "type": string;
  };
  than: never;
  phanHoi: {
    200: page_Result_comms_noiDungRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/content-items — Soạn một mục nội dung cho Mini App — chưa bật `publish` thì bà con chưa thấy */
export type comms_post_content_items = {
  duongDan: "/api/v1/content-items";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: comms_themNoiDungVao;
  phanHoi: {
    201: comms_noiDungRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/content-items/{id} — Một mục nội dung Mini App kèm toàn văn — dùng cho modal sửa ở §7 */
export type comms_get_content_items_by_id = {
  duongDan: "/api/v1/content-items/{id}";
  phuongThuc: "GET";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: comms_noiDungRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/content-items/{id} — Sửa một mục nội dung Mini App — sửa bài đồng bộ về sẽ khoá không cho lượt đồng bộ sau ghi đè */
export type comms_patch_content_items_by_id = {
  duongDan: "/api/v1/content-items/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: comms_suaNoiDungVao;
  phanHoi: {
    200: comms_noiDungRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/disbursements — Ghi nhận một chứng từ giải ngân cho dự án đầu tư */
export type finance_post_disbursements = {
  duongDan: "/api/v1/disbursements";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: finance_themChungTuVao;
  phanHoi: {
    201: finance_chungTuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
    503: httpx_Error;
  };
};

/** PATCH /api/v1/disbursements/{id} — Sửa ngày chi, số tiền, nội dung, đối tác hoặc số chứng từ của một chứng từ chưa khoá — chứng từ đã xác nhận sẽ về nháp */
export type finance_patch_disbursements_by_id = {
  duongDan: "/api/v1/disbursements/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_suaChungTuVao;
  phanHoi: {
    200: finance_chungTuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/disbursements/{id} — Gỡ mềm một chứng từ giải ngân, kèm lý do bắt buộc */
export type finance_delete_disbursements_by_id = {
  duongDan: "/api/v1/disbursements/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_goChungTuVao;
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

/** POST /api/v1/disbursements/{id}/confirmation — Xác nhận một chứng từ giải ngân (`Kế toán nhập` → `Đã xác nhận`) */
export type finance_post_disbursements_by_id_confirmation = {
  duongDan: "/api/v1/disbursements/{id}/confirmation";
  phuongThuc: "POST";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: finance_chungTuRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/disbursements/{id}/lockout — Khoá một chứng từ giải ngân (`Đã xác nhận` → `Đã khoá`) */
export type finance_post_disbursements_by_id_lockout = {
  duongDan: "/api/v1/disbursements/{id}/lockout";
  phuongThuc: "POST";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: finance_chungTuRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/disbursements/{id}/lockout — Mở khoá một chứng từ giải ngân, kèm lý do bắt buộc; người vừa khoá không tự mở lại được */
export type finance_delete_disbursements_by_id_lockout = {
  duongDan: "/api/v1/disbursements/{id}/lockout";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_moKhoaVao;
  phanHoi: {
    200: finance_chungTuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
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

/** GET /api/v1/incoming-documents — Danh sách sổ văn bản đến, phân trang theo con trỏ, lọc theo năm · trạng thái · loại · bộ phận đang giữ */
export type documents_get_incoming_documents = {
  duongDan: "/api/v1/incoming-documents";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "limit"?: number;
    "cursor"?: string;
    "sort"?: "number" | "created_at";
    "order"?: "asc" | "desc";
    "document_type"?: string;
    "holding_unit"?: string;
    "q"?: string;
    "status"?: string;
    "year"?: string;
  };
  than: never;
  phanHoi: {
    200: page_Result_documents_vanBanDenRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/incoming-documents — Vào sổ một văn bản đến; hệ thống cấp số đến và ấn định hạn xử lý theo cấu hình của xã */
export type documents_post_incoming_documents = {
  duongDan: "/api/v1/incoming-documents";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: documents_themVanBanDenVao;
  phanHoi: {
    201: documents_vanBanDenRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/incoming-documents/{id} — Một văn bản đến của xã, cùng dạng với một dòng của danh sách — dùng cho ngăn chi tiết */
export type documents_get_incoming_documents_by_id = {
  duongDan: "/api/v1/incoming-documents/{id}";
  phuongThuc: "GET";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: documents_vanBanDenRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/incoming-documents/{id} — Sửa thông tin một văn bản đến đã vào sổ (số đến, trạng thái và hạn xử lý không sửa được) */
export type documents_patch_incoming_documents_by_id = {
  duongDan: "/api/v1/incoming-documents/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: documents_suaVanBanDenVao;
  phanHoi: {
    200: documents_vanBanDenRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/incoming-documents/{id} — Gỡ một văn bản đến khỏi sổ (xoá mềm, kèm lý do bắt buộc; số đến không được cấp lại) */
export type documents_delete_incoming_documents_by_id = {
  duongDan: "/api/v1/incoming-documents/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: documents_goVanBanVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/incoming-documents/{id}/routings — Dòng thời gian chuyển xử lý của một văn bản đến, cũ nhất trước — chỉ đọc */
export type documents_get_incoming_documents_by_id_routings = {
  duongDan: "/api/v1/incoming-documents/{id}/routings";
  phuongThuc: "GET";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: documents_danhSachLichSuChuyenRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/incoming-documents/{id}/routings — Chuyển văn bản đến cho một bộ phận xử lý, kèm ý kiến chỉ đạo — ghi vào dòng thời gian không sửa được */
export type documents_post_incoming_documents_by_id_routings = {
  duongDan: "/api/v1/incoming-documents/{id}/routings";
  phuongThuc: "POST";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: documents_chuyenVanBanVao;
  phanHoi: {
    200: documents_vanBanDenRa;
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

/** POST /api/v1/investment-projects — Thêm một dự án đầu tư cho năm ngân sách, kèm phân bổ nguồn vốn nếu xã khai */
export type finance_post_investment_projects = {
  duongDan: "/api/v1/investment-projects";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: finance_themDuAnVao;
  phanHoi: {
    201: finance_duAnGhiRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
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

/** PATCH /api/v1/investment-projects/{id} — Sửa hạng mục, tên, mô tả, kế hoạch vốn, đơn vị, cán bộ phụ trách hoặc các mốc thời gian của một dự án đầu tư */
export type finance_patch_investment_projects_by_id = {
  duongDan: "/api/v1/investment-projects/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_suaDuAnVao;
  phanHoi: {
    200: finance_duAnGhiRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/investment-projects/{id} — Xoá mềm một dự án đầu tư kèm lý do bắt buộc — từ chối khi dự án còn chứng từ giải ngân */
export type finance_delete_investment_projects_by_id = {
  duongDan: "/api/v1/investment-projects/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: finance_xoaDuAnVao;
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

/** GET /api/v1/meetings — Danh sách biên bản họp của xã — mỗi biên bản kèm các kết luận và bộ đếm nhiệm vụ đã tách / đã xong */
export type petitions_get_meetings = {
  duongDan: "/api/v1/meetings";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "limit"?: number;
    "cursor"?: string;
    "sort"?: "created_at";
    "order"?: "asc" | "desc";
  };
  than: never;
  phanHoi: {
    200: page_Result_petitions_bienBanRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/meetings — Nhập một biên bản họp, kèm các kết luận đã gõ trên biểu mẫu — kết luận đánh số ① ② ③ theo thứ tự nhập */
export type petitions_post_meetings = {
  duongDan: "/api/v1/meetings";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: petitions_taoBienBanVao;
  phanHoi: {
    201: petitions_bienBanRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/meetings/{id}/conclusions — Thêm một kết luận vào biên bản đã có — số thứ tự nối tiếp số ĐÃ CẤP, kể cả khi kết luận mang số đó đã bị xoá */
export type petitions_post_meetings_by_id_conclusions = {
  duongDan: "/api/v1/meetings/{id}/conclusions";
  phuongThuc: "POST";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: petitions_themKetLuanVao;
  phanHoi: {
    201: petitions_ketLuanRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/meetings/{id}/conclusions/{stt}/task — Tách một kết luận họp thành một nhiệm vụ — nhiệm vụ giữ liên kết ngược về kết luận gốc qua cặp nguồn giao */
export type petitions_post_meetings_by_id_conclusions_by_stt_task = {
  duongDan: "/api/v1/meetings/{id}/conclusions/{stt}/task";
  phuongThuc: "POST";
  thamSo: {
    "id": string;
    "stt": string;
  };
  truyVan: {
  };
  than: petitions_tachKetLuanVao;
  phanHoi: {
    201: petitions_nhiemVuRa;
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

/** POST /api/v1/org-units — Thêm một bộ phận vào sơ đồ tổ chức của xã — mã tự sinh từ tên nếu không nhập */
export type identity_post_org_units = {
  duongDan: "/api/v1/org-units";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_themBoPhanVao;
  phanHoi: {
    201: identity_boPhanDaGhiRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/org-units/{id} — Sửa tên, dời bộ phận cha, đổi thứ tự một bộ phận — mã đã cấp không đổi */
export type identity_patch_org_units_by_id = {
  duongDan: "/api/v1/org-units/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_suaBoPhanVao;
  phanHoi: {
    200: identity_boPhanDaGhiRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/outgoing-documents — Danh sách sổ văn bản đi, phân trang theo con trỏ, lọc theo năm · loại văn bản */
export type documents_get_outgoing_documents = {
  duongDan: "/api/v1/outgoing-documents";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "limit"?: number;
    "cursor"?: string;
    "sort"?: "number" | "created_at";
    "order"?: "asc" | "desc";
    "document_type"?: string;
    "q"?: string;
    "year"?: string;
  };
  than: never;
  phanHoi: {
    200: page_Result_documents_vanBanDiRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/outgoing-documents — Cấp số văn bản đi và ghi vào sổ; số đã cấp không bao giờ cấp lại */
export type documents_post_outgoing_documents = {
  duongDan: "/api/v1/outgoing-documents";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: documents_capSoVanBanDiVao;
  phanHoi: {
    201: documents_vanBanDiRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/outgoing-documents/{id} — Sửa thông tin một văn bản đi đã cấp số (số đi và năm không sửa được) */
export type documents_patch_outgoing_documents_by_id = {
  duongDan: "/api/v1/outgoing-documents/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: documents_suaVanBanDiVao;
  phanHoi: {
    200: documents_vanBanDiRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/outgoing-documents/{id} — Gỡ một văn bản đi khỏi sổ (xoá mềm, kèm lý do bắt buộc; số đi không được cấp lại) */
export type documents_delete_outgoing_documents_by_id = {
  duongDan: "/api/v1/outgoing-documents/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: documents_goVanBanVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
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

/** POST /api/v1/public-holidays — Thêm một ngày nghỉ lễ của xã — ngày xã KHÔNG làm việc, gồm cả lễ quốc gia lẫn lễ địa phương */
export type identity_post_public_holidays = {
  duongDan: "/api/v1/public-holidays";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_themNgayNghiLeVao;
  phanHoi: {
    201: identity_ngayNghiLeRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/public-holidays/defaults — Gieo các ngày nghỉ lễ CỐ ĐỊNH THEO DƯƠNG LỊCH của một năm — BỐN ngày, không phải mười một */
export type identity_post_public_holidays_defaults = {
  duongDan: "/api/v1/public-holidays/defaults";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_gieoNgayNghiLeVao;
  phanHoi: {
    200: identity_gieoLichRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/public-holidays/{id} — Sửa một ngày nghỉ lễ — KHÔNG hồi tố lên hạn đã phát ra cho hồ sơ cũ */
export type identity_patch_public_holidays_by_id = {
  duongDan: "/api/v1/public-holidays/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_suaNgayNghiLeVao;
  phanHoi: {
    200: identity_ngayNghiLeRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/public-holidays/{id} — Xoá mềm một ngày nghỉ lễ, kèm lý do bắt buộc — dòng ở lại, ngày đó không khai lại được */
export type identity_delete_public_holidays_by_id = {
  duongDan: "/api/v1/public-holidays/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_xoaLichVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
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

/** POST /api/v1/residential-unit-types — Thêm một loại đơn vị dân cư của riêng xã vào danh mục */
export type identity_post_residential_unit_types = {
  duongDan: "/api/v1/residential-unit-types";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_themDanhMucVao;
  phanHoi: {
    201: identity_loaiDonViDanCuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/residential-unit-types/{id} — Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một loại đơn vị dân cư */
export type identity_patch_residential_unit_types_by_id = {
  duongDan: "/api/v1/residential-unit-types/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_suaDanhMucVao;
  phanHoi: {
    200: identity_loaiDonViDanCuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/residential-unit-types/{id} — Xoá mềm một loại đơn vị dân cư do xã tự thêm, kèm lý do bắt buộc */
export type identity_delete_residential_unit_types_by_id = {
  duongDan: "/api/v1/residential-unit-types/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_xoaDanhMucVao;
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

/** PUT /api/v1/roles/{id}/permissions — Lưu phân quyền của MỘT vai trò (một cột ma trận) — gửi toàn bộ danh sách quyền */
export type identity_put_roles_by_id_permissions = {
  duongDan: "/api/v1/roles/{id}/permissions";
  phuongThuc: "PUT";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_luuPhanQuyenVao;
  phanHoi: {
    200: identity_cotPhanQuyenRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
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

/** GET /api/v1/sla — Bảng thời hạn xử lý của xã — số GIỜ LÀM VIỆC cho từng loại việc và lĩnh vực */
export type identity_get_sla = {
  duongDan: "/api/v1/sla";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_danhSachSLARa;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/sla/defaults — Gieo bộ thời hạn mặc định cho xã chưa cấu hình — KHÔNG ghi đè con số xã đã sửa */
export type identity_post_sla_defaults = {
  duongDan: "/api/v1/sla/defaults";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_gieoSLARa;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/sla/{id} — Sửa năm con số của một dòng thời hạn — KHÔNG hồi tố lên hồ sơ đã tiếp nhận */
export type identity_patch_sla_by_id = {
  duongDan: "/api/v1/sla/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_suaSLAVao;
  phanHoi: {
    200: identity_dongSLARa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
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
    "published"?: string;
    "unit"?: string;
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

/** GET /api/v1/staff-directory — Danh bạ chọn người nhận việc của xã — mã cán bộ, họ tên, chức vụ, bộ phận; chỉ người có tài khoản đang hoạt động, không số điện thoại, không email */
export type identity_get_staff_directory = {
  duongDan: "/api/v1/staff-directory";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "unit"?: string;
  };
  than: never;
  phanHoi: {
    200: identity_danhBaChonNguoiRa;
    400: httpx_Error;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** PUT /api/v1/staff/current/password — Cán bộ tự đổi mật khẩu của chính mình — bắt buộc ở lần đăng nhập đầu, và là đường DUY NHẤT gỡ cờ bắt đổi */
export type identity_put_staff_current_password = {
  duongDan: "/api/v1/staff/current/password";
  phuongThuc: "PUT";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_doiMatKhauVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/staff/searches — Tìm cán bộ trong danh bạ của xã theo họ tên, chức vụ hoặc số điện thoại — từ khoá đi trong THÂN, không lên URL */
export type identity_post_staff_searches = {
  duongDan: "/api/v1/staff/searches";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_timCanBoVao;
  phanHoi: {
    200: page_Result_identity_canBoTomTat;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
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

/** DELETE /api/v1/staff/{id} — Xoá mềm một dòng danh bạ NHẬP TRÙNG, kèm lý do bắt buộc — từ chối dòng đang có tài khoản đăng nhập */
export type identity_delete_staff_by_id = {
  duongDan: "/api/v1/staff/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_xoaCanBoVao;
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

/** POST /api/v1/staff/{id}/account — Cấp tài khoản đăng nhập cho một cán bộ đang có trong danh bạ — hệ thống sinh mật khẩu tạm, trả về ĐÚNG MỘT LẦN */
export type identity_post_staff_by_id_account = {
  duongDan: "/api/v1/staff/{id}/account";
  phuongThuc: "POST";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    201: identity_capTaiKhoanRa;
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

/** PUT /api/v1/staff/{id}/password — Quản trị viên xã đặt lại mật khẩu hộ một cán bộ — sinh mật khẩu tạm mới, trả về ĐÚNG MỘT LẦN */
export type identity_put_staff_by_id_password = {
  duongDan: "/api/v1/staff/{id}/password";
  phuongThuc: "PUT";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_capTaiKhoanRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PUT /api/v1/staff/{id}/publication — Công khai / thôi công khai một cán bộ lên danh bạ Zalo Mini App — bắt buộc xác nhận đã được người đó đồng ý (#12) */
export type identity_put_staff_by_id_publication = {
  duongDan: "/api/v1/staff/{id}/publication";
  phuongThuc: "PUT";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_datCongKhaiVao;
  phanHoi: {
    200: identity_canBoTomTat;
    400: httpx_Error;
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

/** POST /api/v1/swap-working-days — Thêm một ca làm bù — ngày xã CÓ làm việc dù lịch tuần nói không, kèm giờ làm của chính ngày đó */
export type identity_post_swap_working_days = {
  duongDan: "/api/v1/swap-working-days";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_themNgayLamBuVao;
  phanHoi: {
    201: identity_caLamBuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/swap-working-days/{id} — Sửa một ca làm bù — KHÔNG hồi tố lên hạn đã phát ra cho hồ sơ cũ */
export type identity_patch_swap_working_days_by_id = {
  duongDan: "/api/v1/swap-working-days/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_suaNgayLamBuVao;
  phanHoi: {
    200: identity_caLamBuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/swap-working-days/{id} — Xoá mềm một ca làm bù, kèm lý do bắt buộc — dòng ở lại vì hạn đã phát ra đếm qua nó */
export type identity_delete_swap_working_days_by_id = {
  duongDan: "/api/v1/swap-working-days/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_xoaLichVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
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

/** POST /api/v1/task-blocs — Thêm một khối nhiệm vụ của riêng xã vào danh mục */
export type identity_post_task_blocs = {
  duongDan: "/api/v1/task-blocs";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_themDanhMucVao;
  phanHoi: {
    201: identity_khoiNhiemVuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/task-blocs/{id} — Sửa nhãn, thứ tự, trạng thái dùng hoặc đặt mặc định cho một khối nhiệm vụ */
export type identity_patch_task_blocs_by_id = {
  duongDan: "/api/v1/task-blocs/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_suaDanhMucVao;
  phanHoi: {
    200: identity_khoiNhiemVuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/task-blocs/{id} — Xoá mềm một khối nhiệm vụ do xã tự thêm, kèm lý do bắt buộc */
export type identity_delete_task_blocs_by_id = {
  duongDan: "/api/v1/task-blocs/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_xoaDanhMucVao;
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

/** GET /api/v1/task-statuses — Bảy trạng thái nhiệm vụ với nhãn và thứ tự của xã (mặc định nếu xã chưa sửa) — cột Kanban, bộ lọc, màn hình cấu hình */
export type petitions_get_task_statuses = {
  duongDan: "/api/v1/task-statuses";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: petitions_danhSachTrangThaiNhiemVuRa;
    401: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/task-statuses/{code} — Sửa nhãn và/hoặc thứ tự hiển thị của một trạng thái nhiệm vụ trong xã (không thêm, xoá hay tắt mã) */
export type petitions_patch_task_statuses_by_code = {
  duongDan: "/api/v1/task-statuses/{code}";
  phuongThuc: "PATCH";
  thamSo: {
    "code": string;
  };
  truyVan: {
  };
  than: petitions_suaTrangThaiNhiemVuVao;
  phanHoi: {
    200: petitions_trangThaiNhiemVuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
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

/** GET /api/v1/tasks — Danh sách nhiệm vụ của xã — phân trang theo con trỏ, lọc theo trạng thái · loại · khối · ưu tiên · bộ phận · người thực hiện · nguồn giao · trễ hạn */
export type petitions_get_tasks = {
  duongDan: "/api/v1/tasks";
  phuongThuc: "GET";
  thamSo: {
  };
  truyVan: {
    "limit"?: number;
    "cursor"?: string;
    "sort"?: "created_at" | "code";
    "order"?: "asc" | "desc";
    "assignee"?: string;
    "bloc"?: string;
    "late"?: string;
    "priority"?: string;
    "q"?: string;
    "scope"?: string;
    "soon"?: string;
    "source"?: string;
    "status"?: string;
    "type"?: string;
    "unit"?: string;
  };
  than: never;
  phanHoi: {
    200: page_Result_petitions_nhiemVuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/tasks — Giao việc mới — tạo một nhiệm vụ, tự sinh mã theo dãy NV của xã hoặc nhận mã tự nhập */
export type petitions_post_tasks = {
  duongDan: "/api/v1/tasks";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: petitions_taoNhiemVuVao;
  phanHoi: {
    201: petitions_nhiemVuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** GET /api/v1/tasks/{ma} — Một nhiệm vụ, tra theo mã nhiệm vụ của xã (NV19) */
export type petitions_get_tasks_by_ma = {
  duongDan: "/api/v1/tasks/{ma}";
  phuongThuc: "GET";
  thamSo: {
    "ma": string;
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: petitions_nhiemVuRa;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/tasks/{ma} — Sửa thông tin mô tả của một nhiệm vụ — không đụng tới hạn, trạng thái hay phân công */
export type petitions_patch_tasks_by_ma = {
  duongDan: "/api/v1/tasks/{ma}";
  phuongThuc: "PATCH";
  thamSo: {
    "ma": string;
  };
  truyVan: {
  };
  than: petitions_suaNhiemVuVao;
  phanHoi: {
    200: petitions_nhiemVuRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/tasks/{ma} — Xoá mềm một nhiệm vụ khỏi sổ, kèm lý do bắt buộc — từ chối khi còn việc con chưa xoá */
export type petitions_delete_tasks_by_ma = {
  duongDan: "/api/v1/tasks/{ma}";
  phuongThuc: "DELETE";
  thamSo: {
    "ma": string;
  };
  truyVan: {
  };
  than: petitions_xoaNhiemVuVao;
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

/** POST /api/v1/tasks/{ma}/extensions — Gửi đề nghị lùi hạn cho một nhiệm vụ — hạn mới phải muộn hơn hạn đang có, kèm lý do bắt buộc */
export type petitions_post_tasks_by_ma_extensions = {
  duongDan: "/api/v1/tasks/{ma}/extensions";
  phuongThuc: "POST";
  thamSo: {
    "ma": string;
  };
  truyVan: {
  };
  than: petitions_deNghiLuiHanVao;
  phanHoi: {
    201: petitions_deNghiLuiHanRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/tasks/{ma}/extensions/{deNghiID}/decision — Lãnh đạo giao việc duyệt hoặc từ chối một đề nghị lùi hạn — duyệt thì đổi hạn xử lý, hạn ban đầu giữ nguyên */
export type petitions_post_tasks_by_ma_extensions_by_deNghiID_decision = {
  duongDan: "/api/v1/tasks/{ma}/extensions/{deNghiID}/decision";
  phuongThuc: "POST";
  thamSo: {
    "ma": string;
    "deNghiID": string;
  };
  truyVan: {
  };
  than: petitions_quyetDinhLuiHanVao;
  phanHoi: {
    200: petitions_deNghiLuiHanRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/tasks/{ma}/status — Chuyển trạng thái một nhiệm vụ theo vòng đời §6, kèm ghi nhật ký — hoàn thành cần quyền duyệt và mọi việc con đã xong */
export type petitions_post_tasks_by_ma_status = {
  duongDan: "/api/v1/tasks/{ma}/status";
  phuongThuc: "POST";
  thamSo: {
    "ma": string;
  };
  truyVan: {
  };
  than: petitions_doiTrangThaiVao;
  phanHoi: {
    200: petitions_nhiemVuRa;
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

/** POST /api/v1/working-hours — Thêm một ca làm việc vào tuần của xã — nghỉ trưa là khoảng hở giữa hai ca, không phải một cờ */
export type identity_post_working_hours = {
  duongDan: "/api/v1/working-hours";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: identity_themCaLamViecVao;
  phanHoi: {
    201: identity_caLamViecRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** POST /api/v1/working-hours/defaults — Gieo tuần làm việc mặc định cho xã chưa cấu hình — KHÔNG ghi đè giờ xã đã sửa */
export type identity_post_working_hours_defaults = {
  duongDan: "/api/v1/working-hours/defaults";
  phuongThuc: "POST";
  thamSo: {
  };
  truyVan: {
  };
  than: never;
  phanHoi: {
    200: identity_gieoLichRa;
    401: httpx_Error;
    403: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** PATCH /api/v1/working-hours/{id} — Sửa một ca làm việc — KHÔNG hồi tố lên hạn đã phát ra cho hồ sơ cũ */
export type identity_patch_working_hours_by_id = {
  duongDan: "/api/v1/working-hours/{id}";
  phuongThuc: "PATCH";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_suaCaLamViecVao;
  phanHoi: {
    200: identity_caLamViecRa;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    409: httpx_Error;
    500: httpx_Error;
  };
};

/** DELETE /api/v1/working-hours/{id} — Xoá mềm một ca làm việc, kèm lý do bắt buộc — dòng ở lại, giờ mở ca không cấp lại được */
export type identity_delete_working_hours_by_id = {
  duongDan: "/api/v1/working-hours/{id}";
  phuongThuc: "DELETE";
  thamSo: {
    "id": string;
  };
  truyVan: {
  };
  than: identity_xoaLichVao;
  phanHoi: {
    204: void;
    400: httpx_Error;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};
