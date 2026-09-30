"use client";

import { useCallback, useEffect, useMemo, useState, type ReactNode } from "react";

import { ChonNam } from "@/components/chon-nam";
import {
  TRANG_DAU,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { bangTraTuKetQua, traTen, type BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { usePhien } from "@/features/phien/phien-hien-tai";
import { layLoaiVanBan } from "@/lib/api/danh-muc-nghiep-vu";
import type { KetQua } from "@/lib/api/goi";
import type {
  documents_vanBanDiRa,
  page_Result_documents_vanBanDiRa,
} from "@/lib/api/schema.gen";
import { capSoVanBanDi, laySoVanBanDi, suaVanBanDi, type LocVanBanDi } from "@/lib/api/van-ban";
import { namTheoDongHoMay } from "@/lib/nam";
import { QUYEN_GHI_SO_VAN_BAN, quyetDinhTheoKhoa } from "@/lib/quyen";

import {
  CANH_BAO_GO_KHONG_TRA_SO,
  DAN_SO_DI,
  GIAI_THICH_LY_DO_GO,
  LOI_THIEU_LY_DO_GO,
  NUT_CAP_SO,
  NUT_GO,
  NUT_HUY,
  NUT_LUU,
  NUT_SUA,
  NUT_XAC_NHAN_GO,
  O_LOAI_VAN_BAN,
  O_LY_DO_GO,
  O_NGAY_VAN_BAN,
  O_NGUOI_KY,
  O_NOI_NHAN,
  O_TRICH_YEU,
  SO_DI_RONG,
  nhanLoaiVanBan,
  nhanNgayCoThe,
  nhanSoVaoSo,
} from "./nhan-van-ban";
import {
  ChonThuTu,
  GOI_Y_TIM_DI,
  OTimVanBan,
  doiLocVeTrangDau,
  sapXepTheoThuTu,
  type MaThuTu,
} from "./loc-so-van-ban";
import { DieuHuongTrang } from "./so-van-ban-den";
import { guiGoVanBanDi } from "./thao-tac-van-ban";

/**
 * Sổ văn bản ĐI — bốn tuyến của `service-documents`.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * ⚠ QUYỂN SỔ NÀY KHÔNG CÓ ĐẶC TẢ NÀO. `docs/ui-ux/05-van-ban-don-thu.md` chỉ tả văn bản ĐẾN và đơn
 * thư; `x-vigov-screen` của cả bốn tuyến ghi *"chưa có đặc tả — xem migration 0004"*. Vì vậy màn
 * hình này chỉ vẽ ĐÚNG những trường hợp đồng có, và mọi câu chữ trên đây là của lượt này — đã liệt
 * kê trong báo cáo bàn giao để có người rà lại.
 *
 * KHÔNG CÓ TRẠNG THÁI VÀ KHÔNG CÓ QUY TRÌNH. `vanBanDiRa` cố ý không có `status`, không có
 * `due_at`: một văn bản đi không có vòng đời và không mang cam kết nào — CẤP SỐ CHÍNH LÀ hành vi
 * phát hành (`service-documents/internal/http/van_ban_di.go:40`). Vẽ thêm "nháp → đã ký → đã phát
 * hành" ở đây là bịa ra một quy trình mà mọi xã sau đó buộc phải đi theo, và không ai quyết định
 * nó cả.
 *
 * KHÔNG CÓ CHUYỂN XỬ LÝ, cùng một lẽ: văn bản đi rời khỏi xã, nó không đi giữa các bộ phận.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CỔNG QUYỀN BỌC PHẦN GHI, KHÔNG BỌC BẢNG — cùng khuôn và cùng lý do với sổ văn bản đến.
 */

/* ---- trạng thái ---------------------------------------------------------------------------- */

export type DangMoDi =
  | { kieu: "them"; khoaChongTrung: string }
  | { kieu: "sua"; vb: documents_vanBanDiRa }
  | { kieu: "go"; vb: documents_vanBanDiRa }
  | null;

export type BanNhapDi = {
  ngayVanBan: string;
  loaiVanBan: string;
  trichYeu: string;
  noiNhan: string;
  nguoiKy: string;
  lyDoGo: string;
};

export const BAN_DI_TRONG: BanNhapDi = {
  ngayVanBan: "",
  loaiVanBan: "",
  trichYeu: "",
  noiNhan: "",
  nguoiKy: "",
  lyDoGo: "",
};

export type ThaoTacDi = {
  readonly them: () => void;
  readonly sua: (vb: documents_vanBanDiRa) => void;
  readonly go: (vb: documents_vanBanDiRa) => void;
};

function homNay(): string {
  const d = new Date();
  const hai = (n: number) => String(n).padStart(2, "0");
  return `${d.getFullYear()}-${hai(d.getMonth() + 1)}-${hai(d.getDate())}`;
}

function banTuDong(vb: documents_vanBanDiRa): BanNhapDi {
  return {
    ...BAN_DI_TRONG,
    ngayVanBan: vb.document_date,
    loaiVanBan: vb.document_type,
    trichYeu: vb.summary,
    noiNhan: vb.recipient,
    nguoiKy: vb.signer ?? "",
  };
}

/* ---- vỏ đọc dữ liệu ------------------------------------------------------------------------ */

export function SoVanBanDi() {
  const [namGoc] = useState(namTheoDongHoMay);
  const [nam, datNam] = useState(namGoc);
  const [loaiLoc, datLoaiLoc] = useState("");
  const [tim, datTim] = useState("");
  const [thuTu, datThuTu] = useState<MaThuTu>("");

  const [nganXep, datNganXep] = useState<NganXepConTro>(TRANG_DAU);
  const [lanDoc, datLanDoc] = useState(0);

  /** Kết quả đọc kèm bộ lọc đã sinh ra nó — "đang tải" suy ra từ so sánh. Xem `so-van-ban-den.tsx`. */
  const [kq, datKq] = useState<{
    loc: LocVanBanDi;
    kq: KetQua<page_Result_documents_vanBanDiRa>;
  } | null>(null);
  const [loai, datLoai] = useState<BangTraDanhMuc>({ pha: "dangDoc" });

  const [dangMo, datDangMo] = useState<DangMoDi>(null);
  const [ban, datBan] = useState<BanNhapDi>(BAN_DI_TRONG);
  const [loi, datLoi] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [cauDaXong, datCauDaXong] = useState("");

  const phien = usePhien();
  const quyetDinhGhi = phien === null ? null : quyetDinhTheoKhoa(phien, QUYEN_GHI_SO_VAN_BAN);

  const loc = useMemo<LocVanBanDi>(
    () => ({
      nam,
      loaiVanBan: loaiLoc,
      tim,
      ...sapXepTheoThuTu(thuTu),
      cursor: nganXep.hienTai,
    }),
    [nam, loaiLoc, tim, thuTu, nganXep],
  );

  useEffect(() => {
    let bo = false;
    laySoVanBanDi(loc).then((k) => {
      if (!bo) datKq({ loc, kq: k });
    });
    return () => {
      bo = true;
    };
  }, [loc, lanDoc]);

  // MỘT DANH MỤC, ĐỌC MỘT LẦN CHO CẢ MÀN — và đúng danh mục Loại văn bản mà sổ đến dùng: hai quyển
  // sổ chọn loại từ cùng một danh mục của xã (`documents.loai_van_ban`).
  useEffect(() => {
    let bo = false;
    layLoaiVanBan().then((k) => {
      if (bo) return;
      datLoai(
        bangTraTuKetQua(
          k.ok
            ? { ok: true, duLieu: { items: k.duLieu.items.map((m) => ({ id: m.code, name: m.label })) } }
            : k,
        ),
      );
    });
    return () => {
      bo = true;
    };
  }, []);

  const doiLoc = useCallback((dat: () => void) => doiLocVeTrangDau(dat, datNganXep), []);

  const mo = useCallback((m: DangMoDi, banDau: BanNhapDi) => {
    datDangMo(m);
    datBan(banDau);
    datLoi("");
    datCauDaXong("");
  }, []);

  const dong = useCallback(() => {
    datDangMo(null);
    datBan(BAN_DI_TRONG);
    datLoi("");
  }, []);

  const thaoTac: ThaoTacDi = {
    // KHOÁ CHỐNG TRÙNG SINH LÚC MỞ BIỂU MẪU, và ở quyển sổ này hậu quả của việc sinh lúc gửi là
    // nặng nhất: một lần bấm lại với khoá mới cấp một số thứ hai cho một văn bản đã có số, và số
    // thứ nhất đã nằm trên tờ giấy đóng dấu gửi đi.
    them: () =>
      mo({ kieu: "them", khoaChongTrung: crypto.randomUUID() }, { ...BAN_DI_TRONG, ngayVanBan: homNay() }),
    sua: (vb) => mo({ kieu: "sua", vb }, banTuDong(vb)),
    go: (vb) => mo({ kieu: "go", vb }, BAN_DI_TRONG),
  };

  const thucHien = useCallback(function <T>(goi: Promise<KetQua<T>>, cau: string) {
    datLoi("");
    datCauDaXong("");
    datDangGui(true);
    void goi.then((k) => {
      datDangGui(false);
      if (!k.ok) {
        datLoi(k.thongBao);
        return;
      }
      datDangMo(null);
      datBan(BAN_DI_TRONG);
      datCauDaXong(cau);
      datLanDoc((n) => n + 1);
    });
  }, []);

  const guiBieuMau = useCallback(() => {
    if (dangMo === null || dangGui) return;

    switch (dangMo.kieu) {
      case "them":
        thucHien(
          capSoVanBanDi(
            {
              document_date: ban.ngayVanBan,
              document_type: ban.loaiVanBan,
              summary: ban.trichYeu,
              recipient: ban.noiNhan,
              signer: ban.nguoiKy,
            },
            dangMo.khoaChongTrung,
          ),
          "Đã cấp số và ghi vào sổ văn bản đi.",
        );
        return;
      case "sua":
        thucHien(
          suaVanBanDi(dangMo.vb.id, {
            document_date: ban.ngayVanBan,
            document_type: ban.loaiVanBan,
            summary: ban.trichYeu,
            recipient: ban.noiNhan,
            signer: ban.nguoiKy,
          }),
          "Đã lưu thay đổi.",
        );
        return;
      default:
        thucHien(
          guiGoVanBanDi(dangMo.vb.id, ban.lyDoGo),
          "Đã gỡ văn bản khỏi sổ. Số đi của văn bản ấy không được cấp lại.",
        );
    }
  }, [ban, dangGui, dangMo, thucHien]);

  return (
    <ManSoVanBanDi
      kq={kq !== null && kq.loc === loc ? kq.kq : null}
      nam={nam}
      namGoc={namGoc}
      datNam={(n) => doiLoc(() => datNam(n))}
      loaiLoc={loaiLoc}
      datLoaiLoc={(v) => doiLoc(() => datLoaiLoc(v))}
      tim={tim}
      datTim={(v) => doiLoc(() => datTim(v))}
      thuTu={thuTu}
      datThuTu={(v) => doiLoc(() => datThuTu(v))}
      traLoai={loai}
      coQuyenGhi={quyetDinhGhi !== null && quyetDinhGhi.hien}
      thieuQuyenGhi={
        quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-du-quyen"
      }
      thaoTac={thaoTac}
      cauDaXong={cauDaXong}
      loiNgoaiForm={dangMo === null ? loi : ""}
      nganXep={nganXep}
      diToiTrang={datNganXep}
      form={
        dangMo === null ? null : (
          <BieuMauVanBanDi
            dangMo={dangMo}
            ban={ban}
            datBan={datBan}
            traLoai={loai}
            loi={loi}
            dangGui={dangGui}
            onGui={guiBieuMau}
            onHuy={dong}
          />
        )
      }
    />
  );
}

/* ---- phần trình bày ------------------------------------------------------------------------ */

export function ManSoVanBanDi({
  kq,
  nam,
  namGoc,
  datNam,
  loaiLoc,
  datLoaiLoc,
  tim,
  datTim,
  thuTu,
  datThuTu,
  traLoai,
  coQuyenGhi,
  thieuQuyenGhi,
  thaoTac,
  cauDaXong,
  loiNgoaiForm,
  nganXep,
  diToiTrang,
  form,
}: {
  kq: KetQua<page_Result_documents_vanBanDiRa> | null;
  nam: number;
  namGoc: number;
  datNam: (n: number) => void;
  loaiLoc: string;
  datLoaiLoc: (v: string) => void;
  tim: string;
  datTim: (v: string) => void;
  thuTu: MaThuTu;
  datThuTu: (v: MaThuTu) => void;
  traLoai: BangTraDanhMuc;
  coQuyenGhi: boolean;
  thieuQuyenGhi: boolean;
  thaoTac: ThaoTacDi;
  cauDaXong: string;
  loiNgoaiForm: string;
  nganXep: NganXepConTro;
  diToiTrang: (toi: NganXepConTro) => void;
  form: ReactNode;
}) {
  return (
    <section className="man-van-ban" aria-labelledby="tieu-de-so-di">
      <h2 id="tieu-de-so-di">Sổ văn bản đi</h2>
      <p className="ghi-chu">{DAN_SO_DI}</p>

      {thieuQuyenGhi && (
        <p className="trang-thai-rong">
          Tài khoản của bạn không có quyền cấp số, sửa hay gỡ văn bản đi. Sổ dưới đây vẫn xem được.
        </p>
      )}

      {coQuyenGhi && (
        <p className="cum-nut">
          <button type="button" className="nut-chinh" onClick={thaoTac.them}>
            {NUT_CAP_SO}
          </button>
        </p>
      )}

      {cauDaXong !== "" && <p role="status">{cauDaXong}</p>}
      {loiNgoaiForm !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiNgoaiForm}
        </p>
      )}

      {form}

      <div className="hang-loc">
        <ChonNam id="nam-so-van-ban-di" nhan="Năm của sổ" nam={nam} namGoc={namGoc} datNam={datNam} />
        <p className="chon-hang-muc">
          <label htmlFor="loc-loai-di">Loại văn bản</label>{" "}
          <select id="loc-loai-di" value={loaiLoc} onChange={(e) => datLoaiLoc(e.target.value)}>
            <option value="">Tất cả loại</option>
            {traLoai.pha === "xong" &&
              [...traLoai.ten].map(([ma, ten]) => (
                <option key={ma} value={ma}>
                  {ten}
                </option>
              ))}
          </select>
        </p>

        <ChonThuTu id="thu-tu-so-di" thuTu={thuTu} datThuTu={datThuTu} />

        <OTimVanBan id="tim-van-ban-di" goiY={GOI_Y_TIM_DI} tim={tim} datTim={datTim} />
      </div>

      <BangVanBanDi
        kq={kq}
        traLoai={traLoai}
        coQuyenGhi={coQuyenGhi}
        thaoTac={thaoTac}
        soCuTruoc={thuTu === "so-tang"}
      />

      {kq !== null && kq.ok && (
        <DieuHuongTrang
          nganXep={nganXep}
          conTroTiep={kq.duLieu.next_cursor}
          conTrangSau={kq.duLieu.has_more}
          diToiTrang={diToiTrang}
        />
      )}
    </section>
  );
}

