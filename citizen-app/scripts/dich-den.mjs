/**
 * CHỌN ĐÍCH CỦA MỘT LẦN ĐẨY: app nào → App ID → token nào được dùng (ADR 0047; chủ dự án 06/10/2026).
 *
 * HAI ĐÍCH, CHỌN TƯỜNG MINH — không bao giờ mặc định:
 *
 * | Cờ | Đích | Tên miền vào bundle |
 * |---|---|---|
 * | `--app=vihat` | App ViHAT — app chung | KHÔNG BAO GIỜ (ADR 0044: một xã trong bundle chung đưa MỌI người, kể cả người duyệt của Zalo, vào xã ấy) |
 * | `--domain=<tên-miền>` | app riêng của xã ấy | LUÔN — app mở thẳng vào xã (ADR 0047 §6; trước 06/10/2026 là cờ `--vao-thang`, nay đã bỏ) |
 * | không cờ nào | HỎI "vihat hay tên miền xã" khi có người ngồi trước cửa sổ lệnh; không thì TỪ CHỐI | — |
 *
 * VÌ SAO KHÔNG CÒN "KHÔNG CỜ = APP CHUNG": trước 06/10/2026, không cờ thì zmp-cli đọc `ZMP_TOKEN` trong
 * `citizen-app/.env` — tức đẩy lên app nào người ấy đăng nhập lần cuối trên máy ấy. Đích được chọn
 * ngầm bởi một tệp không ai nhìn. Nay script KHÔNG BAO GIỜ để zmp rơi về tệp ấy, với bất kỳ đích nào.
 *
 * APP ID CỦA ĐÍCH ĐỌC TỪ PLATFORM (chủ dự án 06/10/2026, phương án A): bảng `mini_app` của
 * `service-platform` là nguồn sự thật DUY NHẤT về App ID — cũng là bảng máy chủ đọc để biết một App ID
 * phục vụ xã nào (ADR 0044, 0045). Trước ngày ấy App ID còn được chép tay vào
 * `scripts/ung-dung-theo-ten-mien.mjs`; hai nơi cho một cặp (xã, App ID) là hai nơi lệch nhau (ADR 0047
 * CÒN MỞ #3), và khi lệch thì máy chủ thắng — app vừa đẩy mở ra bị từ chối. Nay tệp ấy không còn App ID
 * nào; `GET /api/v1/mini-app-ids` trả App ID, và script chỉ đối chiếu (`appIdFromPlatform`).
 *
 * KHÔNG CÓ MENU CÁC XÃ: tuyến ấy cố ý không có lệnh "liệt kê" — nó trả lời đúng một câu hỏi cho đúng
 * một đích. Nên khi không cờ, script hỏi người gõ tên đích, không đưa danh sách.
 *
 * QR mở App ViHAT vào một xã (`zalo.me/s/<App ID app chung>/?d=<host>&src=qr`) KHÔNG phải việc của
 * bước đẩy: nó làm ở platform-admin (chi tiết xã → "Mở bằng app ViHAT").
 *
 * Mọi hàm ở đây THUẦN — nhận dữ liệu, trả dữ liệu hoặc ném lỗi, không đọc đĩa, không đọc môi
 * trường, không gọi mạng, không hỏi ai — để `dich-den.test.mjs` kiểm được từng nhánh mà không chạy
 * `zmp-cli`. Lời gọi mạng là `mini-app-id-lookup.mjs`; `deploy.mjs` là nơi duy nhất nối chúng với
 * `process.argv` / `process.env` / bàn phím / tệp / mạng.
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

/** Tên đọc được của app chung, trong câu hỏi, câu từ chối và kế hoạch. */
export const SHARED_APP_NAME = "App ViHAT";

/** Tuyến của platform trả App ID của một đích (hợp đồng cố định 06/10/2026). */
export const MINI_APP_ID_PATH = "/api/v1/mini-app-ids";

/** Nơi sửa khi platform không có App ID — người chạy lệnh không sửa được gì ở kho này. */
const WHERE_TO_FIX = {
  "app-chung": "platform-admin → khai App ViHAT (app chung) với App ID của nó",
  "app-rieng": (ten_mien) => `platform-admin → chi tiết xã ${ten_mien} → gắn App ID app riêng của xã`,
};

/** Mọi cờ còn nhận, nói ra mỗi khi một cờ bị từ chối. */
const VALID_FLAGS = "--app=vihat · --domain=<tên-miền> · --app-id=<chữ số> · --phat-hanh · --thu";

