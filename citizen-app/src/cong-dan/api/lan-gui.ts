/**
 * MỘT LẦN GỬI — thân yêu cầu và khoá chống gửi trùng (`Idempotency-Key`) đi cùng nhau.
 *
 * VÌ SAO GẮN KHOÁ VỚI THÂN, không sinh khoá mỗi lần bấm:
 *
 *   Tuyến gửi phiếu có `idem.Required` và KHÔNG có lớp chống trùng thứ hai (`routes_cong_dan.go`:
 *   hai phiếu cùng nội dung là hai dòng khác nhau, hai mã tra cứu khác nhau). Mạng nông thôn rớt
 *   giữa chừng là chuyện thường: yêu cầu tới máy chủ, phiếu đã vào sổ, trả lời không về. Bấm "Gửi
 *   lại" với khoá MỚI là một phiếu thứ hai — mà luật 7 làm nó vĩnh viễn (chỉ xoá mềm, mã đã cấp
 *   không cấp lại). Bấm lại với CÙNG khoá thì máy chủ trả lại đúng 201 và mã tra cứu của lần đầu.
 *
 *   Ngược lại, người dân SỬA nội dung rồi gửi là một lần gửi MỚI, và phải có khoá mới: cùng khoá với
 *   thân khác là yêu cầu máy chủ coi hai thứ khác nhau là một.
 *
 * Nên màn hình giữ một `LanGui` trong `useState`: tạo khi bấm Gửi lần đầu, dùng lại khi "Gửi lại",
 * bỏ đi khi người dân quay lại sửa. Không ghi xuống máy (`ranh-gioi-hai-nua.test.ts` §3b).
 */
export type LanGui = {
  readonly khoa: string;
  readonly than: string;
};

/**
 * Khoá ngẫu nhiên 128 bit, dạng hex.
 *
 * `crypto.getRandomValues` chứ không `Math.random`: hai máy cùng sinh một khoá thì phạm vi khoá
 * theo công dân của máy chủ vẫn tách chúng, nhưng một khoá đoán được là một khoá không nên có.
 * Không có `crypto` thì NÉM — màn hình bắt và nói "chưa gửi được", thay vì gửi không khoá.
 */
function khoaNgauNhien(): string {
  const byte = new Uint8Array(16);
  globalThis.crypto.getRandomValues(byte);
  return Array.from(byte, (b) => b.toString(16).padStart(2, "0")).join("");
}

export function taoLanGui(than: string): LanGui {
  return { khoa: khoaNgauNhien(), than };
}