/**
 * Bảng sổ văn bản đi.
 *
 * ⚠ `recipient` CÓ THỂ MANG TÊN MỘT CÔNG DÂN — "Ông Nguyễn Văn A, thôn Bình Trị" là điều một xã
 * viết trên một công văn trả lời (`van_ban_di.go:36`). Nó hiện nguyên văn cho cán bộ của chính xã
 * ấy, và điều màn hình bảo đảm hẹp hơn: giá trị ấy không đi vào một `aria-label`, một `title`, một
 * tên tệp hay một URL nào (luật 3, cấm #4) — nhãn trợ năng của nút dùng SỐ ĐI.
 */
export function BangVanBanDi({
  kq,
  traLoai,
  coQuyenGhi,
  thaoTac,
  soCuTruoc = false,
}: {
  kq: KetQua<page_Result_documents_vanBanDiRa> | null;
  traLoai: BangTraDanhMuc;
  coQuyenGhi: boolean;
  thaoTac: ThaoTacDi;
  /** Chú thích bảng nói đúng thứ tự đang xem — xem `BangVanBanDen`. */
  soCuTruoc?: boolean;
}) {
  if (kq === null) return <p role="status">Đang tải sổ văn bản đi…</p>;
  if (!kq.ok) {
    return (
      <p className="thong-bao-loi" role="alert">
        {kq.thongBao}
      </p>
    );
  }
  if (kq.duLieu.items.length === 0) return <p className="trang-thai-rong">{SO_DI_RONG}</p>;

  return (
    <div className="bang-cuon" role="region" aria-label="Sổ văn bản đi" tabIndex={0}>
      <table className="bang-danh-muc bang-van-ban">
        <caption className="an-thi-giac">
          Các văn bản xã đã phát hành, {soCuTruoc ? "số cũ nhất trước" : "số mới nhất trước"}
        </caption>
        <thead>
          <tr>
            <th scope="col">Số đi</th>
            <th scope="col">Ngày văn bản</th>
            <th scope="col">Loại văn bản</th>
            <th scope="col">Trích yếu</th>
            <th scope="col">Nơi nhận</th>
            <th scope="col">Người ký</th>
            {coQuyenGhi && (
              <th scope="col">
                <span className="an-thi-giac">Thao tác</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {kq.duLieu.items.map((vb) => {
            const so = nhanSoVaoSo(vb.number, vb.year);
            return (
              <tr key={vb.id}>
                <td>{so}</td>
                <td>{nhanNgayCoThe(vb.document_date)}</td>
                <td>{nhanLoaiVanBan(traTen(traLoai, vb.document_type))}</td>
                <td className="o-trich-yeu">{vb.summary}</td>
                <td>{vb.recipient}</td>
                <td>{vb.signer === undefined || vb.signer === "" ? "Không ghi" : vb.signer}</td>
                {coQuyenGhi && (
                  <td className="o-thao-tac">
                    <button
                      type="button"
                      className="nut-phu"
                      aria-label={`${NUT_SUA} văn bản đi số ${so}`}
                      onClick={() => thaoTac.sua(vb)}
                    >
                      {NUT_SUA}
                    </button>
                    <button
                      type="button"
                      className="nut-phu nut-xoa"
                      aria-label={`${NUT_GO} văn bản đi số ${so}`}
                      onClick={() => thaoTac.go(vb)}
                    >
                      {NUT_GO}
                    </button>
                  </td>
                )}
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

/* ---- biểu mẫu ------------------------------------------------------------------------------ */

const TIEU_DE_DI: Record<NonNullable<DangMoDi>["kieu"], string> = {
  them: "Cấp số văn bản đi",
  sua: "Sửa văn bản đi",
  go: "Gỡ văn bản đi khỏi sổ",
};

export function BieuMauVanBanDi({
  dangMo,
  ban,
  datBan,
  traLoai,
  loi,
  dangGui,
  onGui,
  onHuy,
}: {
  dangMo: NonNullable<DangMoDi>;
  ban: BanNhapDi;
  datBan: (b: BanNhapDi) => void;
  traLoai: BangTraDanhMuc;
  /** MỘT vùng lỗi — xem `BieuMauVanBanDen`, cùng lý do. */
  loi: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const tieuDe = TIEU_DE_DI[dangMo.kieu];
  const laNhap = dangMo.kieu === "them" || dangMo.kieu === "sua";

  return (
    <form
      className="form-danh-muc"
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <h4>{tieuDe}</h4>

      {dangMo.kieu !== "them" && (
        <p className="ghi-chu">Văn bản đi số {nhanSoVaoSo(dangMo.vb.number, dangMo.vb.year)}</p>
      )}

      {laNhap && (
        <>
          <div className="o-nhap">
            <label htmlFor="o-ngay-van-ban-di">{O_NGAY_VAN_BAN}</label>
            <input
              id="o-ngay-van-ban-di"
              name="ngayVanBan"
              type="date"
              value={ban.ngayVanBan}
              onChange={(e) => datBan({ ...ban, ngayVanBan: e.target.value })}
            />
          </div>

          <div className="o-nhap">
            <label htmlFor="o-loai-van-ban-di">{O_LOAI_VAN_BAN}</label>
            <select
              id="o-loai-van-ban-di"
              name="loaiVanBan"
              value={ban.loaiVanBan}
              onChange={(e) => datBan({ ...ban, loaiVanBan: e.target.value })}
            >
              <option value="">— Chọn loại văn bản —</option>
              {traLoai.pha === "xong" &&
                [...traLoai.ten].map(([ma, ten]) => (
                  <option key={ma} value={ma}>
                    {ten}
                  </option>
                ))}
              {ban.loaiVanBan !== "" &&
                !(traLoai.pha === "xong" && traLoai.ten.has(ban.loaiVanBan)) && (
                  <option value={ban.loaiVanBan}>{ban.loaiVanBan} (không còn trong danh mục)</option>
                )}
            </select>
          </div>

          <div className="o-nhap">
            <label htmlFor="o-trich-yeu-di">{O_TRICH_YEU}</label>
            <textarea
              id="o-trich-yeu-di"
              name="trichYeu"
              rows={3}
              value={ban.trichYeu}
              onChange={(e) => datBan({ ...ban, trichYeu: e.target.value })}
            />
          </div>

          <div className="o-nhap">
            <label htmlFor="o-noi-nhan">{O_NOI_NHAN}</label>
            <input
              id="o-noi-nhan"
              name="noiNhan"
              value={ban.noiNhan}
              onChange={(e) => datBan({ ...ban, noiNhan: e.target.value })}
            />
          </div>

          <div className="o-nhap">
            <label htmlFor="o-nguoi-ky">{O_NGUOI_KY}</label>
            <input
              id="o-nguoi-ky"
              name="nguoiKy"
              value={ban.nguoiKy}
              onChange={(e) => datBan({ ...ban, nguoiKy: e.target.value })}
            />
          </div>

          {dangMo.kieu === "them" && (
            <p className="canh-bao-pham-vi">
              Hệ thống cấp số đi ngay khi lưu, và số đã cấp không bao giờ được cấp lại. Kiểm lại
              nội dung trước khi bấm {NUT_LUU}.
            </p>
          )}
        </>
      )}

      {dangMo.kieu === "go" && (
        <>
          <p className="canh-bao-pham-vi">{CANH_BAO_GO_KHONG_TRA_SO}</p>
          <div className="o-nhap">
            <label htmlFor="o-ly-do-go-di">{O_LY_DO_GO}</label>
            <input
              id="o-ly-do-go-di"
              name="lyDoGo"
              required
              value={ban.lyDoGo}
              onChange={(e) => datBan({ ...ban, lyDoGo: e.target.value })}
              aria-invalid={loi === LOI_THIEU_LY_DO_GO}
              aria-describedby="giai-thich-ly-do-go-di"
            />
            <p className="ghi-chu" id="giai-thich-ly-do-go-di">
              {GIAI_THICH_LY_DO_GO}
            </p>
          </div>
        </>
      )}

      {loi !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loi}
        </p>
      )}

      <div className="cum-nut">
        <button type="submit" className="nut-chinh" disabled={dangGui}>
          {dangMo.kieu === "go" ? NUT_XAC_NHAN_GO : NUT_LUU}
        </button>
        <button type="button" className="nut-phu" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </button>
      </div>
    </form>
  );
}
