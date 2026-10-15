"use client";

import { X } from "lucide-react";
import { useCallback, useEffect, useState, type ReactNode } from "react";
import { toast } from "sonner";

import { IconButton } from "@/components/ui/icon-button";
import { LargeDialog } from "@/components/ui/large-dialog";
import { bangTraTuKetQua, type BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc"; // vi-name-ok: existing catalogue lookup, imported not declared
import { ChiTietDuAn } from "@/features/giai-ngan/chi-tiet-du-an"; // vi-name-ok: existing project detail, imported not declared
import {
  ASSIGNED_TOAST,
  congThaoTac, // vi-name-ok: existing gate reader of the register, imported not declared
  danhBaTheoMa, // vi-name-ok: existing directory reader, imported not declared
  HIDDEN_TOAST,
  movedToast,
  PUBLISHED_TOAST,
  type DanhBaTheoMa, // vi-name-ok: existing directory type, imported not declared
} from "@/features/phan-anh/nhan-phieu";
import { ChiTietPhieu } from "@/features/phan-anh/so-phan-anh"; // vi-name-ok: existing petition drawer, imported not declared
import { usePhien } from "@/features/phien/phien-hien-tai"; // vi-name-ok: existing session hook
import { BAN_CHUYEN_TRONG, type BanChuyen } from "@/features/van-ban/nhan-van-ban"; // vi-name-ok: existing routing draft of the register
import { NganVanBanDen } from "@/features/van-ban/ngan-van-ban-den"; // vi-name-ok: existing incoming-document drawer
import { guiChuyenVanBan } from "@/features/van-ban/thao-tac-van-ban"; // vi-name-ok: existing routing act of the register
import { layDanhBaChonNguoi } from "@/lib/api/danh-ba-chon-nguoi"; // vi-name-ok: existing staff-directory client
import { layDanhMucBoPhan } from "@/lib/api/danh-muc"; // vi-name-ok: existing org-unit client
import { layLoaiVanBan } from "@/lib/api/danh-muc-nghiep-vu"; // vi-name-ok: existing document-type client
import type { KetQua } from "@/lib/api/goi"; // vi-name-ok: existing result type
import {
  chuyenCapTrenPhieu, // vi-name-ok: existing petition clients, imported not declared
  chuyenXuLyPhieu, // vi-name-ok: existing petition client
  dongPhieu, // vi-name-ok: existing petition client
  khongTiepNhanPhieu, // vi-name-ok: existing petition client
  layPhieuPhanAnh, // vi-name-ok: existing petition client
  phanLoaiPhieu, // vi-name-ok: existing petition client
  setPetitionPublication,
  tienTrangThaiPhieu, // vi-name-ok: existing petition client
} from "@/lib/api/phieu-phan-anh"; // vi-name-ok: existing petition-register client
import type {
  documents_danhSachLichSuChuyenRa, // vi-name-ok: generated contract type
  documents_vanBanDenRa, // vi-name-ok: generated contract type
  identity_boPhanRa, // vi-name-ok: generated contract type
  identity_danhBaChonNguoiRa, // vi-name-ok: generated contract type
  identity_thonToDanPhoRa, // vi-name-ok: generated contract type
  petitions_phieuPhanAnhRa, // vi-name-ok: generated contract type
} from "@/lib/api/schema.gen";
import { layDanhSachThonToDanPho } from "@/lib/api/thon-to-dan-pho"; // vi-name-ok: existing hamlet client
import { layLichSuChuyenVanBanDen, layVanBanDen } from "@/lib/api/van-ban"; // vi-name-ok: existing incoming-register client
import {
  coQuyen, // vi-name-ok: existing permission check
  QUYEN_CHUYEN_VAN_BAN, // vi-name-ok: existing permission constant
  QUYEN_DONG_PHAN_ANH, // vi-name-ok: existing permission constant
  QUYEN_PHAN_CONG_PHAN_ANH, // vi-name-ok: existing permission constant
  QUYEN_PHAN_LOAI_PHAN_ANH, // vi-name-ok: existing permission constant
  QUYEN_XEM_PHAN_ANH, // vi-name-ok: existing permission constant
  quyetDinhTheoKhoa, // vi-name-ok: existing permission decision
} from "@/lib/quyen";

/**
 * ONE RECORD of a Tổng quan list, opened IN PLACE — tester bug sheet, Tổng quan: "Popup số liệu của các
 * cụm Văn bản & Đơn thư, Giải ngân, Phản ánh của người dân: Khi click mở sẽ mở chi tiết item đó ngay
 * màn hình Tổng quan, không được mở ra màn hình menu chức năng". The prototype's `DrillDialog` links
 * `Mở` to the module (`DrillDialog.tsx:110-117`); the tester's sheet overrides that, as the customer's
 * sheet row 5 already did for tasks (`task-drill.tsx`).
 *
 * EACH REGISTER'S OWN DETAIL, NEVER A NEW ONE: the incoming document's drawer (`NganVanBanDen`), the
 * petition's drawer (`ChiTietPhieu`), the project's detail (`ChiTietDuAn`, in a right-hand panel here
 * because its own screen is a page). Each reads through its own detail route — the server checks the
 * read key and the commune again, and a record of another commune is the same 404 as a missing one.
 *
 * GATES ARE THE REGISTER'S, UNCHANGED: every write control is decided by the SAME key the register reads
 * (`document.route`; `feedback.classify` / `feedback.assign` / `feedback.resolve` through `congThaoTac`;
 * the project's own `budget.update` / `budget.confirm` inside `ChiTietDuAn`), from the same session, and
 * the server checks every write again (rule 5). ONE EXCEPTION, NARROWER, NOT WIDER: the incoming
 * document's `Sửa` / `Gỡ khỏi sổ` are not drawn here — their entry form belongs to the register
 * (`so-van-ban-den.tsx`), so they stay on the Văn bản screen.
 *
 * Rule 3: the dialog's NAME is the record's code (or number), never its free text.
 */
export type RecordDetailTarget =
  | { readonly kind: "incoming-document"; readonly id: string }
  | { readonly kind: "citizen-report"; readonly code: string }
  | { readonly kind: "project"; readonly id: string };

export function RecordDetail({
  target,
  onClose,
  onChanged,
}: {
  target: RecordDetailTarget;
  onClose: () => void;
  /** A write succeeded inside: the caller re-reads its rows and the page's figures once this closes. */
  onChanged: () => void;
}) {
  switch (target.kind) {
    case "incoming-document":
      return <IncomingDocumentDetail id={target.id} onClose={onClose} onChanged={onChanged} />;
    case "citizen-report":
      return <CitizenReportDetail code={target.code} onClose={onClose} onChanged={onChanged} />;
    case "project":
      return <ProjectDetail id={target.id} onClose={onClose} />;
  }
}

/* ── Văn bản đến ─────────────────────────────────────────────────────────────────────────── */

/** The register's wiring of its drawer (`so-van-ban-den.tsx`, `SoVanBanDen`), for ONE document. */
function IncomingDocumentDetail({
  id,
  onClose,
  onChanged,
}: {
  id: string;
  onClose: () => void;
  onChanged: () => void;
}) {
  const [reads, setReads] = useState(0);
  const [detail, setDetail] = useState<{
    id: string;
    vb: KetQua<documents_vanBanDenRa>;
    history: KetQua<documents_danhSachLichSuChuyenRa>;
  } | null>(null);
  const [types, setTypes] = useState<BangTraDanhMuc>({ pha: "dangDoc" });
  const [units, setUnits] = useState<BangTraDanhMuc>({ pha: "dangDoc" });
  const [directory, setDirectory] = useState<DanhBaTheoMa | null>(null);
  const [draft, setDraft] = useState<BanChuyen>(BAN_CHUYEN_TRONG);
  const [error, setError] = useState("");
  const [sending, setSending] = useState(false);
  const [done, setDone] = useState("");

  const session = usePhien();
  // Three states, as the register: an unread session draws no write control.
  const canRoute = session !== null && quyetDinhTheoKhoa(session, QUYEN_CHUYEN_VAN_BAN).hien;

  useEffect(() => {
    let dropped = false;
    // Document and timeline of the SAME read, assigned together (`SoVanBanDen`).
    Promise.all([layVanBanDen(id), layLichSuChuyenVanBanDen(id)]).then(([vb, history]) => {
      if (!dropped) setDetail({ id, vb, history });
    });
    return () => {
      dropped = true;
    };
  }, [id, reads]);

  useEffect(() => {
    let dropped = false;
    layLoaiVanBan().then((k) => {
      if (dropped) return;
      setTypes(
        bangTraTuKetQua(
          k.ok ? { ok: true, duLieu: { items: k.duLieu.items.map((m) => ({ id: m.code, name: m.label })) } } : k,
        ),
      );
    });
    layDanhMucBoPhan().then((k) => {
      if (!dropped) setUnits(bangTraTuKetQua(k));
    });
    layDanhBaChonNguoi().then((k) => {
      if (!dropped && k.ok) setDirectory(danhBaTheoMa(k.duLieu.items));
    });
    return () => {
      dropped = true;
    };
  }, []);

  const send = useCallback(() => {
    if (sending) return;
    setError("");
    setDone("");
    setSending(true);
    // The unit and reason checks live in `guiChuyenVanBan` (`thao-tac-van-ban.ts`), as on the register.
    void guiChuyenVanBan(id, draft).then((k) => {
      setSending(false);
      if (!k.ok) {
        setError(k.thongBao);
        return;
      }
      setDraft(BAN_CHUYEN_TRONG);
      setDone("Đã chuyển và ghi vào dòng thời gian.");
      setReads((n) => n + 1);
      onChanged();
    });
  }, [draft, id, onChanged, sending]);

  // Re-reading the SAME document after a routing keeps the previous answer on screen until the new one
  // arrives — the routing box is not torn down under the officer's hand.
  const shown = detail !== null && detail.id === id ? detail : null;
  return (
    <NganVanBanDen
      vb={shown?.vb ?? null}
      lichSu={shown?.history ?? null}
      bayGio={new Date()}
      traLoai={types}
      traBoPhan={units}
      coQuyenChuyen={canRoute}
      ban={draft}
      datBan={setDraft}
      loi={error}
      dangGui={sending}
      cauDaXong={done}
      onGui={send}
      onDong={onClose}
      danhBa={directory}
      coQuyenGhi={false}
    />
  );
}

/* ── Phản ánh của người dân ──────────────────────────────────────────────────────────────── */

/**
 * The register's wiring of its drawer (`so-phan-anh.tsx`, `SoPhanAnh`), for ONE petition read through
 * the detail route — as the register's `?id=` deep link does. Every act calls the SAME client the
 * register calls, with the same refusal placement and the same toasts.
 */
function CitizenReportDetail({
  code: initialCode,
  onClose,
  onChanged,
}: {
  code: string;
  onClose: () => void;
  onChanged: () => void;
}) {
  const [code, setCode] = useState(initialCode);
  const [loaded, setLoaded] = useState<{ code: string; result: KetQua<petitions_phieuPhanAnhRa> } | null>(null);
  const [units, setUnits] = useState<readonly identity_boPhanRa[]>([]);
  const [hamlets, setHamlets] = useState<readonly identity_thonToDanPhoRa[]>([]);
  const [directory, setDirectory] = useState<KetQua<identity_danhBaChonNguoiRa> | null>(null);
  const [writeError, setWriteError] = useState<string | null>(null);
  const [sending, setSending] = useState(false);
  const [closeRefusal, setCloseRefusal] = useState<string | null>(null);
  const [assignRefusal, setAssignRefusal] = useState<string | null>(null);
  const [classifyRefusal, setClassifyRefusal] = useState<string | null>(null);
  const [logReads, setLogReads] = useState(0);

  useEffect(() => {
    let dropped = false;
    layPhieuPhanAnh(code).then((result) => {
      if (!dropped) setLoaded({ code, result });
    });
    return () => {
      dropped = true;
    };
  }, [code]);

  useEffect(() => {
    let dropped = false;
    layDanhMucBoPhan().then((k) => {
      if (!dropped && k.ok) setUnits(k.duLieu.items);
    });
    layDanhSachThonToDanPho().then((k) => {
      if (!dropped && k.ok) setHamlets(k.duLieu.items);
    });
    layDanhBaChonNguoi().then((k) => {
      if (!dropped) setDirectory(k);
    });
    return () => {
      dropped = true;
    };
  }, []);

  const session = usePhien();
  // FAIL CLOSED: an unread or failed session holds no key (rule 1, forbidden #1).
  const permissions: readonly string[] = session !== null && session.ok ? session.duLieu.permissions : [];
  const staffCode = session !== null && session.ok ? (session.duLieu.staff?.code ?? "") : "";
  const gates = congThaoTac(
    coQuyen(permissions, QUYEN_PHAN_LOAI_PHAN_ANH),
    coQuyen(permissions, QUYEN_PHAN_CONG_PHAN_ANH),
    coQuyen(permissions, QUYEN_DONG_PHAN_ANH),
  );

  const shown = loaded !== null && loaded.code === code ? loaded.result : null;
  const petition = shown !== null && shown.ok ? shown.duLieu : null;

  /** The 200 body replaces the open petition; the log re-reads; the caller learns a write happened. */
  function replaced(p: petitions_phieuPhanAnhRa, rereadLog: boolean): void {
    setWriteError(null);
    setLoaded({ code: p.code, result: { ok: true, duLieu: p } });
    setCode(p.code);
    if (rereadLog) setLogReads((n) => n + 1);
    onChanged();
  }

  function settle(kq: KetQua<petitions_phieuPhanAnhRa>): void {
    setSending(false);
    setAssignRefusal(null);
    setClassifyRefusal(null);
    if (!kq.ok) {
      // The server's sentence VERBATIM, as on the register.
      setWriteError(kq.thongBao);
      return;
    }
    replaced(kq.duLieu, true);
  }

  function run(call: Promise<KetQua<petitions_phieuPhanAnhRa>>, success: string): Promise<boolean> {
    setSending(true);
    return call.then((kq) => {
      settle(kq);
      if (kq.ok) toast.success(success);
      return kq.ok;
    });
  }

  /** A duplicate / main / merged petition, through the detail route — as the register's `openByCode`. */
  function openByCode(next: string): void {
    layPhieuPhanAnh(next).then((kq) => {
      if (!kq.ok) {
        setWriteError(kq.thongBao);
        return;
      }
      setWriteError(null);
      setCloseRefusal(null);
      setAssignRefusal(null);
      setClassifyRefusal(null);
      setLoaded({ code: kq.duLieu.code, result: kq });
      setCode(kq.duLieu.code);
    });
  }

  if (petition === null) {
    return (
      <PendingDetail title={`Phản ánh ${code}`} onClose={onClose}>
        {shown === null ? (
          <p role="status" className="m-0 text-[13px] text-ink-muted">
            Đang tải phiếu…
          </p>
        ) : (
          !shown.ok && (
            <p role="alert" className="thong-bao-loi m-0">
              {shown.thongBao}
            </p>
          )
        )}
      </PendingDetail>
    );
  }

  return (
    <ChiTietPhieu
      // Keyed by code, as the register: another petition is a fresh drawer, no draft carried over.
      key={petition.code}
      phieu={petition}
      bayGio={new Date()}
      cong={gates}
      tenBoPhan={new Map(units.map((b) => [b.id, b.name]))}
      boPhan={units}
      danhBa={directory}
      dangGui={sending}
      loiGhi={writeError}
      coGhiNhatKy={coQuyen(permissions, QUYEN_XEM_PHAN_ANH)}
      lanLamMoiNhatKy={logReads}
      permissions={permissions}
      sessionStaffCode={staffCode}
      thon={hamlets}
      classifyRefusal={classifyRefusal}
      onMergeChanged={(p) => replaced(p, true)}
      onStaffRatingRecorded={(p) => replaced(p, false)}
      onOpenPetition={openByCode}
      dong={onClose}
      phanLoai={(field, note, extra) => {
        setSending(true);
        return phanLoaiPhieu(petition.code, field, note, extra).then((kq) => {
          if (!kq.ok) {
            setSending(false);
            setWriteError(null);
            setClassifyRefusal(kq.thongBao);
            return false;
          }
          settle(kq);
          toast.success(movedToast("dang-phan-loai"));
          return true;
        });
      }}
      chuyenXuLy={(unitId, staff, note, attachments) => {
        setSending(true);
        const fromScreening = petition.status === "dang-phan-loai";
        return chuyenXuLyPhieu(petition.code, unitId, staff, note, attachments).then((kq) => {
          if (!kq.ok) {
            setSending(false);
            setWriteError(null);
            setAssignRefusal(kq.thongBao);
            return false;
          }
          settle(kq);
          toast.success(fromScreening ? movedToast("da-chuyen-xu-ly") : ASSIGNED_TOAST);
          return true;
        });
      }}
      assignRefusal={assignRefusal}
      tienTrangThai={(note, attachments) => {
        setSending(true);
        return tienTrangThaiPhieu(petition.code, note, attachments).then((kq) => {
          settle(kq);
          if (kq.ok) toast.success(movedToast(kq.duLieu.status));
          return kq.ok;
        });
      }}
      closeRefusal={closeRefusal}
      dongPhieuLai={(result, note, attachments) => {
        setSending(true);
        return dongPhieu(petition.code, result, note, attachments).then((kq) => {
          if (!kq.ok && kq.afterPhotoRequired) {
            setSending(false);
            setWriteError(null);
            setCloseRefusal(kq.thongBao);
            return false;
          }
          setCloseRefusal(null);
          settle(kq);
          if (kq.ok) toast.success(movedToast("da-dong"));
          return kq.ok;
        });
      }}
      khongTiepNhan={(reason, note, attachments) =>
        run(khongTiepNhanPhieu(petition.code, reason, note, attachments), movedToast("khong-tiep-nhan"))
      }
      chuyenCapTren={(reason, body, note, attachments) =>
        run(chuyenCapTrenPhieu(petition.code, reason, body, note, attachments), movedToast("chuyen-cap-tren"))
      }
      setPublication={(target) =>
        run(setPetitionPublication(petition.code, target), target === "cong-khai" ? PUBLISHED_TOAST : HIDDEN_TOAST)
      }
    />
  );
}

/* ── Dự án (Giải ngân) ───────────────────────────────────────────────────────────────────── */

export const PROJECT_DETAIL_TITLE_ID = "tieu-de-chi-tiet-du-an-tong-quan";

/**
 * The project's own detail (`ChiTietDuAn` — its reads, tabs and every write gate), in the right-hand
 * panel the other two registers use, with no link back to `/giai-ngan`.
 */
function ProjectDetail({ id, onClose }: { id: string; onClose: () => void }) {
  return (
    <LargeDialog titleId={PROJECT_DETAIL_TITLE_ID} onDismiss={onClose}>
      <DetailHeader titleId={PROJECT_DETAIL_TITLE_ID} title="Chi tiết dự án" onClose={onClose} />
      <div className="min-h-0 flex-1 overflow-y-auto p-5">
        <ChiTietDuAn id={id} backLink={false} />
      </div>
    </LargeDialog>
  );
}

/* ── shared frame ────────────────────────────────────────────────────────────────────────── */

const PENDING_TITLE_ID = "tieu-de-chi-tiet-ban-ghi-tong-quan";

/** The panel while the record is read, or its refusal verbatim (404 for another commune's code). */
function PendingDetail({ title, onClose, children }: { title: string; onClose: () => void; children: ReactNode }) {
  return (
    <LargeDialog titleId={PENDING_TITLE_ID} onDismiss={onClose} className="bg-white">
      <DetailHeader titleId={PENDING_TITLE_ID} title={title} onClose={onClose} />
      <div className="p-5">{children}</div>
    </LargeDialog>
  );
}

/** The drawers' header row: the name, then the outline ✕ (`ngan-van-ban-den.tsx`). */
function DetailHeader({ titleId, title, onClose }: { titleId: string; title: string; onClose: () => void }) {
  return (
    <header className="flex shrink-0 items-start gap-3 border-0 border-b border-solid border-line bg-white px-5 py-4">
      <h2
        id={titleId}
        tabIndex={-1}
        className="m-0 min-w-0 flex-1 text-[15px] leading-snug font-bold text-navy outline-none"
      >
        {title}
      </h2>
      <IconButton type="button" variant="secondary" size="md" label="Đóng" onClick={onClose}>
        <X aria-hidden="true" />
      </IconButton>
    </header>
  );
}
