// Sinh kiểu TypeScript từ kb/20-contracts/openapi.json.
//
// VÌ SAO CÓ TỆP NÀY — và vì sao không ai được gõ tay các kiểu ấy:
//
//   Hệ v1 khai "nguồn sự thật" của kiểu nằm ở frontend, rồi backend nhập lại. Kết cục là ba
//   bản chép tay của cùng một hình dạng, trôi dần khỏi nhau, và bản sai lại là bản chạy thật
//   (ROUTING.md:229, README của ứng dụng, mục 6). Ở đây nguồn sự thật là khai báo route trong
//   */internal/ → tools/apidoc sinh openapi.json → tệp này sinh schema.gen.ts. Một
//   chiều, không có đường ngược.
//
// VÌ SAO TỰ VIẾT, KHÔNG THÊM `openapi-typescript`:
//
//   Hợp đồng hiện tại là một tập con rất nhỏ của OpenAPI — 4 schema phẳng, 2 đường dẫn. Một
//   phụ thuộc mới trong hệ thống của cơ quan nhà nước phải trả giá bằng chuỗi cung ứng và bằng
//   việc nâng cấp về sau, mà ở đây nó chỉ để đọc bốn đối tượng phẳng. Đổi lại, bộ sinh này
//   **từ chối ồn ào** mọi cấu trúc nó chưa hiểu (allOf, oneOf, enum, kiểu trống) thay vì phát
//   ra `unknown`: một kiểu `unknown` lọt qua tsc là đúng cái hỏng mà tệp này sinh ra để chặn.
//
// Dùng:
//   node scripts/gen-api-types.mjs          ghi lại src/lib/api/schema.gen.ts
//   node scripts/gen-api-types.mjs --check  không ghi; khác thì thoát mã 1 (dùng trong CI)

import { readFileSync, writeFileSync, mkdirSync, existsSync } from "node:fs";
import { dirname, resolve } from "node:path";
import { fileURLToPath } from "node:url";

const thuMuc = dirname(fileURLToPath(import.meta.url));
// TÌM gốc kho, không ĐẾM số cấp thư mục.
//
// Dòng này từng là `resolve(thuMuc, "../../../kb/...")` — ba cấp, đúng khi ứng dụng còn nằm
// ở `apps/commune-admin/`. Bố cục phẳng bớt đi một cấp và nó trỏ ra NGOÀI kho, tới một
// đường dẫn không tồn tại. Một số cấp ghi cứng là một giả định về vị trí của chính mình, và
// giả định đó sai vào đúng ngày cây thư mục đổi — trong khi `go.mod` thì luôn ở gốc.
function timGocKho(batDau) {
  let d = batDau;
  for (let i = 0; i < 8; i++) {
    if (existsSync(resolve(d, "go.work"))) return d;
    const cha = dirname(d);
    if (cha === d) break;
    d = cha;
  }
  throw new Error(
    "gen-api-types: không tìm thấy gốc kho (không thấy go.work ở thư mục cha nào).\n" +
      "Dấu hiệu là go.work chứ không phải go.mod: mỗi đơn vị triển khai có go.mod riêng."
  );
}

const GOC = timGocKho(thuMuc);
const HOP_DONG = resolve(GOC, "kb/20-contracts/openapi.json");
const DICH = resolve(thuMuc, "../src/lib/api/schema.gen.ts");

/** Tên TypeScript hợp lệ, suy ra một-một từ tên trong hợp đồng (`httpx.Error` → `httpx_Error`). */
function tenKieu(tenTrongHopDong) {
  return tenTrongHopDong.replace(/[^A-Za-z0-9_]/g, "_");
}

function tuChoi(duong, viSao) {
  throw new Error(
    `gen-api-types: không sinh được ${duong}: ${viSao}.\n` +
      `Bộ sinh từ chối thay vì phát ra 'unknown' — một kiểu rỗng lọt qua tsc là đúng cái hỏng nó chặn.\n` +
      `Hãy mở rộng scripts/gen-api-types.mjs cho cấu trúc này, đừng gõ tay kiểu ở nơi khác.`,
  );
}

