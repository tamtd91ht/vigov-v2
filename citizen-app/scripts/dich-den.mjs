/**
 * CHỌN ĐÍCH CỦA MỘT LẦN ĐẨY: tên miền → App ID → token nào được dùng (ADR 0047).
 *
 * Tên miền CHỈ chọn đích. Bundle là một, không biến thể (27/09/2026, xem đầu `vite.config.ts`).
 *
 * Mọi hàm ở đây THUẦN — nhận dữ liệu, trả dữ liệu hoặc ném lỗi, không đọc đĩa, không đọc môi
 * trường — để `dich-den.test.mjs` kiểm được từng nhánh mà không chạy `zmp-cli`. `deploy.mjs` là
 * nơi duy nhất nối chúng với `process.argv` / `process.env`.
 *
 * ⚠ APP ID KHÔNG CHỌN ĐƯỢC ĐÍCH — TOKEN MỚI CHỌN. ĐÃ ĐO, KHÔNG PHẢI SUY ĐOÁN (27/09/2026, zmp-cli 4.0.3):
 *
 *   - `deploy` chỉ gọi `getEnv("ZMP_TOKEN")`; nó KHÔNG đọc `APP_ID`, và yêu cầu đầu tiên
 *     (`app/request-upload`) chỉ mang `Authorization`, không có App ID ở đâu cả. Đo bằng một tệp
 *     nạp trước chặn mọi yêu cầu ra mạng rồi in ra thứ lẽ ra được gửi.
 *   - `login` nhận một JWT, `decode` nó, rồi ghi `APP_ID = jwt.appId` và `ZMP_TOKEN = jwt`.
 *     Tức `APP_ID` trong `.env` chỉ là BẢN SAO của claim trong token.
 *   - `getEnv(k)` trả `process.env[k]` nếu có giá trị, chỉ khi rỗng mới đọc `<cwd>/.env`.
 *
 *   Hệ quả: đặt `APP_ID` cho zmp là vô tác dụng; token của app X thì đẩy lên app X, bất kể ta
 *   định đẩy đi đâu. Nên phép kiểm có nghĩa duy nhất là **claim `appId` của token == App ID đích**,
 *   và đó là việc của `kiemToken`. Nó GIẢI MÃ payload để đọc claim, KHÔNG xác minh chữ ký — nó
 *   không cần tin token, chỉ cần biết zmp sẽ đẩy token ấy đi đâu.
 */
/** Tên miền dạng máy chủ: chữ thường, số, gạch nối; ít nhất hai nhãn; không scheme/cổng/đường dẫn. */
const NHAN = /^(?!-)[a-z0-9-]{1,63}(?<!-)$/;

/** Placeholder trong tệp ánh xạ: `<…>`. `--thu` in được, lần chạy thật thì không. */
export function laPlaceholder(app_id) {
  return /^<[^>]*>$/.test(app_id);
}

