import { describe, expect, it } from "vitest";

import { COMPANY, CONTACT } from "../../content/company-profile";

import { docMaQR } from "./danh-thiep";
import {
  base64Utf8,
  dungVCard,
  thoatVCard,
  TRUONG_THIEP_CUA_CHUNG_TOI,
  vCardCuaChungToi,
} from "./vcard";

/**
 * BỘ SINH vCARD — PHẦN ĐÁNG KIỂM NHẤT CỦA BA TÍNH NĂNG THÊM VÀO, và vì một lý do rất cụ thể:
 *
 *   Chuỗi mà tệp này sinh ra đi vào ĐÚNG HAI CHỖ người ngoài cầm về — nội dung mã QR người khác
 *   quét, và một tệp ghi xuống máy họ. Cả hai đều rời khỏi tầm tay ta ngay khi phát ra. Một dấu
 *   chấm phẩy không thoát là tên công ty hiện sai trên danh bạ của một khách hàng; một lỗi mã
 *   hoá base64 là một tệp hỏng nằm trong máy họ. Không sửa lại được bằng một bản cập nhật.
 *
 *   Và nó kiểm được ĐẦY ĐỦ mà không cần một chiếc điện thoại nào, vì mọi hàm ở đây là hàm thuần:
 *   chuỗi vào, chuỗi ra, không tác dụng phụ.
 */

describe("thoát ký tự của vCard — RFC 6350 §3.4", () => {
  it("thoát dấu phẩy, dấu chấm phẩy và dấu gạch chéo ngược", () => {
    expect(thoatVCard("Công ty A, B và C")).toBe("Công ty A\\, B và C");
    expect(thoatVCard("Phòng Kinh doanh; Miền Nam")).toBe("Phòng Kinh doanh\\; Miền Nam");
    expect(thoatVCard("C:\\dulieu")).toBe("C:\\\\dulieu");
  });

  it("thoát gạch chéo ngược TRƯỚC, nếu không thì dấu thoát vừa thêm lại bị thoát lần nữa", () => {
    // Đây là cái bẫy thật của hàm này. Làm ngược thứ tự thì "A,B" ra "A\\\\,B" — trên máy người
    // nhận hiện thành `A\,B`, tức một gạch chéo thừa ngay giữa tên một pháp nhân.
    expect(thoatVCard("A\\B,C")).toBe("A\\\\B\\,C");
  });

  it("quy mọi kiểu xuống dòng về đúng một chuỗi thoát", () => {
    expect(thoatVCard("dòng một\ndòng hai")).toBe("dòng một\\ndòng hai");
    expect(thoatVCard("dòng một\r\ndòng hai")).toBe("dòng một\\ndòng hai");
    expect(thoatVCard("dòng một\rdòng hai")).toBe("dòng một\\ndòng hai");
  });

  it("để yên tiếng Việt có dấu — không có ký tự nào cần thoát trong đó", () => {
    expect(thoatVCard("Nguyễn Thị Hoà — Trưởng phòng")).toBe("Nguyễn Thị Hoà — Trưởng phòng");
  });

  it("để yên chuỗi không có gì phải thoát", () => {
    expect(thoatVCard("")).toBe("");
    expect(thoatVCard("https://vihatsoftware.com")).toBe("https://vihatsoftware.com");
  });
});

describe("dựng một vCard", () => {
  it("mở, khai phiên bản, và đóng — theo đúng thứ tự", () => {
    const dong = dungVCard([{ ten: "FN", gia_tri: "Công ty X" }]).split("\r\n");
    expect(dong[0]).toBe("BEGIN:VCARD");
    expect(dong[1]).toBe("VERSION:3.0");
    expect(dong[2]).toBe("FN:Công ty X");
    expect(dong[3]).toBe("END:VCARD");
  });

  it("kết dòng bằng CRLF và kết thúc bằng một lần xuống dòng", () => {
    const ra = dungVCard([{ ten: "FN", gia_tri: "X" }]);
    expect(ra.endsWith("END:VCARD\r\n")).toBe(true);
    expect(ra).not.toMatch(/[^\r]\n/);
  });

  it("BỎ HẲN trường rỗng, không sinh ra một dòng trống", () => {
    // Thiếu trường là chuyện bình thường: nhiều thứ trong `company-profile.ts` chưa có nguồn,
    // và một dòng `TITLE:` trống là một tuyên bố rằng ta có chức danh nhưng để trống.
    const ra = dungVCard([
      { ten: "FN", gia_tri: "Công ty X" },
      { ten: "TITLE", gia_tri: "" },
      { ten: "URL", gia_tri: "" },
    ]);
    expect(ra).not.toContain("TITLE");
    expect(ra).not.toContain("URL");
    expect(ra.split("\r\n").filter((d) => d !== "")).toHaveLength(4);
  });

  it("thoát GIÁ TRỊ nhưng KHÔNG thoát tên trường mang tham số", () => {
    // `TEL;TYPE=WORK,VOICE` có cả `;` và `,` trong phần TÊN. Chúng là cú pháp, không phải dữ
    // liệu: thoát chúng là biến một tham số hợp lệ thành một tên trường không ai hiểu.
    const ra = dungVCard([{ ten: "TEL;TYPE=WORK,VOICE", gia_tri: "Số nội bộ, máy lẻ 1" }]);
    expect(ra).toContain("TEL;TYPE=WORK,VOICE:Số nội bộ\\, máy lẻ 1");
  });

  it("giữ nguyên tiếng Việt có dấu trong giá trị", () => {
    expect(dungVCard([{ ten: "ORG", gia_tri: "Giải pháp Số Đông Á" }])).toContain(
      "ORG:Giải pháp Số Đông Á",
    );
  });
});