// THE EMPTY SCHEMA `{}` — "any JSON value", and why it is NOT `unknown`.
//
// JSON Schema reads `{}` as "any value". tools/apidoc emits it on purpose for exactly one kind of
// field: a Go `any` that carries a JSON document whose shape differs per record — today
// `audit.EntryView.delta`, the audit delta stored per verb (core/audit/read.go:89-92). A per-verb
// shape cannot live in a generated type.
//
// It becomes a named recursive `JsonValue`, not `unknown`: the screen can still only reach into it
// by narrowing, but tsc knows it is serialisable data (no functions, no Date, no undefined), so
// `JSON.stringify` and a `typeof` walk are type-checked. Only the BARE `{}` (a `description` at most)
// takes this branch — a schema with any other key and no `type` is still a hole and still refused.
const JSON_VALUE = "JsonValue";
const JSON_VALUE_DECL =
  `export type ${JSON_VALUE} =\n` +
  `  | string\n  | number\n  | boolean\n  | null\n` +
  `  | ${JSON_VALUE}[]\n  | { [key: string]: ${JSON_VALUE} };`;
let jsonValueUsed = false;

/** Dịch một schema OpenAPI sang biểu thức kiểu TypeScript. */
function dichKieu(schema, duong) {
  if (!schema || typeof schema !== "object") tuChoi(duong, "schema rỗng");

  if (Object.keys(schema).every((k) => k === "description")) {
    jsonValueUsed = true;
    return JSON_VALUE;
  }

  if (schema.$ref) {
    const tien = "#/components/schemas/";
    if (!schema.$ref.startsWith(tien)) tuChoi(duong, `$ref ngoài components/schemas: ${schema.$ref}`);
    return tenKieu(schema.$ref.slice(tien.length));
  }
  // `anyOf: [{$ref}, {type:"null"}]` — thành ngữ OpenAPI 3.1 cho một THAM CHIẾU có thể rỗng.
  //
  // Vì sao không dùng `type: [T, "null"]` như các trường vô hướng: `$ref` không nhận thêm
  // khoá nào bên cạnh, nên một tham chiếu có thể rỗng chỉ diễn đạt được bằng `anyOf`. Đây
  // đúng là hình dạng `tools/apidoc` phát ra cho `phienHienTaiRa.role` — một cán bộ có thể
  // CHƯA ĐƯỢC GÁN VAI TRÒ, và trường vẫn nằm trong `required`: luôn có mặt, giá trị `null`.
  //
  // Dịch thành `T | null`, cùng lý do với nhánh `type: [T,"null"]` phía dưới: `null` là câu
  // trả lời máy chủ KHẲNG ĐỊNH ("chưa gán vai trò"), khác hẳn trường vắng mặt ("máy chủ
  // không nói gì"). Gộp hai cái thì màn hình không phân biệt được "chưa gán" với "chưa đọc
  // được", và nó sẽ hiển thị cùng một dấu gạch ngang cho cả hai.
  //
  // CHỈ nhận đúng dạng hai nhánh có một `{type:"null"}`. Một liên hợp thật sự giữa hai kiểu
  // khác nhau là chuyện khác và phải được cân nhắc riêng, nên nó vẫn rơi xuống nhánh từ chối.
  if ("anyOf" in schema) {
    const ds = schema.anyOf;
    const laNull = (x) => x && typeof x === "object" && x.type === "null" && !x.$ref;
    const khongNull = Array.isArray(ds) ? ds.filter((x) => !laNull(x)) : [];
    if (!Array.isArray(ds) || ds.length !== 2 || khongNull.length !== 1) {
      tuChoi(duong, "anyOf không phải dạng <kiểu> | null — chưa hỗ trợ");
    }
    return `${dichKieu(khongNull[0], duong)} | null`;
  }

  for (const cam of ["allOf", "oneOf", "not", "const"]) {
    if (cam in schema) tuChoi(duong, `chưa hỗ trợ '${cam}'`);
  }

  // `enum` → hợp của các chuỗi hằng, KHÔNG phải `string`.
  //
  // ĐÂY LÀ CHỖ ĐẮT NHẤT CỦA TỆP NÀY. `sort` có danh sách đóng do máy chủ khai, và hợp đồng nay
  // mang chính danh sách ấy (tools/apidoc đọc `page.NewAllowlist` trong mã Go). Dịch nó thành
  // `string` là ném đi đúng thứ vừa được sinh ra để mang sang: khi ấy web gõ tay lại danh sách
  // cột ở một hằng số của riêng nó, và hằng số đó trôi mà không bài test nào đỏ — thêm một cột
  // sắp xếp ở máy chủ thì web không biết, gỡ một cột đi thì web vẫn gửi và nhận 400.
  //
  // Chỉ nhận enum CHUỖI. Một enum số hay enum trộn kiểu là chuyện khác và phải được cân nhắc
  // riêng, nên nó vẫn rơi xuống nhánh từ chối.
  if ("enum" in schema) {
    const ds = schema.enum;
    if (!Array.isArray(ds) || ds.length === 0) tuChoi(duong, "enum rỗng hoặc không phải mảng");
    if (!ds.every((v) => typeof v === "string")) {
      tuChoi(duong, "enum không phải toàn chuỗi — chưa hỗ trợ");
    }
    if (schema.type !== undefined && schema.type !== "string") {
      tuChoi(duong, `enum chuỗi nhưng type là '${schema.type}'`);
    }
    return ds.map((v) => JSON.stringify(v)).join(" | ");
  }

  // `"type": ["string", "null"]` — cách OpenAPI 3.1 khai một trường CÓ THỂ RỖNG.
  //
  // Dịch thành `T | null`, KHÔNG phải `T | undefined` và cũng không phải `T?`. Ba thứ này khác
  // nhau ở đúng chỗ quan trọng: `last_login_at: null` nghĩa là "người này chưa đăng nhập bao
  // giờ" — một sự thật máy chủ khẳng định. Trường vắng mặt nghĩa là "máy chủ không nói gì".
  // Gộp hai cái làm một thì màn hình không còn phân biệt được "chưa từng đăng nhập" với "chưa
  // đọc được", và nó sẽ hiển thị dấu gạch ngang cho cả hai.
  //
  // Chỉ nhận đúng dạng hai phần tử có `null`. Một liên hợp kiểu thật (`["string","number"]`)
  // là chuyện khác hẳn và phải được cân nhắc riêng, nên nó vẫn rơi xuống nhánh từ chối.
  if (Array.isArray(schema.type)) {
    const khongRong = schema.type.filter((t) => t !== "null");
    if (schema.type.includes("null") && khongRong.length === 1) {
      return `${dichKieu({ ...schema, type: khongRong[0] }, duong)} | null`;
    }
    tuChoi(duong, `liên hợp kiểu '${schema.type.join(",")}' chưa hỗ trợ`);
  }

  switch (schema.type) {
    case "string":
      // date-time giữ nguyên `string`: JSON không có kiểu ngày, và Date ở đây sẽ là một phép
      // chuyển ngầm mà phía máy chủ không hề hứa. Ai cần Date thì tự dựng, có chỗ để thấy.
      return "string";
    case "integer":
    case "number":
      return "number";
    case "boolean":
      return "boolean";
    case "array":
      if (!schema.items) tuChoi(duong, "mảng không khai 'items'");
      return `Array<${dichKieu(schema.items, `${duong}[]`)}>`;
    case "object":
      return dichDoiTuong(schema, duong);
    default:
      tuChoi(duong, `kiểu '${schema.type ?? "(không khai)"}' chưa hỗ trợ`);
  }
}

