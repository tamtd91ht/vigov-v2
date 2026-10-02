"use client";

import { Building2, Network, Pencil, Plus, Trash2, Upload, UsersRound } from "lucide-react";
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";

import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import type { identity_boPhanRa } from "@/lib/api/schema.gen";
import { deleteOrgUnit } from "@/lib/api/so-do-to-chuc";
import { QUYEN_QUAN_LY_SO_DO, quyetDinhTheoKhoa } from "@/lib/quyen";
import { usePhien } from "@/features/phien/phien-hien-tai";

import {
  API_SO_DO,
  banSua,
  banThem,
  dungCay,
  guiBieuMau,
  idNutMo,
  luaChonCha,
  moThem,
  theNeo,
  type BanNhap,
  type DangMo,
  type DongPhang,
  type NutCay,
} from "./cay-bo-phan";
import {
  CAU_THIEU_QUYEN_GHI,
  CHON_KHONG_CO_CHA,
  DANG_TAI,
  GIAI_THICH_O_CHA_SUA,
  GIAI_THICH_O_MA_THEM,
  GIAI_THICH_O_TEN,
  GIAI_THICH_O_THU_TU,
  LOI_THU_TU,
  NUT_HUY,
  NUT_LUU,
  NUT_SUA_BO_PHAN,
  NUT_THEM_BO_PHAN,
  NUT_THEM_CON,
  O_CHA,
  O_MA,
  O_TEN,
  O_THU_TU,
  TIEU_DE_SO_DO,
  giaiThichMaKhongSua,
  nhanCayRong,
  nhanNutSua,
  nhanNutThemCon,
  nhanSoCanBo,
  tieuDeSua,
  tieuDeThemCon,
  tieuDeThemGoc,
} from "./nhan-so-do";
import { DELETE_REASON_ID, OrgUnitDeleteForm } from "./org-unit-delete-form";
import type { DeleteRefusal } from "./org-unit-delete-form";
import { DELETE_BUTTON, deleteButtonLabel, deletedSentence } from "./org-unit-delete";
import { IMPORT_BUTTON } from "./org-unit-import-flow";
import { OrgUnitImportPanel } from "./org-unit-import-panel";
import { kiemLyDoXoa } from "./tang-danh-muc";

