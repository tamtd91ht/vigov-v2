// TỆP SINH RA. ĐỪNG SỬA TAY — sửa tay mất sạch ở lần sinh sau (luật 9, bất biến 8).
//
// Nguồn: kb/20-contracts/openapi.json (do tools/apidoc sinh từ khai báo route trong
// */internal/). Sinh lại: npm run gen:api — trong web-admin.
//
// Hợp đồng: ViGov — REST API cho web quản trị v1

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

export type identity_danhSachVaiTroRa = {
  "items": Array<identity_vaiTroMucRa>;
};

export type identity_maTranQuyenRa = {
  "groups": Array<identity_nhomQuyenRa>;
  "roles": Array<identity_vaiTroCotRa>;
  "grants": Array<identity_capQuyenRa>;
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

export type identity_thanDangNhap = {
  "email": string;
  "password": string;
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

export type page_Result_identity_canBoTomTat = {
  "items": Array<identity_canBoTomTat>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
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
