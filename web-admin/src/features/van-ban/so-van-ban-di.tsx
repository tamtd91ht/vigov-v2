"use client";

import { CloudOff, Pencil, Plus, RefreshCw, Trash2 } from "lucide-react";
import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";
import { toast } from "sonner";

import { ActionMenu, type ActionMenuItem } from "@/components/ui/action-menu";
import { Button } from "@/components/ui/button";
import { Card, CardFooter } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { IconButton } from "@/components/ui/icon-button";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { Notice } from "@/components/ui/notice";
import {
  TRANG_DAU,
  coTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { BusyLabel } from "@/features/danh-ba/busy-label";
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
import { cn } from "@/lib/cn";
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
  FilterSelect,
  GOI_Y_TIM_DI,
  OTimVanBan,
  RegisterYearSelect,
  doiLocVeTrangDau,
  sapXepTheoThuTu,
  type MaThuTu,
} from "./loc-so-van-ban";
import { Glyph, plainFrame, type RegisterFrame } from "./document-ui";
import {
  DieuHuongTrang,
  REGISTER_HEAD_ROW,
  REGISTER_ROW,
  REGISTER_TD,
  REGISTER_TH,
  RegisterEmpty,
  RegisterLoading,
  SortHeader,
} from "./so-van-ban-den";
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
 *
 * CHANGED 06/10/2026 (ADR 0068 lần 5): the prototype has no outgoing register, so this tab takes the
 * incoming tab's composition — `Cấp số văn bản đi` in the page header, one filter row without a "Bộ
 * lọc" panel, the table in a card, and the issue / edit / removal forms as centred dialogs. Its row
 * actions stay on the row (Sửa + "⋯"): there is no detail view to move them into.
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