/**
 * Tab "Sơ đồ tổ chức" — `docs/ui-ux/14-cau-hinh.md §1`: cây bộ phận của đơn vị; thêm, đổi tên,
 * dời sang bộ phận cha khác, đổi thứ tự, XOÁ MỀM kèm lý do (ADR 0056), và nhập từ Excel.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * CÂY HIỆN CHO MỌI TÀI KHOẢN, CHỈ NÚT GHI MỚI ẨN — cùng khuôn tab Danh mục, và vì cùng một lý do ở
 * máy chủ: `GET /api/v1/org-units` khai `any-authenticated` (tên bộ phận có ở ô phân công và bộ lọc
 * của mọi màn), còn hai tuyến ghi khai `RequirePermission("admin.org")`. Ẩn nút là TIỆN DỤNG, không
 * phải biện pháp: máy chủ kiểm khoá trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * ĐỌC LẠI SAU MỖI LẦN GHI, KHÔNG VÁ TẠI CHỖ. Phản hồi của hai tuyến ghi KHÔNG mang `staff_count`
 * (`bo_phan.go:53`), nên vẽ lại thẻ từ phản hồi ấy là in "0 cán bộ" cho một bộ phận mười hai người.
 *
 * XOÁ: máy chủ là bên chặn khi bộ phận còn cán bộ, bộ phận con hay hồ sơ đang mở ở phân hệ khác
 * (409 kèm số đếm từng loại; 503 khi chưa hỏi được phân hệ khác — KHÔNG BAO GIỜ là "đã xoá"). Nút
 * `🗑` vì vậy hiện trên mọi thẻ, kể cả thẻ "0 cán bộ": con số ấy chỉ là một trong năm thứ được đếm.
 *
 * MỘT BIỂU MẪU MỞ MỘT LÚC: mở biểu mẫu xoá thì đóng biểu mẫu thêm/sửa và ngược lại — hai bản nháp
 * mở cùng lúc trên màn 320px là một bản nháp không nhìn thấy.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
export function TabSoDoToChuc() {
  const [tai, datTai] = useState<TrangThaiTai>({ pha: "dangDoc" });
  const [lanDoc, datLanDoc] = useState(0);

  const [dangMo, datDangMo] = useState<DangMo | null>(null);
  const [ban, datBan] = useState<BanNhap>(banThem(""));
  const [loiTaiCho, datLoiTaiCho] = useState("");
  const [loiMayChu, datLoiMayChu] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [cauDaXong, datCauDaXong] = useState("");

  /**
   * Nút đã mở biểu mẫu — tiêu điểm trả về nó khi biểu mẫu đóng (Huỷ hoặc lưu xong). Giữ `id` chứ
   * không giữ phần tử: sau một lần lưu, cây được đọc lại và nút có thể là một phần tử MỚI.
   */
  const nutDaMo = useRef<string | null>(null);

  /** The unit whose delete form is open, and that form's state. */
  const [removing, setRemoving] = useState<identity_boPhanRa | null>(null);
  const [reason, setReason] = useState("");
  const [deleteLocalError, setDeleteLocalError] = useState("");
  const [refusal, setRefusal] = useState<DeleteRefusal | null>(null);
  const [deleting, setDeleting] = useState(false);
  const [importOpen, setImportOpen] = useState(false);

  const phien = usePhien();
  /** Ba trạng thái: chưa đọc xong phiên thì chưa vẽ nút ghi nào, cũng chưa nói "thiếu quyền". */
  const quyetDinhGhi = phien === null ? null : quyetDinhTheoKhoa(phien, QUYEN_QUAN_LY_SO_DO);
  const coQuyenGhi = quyetDinhGhi !== null && quyetDinhGhi.hien;

  useEffect(() => {
    let bo = false;
    // KHÔNG đặt lại về "đang đọc" trước lượt đọc lại: cây cũ còn trên màn hình trong lúc chờ, nên
    // nút vừa mở biểu mẫu vẫn còn đó để nhận lại tiêu điểm.
    layDanhMucBoPhan().then((kq) => {
      if (bo) return;
      datTai(kq.ok ? { pha: "xong", items: kq.duLieu.items } : { pha: "loi", thongBao: kq.thongBao });
    });
    return () => {
      bo = true;
    };
  }, [lanDoc]);

  // Mở biểu mẫu → tiêu điểm vào ô đầu tiên. Đóng → tiêu điểm về nút đã mở nó.
  useEffect(() => {
    if (removing !== null) {
      document.getElementById(DELETE_REASON_ID)?.focus();
      return;
    }
    if (dangMo !== null) {
      document.getElementById(O_TEN_ID)?.focus();
      return;
    }
    if (nutDaMo.current !== null) {
      document.getElementById(nutDaMo.current)?.focus();
      nutDaMo.current = null;
    }
  }, [dangMo, removing]);

  const items = useMemo(() => (tai.pha === "xong" ? tai.items : []), [tai]);
  const cay = useMemo(() => dungCay(items), [items]);

  const mo = useCallback((m: DangMo, banDau: BanNhap, idNut: string) => {
    nutDaMo.current = idNut;
    setRemoving(null);
    datDangMo(m);
    datBan(banDau);
    datLoiTaiCho("");
    datLoiMayChu("");
    datCauDaXong("");
  }, []);

  const thaoTac = useMemo<ThaoTacCay>(
    () => ({
      themGoc: () =>
        mo(moThem("", null, () => crypto.randomUUID()), banThem(""), idNutMo("themGoc")),
      themCon: (cha) =>
        mo(
          moThem(cha.id, cha.name, () => crypto.randomUUID()),
          banThem(cha.id),
          idNutMo("themCon", cha.id),
        ),
      sua: (bp) => mo({ kieu: "sua", bp }, banSua(bp), idNutMo("sua", bp.id)),
      xoa: (bp) => {
        nutDaMo.current = deleteButtonId(bp.id);
        datDangMo(null);
        setRemoving(bp);
        setReason("");
        setDeleteLocalError("");
        setRefusal(null);
        datCauDaXong("");
      },
    }),
    [mo],
  );

  const closeDelete = useCallback(() => {
    setRemoving(null);
    setDeleteLocalError("");
    setRefusal(null);
  }, []);

  const submitDelete = useCallback(() => {
    if (removing === null || deleting) return;
    const checked = kiemLyDoXoa(reason);
    if (!checked.ok) {
      setDeleteLocalError(checked.loi);
      return;
    }
    setDeleteLocalError("");
    setRefusal(null);
    setDeleting(true);
    const unit = removing;
    void deleteOrgUnit(unit.id, checked.giaTri).then((r) => {
      setDeleting(false);
      if (!r.ok) {
        // The form stays open with the reason as typed: once what the unit holds is moved, the
        // same click is the retry.
        setRefusal({ message: r.message, holdings: r.holdings });
        return;
      }
      // The card is gone after the reload, so focus has nowhere to return to.
      nutDaMo.current = null;
      setRemoving(null);
      datCauDaXong(deletedSentence(unit.name));
      datLanDoc((n) => n + 1);
    });
  }, [deleting, reason, removing]);

  const dong = useCallback(() => {
    datDangMo(null);
    datLoiTaiCho("");
    datLoiMayChu("");
  }, []);

  const gui = useCallback(() => {
    if (dangMo === null || dangGui) return;
    datLoiTaiCho("");
    datLoiMayChu("");
    datDangGui(true);
    void guiBieuMau(dangMo, ban, API_SO_DO).then((kq) => {
      datDangGui(false);
      if (kq.kieu === "loiTaiCho") {
        datLoiTaiCho(kq.loi);
        return;
      }
      if (kq.kieu === "loiMayChu") {
        // Biểu mẫu GIỮ NGUYÊN chữ đã gõ, và GIỮ NGUYÊN khoá chống trùng: lần bấm lại là lần thử lại
        // của CÙNG một lần thêm.
        datLoiMayChu(kq.thongBao);
        return;
      }
      datDangMo(null);
      datCauDaXong(kq.cau);
      datLanDoc((n) => n + 1);
    });
  }, [ban, dangGui, dangMo]);

  const bieuMau =
    dangMo === null ? null : (
      <BieuMauBoPhan
        dangMo={dangMo}
        ban={ban}
        datBan={datBan}
        luaChon={luaChonCha(cay, items, dangMo.kieu === "sua" ? dangMo.bp.id : null)}
        loiTaiCho={loiTaiCho}
        loiMayChu={loiMayChu}
        dangGui={dangGui}
        onGui={gui}
        onHuy={dong}
      />
    );
  const neo = dangMo === null ? undefined : theNeo(dangMo);
  const deleteForm =
    removing === null ? null : (
      <OrgUnitDeleteForm
        unit={removing}
        reason={reason}
        setReason={setReason}
        localError={deleteLocalError}
        refusal={refusal}
        sending={deleting}
        onSubmit={submitDelete}
        onCancel={closeDelete}
      />
    );

  return (
    <Card as="section" className="tab-so-do-to-chuc" aria-labelledby="tieu-de-so-do">
      <CardHeader>
        <CardTitle as="h2" id="tieu-de-so-do" className="flex items-center gap-2">
          <Network aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-[18px] shrink-0 text-brand-600" />
          {TIEU_DE_SO_DO}
        </CardTitle>
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-3 [&>*]:my-0">

      {/* FIRST LOAD (spec §8b): the sentence stays the live region; the eye gets placeholder rows. */}
      {tai.pha === "dangDoc" && (
        <>
          <p role="status" className="an-thi-giac">
            {DANG_TAI}
          </p>
          <SkeletonRows rows={4} columns={2} className="rounded-xl border border-line" />
        </>
      )}
      {cauDaXong !== "" && (
        <p role="status" className="text-sm font-medium text-success-600">
          {cauDaXong}
        </p>
      )}

      {quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-doc-duoc" && (
        <p className="thong-bao-loi" role="alert">
          {quyetDinhGhi.thongBao}
        </p>
      )}

      <KhungSoDo
        tai={tai}
        cay={cay}
        coQuyenGhi={coQuyenGhi}
        thieuQuyen={quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-du-quyen"}
        thaoTac={thaoTac}
        onOpenImport={() => setImportOpen(true)}
        bieuMauDauTab={
          importOpen && coQuyenGhi ? (
            <OrgUnitImportPanel
              onImported={() => datLanDoc((n) => n + 1)}
              onClose={() => setImportOpen(false)}
            />
          ) : neo === null ? (
            bieuMau
          ) : null
        }
        bieuMauTaiThe={
          removing !== null
            ? { id: removing.id, node: deleteForm }
            : neo !== undefined && neo !== null
              ? { id: neo, node: bieuMau }
              : null
        }
      />
      </CardContent>
    </Card>
  );
}

