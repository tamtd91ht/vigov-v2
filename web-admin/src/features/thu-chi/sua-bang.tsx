"use client";

import { useState, type FormEvent } from "react";

import { Button } from "@/components/ui/button";
import { Card, CardFooter, CardHeader, CardTitle } from "@/components/ui/card";
import { Field } from "@/components/ui/field";
import { BUSY_SAVING, BusyLabel } from "@/features/danh-ba/busy-label";
import type { finance_bangRa } from "@/lib/api/schema.gen";
import type { SuaBangVao } from "@/lib/api/thu-chi";

import { DON_VI_TINH, dungThanSuaBang, GOI_Y_DOI_DON_VI, laMaDonVi } from "./nhan-thu-chi";

/**
 * Biểu mẫu **Sửa thông tin bảng**: tiêu đề, `Luỹ kế đến`, đơn vị tính (§6 "Luỹ kế … nhập tay").
 *
 * BA TRƯỜNG, KHÔNG HƠN: `PATCH /budget-sheets/{id}` từ chối năm, loại, mã và bộ cột bằng 400 — đổi
 * những thứ ấy là đổi bảng này thành một bảng khác. Biểu mẫu chỉ gửi trường THẬT SỰ đổi
 * (`dungThanSuaBang`).
 *
 * BẢNG MANG ĐƠN VỊ CŨ CHƯA ÁNH XẠ ĐƯỢC (`unit` rỗng) THÌ Ô CHỌN KHÔNG CHỌN SẴN GÌ: chọn thay cán bộ
 * là đoán hệ số chia cho mọi con số của bảng — đúng điều máy chủ đã từ chối đoán.
 */
export function FormSuaBang({
  bang,
  dangGui,
  huy,
  luu,
}: {
  bang: finance_bangRa;
  dangGui: boolean;
  huy: () => void;
  luu: (than: SuaBangVao) => void;
}) {
  const [loi, datLoi] = useState<string | null>(null);

  return (
    <Card
      as="form"
      aria-labelledby="tieu-de-sua-bang"
      onSubmit={(e: FormEvent<HTMLFormElement>) => {
        e.preventDefault();
        const fd = new FormData(e.currentTarget);
        const dung = dungThanSuaBang(bang, {
          tieuDe: String(fd.get("title") ?? ""),
          luyKe: String(fd.get("cumulative_to") ?? ""),
          donVi: String(fd.get("unit") ?? ""),
        });
        if (!dung.ok) {
          datLoi(dung.thongBao);
          return;
        }
        datLoi(null);
        luu(dung.than);
      }}
    >
      <CardHeader>
        <CardTitle as="h3" id="tieu-de-sua-bang">
          Sửa thông tin bảng
        </CardTitle>
      </CardHeader>

      {/* Labels above, 40px controls, two columns from 640px (spec §6.3, §6.5). */}
      <div className="grid min-w-0 gap-4 p-4 sm:grid-cols-2">
        <Field
          label="Tiêu đề bảng (in trên đầu báo cáo)"
          htmlFor="sua-bang-title"
          grow="auto"
          className="sm:col-span-2"
        >
          <input
            id="sua-bang-title"
            name="title"
            className="o-nhap"
            type="text"
            required
            maxLength={300}
            defaultValue={bang.title}
          />
        </Field>

        <Field
          label="Luỹ kế đến"
          htmlFor="sua-bang-cumulative"
          grow="auto"
          hint="Để trống để bỏ mốc luỹ kế khỏi đầu báo cáo."
        >
          <input
            id="sua-bang-cumulative"
            name="cumulative_to"
            className="o-nhap"
            type="date"
            defaultValue={bang.cumulative_to ?? ""}
          />
        </Field>

        <Field label="Đơn vị tính" htmlFor="sua-bang-unit" kind="select" grow="auto" hint={GOI_Y_DOI_DON_VI}>
          <select
            id="sua-bang-unit"
            name="unit"
            required
            defaultValue={laMaDonVi(bang.unit) ? bang.unit : ""}
          >
            {!laMaDonVi(bang.unit) && <option value="">— Chọn đơn vị tính —</option>}
            {DON_VI_TINH.map((d) => (
              <option key={d.ma} value={d.ma}>
                {d.nhan}
              </option>
            ))}
          </select>
        </Field>

        {loi !== null && (
          <p className="thong-bao-loi m-0 sm:col-span-2" role="alert">
            {loi}
          </p>
        )}
      </div>

      <CardFooter className="justify-end">
        <Button type="button" variant="secondary" disabled={dangGui} onClick={huy}>
          Huỷ
        </Button>
        <Button type="submit" variant="primary" disabled={dangGui} aria-busy={dangGui || undefined}>
          <BusyLabel busy={dangGui} label="Lưu thông tin bảng" busyText={BUSY_SAVING} />
        </Button>
      </CardFooter>
    </Card>
  );
}