function dichDoiTuong(schema, duong) {
  const thuocTinh = schema.properties ?? {};
  const batBuoc = new Set(schema.required ?? []);
  const them = schema.additionalProperties;

  // BẢNG TRA (`Record<string, T>`) — `additionalProperties` là một SCHEMA, và không có
  // `properties` nào. Đây là cách OpenAPI khai một đối tượng mà KHOÁ là dữ liệu chứ không phải
  // tên trường: `finance.dongRa.values` là `columnID -> đồng`, và số cột do xã tự khai nên tên
  // khoá không thể nằm trong một kiểu sinh sẵn.
  //
  // VÌ SAO NHÁNH NÀY PHẢI CÓ, chứ không phải "chưa tới lượt": thiếu nó thì bộ sinh ném ở
  // `finance.dongRa` và **KHÔNG SINH ĐƯỢC TỆP NÀO CẢ** — một lược đồ hình bảng tra làm hỏng
  // kiểu của toàn bộ hợp đồng, kể cả những phân hệ không liên quan. Đo 23/09/2026: một agent
  // dựng màn Thu-chi dừng với 0 tệp vì `npm run gen:api` hỏng toàn tệp, không riêng phần nó cần.
  //
  // CHỈ NHẬN DẠNG THUẦN. Có `properties` VÀ `additionalProperties` cùng lúc là "vài trường biết
  // trước, phần còn lại tuỳ ý" — nó dịch được, nhưng nó cũng là chỗ một trường gõ sai tên lặng
  // lẽ rơi vào phần tuỳ ý thay vì thành lỗi kiểu. Dạng ấy rơi xuống nhánh từ chối bên dưới để
  // có người cân nhắc, đúng cách liên hợp kiểu thật đang được xử lý ở `dichKieu`.
  if (them && typeof them === "object" && Object.keys(thuocTinh).length === 0) {
    return `Record<string, ${dichKieu(them, `${duong}.<khoá>`)}>`;
  }

  // `additionalProperties: true` cũng rơi vào đây: nó là `Record<string, unknown>`, tức bỏ hẳn
  // kiểm kiểu cho mọi giá trị. Từ chối để người viết hợp đồng nói ra kiểu, thay vì để màn hình
  // đoán.
  if (them !== false && them !== undefined) {
    tuChoi(duong, "additionalProperties khác false chưa hỗ trợ");
  }
  const dong = [];
  for (const [ten, con] of Object.entries(thuocTinh)) {
    if (con.description) dong.push(`  /** ${con.description} */`);
    const dau = batBuoc.has(ten) ? "" : "?";
    dong.push(`  ${JSON.stringify(ten)}${dau}: ${dichKieu(con, `${duong}.${ten}`)};`);
  }
  return dong.length === 0 ? "Record<string, never>" : `{\n${dong.join("\n")}\n}`;
}