/** `id` của ô `Tên` — nơi tiêu điểm tới khi biểu mẫu mở. */
const O_TEN_ID = "o-ten-bo-phan";

type TrangThaiTai =
  | { pha: "dangDoc" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; items: readonly identity_boPhanRa[] };

/** Bốn thao tác mà nút trên tab gọi. */
export type ThaoTacCay = {
  readonly themGoc: () => void;
  readonly themCon: (cha: identity_boPhanRa) => void;
  readonly sua: (bp: identity_boPhanRa) => void;
  readonly xoa: (bp: identity_boPhanRa) => void;
};

/** `id` of a card's delete button — focus returns there when the delete form is cancelled. */
export function deleteButtonId(unitId: string): string {
  return `nut-xoa-bo-phan-${unitId}`;
}

/**
 * Thân tab: nút thêm, câu thiếu quyền, cây hoặc trạng thái rỗng/lỗi.
 *
 * THUẦN TRÌNH BÀY và XUẤT RA để `tab-so-do-to-chuc.test.tsx` kết xuất bằng `react-dom/server`:
 * câu "nút ghi ẩn khi thiếu `admin.org`" là một điều của JSX, và một phép quyết định đúng trong
 * module thuần không nói gì về việc JSX có vẽ theo nó hay không.
 */