/** Tên miền dạng máy chủ: chữ thường, số, gạch nối; ít nhất hai nhãn; không scheme/cổng/đường dẫn. */
const NHAN = /^(?!-)[a-z0-9-]{1,63}(?<!-)$/;

/** App ID thật: chỉ chữ số. */
function isAppId(value) {
  return typeof value === "string" && /^\d+$/.test(value);
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
 * Địa chỉ platform đã kiểm: một ORIGIN `https://<host>[:<cổng>]`, không đường dẫn, không truy vấn,
 * không tên/mật khẩu. Trả origin (không dấu `/` cuối). Kiểm hằng `VIGOV_PLATFORM_API_HOST` của
 * `deploy-config.mjs` lúc khởi động và trong test: một lượt sửa hằng ấy thành `http://…` hay thêm đường
 * dẫn phải đỏ trước khi một App ID được đọc từ đó.
 *
 * CHỈ HTTPS: câu trả lời quyết định bản dựng đi lên app nào trên Zalo.
 */
export function checkPlatformApiHost(value) {
  const how = "  Sửa hằng VIGOV_PLATFORM_API_HOST trong citizen-app/scripts/deploy-config.mjs (origin https của service-platform).";
  const v = typeof value === "string" ? value.trim() : "";
  if (v === "") throw new Error(`Không đẩy: VIGOV_PLATFORM_API_HOST rỗng.\n${how}`);
  let url;
  try {
    url = new URL(v);
  } catch {
    throw new Error(`Không đẩy: VIGOV_PLATFORM_API_HOST="${v}" không phải một địa chỉ.\n${how}`);
  }
  const vi_sao =
    url.protocol !== "https:"
      ? "phải là https://"
      : url.username !== "" || url.password !== ""
        ? "không được mang tên/mật khẩu"
        : (url.pathname !== "/" && url.pathname !== "") || url.search !== "" || url.hash !== "" || /[?#]/.test(v)
          ? "chỉ là origin — bỏ đường dẫn, `?…`, `#…`"
          : null;
  if (vi_sao !== null) throw new Error(`Không đẩy: VIGOV_PLATFORM_API_HOST="${v}" ${vi_sao}.\n${how}`);
  return url.origin;
}

/**
 * Đọc cờ dòng lệnh. CỜ LẠ THÌ DỪNG: `--domian=xa-a…` gõ nhầm mà bị bỏ qua là một lần đẩy lên
 * một đích người gõ không chọn.
 *
 * `--domain=` (rỗng) CŨNG DỪNG: một tham số để trống vì quên và một tham số để trống vì muốn app chung
 * trông giống hệt nhau. App chung có cờ riêng của nó (`--app=vihat`).
 *
 * Không cờ đích nào thì trả `shared_app: false, ten_mien: null` — CHƯA CÓ ĐÍCH, không phải "app chung".
 * `deploy.mjs` hỏi, hoặc từ chối khi không có người để hỏi.
 */
export function docCo(argv) {
  const co = { ten_mien: null, shared_app: false, app_id: null, phat_hanh: false, chi_thu: false };
  for (const c of argv) {
    if (c === "--phat-hanh") co.phat_hanh = true;
    else if (c === "--thu") co.chi_thu = true;
    else if (c === "--vao-thang" || c.startsWith("--vao-thang=")) {
      // Bỏ 06/10/2026. Nói rõ điều gì thay nó, thay vì chỉ "cờ không có": người gõ nó đang tin rằng
      // KHÔNG có nó thì app riêng không mở thẳng vào xã — điều đó không còn đúng.
      throw new Error(
        "--vao-thang đã bỏ (06/10/2026): --domain=<tên-miền> nay LUÔN nung tên miền ấy vào app riêng của " +
          "xã, và app mở thẳng vào xã. Bỏ cờ này, giữ --domain.",
      );
    } else if (c.startsWith("--app-id=")) {
      const gia_tri = c.slice("--app-id=".length);
      if (!isAppId(gia_tri)) {
        throw new Error(`--app-id="${gia_tri}" không phải App ID: chỉ gồm chữ số.`);
      }
      if (co.app_id !== null && co.app_id !== gia_tri) throw new Error("--app-id có hai giá trị khác nhau.");
      co.app_id = gia_tri;
    } else if (c.startsWith("--app=")) {
      const gia_tri = c.slice("--app=".length);
      if (gia_tri !== "vihat") {
        throw new Error(
          `--app="${gia_tri}" không có. Chỉ nhận --app=vihat (App ViHAT); app riêng của xã chọn bằng ` +
            "--domain=<tên-miền>.",
        );
      }
      co.shared_app = true;
    } else if (c.startsWith("--domain=")) {
      const gia_tri = c.slice("--domain=".length);
      if (gia_tri === "") {
        throw new Error(
          "--domain= để trống. Ghi tên miền của xã để đẩy app riêng của xã ấy; muốn App ViHAT thì dùng --app=vihat.",
        );
      }
      kiemTenMien(gia_tri);
      if (co.ten_mien !== null && co.ten_mien !== gia_tri) throw new Error("--domain có hai giá trị khác nhau.");
      co.ten_mien = gia_tri;
    } else if (c.startsWith("--bien-the")) {
      // Cờ đã bỏ cùng hai biến thể (27/09/2026). Nói rõ vì sao, thay vì chỉ "cờ không có": người gõ
      // nó đang tin rằng họ chọn được nội dung bản dựng, và điều đó không còn đúng.
      throw new Error(
        "--bien-the đã bỏ: bản dựng không còn biến thể nào. Bỏ cờ này; chọn đích bằng --app=vihat " +
          "(App ViHAT) hoặc --domain=<tên-miền> (app riêng của xã, mở thẳng vào xã).",
      );
    } else {
      throw new Error(`Cờ "${c}" không có. Chỉ nhận: ${VALID_FLAGS}`);
    }
  }
  if (co.shared_app && co.ten_mien !== null) {
    throw new Error(
      "--app=vihat không đi cùng --domain: App ViHAT là bundle chung, không nung xã nào (ADR 0044).\n" +
        "  QR mở App ViHAT vào một xã làm ở platform-admin (chi tiết xã → \"Mở bằng app ViHAT\"), " +
        "không phải bằng một lần đẩy.\n" +
        "  Muốn đẩy app riêng của xã thì bỏ --app=vihat, giữ --domain.",
    );
  }
  return co;
}

/** Cờ đã chọn một đích chưa. */
export function hasTarget(co) {
  return co.shared_app || co.ten_mien !== null;
}

/** "App ViHAT" hoặc "app riêng của xã <tên-miền>" — trong câu hỏi, câu từ chối và kế hoạch. */
export function targetName(dich) {
  return dich.loai === "app-chung" ? SHARED_APP_NAME : `app riêng của xã ${dich.ten_mien}`;
}

/** Câu hỏi đích khi không có cờ. Không có danh sách: platform cố ý không có lệnh liệt kê. */
export const TARGET_PROMPT = "App ViHAT (gõ vihat) hay tên miền xã: ";

/**
 * Câu trả lời cho `TARGET_PROMPT` → `{ shared_app, ten_mien }`. `vihat` (không phân biệt hoa thường,
 * bỏ khoảng trắng hai đầu) là App ViHAT; còn lại phải là một tên miền trần. Ném lỗi thì hỏi lại.
 */
export function parseTargetAnswer(answer) {
  const t = String(answer ?? "").trim();
  if (t === "") throw new Error("Chưa gõ gì. Gõ vihat cho App ViHAT, hoặc tên miền của xã (vd xa-a.vigov.vn).");
  if (t.toLowerCase() === "vihat") return { shared_app: true, ten_mien: null };
  return { shared_app: false, ten_mien: kiemTenMien(t) };
}

/** Câu từ chối khi không có cờ đích và không có người để hỏi — đủ hai cách chọn. */
export function noTargetMessage() {
  return [
    "Không đẩy: chưa chọn đích, và đây không phải một cửa sổ lệnh có người ngồi để chọn.",
    "  Script không bao giờ tự chọn đích, cũng không lấy đích từ citizen-app/.env. Chạy lại với một trong:",
    `    --app=vihat               ${SHARED_APP_NAME} (app chung)`,
    "    --domain=<tên-miền>       app riêng của xã có tên miền ấy",
  ].join("\n");
}

/** `{ loai, ten_mien }` của đích đã chọn. Ném nếu chưa chọn — hàm không tự chọn. */
export function targetOf({ shared_app, ten_mien }) {
  if (!shared_app && ten_mien === null) throw new Error("Chưa chọn đích: --app=vihat hoặc --domain=<tên-miền>.");
  return shared_app ? { loai: "app-chung", ten_mien: null } : { loai: "app-rieng", ten_mien };
}

/** Truy vấn của tuyến App ID cho một đích: `app=vihat` hoặc `host=<tên-miền>`. */
export function miniAppIdQuery(dich) {
  return dich.loai === "app-chung" ? { app: "vihat" } : { host: dich.ten_mien };
}

/**
 * KẾT QUẢ CỦA PLATFORM → APP ID CỦA ĐÍCH. `outcome` là thứ `lookupMiniAppId` trả:
 * `{ kind: "http", status, body }` (body đã parse JSON, hoặc `null`) hoặc `{ kind: "network", reason }`.
 *
 * | Platform | `--app-id` | Kết quả |
 * |---|---|---|
 * | 200, `source` đúng loại đích | không | `{ kind: "verified", app_id }` |
 * | 200, `source` đúng loại đích | trùng | như trên |
 * | 200, `source` đúng loại đích | KHÁC | DỪNG — một trong hai đã sai, script không chọn hộ |
 * | 200, `source` sai loại (App ViHAT ra `rieng`, xã ra `chung`) | bất kỳ | DỪNG — đẩy sẽ đè app phía bên kia |
 * | 200 mà thân hỏng | bất kỳ | DỪNG |
 * | 404 `mini_app_id_not_found` | bất kỳ | DỪNG — chỉ chỗ sửa ở platform-admin |
 * | 409 | bất kỳ | DỪNG — platform có hơn một app sống cho đích, script không chọn |
 * | 400 | bất kỳ | DỪNG |
 * | lỗi mạng · 5xx · 429 · 404 không mang mã ấy (tuyến chưa có) | không | DỪNG (fail closed) |
 * | như dòng trên | có | `{ kind: "unverified", app_id: --app-id, warning }` — deploy.mjs chỉ đi tiếp khi có người gõ "c" |
 * | mã khác (401, 403, 3xx…) | bất kỳ | DỪNG |
 *
 * VÌ SAO 404 KHÔNG CHO `--app-id` ĐI QUA: platform đã TRẢ LỜI rằng đích không có App ID. Máy chủ đọc
 * đúng bảng ấy để biết một App ID phục vụ xã nào, nên app đẩy lên bằng một App ID bảng không biết sẽ
 * mở ra bị từ chối. Chỗ sửa là bảng, không phải cờ.
 *
 * VÌ SAO 404 KHÔNG MANG MÃ ẤY LẠI LÀ "KHÔNG TỚI ĐƯỢC": hợp đồng nói MỌI "không có" là một thân duy
 * nhất. Một 404 khác là ingress chưa có tuyến (platform chưa bản mới) — platform không trả lời gì về đích.
 */
export function appIdFromPlatform(dich, outcome, given = null) {
  const name = targetName(dich);
  const fix = dich.loai === "app-chung" ? WHERE_TO_FIX["app-chung"] : WHERE_TO_FIX["app-rieng"](dich.ten_mien);
  const code = outcome?.kind === "http" && typeof outcome.body?.code === "string" ? outcome.body.code : null;

  const unreachable = (why) => {
    if (given === null) {
      throw new Error(
        `Không đẩy: không hỏi được platform App ID của ${name} — ${why}.\n` +
          "  Script không đoán App ID khi platform không trả lời (fail closed). Thử lại khi platform chạy.\n" +
          "  Khẩn cấp, có người ngồi trước cửa sổ lệnh: thêm --app-id=<chữ số> (App ID ở platform-admin) —\n" +
          "  script sẽ cảnh báo rằng App ID ấy CHƯA được đối chiếu và hỏi xác nhận. Không có người thì không có lối này.",
      );
    }
    return {
      kind: "unverified",
      app_id: given,
      warning:
        `⚠ ⚠ App ID ${given} của ${name} KHÔNG được đối chiếu với platform (${why}).\n` +
        "  Nếu nó sai, bản dựng đè lên app của người khác, hoặc lên một App ID máy chủ không biết — app mở ra bị từ chối.\n" +
        "  Chỉ đi tiếp nếu bạn đã tự mở platform-admin và thấy đúng App ID này cho đích này.",
    };
  };

  if (outcome?.kind === "network") return unreachable(`không kết nối được (${outcome.reason})`);
  if (outcome?.kind !== "http") throw new Error("Kết quả tra App ID không đọc được.");
  const { status, body } = outcome;

  if (status === 200) {
    const app_id = body?.app_id;
    const source = body?.source;
    if (!isAppId(app_id) || (source !== "chung" && source !== "rieng")) {
      throw new Error(
        `Không đẩy: platform trả 200 nhưng thân không đúng hợp đồng {"app_id":"<chữ số>","source":"chung"|"rieng"}.`,
      );
    }
    const expected = dich.loai === "app-chung" ? "chung" : "rieng";
    if (source !== expected) {
      throw new Error(
        dich.loai === "app-chung"
          ? `Không đẩy: platform nói App ID ${app_id} là app RIÊNG của một xã, không phải ${SHARED_APP_NAME} — đẩy lên đó sẽ đè app của xã.\n  Sửa ở: ${fix}.`
          : `Không đẩy: platform nói App ID ${app_id} của ${dich.ten_mien} là app CHUNG — đẩy app riêng lên đó sẽ đè ${SHARED_APP_NAME}.\n  Sửa ở: ${fix}.`,
      );
    }
    if (given !== null && given !== app_id) {
      throw new Error(
        `Không đẩy: --app-id=${given} khác App ID platform trả cho ${name} (${app_id}).\n` +
          "  Một trong hai đã sai. Platform là nguồn sự thật: bỏ --app-id, hoặc sửa App ID ở platform-admin nếu chính nó sai.",
      );
    }
    return { kind: "verified", app_id };
  }
  if (status === 404 && code === "mini_app_id_not_found") {
    throw new Error(
      `Không đẩy: platform chưa có App ID nào đang dùng cho ${name}.\n` +
        `  Sửa ở: ${fix}, rồi chạy lại. --app-id không thay được bước này: máy chủ đọc chính bảng ấy để mở app.`,
    );
  }
  if (status === 409) {
    throw new Error(
      `Không đẩy: platform trả 409${code ? ` (${code})` : ""} — có hơn một app đang sống cho ${name}.\n` +
        `  Script không chọn hộ. Tắt app thừa ở platform-admin (${fix.replace(/^platform-admin → /, "")}), rồi chạy lại.`,
    );
  }
  if (status === 400) {
    throw new Error(`Không đẩy: platform từ chối truy vấn (400${code ? ` ${code}` : ""}). Kiểm lại tên miền của đích.`);
  }
  if (status === 404) return unreachable("platform chưa có tuyến tra App ID (404 không mang mã mini_app_id_not_found)");
  if (status === 429) return unreachable("platform đang giới hạn tần suất (429)");
  if (status >= 500) return unreachable(`platform lỗi (${status})`);
  throw new Error(`Không đẩy: platform trả ${status}${code ? ` (${code})` : ""} cho tuyến tra App ID — không đúng hợp đồng.`);
}

/** Câu hỏi xác nhận khi App ID chưa đối chiếu được. Chỉ "c" là đi tiếp. */
export const UNVERIFIED_CONFIRM_PROMPT = "Vẫn đẩy với App ID CHƯA đối chiếu này? (c/k) ";

/** Câu từ chối khi App ID chưa đối chiếu được và không có người để xác nhận. */
export function unverifiedRefusal(dich) {
  return (
    `Không đẩy: App ID của ${targetName(dich)} chưa đối chiếu được với platform, và đây không phải một ` +
    "cửa sổ lệnh có người ngồi để xác nhận. --app-id chỉ là lối khẩn cấp có người gõ \"c\"; Jenkins và mọi " +
    "lần chạy không người phải đợi platform trả lời."
  );
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
 * Cùng một luật cho CẢ HAI đích (06/10/2026 — trước đó app chung không token thì để zmp đọc `.env`):
 *
 * | Tình huống | Kết quả |
 * |---|---|
 * | đích chưa có App ID | TỪ CHỐI — không biết token phải thuộc app nào |
 * | không có token trong môi trường | TỪ CHỐI — `.env` của máy là token của app đăng nhập lần cuối |
 * | token không đọc được claim | TỪ CHỐI |
 * | claim ≠ App ID đích | TỪ CHỐI |
 * | claim == App ID đích | QUA |
 *
 * `deploy.mjs` đọc mọi lần TỪ CHỐI (trừ dòng đầu, đã chặn trước đó) là "cần đăng nhập cho đúng App ID"
 * khi có người ngồi trước cửa sổ lệnh.
 */
export function kiemToken(dich, token) {
  if (!isAppId(dich.app_id)) {
    return { ok: false, ly_do: `App ID của ${targetName(dich)} chưa có — không biết token phải thuộc app nào.` };
  }
  if (typeof token !== "string" || token === "") {
    return {
      ok: false,
      ly_do:
        "ZMP_TOKEN chưa có trong môi trường. Script KHÔNG dùng citizen-app/.env — token trong ấy thuộc app " +
        `nào người ấy đăng nhập lần cuối. Cần ZMP_TOKEN của đúng App ID ${dich.app_id}.`,
    };
  }
  const claim = appIdTrongToken(token);
  if (claim === null) return { ok: false, ly_do: "ZMP_TOKEN không đọc được claim appId — không biết nó đẩy đi đâu." };
  if (claim !== dich.app_id) {
    return { ok: false, ly_do: `ZMP_TOKEN là token của App ID ${claim}, không phải ${dich.app_id}.` };
  }
  return { ok: true, ly_do: `thuộc đúng App ID ${claim}.` };
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

/**
 * `app-config.json` cho một lần đẩy. App riêng của xã ẨN thanh tiêu đề gốc của Zalo
 * (`app.actionBarHidden`, khoá mà trình giả lập của zmp-cli 4.0.3 đọc — `start/frame/index.html`):
 * thanh ấy mang `app.title` chung mọi bản dựng ("ViHAT Group"), và app riêng không được mang chữ nào
 * của ViHAT (chủ dự án, 27–28/09/2026). App ViHAT thì trả NGUYÊN VĂN — không đổi một byte.
 */
export function appConfigChoLanDay(noi_dung, own_app) {
  if (!own_app) return noi_dung;
  const cau_hinh = JSON.parse(noi_dung);
  cau_hinh.app = { ...cau_hinh.app, actionBarHidden: true };
  return `${JSON.stringify(cau_hinh, null, 2)}
`;
}

/**
 * Nhãn phiên bản trên console Zalo: đích (App ID, + tên miền cho app riêng) · commit · lúc · dirty.
 */
export function nhanPhienBan({ dich, sha, luc, dirty }) {
  const noi =
    dich.loai === "app-rieng" ? `${dich.ten_mien} · app ${dich.app_id}` : `app-vihat · app ${dich.app_id}`;
  return `${noi} · ${sha} · ${luc}${dirty ? " · dirty" : ""}`;
}

/**
 * Các dòng của kế hoạch "ĐỌC TRƯỚC KHI ĐỂ NÓ CHẠY TIẾP" — những điều quyết định hậu quả: đẩy lên app
 * nào, App ID nào và lấy từ đâu, tên miền có vào bundle không, token từ đâu và có qua không, bản thử
 * nghiệm hay phát hành, nhãn nào hiện trong console Zalo, máy chủ nào nung vào bundle.
 */
export function planLines({
  dich,
  kiem_token,
  token_source,
  phat_hanh,
  api_host,
  platform_host,
  mota,
  logo = null,
  banner = null,
}) {
  const own = dich.loai === "app-rieng";
  const source =
    dich.app_id_source === "platform"
      ? `platform ${platform_host}`
      : dich.app_id_source === "unverified"
        ? "--app-id, CHƯA ĐỐI CHIẾU platform"
        : "?";
  const lines = [
    own ? `  Đích     : APP RIÊNG của xã ${dich.ten_mien}` : `  Đích     : ${SHARED_APP_NAME} (app chung)`,
    `  App ID   : ${dich.app_id}  (${source})`,
    own
      ? `  Nung xã  : CÓ — ${dich.ten_mien} nung vào bundle; app mở thẳng vào xã, ẩn thanh tiêu đề Zalo`
      : "  Nung xã  : KHÔNG — bundle chung; vào xã bằng QR có `d` (làm ở platform-admin)",
  ];
  if (own) {
    lines.push(`  Logo xã  : ${logo ?? "(chưa có — header hiện biểu tượng)"}`);
    lines.push(`  Banner xã: ${banner ?? "(chưa có — trang chủ không có banner)"}`);
  }
  lines.push(`  Token    : ${token_source} — ${kiem_token.ok ? "" : "KHÔNG QUA: "}${kiem_token.ly_do}`);
  lines.push(
    phat_hanh
      ? "  Loại bản : PHÁT HÀNH — bỏ -t. Bản này ra người dùng thật / gửi duyệt."
      : "  Loại bản : THỬ NGHIỆM — có -t. Chỉ mở được bằng link bản thử nghiệm.",
  );
  lines.push(`  Nhãn     : ${mota}`);
  lines.push(`  Máy chủ  : ${api_host === "" ? "(chưa khai — chỉ hợp lệ với --thu)" : api_host}`);
  lines.push("  Các bước : vite build → zmp sync-config → zmp deploy");
  return lines;
}