// REQUEST BODIES: `application/json` OR `multipart/form-data`, and nothing else — loudly.
//
// Until 09/10/2026 this read `content["application/json"]` and nothing more, so a multipart route
// (every upload, ADR 0052 §Sửa đổi 09/10/2026) came out as `than: never` WITHOUT A WORD: the screen
// then typed its form parts by hand, which is exactly the copy this file exists to prevent. Any other
// content type (`application/x-www-form-urlencoded`, `application/octet-stream`, two types on one
// route) is refused here, so the day one appears somebody decides how it is typed.
const REQUEST_CONTENT_TYPES = new Set(["application/json", "multipart/form-data"]);

function requestBodyOf(op, where) {
  const content = op.requestBody?.content;
  if (content === undefined) return { jsonSchema: undefined, multipart: undefined };
  const kinds = Object.keys(content);
  for (const k of kinds) {
    if (!REQUEST_CONTENT_TYPES.has(k)) tuChoi(`${where}.requestBody`, `content type '${k}' chưa hỗ trợ`);
  }
  if (kinds.length !== 1) {
    tuChoi(`${where}.requestBody`, `cần đúng MỘT content type, hợp đồng khai ${kinds.length}: ${kinds.join(", ")}`);
  }
  if (kinds[0] === "application/json") return { jsonSchema: content["application/json"].schema, multipart: undefined };
  return { jsonSchema: undefined, multipart: dichMultipart(content["multipart/form-data"].schema, `${where}.multipart`) };
}

