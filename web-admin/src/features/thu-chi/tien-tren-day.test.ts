import { afterEach, describe, expect, it, vi } from "vitest";

import type { finance_cotRa } from "@/lib/api/schema.gen";
import { ghiDot, layDot, suaKhoanMuc } from "@/lib/api/thu-chi";

import { docSoNhap, dongSangChuoi, dungThanDot, nhanSoTien } from "./nhan-thu-chi";

/**
 * MỘT CON SỐ ĐI TRỌN VÒNG: chữ cán bộ gõ theo đơn vị của bảng → thân HTTP mà HÀM GỌI THẬT gửi đi →
 * JSON máy chủ trả về (số nguyên đồng, đúng chữ số như Go mã hoá `int64`) → chữ màn hình in ra.
 *
 * Từng mắt xích đã có bài kiểm riêng (`nhan-thu-chi.test.ts`, `lib/api/thu-chi.test.ts`). Tệp này
 * canh CHỖ NỐI: một mắt xích đúng một mình vẫn có thể ghép sai với mắt xích kế bên — một hàm gọi
 * "tiện tay" làm tròn hay đổi số sang chuỗi, một phép đọc phản hồi đi qua số thực — và khi ấy mọi
 * bài kiểm đơn vị vẫn xanh trong khi con số lưu xuống lệch con số cán bộ gõ.
 */

const COT: finance_cotRa[] = [
  { id: "C1", name: "Thu ngân sách", order: 1, type: "so" },
  { id: "C2", name: "Dự toán năm", order: 2, type: "so" },
  { id: "C3", name: "So sánh (%)", order: 3, type: "phan_tram", formula: "col_1 / col_2 * 100" },
];

afterEach(() => {
  vi.unstubAllGlobals();
  vi.restoreAllMocks();
});

function batFetch(tra: () => Response) {
  const gia = vi.fn(async (_duongDan: string, _tuyChon?: RequestInit) => tra());
  vi.stubGlobal("fetch", gia);
  return gia;
}

/** Thân gửi đi, NGUYÊN VĂN — để thấy số đi lên dây dưới dạng nào, không chỉ giá trị sau khi parse. */
function thanTho(goi: ReturnType<typeof batFetch>): string {
  const tho = goi.mock.calls[0]?.[1]?.body;
  return typeof tho === "string" ? tho : "";
}

/**
 * Phản hồi danh sách đợt dựng bằng CHUỖI, với chữ số đồng chép thẳng vào JSON — đúng hình dạng Go
 * ghi một `int64`. Dựng bằng `JSON.stringify(number)` thì bài kiểm sẽ đi qua chính phép chuyển số
 * của JS mà nó đang muốn canh.
 */
function phanHoiDanhSach(chuSoDong: string): Response {
  const than =
    `{"line_id":"L1","method":"entries","entries":[{"id":"D1","line_id":"L1",` +
    `"date":"2026-09-25","content":"Thu đợt 1","values":{"C1":${chuSoDong},"C2":null}}]}`;
  return new Response(than, { status: 200, headers: { "Content-Type": "application/json" } });
}

describe("một con số triệu đồng đi trọn vòng qua các hàm gọi THẬT", () => {
  // [chữ gõ theo triệu đồng, số nguyên đồng phải lên dây]
  const CA: readonly [string, number][] = [
    ["-1.234,567891", -1234567891], // âm, đủ sáu chữ số lẻ
    ["1,005", 1005000], // 1.005 * 1e6 === 1004999.9999999999 trong JS
    ["-0,000001", -1], // một đồng âm
    ["9.007.199.254,740991", Number.MAX_SAFE_INTEGER], // sát trần số nguyên chính xác của JS
    ["-9.007.199.254,740991", -Number.MAX_SAFE_INTEGER],
  ];

  for (const [go, dong] of CA) {
    it(`"${go}" triệu ⇒ ${dong} đồng lên dây ⇒ "${go}" trên màn hình`, async () => {
      // 1. Biểu mẫu ghi đợt dựng thân.
      const dung = dungThanDot(
        { ngay: "2026-09-25", noiDung: "Thu đợt 1", doiTac: "", soChungTu: "", gia: { C1: go } },
        COT,
        "trieu-dong",
      );
      expect(dung.ok).toBe(true);
      if (!dung.ok) return;

      // 2. Hàm gọi thật gửi đi. Số lên dây là SỐ JSON NGUYÊN, đúng từng chữ số — không chuỗi, không mũ.
      const goiGhi = batFetch(
        () => new Response(JSON.stringify({ id: "D1" }), { status: 201 }),
      );
      await ghiDot("L1", dung.than, "khoa-1");
      const tho = thanTho(goiGhi);
      expect(tho).toContain(`"C1":${String(dong)}`);
      expect((JSON.parse(tho) as { values: Record<string, unknown> }).values).toEqual({
        C1: dong,
        C2: null,
      });

      // 3. Ô sửa khoản mục đi qua `docSoNhap` rồi `suaKhoanMuc` — cùng một con số, cùng một chữ số.
      const doc = docSoNhap(go, "trieu-dong");
      expect(doc).toEqual({ loai: "so", gia: dong });
      const goiSua = batFetch(() => new Response(JSON.stringify({ id: "L1" }), { status: 200 }));
      await suaKhoanMuc("L1", { values: { C1: doc.loai === "so" ? doc.gia : null } });
      expect(thanTho(goiSua)).toContain(`"C1":${String(dong)}`);

      // 4. Máy chủ trả lại đúng số đồng đã lưu; màn hình in lại ĐÚNG chữ cán bộ đã gõ.
      batFetch(() => phanHoiDanhSach(String(dong)));
      const kq = await layDot("L1");
      expect(kq.ok).toBe(true);
      if (!kq.ok) return;
      const ve = kq.duLieu.entries[0]?.values["C1"] ?? null;
      expect(ve).toBe(dong);
      expect(nhanSoTien(ve, "trieu-dong")).toBe(go);
      expect(dongSangChuoi(dong, "trieu-dong")).toBe(go);
    });
  }

  it("máy chủ gửi một số VƯỢT trần chính xác của JS: màn hình nói 'Không đọc được', không in một số đã bị làm tròn", async () => {
    // `GiaTriToiDa` của máy chủ là 10^17 đồng, lớn hơn 2^53. `JSON.parse` của 9007199254740993 ra
    // 9007199254740992 — một số TRÔNG hợp lệ lệch một đồng. Nó không phải số nguyên an toàn, và đó
    // là thứ duy nhất màn hình còn dựa vào để không in nó ra như một con số thật.
    batFetch(() => phanHoiDanhSach("9007199254740993"));
    const kq = await layDot("L1");
    expect(kq.ok).toBe(true);
    if (!kq.ok) return;
    expect(nhanSoTien(kq.duLieu.entries[0]?.values["C1"] ?? null, "trieu-dong")).toBe(
      "Không đọc được",
    );
  });
});