describe("tấm thiếp của chính chúng tôi", () => {
  const vcard = vCardCuaChungToi();

  it("mang đúng những gì COMPANY và CONTACT có", () => {
    expect(vcard).toContain(`FN:${COMPANY.name}`);
    expect(vcard).toContain(`ORG:${COMPANY.name}`);
    expect(vcard).toContain(`TEL;TYPE=WORK,VOICE:${CONTACT.hotlineDialable}`);
    expect(vcard).toContain(`EMAIL;TYPE=WORK:${CONTACT.email}`);
    // ⚠ `URL` CHỈ CÓ MẶT KHI CÓ ĐỊA CHỈ THẬT. `COMPANY.website` trống từ 21/09/2026 (địa chỉ cũ
    // là của VihatSoftware, địa chỉ tập đoàn chưa ai cấp), và `dungVCard` bỏ hẳn dòng của một
    // trường rỗng. Một dòng `URL:` trống đi vào danh bạ người khác là một tấm thiếp hỏng.
    if (COMPANY.website === undefined) {
      expect(vcard, "thiếp mang một dòng URL trong khi không có địa chỉ nào").not.toContain("URL:");
    } else {
      expect(vcard).toContain(`URL:${COMPANY.website}`);
    }
  });

  it("KHÔNG mang một trường nào ngoài những trường đã khai", () => {
    // Một địa chỉ, một mã số thuế hay một người đại diện thêm vào đây là một tuyên bố sai dưới
    // tên một pháp nhân có thật — và lần này là tuyên bố nằm trong danh bạ của người khác.
    // README §"Còn thiếu": hai trường ấy KHÔNG CÓ NGUỒN, nên chúng vắng mặt.
    const truong = vcard
      .split("\r\n")
      .filter((d) => d !== "")
      .map((d) => d.slice(0, d.indexOf(":")));
    expect(truong).toEqual([
      "BEGIN",
      "VERSION",
      // Trường rỗng không sinh ra dòng nào — cùng một luật với `dungVCard`, nên danh sách kỳ
      // vọng phải lọc y như thế thay vì liệt kê cứng.
      ...TRUONG_THIEP_CUA_CHUNG_TOI.filter((t) => t.gia_tri !== "").map((t) => t.ten),
      "END",
    ]);
    expect(vcard).not.toContain("ADR");
    expect(vcard).not.toContain("NOTE");
  });

  it("đọc ngược lại được bằng chính bộ bóc tách của ứng dụng", () => {
    // Vòng tròn khép kín: thứ ta phát ra phải là thứ ta đọc được. Nếu bộ bóc tách không nhận ra
    // tấm thiếp của chính mình thì nó cũng sẽ không nhận ra tấm thiếp của một đối tác dựng bằng
    // cùng một công cụ chuẩn.
    const doc = docMaQR(vcard);
    expect(doc.loai).toBe("danh-thiep");
    if (doc.loai !== "danh-thiep") return;
    expect(doc.ho_ten).toBe(COMPANY.name);
    expect(doc.to_chuc).toBe(COMPANY.name);
    expect(doc.dien_thoai).toEqual([CONTACT.hotlineDialable]);
    expect(doc.email).toEqual([CONTACT.email]);
    expect(doc.trang_web).toEqual(COMPANY.website === undefined ? [] : [COMPANY.website]);
  });
});

describe("mã hoá base64 cho downloadFile", () => {
  /** Giải mã ngược: base64 -> byte -> chuỗi UTF-8. Đúng việc nền tảng sẽ làm khi ghi tệp. */
  const giaiMa = (b64: string): string => {
    const nhi_phan = atob(b64);
    const byte = new Uint8Array(nhi_phan.length);
    for (let i = 0; i < nhi_phan.length; i += 1) byte[i] = nhi_phan.charCodeAt(i);
    return new TextDecoder().decode(byte);
  };

  it("giải ngược lại ĐÚNG NGUYÊN VĂN tấm thiếp, từng ký tự một", () => {
    // Phép kiểm quan trọng nhất của tệp này. Nội dung base64 là thứ duy nhất nền tảng nhận được;
    // sai một byte ở đây là một tệp hỏng nằm trên máy người dùng, và không có gì báo lỗi.
    const vcard = vCardCuaChungToi();
    expect(giaiMa(base64Utf8(vcard))).toBe(vcard);
  });

  it("qua được tiếng Việt có dấu — chỗ `btoa` trần sẽ ném lỗi", () => {
    const chu = "Nguyễn Văn An — Trưởng phòng Kinh doanh, Công ty TNHH Giải pháp Số";
    expect(() => base64Utf8(chu)).not.toThrow();
    expect(giaiMa(base64Utf8(chu))).toBe(chu);
  });

  it("qua được cả xuống dòng CRLF và chuỗi rỗng", () => {
    expect(giaiMa(base64Utf8("a\r\nb"))).toBe("a\r\nb");
    expect(base64Utf8("")).toBe("");
  });

  it("phát ra chuỗi base64 hợp lệ, không có ký tự nào ngoài bảng", () => {
    expect(base64Utf8(vCardCuaChungToi())).toMatch(/^[A-Za-z0-9+/]+=*$/);
  });
});
