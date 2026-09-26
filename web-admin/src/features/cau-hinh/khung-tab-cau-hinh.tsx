"use client";

import { useRef, useState, type KeyboardEvent, type ReactNode } from "react";

import { usePhien } from "@/features/phien/phien-hien-tai";

import { TabDanhMuc } from "./tab-danh-muc";
import { TabNguoiDung } from "./tab-nguoi-dung";
import { TabPhanQuyen } from "./tab-phan-quyen";
import { TabSoDoToChuc } from "./tab-so-do-to-chuc";
import { TabThoiHanXuLy } from "./tab-thoi-han-xu-ly";
import { TabThonToDanPho } from "./tab-thon-to-dan-pho";
import {
  cacTabHien,
  coThanhTab,
  TAB_CAU_HINH,
  tabKeTheoPhim,
  type MaTabCauHinh,
} from "./thanh-tab-cau-hinh";

const NOI_DUNG: Record<MaTabCauHinh, () => ReactNode> = {
  "so-do-to-chuc": () => <TabSoDoToChuc />,
  "thon-to-dan-pho": () => <TabThonToDanPho />,
  "nguoi-dung": () => <TabNguoiDung />,
  "phan-quyen": () => <TabPhanQuyen />,
  "danh-muc": () => <TabDanhMuc />,
  "thoi-han-xu-ly": () => <TabThoiHanXuLy />,
};

const idTab = (ma: MaTabCauHinh) => `tab-cau-hinh-${ma}`;
const idPanel = (ma: MaTabCauHinh) => `panel-cau-hinh-${ma}`;

/**
 * Thanh tab + các panel của màn Cấu hình. Quyết định tab nào hiện và có thanh hay không nằm ở
 * `thanh-tab-cau-hinh.ts` (có bài test riêng); ở đây chỉ dựng.
 *
 * PANEL KHÔNG ĐƯỢC CHỌN VẪN ĐƯỢC GIỮ (`hidden`), không gỡ khỏi cây: một cột Phân quyền đang sửa dở
 * hay một biểu mẫu nhập nửa chừng mà mất chỉ vì bấm sang tab khác là mất việc của cán bộ. Cái giá là
 * mọi tab được phép đều đọc dữ liệu của nó ngay khi mở màn — đúng như khi sáu phần còn dựng nối tiếp.
 *
 * Tab đang chọn chỉ nằm trong state của component: không ghi `localStorage`, không đồng bộ URL.
 */
export function KhungTabCauHinh() {
  const phien = usePhien();
  const hien = cacTabHien(TAB_CAU_HINH, phien);
  const coThanh = coThanhTab(phien, hien.length);

  const [dangChon, datDangChon] = useState<MaTabCauHinh>("so-do-to-chuc");
  const nutTab = useRef<Partial<Record<MaTabCauHinh, HTMLButtonElement | null>>>({});

  // Tab đã chọn mà nay không còn trong danh sách được hiện thì về tab đầu tiên được hiện — không
  // bao giờ để một panel bị ẩn mà không panel nào hiện thay.
  const chon: MaTabCauHinh | undefined = hien.some((t) => t.ma === dangChon)
    ? dangChon
    : hien[0]?.ma;

  function xuLyPhim(e: KeyboardEvent<HTMLButtonElement>, viTri: number) {
    const ke = tabKeTheoPhim(e.key, viTri, hien.length);
    const toi = ke === null ? undefined : hien[ke];
    if (toi === undefined) return;
    e.preventDefault();
    const ma = toi.ma;
    datDangChon(ma);
    nutTab.current[ma]?.focus();
  }

  return (
    <>
      {/* Phiên đọc hỏng thì hai tab có cổng bị ẩn (đóng khi không chắc). Câu của máy chủ — thường là
          "phiên đã hết hạn" — phải còn ra tới màn hình, như khi hai phần ấy tự hiện nó. */}
      {phien !== null && !phien.ok && (
        <p className="thong-bao-loi" role="alert">
          {phien.thongBao}
        </p>
      )}

      {coThanh && (
        <div role="tablist" aria-label="Các phần cấu hình" className="thanh-tab-cau-hinh">
          {hien.map((t, i) => (
            <button
              key={t.ma}
              ref={(el) => {
                nutTab.current[t.ma] = el;
              }}
              type="button"
              role="tab"
              id={idTab(t.ma)}
              aria-selected={t.ma === chon}
              aria-controls={idPanel(t.ma)}
              tabIndex={t.ma === chon ? 0 : -1}
              onClick={() => datDangChon(t.ma)}
              onKeyDown={(e) => xuLyPhim(e, i)}
            >
              {t.nhan}
            </button>
          ))}
        </div>
      )}

      {/* Cùng một danh sách có `key` ở cả hai trạng thái (có thanh / không thanh), nên khi thanh
          xuất hiện React giữ nguyên các panel đã dựng — không đọc lại, không mất gì đang nhập. */}
      {hien.map((t) => (
        <div
          key={t.ma}
          id={idPanel(t.ma)}
          className="panel-cau-hinh"
          hidden={t.ma !== chon}
          {...(coThanh
            ? { role: "tabpanel", "aria-labelledby": idTab(t.ma), tabIndex: 0 }
            : {})}
        >
          {NOI_DUNG[t.ma]()}
        </div>
      ))}
    </>
  );
}