export function KhungSoDo({
  tai,
  cay,
  coQuyenGhi,
  thieuQuyen,
  thaoTac,
  onOpenImport,
  bieuMauDauTab,
  bieuMauTaiThe,
}: {
  tai: TrangThaiTai;
  cay: readonly NutCay[];
  coQuyenGhi: boolean;
  thieuQuyen: boolean;
  thaoTac: ThaoTacCay;
  onOpenImport: () => void;
  bieuMauDauTab: ReactNode;
  bieuMauTaiThe: { id: string; node: ReactNode } | null;
}) {
  return (
    <>
      {coQuyenGhi && (
        <p className="cum-nut m-0 flex flex-wrap items-center justify-end gap-2">
          <Button type="button" variant="secondary" icon={<Upload aria-hidden="true" focusable="false" strokeWidth={1.8} />} onClick={onOpenImport}>
            {IMPORT_BUTTON}
          </Button>
          <Button
            type="button"
            variant="primary"
            id={idNutMo("themGoc")}
            icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={thaoTac.themGoc}
          >
            {NUT_THEM_BO_PHAN}
          </Button>
        </p>
      )}
      {/* Read-only is a normal state of an account, not an error: a neutral note, no role. */}
      {thieuQuyen && (
        <Notice tone="neutral" className="m-0">
          {CAU_THIEU_QUYEN_GHI}
        </Notice>
      )}

      {bieuMauDauTab}

      {/* LỖI ĐỌC: nguyên câu của máy chủ, không diễn giải, không rẽ nhánh theo `code`. */}
      {tai.pha === "loi" && (
        <ErrorState role="alert" title="Chưa tải được sơ đồ tổ chức" message={tai.thongBao} />
      )}

      {tai.pha === "xong" && cay.length === 0 && <EmptyState icon={Network} title={nhanCayRong(coQuyenGhi)} />}

      {tai.pha === "xong" && cay.length > 0 && (
        <CapBoPhan
          nut={cay}
          coQuyenGhi={coQuyenGhi}
          thaoTac={thaoTac}
          bieuMauTaiThe={bieuMauTaiThe}
          nhan={TIEU_DE_SO_DO}
        />
      )}
    </>
  );
}

/**
 * Một cấp của cây — danh sách lồng nhau, `<ul>` trong `<li>` của cha.
 *
 * DANH SÁCH LỒNG CHỨ KHÔNG PHẢI THỤT LỀ BẰNG CSS TRÊN MỘT DANH SÁCH PHẲNG: trình đọc màn hình đọc
 * được cấp của một `<ul>` lồng ("danh sách, cấp 2"), còn một lề trái chỉ người nhìn thấy mới đọc
 * được. Quan hệ cha–con là dữ liệu, không phải trang trí.
 */