// A MULTIPART UPLOAD (`core/httpx` ReadUpload): text parts, then ONE binary part named `file`, LAST.
//
// Two things come out, because the wire shape has two facts:
//   `multipart`       the parts and their types — text parts are `string` (a form part has no number),
//                     the file is `Blob`; `?` follows the contract's `required`.
//   `multipartParts`  the parts IN THE CONTRACT'S ORDER, as a tuple type. Order is part of the contract
//                     here: the server reads `size` before the file (multipart carries no part length)
//                     and refuses a field after it. A client constant typed with this tuple fails tsc
//                     the day the order or a name changes — the hand-copied order cannot drift silently.
//
// Refused: anything but string / binary-string parts, a binary part other than `file`, `file` not last
// or not required, `additionalProperties` other than false.
function dichMultipart(schema, duong) {
  if (!schema || schema.type !== "object") tuChoi(duong, "multipart schema không phải object");
  if (schema.additionalProperties !== false) tuChoi(duong, "multipart cần additionalProperties: false");
  const props = Object.entries(schema.properties ?? {});
  const required = new Set(schema.required ?? []);
  if (props.length === 0) tuChoi(duong, "multipart không khai phần nào");
  const lines = [];
  const names = [];
  props.forEach(([name, part], i) => {
    const binary = part?.type === "string" && part.format === "binary";
    const text = part?.type === "string" && part.format === undefined;
    if (!binary && !text) tuChoi(`${duong}.${name}`, "phần multipart chỉ nhận string hoặc string/binary");
    if (binary && name !== "file") tuChoi(`${duong}.${name}`, "phần nhị phân phải tên 'file'");
    if (name === "file" && (!binary || i !== props.length - 1 || !required.has("file"))) {
      tuChoi(`${duong}.file`, "'file' phải là string/binary, bắt buộc, và là phần CUỐI");
    }
    if (part.description) lines.push(`    /** ${part.description} */`);
    lines.push(`    ${JSON.stringify(name)}${required.has(name) ? "" : "?"}: ${binary ? "Blob" : "string"};`);
    names.push(JSON.stringify(name));
  });
  if (!names.includes('"file"')) tuChoi(duong, "multipart không có phần 'file'");
  return { type: `{\n${lines.join("\n")}\n  }`, order: `readonly [${names.join(", ")}]` };
}

