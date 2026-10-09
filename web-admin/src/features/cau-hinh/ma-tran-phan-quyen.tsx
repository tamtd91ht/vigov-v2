"use client";

import { Check, Loader2, Lock, Minus, ShieldQuestion } from "lucide-react";
import { useEffect, useMemo, useState } from "react";
import { toast } from "sonner";

import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Skeleton } from "@/components/ui/skeleton";

import type { KetQua } from "@/lib/api/goi";
import { layMaTranQuyen, luuPhanQuyenVaiTro } from "@/lib/api/phan-quyen";
import type {
  identity_maTranQuyenRa,
  identity_nhomQuyenRa,
  identity_vaiTroCotRa,
} from "@/lib/api/schema.gen";

import { oDaCap, orderRoleColumns, trangThaiMaTran, type BangDaCap } from "./ma-tran-quyen";
import {
  CHU_THICH_BANG_SUA,
  CHU_THICH_BANG_XEM,
  LY_DO_KHONG_TU_SUA,
  NHAN_LANH_DAO,
  NUT_HUY,
  NUT_LUU,
  nhanChuaCauHinh,
  nhanNutHuy,
  nhanNutLuu,
  nhanO,
  nhanOBatTat,
  nhanSoNguoiGiuVaiTro,
  savedToast,
} from "./nhan-ma-tran";
import {
  banSuaMoi,
  batDauLuu,
  batTatO,
  bangHienThi,
  cotDaSua,
  guiCot,
  huyCot,
  ketThucLuu,
  type BanSua,
} from "./sua-phan-quyen";

/**
 * Ma trận phân quyền — `/nguoi-dung/phan-quyen`, theo prototype `vigov-require/apps/admin/src/
 * components/admin/RolePermissionMatrix.tsx` (thẻ B, 08/10/2026).
 * **Hàng = quyền** (gom theo nhóm) · **cột = vai trò**.
 *
 * ═════════════════════════════════════════════════════════════════════════════════════════
 * LƯU THEO TỪNG CỘT, KHÔNG LƯU TOÀN MA TRẬN (§12.5) — `PUT /api/v1/roles/{id}/permissions`.
 *
 * Mỗi cột có trạng thái sửa riêng (`sua-phan-quyen.ts`): nút `Lưu` chỉ hiện khi cột ấy đã khác bản
 * máy chủ, bấm thì gửi đúng tập của cột ấy, và các cột khác — đã sửa hay chưa — không đi kèm. 200
 * thay cột bằng tập máy chủ trả và báo bằng toast; lỗi giữ nguyên phần cán bộ đã tick và hiện nguyên
 * câu máy chủ viết cạnh cột. Không tự thử lại.
 *
 * Ô chỉ thành nút bật/tắt khi tài khoản giữ `admin.role` (`choSua`). Đó là tiện dụng: ba ràng buộc
 * thật — #13 người giữ cuối cùng, #14 không tự sửa vai trò mình, không cấp/gỡ khoá mình không giữ —
 * nằm ở máy chủ, và màn hình không dựng bản sao nào của chúng ngoài một chỗ: cột của chính vai trò
 * người đang đăng nhập được khoá sẵn kèm lý do (#14), vì phiên có phát mã vai trò (`quyen-tab.ts`,
 * `maVaiTroCuaToi`). Khoá ấy sai thì máy chủ vẫn trả 403.
 * ═════════════════════════════════════════════════════════════════════════════════════════
 *
 * KHÔNG HIỆN CON SỐ TỔNG NÀO — không "33 quyền", không "10 nhóm", không "8 vai trò". Danh mục
 * quyền nằm trong CSDL và khách còn có thể chốt thêm khoá; một con số cứng trên màn hình sẽ sai
 * vào đúng ngày ấy, lặng lẽ, vì không ai kiểm lại nó.
 *
 * KHÔNG DỮ LIỆU CÁ NHÂN (luật 3): đầu cột chỉ có một số đếm, và màn hình này không gọi thêm tuyến
 * nào để đổi số đếm thành danh sách tên.
 *
 * Owner decision 08/10/2026: the "Tạo tám vai trò mẫu" panel and the ADR 0055 unheld-permission
 * warning are NOT on this page any more — the prototype has neither.
 */
