"use client";

import { Pencil, RotateCcw } from "lucide-react";
import { useCallback, useEffect, useState, type ReactNode } from "react";

import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { CardHeader } from "@/components/ui/card";
import { DATA_TABLE_CLASS, TableScroll } from "@/components/ui/data-table";
import { ErrorState } from "@/components/ui/error-state";
import { SkeletonRows } from "@/components/ui/skeleton";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";

import type { KetQua } from "@/lib/api/goi";
import type {
  petitions_danhSachTrangThaiNhiemVuRa,
  petitions_trangThaiNhiemVuRa,
} from "@/lib/api/schema.gen";
import { layTrangThaiNhiemVu, suaTrangThaiNhiemVu } from "@/lib/api/trang-thai-nhiem-vu";

import {
  GIAI_THICH_O_NHAN,
  NUT_HUY,
  NUT_LUU,
  NUT_SUA,
  O_NHAN,
  O_THU_TU,
  nhanNutCuaDong,
  nhanSoMuc,
} from "./nhan-danh-muc";
import {
  GHI_CHU_NHOM_TRANG_THAI,
  GIAI_THICH_THU_TU_TRANG_THAI,
  NUT_DAT_LAI,
  TIEU_DE_NHOM_TRANG_THAI,
  banTuTrangThai,
  chuMacDinh,
  daDatLaiTrangThai,
  daLuuTrangThai,
  nhanTuyChinh,
  nhanVaiTro,
  thanDatLaiMacDinh,
  thanSuaTrangThai,
  tieuDeSuaTrangThai,
  type BanNhapTrangThai,
} from "./trang-thai-nhiem-vu";

/**
 * Nhóm thứ tám của tab "Danh mục" — `Trạng thái nhiệm vụ` (quyết định #21, ADR 0035 §C).
 *
 * TỰ ĐỌC LẤY, MỘT LỜI GỌI: tuyến do `service-petitions` phục vụ, rời với bảy danh mục kia, nên nó
 * hỏng hay chậm thì chỉ khối này nói điều ấy — không kéo cả tab vào một pha tải chung.
 *
 * MỘT BIỂU MẪU CHO CẢ TAB VẪN GIỮ: mã đang sửa (`maDangSua`) do TAB giữ, và mở biểu mẫu ở đây thì
 * tab đóng biểu mẫu của bảy nhóm kia (`moSua`), và ngược lại. Hai bản nháp mở cùng lúc trên màn
 * 320px là hai bản nháp cán bộ không thấy hết.
 *
 * ẨN NÚT LÀ TIỆN DỤNG, KHÔNG PHẢI BIỆN PHÁP: tuyến PATCH khai `RequirePermission("admin.lookup")` và
 * kiểm trên TỪNG yêu cầu (luật 5, cấm #1). Bảng vẫn hiện cho mọi tài khoản — tuyến đọc là
 * `any-authenticated`.
 */
