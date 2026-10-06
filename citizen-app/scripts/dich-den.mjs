/**
 * CHỌN ĐÍCH CỦA MỘT LẦN ĐẨY: app nào → App ID → token nào được dùng (ADR 0047; chủ dự án 06/10/2026).
 *
 * HAI ĐÍCH, CHỌN TƯỜNG MINH — không bao giờ mặc định:
 *
 * | Cờ | Đích | Tên miền vào bundle |
 * |---|---|---|
 * | `--app=vihat` | App ViHAT — app chung | KHÔNG BAO GIỜ (ADR 0044: một xã trong bundle chung đưa MỌI người, kể cả người duyệt của Zalo, vào xã ấy) |
 * | `--domain=<tên-miền>` | app riêng của xã ấy | LUÔN — app mở thẳng vào xã (ADR 0047 §6; trước 06/10/2026 là cờ `--vao-thang`, nay đã bỏ) |
 * | không cờ nào | hỏi bằng menu khi có người ngồi trước cửa sổ lệnh; không thì TỪ CHỐI | — |
 *
 * VÌ SAO KHÔNG CÒN "KHÔNG CỜ = APP CHUNG": trước 06/10/2026, không cờ thì zmp-cli đọc `ZMP_TOKEN` trong
 * `citizen-app/.env` — tức đẩy lên app nào người ấy đăng nhập lần cuối trên máy ấy. Đích được chọn
 * ngầm bởi một tệp không ai nhìn. Nay script KHÔNG BAO GIỜ để zmp rơi về tệp ấy, với bất kỳ đích nào.
 *
 * QR mở App ViHAT vào một xã (`zalo.me/s/<App ID app chung>/?d=<host>&src=qr`) KHÔNG phải việc của
 * bước đẩy: nó làm ở platform-admin (chi tiết xã → "Mở bằng app ViHAT").
 *
 * Mọi hàm ở đây THUẦN — nhận dữ liệu, trả dữ liệu hoặc ném lỗi, không đọc đĩa, không đọc môi
 * trường, không hỏi ai — để `dich-den.test.mjs` kiểm được từng nhánh mà không chạy `zmp-cli`.
 * `deploy.mjs` là nơi duy nhất nối chúng với `process.argv` / `process.env` / bàn phím / tệp.
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

/** Tên đọc được của app chung, trên menu, trong kế hoạch và trong câu hỏi. */
export const SHARED_APP_NAME = "App ViHAT";

/** Tệp ánh xạ, gọi đúng tên trong mọi câu nói với người chạy lệnh. */
const REGISTRY_FILE = "scripts/ung-dung-theo-ten-mien.mjs";

/** Nơi người chạy lệnh tìm được App ID thật — không đoán, không chép từ chỗ khác. */
const WHERE_APP_ID = "xem platform-admin → chi tiết xã → ô QR";

/** Mọi cờ còn nhận, nói ra mỗi khi một cờ bị từ chối. */
const VALID_FLAGS = "--app=vihat · --domain=<tên-miền> · --app-id=<chữ số> · --phat-hanh · --thu";

/** Tên miền dạng máy chủ: chữ thường, số, gạch nối; ít nhất hai nhãn; không scheme/cổng/đường dẫn. */
const NHAN = /^(?!-)[a-z0-9-]{1,63}(?<!-)$/;

/** Placeholder trong tệp ánh xạ: `<…>`. Coi như CHƯA CÓ App ID. */
export function laPlaceholder(app_id) {
  return /^<[^>]*>$/.test(app_id);
}

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
 * Kiểm cả tệp ánh xạ. Ném lỗi nếu một khoá không phải tên miền trần, một App ID rỗng / có khoảng
 * trắng / không phải chữ số (placeholder `<…>` vẫn được), hoặc App ID của app chung trùng App ID của
 * một xã (đẩy app chung đè lên app của xã ấy).
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
    if (!isAppId(app_id) && !laPlaceholder(app_id)) {
      throw new Error(`App ID của "${ten}" phải chỉ gồm chữ số (hoặc placeholder <…>): "${app_id}".`);
    }
  }
  if (app_chung !== null) {
    if (typeof app_chung !== "string" || app_chung.trim() === "" || /\s/.test(app_chung)) {
      throw new Error("APP_ID_APP_CHUNG phải là null hoặc một App ID không rỗng.");
    }
    if (!isAppId(app_chung)) {
      throw new Error(`APP_ID_APP_CHUNG phải là null hoặc chỉ gồm chữ số: "${app_chung}".`);
    }
    if (Object.values(bang).includes(app_chung)) {
      throw new Error("APP_ID_APP_CHUNG trùng App ID của một xã: đẩy app chung sẽ đè lên app của xã ấy.");
    }
  }
}

