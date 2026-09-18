/**
 * Bảng chẩn đoán: in nguyên si những gì nền tảng đưa vào lúc mở app.
 *
 * KHÔNG PHẢI MỘT TÍNH NĂNG. Nó tồn tại để trả lời đúng một câu còn treo của ADR 0018 — tham số
 * deep link có tới app trong mọi đường mở không, và có giữ khi app đang chạy nền không — bằng
 * cách đo trên máy thật thay vì suy đoán tiếp.
 *
 * CÁCH ĐO:
 *
 *   1. Quét QR / mở link `https://zalo.me/s/<APP_ID>/?debug=1&t=THU&src=qr&v=1` → xem bảng này
 *      liệt kê đủ ba tham số không.
 *   2. Bấm nút Home, mở lại app từ danh sách app đang chạy → **số lần vẽ tăng, tham số có còn
 *      nguyên không**. Đây là nửa câu hỏi chưa ai trả lời.
 *   3. Đóng hẳn app rồi mở lại từ danh sách app ghim → dự kiến KHÔNG có tham số, và đó là đường
 *      phổ biến nhất của người dân từ lần mở thứ hai (`skills/zalo-miniapp-multi-tenant`).
 *
 * Xoá được toàn bộ khi câu hỏi đã có đáp: một tệp, một nhánh trong App.tsx, một dòng trong
 * launch-params.ts.
 */
import { useEffect, useRef, useState } from "react";

import type { KetQuaDo } from "../../lib/launch-params";

type Props = { thamSo: KetQuaDo };

function Nguon({ nhan, cap }: { nhan: string; cap: [string, string][] }) {
  return (
    <div className="chan-doan__nguon">
      <p className="chan-doan__nhan-nguon">{nhan}</p>
      {cap.length === 0 ? (
        <p className="chan-doan__trong">Không nhận được tham số nào.</p>
      ) : (
        <dl className="chan-doan__bang">
          {cap.map(([khoa, gia_tri]) => (
            <div className="chan-doan__dong" key={khoa}>
              <dt>{khoa}</dt>
              <dd>{gia_tri === "" ? "(rỗng)" : gia_tri}</dd>
            </div>
          ))}
        </dl>
      )}
    </div>
  );
}

export function LaunchParamsPanel({ thamSo }: Props) {
  const capSdk = Object.entries(thamSo.sdk);
  const capUrl = Object.entries(thamSo.url);

  // Số lần vẽ + thời điểm: đây là thứ phân biệt "mở nguội" với "quay lại app đang chạy nền".
  // Không có hai con số này thì bước 2 ở trên không đọc được kết quả — hai lần mở trông y hệt.
  const soLanVe = useRef(0);
  soLanVe.current += 1;
  const [luc] = useState(() => new Date().toLocaleTimeString("vi-VN"));
  const [quayLai, setQuayLai] = useState(0);

  useEffect(() => {
    // `visibilitychange` là tín hiệu gần nhất với "người dùng quay lại app" mà không cần SDK.
    const khi = () => {
      if (document.visibilityState === "visible") setQuayLai((n) => n + 1);
    };
    document.addEventListener("visibilitychange", khi);
    return () => document.removeEventListener("visibilitychange", khi);
  }, []);

  return (
    <section className="chan-doan" aria-label="Thông tin chẩn đoán">
      <p className="chan-doan__nhan">Chẩn đoán tham số mở app</p>

      {/* HAI NGUỒN cạnh nhau. Nếu cột dưới đủ như cột trên thì lớp khám phá của ADR 0005 chạy
          được mà không cần zmp-sdk — và đó là 64 kB gzip không phải tải, trên mạng di động của
          một người dùng ở xã. Xem lý do đầy đủ trong lib/launch-params.ts. */}
      <Nguon nhan="Theo SDK (getRouteParams)" cap={capSdk} />
      <Nguon nhan="Theo URL (location.search)" cap={capUrl} />

      <p className="chan-doan__do">
        Vẽ lần {soLanVe.current} · lúc {luc} · quay lại {quayLai} lần
      </p>
    </section>
  );
}