function CapBoPhan({
  nut,
  coQuyenGhi,
  thaoTac,
  bieuMauTaiThe,
  nhan,
}: {
  nut: readonly NutCay[];
  coQuyenGhi: boolean;
  thaoTac: ThaoTacCay;
  bieuMauTaiThe: { id: string; node: ReactNode } | null;
  nhan?: string;
}) {
  return (
    <ul className="cay-bo-phan" aria-label={nhan}>
      {nut.map((n) => (
        <li key={n.bp.id}>
          <TheBoPhan bp={n.bp} coQuyenGhi={coQuyenGhi} thaoTac={thaoTac} />
          {bieuMauTaiThe !== null && bieuMauTaiThe.id === n.bp.id && bieuMauTaiThe.node}
          {n.con.length > 0 && (
            <CapBoPhan
              nut={n.con}
              coQuyenGhi={coQuyenGhi}
              thaoTac={thaoTac}
              bieuMauTaiThe={bieuMauTaiThe}
            />
          )}
        </li>
      ))}
    </ul>
  );
}

/**
 * Một thẻ bộ phận: tên, mã, số cán bộ, và — khi có `admin.org` — ba nút `＋`, `✎` và `🗑`.
 */
function TheBoPhan({
  bp,
  coQuyenGhi,
  thaoTac,
}: {
  bp: identity_boPhanRa;
  coQuyenGhi: boolean;
  thaoTac: ThaoTacCay;
}) {
  return (
    <div className="the-bo-phan">
      <div className="the-bo-phan-than">
        <span className="the-bo-phan-ten inline-flex items-center gap-2">
          <Building2 aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0 text-brand-600" />
          {bp.name}
        </span>
        {/* Mã font đẳng chiều: nó là slug đọc qua điện thoại, `l`/`1` phải phân biệt được. */}
        <span className="ma-muc the-bo-phan-ma">{bp.code}</span>
      </div>
      <span className="the-bo-phan-so inline-flex items-center gap-1.5">
        <UsersRound aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 shrink-0 text-ink-500" />
        {nhanSoCanBo(bp.staff_count)}
      </span>
      {coQuyenGhi && (
        // Icon + word on every button, Xoá included (spec v2 §7: an action with a consequence is never
        // icon-only). Same ids, names and handlers.
        <span className="cum-nut flex flex-wrap items-center gap-1.5">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            id={idNutMo("themCon", bp.id)}
            aria-label={nhanNutThemCon(bp.name)}
            icon={<Plus aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={() => thaoTac.themCon(bp)}
          >
            {NUT_THEM_CON}
          </Button>
          <Button
            type="button"
            variant="ghost"
            size="sm"
            id={idNutMo("sua", bp.id)}
            aria-label={nhanNutSua(bp.name)}
            icon={<Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={() => thaoTac.sua(bp)}
          >
            {NUT_SUA_BO_PHAN}
          </Button>
          <Button
            type="button"
            variant="danger"
            size="sm"
            id={deleteButtonId(bp.id)}
            aria-label={deleteButtonLabel(bp.name)}
            icon={<Trash2 aria-hidden="true" focusable="false" strokeWidth={1.8} />}
            onClick={() => thaoTac.xoa(bp)}
          >
            {DELETE_BUTTON}
          </Button>
        </span>
      )}
    </div>
  );
}

/**
 * Biểu mẫu thêm hoặc sửa một bộ phận.
 *
 * THUẦN TRÌNH BÀY: mọi giá trị vào qua `ban`, mọi thay đổi ra qua `datBan`, phép dựng thân yêu cầu
 * nằm ở `cay-bo-phan.ts`. XUẤT RA để kết xuất được nhánh "máy chủ vừa từ chối" — câu 409 về vòng
 * lặp phải ra tới trang NGUYÊN VĂN.
 *
 * Ô `Mã` CHỈ CÓ Ở BIỂU MẪU THÊM. Ở biểu mẫu sửa, mã hiện thành chữ: một ô nhập mã sửa được là một ô
 * hứa điều máy chủ sẽ từ chối 400 (luật 7, bất biến 3).
 */
export function BieuMauBoPhan({
  dangMo,
  ban,
  datBan,
  luaChon,
  loiTaiCho,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  dangMo: DangMo;
  ban: BanNhap;
  datBan: (b: BanNhap) => void;
  luaChon: readonly DongPhang[];
  loiTaiCho: string;
  loiMayChu: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const tieuDe =
    dangMo.kieu === "sua"
      ? tieuDeSua(dangMo.bp.name)
      : dangMo.tenCha === null
        ? tieuDeThemGoc()
        : tieuDeThemCon(dangMo.tenCha);

  return (
    <form
      className="form-danh-muc form-bo-phan grid min-w-0 gap-4 sm:grid-cols-2 [&>*]:m-0 [&>.cum-nut]:col-span-full [&>.thong-bao-loi]:col-span-full [&>h4]:col-span-full"
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
      onKeyDown={(e) => {
        // Esc đóng biểu mẫu như `Huỷ` — trừ khi đang gửi, để một lần gửi dở không mất dấu.
        if (e.key === "Escape" && !dangGui) onHuy();
      }}
    >
      <h4 className="text-[15px] font-semibold text-ink-900">{tieuDe}</h4>

      <div className="o-nhap">
        <label htmlFor={O_TEN_ID}>{O_TEN}</label>
        <input
          id={O_TEN_ID}
          name="ten"
          value={ban.ten}
          onChange={(e) => datBan({ ...ban, ten: e.target.value })}
          aria-describedby="giai-thich-ten-bo-phan"
        />
        <p className="ghi-chu" id="giai-thich-ten-bo-phan">
          {GIAI_THICH_O_TEN}
        </p>
      </div>

      {dangMo.kieu === "them" ? (
        <div className="o-nhap">
          <label htmlFor="o-ma-bo-phan">{O_MA}</label>
          <input
            id="o-ma-bo-phan"
            name="ma"
            value={ban.ma}
            onChange={(e) => datBan({ ...ban, ma: e.target.value })}
            aria-describedby="giai-thich-ma-bo-phan"
          />
          <p className="ghi-chu" id="giai-thich-ma-bo-phan">
            {GIAI_THICH_O_MA_THEM}
          </p>
        </div>
      ) : (
        <p className="ghi-chu">{giaiThichMaKhongSua(dangMo.bp.code)}</p>
      )}

      <div className="o-nhap">
        <label htmlFor="o-cha-bo-phan">{O_CHA}</label>
        <select
          id="o-cha-bo-phan"
          name="chaId"
          value={ban.chaId}
          onChange={(e) => datBan({ ...ban, chaId: e.target.value })}
          aria-describedby={dangMo.kieu === "sua" ? "giai-thich-cha-bo-phan" : undefined}
        >
          <option value="">{CHON_KHONG_CO_CHA}</option>
          {luaChon.map((d) => (
            <option key={d.bp.id} value={d.bp.id}>
              {/* Thụt bằng khoảng trắng không ngắt: ô chọn không nhận lề, và cấp phải thấy được. */}
              {"   ".repeat(d.cap)}
              {d.bp.name}
            </option>
          ))}
        </select>
        {dangMo.kieu === "sua" && (
          <p className="ghi-chu" id="giai-thich-cha-bo-phan">
            {GIAI_THICH_O_CHA_SUA}
          </p>
        )}
      </div>

      <div className="o-nhap">
        <label htmlFor="o-thu-tu-bo-phan">{O_THU_TU}</label>
        {/* `inputMode="numeric"` chứ không `type="number"`: cuộn chuột trên ô số đổi giá trị mà
            người dùng không biết. */}
        <input
          id="o-thu-tu-bo-phan"
          name="thuTu"
          inputMode="numeric"
          value={ban.thuTu}
          onChange={(e) => datBan({ ...ban, thuTu: e.target.value })}
          aria-invalid={loiTaiCho === LOI_THU_TU}
          aria-describedby="giai-thich-thu-tu-bo-phan"
        />
        <p className="ghi-chu" id="giai-thich-thu-tu-bo-phan">
          {GIAI_THICH_O_THU_TU}
        </p>
      </div>

      {/* HAI VÙNG LỖI RIÊNG: lỗi tại chỗ (chưa gửi gì) và câu của máy chủ, nguyên văn. */}
      {loiTaiCho !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiTaiCho}
        </p>
      )}
      {loiMayChu !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiMayChu}
        </p>
      )}

      <div className="cum-nut flex flex-wrap justify-end gap-2">
        <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui}>
          <BusyLabel busy={dangGui} label={NUT_LUU} busyText={BUSY_SAVING} />
        </Button>
        <Button type="button" variant="secondary" onClick={onHuy} disabled={dangGui}>
          {NUT_HUY}
        </Button>
      </div>
    </form>
  );
}