/**
 * Đọc cờ dòng lệnh. CỜ LẠ THÌ DỪNG: `--domian=xa-a…` gõ nhầm mà bị bỏ qua là một lần đẩy lên
 * một đích người gõ không chọn.
 *
 * `--domain=` (rỗng) CŨNG DỪNG: một tham số để trống vì quên và một tham số để trống vì muốn app chung
 * trông giống hệt nhau. App chung có cờ riêng của nó (`--app=vihat`).
 *
 * Không cờ đích nào thì trả `shared_app: false, ten_mien: null` — CHƯA CÓ ĐÍCH, không phải "app chung".
 * `deploy.mjs` hỏi bằng menu, hoặc từ chối khi không có người để hỏi.
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
        throw new Error(`--app-id="${gia_tri}" không phải App ID: chỉ gồm chữ số (${WHERE_APP_ID}).`);
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

/**
 * Menu khi không có cờ đích: 1 = App ViHAT, rồi từng tên miền của tệp ánh xạ theo thứ tự trong tệp.
 * Mỗi dòng mang App ID, để người chọn đọc được mình sắp thay app nào.
 */
export function buildMenu(bang, app_chung) {
  const show = (app_id) => (app_id === null || laPlaceholder(app_id) ? "chưa có App ID" : app_id);
  return [
    { number: 1, label: `${SHARED_APP_NAME} (${show(app_chung)})`, shared_app: true, ten_mien: null },
    ...Object.entries(bang).map(([ten, app_id], i) => ({
      number: i + 2,
      label: `app riêng xã ${ten} (${show(app_id)})`,
      shared_app: false,
      ten_mien: ten,
    })),
  ];
}

/** Dòng menu ứng với câu trả lời; ném lỗi nếu câu trả lời không phải một số trên menu. */
export function pickMenuItem(menu, answer) {
  const t = String(answer ?? "").trim();
  const item = /^\d+$/.test(t) ? menu.find((m) => m.number === Number(t)) : undefined;
  if (item === undefined) throw new Error(`"${t}" không phải một lựa chọn. Gõ một số từ 1 đến ${menu.length}.`);
  return item;
}

/** Câu từ chối khi không có cờ đích và không có người để hỏi — liệt kê đủ các cách chọn. */
export function noTargetMessage(menu) {
  const dong = menu.map((m) =>
    m.shared_app ? `    --app=vihat                 ${m.label}` : `    --domain=${m.ten_mien}   ${m.label}`,
  );
  return [
    "Không đẩy: chưa chọn đích, và đây không phải một cửa sổ lệnh có người ngồi để chọn.",
    "  Script không bao giờ tự chọn đích, cũng không lấy đích từ citizen-app/.env. Chạy lại với một trong:",
    ...dong,
    "    --domain=<tên-miền>         app riêng của một xã chưa có trong tệp (kèm --app-id=<chữ số>)",
  ].join("\n");
}

/**
 * Chọn đích và App ID của nó. Trả `{ loai, ten_mien, app_id, app_id_source }`.
 *
 * | Tệp ánh xạ có App ID | `--app-id` | Kết quả |
 * |---|---|---|
 * | có | không | App ID của tệp |
 * | có | trùng | App ID của tệp |
 * | có | khác | DỪNG — một trong hai đã sai, và script không chọn hộ |
 * | không (null / `<…>` / tên miền chưa có dòng) | có | App ID của cờ |
 * | không | không | `app_id: null` — `deploy.mjs` hỏi, hoặc từ chối khi không có người để hỏi |
 *
 * App ID của cờ còn bị chặn khi nó là App ID của "phía bên kia": App ViHAT mang App ID của một xã
 * (đè app của xã), hoặc app riêng mang App ID của App ViHAT (đè app chung).
 */
