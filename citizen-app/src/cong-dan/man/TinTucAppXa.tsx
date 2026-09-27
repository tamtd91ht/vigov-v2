/**
 * TIN TỨC CỦA XÃ — giao diện theo bản mẫu `vi-gov/zalo-miniapp` (`features/news/*`): bài đầu là thẻ nổi
 * bật có dải màu, các bài sau là hàng có ô vuông; chạm để đọc toàn văn.
 *
 * DỮ LIỆU VÀ QUY TẮC GIỮ NGUYÊN của `TinTucXaScreen.tsx` — cùng trạng thái thuần (`TIN_DAU`,
 * `sauKhiTaiTin`, `sauKhiTaiBai`), cùng tuyến công khai theo tên miền, cùng "Xem thêm" thay cho cuộn
 * vô hạn. VĂN BẢN THUẦN: thân tin vẽ bằng nút chữ của React, chia đoạn theo dòng trống; không HTML.
 */
import { useEffect, useRef, useState } from "react";

import { baiTinCuaXa, tinCuaXa } from "../api/goi-vigov";
import { chiaDoan, type TinXaTomTat } from "../api/hop-dong-cong-khai";
import { NGAY_KHONG_DOC_DUOC, ngayVN } from "../../lib/thoi-diem";
import { TIN_XA } from "./noi-dung";
import {
  batDauTaiTin,
  type DanhSachTin,
  sauKhiTaiBai,
  sauKhiTaiTin,
  TIN_DAU,
  type TrangBai,
} from "./TinTucXaScreen";

import { BieuTuong } from "./BieuTuong";
import { DauManCon, KhoiTrangThai, TrangCon } from "./khung-xa";

const ngay = (s: string) => ngayVN(s) ?? NGAY_KHONG_DOC_DUOC;

function dongPhu(tin: TinXaTomTat): string {
  return tin.chuyen_muc !== "" ? `${tin.chuyen_muc} · ${ngay(tin.ngay_dang)}` : ngay(tin.ngay_dang);
}

/** Một hàng tin: ô vuông + tiêu đề + chuyên mục · ngày. Dùng cả ở trang chủ. */
export function HangTin({ tin, onMo }: { tin: TinXaTomTat; onMo: (id: string) => void }) {
  return (
    <button type="button" className="xa-the xa-hang-tin" onClick={() => onMo(tin.id)}>
      <span className="xa-hang-tin__o">
        <BieuTuong ten="news" co={26} />
      </span>
      <span className="xa-hang-tin__chu">
        <strong className="xa-hang-tin__tieu-de">{tin.tieu_de}</strong>
        <span className="xa-phu">{dongPhu(tin)}</span>
      </span>
    </button>
  );
}

/** Bài đầu danh sách: dải màu lớn + tiêu đề + tóm tắt. */
function TheNoiBat({ tin, onMo }: { tin: TinXaTomTat; onMo: (id: string) => void }) {
  return (
    <button type="button" className="xa-the xa-noi-bat" onClick={() => onMo(tin.id)}>
      <span className="xa-noi-bat__bia" aria-hidden="true">
        <BieuTuong ten="news" co={96} />
      </span>
      <span className="xa-noi-bat__chu">
        <strong className="xa-noi-bat__tieu-de">{tin.tieu_de}</strong>
        {tin.tom_tat !== "" && <span className="xa-noi-bat__tom-tat">{tin.tom_tat}</span>}
        <span className="xa-phu">{dongPhu(tin)}</span>
      </span>
    </button>
  );
}

const CAU_LOI = {
  "loi-mang": TIN_XA.loi_mang,
  "loi-may-chu": TIN_XA.loi_may_chu,
  "khong-hop-le": TIN_XA.khong_hop_le,
} as const;

