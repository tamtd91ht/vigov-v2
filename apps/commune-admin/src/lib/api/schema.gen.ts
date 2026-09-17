// TỆP SINH RA. ĐỪNG SỬA TAY — sửa tay mất sạch ở lần sinh sau (luật 9, bất biến 8).
//
// Nguồn: kb/20-contracts/openapi.json (do tools/apidoc sinh từ khai báo route trong
// services/*/internal/). Sinh lại: npm run gen:api — trong apps/commune-admin.
//
// Hợp đồng: ViGov — REST API cho web quản trị v1

export type httpx_Error = {
  "code": string;
  /** safe for a citizen to read: never personal data, never internals */
  "message": string;
  "trace_id": string;
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

export type identity_phanHoiDangNhap = {
  "sid": string;
  "expires_at": string;
  "staff": identity_canBoGon;
};

export type identity_thanDangNhap = {
  "email": string;
  "password": string;
};

export type identity_trangCanBo = {
  "items": Array<identity_canBoTomTat>;
  /** empty when has_more is false */
  "next_cursor": string;
  "has_more": boolean;
};

/** POST /api/v1/sessions — Đăng nhập bằng email và mật khẩu, mở một phiên làm việc */
export type identity_post_sessions = {
  duongDan: "/api/v1/sessions";
  phuongThuc: "POST";
  thamSo: {
  };
  than: identity_thanDangNhap;
  phanHoi: {
    201: identity_phanHoiDangNhap;
    400: httpx_Error;
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
  than: never;
  phanHoi: {
    200: identity_trangCanBo;
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
  than: never;
  phanHoi: {
    200: identity_canBoTomTat;
    401: httpx_Error;
    403: httpx_Error;
    404: httpx_Error;
    500: httpx_Error;
  };
};