function sinh(hopDong) {
  const ra = [];
  ra.push("// TỆP SINH RA. ĐỪNG SỬA TAY — sửa tay mất sạch ở lần sinh sau (luật 9, bất biến 8).");
  ra.push("//");
  ra.push("// Nguồn: kb/20-contracts/openapi.json (do tools/apidoc sinh từ khai báo route trong");
  ra.push("// */internal/). Sinh lại: npm run gen:api — trong web-admin.");
  ra.push("//");
  ra.push(`// Hợp đồng: ${hopDong.info.title} ${hopDong.info.version}`);
  ra.push("");

  const schemas = hopDong.components?.schemas ?? {};
  for (const [ten, schema] of Object.entries(schemas)) {
    ra.push(`export type ${tenKieu(ten)} = ${dichKieu(schema, ten)};`);
    ra.push("");
  }

  const PHUONG_THUC = ["get", "post", "put", "patch", "delete"];
  for (const [duongDan, mucDuongDan] of Object.entries(hopDong.paths ?? {})) {
    for (const phuongThuc of PHUONG_THUC) {
      const op = mucDuongDan[phuongThuc];
      if (!op) continue;
      const ten = tenKieu(op.operationId ?? tuChoi(duongDan, "operation thiếu operationId"));

      const thamSo = [...(mucDuongDan.parameters ?? []), ...(op.parameters ?? [])];
      const thamSoDuongDan = thamSo.filter((p) => p.in === "path");
      const dongThamSo = thamSoDuongDan.map(
        (p) => `    ${JSON.stringify(p.name)}: ${dichKieu(p.schema, `${duongDan}.${p.name}`)};`,
      );

      // THAM SỐ TRUY VẤN, và vì sao chúng phải có mặt ở đây.
      //
      // Cho tới 2026-09-17 bộ sinh chỉ lấy tham số ĐƯỜNG DẪN, nên `limit/cursor/sort/order`
      // không tới được TypeScript. Web vì thế gõ tay danh sách cột sắp xếp trong một hằng số
      // của riêng nó — chỗ DUY NHẤT trong cả ứng dụng chép một phần hợp đồng, với nguồn sự
      // thật nằm cách đó hai module trong một tệp Go. Bản chép ấy không làm đỏ bài test nào
      // lúc nó trôi.
      //
      // `?` cho mọi tham số truy vấn: chúng đều có mặc định ở máy chủ, nên không gửi là hợp lệ.
      const thamSoTruyVan = thamSo.filter((p) => p.in === "query");
      const dongTruyVan = thamSoTruyVan.map(
        (p) =>
          `    ${JSON.stringify(p.name)}${p.required ? "" : "?"}: ` +
          `${dichKieu(p.schema, `${duongDan}.?${p.name}`)};`,
      );

      const { jsonSchema: noiDung, multipart } = requestBodyOf(op, `${phuongThuc.toUpperCase()} ${duongDan}`);
      const kieuThan = noiDung ? dichKieu(noiDung, `${duongDan}.requestBody`) : "never";

      const dongPhanHoi = [];
      // NAMED ERROR CODES (`x-vigov-error-codes`, tools/apidoc) → a string union per status, so a
      // screen branches on `err.code` against what the server declares instead of a hand-typed literal
      // that compiles whether or not the server ever sends it. Emitted ONLY on operations that declare
      // codes: every other operation type stays byte-identical to before this branch existed.
      const errorCodeLines = [];
      for (const [ma, phanHoi] of Object.entries(op.responses ?? {})) {
        const s = phanHoi.content?.["application/json"]?.schema;
        dongPhanHoi.push(`    ${ma}: ${s ? dichKieu(s, `${duongDan}.${ma}`) : "void"};`);
        const codes = phanHoi["x-vigov-error-codes"];
        if (codes !== undefined) {
          if (!Array.isArray(codes) || codes.length === 0 || !codes.every((c) => typeof c === "string")) {
            tuChoi(`${duongDan}.${ma}`, "x-vigov-error-codes không phải mảng chuỗi khác rỗng");
          }
          errorCodeLines.push(`    ${ma}: ${codes.map((c) => JSON.stringify(c)).join(" | ")};`);
        }
      }

      ra.push(`/** ${phuongThuc.toUpperCase()} ${duongDan} — ${op.summary ?? ""} */`);
      ra.push(`export type ${ten} = {`);
      ra.push(`  duongDan: ${JSON.stringify(duongDan)};`);
      ra.push(`  phuongThuc: ${JSON.stringify(phuongThuc.toUpperCase())};`);
      ra.push(`  thamSo: {\n${dongThamSo.join("\n")}${dongThamSo.length ? "\n" : ""}  };`);
      ra.push(`  truyVan: {\n${dongTruyVan.join("\n")}${dongTruyVan.length ? "\n" : ""}  };`);
      ra.push(`  than: ${kieuThan};`);
      if (multipart) {
        ra.push(`  multipart: ${multipart.type};`);
        ra.push(`  multipartParts: ${multipart.order};`);
      }
      ra.push(`  phanHoi: {\n${dongPhanHoi.join("\n")}\n  };`);
      if (errorCodeLines.length > 0) ra.push(`  errorCodes: {\n${errorCodeLines.join("\n")}\n  };`);
      ra.push("};");
      ra.push("");
    }
  }
  // Declared once, after the header, and only when some schema used it — a contract with no
  // bare `{}` produces byte-identical output to before this branch existed.
  if (jsonValueUsed) ra.splice(7, 0, JSON_VALUE_DECL, "");
  return ra.join("\n");
}

const hopDong = JSON.parse(readFileSync(HOP_DONG, "utf8"));
const noiDung = sinh(hopDong);

if (process.argv.includes("--check")) {
  let hienCo = "";
  try {
    hienCo = readFileSync(DICH, "utf8");
  } catch {
    /* chưa có tệp — coi như khác */
  }
  if (hienCo !== noiDung) {
    console.error(
      "src/lib/api/schema.gen.ts đã lệch khỏi kb/20-contracts/openapi.json. Chạy: npm run gen:api",
    );
    process.exit(1);
  }
  console.log("schema.gen.ts khớp hợp đồng.");
} else {
  mkdirSync(dirname(DICH), { recursive: true });
  writeFileSync(DICH, noiDung, "utf8");
  console.log(`Đã sinh ${DICH}`);
}
