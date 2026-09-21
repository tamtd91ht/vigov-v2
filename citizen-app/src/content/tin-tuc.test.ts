import { describe, expect, it } from "vitest";

import { GHI_CHU_TIN, NGAY_CHUP_TIN, ngayDoc, TIN_VIHAT } from "./tin-tuc";

/**
 * KHỐI TIN LÀ MỘT **BẢN CHỤP**, VÀ ĐÓ LÀ TOÀN BỘ THỨ CÁC CA DƯỚI ĐÂY CANH.
 *
 *   Một khối tin không nói ra rằng nó là bản chụp thì trông như tin trực tiếp. Người đọc không có
 *   cách nào biết mình đang xem một thứ đã cũ ba tháng, và một khối trang trí trở thành một lời
 *   nói sai dưới tên một pháp nhân có thật.
 *
 *   Cách hỏng rẻ nhất: ai đó cập nhật hai bài mà quên đổi `NGAY_CHUP_TIN`, hoặc ngược lại — đổi
 *   ngày mà không đổi bài. Ca "ngày chụp KHÔNG sớm hơn bài mới nhất" bắt được vế thứ nhất; ca
 *   "màn chủ in ra ngày chụp" (`screens.test.tsx`) bắt được việc câu ghi chú bị bỏ đi.
 */

describe("hai bài tin là một bản chụp có ngày, không phải một lời gọi mạng", () => {
  it("đúng hai bài, mỗi bài đủ bốn phần", () => {
    expect(TIN_VIHAT).toHaveLength(2);
    for (const bai of TIN_VIHAT) {
      expect(bai.ma.trim().length, "một bài không có mã").toBeGreaterThan(0);
      expect(bai.tieu_de.trim().length, `${bai.ma}: không có tiêu đề`).toBeGreaterThan(10);
      expect(bai.trich.trim().length, `${bai.ma}: không có đoạn trích`).toBeGreaterThan(40);
      expect(bai.ngay, `${bai.ma}: ngày không đúng dạng ISO`).toMatch(/^\d{4}-\d{2}-\d{2}$/);
    }
    const ma = TIN_VIHAT.map((b) => b.ma);
    expect(new Set(ma).size, "hai bài trùng mã — React sẽ vẽ sai khoá").toBe(ma.length);
  });

  it("mới nhất đứng trước", () => {
    const ngay = TIN_VIHAT.map((b) => b.ngay);
    expect([...ngay].sort().reverse(), "khối tin đang xếp ngược, bài cũ đứng trên").toEqual(ngay);
  });

  it("mỗi đường dẫn là https và trỏ về trang tin của chính chúng tôi", () => {
    // Một đường dẫn `http://` trong một app đã duyệt là một cảnh báo bảo mật trên máy người dùng;
    // một đường dẫn sang tên miền khác là một nơi nhận dữ liệu mà chính sách chưa khai.
    for (const bai of TIN_VIHAT) {
      expect(bai.duong_dan, `${bai.ma}: không phải https`).toMatch(/^https:\/\//);
      expect(bai.duong_dan, `${bai.ma}: không thuộc trang tin của chúng tôi`).toContain(
        "https://vihatgroup.com/",
      );
    }
  });

  it("ngày chụp KHÔNG sớm hơn bài mới nhất", () => {
    // Bản chụp ngày 21/09 mà chứa một bài đăng 01/10 là một trong hai thứ đã sai, và cả hai đều
    // là chuyện phải dừng lại: hoặc ngày chụp cũ, hoặc bài được gõ vào từ một chỗ khác.
    const [ngay, thang, nam] = NGAY_CHUP_TIN.split("/");
    expect(NGAY_CHUP_TIN, "ngày chụp không đúng dạng dd/mm/yyyy").toMatch(/^\d{2}\/\d{2}\/\d{4}$/);
    const chup_iso = `${nam}-${thang}-${ngay}`;
    for (const bai of TIN_VIHAT) {
      expect(bai.ngay <= chup_iso, `${bai.ma} đăng sau ngày chụp ${NGAY_CHUP_TIN}`).toBe(true);
    }
  });

  it("câu ghi chú nói ra NGÀY CHỤP và nói rõ app không tự tải tin mới", () => {
    expect(GHI_CHU_TIN).toContain(NGAY_CHUP_TIN);
    expect(GHI_CHU_TIN, "không nói ra rằng ứng dụng không tự tải tin").toMatch(
      /không tự tải tin mới/,
    );
  });

  it("đổi ngày ISO sang ngày Việt mà không đi qua múi giờ", () => {
    // `new Date("2026-07-20")` phân giải theo UTC rồi in theo múi giờ của MÁY — trên một máy ở
    // UTC-5 nó lùi thành 19/07. Ngày đăng là một nhãn, không phải một thời điểm.
    expect(ngayDoc("2026-07-20")).toBe("20/07/2026");
    expect(ngayDoc("2026-06-02")).toBe("02/06/2026");
    // Chuỗi lạ trả nguyên văn: một nhãn trông lạ vẫn đọc được, `Invalid Date` thì không.
    expect(ngayDoc("hôm qua")).toBe("hôm qua");
    expect(ngayDoc("")).toBe("");
  });
});

describe("khối tin không mang dữ liệu cá nhân, và tiếng Việt lưu đúng cách", () => {
  const MOI_CHUOI = TIN_VIHAT.flatMap((b) => [b.ma, b.ngay, b.tieu_de, b.trich, b.duong_dan]).concat(
    [NGAY_CHUP_TIN, GHI_CHU_TIN],
  );

  it("không chuỗi nào chứa số di động hay số định danh", () => {
    // Luật 3. Một bài viết chép về có thể mang theo một số hotline cá nhân trong đoạn trích, và
    // lúc ấy nó đi thẳng vào một bundle đã xuất bản.
    for (const chuoi of MOI_CHUOI) {
      expect(chuoi.replace(/\s/g, ""), `số trông như số di động trong: ${chuoi}`).not.toMatch(
        /(^|\D)0[35789]\d{8}(\D|$)/,
      );
      expect(chuoi.replace(/\s/g, ""), `dãy 12 chữ số trong: ${chuoi}`).not.toMatch(
        /(^|\D)\d{12}(\D|$)/,
      );
    }
  });

  it("mọi chuỗi ở dạng NFC và không có ký tự giải mã sai", () => {
    // Hai lỗi mã hoá không trình biên dịch nào thấy — cùng hai lỗi mà `company-profile.test.ts`
    // canh, và tệp này là một tệp nội dung thứ hai nên nó phải tự canh lấy.
    for (const chuoi of MOI_CHUOI) {
      expect(chuoi.normalize("NFC"), `tiếng Việt ở dạng tách dấu trong: ${chuoi}`).toBe(chuoi);
      expect(chuoi, `ký tự giải mã sai trong: ${chuoi}`).not.toMatch(/[ÃÄÆ][-¿]|â€|�/);
    }
  });
});