export function MaTranPhanQuyen({
  choSua,
  maVaiTroCuaToi,
}: {
  /** Tài khoản giữ `admin.role` — ô thành ô bấm. Tiện dụng, máy chủ vẫn kiểm. */
  choSua: boolean;
  /** `role.code` của phiên, để khoá sẵn cột của chính mình (#14). `null` = không biết, không khoá. */
  maVaiTroCuaToi: string | null;
}) {
  /** `null` là CHƯA ĐỌC XONG, khác hẳn "đọc xong và hỏng". Ba pha, không hai (`goi.ts`). */
  const [phanHoi, datPhanHoi] = useState<KetQua<identity_maTranQuyenRa> | null>(null);

  /**
   * MỘT LỜI GỌI CHO CẢ MA TRẬN, một lần khi mở màn. Hàng, cột và ô đã cấp về trong cùng một phản
   * hồi vì ma trận chỉ đúng khi ba thứ ấy được đọc ở cùng một thời điểm (`lib/api/phan-quyen.ts`).
   *
   * `bo` chặn một phản hồi đến muộn ghi vào một component đã rời màn hình.
   *
   * KHÔNG CÓ BỘ ĐỆM NÀO SỐNG QUA LẦN MỞ MÀN HÌNH: ma trận là của MỘT xã. Một biến ở mức module
   * giữ nó lại, trong một tiến trình phục vụ nhiều tên miền, là đúng hình dạng của một lần bảng
   * phân quyền xã này hiện trên màn hình xã khác (luật 1, cấm #1).
   */
  useEffect(() => {
    let bo = false;
    layMaTranQuyen().then((kq) => {
      if (!bo) datPhanHoi(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  const trangThai = useMemo(() => trangThaiMaTran(phanHoi), [phanHoi]);

  /**
   * Bản sửa, dựng lại MỖI LẦN có phản hồi đọc mới. `null` khi chưa có ma trận để sửa. Cặp
   * `[daCapGoc, banSua]` dựng theo khuôn "điều chỉnh state khi prop đổi" của React: so với bảng gốc
   * của lần trước ngay trong lúc vẽ, không cần một effect chạy sau.
   */
  const daCapGoc = trangThai.pha === "coDuLieu" ? trangThai.daCap : null;
  const [banSua, datBanSua] = useState<BanSua | null>(null);
  const [gocDaDung, datGocDaDung] = useState<BangDaCap | null>(null);
  if (daCapGoc !== gocDaDung) {
    datGocDaDung(daCapGoc);
    datBanSua(daCapGoc === null ? null : banSuaMoi(daCapGoc));
  }

  async function luuCot(vaiTroId: string) {
    const b = banSua;
    // One column at a time (prototype `savingRoleId !== null`): every Lưu/Huỷ is disabled while any
    // column saves, and this guard holds the same line if a click slips through.
    if (b === null || !cotDaSua(b, vaiTroId) || b.dangLuu.size > 0) return;
    datBanSua((x) => (x === null ? x : batDauLuu(x, vaiTroId)));
    // Thân dựng từ bản sửa TẠI LÚC BẤM, và chỉ cho cột này — `guiCot` gọi mạng đúng một lần.
    const kq = await guiCot(b, vaiTroId, luuPhanQuyenVaiTro);
    datBanSua((x) => (x === null ? x : ketThucLuu(x, vaiTroId, kq)));
    // Success is a toast (ADR 0068 lần 6 #4). A refusal stays inline under the column: it carries
    // the server's #13/#14 reason, and a toast disappears. A 200 for ANOTHER role is not a success —
    // `ketThucLuu` turns it into that column's error, so no toast either.
    if (kq.ok && kq.duLieu.role_id === vaiTroId && trangThai.pha === "coDuLieu") {
      const role = trangThai.vaiTro.find((v) => v.id === vaiTroId);
      if (role !== undefined) toast.success(savedToast(role.name));
    }
  }

  return (
    // The prototype's composition: under the page header, one hint line, then the bordered matrix,
    // `space-y-3`. The section keeps its accessible name from the page `<h1>`'s words.
    <section className="space-y-3" aria-label="Phân quyền">
      {/* Prototype `AccountWorkspace.tsx` `Loading`: three 44px bars while the matrix reads. */}
      {trangThai.pha === "dangDoc" && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải ma trận phân quyền…
          </p>
          <div className="space-y-2">
            <Skeleton className="h-11 w-full" />
            <Skeleton className="h-11 w-full" />
            <Skeleton className="h-11 w-full" />
          </div>
        </>
      )}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải, không rẽ nhánh theo `code`, không
          hiện `trace_id` (`lib/api/goi.ts`). 403 ở đây là ca thật: quyền `admin.role` có thể vừa
          bị gỡ giữa lúc màn hình đang mở. The prototype is silent on this state; kept. */}
      {trangThai.pha === "khongDocDuoc" && (
        <ErrorState role="alert" title="Chưa tải được ma trận phân quyền" message={trangThai.thongBao} />
      )}

      {/* TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI — và không bao giờ là một bảng trống không có
          lời giải thích nào. Xem `nhan-ma-tran.ts`. */}
      {trangThai.pha === "chuaCauHinh" && (
        <EmptyState icon={ShieldQuestion} title={nhanChuaCauHinh(trangThai.thieu)} />
      )}

      {/* The hint only when the account can edit (prototype :127-132); read-only prints nothing. */}
      {trangThai.pha === "coDuLieu" && choSua && (
        <p className="text-ink-muted text-[12px]">
          Bấm vào ô để bật hoặc tắt quyền, sau đó bấm <b>Lưu</b> ở đầu cột của vai trò đó.
        </p>
      )}

      {trangThai.pha === "coDuLieu" &&
        (choSua && banSua !== null ? (
          <BangMaTran
            nhom={trangThai.nhom}
            vaiTro={trangThai.vaiTro}
            daCap={bangHienThi(banSua)}
            chinhSua={{
              banSua,
              maVaiTroCuaToi,
              batTat: (vaiTroId, khoa) =>
                datBanSua((x) => (x === null ? x : batTatO(x, vaiTroId, khoa))),
              luu: (vaiTroId) => void luuCot(vaiTroId),
              huy: (vaiTroId) => datBanSua((x) => (x === null ? x : huyCot(x, vaiTroId))),
            }}
          />
        ) : (
          <BangMaTran nhom={trangThai.nhom} vaiTro={trangThai.vaiTro} daCap={trangThai.daCap} />
        ))}
    </section>
  );
}

/**
 * Bảng ma trận — the prototype's table (`RolePermissionMatrix.tsx:134-247`), class for class.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BỀ RỘNG NHỎ NHẤT ĐƯỢC HỖ TRỢ LÀ 320px, và ở đó một bảng 8 cột vai trò không vừa. Hai điều giữ
 * cho nó vẫn đọc được:
 *
 *   1. Cột đầu (tên quyền) GHIM TRÁI khi cuộn ngang — không có nó thì cuộn sang cột thứ tư là
 *      không còn biết mình đang ở hàng nào, và một dấu tích đọc nhầm hàng là đọc sai quyền.
 *   2. Mỗi nhóm quyền có một DẢI TIÊU ĐỀ riêng chạy ngang bảng.
 *
 * The frame scrolls HORIZONTALLY only and the header row is not sticky — the prototype's choice
 * (card B, P8): the page scrolls vertically, as on every other screen.
 *
 * Bảng CUỘN NGANG chứ không đổi thành thẻ: đổi `display` của `table`/`tr`/`td` làm mất ngữ nghĩa
 * bảng với trình đọc màn hình, mà đây đúng là dữ liệu dạng bảng.
 * ─────────────────────────────────────────────────────────────────────────────────────────
 */
// EXPORTED FOR ONE REASON, and it is worth the widened surface: this is the component that turns
// the server's answer into the rows a person reads, and until 2026-09-22 NOTHING rendered it in a
// test. `ma-tran-quyen.test.ts` proves the module shapes the data correctly; it cannot see whether
// this JSX uses that data or a list typed by hand. Replacing `n.permissions.map` with a hardcoded
// array stayed green — measured, not assumed.
//
// The parent reads the API inside `useEffect`, which `renderToStaticMarkup` never runs, so the
// property can only be pinned by rendering this half directly with data the test controls.
/** Phần sửa của bảng. Vắng mặt = bảng chỉ xem, không một ô bấm nào. */
export type ChinhSuaMaTran = {
  banSua: BanSua;
  maVaiTroCuaToi: string | null;
  batTat: (vaiTroId: string, khoa: string) => void;
  luu: (vaiTroId: string) => void;
  huy: (vaiTroId: string) => void;
};

export function BangMaTran({
  nhom,
  vaiTro,
  daCap,
  chinhSua,
}: {
  nhom: readonly identity_nhomQuyenRa[];
  vaiTro: readonly identity_vaiTroCotRa[];
  /** Thứ bảng HIỆN RA ở từng ô — ở chế độ sửa, bên gọi truyền `bangHienThi(banSua)`. */
  daCap: BangDaCap;
  chinhSua?: ChinhSuaMaTran;
}) {
  // DISPLAY ORDER ONLY (user 09/10/2026): by name A→Z, the system-administrator role last
  // (`orderRoleColumns`). The data, the saves and the server's order are untouched.
  const columns = orderRoleColumns(vaiTro);
  return (
    // `role="region"` + `tabIndex` để vùng cuộn ngang tới được bằng bàn phím.
    <div
      className="border-line overflow-x-auto rounded-[10px] border"
      role="region"
      aria-label="Ma trận phân quyền"
      tabIndex={0}
    >
      <table className="w-full border-collapse text-[12.5px]">
        <caption className="an-thi-giac">
          {chinhSua === undefined ? CHU_THICH_BANG_XEM : CHU_THICH_BANG_SUA}
        </caption>
        <thead>
          <tr>
            <th
              scope="col"
              className="bg-canvas border-line text-ink-muted sticky left-0 z-10 w-48 min-w-48 border-b px-4 py-3 text-left font-semibold"
            >
              Quyền
            </th>
            {columns.map((vt) => (
              <th
                scope="col"
                key={vt.id}
                className="bg-canvas border-line min-w-30 border-b px-3 py-3 text-center align-bottom"
              >
                <div className="text-navy font-semibold">{vt.name}</div>
                <div className="text-ink-muted mt-1 text-[10.5px] font-normal">{nhanSoNguoiGiuVaiTro(vt)}</div>
                {/*
                  `is_leader` CHỈ SINH RA NHÃN NÀY VÀ KHÔNG QUYẾT ĐỊNH GÌ KHÁC — không ẩn/hiện cột,
                  không đổi thứ tự, không đổi cách đọc một ô. Dùng nó để rẽ nhánh là mở một hệ
                  phân quyền THỨ HAI không đi qua `(tenant_id, vai trò, quyền)`, tức là đúng thứ
                  luật 5 cấm ở #2 và #3 (`service-identity/internal/domain/quyen.go`).

                  A plain span, not `Badge`: ours always draws a tone icon, and the prototype's
                  shadcn Badge here has none (card B, P14). The base classes are `Badge`'s own.
                */}
                {vt.is_leader && (
                  <span className="inline-flex h-5 w-fit shrink-0 items-center justify-center overflow-hidden rounded-4xl border border-solid px-2 py-0.5 leading-none font-medium whitespace-nowrap bg-violet/12 text-violet border-violet/25 mt-1.5 text-[9.5px]">
                    {NHAN_LANH_DAO}
                  </span>
                )}
                {chinhSua !== undefined && <DauCotSua vt={vt} chinhSua={chinhSua} />}
              </th>
            ))}
          </tr>
        </thead>

        {/*
          MỖI NHÓM MỘT `<tbody>`, và dải tiêu đề nhóm là một hàng trong chính nhóm ấy — nên quan hệ
          "quyền này thuộc nhóm kia" có thật trong cấu trúc bảng chứ không chỉ có trên hình.

          `key` theo tên nhóm: máy chủ gom nhóm bằng một bảng vị trí nên không phát ra hai dải cùng
          tên (`gomTheoNhom`). Tên nhóm hiện NGUYÊN VĂN thứ máy chủ trả — danh mục quyền là danh
          mục dùng chung, và dịch lại nhãn ở đây là dựng một nguồn sự thật thứ hai cho cùng một chữ.
        */}
        {nhom.map((n) => (
          <tbody key={n.name}>
            <tr>
              <th
                scope="rowgroup"
                colSpan={columns.length + 1}
                className="bg-canvas/70 border-line text-navy border-y px-4 py-2 text-left text-[11.5px] font-bold tracking-wide uppercase"
              >
                {n.name}
              </th>
            </tr>
            {n.permissions.map((q) => (
              <tr key={q.code} className="hover:bg-canvas/60">
                {/* Khoá quyền hiện ngay dưới nhãn: nó là MỘT khoá phẳng `<nhóm>.<việc>` (luật 5,
                    bất biến 3b), là chuỗi mà máy chủ kiểm trên từng tuyến. 192px column, the label on
                    ONE line (user 09/10/2026: rows ≈ 59px like the prototype) — a longer label widens
                    the column rather than wrapping or being cut. */}
                <th
                  scope="row"
                  className="border-line sticky left-0 z-10 w-48 min-w-48 border-b bg-white px-4 py-2.5 text-left font-normal"
                >
                  <div className="text-navy font-medium whitespace-nowrap">{q.label}</div>
                  <code className="text-ink-muted font-mono text-xs">{q.code}</code>
                </th>
                {columns.map((vt) =>
                  chinhSua === undefined ? (
                    <ODaCap key={vt.id} daCap={oDaCap(daCap, vt.id, q.code)} />
                  ) : (
                    <OBatTat
                      key={vt.id}
                      daCap={oDaCap(daCap, vt.id, q.code)}
                      daDoi={
                        oDaCap(daCap, vt.id, q.code) !== oDaCap(chinhSua.banSua.goc, vt.id, q.code)
                      }
                      khoa={khoaCot(chinhSua, vt)}
                      nhan={nhanOBatTat(q.label, vt.name)}
                      batTat={() => chinhSua.batTat(vt.id, q.code)}
                    />
                  ),
                )}
              </tr>
            ))}
          </tbody>
        ))}
      </table>
    </div>
  );
}

/** Cột không bấm được: đang lưu, hoặc là vai trò của chính người đang đăng nhập (#14). */
function khoaCot(c: ChinhSuaMaTran, vt: identity_vaiTroCotRa): boolean {
  return c.banSua.dangLuu.has(vt.id) || laCotCuaToi(c, vt);
}

function laCotCuaToi(c: ChinhSuaMaTran, vt: identity_vaiTroCotRa): boolean {
  return c.maVaiTroCuaToi !== null && c.maVaiTroCuaToi === vt.code;
}

/**
 * Phần sửa ở đầu một cột: `Lưu` · `Huỷ` · câu lỗi của máy chủ · lý do cột bị khoá.
 *
 * THE PROTOTYPE'S RULE (`RolePermissionMatrix.tsx:155-178`): `Lưu` · `Huỷ` appear ONLY on a column
 * with unsaved changes (or one being saved); while ANY column saves, every column's pair is disabled
 * and the saving one spins beside "Lưu". The column of the signed-in admin's own role can never be
 * changed (#14), so it never shows them; it shows the reason instead. Câu lỗi mang `role="alert"` và
 * nằm ngay dưới nút của CHÍNH cột ấy — một câu lỗi chung ở đầu bảng không cho biết cột nào chưa lưu.
 */
function DauCotSua({ vt, chinhSua }: { vt: identity_vaiTroCotRa; chinhSua: ChinhSuaMaTran }) {
  const { banSua } = chinhSua;
  const daSua = cotDaSua(banSua, vt.id);
  const dangLuu = banSua.dangLuu.has(vt.id);
  const anySaving = banSua.dangLuu.size > 0;
  const cuaToi = laCotCuaToi(chinhSua, vt);
  const loi = banSua.loi.get(vt.id);

  return (
    <>
      {(daSua || dangLuu) && !cuaToi && (
        <div className="mt-2 flex justify-center gap-1">
          <Button
            type="button"
            variant="primary"
            size="sm"
            className="h-7 px-2 text-[11px]"
            disabled={anySaving}
            aria-busy={dangLuu}
            aria-label={nhanNutLuu(vt.name)}
            onClick={() => chinhSua.luu(vt.id)}
          >
            {dangLuu && <Loader2 aria-hidden="true" focusable="false" className="size-3 animate-spin" />}
            {NUT_LUU}
          </Button>
          <Button
            type="button"
            variant="outline"
            size="sm"
            className="h-7 px-2 text-[11px]"
            disabled={anySaving}
            aria-label={nhanNutHuy(vt.name)}
            onClick={() => chinhSua.huy(vt.id)}
          >
            {NUT_HUY}
          </Button>
        </div>
      )}
      {/* A lock, not the sentence (user 09/10/2026: header ≤ 110px). The sentence is the lock's tooltip
          and, for a screen reader, its visually hidden text — never colour or shape alone. */}
      {cuaToi && (
        <span className="text-ink-muted mt-2 inline-flex justify-center" title={LY_DO_KHONG_TU_SUA}>
          <Lock aria-hidden="true" focusable="false" className="size-3.5" />
          <span className="an-thi-giac">{LY_DO_KHONG_TU_SUA}</span>
        </span>
      )}
      {loi !== undefined && (
        <div className="text-danger mt-1 text-[11px] font-normal" role="alert">
          {loi}
        </div>
      )}
    </>
  );
}

/** The prototype's two marks (`RolePermissionMatrix.tsx:208-218`): two SHAPES, not two colours. */
function Mark({ daCap }: { daCap: boolean }) {
  return daCap ? (
    <Check aria-hidden="true" focusable="false" className="text-leaf mx-auto size-4" />
  ) : (
    <Minus aria-hidden="true" focusable="false" className="text-ink-muted/40 mx-auto size-4" />
  );
}

const CELL_CLASS = "border-line border-b px-3 py-2.5 text-center";

/**
 * Một ô ở chế độ XEM — the icon alone (prototype :236-238). **Không phải điều khiển.** Người dùng
 * trình đọc màn hình nghe được câu đầy đủ ("Đã cấp" / "Chưa cấp") thay vì một ký tự.
 */
function ODaCap({ daCap }: { daCap: boolean }) {
  return (
    <td className={CELL_CLASS}>
      <Mark daCap={daCap} />
      <span className="an-thi-giac">{nhanO(daCap)}</span>
    </td>
  );
}

/**
 * Một ô ở chế độ SỬA: the prototype's toggle — a `<button aria-pressed>` drawing the same two marks,
 * named "{tên vai trò} — {nhãn quyền}". `aria-pressed` is the state a screen reader announces with
 * the name, so for that user the state is never shape or colour alone.
 *
 * Ô đã khác bản máy chủ mang thêm nền `bg-tangerine/12` — cán bộ thấy được mình đã đổi những ô nào
 * trước khi bấm Lưu. Ô của cột bị khoá (#14, hoặc đang lưu) là nút `disabled`, mờ đi.
 *
 * `border-0 bg-transparent p-0 shadow-none`: preflight is off, so a bare `<button>` keeps the UA's
 * border and grey fill — the frame round the glyph the prototype does not have (user 09/10/2026).
 * The hover is `bg-canvas`: the spec's `bg-surface` is the PAGE colour, which is `canvas` in this app
 * (`globals.css`), where `bg-surface` is the white card and would not show on a white row.
 */
function OBatTat({
  daCap,
  daDoi,
  khoa,
  nhan,
  batTat,
}: {
  daCap: boolean;
  daDoi: boolean;
  khoa: boolean;
  nhan: string;
  batTat: () => void;
}) {
  return (
    <td className={daDoi ? `${CELL_CLASS} bg-tangerine/12` : CELL_CLASS}>
      <button
        type="button"
        className="hover:bg-canvas mx-auto grid size-7 place-items-center rounded-md border-0 bg-transparent p-0 shadow-none disabled:cursor-not-allowed disabled:opacity-50"
        aria-pressed={daCap}
        disabled={khoa}
        aria-label={nhan}
        title={nhan}
        onClick={batTat}
      >
        <Mark daCap={daCap} />
      </button>
    </td>
  );
}