export function chonDich({ shared_app, ten_mien, app_id: given = null }, bang, app_chung) {
  if (!shared_app && ten_mien === null) {
    throw new Error("Chưa chọn đích: --app=vihat hoặc --domain=<tên-miền>.");
  }
  const loai = shared_app ? "app-chung" : "app-rieng";
  const ten = shared_app ? null : ten_mien;
  const name = targetName({ loai, ten_mien: ten });
  const registry = shared_app ? app_chung : Object.hasOwn(bang, ten_mien) ? bang[ten_mien] : null;
  const known = registry !== null && !laPlaceholder(registry) ? registry : null;

  if (given !== null) {
    if (known !== null && given !== known) {
      throw new Error(
        `--app-id=${given} khác App ID của ${name} trong ${REGISTRY_FILE} (${known}).\n` +
          "  Một trong hai đã sai. App ID thật đã đổi thì sửa tệp ấy (và dòng MiniApp ở service-platform), " +
          "rồi commit; không thì bỏ --app-id.",
      );
    }
    if (shared_app && Object.values(bang).includes(given)) {
      throw new Error(`--app-id=${given} là App ID của app riêng một xã: đẩy ${SHARED_APP_NAME} lên đó sẽ đè app của xã.`);
    }
    if (!shared_app && app_chung !== null && given === app_chung) {
      throw new Error(`--app-id=${given} là App ID của ${SHARED_APP_NAME}: đẩy app riêng của xã lên đó sẽ đè app chung.`);
    }
  }
  const app_id = known ?? given;
  return {
    loai,
    ten_mien: ten,
    app_id,
    app_id_source: known !== null ? "registry" : given !== null ? "flag" : null,
  };
}

/** Câu hỏi App ID khi đích chưa có. */
export function appIdPrompt(dich) {
  return `Nhập App ID của ${targetName(dich)} (${WHERE_APP_ID}): `;
}

/** App ID người dùng gõ: bỏ khoảng trắng hai đầu, phải chỉ gồm chữ số. */
export function checkAppIdInput(text) {
  const t = String(text ?? "").trim();
  if (!isAppId(t)) throw new Error(`"${t}" không phải App ID: chỉ gồm chữ số (${WHERE_APP_ID}).`);
  return t;
}

/** Câu từ chối khi đích chưa có App ID và không có người để hỏi. */
export function missingAppIdMessage(dich) {
  return (
    `Không đẩy: App ID của ${targetName(dich)} chưa có trong ${REGISTRY_FILE}, và đây không phải một ` +
    "cửa sổ lệnh có người ngồi để nhập.\n" +
    `  Chạy lại với --app-id=<chữ số> (${WHERE_APP_ID}), hoặc ghi App ID vào tệp ấy rồi commit.`
  );
}

/** Lời nhắc sau khi script vừa ghi App ID vào tệp ánh xạ. */
export function savedRegistryNote(dich) {
  const dong = [
    `Đã ghi App ID của ${targetName(dich)} vào ${REGISTRY_FILE}. Tệp ấy PHẢI được commit —`,
    "  kèm cặp ghim trong scripts/dich-den.test.mjs (ca \"chỉ có App ID chủ dự án đã giao\" sẽ đỏ tới khi sửa).",
  ];
  if (dich.loai === "app-rieng") {
    dong.push("  Và thêm dòng MiniApp tương ứng ở service-platform: máy chủ đọc xã của một App ID ở đó (ADR 0047).");
  }
  return dong.join("\n");
}

