"use client";

import { Building2, Pencil, Plus, Trash2, Users } from "lucide-react";
import { useCallback, useEffect, useId, useMemo, useRef, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { ErrorState } from "@/components/ui/error-state";
import { controlClass } from "@/components/ui/field";
import { ModalDialog, ModalDialogHeader } from "@/components/ui/modal-dialog";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import { cn } from "@/lib/cn";

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
  type BanNhap,
  type DangMo,
  type DongPhang,
  type NutCay,
} from "./cay-bo-phan";
import { ConfigImportButton } from "./config-import-button";
import { ConfigLoading, SMALL_BUTTON_CLASS, selectCls } from "./config-ui";
import { ORG_UNIT_IMPORT_TARGET } from "./excel-import-targets";
import {
  ADD_BUTTON,
  ADD_TITLE,
  CHON_KHONG_CO_CHA,
  DANG_TAI,
  DIALOG_DESCRIPTION,
  EDIT_TITLE,
  EMPTY_TREE,
  LOI_THU_TU,
  NAME_PLACEHOLDER,
  NAME_REQUIRED,
  NUT_HUY,
  NUT_LUU,
  NUT_THEM_BO_PHAN,
  O_CHA,
  O_TEN,
  O_THU_TU,
  TIEU_DE_SO_DO,
  TITLE_ADD_CHILD,
  TITLE_DELETE,
  TITLE_EDIT,
  nhanNutSua,
  nhanNutThemCon,
  nhanSoCanBo,
} from "./nhan-so-do";
import { deleteButtonLabel, deletedSentence } from "./org-unit-delete";
import { DELETE_REASON_ID, OrgUnitDeleteForm } from "./org-unit-delete-form";
import type { DeleteRefusal } from "./org-unit-delete-form";
import { DIALOG_FOOTER_CLASS, DIALOG_LABEL_CLASS } from "./org-unit-dialog-classes";
import { kiemLyDoXoa } from "./tang-danh-muc";