export function NhomTrangThaiNhiemVu({
  coQuyenGhi,
  maDangSua,
  moSua,
}: {
  coQuyenGhi: boolean;
  /** Mã đang mở biểu mẫu sửa, hoặc `null`. Do tab giữ — xem khối trên. */
  maDangSua: string | null;
  /** Mở (mã) hoặc đóng (`null`) biểu mẫu sửa của nhóm này. */
  moSua: (ma: string | null) => void;
}) {
  const [tai, datTai] = useState<KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null>(null);
  /** Tăng sau mỗi lần ghi thành công để ĐỌC LẠI — thứ tự hoà của máy chủ không vá tay được. */
  const [lanDoc, datLanDoc] = useState(0);
  const [ban, datBan] = useState<BanNhapTrangThai>({ nhan: "", thuTu: "" });
  const [loiTaiCho, datLoiTaiCho] = useState("");
  const [loiMayChu, datLoiMayChu] = useState("");
  const [dangGui, datDangGui] = useState(false);
  const [cauDaXong, datCauDaXong] = useState("");

  useEffect(() => {
    let bo = false;
    layTrangThaiNhiemVu().then((kq) => {
      if (!bo) datTai(kq);
    });
    return () => {
      bo = true;
    };
  }, [lanDoc]);

  const xong = useCallback(
    (cau: string) => {
      moSua(null);
      datLoiTaiCho("");
      datLoiMayChu("");
      datCauDaXong(cau);
      datLanDoc((n) => n + 1);
    },
    [moSua],
  );

  const moBieuMau = useCallback(
    (d: petitions_trangThaiNhiemVuRa) => {
      datBan(banTuTrangThai(d));
      datLoiTaiCho("");
      datLoiMayChu("");
      datCauDaXong("");
      moSua(d.code);
    },
    [moSua],
  );

  const datLai = useCallback(
    (d: petitions_trangThaiNhiemVuRa) => {
      if (dangGui) return;
      datLoiMayChu("");
      datCauDaXong("");
      datDangGui(true);
      void suaTrangThaiNhiemVu(d.code, thanDatLaiMacDinh(d)).then((kq) => {
        datDangGui(false);
        if (kq.ok) xong(daDatLaiTrangThai(d.code));
        else datLoiMayChu(kq.thongBao);
      });
    },
    [dangGui, xong],
  );

  const dongDangSua =
    tai !== null && tai.ok && maDangSua !== null
      ? (tai.duLieu.items.find((d) => d.code === maDangSua) ?? null)
      : null;

  const gui = useCallback(() => {
    if (dongDangSua === null || dangGui) return;
    const kiem = thanSuaTrangThai(dongDangSua, ban);
    if (!kiem.ok) {
      datLoiTaiCho(kiem.loi);
      return;
    }
    datLoiTaiCho("");
    datLoiMayChu("");
    datDangGui(true);
    void suaTrangThaiNhiemVu(dongDangSua.code, kiem.than).then((kq) => {
      datDangGui(false);
      if (kq.ok) xong(daLuuTrangThai(dongDangSua.code));
      else datLoiMayChu(kq.thongBao);
    });
  }, [ban, dangGui, dongDangSua, xong]);

  return (
    <KhoiTrangThaiNhiemVu
      tai={tai}
      coQuyenGhi={coQuyenGhi}
      dangGui={dangGui}
      cauDaXong={cauDaXong}
      // Lỗi máy chủ ở MỨC NHÓM chỉ khi không có biểu mẫu mở (lần `Đặt lại` hỏng); có biểu mẫu thì
      // câu ấy nằm trong biểu mẫu, cạnh ô cán bộ vừa gõ.
      loiNhom={dongDangSua === null ? loiMayChu : ""}
      onSua={moBieuMau}
      onDatLai={datLai}
      form={
        dongDangSua === null ? null : (
          <BieuMauSuaTrangThai
            dong={dongDangSua}
            ban={ban}
            datBan={datBan}
            loiTaiCho={loiTaiCho}
            loiMayChu={loiMayChu}
            dangGui={dangGui}
            onGui={gui}
            onHuy={() => {
              moSua(null);
              datLoiTaiCho("");
              datLoiMayChu("");
            }}
          />
        )
      }
    />
  );
}

/**
 * Phần trình bày của nhóm — THUẦN, xuất ra để `react-dom/server` kết xuất được trong bài kiểm.
 *
 * KHÔNG CÓ `+ Thêm mục`, `Tắt`, `Bật lại`, `Xoá` Ở BẤT KỲ NHÁNH NÀO (#21). Không vẽ nút mờ để dành
 * chỗ: một nút bấm vào không có gì xảy ra khiến cán bộ tin mình bấm sai. Câu
 * `GHI_CHU_NHOM_TRANG_THAI` nói lý do một lần, ngay dưới tiêu đề.
 */
