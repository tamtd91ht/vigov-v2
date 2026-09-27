/**
 * DANH BẠ CÁN BỘ XÃ — giao diện theo bản mẫu `vi-gov/zalo-miniapp` (`features/directory/DirectoryPage`):
 * ô tìm, mỗi cán bộ một thẻ có ô chữ cái đầu, nút gọi tròn bên phải.
 *
 * DỮ LIỆU VÀ QUY TẮC GIỮ NGUYÊN của `DanhBaCanBoScreen.tsx`: chỉ người đã đồng ý công khai (#12), số
 * gọi qua `tel:` (`dichGoi`) — mở màn quay số của máy, không đi qua mạng, không log, không lưu. Tìm
 * kiếm lọc NGAY TRÊN MÁY trong danh sách đã tải; từ khoá không gửi đi đâu.
 *
 * Không có nút "nhắn Zalo" như bản mẫu: chưa có lời gọi nền tảng nào cho việc ấy ở nửa này.
 */
import { useEffect, useMemo, useRef, useState } from "react";

import { danhBaCanBoXa } from "../api/goi-vigov";
import type { CanBoCongKhai } from "../api/hop-dong-cong-khai";
import { dichGoi, sauKhiTaiDanhBa, type TrangDanhBa } from "./DanhBaCanBoScreen";
import { DANH_BA, XA_GIAO_DIEN } from "./noi-dung";

import { BieuTuong } from "./BieuTuong";
import { KhoiTrangThai } from "./khung-xa";
import { ONhapDong } from "./o-nhap";

/** Chữ cái đầu của TÊN GỌI (từ cuối) — quy ước của bản mẫu. */
function chuDau(ho_ten: string): string {
  const cuoi = ho_ten.trim().split(/\s+/).pop() ?? "";
  return (cuoi[0] ?? "?").toUpperCase();
}

/** Lọc trên máy, không phân biệt hoa thường. Không dấu hiệu nào của từ khoá rời khỏi máy. */
export function locCanBo(ds: readonly CanBoCongKhai[], tu_khoa: string): readonly CanBoCongKhai[] {
  const q = tu_khoa.trim().toLocaleLowerCase("vi");
  if (q === "") return ds;
  return ds.filter((cb) =>
    [cb.ho_ten, cb.chuc_vu, cb.bo_phan].some((o) => o.toLocaleLowerCase("vi").includes(q)),
  );
}

function TheCanBoXa({ cb }: { cb: CanBoCongKhai }) {
  // Số cơ quan trước: công khai theo bản chất; di động chỉ khi người ấy đồng ý (#12).
  const so = cb.so_co_quan.trim() !== "" ? cb.so_co_quan : cb.di_dong;
  const dich = so.trim() === "" ? null : dichGoi(so);
  return (
    <li className="xa-the xa-can-bo">
      <span className="xa-can-bo__chu-dau" aria-hidden="true">
        {chuDau(cb.ho_ten)}
      </span>
      <span className="xa-can-bo__chu">
        <strong className="xa-can-bo__ten">{cb.ho_ten}</strong>
        {cb.chuc_vu !== "" && <span>{cb.chuc_vu}</span>}
        {cb.bo_phan !== "" && <span className="xa-phu">{cb.bo_phan}</span>}
        {cb.so_co_quan.trim() !== "" && (
          <span className="xa-phu">
            {DANH_BA.so_co_quan}: {cb.so_co_quan.trim()}
          </span>
        )}
        {cb.di_dong.trim() !== "" && (
          <span className="xa-phu">
            {DANH_BA.di_dong}: {cb.di_dong.trim()}
          </span>
        )}
        {cb.co_zalo && <span className="xa-phu">{DANH_BA.co_zalo}</span>}
      </span>
      {dich !== null && (
        <a className="xa-can-bo__goi" href={dich} aria-label={XA_GIAO_DIEN.goi_ai(cb.ho_ten)}>
          <BieuTuong ten="phone" co={22} />
          <span>{XA_GIAO_DIEN.goi}</span>
        </a>
      )}
    </li>
  );
}

const CAU_LOI = {
  "loi-mang": DANH_BA.loi_mang,
  "loi-may-chu": DANH_BA.loi_may_chu,
  "khong-hop-le": DANH_BA.khong_hop_le,
} as const;

export function ThanDanhBaXa(props: { trang: TrangDanhBa; onTai: () => void }) {
  const [tu_khoa, datTuKhoa] = useState("");
  const { trang } = props;
  const loc = useMemo(
    () => (trang.kieu === "xong" ? locCanBo(trang.can_bo, tu_khoa) : []),
    [trang, tu_khoa],
  );

  if (trang.kieu === "dang-tai") return <KhoiTrangThai bieu_tuong="users" cau={DANH_BA.dang_tai} dang_tai />;
  if (trang.kieu === "loi") {
    return (
      <KhoiTrangThai
        bieu_tuong="alert"
        loi
        cau={CAU_LOI[trang.loi]}
        nut={trang.loi !== "khong-hop-le" ? { nhan: DANH_BA.nut_thu_lai, onBam: props.onTai } : undefined}
      />
    );
  }
  if (trang.can_bo.length === 0) return <KhoiTrangThai bieu_tuong="users" cau={DANH_BA.trong} />;

  return (
    <>
      <div className="xa-tim">
        <ONhapDong
          id="xa-tim-can-bo"
          nhan={XA_GIAO_DIEN.tim_danh_ba}
          gia_tri={tu_khoa}
          toi_da={60}
          onDoi={datTuKhoa}
        />
      </div>
      {loc.length === 0 ? (
        <KhoiTrangThai bieu_tuong="users" cau={XA_GIAO_DIEN.khong_thay_can_bo} />
      ) : (
        <ul className="xa-ds">
          {loc.map((cb, i) => (
            // Không có mã trong hợp đồng công khai (cố ý); danh sách không sắp lại, nên khoá là vị trí
            // trong danh sách ĐÃ LỌC cộng tên — đủ ổn cho một lần xem.
            <TheCanBoXa key={`${i}-${cb.ho_ten}`} cb={cb} />
          ))}
        </ul>
      )}
    </>
  );
}

export function useDanhBaXa(ten_mien: string) {
  const [trang, datTrang] = useState<TrangDanhBa>({ kieu: "dang-tai" });
  const da_tai = useRef(false);

  async function tai() {
    datTrang({ kieu: "dang-tai" });
    datTrang(sauKhiTaiDanhBa(await danhBaCanBoXa(ten_mien)));
  }

  useEffect(() => {
    if (da_tai.current) return;
    da_tai.current = true;
    void tai();
  }, []);

  return { trang, taiLai: () => void tai() };
}
