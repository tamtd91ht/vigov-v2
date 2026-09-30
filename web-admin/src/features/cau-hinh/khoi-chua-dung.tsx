import { PHAN_CHUA_DUNG } from "./nhan-cau-hinh";

/**
 * Khối "phần chưa dựng" ở cuối màn Cấu hình — cùng khuôn `<details className="khoi-chua-khai">`
 * của màn Phản ánh và màn Nội dung. Không gọi mạng, không cổng quyền: danh sách này nói về phần
 * mềm, không về dữ liệu của đơn vị.
 */
export function KhoiChuaDung() {
  return (
    <details className="khoi-chua-khai">
      <summary>
        {PHAN_CHUA_DUNG.length} phần của bản thiết kế chưa dựng được — bấm để xem từng phần và lý do
      </summary>
      <dl className="danh-sach-truong">
        {PHAN_CHUA_DUNG.map((p) => (
          <div key={p.ten}>
            <dt>{p.ten}</dt>
            <dd>{p.viSao}</dd>
          </div>
        ))}
      </dl>
    </details>
  );
}