/** Ném lỗi nếu `ten` không phải một tên miền trần. Trả lại chính nó. */
export function kiemTenMien(ten) {
  const vi_sao = (() => {
    if (typeof ten !== "string" || ten === "") return "rỗng";
    if (ten.includes("://")) return "có scheme — bỏ `https://`";
    if (/[/?#]/.test(ten)) return "có đường dẫn — chỉ giữ phần tên máy";
    if (ten.includes(":")) return "có cổng — bỏ `:<cổng>`";
    if (ten !== ten.toLowerCase()) return "có chữ hoa — viết thường toàn bộ";
    if (ten.length > 253) return "dài quá 253 ký tự";
    const nhan = ten.split(".");
    if (nhan.length < 2) return "cần ít nhất hai phần cách nhau bằng dấu chấm";
    if (!nhan.every((n) => NHAN.test(n))) return "có phần rỗng, ký tự lạ, hoặc gạch nối ở đầu/cuối";
    return null;
  })();
  if (vi_sao !== null) throw new Error(`Tên miền "${ten}" không hợp lệ: ${vi_sao}.`);
  return ten;
}

/**
 * Kiểm cả tệp ánh xạ. Ném lỗi nếu một khoá không phải tên miền trần, một App ID rỗng / có khoảng
 * trắng, hoặc App ID của app chung trùng App ID của một xã (đẩy app chung đè lên app của xã ấy).
 */
export function kiemBangAnhXa(bang, app_chung) {
  if (bang === null || typeof bang !== "object" || Array.isArray(bang)) {
    throw new Error("Tệp ánh xạ tên miền → App ID phải là một đối tượng.");
  }
  for (const [ten, app_id] of Object.entries(bang)) {
    kiemTenMien(ten);
    if (typeof app_id !== "string" || app_id.trim() === "" || /\s/.test(app_id)) {
      throw new Error(`App ID của "${ten}" rỗng hoặc có khoảng trắng.`);
    }
  }
  if (app_chung !== null) {
    if (typeof app_chung !== "string" || app_chung.trim() === "" || /\s/.test(app_chung)) {
      throw new Error("APP_ID_APP_CHUNG phải là null hoặc một App ID không rỗng.");
    }
    if (Object.values(bang).includes(app_chung)) {
      throw new Error("APP_ID_APP_CHUNG trùng App ID của một xã: đẩy app chung sẽ đè lên app của xã ấy.");
    }
  }
}

/**
 * Đọc cờ dòng lệnh. CỜ LẠ THÌ DỪNG: `--domian=xa-a…` gõ nhầm mà bị bỏ qua là một lần đẩy lên
 * app chung trong khi người gõ tưởng đang đẩy app của xã.
 *
 * `--domain=` (rỗng) CŨNG DỪNG, không hiểu thành "app chung": một tham số Jenkins để trống vì quên
 * và một tham số để trống vì muốn app chung trông giống hệt nhau. Muốn app chung thì bỏ hẳn cờ.
 */
export function docCo(argv) {
  const co = { ten_mien: null, phat_hanh: false, chi_thu: false, vao_thang: false };
  for (const c of argv) {
    if (c === "--phat-hanh") co.phat_hanh = true;
    else if (c === "--vao-thang") co.vao_thang = true;
    else if (c === "--thu") co.chi_thu = true;
    else if (c.startsWith("--domain=")) {
      const gia_tri = c.slice("--domain=".length);
      if (gia_tri === "") {
        throw new Error("--domain= để trống. Muốn đẩy lên APP CHUNG thì bỏ hẳn cờ --domain.");
      }
      co.ten_mien = kiemTenMien(gia_tri);
    } else if (c.startsWith("--bien-the")) {
      // Cờ đã bỏ cùng hai biến thể (27/09/2026). Nói rõ vì sao, thay vì chỉ "cờ không có": người gõ
      // nó đang tin rằng họ chọn được nội dung bản dựng, và điều đó không còn đúng.
      throw new Error(
        "--bien-the đã bỏ: bản dựng không còn biến thể nào, app chung và app riêng của xã chạy cùng " +
          "một bundle. Bỏ cờ này; muốn đẩy app riêng của xã thì dùng --domain=<tên-miền>.",
      );
    } else throw new Error(`Cờ "${c}" không có. Chỉ nhận: --domain=<tên-miền> · --vao-thang · --phat-hanh · --thu`);
  }
  // `--vao-thang` nung TÊN MIỀN CỦA `--domain` vào bundle. Không có `--domain` thì không có tên miền
  // nào để nung — và app chung nung một xã là app chung mở vào xã ấy cho mọi người.
  if (co.vao_thang && co.ten_mien === null) {
    throw new Error("--vao-thang cần --domain=<tên-miền>: nó nung đúng tên miền ấy vào app riêng của xã.");
  }
  return co;
}

/**
 * Chọn đích. Tên miền không có trong bảng thì DỪNG — không bao giờ rơi về app chung. Thông báo
 * không liệt kê bảng: người gõ sai cần biết SỬA Ở ĐÂU, không cần xem danh sách các xã.
 */
export function chonDich(ten_mien, bang, app_chung) {
  if (ten_mien === null) return { loai: "app-chung", ten_mien: null, app_id: app_chung };
  if (!Object.hasOwn(bang, ten_mien)) {
    throw new Error(
      `Tên miền "${ten_mien}" chưa có trong scripts/ung-dung-theo-ten-mien.mjs.\n` +
        "  Thêm một dòng `<tên-miền>: <App ID>` vào tệp ấy — VÀ dòng MiniApp tương ứng ở " +
        "service-platform, vì bảng ấy mới là nơi máy chủ đọc xã của một App ID (ADR 0047).",
    );
  }
  return { loai: "app-rieng", ten_mien, app_id: bang[ten_mien] };
}

/** Claim `appId` trong payload JWT của `ZMP_TOKEN`, hoặc `null` nếu không đọc được. */
export function appIdTrongToken(token) {
  if (typeof token !== "string") return null;
  const phan = token.split(".");
  if (phan.length !== 3) return null;
  try {
    const payload = JSON.parse(Buffer.from(phan[1], "base64url").toString("utf8"));
    const app_id = payload?.appId;
    return typeof app_id === "string" || typeof app_id === "number" ? String(app_id) : null;
  } catch {
    return null;
  }
}

/**
 * Token này có đẩy đúng lên đích không. Trả `{ ok, ly_do }`; không bao giờ đưa token vào `ly_do`.
 *
 * | Đích | Token trong môi trường | Kết quả |
 * |---|---|---|
 * | app riêng | không có | TỪ CHỐI — `.env` của máy là token của app nào đó người ấy đăng nhập lần cuối |
 * | app riêng | claim ≠ App ID đích | TỪ CHỐI |
 * | app riêng | App ID đích là placeholder | TỪ CHỐI |
 * | app chung | không có | QUA — như trước, zmp đọc `citizen-app/.env` (script không đọc tệp ấy) |
 * | app chung | claim là App ID của một xã trong bảng | TỪ CHỐI — sẽ đè lên app của xã |
 * | app chung | `APP_ID_APP_CHUNG` đã khai và claim khác nó | TỪ CHỐI |
 */
export function kiemToken(dich, token, bang) {
  const co_token = typeof token === "string" && token !== "";
  const claim = co_token ? appIdTrongToken(token) : null;

  if (dich.loai === "app-rieng") {
    if (laPlaceholder(dich.app_id)) {
      return { ok: false, ly_do: `App ID của "${dich.ten_mien}" còn là placeholder ${dich.app_id}.` };
    }
    if (!co_token) {
      return {
        ok: false,
        ly_do:
          "ZMP_TOKEN chưa có trong môi trường. Đẩy app riêng của xã KHÔNG dùng `.env` của máy — " +
          "token trong ấy thuộc app nào người ấy đăng nhập lần cuối. Đặt ZMP_TOKEN của đúng app này.",
      };
    }
    if (claim === null) return { ok: false, ly_do: "ZMP_TOKEN không đọc được claim appId — không biết nó đẩy đi đâu." };
    if (claim !== dich.app_id) {
      return { ok: false, ly_do: `ZMP_TOKEN là token của App ID ${claim}, không phải ${dich.app_id}.` };
    }
    return { ok: true, ly_do: `ZMP_TOKEN (môi trường) thuộc đúng App ID ${claim}.` };
  }

  if (!co_token) {
    return { ok: true, ly_do: "ZMP_TOKEN không có trong môi trường: zmp-cli đọc citizen-app/.env như trước." };
  }
  if (claim === null) return { ok: false, ly_do: "ZMP_TOKEN không đọc được claim appId — không biết nó đẩy đi đâu." };
  if (Object.values(bang).includes(claim)) {
    return { ok: false, ly_do: `ZMP_TOKEN là token của App ID ${claim} — app RIÊNG của một xã, không phải app chung.` };
  }
  if (dich.app_id !== null && claim !== dich.app_id) {
    return { ok: false, ly_do: `ZMP_TOKEN là token của App ID ${claim}, không phải app chung ${dich.app_id}.` };
  }
  return { ok: true, ly_do: `ZMP_TOKEN (môi trường) thuộc App ID ${claim}.` };
}

/**
 * `ZMP_TOKEN` in the `.env` that `zmp-cli login` just wrote into a throwaway directory, or `null`.
 * Tolerates quoting and CRLF because the writer's exact format is inside obfuscated vendor code.
 */
export function tokenTrongTepEnv(noi_dung) {
  const dong = /^\s*ZMP_TOKEN\s*=\s*(.*?)\s*$/m.exec(String(noi_dung ?? "").replace(/\r/g, ""));
  if (!dong) return null;
  const gia_tri = dong[1].replace(/^(["'])(.*)\1$/, "$2");
  return gia_tri === "" ? null : gia_tri;
}

/** Nhãn phiên bản trên console Zalo: đích · commit · lúc · dirty. */
export function nhanPhienBan({ dich, sha, luc, dirty }) {
  const noi =
    dich.loai === "app-rieng"
      ? `${dich.ten_mien} · app ${dich.app_id}`
      : dich.app_id === null
        ? "app-chung"
        : `app-chung · app ${dich.app_id}`;
  return `${noi} · ${sha} · ${luc}${dirty ? " · dirty" : ""}`;
}