/** Danh sách tin — thân của tab "Tin tức". */
export function DanhSachTinXa(props: { ds: DanhSachTin; onMo: (id: string) => void; onTai: () => void }) {
  const { ds } = props;
  if (!ds.da_co_trang_dau && ds.dang_tai) {
    return <KhoiTrangThai bieu_tuong="news" cau={TIN_XA.dang_tai} dang_tai />;
  }
  if (!ds.da_co_trang_dau && ds.loi !== null) {
    return (
      <KhoiTrangThai
        bieu_tuong="alert"
        loi
        cau={CAU_LOI[ds.loi]}
        nut={ds.loi !== "khong-hop-le" ? { nhan: TIN_XA.nut_thu_lai, onBam: props.onTai } : undefined}
      />
    );
  }
  if (ds.muc.length === 0) return <KhoiTrangThai bieu_tuong="news" cau={TIN_XA.trong} />;
  return (
    <>
      <ul className="xa-ds">
        {ds.muc.map((t, i) => (
          <li key={t.id}>{i === 0 ? <TheNoiBat tin={t} onMo={props.onMo} /> : <HangTin tin={t} onMo={props.onMo} />}</li>
        ))}
      </ul>
      {ds.loi !== null && (
        <KhoiTrangThai
          bieu_tuong="alert"
          loi
          cau={CAU_LOI[ds.loi]}
          nut={ds.loi !== "khong-hop-le" ? { nhan: TIN_XA.nut_thu_lai, onBam: props.onTai } : undefined}
        />
      )}
      {ds.loi === null && ds.con_nua && (
        <button type="button" className="xa-nut xa-nut--phu" disabled={ds.dang_tai} onClick={props.onTai}>
          {ds.dang_tai ? TIN_XA.dang_tai_them : TIN_XA.nut_xem_them}
        </button>
      )}
      {!ds.con_nua && <p className="xa-phu xa-giua">{TIN_XA.het_danh_sach}</p>}
    </>
  );
}

/** Tải danh sách tin của xã; trạng thái sống ở đây để tab và trang chủ dùng chung một lần tải. */
export function useTinXa(ten_mien: string) {
  const [ds, datDs] = useState<DanhSachTin>(TIN_DAU);
  const da_tai = useRef(false);

  async function tai(con_tro: string) {
    datDs(batDauTaiTin);
    const kq = await tinCuaXa(ten_mien, con_tro);
    datDs((truoc) => sauKhiTaiTin(truoc, kq));
  }

  useEffect(() => {
    if (da_tai.current) return;
    da_tai.current = true;
    void tai("");
  }, []);

  return {
    ds,
    taiTiep: () => {
      if (!ds.dang_tai) void tai(ds.da_co_trang_dau ? ds.con_tro : "");
    },
  };
}

/** Một bài — dải bìa + tiêu đề + ngày + toàn văn. Màn con, có nút quay lại. */
export function BaiTinXa(props: { ten_mien: string; id: string; onQuayLai: () => void }) {
  const [trang, datTrang] = useState<TrangBai>({ kieu: "dang-tai" });
  const da_tai = useRef(false);

  async function tai() {
    datTrang({ kieu: "dang-tai" });
    datTrang(sauKhiTaiBai(await baiTinCuaXa(props.ten_mien, props.id)));
  }

  useEffect(() => {
    if (da_tai.current) return;
    da_tai.current = true;
    void tai();
  }, []);

  return (
    <>
      <DauManCon tieu_de={TIN_XA.tieu_de} onQuayLai={props.onQuayLai} />
      <TrangCon>
        {trang.kieu === "dang-tai" && <KhoiTrangThai bieu_tuong="news" cau={TIN_XA.dang_tai_bai} dang_tai />}
        {trang.kieu === "khong-thay" && <KhoiTrangThai bieu_tuong="news" loi cau={TIN_XA.khong_thay} />}
        {trang.kieu === "loi" && (
          <KhoiTrangThai
            bieu_tuong="alert"
            loi
            cau={CAU_LOI[trang.loi]}
            nut={trang.loi !== "khong-hop-le" ? { nhan: TIN_XA.nut_thu_lai, onBam: () => void tai() } : undefined}
          />
        )}
        {trang.kieu === "xong" && (
          <article className="xa-bai">
            <div className="xa-bai__bia" aria-hidden="true">
              <BieuTuong ten="news" co={110} />
            </div>
            <h2 className="xa-bai__tieu-de">{trang.bai.tieu_de}</h2>
            <p className="xa-phu">{dongPhu(trang.bai)}</p>
            <div className="xa-ke" />
            {chiaDoan(trang.bai.noi_dung).map((doan, i) => (
              <p key={i} className="xa-bai__doan">
                {doan}
              </p>
            ))}
          </article>
        )}
      </TrangCon>
    </>
  );
}