/**
 * Tab "Sơ đồ tổ chức" — spec Cấu hình `03-so-do-to-chuc.md` (ADR 0079), prototype
 * `vigov-require/apps/admin/src/components/admin/OrgChart.tsx`: cây bộ phận của đơn vị; thêm, đổi tên,
 * dời sang bộ phận khác, đổi thứ tự, XOÁ MỀM kèm lý do (ADR 0056), và nhập từ Excel.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * CÂY HIỆN CHO MỌI TÀI KHOẢN, CHỈ NÚT GHI MỚI ẨN: `GET /api/v1/org-units` khai `any-authenticated`
 * (tên bộ phận có ở ô phân công và bộ lọc của mọi màn), còn các tuyến ghi khai
 * `RequirePermission("admin.org")` — NOT the spec's `admin.user` (ADR 0079 "Giữ bất kể spec"). Ẩn nút
 * là TIỆN DỤNG, không phải biện pháp: máy chủ kiểm khoá trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * ĐỌC LẠI SAU MỖI LẦN GHI, KHÔNG VÁ TẠI CHỖ. Phản hồi của hai tuyến ghi KHÔNG mang `staff_count`
 * (`bo_phan.go:53`), nên vẽ lại thẻ từ phản hồi ấy là in "0 cán bộ" cho một bộ phận mười hai người.
 *
 * XOÁ: máy chủ là bên chặn khi bộ phận còn cán bộ, bộ phận con hay hồ sơ đang mở ở phân hệ khác
 * (409 kèm số đếm từng loại; 503 khi chưa hỏi được phân hệ khác — KHÔNG BAO GIỜ là "đã xoá"). Nút xoá
 * vì vậy hiện trên mọi thẻ, kể cả thẻ "0 cán bộ": con số ấy chỉ là một trong năm thứ được đếm. Spec 03
 * xoá không hỏi; ở đây vẫn hỏi lý do vì máy chủ đòi `reason` (luật 7, ADR 0079).
 *
 * MỘT HỘP THOẠI MỞ MỘT LÚC: mở hộp xoá thì đóng hộp thêm/sửa và ngược lại.
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

  /**
   * Nút đã mở hộp thoại — tiêu điểm trả về nó khi hộp đóng (Huỷ hoặc lưu xong). Giữ `id` chứ không
   * giữ phần tử: sau một lần lưu, cây được đọc lại và nút có thể là một phần tử MỚI.
   */
  const nutDaMo = useRef<string | null>(null);

  /** The unit whose delete dialog is open, and that dialog's state. */
  const [removing, setRemoving] = useState<identity_boPhanRa | null>(null);
  const [reason, setReason] = useState("");
  const [deleteLocalError, setDeleteLocalError] = useState("");
  const [refusal, setRefusal] = useState<DeleteRefusal | null>(null);
  const [deleting, setDeleting] = useState(false);

  const phien = usePhien();
  /** Ba trạng thái: chưa đọc xong phiên thì chưa vẽ nút ghi nào. */
  const quyetDinhGhi = phien === null ? null : quyetDinhTheoKhoa(phien, QUYEN_QUAN_LY_SO_DO);
  const coQuyenGhi = quyetDinhGhi !== null && quyetDinhGhi.hien;

  useEffect(() => {
    let bo = false;
    // KHÔNG đặt lại về "đang đọc" trước lượt đọc lại: cây cũ còn trên màn hình trong lúc chờ, nên
    // nút vừa mở hộp thoại vẫn còn đó để nhận lại tiêu điểm.
    layDanhMucBoPhan().then((kq) => {
      if (bo) return;
      datTai(kq.ok ? { pha: "xong", items: kq.duLieu.items } : { pha: "loi", thongBao: kq.thongBao });
    });
    return () => {
      bo = true;
    };
  }, [lanDoc]);

  // Mở hộp thoại → tiêu điểm vào ô đầu tiên. Đóng → tiêu điểm về nút đã mở nó.
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
        // The dialog stays open with the reason as typed: once what the unit holds is moved, the
        // same click is the retry.
        setRefusal({ message: r.message, holdings: r.holdings });
        return;
      }
      // The card is gone after the reload, so focus has nowhere to return to.
      nutDaMo.current = null;
      setRemoving(null);
      toast.success(deletedSentence(unit.name));
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
        // Hộp thoại GIỮ NGUYÊN chữ đã gõ, và GIỮ NGUYÊN khoá chống trùng: lần bấm lại là lần thử lại
        // của CÙNG một lần thêm. The server's sentence is shown verbatim — its 409 `org_unit_cycle`
        // covers "itself" and "a descendant" alike, so the spec's self-parent sentence is not substituted.
        datLoiMayChu(kq.thongBao);
        return;
      }
      datDangMo(null);
      toast.success(kq.cau);
      datLanDoc((n) => n + 1);
    });
  }, [ban, dangGui, dangMo]);

  return (
    // No card, no visible title: the tab IS the section, as in the prototype. The heading stays for
    // the outline and the region's name, out of view.
    <section className="min-w-0 space-y-3" aria-labelledby="tieu-de-so-do">
      <h2 id="tieu-de-so-do" className="an-thi-giac">
        {TIEU_DE_SO_DO}
      </h2>

      {quyetDinhGhi !== null && !quyetDinhGhi.hien && quyetDinhGhi.vi === "khong-doc-duoc" && (
        <p className="text-danger m-0 text-[12px] font-medium" role="alert">
          {quyetDinhGhi.thongBao}
        </p>
      )}

      <KhungSoDo
        tai={tai}
        cay={cay}
        coQuyenGhi={coQuyenGhi}
        thaoTac={thaoTac}
        onImported={() => datLanDoc((n) => n + 1)}
      />

      {dangMo !== null && (
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
      )}
      {removing !== null && (
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
      )}
    </section>
  );
}

