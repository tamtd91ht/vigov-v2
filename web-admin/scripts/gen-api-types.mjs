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

/** Dịch một schema OpenAPI sang biểu thức kiểu TypeScript. */
function dichKieu(schema, duong) {
  if (!schema || typeof schema !== "object") tuChoi(duong, "schema rỗng");

  if (schema.$ref) {
    const tien = "#/components/schemas/";
    if (!schema.$ref.startsWith(tien)) tuChoi(duong, `$ref ngoài components/schemas: ${schema.$ref}`);
    return tenKieu(schema.$ref.slice(tien.length));
  }
  for (const cam of ["allOf", "oneOf", "anyOf", "not", "enum", "const"]) {
    if (cam in schema) tuChoi(duong, `chưa hỗ trợ '${cam}'`);
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
  if (schema.additionalProperties !== false && schema.additionalProperties !== undefined) {
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

      const noiDung = op.requestBody?.content?.["application/json"]?.schema;
      const kieuThan = noiDung ? dichKieu(noiDung, `${duongDan}.requestBody`) : "never";

      const dongPhanHoi = [];
      for (const [ma, phanHoi] of Object.entries(op.responses ?? {})) {
        const s = phanHoi.content?.["application/json"]?.schema;
        dongPhanHoi.push(`    ${ma}: ${s ? dichKieu(s, `${duongDan}.${ma}`) : "void"};`);
      }

      ra.push(`/** ${phuongThuc.toUpperCase()} ${duongDan} — ${op.summary ?? ""} */`);
      ra.push(`export type ${ten} = {`);
      ra.push(`  duongDan: ${JSON.stringify(duongDan)};`);
      ra.push(`  phuongThuc: ${JSON.stringify(phuongThuc.toUpperCase())};`);
      ra.push(`  thamSo: {\n${dongThamSo.join("\n")}${dongThamSo.length ? "\n" : ""}  };`);
      ra.push(`  than: ${kieuThan};`);
      ra.push(`  phanHoi: {\n${dongPhanHoi.join("\n")}\n  };`);
      ra.push("};");
      ra.push("");
    }
  }
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