/**
 * Văn bản mới của tệp ánh xạ sau khi ghi `app_id` cho đích. Sửa TỐI THIỂU: đúng một dòng
 * (`APP_ID_APP_CHUNG`, hoặc dòng của tên miền trong `APP_ID_THEO_TEN_MIEN`), hoặc thêm một dòng cuối
 * bảng ấy khi tên miền chưa có. Không đụng `COMMUNE_TERMS_BY_DOMAIN` — bảng ấy cũng khoá bằng tên miền,
 * nên chỉ tìm TRONG khối `APP_ID_THEO_TEN_MIEN`. Không tìm thấy hình dạng mong đợi thì ném lỗi, không đoán.
 */
export function updateRegistryText(text, dich, app_id) {
  if (!isAppId(app_id)) throw new Error(`"${app_id}" không phải App ID: chỉ gồm chữ số.`);
  if (dich.loai === "app-chung") {
    const re = /^export const APP_ID_APP_CHUNG = (?:null|"[^"\n]*");$/m;
    if (!re.test(text)) throw new Error(`Không tìm thấy dòng \`export const APP_ID_APP_CHUNG = …;\` trong ${REGISTRY_FILE}.`);
    return text.replace(re, () => `export const APP_ID_APP_CHUNG = "${app_id}";`);
  }
  const ten = kiemTenMien(dich.ten_mien);
  const block = /^(export const APP_ID_THEO_TEN_MIEN = \{\r?\n)([\s\S]*?)(^\};)/m.exec(text);
  if (block === null) throw new Error(`Không tìm thấy khối \`export const APP_ID_THEO_TEN_MIEN = { … };\` trong ${REGISTRY_FILE}.`);
  const [whole, open, body, close] = block;
  const escaped = ten.replace(/[.]/g, "\\.");
  const line = new RegExp(`^(\\s*)"${escaped}"\\s*:\\s*"[^"\\n]*"(,?)`, "m");
  const eol = open.endsWith("\r\n") ? "\r\n" : "\n";
  const new_body = line.test(body)
    ? body.replace(line, (_m, indent, comma) => `${indent}"${ten}": "${app_id}"${comma}`)
    : `${body}  "${ten}": "${app_id}",${eol}`;
  return text.slice(0, block.index) + open + new_body + close + text.slice(block.index + whole.length);
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
 * | claim ≠ App ID đích | TỪ CHỐI (App ViHAT mà claim là App ID của một xã: nói rõ sẽ đè app của xã) |
 * | claim == App ID đích | QUA |
 *
 * `deploy.mjs` đọc mọi lần TỪ CHỐI (trừ dòng đầu, đã chặn trước đó) là "cần đăng nhập cho đúng App ID"
 * khi có người ngồi trước cửa sổ lệnh.
 */
export function kiemToken(dich, token, bang) {
  if (dich.app_id === null || laPlaceholder(dich.app_id)) {
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
    if (dich.loai === "app-chung" && Object.values(bang).includes(claim)) {
      return {
        ok: false,
        ly_do: `ZMP_TOKEN là token của App ID ${claim} — app RIÊNG của một xã, không phải ${SHARED_APP_NAME} ${dich.app_id}.`,
      };
    }
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
    dich.loai === "app-rieng"
      ? `${dich.ten_mien} · app ${dich.app_id}`
      : dich.app_id === null
        ? "app-vihat"
        : `app-vihat · app ${dich.app_id}`;
  return `${noi} · ${sha} · ${luc}${dirty ? " · dirty" : ""}`;
}

/**
 * Các dòng của kế hoạch "ĐỌC TRƯỚC KHI ĐỂ NÓ CHẠY TIẾP" — những điều quyết định hậu quả: đẩy lên app
 * nào, App ID nào và lấy từ đâu, tên miền có vào bundle không, token từ đâu và có qua không, bản thử
 * nghiệm hay phát hành, nhãn nào hiện trong console Zalo, máy chủ nào nung vào bundle.
 */
export function planLines({ dich, kiem_token, token_source, phat_hanh, api_host, mota, logo = null, banner = null }) {
  const own = dich.loai === "app-rieng";
  const source = { registry: "tệp ánh xạ", flag: "--app-id", typed: "vừa nhập" }[dich.app_id_source] ?? "?";
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