export function SoVanBanDi({ frame = plainFrame }: { frame?: RegisterFrame } = {}) {
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
    datDangGui(true);
    void goi.then((k) => {
      datDangGui(false);
      if (!k.ok) {
        // The refusal stays IN the dialog, under the fields (ADR 0068 lần 6 #4).
        datLoi(k.thongBao);
        return;
      }
      datDangMo(null);
      datBan(BAN_DI_TRONG);
      // The dialog is closed: the outcome is a toast (lần 6 #4).
      toast.success(cau);
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
      // "Tải lại" after a failed read asks the SAME read again through the existing re-read key.
      onReload={() => datLanDoc((n) => n + 1)}
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
      frame={frame}
    />
  );
}

/* ---- phần trình bày ------------------------------------------------------------------------ */

export function ManSoVanBanDi({
  kq,
  onReload,
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
  loiNgoaiForm,
  nganXep,
  diToiTrang,
  form,
  frame = plainFrame,
}: {
  kq: KetQua<page_Result_documents_vanBanDiRa> | null;
  /** Re-read after a failed load ("Tải lại"). Absent → the error state draws no button. */
  onReload?: () => void;
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
  loiNgoaiForm: string;
  nganXep: NganXepConTro;
  diToiTrang: (toi: NganXepConTro) => void;
  form: ReactNode;
  frame?: RegisterFrame;
}) {
  // "Nothing at all" vs "nothing under these filters" (spec §8b), read off the props already held.
  const filtered = nam !== namGoc || loaiLoc !== "" || tim !== "" || coTrangTruoc(nganXep);

  // The incoming tab's header button, the prototype's 40px (`DocumentWorkspace.tsx:127-133`).
  const headerActions = coQuyenGhi ? (
    <Button
      type="button"
      variant="primary"
      className="h-10 justify-center px-4 text-[13px]"
      icon={<Glyph icon={Plus} />}
      aria-haspopup="dialog"
      onClick={thaoTac.them}
    >
      {NUT_CAP_SO}
    </Button>
  ) : null;

  // Dialogs are siblings of the section — see `ManSoVanBanDen`.
  const body = (
    <>
      <section className="flex min-w-0 flex-col gap-4 [&>*]:my-0" aria-labelledby="tieu-de-so-di">
        <h2 id="tieu-de-so-di" className="an-thi-giac">
          Sổ văn bản đi
        </h2>
        {/* One line about the register, where the prototype's petition tab puts its own. */}
        <p className="m-0 max-w-3xl text-[12.5px] text-ink-muted">{DAN_SO_DI}</p>

        {thieuQuyenGhi && (
          <Notice tone="neutral">
            Tài khoản của bạn không có quyền cấp số, sửa hay gỡ văn bản đi. Sổ dưới đây vẫn xem được.
          </Notice>
        )}
        {loiNgoaiForm !== "" && (
          <p className="thong-bao-loi" role="alert">
            {loiNgoaiForm}
          </p>
        )}

        {/* ONE filter row, the incoming tab's shape and classes: search, type, the register's year.
            The order is the "Số đi" header, as on the incoming table. */}
        <div id="outgoing-document-filters" className="flex min-w-0 flex-wrap items-center gap-2.5 [&>*]:my-0">
          <OTimVanBan id="tim-van-ban-di" goiY={GOI_Y_TIM_DI} tim={tim} datTim={datTim} />
          <FilterSelect id="loc-loai-di" label="Lọc theo loại văn bản" value={loaiLoc} onChange={datLoaiLoc}>
            <option value="">Tất cả loại</option>
            {traLoai.pha === "xong" &&
              [...traLoai.ten].map(([ma, ten]) => (
                <option key={ma} value={ma}>
                  {ten}
                </option>
              ))}
          </FilterSelect>
          <RegisterYearSelect id="nam-so-van-ban-di" year={nam} anchorYear={namGoc} onYear={datNam} />
        </div>

        <Card className="bg-white">
          <BangVanBanDi
            kq={kq}
            traLoai={traLoai}
            coQuyenGhi={coQuyenGhi}
            thaoTac={thaoTac}
            soCuTruoc={thuTu === "so-tang"}
            onToggleSort={() => datThuTu(thuTu === "so-tang" ? "" : "so-tang")}
            filtered={filtered}
            onReload={onReload}
          />
          {kq !== null && kq.ok && (kq.duLieu.has_more || coTrangTruoc(nganXep)) && (
            <CardFooter className="justify-end">
              <DieuHuongTrang
                nganXep={nganXep}
                conTroTiep={kq.duLieu.next_cursor}
                conTrangSau={kq.duLieu.has_more}
                diToiTrang={diToiTrang}
              />
            </CardFooter>
          )}
        </Card>
      </section>
      {form}
    </>
  );

  return frame(headerActions, body);
}

/**
 * Bảng sổ văn bản đi — the incoming table's look (`DocumentTable.tsx` classes, spec 00): 11px caps
 * header, 12.5px rows, 16px cell sides, a hairline per row, "Số đi" sortable, the summary and the
 * recipient cut to one line. Its own columns: there is no prototype for this register.
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
  onToggleSort,
  filtered = false,
  onReload,
}: {
  kq: KetQua<page_Result_documents_vanBanDiRa> | null;
  traLoai: BangTraDanhMuc;
  coQuyenGhi: boolean;
  thaoTac: ThaoTacDi;
  /** Chú thích bảng nói đúng thứ tự đang xem — xem `BangVanBanDen`. */
  soCuTruoc?: boolean;
  /** Flips the number order from the "Số đi" header. Absent → a plain header. */
  onToggleSort?: () => void;
  /** A filter or a later page is on: an empty answer is "nothing matches", not "nothing yet". */
  filtered?: boolean;
  /** "Tải lại" on a failed read. Absent → no button. */
  onReload?: () => void;
}) {
  if (kq === null) {
    return <RegisterLoading sentence="Đang tải sổ văn bản đi…" />;
  }
  if (!kq.ok) {
    return (
      <EmptyState
        icon={CloudOff}
        tone="neutral"
        title="Chưa tải được sổ văn bản đi"
        description={
          <span className="text-danger-600" role="alert">
            {kq.thongBao}
          </span>
        }
        action={
          onReload !== undefined ? (
            <Button type="button" variant="secondary" icon={<Glyph icon={RefreshCw} />} onClick={onReload}>
              Tải lại
            </Button>
          ) : undefined
        }
      />
    );
  }
  if (kq.duLieu.items.length === 0) {
    return <RegisterEmpty sentence={SO_DI_RONG} filtered={filtered} />;
  }

  return (
    <div className="overflow-x-auto" role="region" aria-label="Sổ văn bản đi" tabIndex={0}>
      <table className="w-full min-w-[900px] border-collapse text-[12.5px]">
        <caption className="an-thi-giac">
          Các văn bản xã đã phát hành, {soCuTruoc ? "số cũ nhất trước" : "số mới nhất trước"}
        </caption>
        <thead>
          <tr className={REGISTER_HEAD_ROW}>
            <SortHeader label="Số đi" ascending={soCuTruoc} onToggle={onToggleSort} />
            <th scope="col" className={REGISTER_TH}>
              Ngày văn bản
            </th>
            <th scope="col" className={REGISTER_TH}>
              Loại văn bản
            </th>
            <th scope="col" className={REGISTER_TH}>
              Trích yếu
            </th>
            <th scope="col" className={REGISTER_TH}>
              Nơi nhận
            </th>
            <th scope="col" className={REGISTER_TH}>
              Người ký
            </th>
            {coQuyenGhi && (
              <th scope="col" className={REGISTER_TH}>
                <span className="an-thi-giac">Thao tác</span>
              </th>
            )}
          </tr>
        </thead>
        <tbody>
          {kq.duLieu.items.map((vb) => {
            const so = nhanSoVaoSo(vb.number, vb.year);
            return (
              <tr key={vb.id} className={REGISTER_ROW}>
                <td className={cn(REGISTER_TD, "font-semibold text-navy tabular-nums")}>{so}</td>
                <td className={cn(REGISTER_TD, "whitespace-nowrap tabular-nums")}>{nhanNgayCoThe(vb.document_date)}</td>
                <td className={cn(REGISTER_TD, "whitespace-nowrap")}>{nhanLoaiVanBan(traTen(traLoai, vb.document_type))}</td>
                <td className={REGISTER_TD}>
                  <span className="block max-w-96 truncate text-navy">{vb.summary}</span>
                </td>
                <td className={REGISTER_TD}>
                  <span className="block max-w-56 truncate">{vb.recipient}</span>
                </td>
                <td className={cn(REGISTER_TD, "whitespace-nowrap")}>{vb.signer === undefined || vb.signer === "" ? "Không ghi" : vb.signer}</td>
                {coQuyenGhi && (
                  <td className={cn(REGISTER_TD, "py-1.5 align-middle")}>
                    {/* Sửa as an icon, "Gỡ khỏi sổ" with its words in "⋯" (spec v2 §7). Both labels
                        name the row by its NUMBER, never by recipient or summary (rule 3, #4). */}
                    <span className="flex items-center justify-end gap-1">
                      <IconButton
                        type="button"
                        label={`${NUT_SUA} văn bản đi số ${so}`}
                        onClick={() => thaoTac.sua(vb)}
                      >
                        <Pencil aria-hidden="true" />
                      </IconButton>
                      <ActionMenu
                        label={`Thao tác khác: văn bản đi số ${so}`}
                        items={outgoingRowMoreActions(vb, thaoTac)}
                      />
                    </span>
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

/**
 * The "⋯" items of one outgoing row: "Gỡ khỏi sổ", red. Drawn only with `document.create` — the
 * same gate as the whole actions column. Exported so tests read it without opening a Radix menu.
 */
export function outgoingRowMoreActions(doc: documents_vanBanDiRa, actions: ThaoTacDi): ActionMenuItem[] {
  return [{ kind: "item", id: "go", label: NUT_GO, icon: Trash2, tone: "danger", onSelect: () => actions.go(doc) }];
}

/* ---- biểu mẫu ------------------------------------------------------------------------------ */

/** Titles of the two entry forms. The removal form asks the specific question instead. */
const TIEU_DE_DI: Record<"them" | "sua", string> = {
  them: "Cấp số văn bản đi",
  sua: "Sửa văn bản đi",
};

/** Removal question (spec v2 §7), naming the document by its NUMBER only — it is also the `aria-label`. */
export function removeOutgoingQuestion(number: string): string {
  return `Gỡ văn bản đi số ${number} khỏi sổ?`;
}

/** Heading id of the issue / edit / removal dialog — its accessible name. */
const OUTGOING_FORM_TITLE_ID = "tieu-de-bieu-mau-van-ban-di";

/** The three forms as centred dialogs (500px), the incoming register's shape. */
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
  const submit = (e: FormEvent<HTMLFormElement>) => {
    e.preventDefault();
    onGui();
  };
  const dismiss = () => {
    if (!dangGui) onHuy();
  };

  const refusal =
    loi !== "" ? (
      <p className="thong-bao-loi m-0 sm:col-span-2" role="alert">
        {loi}
      </p>
    ) : null;

  if (dangMo.kieu === "go") {
    const question = removeOutgoingQuestion(nhanSoVaoSo(dangMo.vb.number, dangMo.vb.year));
    return (
      <ModalDialog titleId={OUTGOING_FORM_TITLE_ID} onDismiss={dismiss}>
        <form className="flex min-h-0 flex-col gap-4" aria-label={question} onSubmit={submit}>
          <ModalDialogHeader titleId={OUTGOING_FORM_TITLE_ID} title={question} />
          <div className="flex min-h-0 flex-col gap-4 overflow-y-auto">
            {/* CÂU QUAN TRỌNG NHẤT, đứng ĐÚNG CHỖ sắp bấm gỡ. */}
            <Notice tone="legal" icon={Trash2}>
              {CANH_BAO_GO_KHONG_TRA_SO}
            </Notice>
            <Field
              label={O_LY_DO_GO}
              htmlFor="o-ly-do-go-di"
              grow="auto"
              hint={<span id="giai-thich-ly-do-go-di">{GIAI_THICH_LY_DO_GO}</span>}
            >
              <input
                id="o-ly-do-go-di"
                name="lyDoGo"
                required
                value={ban.lyDoGo}
                onChange={(e) => datBan({ ...ban, lyDoGo: e.target.value })}
                aria-invalid={loi === LOI_THIEU_LY_DO_GO}
                aria-describedby="giai-thich-ly-do-go-di"
              />
            </Field>
            {refusal}
          </div>
          <div className="flex flex-wrap items-center justify-end gap-2">
            <Button type="button" variant="outline" onClick={onHuy} disabled={dangGui}>
              {NUT_HUY}
            </Button>
            <Button type="submit" variant="danger" disabled={dangGui} aria-busy={dangGui || undefined}>
              <BusyLabel busy={dangGui} label={NUT_XAC_NHAN_GO} busyText="Đang gỡ…" />
            </Button>
          </div>
        </form>
      </ModalDialog>
    );
  }

  const tieuDe = TIEU_DE_DI[dangMo.kieu];

  return (
    <ModalDialog titleId={OUTGOING_FORM_TITLE_ID} onDismiss={dismiss}>
      <form className="flex min-h-0 flex-col gap-4" aria-label={tieuDe} onSubmit={submit}>
        <ModalDialogHeader
          titleId={OUTGOING_FORM_TITLE_ID}
          title={tieuDe}
          description={
            dangMo.kieu === "sua" ? `Văn bản đi số ${nhanSoVaoSo(dangMo.vb.number, dangMo.vb.year)}` : undefined
          }
        />

        <div className="grid min-h-0 min-w-0 gap-3.5 overflow-y-auto sm:grid-cols-2">
          <Field label={O_NGAY_VAN_BAN} htmlFor="o-ngay-van-ban-di" grow="auto">
            <input
              id="o-ngay-van-ban-di"
              name="ngayVanBan"
              type="date"
              value={ban.ngayVanBan}
              onChange={(e) => datBan({ ...ban, ngayVanBan: e.target.value })}
            />
          </Field>

          <Field label={O_LOAI_VAN_BAN} htmlFor="o-loai-van-ban-di" kind="select" grow="auto">
            <select
              id="o-loai-van-ban-di"
              name="loaiVanBan"
              value={ban.loaiVanBan}
              onChange={(e) => datBan({ ...ban, loaiVanBan: e.target.value })}
            >
              <option value="">— Chọn loại —</option>
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
          </Field>

          <Field label={O_TRICH_YEU} htmlFor="o-trich-yeu-di" grow="auto" className="sm:col-span-2">
            <textarea
              id="o-trich-yeu-di"
              name="trichYeu"
              rows={2}
              className="py-2"
              value={ban.trichYeu}
              onChange={(e) => datBan({ ...ban, trichYeu: e.target.value })}
            />
          </Field>

          <Field label={O_NOI_NHAN} htmlFor="o-noi-nhan" grow="auto">
            <input
              id="o-noi-nhan"
              name="noiNhan"
              value={ban.noiNhan}
              onChange={(e) => datBan({ ...ban, noiNhan: e.target.value })}
            />
          </Field>

          <Field label={O_NGUOI_KY} htmlFor="o-nguoi-ky" grow="auto">
            <input
              id="o-nguoi-ky"
              name="nguoiKy"
              value={ban.nguoiKy}
              onChange={(e) => datBan({ ...ban, nguoiKy: e.target.value })}
            />
          </Field>

          {dangMo.kieu === "them" && (
            <Notice tone="neutral" className="sm:col-span-2">
              Hệ thống cấp số đi ngay khi lưu, và số đã cấp không bao giờ được cấp lại. Kiểm lại
              nội dung trước khi bấm {NUT_LUU}.
            </Notice>
          )}

          {refusal}
        </div>

        <div className="flex flex-wrap items-center justify-end gap-2">
          <Button type="button" variant="outline" onClick={onHuy} disabled={dangGui}>
            {NUT_HUY}
          </Button>
          <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui || undefined}>
            <BusyLabel busy={dangGui} label={NUT_LUU} busyText="Đang lưu…" />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}