/** `id` của ô `Tên` — nơi tiêu điểm tới khi hộp thoại mở. */
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

/** `id` of a card's delete button — focus returns there when the delete dialog is cancelled. */
export function deleteButtonId(unitId: string): string {
  return `nut-xoa-bo-phan-${unitId}`;
}

/**
 * Thân tab (spec 03): hàng "Nhập từ Excel", nút "Thêm bộ phận", rồi cây — hoặc trạng thái đang tải /
 * rỗng / lỗi.
 *
 * THUẦN TRÌNH BÀY và XUẤT RA để `tab-so-do-to-chuc.test.tsx` kết xuất bằng `react-dom/server`:
 * câu "nút ghi ẩn khi thiếu `admin.org`" là một điều của JSX, và một phép quyết định đúng trong
 * module thuần không nói gì về việc JSX có vẽ theo nó hay không.
 *
 * No read-only notice (spec 02/ADR 0079): an account without `admin.org` simply sees the tree.
 */
export function KhungSoDo({
  tai,
  cay,
  coQuyenGhi,
  thaoTac,
  onImported,
}: {
  tai: TrangThaiTai;
  cay: readonly NutCay[];
  coQuyenGhi: boolean;
  thaoTac: ThaoTacCay;
  onImported: () => void;
}) {
  return (
    <>
      {coQuyenGhi && (
        <>
          {/* Draws its own `mb-3 flex justify-end` row (spec 02); the dialog title is the target's,
              "Nhập sơ đồ tổ chức từ Excel". */}
          <ConfigImportButton target={ORG_UNIT_IMPORT_TARGET} onImported={onImported} />
          {/* While loading the prototype draws only the import row and the skeletons
              (ConfigWorkspace.tsx:86, :173-180): the add button belongs to the loaded tree. */}
          {tai.pha !== "dangDoc" && (
            <div className="flex justify-end">
              <Button
                type="button"
                variant="primary"
                size="sm"
                className={SMALL_BUTTON_CLASS}
                id={idNutMo("themGoc")}
                icon={<Plus aria-hidden="true" focusable="false" className="size-4" />}
                onClick={thaoTac.themGoc}
              >
                {NUT_THEM_BO_PHAN}
              </Button>
            </div>
          )}
        </>
      )}

      {tai.pha === "dangDoc" && <ConfigLoading label={DANG_TAI} />}

      {/* LỖI ĐỌC: nguyên câu của máy chủ, không diễn giải, không rẽ nhánh theo `code`. */}
      {tai.pha === "loi" && (
        <ErrorState role="alert" title="Chưa tải được sơ đồ tổ chức" message={tai.thongBao} />
      )}

      {tai.pha === "xong" && cay.length === 0 && (
        <p className="text-ink-muted m-0 py-8 text-center text-[13px]">{EMPTY_TREE}</p>
      )}

      {tai.pha === "xong" && cay.length > 0 && (
        // ~4px between cards and ~65px per card (`space-y-1`, `py-2.5` below): the prototype as measured at
        // 1534px (user decision 09/10/2026, spec §3) — not `OrgChart.tsx`'s `space-y-2 py-3` as written.
        <ul className="m-0 list-none space-y-1 p-0" aria-label={TIEU_DE_SO_DO}>
          {cay.map((n) => (
            <OrgUnitBranch key={n.bp.id} node={n} coQuyenGhi={coQuyenGhi} thaoTac={thaoTac} />
          ))}
        </ul>
      )}
    </>
  );
}

/**
 * One unit and, under it, its sub-units — the prototype's recursive `TreeBranch`
 * (`OrgChart.tsx:124-219`).
 *
 * NESTED LISTS, NOT A FLAT LIST INDENTED BY CSS: a screen reader announces the level of a nested
 * `<ul>` ("list, level 2"), while a left margin is only read by the eye. The parent–child relation is
 * data, not decoration. The dashed left rule is the prototype's (`OrgChart.tsx:206`).
 */
