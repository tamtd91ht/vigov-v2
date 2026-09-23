"use client";

import Link from "next/link";
import { usePathname } from "next/navigation";
import { useSyncExternalStore } from "react";

import { usePhien } from "@/features/phien/phien-hien-tai";

import { CHUA_CO_MAN, dangChon, locMenu, NHOM_MENU } from "./muc-menu";

/**
 * Thanh bên — `docs/ui-ux/15-phu-luc-giao-dien-chung.md` §2.
 *
 * VÌ SAO THANH NÀY RA ĐỜI MUỘN HƠN NĂM MÀN NÓ TRỎ TỚI: cho tới 23/09/2026 kho này không có
 * component điều hướng nào, nên `/danh-ba`, `/van-ban`, `/giai-ngan`, `/phan-anh` và `/cau-hinh`
 * **chỉ tới được bằng cách gõ đường dẫn**. Năm màn đã dựng xong mà không ai bấm tới được.
 *
 * QUYẾT ĐỊNH VÀ LỌC nằm ở `muc-menu.ts`, không ở đây — xem khối chú thích đầu tệp ấy để biết vì
 * sao chín mục chưa có màn vẫn hiện, và vì sao chúng không phải liên kết.
 *
 * TRẠNG THÁI THU GỌN đọc/ghi `localStorage` qua `useSyncExternalStore`, KHÔNG qua
 * `useState` + `useEffect`. Ba lý do, và lý do thứ ba là lý do bắt buộc:
 *   1. máy chủ không có `localStorage`, nên đọc lúc dựng lần đầu sẽ làm bản kết xuất ở máy chủ
 *      khác bản ở trình duyệt và React kêu hydrate;
 *   2. `localStorage` là một kho NGOÀI React — `useSyncExternalStore` sinh ra đúng cho nó, và
 *      nó nhận luôn thay đổi từ TAB KHÁC qua sự kiện `storage`;
 *   3. **React Compiler đang bật ở kho này và `setState` trong `useEffect` bị cấm**
 *      (`react-hooks/set-state-in-effect`, và `docs/ui-ux` ghi cùng luật). Bản đầu của tệp này
 *      viết đúng lối bị cấm ấy và eslint chặn.
 *
 * CHÂN THANH BÊN (phiên bản + môi trường) CHƯA DỰNG, có chủ ý. Đặc tả §2 nói lấy từ biến môi
 * trường, mà một biến `NEXT_PUBLIC_*` được nung vào bundle lúc build (luật 8, bất biến 4) và
 * chưa có dòng nào cho nó trong `.env.example` (luật 11, cấm #4). Khai một biến chỉ để in một
 * dòng chữ là mở một cửa mà `env_contract_guard` tồn tại để đóng. Cùng lý do `dau-trang.tsx`
 * chưa vẽ ô tìm kiếm.
 */

const KHOA_THU_GON = "vigov.thanh-ben.thu-gon";

/** Người nghe của chính tab này — `storage` chỉ bắn sang tab KHÁC, không bắn cho tab vừa ghi. */
const nguoiNghe = new Set<() => void>();

function dangKy(bao: () => void) {
  nguoiNghe.add(bao);
  window.addEventListener("storage", bao);
  return () => {
    nguoiNghe.delete(bao);
    window.removeEventListener("storage", bao);
  };
}

function docThuGon(): boolean {
  try {
    return window.localStorage.getItem(KHOA_THU_GON) === "1";
  } catch {
    // Cửa sổ riêng tư, hoặc trình duyệt chặn lưu trữ. Thanh bên ở dạng mở rộng vẫn dùng được
    // bình thường, nên không có gì để báo cho người dùng.
    return false;
  }
}

/** Ở máy chủ luôn là dạng mở rộng — không có `localStorage` để mà đọc. */
function docThuGonOMayChu(): boolean {
  return false;
}

export function ThanhBen() {
  const phien = usePhien();
  const duongHienTai = usePathname() ?? "/";
  const thuGon = useSyncExternalStore(dangKy, docThuGon, docThuGonOMayChu);

  function doiThuGon() {
    try {
      window.localStorage.setItem(KHOA_THU_GON, thuGon ? "0" : "1");
    } catch {
      // Không lưu được thì thanh bên không đổi — và đó là hành vi TRUNG THỰC: trạng thái hiển
      // thị luôn bằng đúng thứ đang nằm trong kho. Giữ một trạng thái trong React song song với
      // kho sẽ cho người dùng thấy thanh đã thu trong khi lần mở sau nó bung ra như cũ.
    }
    for (const bao of nguoiNghe) bao();
  }

  // `null` là CHƯA ĐỌC XONG phiên, không phải "không có quyền" — ba trạng thái, không hai.
  const dsQuyen = phien === null ? null : phien.ok ? phien.duLieu.permissions : [];
  const nhom = locMenu(NHOM_MENU, dsQuyen);

  return (
    <nav className={thuGon ? "thanh-ben thu-gon" : "thanh-ben"} aria-label="Điều hướng chính">
      <div className="thanh-ben-dinh">
        <span className="thanh-ben-dau" aria-hidden="true">
          VG
        </span>
        <span className="thanh-ben-ten">
          <span className="thanh-ben-san-pham">ViGov</span>
          <span className="thanh-ben-phu">ĐIỀU HÀNH SỐ CẤP XÃ</span>
        </span>
        <button
          type="button"
          className="thanh-ben-nut-thu-gon"
          onClick={doiThuGon}
          aria-expanded={!thuGon}
          // Nhãn nói HÀNH ĐỘNG SẼ XẢY RA, không nói trạng thái hiện tại: người dùng trình đọc
          // màn hình nghe "Thu gọn menu" thì biết bấm vào sẽ thu gọn.
        >
          {thuGon ? "Mở rộng menu" : "Thu gọn menu"}
        </button>
      </div>

      {nhom.map((n) => (
        <div key={n.ten} className="thanh-ben-nhom">
          <p className="thanh-ben-nhan-nhom">{n.ten}</p>
          <ul>
            {n.muc.map((m) =>
              m.duong === null ? (
                // KHÔNG PHẢI LIÊN KẾT, và không phải `<button disabled>`: cả hai đều nhận được
                // tiêu điểm bàn phím rồi không làm gì, tức bắt người dùng bàn phím đi qua chín
                // chặng chết để tới mục dùng được. Một `<span>` mang `aria-disabled` nằm ngoài
                // thứ tự tiêu điểm và vẫn được trình đọc màn hình đọc đúng.
                <li key={m.nhan} className="thanh-ben-muc chua-co">
                  <span aria-disabled="true" title={CHUA_CO_MAN}>
                    {m.nhan}
                  </span>
                  <span className="thanh-ben-dau-chua-co">{CHUA_CO_MAN}</span>
                </li>
              ) : (
                <li key={m.nhan} className="thanh-ben-muc">
                  <Link
                    href={m.duong}
                    aria-current={dangChon(m.duong, duongHienTai) ? "page" : undefined}
                    className={dangChon(m.duong, duongHienTai) ? "dang-chon" : undefined}
                  >
                    {m.nhan}
                  </Link>
                </li>
              ),
            )}
          </ul>
        </div>
      ))}
    </nav>
  );
}