export function KhoiTrangThaiNhiemVu({
  tai,
  coQuyenGhi,
  dangGui,
  cauDaXong,
  loiNhom,
  onSua,
  onDatLai,
  form,
}: {
  tai: KetQua<petitions_danhSachTrangThaiNhiemVuRa> | null;
  coQuyenGhi: boolean;
  dangGui: boolean;
  cauDaXong: string;
  loiNhom: string;
  onSua: (d: petitions_trangThaiNhiemVuRa) => void;
  onDatLai: (d: petitions_trangThaiNhiemVuRa) => void;
  form: ReactNode;
}) {
  const maTieuDe = "nhom-danh-muc-trangThaiNhiemVu";
  return (
    // Same card as the seven other groups (`NhomMuc` in `tab-danh-muc.tsx`).
    <section
      className="nhom-danh-muc m-0 min-w-0 overflow-hidden rounded-card border border-line bg-surface p-0 shadow-sm"
      aria-labelledby={maTieuDe}
    >
      <CardHeader className="m-0">
        <div className="min-w-0 flex-1 basis-64">
          <h3 id={maTieuDe} className="m-0 flex flex-wrap items-center gap-2 text-[15px] leading-snug font-semibold text-ink-900">
            {TIEU_DE_NHOM_TRANG_THAI}
            {tai !== null && tai.ok && (
              <span className="dem-muc text-xs font-medium text-ink-500">{nhanSoMuc(tai.duLieu.items.length)}</span>
            )}
          </h3>
          <p className="ghi-chu m-0 mt-1 text-[13px] text-ink-500">{GHI_CHU_NHOM_TRANG_THAI}</p>
        </div>
      </CardHeader>
      <div className="flex min-w-0 flex-col gap-3 p-4 empty:hidden [&>*]:my-0">

      {tai === null && (
        <>
          <p role="status" className="an-thi-giac">
            Đang tải trạng thái nhiệm vụ của đơn vị…
          </p>
          <SkeletonRows rows={4} className="rounded-xl border border-line" />
        </>
      )}
      {tai !== null && !tai.ok && (
        <ErrorState role="alert" title="Chưa tải được trạng thái nhiệm vụ" message={tai.thongBao} />
      )}
      {cauDaXong !== "" && (
        <p role="status" className="text-sm font-medium text-success-600">
          {cauDaXong}
        </p>
      )}
      {loiNhom !== "" && (
        <p className="thong-bao-loi" role="alert">
          {loiNhom}
        </p>
      )}

      {tai !== null && tai.ok && (
        <>
          <p className="ghi-chu text-[13px] text-ink-500">{GIAI_THICH_THU_TU_TRANG_THAI}</p>
          {/* `role="region"` + `tabIndex` để vùng cuộn ngang tới được bằng bàn phím ở 320px —
              cùng lý lẽ với bảng của bảy nhóm kia. */}
          <TableScroll sticky aria-label={`Danh mục ${TIEU_DE_NHOM_TRANG_THAI}`}>
            <table className={`bang-danh-muc ${DATA_TABLE_CLASS}`}>
              <caption className="an-thi-giac">
                Bảy trạng thái nhiệm vụ, theo thứ tự đơn vị đang dùng
              </caption>
              <thead>
                <tr>
                  <th scope="col">Mã</th>
                  <th scope="col">Nhãn hiển thị</th>
                  <th scope="col">Thứ tự</th>
                  <th scope="col">Vai trò</th>
                  <th scope="col">Tuỳ chỉnh</th>
                  {coQuyenGhi && (
                    <th scope="col">
                      <span className="an-thi-giac">Thao tác</span>
                    </th>
                  )}
                </tr>
              </thead>
              <tbody>
                {/* THỨ TỰ NHẬN ĐƯỢC LÀ THỨ TỰ VẼ — máy chủ đã sắp theo thứ tự hiệu lực. */}
                {tai.duLieu.items.map((d) => (
                  <tr key={d.code}>
                    <td className="ma-muc">{d.code}</td>
                    <td>{d.label}</td>
                    <td>{d.order}</td>
                    <td>{nhanVaiTro(d.role)}</td>
                    <td>
                      {/* Tone by the CODE (`customised`); icon + word, never colour alone. */}
                      <Badge tone={d.customised ? "info" : "neutral"}>
                        {nhanTuyChinh(d.customised)}
                      </Badge>
                      {d.customised && <span className="dong-phu"> {chuMacDinh(d)}</span>}
                    </td>
                    {coQuyenGhi && (
                      <td className="o-thao-tac">
                        <span className="cum-nut flex flex-wrap items-center gap-1.5">
                          <Button
                            type="button"
                            variant="ghost"
                            size="sm"
                            aria-label={nhanNutCuaDong(NUT_SUA, d.label)}
                            icon={<Pencil aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                            disabled={dangGui}
                            onClick={() => onSua(d)}
                          >
                            {NUT_SUA}
                          </Button>
                          {d.customised && (
                            <Button
                              type="button"
                              variant="ghost"
                              size="sm"
                              aria-label={nhanNutCuaDong(NUT_DAT_LAI, d.label)}
                              icon={<RotateCcw aria-hidden="true" focusable="false" strokeWidth={1.8} />}
                              disabled={dangGui}
                              onClick={() => onDatLai(d)}
                            >
                              {NUT_DAT_LAI}
                            </Button>
                          )}
                        </span>
                      </td>
                    )}
                  </tr>
                ))}
              </tbody>
            </table>
          </TableScroll>
        </>
      )}

      {form}
      </div>
    </section>
  );
}

/**
 * Biểu mẫu sửa MỘT trạng thái — nhãn và thứ tự, không gì khác.
 *
 * KHÔNG CÓ Ô MÃ (mã nằm trên đường dẫn, không đổi được) và KHÔNG CÓ Ô "ĐANG DÙNG" (#21: không có
 * `Tắt`). Mã hiện thành chữ ở tiêu đề phụ để cán bộ biết mình đang sửa dòng nào.
 */
export function BieuMauSuaTrangThai({
  dong,
  ban,
  datBan,
  loiTaiCho,
  loiMayChu,
  dangGui,
  onGui,
  onHuy,
}: {
  dong: petitions_trangThaiNhiemVuRa;
  ban: BanNhapTrangThai;
  datBan: (b: BanNhapTrangThai) => void;
  loiTaiCho: string;
  loiMayChu: string;
  dangGui: boolean;
  onGui: () => void;
  onHuy: () => void;
}) {
  const tieuDe = tieuDeSuaTrangThai(dong.label);
  return (
    <form
      className="form-danh-muc grid min-w-0 gap-4 sm:grid-cols-2 [&>*]:m-0 [&>.cum-nut]:col-span-full [&>.thong-bao-loi]:col-span-full [&>h4]:col-span-full [&>.ghi-chu]:col-span-full"
      aria-label={tieuDe}
      onSubmit={(e) => {
        e.preventDefault();
        onGui();
      }}
    >
      <h4 className="text-[15px] font-semibold text-ink-900">{tieuDe}</h4>
      <p className="ghi-chu">
        Mã <span className="ma-muc">{dong.code}</span> · {chuMacDinh(dong)}
      </p>

      <div className="o-nhap">
        <label htmlFor="o-nhan-trang-thai">{O_NHAN}</label>
        <input
          id="o-nhan-trang-thai"
          name="nhan"
          value={ban.nhan}
          onChange={(e) => datBan({ ...ban, nhan: e.target.value })}
          aria-describedby="giai-thich-nhan-trang-thai"
        />
        <p className="ghi-chu" id="giai-thich-nhan-trang-thai">
          {GIAI_THICH_O_NHAN}
        </p>
      </div>

      <div className="o-nhap">
        <label htmlFor="o-thu-tu-trang-thai">{O_THU_TU}</label>
        {/* `inputMode="numeric"`, không `type="number"` — cùng lý do với bảy nhóm kia. */}
        <input
          id="o-thu-tu-trang-thai"
          name="thuTu"
          inputMode="numeric"
          value={ban.thuTu}
          onChange={(e) => datBan({ ...ban, thuTu: e.target.value })}
          aria-invalid={loiTaiCho !== ""}
          aria-describedby={loiTaiCho !== "" ? "loi-trang-thai" : undefined}
        />
      </div>

      {loiTaiCho !== "" && (
        <p className="thong-bao-loi" id="loi-trang-thai" role="alert">
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