function OrgUnitBranch({
  node,
  coQuyenGhi,
  thaoTac,
}: {
  node: NutCay;
  coQuyenGhi: boolean;
  thaoTac: ThaoTacCay;
}) {
  return (
    <li className="m-0">
      <TheBoPhan bp={node.bp} coQuyenGhi={coQuyenGhi} thaoTac={thaoTac} />
      {node.con.length > 0 && (
        // Preflight is off: `border-0` zeroes the three other sides before `border-l-2` sets the left.
        <ul className="border-line m-0 mt-1 ml-6 list-none space-y-1 border-0 border-l-2 border-dashed p-0 pl-5">
          {node.con.map((c) => (
            <OrgUnitBranch key={c.bp.id} node={c} coQuyenGhi={coQuyenGhi} thaoTac={thaoTac} />
          ))}
        </ul>
      )}
    </li>
  );
}

/**
 * One unit card (`OrgChart.tsx:154-203`): icon tile · name over code · headcount · and, with
 * `admin.org`, three small outline icon buttons. `flex-wrap` + `min-w-0` only matter at 320px, where a
 * nested card would otherwise push its buttons out of view.
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
    <div className="border-line shadow-card flex flex-wrap items-center gap-3 rounded-[10px] border border-solid bg-white px-4 py-2.5">
      <span aria-hidden="true" className="bg-navy/8 text-navy grid size-9 shrink-0 place-items-center rounded-[9px]">
        <Building2 focusable="false" className="size-4" />
      </span>
      <div className="min-w-0 flex-1">
        <div className="text-navy text-[13px] font-semibold break-words">{bp.name}</div>
        <code className="text-ink-muted text-[11px]">{bp.code}</code>
      </div>
      <span className="text-ink-muted flex shrink-0 items-center gap-1.5 text-[12px]">
        <Users aria-hidden="true" focusable="false" className="size-3.5" />
        {nhanSoCanBo(bp.staff_count)}
      </span>
      {coQuyenGhi && (
        <div className="flex items-center gap-1.5">
          <Button
            type="button"
            variant="outline"
            size="sm"
            className={SMALL_BUTTON_CLASS}
            id={idNutMo("themCon", bp.id)}
            title={TITLE_ADD_CHILD}
            aria-label={nhanNutThemCon(bp.name)}
            onClick={() => thaoTac.themCon(bp)}
          >
            <Plus aria-hidden="true" focusable="false" className="size-3.5" />
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className={SMALL_BUTTON_CLASS}
            id={idNutMo("sua", bp.id)}
            title={TITLE_EDIT}
            aria-label={nhanNutSua(bp.name)}
            onClick={() => thaoTac.sua(bp)}
          >
            <Pencil aria-hidden="true" focusable="false" className="size-3.5" />
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className={cn(SMALL_BUTTON_CLASS, "text-danger")}
            id={deleteButtonId(bp.id)}
            title={TITLE_DELETE}
            aria-label={deleteButtonLabel(bp.name)}
            onClick={() => thaoTac.xoa(bp)}
          >
            <Trash2 aria-hidden="true" focusable="false" className="size-3.5" />
          </Button>
        </div>
      )}
    </div>
  );
}

/**
 * Hộp thoại thêm hoặc sửa một bộ phận — the prototype's `OrgUnitDialog` (`OrgChart.tsx:221-339`).
 *
 * THUẦN TRÌNH BÀY: mọi giá trị vào qua `ban`, mọi thay đổi ra qua `datBan`, phép dựng thân yêu cầu
 * nằm ở `cay-bo-phan.ts`. XUẤT RA để kết xuất được nhánh "máy chủ vừa từ chối" — câu 409 về vòng
 * lặp phải ra tới trang NGUYÊN VĂN.
 *
 * NO CODE FIELD (spec 03): the server derives the code from the name when none is sent
 * (`service-identity/internal/store/bo_phan_ghi.go`, `MaCungGoc`), and an issued code never changes.
 * `Thứ tự` stays after `Trực thuộc` — owner decision 3 of ADR 0079.
 *
 * `Trực thuộc` lists every unit flat, minus — when editing — the unit itself AND its descendants
 * (`luaChonCha`): picking one of those is a cycle the server would refuse anyway.
 *
 * Enter in any field submits the `<form>`; Esc and ✕ ask `onHuy`, refused while a send is in flight.
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
  const titleId = useId();
  const editing = dangMo.kieu === "sua";
  const title = editing ? EDIT_TITLE : ADD_TITLE;

  return (
    <ModalDialog
      titleId={titleId}
      onDismiss={() => {
        if (!dangGui) onHuy();
      }}
      closeDisabled={dangGui}
      className="sm:max-w-lg"
    >
      <ModalDialogHeader titleId={titleId} title={title} description={DIALOG_DESCRIPTION} />
      <form
        className="m-0 flex min-h-0 min-w-0 flex-col gap-4"
        aria-label={title}
        onSubmit={(e) => {
          e.preventDefault();
          onGui();
        }}
      >
        <div className="min-h-0 space-y-4 overflow-y-auto">
          <div className="block">
            <label htmlFor={O_TEN_ID} className={DIALOG_LABEL_CLASS}>
              {O_TEN}
            </label>
            <input
              id={O_TEN_ID}
              name="ten"
              value={ban.ten}
              placeholder={NAME_PLACEHOLDER}
              onChange={(e) => datBan({ ...ban, ten: e.target.value })}
              aria-invalid={loiTaiCho === NAME_REQUIRED}
              className={cn(controlClass, "mt-1.5")}
            />
          </div>

          <div className="block">
            <label htmlFor="o-cha-bo-phan" className={DIALOG_LABEL_CLASS}>
              {O_CHA}
            </label>
            <select
              id="o-cha-bo-phan"
              name="chaId"
              value={ban.chaId}
              onChange={(e) => datBan({ ...ban, chaId: e.target.value })}
              className={cn(selectCls, "mt-1.5 h-9 w-full min-w-0 pr-8 text-[13px]")}
            >
              <option value="">{CHON_KHONG_CO_CHA}</option>
              {luaChon.map((d) => (
                <option key={d.bp.id} value={d.bp.id}>
                  {d.bp.name}
                </option>
              ))}
            </select>
          </div>

          <div className="block">
            <label htmlFor="o-thu-tu-bo-phan" className={DIALOG_LABEL_CLASS}>
              {O_THU_TU}
            </label>
            {/* `inputMode="numeric"` chứ không `type="number"`: cuộn chuột trên ô số đổi giá trị mà
                người dùng không biết. */}
            <input
              id="o-thu-tu-bo-phan"
              name="thuTu"
              inputMode="numeric"
              value={ban.thuTu}
              onChange={(e) => datBan({ ...ban, thuTu: e.target.value })}
              aria-invalid={loiTaiCho === LOI_THU_TU}
              className={cn(controlClass, "mt-1.5")}
            />
          </div>

          {/* HAI VÙNG LỖI RIÊNG, TẠI CHỖ: lỗi chưa gửi gì, và câu của máy chủ, nguyên văn. */}
          {loiTaiCho !== "" && (
            <p role="alert" className="text-danger m-0 text-[12px] font-medium">
              {loiTaiCho}
            </p>
          )}
          {loiMayChu !== "" && (
            <p role="alert" className="text-danger m-0 text-[12px] font-medium">
              {loiMayChu}
            </p>
          )}
        </div>

        <div className={DIALOG_FOOTER_CLASS}>
          <Button type="button" variant="outline" onClick={onHuy} disabled={dangGui}>
            {NUT_HUY}
          </Button>
          <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui}>
            <BusyLabel busy={dangGui} label={editing ? NUT_LUU : ADD_BUTTON} busyText={BUSY_SAVING} />
          </Button>
        </div>
      </form>
    </ModalDialog>
  );
}
