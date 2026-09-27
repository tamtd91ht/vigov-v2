/**
 * TIN TỨC CỦA XÃ — giao diện theo bản mẫu `vi-gov/zalo-miniapp` (`features/news/*`): bài đầu là thẻ nổi
 * bật có dải màu, các bài sau là hàng có ô vuông; chạm để đọc toàn văn.
 *
 * DỮ LIỆU VÀ QUY TẮC GIỮ NGUYÊN của `TinTucXaScreen.tsx` — cùng trạng thái thuần (`TIN_DAU`,
 * `sauKhiTaiTin`, `sauKhiTaiBai`), cùng tuyến công khai theo tên miền, cùng "Xem thêm" thay cho cuộn
 * vô hạn. VĂN BẢN THUẦN: thân tin vẽ bằng nút chữ của React, chia đoạn theo dòng trống; không HTML.
 */
import { useEffect, useMemo, useRef, useState } from "react";

import { baiTinCuaXa, tinCuaXa } from "../api/goi-vigov";
import { chiaDoan, type TinXaTomTat } from "../api/hop-dong-cong-khai";
import { NGAY_KHONG_DOC_DUOC, ngayVN } from "../../lib/thoi-diem";
import { TIN_XA, XA_TN } from "./noi-dung";
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

/**
 * Chuyên mục có trong các tin ĐÃ TẢI, theo thứ tự gặp — nguồn chip lọc. Bản mẫu có ba loại viết cứng
 * (Tin tức · Sự kiện · Thông báo); ViGov không có trường "loại", chỉ có `chuyen_muc` cán bộ gõ, nên chip
 * lấy đúng những gì xã đã đăng. THUẦN.
 */
export function chuyenMucCua(ds: readonly TinXaTomTat[]): string[] {
  const thay: string[] = [];
  for (const t of ds) if (t.chuyen_muc !== "" && !thay.includes(t.chuyen_muc)) thay.push(t.chuyen_muc);
  return thay;
}

/** Tin liên quan: cùng chuyên mục, bỏ tin đang đọc, tối đa 3. THUẦN. */
export function tinLienQuan(ds: readonly TinXaTomTat[], dang_doc: TinXaTomTat | null, toi_da = 3): TinXaTomTat[] {
  if (dang_doc === null || dang_doc.chuyen_muc === "") return [];
  return ds.filter((t) => t.id !== dang_doc.id && t.chuyen_muc === dang_doc.chuyen_muc).slice(0, toi_da);
}

/** Danh sách tin — thân của tab "Tin tức". */
export function DanhSachTinXa(props: { ds: DanhSachTin; onMo: (id: string) => void; onTai: () => void }) {
  const [chuyen_muc, datChuyenMuc] = useState<string | null>(null);
  const cac_muc = useMemo(() => chuyenMucCua(props.ds.muc), [props.ds.muc]);
  const ds: DanhSachTin =
    chuyen_muc === null ? props.ds : { ...props.ds, muc: props.ds.muc.filter((t) => t.chuyen_muc === chuyen_muc) };
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
  const chips =
    cac_muc.length > 1 ? (
      <div className="xa-chips" role="group" aria-label={XA_TN.loc_chuyen_muc}>
        <button type="button" className={`xa-chip${chuyen_muc === null ? " xa-chip--on" : ""}`} aria-pressed={chuyen_muc === null} onClick={() => datChuyenMuc(null)}>
          {XA_TN.loc_tat_ca}
        </button>
        {cac_muc.map((m) => (
          <button key={m} type="button" className={`xa-chip${chuyen_muc === m ? " xa-chip--on" : ""}`} aria-pressed={chuyen_muc === m} onClick={() => datChuyenMuc(m)}>
            {m}
          </button>
        ))}
      </div>
    ) : null;
  if (ds.muc.length === 0) return <KhoiTrangThai bieu_tuong="news" cau={TIN_XA.trong} />;
  return (
    <>
      {chips}
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
export function BaiTinXa(props: {
  ten_mien: string;
  id: string;
  onQuayLai: () => void;
  /** Tin đã tải ở danh sách — nguồn "tin liên quan" (không gọi thêm mạng). */
  ds?: readonly TinXaTomTat[];
  onMo?: (id: string) => void;
}) {
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
            <TinLienQuan ds={props.ds ?? []} bai={trang.bai} onMo={props.onMo} />
          </article>
        )}
      </TrangCon>
    </>
  );
}

function TinLienQuan(props: { ds: readonly TinXaTomTat[]; bai: TinXaTomTat; onMo?: (id: string) => void }) {
  const lq = tinLienQuan(props.ds, props.bai);
  if (lq.length === 0 || props.onMo === undefined) return null;
  const mo = props.onMo;
  return (
    <section className="xa-lien-quan">
      <h2 className="xa-dau-khoi__tieu-de">{XA_TN.tin_lien_quan}</h2>
      <ul className="xa-ds">
        {lq.map((t) => (
          <li key={t.id}>
            <HangTin tin={t} onMo={mo} />
          </li>
        ))}
      </ul>
    </section>
  );
}
