"use client";

import { Check, Crown, Minus, ShieldQuestion, TriangleAlert } from "lucide-react";
import { useEffect, useMemo, useState } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { ErrorState } from "@/components/ui/error-state";
import { Notice } from "@/components/ui/notice";
import { SkeletonRows } from "@/components/ui/skeleton";

import type { KetQua } from "@/lib/api/goi";
import { layMaTranQuyen, luuPhanQuyenVaiTro } from "@/lib/api/phan-quyen";
import type {
  identity_maTranQuyenRa,
  identity_nhomQuyenRa,
  identity_vaiTroCotRa,
} from "@/lib/api/schema.gen";

import { oDaCap, trangThaiMaTran, type BangDaCap } from "./ma-tran-quyen";
import { RoleTemplateSeedPanel } from "./role-template-seed-panel";
import { unheldPermissionWarnings } from "./role-templates";
import type { UnheldWarning } from "./role-templates";
import {
  CHU_THICH_BANG_SUA,
  CHU_THICH_BANG_XEM,
  GHI_CHU_CHI_XEM,
  HUONG_DAN_SUA,
  LY_DO_KHONG_TU_SUA,
  NHAN_LANH_DAO,
  NUT_DANG_LUU,
  NUT_HUY,
  NUT_LUU,
  nhanChuaCauHinh,
  nhanNutHuy,
  nhanNutLuu,
  nhanO,
  nhanOBatTat,
  nhanSoNguoiGiuVaiTro,
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
 * Ma trận phân quyền — `docs/ui-ux/14-cau-hinh.md §4`, tab "Phân quyền".
 * **Hàng = quyền** (gom theo nhóm) · **cột = vai trò**.
 *
 * ═════════════════════════════════════════════════════════════════════════════════════════
 * LƯU THEO TỪNG CỘT, KHÔNG LƯU TOÀN MA TRẬN (§12.5) — `PUT /api/v1/roles/{id}/permissions`.
 *
 * Mỗi cột có trạng thái sửa riêng (`sua-phan-quyen.ts`): nút `Lưu` chỉ bật khi cột ấy đã khác bản
 * máy chủ, bấm thì gửi đúng tập của cột ấy, và các cột khác — đã sửa hay chưa — không đi kèm. 200
 * thay cột bằng tập máy chủ trả; lỗi giữ nguyên phần cán bộ đã tick và hiện nguyên câu máy chủ
 * viết cạnh cột. Không tự thử lại.
 *
 * Ô chỉ thành `<input type="checkbox">` khi tài khoản giữ `admin.role` (`choSua`). Đó là tiện dụng:
 * ba ràng buộc thật — #13 người giữ cuối cùng, #14 không tự sửa vai trò mình, không cấp/gỡ khoá
 * mình không giữ — nằm ở máy chủ, và màn hình không dựng bản sao nào của chúng ngoài một chỗ: cột
 * của chính vai trò người đang đăng nhập được khoá sẵn kèm lý do (#14), vì phiên có phát mã vai trò
 * (`quyen-tab.ts`, `maVaiTroCuaToi`). Khoá ấy sai thì máy chủ vẫn trả 403.
 * ═════════════════════════════════════════════════════════════════════════════════════════
 *
 * KHÔNG HIỆN CON SỐ TỔNG NÀO — không "33 quyền", không "10 nhóm", không "8 vai trò". Danh mục
 * quyền nằm trong CSDL và khách còn có thể chốt thêm khoá; một con số cứng trên màn hình sẽ sai
 * vào đúng ngày ấy, lặng lẽ, vì không ai kiểm lại nó. (Tiêu đề đặc tả §4.2 ghi "43 quyền, 11
 * nhóm" trong khi bảng liệt kê ngay dưới nó có 33 khoá / 10 nhóm và CSDL khớp bảng — đúng kiểu
 * hỏng mà một con số chép tay gây ra.)
 *
 * KHÔNG DỮ LIỆU CÁ NHÂN (luật 3): đầu cột chỉ có hai số đếm, và màn hình này không gọi thêm tuyến
 * nào để đổi số đếm thành danh sách tên.
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
  /** Bumped after "Tạo tám vai trò mẫu" succeeds: the new columns exist only on the server. */
  const [reloadCount, setReloadCount] = useState(0);

  /**
   * MỘT LỜI GỌI CHO CẢ MA TRẬN — một lần khi mở màn, và một lần nữa sau mỗi lần gieo vai trò mẫu
   * thành công (`reloadCount`), không lần nào khác. Hàng, cột và ô đã cấp về trong cùng một phản hồi
   * vì ma trận chỉ đúng khi ba thứ ấy được đọc ở cùng một thời điểm (`lib/api/phan-quyen.ts`). Đọc
   * lại thì bản sửa chưa lưu của mọi cột bị dựng lại từ bản mới — cái giá được chấp nhận, vì cột mới
   * vừa gieo phải hiện ra đúng quyền máy chủ đã cấp.
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
  }, [reloadCount]);

  const trangThai = useMemo(() => trangThaiMaTran(phanHoi), [phanHoi]);
  /** From the SAVED grants only — see `unheldPermissionWarnings`. */
  const warnings = useMemo(
    () =>
      trangThai.pha === "coDuLieu"
        ? unheldPermissionWarnings(trangThai.nhom, trangThai.vaiTro, trangThai.daCap)
        : [],
    [trangThai],
  );

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
    if (b === null || !cotDaSua(b, vaiTroId) || b.dangLuu.has(vaiTroId)) return;
    datBanSua((x) => (x === null ? x : batDauLuu(x, vaiTroId)));
    // Thân dựng từ bản sửa TẠI LÚC BẤM, và chỉ cho cột này — `guiCot` gọi mạng đúng một lần.
    const kq = await guiCot(b, vaiTroId, luuPhanQuyenVaiTro);
    datBanSua((x) => (x === null ? x : ketThucLuu(x, vaiTroId, kq)));
  }

  return (
    // The prototype's composition (`RolePermissionMatrix`, ADR 0068 lần 5): under the page header, one
    // hint line, then the bordered matrix — no second "Phân quyền" title inside a card. The section keeps
    // its accessible name from the page `<h1>`'s words.
    <section className="tab-phan-quyen flex min-w-0 flex-col gap-3 [&>*]:my-0" aria-label="Phân quyền">
      {/* Câu hướng dẫn của đặc tả §4 khi sửa được; câu chỉ-xem khi không. */}
      <p className="ghi-chu m-0 text-[13px] text-ink-500">{choSua ? HUONG_DAN_SUA : GHI_CHU_CHI_XEM}</p>

      <div className="flex min-w-0 flex-col gap-3 empty:hidden [&>*]:my-0">

      {/* The seed button follows the same gate as the ticks: `admin.role`, the key the route
          declares. Convenience only — the server checks it, and #14, on the call. */}
      {choSua && (
        <RoleTemplateSeedPanel
          onSeeded={() => setReloadCount((n) => n + 1)}
          roles={trangThai.pha === "coDuLieu" ? trangThai.vaiTro : null}
        />
      )}

      <UnheldPermissionWarnings warnings={warnings} />
      </div>

      {trangThai.pha === "dangDoc" && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải ma trận phân quyền…
          </p>
          <SkeletonRows rows={6} />
        </>
      )}

      {/* LỖI: hiện đúng `message` của máy chủ, không diễn giải, không rẽ nhánh theo `code`, không
          hiện `trace_id` (`lib/api/goi.ts`). 403 ở đây là ca thật: quyền `admin.role` có thể vừa
          bị gỡ giữa lúc màn hình đang mở. */}
      {trangThai.pha === "khongDocDuoc" && (
        <ErrorState role="alert" title="Chưa tải được ma trận phân quyền" message={trangThai.thongBao} />
      )}

      {/* TRẠNG THÁI RỖNG, KHÔNG PHẢI TRẠNG THÁI LỖI — và không bao giờ là một bảng trống không có
          lời giải thích nào. Xem `nhan-ma-tran.ts`. */}
      {trangThai.pha === "chuaCauHinh" && (
        <EmptyState icon={ShieldQuestion} title={nhanChuaCauHinh(trangThai.thieu)} />
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
 * Bảng ma trận.
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * BỀ RỘNG NHỎ NHẤT ĐƯỢC HỖ TRỢ LÀ 320px, và ở đó một bảng 8 cột vai trò không vừa. Ba điều giữ
 * cho nó vẫn đọc được, cả ba nằm trong CSS đi kèm (`globals.css`, khối `.bang-phan-quyen`):
 *
 *   1. Cột đầu (tên quyền) GHIM TRÁI khi cuộn ngang — không có nó thì cuộn sang cột thứ tư là
 *      không còn biết mình đang ở hàng nào, và một dấu tích đọc nhầm hàng là đọc sai quyền.
 *   2. Hàng đầu (tên vai trò) GHIM TRÊN khi cuộn dọc — 33 hàng thì đầu cột trôi khỏi màn hình
 *      ngay ở nhóm thứ hai. Ghim được là vì vùng cuộn có TRẦN CHIỀU CAO; không có trần thì
 *      `sticky` không có gì để ghim vào (xem `.bang-cuon-ma-tran`).
 *   3. Mỗi nhóm quyền có một DẢI TIÊU ĐỀ riêng chạy ngang bảng, nên hàng nào cũng biết mình
 *      thuộc nhóm nào mà không phải cuộn ngược lên.
 *
 * Bảng CUỘN NGANG chứ không đổi thành thẻ: đổi `display` của `table`/`tr`/`td` làm mất ngữ nghĩa
 * bảng với trình đọc màn hình, mà đây đúng là dữ liệu dạng bảng — và là loại dữ liệu bảng cần
 * quan hệ hàng–cột nhất.
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
  return (
    // `role="region"` + `tabIndex` để vùng cuộn tới được bằng bàn phím — vùng này cuộn CẢ HAI
    // chiều, nên không tới được bằng bàn phím là 33 hàng đọc được bằng chuột thôi.
    // The prototype's frame: one bordered, rounded box that scrolls inside itself, never the page.
    <div
      className="bang-cuon bang-cuon-ma-tran m-0 min-w-0 rounded-card border border-line bg-surface shadow-none"
      role="region"
      aria-label="Ma trận phân quyền"
      tabIndex={0}
    >
      <table className="bang-phan-quyen">
        <caption className="an-thi-giac">
          {chinhSua === undefined ? CHU_THICH_BANG_XEM : CHU_THICH_BANG_SUA}
        </caption>
        <thead>
          <tr>
            <th scope="col" className="cot-quyen">
              Quyền
            </th>
            {vaiTro.map((vt) => (
              <th scope="col" key={vt.id} className="cot-vai-tro">
                <span className="ten-vai-tro">{vt.name}</span>
                {/*
                  `is_leader` CHỈ SINH RA NHÃN NÀY VÀ KHÔNG QUYẾT ĐỊNH GÌ KHÁC — không ẩn/hiện cột,
                  không đổi thứ tự, không đổi cách đọc một ô. Dùng nó để rẽ nhánh là mở một hệ
                  phân quyền THỨ HAI không đi qua `(tenant_id, vai trò, quyền)`, tức là đúng thứ
                  luật 5 cấm ở #2 và #3. Cấp bậc lãnh đạo là thông tin tổ chức, không phải thang
                  quyền (`service-identity/internal/domain/quyen.go`).
                */}
                {vt.is_leader && (
                  <Badge tone="info" icon={Crown} className="chip-lanh-dao">
                    {NHAN_LANH_DAO}
                  </Badge>
                )}
                {/* HAI SỐ ĐẾM, và số sau là tập con của số trước — câu chữ và lý do ở `nhan-ma-tran.ts`. */}
                <span className="dong-phu">{nhanSoNguoiGiuVaiTro(vt)}</span>
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
            <tr className="dai-nhom">
              <th scope="rowgroup" colSpan={vaiTro.length + 1}>
                {/* Chữ nằm trong một `<span>` GHIM TRÁI, không nằm thẳng trong `<th>`: dải nhóm
                    rộng bằng cả bảng, nên khi cuộn ngang thì chính cái dải ấy đứng yên còn chữ
                    trôi ra khỏi khung nhìn. Ghim chữ thì cuộn tới cột vai trò cuối cùng vẫn biết
                    mình đang ở nhóm nào. */}
                <span className="nhan-nhom">{n.name}</span>
              </th>
            </tr>
            {n.permissions.map((q) => (
              <tr key={q.code}>
                <th scope="row" className="cot-quyen">
                  {/* One line each, "…" + the full words on hover: every row stays the same height, so
                      a tick is read against the right row (layout lesson, ADR 0068 lần 5). */}
                  <span className="nhan-quyen truncate" title={q.label}>
                    {q.label}
                  </span>
                  {/* Khoá quyền hiện ngay dưới nhãn: nó là MỘT khoá phẳng `<nhóm>.<việc>` (luật 5,
                      bất biến 3b), là chuỗi mà máy chủ kiểm trên từng tuyến, và là thứ cán bộ đối
                      chiếu được khi hỏi "vì sao màn hình kia báo không có quyền". */}
                  <code className="dong-phu ma-khoa-quyen truncate">{q.code}</code>
                </th>
                {vaiTro.map((vt) =>
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

/**
 * The ADR 0055 warning lines, one per watched key no working role holds. Exported so the test
 * renders it with data it controls (the parent reads the API inside an effect).
 *
 * TEXT, NOT COLOUR: the sentence names the key and the consequence; the styling is a second signal.
 */
export function UnheldPermissionWarnings({ warnings }: { warnings: readonly UnheldWarning[] }) {
  if (warnings.length === 0) return null;
  return (
    <Notice tone="neutral" icon={TriangleAlert} role="note" className="[&_p]:m-0 [&_p+p]:mt-1">
      {warnings.map((w) => (
        <p key={w.code}>{w.sentence}</p>
      ))}
    </Notice>
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
 * THE PROTOTYPE'S RULE (ADR 0068 lần 5): `Lưu` · `Huỷ` appear ONLY on a column with unsaved changes
 * (or one being saved) — a column nobody touched has nothing to save, and eight greyed `Lưu` buttons
 * read as eight things waiting. The column of the signed-in admin's own role can never be changed
 * (#14), so it never shows them; it shows the reason instead. Câu lỗi mang `role="alert"` và nằm ngay
 * dưới nút của CHÍNH cột ấy — một câu lỗi chung ở đầu bảng không cho biết cột nào chưa lưu được.
 */
function DauCotSua({ vt, chinhSua }: { vt: identity_vaiTroCotRa; chinhSua: ChinhSuaMaTran }) {
  const { banSua } = chinhSua;
  const daSua = cotDaSua(banSua, vt.id);
  const dangLuu = banSua.dangLuu.has(vt.id);
  const cuaToi = laCotCuaToi(chinhSua, vt);
  const loi = banSua.loi.get(vt.id);

  return (
    <>
      {(daSua || dangLuu) && !cuaToi && (
        <span className="nut-cot">
          <Button
            type="button"
            variant="primary"
            size="sm"
            disabled={dangLuu}
            aria-busy={dangLuu}
            aria-label={nhanNutLuu(vt.name)}
            onClick={() => chinhSua.luu(vt.id)}
          >
            {dangLuu ? NUT_DANG_LUU : NUT_LUU}
          </Button>
          <Button
            type="button"
            variant="secondary"
            size="sm"
            disabled={dangLuu}
            aria-label={nhanNutHuy(vt.name)}
            onClick={() => chinhSua.huy(vt.id)}
          >
            {NUT_HUY}
          </Button>
        </span>
      )}
      {cuaToi && <span className="dong-phu">{LY_DO_KHONG_TU_SUA}</span>}
      {loi !== undefined && (
        <span className="thong-bao-loi loi-cot" role="alert">
          {loi}
        </span>
      )}
    </>
  );
}

/**
 * Một ô của ma trận ở chế độ XEM. **Không phải điều khiển** — không `<input>`, không `<button>`,
 * không sự kiện bấm, và CSS cố ý không cho nó con trỏ chuột hay hiệu ứng rê chuột nào.
 *
 * HAI TÍN HIỆU, KHÔNG PHẢI MÀU: dấu `✓` và `–` khác nhau về HÌNH DẠNG, nên người không phân biệt
 * được màu vẫn đọc ra, và người dùng trình đọc màn hình nghe được câu đầy đủ ("Đã cấp" / "Chưa
 * cấp") thay vì một ký tự — `aria-hidden` trên dấu, câu chữ trong lớp chỉ-đọc-màn-hình.
 */
function ODaCap({ daCap }: { daCap: boolean }) {
  return (
    <td className={daCap ? "o-da-cap" : "o-chua-cap"}>
      {/* Two SHAPES (a tick, a dash), not two colours; the words are for the screen reader. */}
      {daCap ? (
        <Check aria-hidden="true" focusable="false" strokeWidth={2.2} className="inline-block size-4 text-success-600" />
      ) : (
        <Minus aria-hidden="true" focusable="false" strokeWidth={1.8} className="inline-block size-4 text-ink-400" />
      )}
      <span className="an-thi-giac">{nhanO(daCap)}</span>
    </td>
  );
}

/**
 * Một ô ở chế độ SỬA: the prototype's toggle — a `<button aria-pressed>` drawing the same two SHAPES as
 * the read-only cell (a tick, a dash), named "{nhãn quyền} — {tên vai trò}" (ADR 0068 lần 5).
 *
 * `aria-pressed` is the state a screen reader announces with the name, so for that user the state is
 * never shape or colour alone; the name itself says which right, for which role.
 *
 * Ô đã khác bản máy chủ mang thêm lớp `o-da-doi` — cán bộ thấy được mình đã đổi những ô nào trước
 * khi bấm Lưu, thay vì phải nhớ. Màu không phải tín hiệu duy nhất: trạng thái của ô là chính dấu
 * tích, lớp ấy chỉ là phần nhắc thêm.
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
    <td className={`${daCap ? "o-da-cap" : "o-chua-cap"}${daDoi ? " o-da-doi" : ""}`}>
      <button
        type="button"
        className="o-bat-tat"
        aria-pressed={daCap}
        disabled={khoa}
        aria-label={nhan}
        title={nhan}
        onClick={batTat}
      >
        {daCap ? (
          <Check aria-hidden="true" focusable="false" strokeWidth={2.2} className="size-4 text-success-600" />
        ) : (
          <Minus aria-hidden="true" focusable="false" strokeWidth={1.8} className="size-4 text-ink-400" />
        )}
      </button>
    </td>
  );
}
