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
import { DANH_BA, DIRECTORY_UNIT_HEAD, XA_GIAO_DIEN, XA_PA, XA_TN } from "./noi-dung";

import { BieuTuong } from "./BieuTuong";
import { KhoiTrangThai } from "./khung-xa";
import { ONhapDong } from "./o-nhap";

/** Chữ cái đầu của TÊN GỌI (từ cuối) — quy ước của bản mẫu. */
function chuDau(ho_ten: string): string {
  const cuoi = ho_ten.trim().split(/\s+/).pop() ?? "";
  return (cuoi[0] ?? "?").toUpperCase();
}

/** Bỏ dấu tiếng Việt và hoa thường — người lớn tuổi hay gõ không dấu ("chu tich"). */
export function boDau(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d").replace(/Đ/g, "D").toLowerCase();
}

/**
 * Lọc trên máy, không phân biệt hoa thường, không phân biệt dấu, và theo SỐ điện thoại (bỏ dấu cách, chấm
 * trong số) — như danh bạ của prototype. Không dấu hiệu nào của từ khoá rời khỏi máy.
 */
export function locCanBo(ds: readonly CanBoCongKhai[], tu_khoa: string): readonly CanBoCongKhai[] {
  const q = boDau(tu_khoa.trim());
  if (q === "") return ds;
  const so = q.replace(/\D/g, "");
  // Chỉ so theo số khi từ khoá TOÀN là số (cho phép dấu cách, chấm, gạch) — "tổ 3" không khớp mọi số có chữ 3.
  const theo_so = so.length >= 3 && /^[\d\s.\-+]+$/.test(q);
  return ds.filter(
    (cb) =>
      // The units a person heads are searchable too: "ha lam" finds the Trưởng thôn Hà Lam.
      [cb.ho_ten, cb.chuc_vu, cb.bo_phan, ...cb.residential_units_headed].some((o) => boDau(o).includes(q)) ||
      (theo_so && [cb.so_co_quan, cb.di_dong].some((o) => o.replace(/\D/g, "").includes(so))),
  );
}

/**
 * The commune's own order first: people with a `display_order` ascending, then everybody without one in
 * the order the server sent. STABLE — equal positions keep the server's order. The server already sends
 * this order (`danh_ba_cong_khai.go:54-56`); sorting here is the guard for the day it does not, so the
 * commune's chosen order is what the citizen sees either way. PURE.
 */
export function byDisplayOrder(ds: readonly CanBoCongKhai[]): CanBoCongKhai[] {
  return ds
    .map((cb, i) => ({ cb, i }))
    .sort((a, b) => {
      const x = a.cb.display_order;
      const y = b.cb.display_order;
      if (x !== null && y !== null && x !== y) return x - y;
      if (x !== null && y === null) return -1;
      if (x === null && y !== null) return 1;
      return a.i - b.i;
    })
    .map((e) => e.cb);
}

/**
 * "Trưởng thôn Hà Lam" for a unit this person heads. The unit name is the commune's own, and may already
 * begin with its kind ("Thôn Hà Lam", "Tổ dân phố 3") — then the line is "Trưởng thôn Hà Lam" / "Trưởng tổ
 * dân phố 3", not "Trưởng thôn Thôn Hà Lam". Otherwise it reads "Trưởng thôn <tên>". PURE.
 */
export function unitHeadLine(unit: string): string {
  const u = unit.trim();
  if (/^(thôn|tổ dân phố|tổ|khu phố|ấp|bản|xóm)(?=\s|$)/i.test(u)) {
    return `${DIRECTORY_UNIT_HEAD.head_of} ${u.charAt(0).toLocaleLowerCase("vi")}${u.slice(1)}`;
  }
  return `${DIRECTORY_UNIT_HEAD.head_of_village} ${u}`;
}

/**
 * Nhóm theo BỘ PHẬN, theo thứ tự máy chủ trả (không sắp lại). Bản mẫu chia "Lãnh đạo UBND xã" /
 * "Bộ phận chuyên môn" bằng một trường nhóm ViGov không có; `bo_phan` là thứ xã thật sự nhập. Người
 * không ghi bộ phận vào nhóm "Cán bộ khác" ở cuối. THUẦN.
 */
export function nhomTheoBoPhan(
  ds: readonly CanBoCongKhai[],
  nhan_khac: string,
): { bo_phan: string; can_bo: CanBoCongKhai[] }[] {
  const nhom: { bo_phan: string; can_bo: CanBoCongKhai[] }[] = [];
  const khac: CanBoCongKhai[] = [];
  for (const cb of ds) {
    const bp = cb.bo_phan.trim();
    if (bp === "") {
      khac.push(cb);
      continue;
    }
    const co = nhom.find((n) => n.bo_phan === bp);
    if (co) co.can_bo.push(cb);
    else nhom.push({ bo_phan: bp, can_bo: [cb] });
  }
  if (khac.length > 0) nhom.push({ bo_phan: nhan_khac, can_bo: khac });
  return nhom;
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
        {cb.residential_units_headed.map((unit) => (
          <span key={unit}>{unitHeadLine(unit)}</span>
        ))}
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
  "khong-hop-le": XA_PA.danh_ba_khong_hop_le,
} as const;

export function ThanDanhBaXa(props: { trang: TrangDanhBa; onTai: () => void }) {
  const [tu_khoa, datTuKhoa] = useState("");
  const { trang } = props;
  const loc = useMemo(
    () => (trang.kieu === "xong" ? locCanBo(byDisplayOrder(trang.can_bo), tu_khoa) : []),
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
  if (trang.can_bo.length === 0) return <KhoiTrangThai bieu_tuong="users" cau={XA_PA.danh_ba_trong} />;

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
        nhomTheoBoPhan(loc, XA_TN.nhom_khac).map((n) => (
          <section key={n.bo_phan} className="xa-nhom">
            <h2 className="xa-dau-khoi__tieu-de xa-nhom__tieu-de">{n.bo_phan}</h2>
            <ul className="xa-ds">
              {n.can_bo.map((cb, i) => (
                // Không có mã trong hợp đồng công khai (cố ý); danh sách không sắp lại, nên khoá là vị trí
                // trong nhóm cộng tên — đủ ổn cho một lần xem.
                <TheCanBoXa key={`${i}-${cb.ho_ten}`} cb={cb} />
              ))}
            </ul>
          </section>
        ))
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
