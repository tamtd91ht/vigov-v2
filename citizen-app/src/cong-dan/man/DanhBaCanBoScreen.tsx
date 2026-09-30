/**
 * MÀN "DANH BẠ CÁN BỘ XÃ" — những cán bộ xã đã công khai, mỗi số điện thoại là một liên kết gọi.
 *
 * ⚠ CÔNG KHAI, KHÔNG CẦN PHIÊN (`api/goi-vigov.ts` `danhBaCanBoXa`). Cổng là TÊN MIỀN xã của lần mở
 *   này — màn chỉ được mở khi đã có nó (`KenhCongDan.tsx`).
 *
 * ⚠ SỐ DI ĐỘNG LÀ DỮ LIỆU CÁ NHÂN, công khai theo đồng ý từng người (#12). Nó chỉ được hiện ra và
 *   thành liên kết `tel:` — thứ mở màn quay số của máy, không đi qua mạng. Không log, không lưu, không
 *   gửi đi đâu, không đưa vào khoá hay tên tệp nào (luật 3).
 *
 * ⚠ "CÓ DÙNG ZALO" LÀ CHỮ, KHÔNG PHẢI MỘT BIỂU TƯỢNG MÀU (README §Non-negotiables #6), và chỉ hiện khi
 *   máy chủ nói có. Không có nút "nhắn Zalo": chưa có lời gọi nền tảng nào cho việc ấy ở nửa này.
 */
import { useEffect, useRef, useState } from "react";

import { danhBaCanBoXa, type KetQuaCongKhai } from "../api/goi-vigov";
import type { CanBoCongKhai } from "../api/hop-dong-cong-khai";
import { layPhienViGov } from "../api/phien-vigov";

import { BangXa } from "./khung";
import { DANH_BA, QUAY_LAI } from "./noi-dung";

export type LoiDanhBa = "loi-mang" | "loi-may-chu" | "khong-hop-le";

export type TrangDanhBa =
  | { readonly kieu: "dang-tai" }
  | { readonly kieu: "xong"; readonly can_bo: readonly CanBoCongKhai[] }
  | { readonly kieu: "loi"; readonly loi: LoiDanhBa };

export function sauKhiTaiDanhBa(kq: KetQuaCongKhai<readonly CanBoCongKhai[]>): TrangDanhBa {
  switch (kq.kieu) {
    case "xong":
      return { kieu: "xong", can_bo: kq.gia_tri };
    case "loi-mang":
      return { kieu: "loi", loi: "loi-mang" };
    case "khong-hop-le":
      return { kieu: "loi", loi: "khong-hop-le" };
    default:
      return { kieu: "loi", loi: "loi-may-chu" };
  }
}

const CAU_LOI: Readonly<Record<LoiDanhBa, string>> = {
  "loi-mang": DANH_BA.loi_mang,
  "loi-may-chu": DANH_BA.loi_may_chu,
  "khong-hop-le": DANH_BA.khong_hop_le,
};

/**
 * Số điện thoại → đích `tel:`, hoặc `null` khi không còn chữ số nào để quay.
 *
 * GIỮ CHỮ SỐ VÀ MỘT DẤU `+` ĐẦU, BỎ MỌI THỨ KHÁC: số do xã nhập có thể mang dấu cách, chấm, gạch
 * (dạng "0900.000 000"). Một `tel:` mang ký tự lạ là liên kết máy quay số từ chối — và người lớn tuổi
 * bấm một số không gọi được sẽ không bấm lần hai.
 */
export function dichGoi(so: string): string | null {
  const gon = so.trim();
  const chu_so = gon.replace(/[^\d]/g, "");
  if (chu_so.length < 3) return null;
  return `tel:${gon.startsWith("+") ? "+" : ""}${chu_so}`;
}

function DongSo({ nhan, so }: { nhan: string; so: string }) {
  if (so.trim() === "") return null;
  const dich = dichGoi(so);
  return (
    <div className="cd-can-bo__so">
      <span className="cd-can-bo__nhan">{nhan}</span>
      {dich === null ? (
        <span>{so}</span>
      ) : (
        <a className="cd-goi" href={dich}>
          {DANH_BA.goi(so.trim())}
        </a>
      )}
    </div>
  );
}

export function TheCanBo({ cb }: { cb: CanBoCongKhai }) {
  return (
    <li className="cd-can-bo">
      <p className="cd-can-bo__ten">{cb.ho_ten}</p>
      {cb.chuc_vu !== "" && (
        <p className="cd-can-bo__dong">
          {DANH_BA.chuc_vu}: {cb.chuc_vu}
        </p>
      )}
      {cb.bo_phan !== "" && (
        <p className="cd-can-bo__dong">
          {DANH_BA.bo_phan}: {cb.bo_phan}
        </p>
      )}
      <DongSo nhan={DANH_BA.so_co_quan} so={cb.so_co_quan} />
      <DongSo nhan={DANH_BA.di_dong} so={cb.di_dong} />
      {cb.co_zalo && <p className="cd-can-bo__dong cd-can-bo__zalo">{DANH_BA.co_zalo}</p>}
    </li>
  );
}

export function ThanDanhBa(props: { trang: TrangDanhBa; onTai: () => void }) {
  const { trang } = props;
  if (trang.kieu === "dang-tai") {
    return (
      <p className="cd-cau" role="status">
        {DANH_BA.dang_tai}
      </p>
    );
  }
  if (trang.kieu === "loi") {
    return (
      <div className="cd-buoc">
        <p className="cd-loi" role="alert">
          {CAU_LOI[trang.loi]}
        </p>
        {trang.loi !== "khong-hop-le" && (
          <button type="button" className="cd-nut" onClick={props.onTai}>
            {DANH_BA.nut_thu_lai}
          </button>
        )}
      </div>
    );
  }
  if (trang.can_bo.length === 0) return <p className="cd-cau">{DANH_BA.trong}</p>;
  return (
    <>
      <p className="cd-cau">{DANH_BA.gioi_thieu}</p>
      <ul className="cd-cua-toi">
        {trang.can_bo.map((cb, i) => (
          // Không có mã nào trong hợp đồng (cố ý: danh bạ công khai không lộ mã nội bộ), nên khoá là
          // vị trí. Danh sách không sắp lại phía client, nên vị trí đứng yên suốt một lần xem.
          <TheCanBo key={i} cb={cb} />
        ))}
      </ul>
    </>
  );
}

export function DanhBaCanBoScreen(props: { ten_mien: string; onQuayLai: () => void }) {
  const [ten_xa] = useState(() => layPhienViGov()?.ten_xa ?? null);
  const [trang, datTrang] = useState<TrangDanhBa>({ kieu: "dang-tai" });
  const da_tai = useRef(false);

  async function tai() {
    datTrang({ kieu: "dang-tai" });
    datTrang(sauKhiTaiDanhBa(await danhBaCanBoXa(props.ten_mien)));
  }

  useEffect(() => {
    if (da_tai.current) return;
    da_tai.current = true;
    void tai();
  }, []);

  return (
    <section className="cd-man" aria-label={DANH_BA.tieu_de}>
      <button type="button" className="quay-lai" onClick={props.onQuayLai}>
        {QUAY_LAI}
      </button>
      {ten_xa !== null && <BangXa ten_xa={ten_xa} />}
      <h1 className="cd-tieu-de">{DANH_BA.tieu_de}</h1>
      <ThanDanhBa trang={trang} onTai={() => void tai()} />
    </section>
  );
}
