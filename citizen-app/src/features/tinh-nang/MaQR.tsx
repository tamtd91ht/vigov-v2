/**
 * VẼ MỘT MÃ QR BẰNG SVG NỘI TUYẾN.
 *
 * VÌ SAO CÓ MỘT PHỤ THUỘC MỚI Ở ĐÂY, VÀ VÌ SAO CHỈ MỘT:
 *
 *   Sinh mã QR đúng gồm chọn chế độ mã hoá, chọn phiên bản, chèn mã sửa lỗi Reed–Solomon, rồi
 *   chọn mặt nạ. Sai MỘT BIT trong khối sửa lỗi là một mã trông hoàn hảo trên màn hình và không
 *   máy nào quét nổi — một lỗi mà mắt không bắt được và một phép kiểm dựng bằng chính mã sai ấy
 *   cũng không bắt được. Đó là lý do bộ mã hoá ở đây là `uqr` (MIT, không phụ thuộc gì khác,
 *   nằm trong bundle — KHÔNG tải từ CDN, vì một Mini App kéo mã từ bên thứ ba là một câu hỏi ở
 *   vòng duyệt), chứ không phải một bản tự viết.
 *
 * VÌ SAO MỘT `<path>` CHỨ KHÔNG PHẢI MỘT `<rect>` MỖI Ô: một mã cỡ trung bình có hơn một nghìn
 * ô tối. Nghìn phần tử DOM trên một máy Android yếu là một màn hình giật; một chuỗi `d` thì
 * trình duyệt vẽ trong một nhịp.
 *
 * ⚠ `aria-hidden` VÀ CHỮ ĐI KÈM. Mã QR là một hình — trình đọc màn hình không đọc được nó, và
 * một `<title>` mô tả "mã QR" cũng chẳng giúp gì. Nên mọi thông tin trong mã đều được hiện lại
 * bằng CHỮ ngay dưới mã (`ManDanhThiepCuaChungToi`). Người khiếm thị đọc được đúng những gì
 * người sáng mắt quét được — đó là điều kiện, không phải một bổ sung.
 */
import { encode } from "uqr";

/**
 * Chuyển ma trận ô tối thành một chuỗi `d` của SVG.
 *
 * Mỗi ô tối là một hình vuông 1×1 ở toạ độ nguyên, nên `shape-rendering: crispEdges` vẽ chúng
 * không bị nhoè — và một cạnh nhoè giữa hai ô là đúng thứ làm máy quét đọc sai.
 */
export function duongDanMaQR(o_toi: readonly (readonly boolean[])[]): string {
  const doan: string[] = [];
  for (let hang = 0; hang < o_toi.length; hang += 1) {
    const dong = o_toi[hang] ?? [];
    for (let cot = 0; cot < dong.length; cot += 1) {
      if (dong[cot] === true) doan.push(`M${cot} ${hang}h1v1h-1z`);
    }
  }
  return doan.join("");
}

/**
 * Mã QR của một chuỗi.
 *
 * `ecc: "M"` — mức sửa lỗi trung bình, phục hồi được khoảng 15% dữ liệu hỏng. Mã này được quét
 * từ một màn hình điện thoại đang cầm trên tay, có bóng và có rung, nên mức thấp nhất (`L`, mặc
 * định của thư viện) là mức sai chỗ; còn mức cao nhất (`H`) làm mã dày thêm mà không giải quyết
 * vấn đề nào ở khoảng cách một sải tay.
 *
 * `border: 2` — vùng yên tĩnh quanh mã. Chuẩn đòi 4 ô; ở đây tấm thẻ trắng bao quanh mã đã là
 * phần còn lại của vùng ấy, nên 2 ô trong SVG cộng với lề của thẻ là đủ, và mã không bị thu nhỏ
 * quá mức trên một màn hình 320px.
 */
export function MaQR({ noi_dung, className }: { noi_dung: string; className?: string }) {
  const ma = encode(noi_dung, { ecc: "M", border: 2 });
  const canh = ma.data.length;

  return (
    <svg
      className={className}
      viewBox={`0 0 ${canh} ${canh}`}
      shapeRendering="crispEdges"
      aria-hidden="true"
      focusable="false"
      xmlns="http://www.w3.org/2000/svg"
    >
      <rect className="ma-qr__nen" x="0" y="0" width={canh} height={canh} />
      <path className="ma-qr__o" d={duongDanMaQR(ma.data)} />
    </svg>
  );
}
