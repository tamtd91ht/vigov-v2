"use client";

import { useCallback, useEffect, useMemo, useState, type FormEvent, type ReactNode } from "react";

import { ChevronLeft, ChevronRight, CloudOff, Plus, RotateCw, Search, Smartphone, Trash2, Upload } from "lucide-react";
import { toast } from "sonner";

import type { MucChon } from "@/components/danh-ba/bieu-mau-ghi-can-bo";
import { BAN_TRONG, banTuCanBo, khoaChongTrungMoi, thanSua, thanThem } from "@/components/danh-ba/nhan-ghi-danh-ba";
import type { BanNhapCanBo } from "@/components/danh-ba/nhan-ghi-danh-ba";
import { Button } from "@/components/ui/button";
import { EmptyState } from "@/components/ui/empty-state";
import { Field } from "@/components/ui/field";
import { PendingButton } from "@/components/ui/pending-feature";
import { Skeleton } from "@/components/ui/skeleton";
import { ConfigDialog } from "@/features/cau-hinh/config-dialog";
import {
  coTrangTruoc,
  sangTrangSau,
  veTrangTruoc,
  type NganXepConTro,
} from "@/features/cau-hinh/ngan-xep-con-tro";
import { bangTraTuKetQua, type BangTraDanhMuc } from "@/features/cau-hinh/tra-danh-muc";
import { usePhien } from "@/features/phien/phien-hien-tai";
import {
  countStaffMatches,
  datCongKhaiCanBo,
  docTrangDanhBa,
  getStaffTallies,
  publishStaffBulk,
  suaCanBo,
  themCanBo,
  xoaCanBo,
  type StaffTallies,
} from "@/lib/api/can-bo";
import { layDanhMucBoPhan } from "@/lib/api/danh-muc";
import type { KetQua } from "@/lib/api/goi";
import type {
  identity_canBoTomTat,
  identity_danhSachBoPhanRa,
  page_Result_identity_canBoTomTat,
} from "@/lib/api/schema.gen";

import { BangLienHe } from "./bang-lien-he";
import { BulkConsentForm, type BulkOutcome } from "./bulk-consent-form";
import {
  BULK_ADD_BUTTON,
  BULK_DELETE_BUTTON,
  BULK_NOTHING_TO_WITHDRAW,
  BULK_REPLAYED,
  BULK_TITLE,
  BULK_WITHDRAW_BUTTON,
  bulkAddedToast,
  bulkRequest,
  bulkResultLines,
  bulkWithdrawFailedText,
  bulkWithdrawnToast,
  consentSelection,
  publishedCount,
  selectedCountText,
  setConsent,
  togglePage,
  toggleRow,
  type BulkSelection,
} from "./bulk-publication";
import {
  banCongKhaiTu,
  duocCongKhaiTheoPhien,
  tieuDeCongKhai,
  tieuDeRut,
  yeuCauCongKhai,
  yeuCauRut,
  type BanCongKhai,
} from "./cong-khai";
import { DirectoryHeading } from "./directory-heading";
import { HopCongKhai, type DangMoCongKhai } from "./hop-cong-khai";
import { HopXoa } from "./hop-xoa";
import { daXoa, duocXoaTheoPhien, tieuDeXoa, yeuCauXoa } from "./xoa-dong";
import {
  GOI_Y_O_TIM,
  LUA_CHON_HIEN_THI,
  NHAN_LOC_HIEN_THI,
  NHAN_LOC_KHOI,
  NHAN_O_TIM,
  TAT_CA_KHOI,
  THU_TU_HIEN_THI,
  TRUY_VAN_DAU,
  apLoc,
  dangLoc,
  ketQuaGuiTim,
  maBoPhanLoc,
  maHienThi,
  thamSoDoc,
  type LocDanhBa,
  type TruyVanDanhBa,
} from "./loc-danh-ba";
import {
  ADD_BUTTON,
  DANH_BA_RONG,
  IMPORT_BUTTON,
  KHONG_KHOP_LOC,
  KPI_PUBLISHED,
  KPI_TOTAL,
  LOAD_FAILED_TITLE,
  NHAN_SO_KHOI,
  PAGE_NEXT,
  PAGE_PREVIOUS,
  RELOAD,
  demSoKhoi,
  departmentOptionText,
  filteredTotal,
  pendingPart,
  shownCountText,
  tallyFigures,
} from "./nhan-danh-ba";
import { StaffContactForm } from "./staff-contact-form";
import { StaffDirectoryImportDialog } from "./staff-directory-import-dialog";
import {
  ADDED_TOAST,
  CONTACT_ADD_TITLE,
  CONTACT_DESCRIPTION,
  CONTACT_EDIT_TITLE,
  SAVED_TOAST,
  firstContactError,
  publicationStep,
  validateContact,
  zaloAfterCreate,
  zaloNotSavedText,
  type ContactErrors,
} from "./staff-contact";

/**
 * Tab **Danh bạ cán bộ** — bản mẫu `StaffDirectoryWorkspace.tsx` (chủ đầu tư chốt 09/10/2026: thêm,
 * sửa và nhập Excel mở HỘP THOẠI ngay tại tab; bảng có cột chọn và thanh chọn).
 *
 * ─────────────────────────────────────────────────────────────────────────────────────────
 * CÁC TUYẾN GHI MÀN NÀY GỌI, mỗi tuyến một khoá ở máy chủ:
 *
 *   POST  /api/v1/staff                     thêm (`admin.user`)
 *   PATCH /api/v1/staff/{id}                sửa hồ sơ, cả "Gọi được qua Zalo" (`admin.user`)
 *   /api/v1/staff/import-*                  nhập Excel — cùng luồng với `/nguoi-dung` (`admin.user`)
 *   PUT   /api/v1/staff/{id}/publication    công khai / rút MỘT người (`content.update`, #12)
 *   POST  /api/v1/staff/publications        công khai NHIỀU người, xác nhận từng dòng (`content.update`)
 *   DELETE /api/v1/staff/{id}               xoá mềm một dòng nhập trùng, kèm lý do (`admin.user.delete`)
 *
 * CÔNG KHAI LUÔN QUA HỘP HỎI Ý (#12, Nghị định 13) — từ nút trên dòng, từ thanh chọn, và từ ô "Hiện
 * trên danh bạ Mini App" của hộp sửa (ô ấy chỉ mở hộp hỏi ý SAU khi lưu, `publicationStep`).
 * ─────────────────────────────────────────────────────────────────────────────────────────
 *
 * CỔNG CỦA CẢ MÀN nằm ở `staff-directory-tab.tsx` (`admin.user`). Nút Mini App và cột chọn cần THÊM
 * `content.update`, nút 🗑 cần THÊM `admin.user.delete`. Mọi lớp ẩn chỉ là tiện dụng: lớp chặn THẬT
 * ở máy chủ, trên TỪNG yêu cầu (luật 5, cấm #1).
 *
 * CHỈ DÙNG BỐN HOOK `useState` / `useEffect` / `useCallback` / `useMemo`: `danh-ba-lien-he.luong.test.tsx`
 * gọi màn như một hàm dưới một React giả chỉ biết bốn hook ấy. Mọi phần cần hook khác (hộp thoại,
 * dấu "?", bảng nhập Excel) là PHẦN TỬ CON, không bao giờ được gọi ở đó.
 */

/** Trạng thái một lần đọc danh sách. Ba nhánh rời nhau. */
type TrangThaiTrang =
  | { pha: "dangTai" }
  | { pha: "loi"; thongBao: string }
  | { pha: "xong"; trang: page_Result_identity_canBoTomTat };

/** The add / edit dialog: who (`null` = adding) and, when adding, the Idempotency-Key of THIS opening. */
type ContactDialog = { readonly editing: identity_canBoTomTat | null; readonly key: string };

const SELECT_CLASS =
  "border-line focus-visible:ring-ring/50 h-9 rounded-md border border-solid bg-white px-3 text-[12.5px] outline-none focus-visible:ring-[3px]";

export function DanhBaLienHe() {
  /** Bộ lọc đang áp + ngăn xếp con trỏ — MỘT state, để đổi lọc không thể quên về trang đầu. */
  const [truyVan, datTruyVan] = useState<TruyVanDanhBa>(TRUY_VAN_DAU);
  const [trangThai, datTrangThai] = useState<TrangThaiTrang>({ pha: "dangTai" });
  /** `null` là chưa đọc xong danh mục bộ phận — KHÔNG phải "xã không có bộ phận nào". */
  const [boPhan, datBoPhan] = useState<KetQua<identity_danhSachBoPhanRa> | null>(null);
  /** `GET /api/v1/staff-counts`: the KPI cards, the filter's "(đang hiện/tổng)", the line under the table. */
  const [tallies, setTallies] = useState<KetQua<StaffTallies> | null>(null);
  /** How many rows the APPLIED search matches (`POST /api/v1/staff-count-queries`); `null` = not read. */
  const [searchCount, setSearchCount] = useState<KetQua<number> | null>(null);
  /** Tăng sau mỗi lần ghi thành công — buộc đọc lại trang đang xem VÀ các con số. */
  const [lanDoc, datLanDoc] = useState(0);

  const [loiMayChu, datLoiMayChu] = useState("");
  const [dangGui, datDangGui] = useState(false);

  /** People ticked in the table (row snapshots, in ticking order). Survives a page or filter change. */
  const [selected, setSelected] = useState<readonly identity_canBoTomTat[]>([]);

  const [contact, setContact] = useState<ContactDialog | null>(null);
  const [draft, setDraft] = useState<BanNhapCanBo>(BAN_TRONG);
  const [showOnMiniApp, setShowOnMiniApp] = useState(false);
  const [contactErrors, setContactErrors] = useState<ContactErrors>({});

  const [importOpen, setImportOpen] = useState(false);

  /** Hộp công khai / rút MỘT người đang mở. */
  const [dangMoCK, datDangMoCK] = useState<DangMoCongKhai | null>(null);
  const [banCK, datBanCK] = useState<BanCongKhai>({ daHoiY: false, thuTu: "" });

  /** Hộp xoá dòng nhập trùng đang mở, và lý do đang gõ. */
  const [dangXoa, datDangXoa] = useState<identity_canBoTomTat | null>(null);
  const [lyDoXoa, datLyDoXoa] = useState("");

  /**
   * The bulk consent dialog. THE IDEMPOTENCY KEY BELONGS TO ONE BODY: re-minted whenever a tick
   * changes and after every completed send; a retry after a network failure keeps it, because that
   * first send may already have published people.
   */
  const [bulkOpen, setBulkOpen] = useState(false);
  const [bulkSelection, setBulkSelection] = useState<BulkSelection>([]);
  const [bulkKey, setBulkKey] = useState(() => crypto.randomUUID());
  const [bulkOutcome, setBulkOutcome] = useState<BulkOutcome | null>(null);
  const [bulkError, setBulkError] = useState("");

  /** Khoá của phiên. CHƯA ĐỌC XONG hay đọc hỏng → không (fail closed, `quyetDinhTheoKhoa`). */
  const phien = usePhien();
  const duocCongKhai = duocCongKhaiTheoPhien(phien);
  const duocXoa = duocXoaTheoPhien(phien);

  /**
   * MỘT DANH MỤC, ĐỌC ĐÚNG MỘT LƯỢT KHI MỞ MÀN HÌNH — không theo trang, không theo dòng
   * (`skills/load-data-once`). Đọc ở trình duyệt: đường dẫn tương đối trên chính host của xã, trình
   * duyệt tự gửi cookie host-only (`lib/api/goi.ts`).
   */
  useEffect(() => {
    let bo = false;
    layDanhMucBoPhan().then((kq) => {
      if (!bo) datBoPhan(kq);
    });
    return () => {
      bo = true;
    };
  }, []);

  const traBoPhan = useMemo<BangTraDanhMuc>(() => bangTraTuKetQua(boPhan), [boPhan]);
  const soKhoi = useMemo(() => demSoKhoi(boPhan), [boPhan]);
  const mucBoPhan = useMemo<readonly MucChon[]>(
    () => (boPhan !== null && boPhan.ok ? boPhan.duLieu.items : []),
    [boPhan],
  );

  useEffect(() => {
    // `bo` chặn một phản hồi đến muộn của lần đọc trước ghi đè lên lần đọc sau.
    let bo = false;
    // KHÔNG TRUYỀN `limit`, `sort`, `order`: máy chủ áp mặc định của chính nó. Có chữ tìm thì
    // `docTrangDanhBa` đi đường POST, chữ và con trỏ nằm trong THÂN — không bao giờ trên URL.
    docTrangDanhBa(truyVan.loc.tuKhoa, thamSoDoc(truyVan)).then((ketQua) => {
      if (bo) return;
      datTrangThai(
        ketQua.ok ? { pha: "xong", trang: ketQua.duLieu } : { pha: "loi", thongBao: ketQua.thongBao },
      );
      // A ticked person on this page is refreshed to the row just read, so the bar acts on what the
      // server says now (published or not), not on what it said when the box was ticked.
      if (ketQua.ok) {
        const fresh = new Map(ketQua.duLieu.items.map((cb) => [cb.id, cb]));
        setSelected((cu) => cu.map((s) => fresh.get(s.id) ?? s));
      }
    });
    return () => {
      bo = true;
    };
  }, [truyVan, lanDoc]);

  /** The commune's tallies: on opening, and again after every write. */
  useEffect(() => {
    let bo = false;
    getStaffTallies().then((kq) => {
      if (!bo) setTallies(kq);
    });
    return () => {
      bo = true;
    };
  }, [lanDoc]);

  /**
   * The number of matches of the APPLIED search. POST, the words in the body (rule 3, forbidden #4) —
   * the same `q`/`unit`/`published` as the page read, so both answers describe one set of rows.
   */
  const loc = truyVan.loc;
  useEffect(() => {
    if (loc.tuKhoa === null) return;
    let bo = false;
    countStaffMatches(loc.tuKhoa, { boPhan: loc.boPhan, congKhai: LUA_CHON_HIEN_THI[loc.hienThi].congKhai }).then(
      (kq) => {
        if (!bo) setSearchCount(kq);
      },
    );
    return () => {
      bo = true;
    };
  }, [loc, lanDoc]);

  /** "Hiển thị n cán bộ." — the total for the filter in force, never the length of the open page. */
  const shownTotal = useMemo<number | null>(() => {
    if (loc.tuKhoa !== null) return searchCount !== null && searchCount.ok ? searchCount.duLieu : null;
    return tallies !== null && tallies.ok
      ? filteredTotal(tallies.duLieu, loc.boPhan, LUA_CHON_HIEN_THI[loc.hienThi].congKhai)
      : null;
  }, [loc, searchCount, tallies]);

  /** Close every dialog, say nothing — the shared first step of opening another one. */
  const closeAll = useCallback(() => {
    setContact(null);
    setContactErrors({});
    datDangMoCK(null);
    datDangXoa(null);
    datLyDoXoa("");
    setBulkOpen(false);
    setBulkOutcome(null);
    setBulkError("");
    setImportOpen(false);
    datLoiMayChu("");
  }, []);

  /**
   * Đổi truy vấn — chuyển trang hoặc đổi bộ lọc. `dangTai` đặt Ở ĐÂY, trong sự kiện, chứ không trong
   * thân effect (lint của React chặn setState đồng bộ trong effect).
   */
  const doiTruyVan = useCallback(
    (tinh: (cu: TruyVanDanhBa) => TruyVanDanhBa) => {
      datTrangThai({ pha: "dangTai" });
      closeAll();
      datTruyVan(tinh);
    },
    [closeAll],
  );

  const diToiTrang = useCallback(
    (toi: NganXepConTro) => doiTruyVan((cu) => ({ ...cu, nganXep: toi })),
    [doiTruyVan],
  );

  /** Đổi bộ lọc — `apLoc` luôn đưa ngăn xếp về trang đầu; con số của lần tìm cũ không còn đúng. */
  const doiLoc = useCallback(
    (doi: Partial<LocDanhBa>) => {
      setSearchCount(null);
      doiTruyVan((cu) => apLoc(cu, doi));
    },
    [doiTruyVan],
  );

  /** After a completed write: close, say what was done, and READ AGAIN the page and the figures. */
  const ghiXong = useCallback(
    (cau: string) => {
      closeAll();
      toast.success(cau);
      datLanDoc((n) => n + 1);
    },
    [closeAll],
  );

  /* ---- add / edit ---------------------------------------------------------------------------- */

  const openAdd = useCallback(() => {
    closeAll();
    // The key is minted when the dialog OPENS: a retry after a network failure reuses it, because the
    // first send may already have created the person (`themCanBo`).
    setContact({ editing: null, key: khoaChongTrungMoi() });
    setDraft(BAN_TRONG);
    setShowOnMiniApp(false);
  }, [closeAll]);

  const moSua = useCallback(
    (cb: identity_canBoTomTat) => {
      closeAll();
      setContact({ editing: cb, key: "" });
      setDraft(banTuCanBo(cb));
      setShowOnMiniApp(cb.published);
    },
    [closeAll],
  );

  const closeContact = useCallback(() => {
    setContact(null);
    setContactErrors({});
    datLoiMayChu("");
  }, []);

  const sendContact = useCallback(() => {
    if (contact === null || dangGui) return;
    const errors = validateContact(draft);
    setContactErrors(errors);
    const first = firstContactError(errors);
    if (first !== null) {
      toast.error(first);
      return;
    }

    datLoiMayChu("");
    datDangGui(true);
    const editing = contact.editing;
    const wanted = showOnMiniApp;
    const save: Promise<KetQua<identity_canBoTomTat>> =
      editing === null
        ? themCanBo(thanThem(draft), contact.key).then(async (created) => {
            if (!created.ok) return created;
            const zalo = zaloAfterCreate(draft);
            if (zalo === null) return created;
            const patched = await suaCanBo(created.duLieu.id, zalo);
            if (patched.ok) return patched;
            toast.error(zaloNotSavedText(patched.thongBao));
            return created;
          })
        : suaCanBo(editing.id, thanSua(draft, editing));

    void save
      .then((kq) => {
        if (!kq.ok) {
          // The server's sentence verbatim, in the dialog, beside the button it refused.
          datLoiMayChu(kq.thongBao);
          return;
        }
        ghiXong(editing === null ? ADDED_TOAST : SAVED_TOAST);
        // The Mini App box never publishes: it opens the consent (or withdrawal) box for this person.
        const step = publicationStep(wanted, kq.duLieu, duocCongKhai);
        if (step !== null) {
          datDangMoCK(step);
          datBanCK(banCongKhaiTu(step.canBo));
        }
      })
      .finally(() => datDangGui(false));
  }, [contact, dangGui, draft, duocCongKhai, ghiXong, showOnMiniApp]);

  /* ---- publish / withdraw ONE person --------------------------------------------------------- */

  /** Mở hộp công khai hoặc rút cho MỘT người. Ô tick luôn bắt đầu TRỐNG (`banCongKhaiTu`). */
  const moCongKhai = useCallback(
    (dm: DangMoCongKhai) => {
      closeAll();
      datDangMoCK(dm);
      datBanCK(banCongKhaiTu(dm.canBo));
    },
    [closeAll],
  );

  const dongCongKhai = useCallback(() => {
    datDangMoCK(null);
    datLoiMayChu("");
  }, []);

  const guiCongKhai = useCallback(() => {
    if (dangMoCK === null || dangGui) return;
    // Công khai: chưa tick hay thứ tự sai thì dừng TẠI ĐÂY, không gọi mạng. Rút: luôn gửi lại thứ tự
    // đang có, vì PUT thiếu `display_order` là xoá nó (`yeuCauRut`).
    let yc;
    if (dangMoCK.kieu === "congKhai") {
      const kq = yeuCauCongKhai(banCK);
      if ("loi" in kq) {
        datLoiMayChu(kq.loi);
        return;
      }
      yc = kq.yeuCau;
    } else {
      yc = yeuCauRut(dangMoCK.canBo);
    }

    datLoiMayChu("");
    datDangGui(true);
    const congKhai = dangMoCK.kieu === "congKhai";
    void datCongKhaiCanBo(dangMoCK.canBo.id, yc)
      .then((kq) => {
        // The prototype's one sentence for any count (`StaffDirectoryWorkspace.tsx:113-117`), n = 1.
        if (kq.ok) ghiXong(congKhai ? bulkAddedToast(1) : bulkWithdrawnToast(1));
        else datLoiMayChu(kq.thongBao);
      })
      .finally(() => datDangGui(false));
  }, [banCK, dangGui, dangMoCK, ghiXong]);

  /* ---- delete ONE row ------------------------------------------------------------------------ */

  /** Mở hộp xoá cho MỘT dòng. Lý do luôn bắt đầu trống — mỗi lần xoá một lý do của riêng nó. */
  const moXoa = useCallback(
    (cb: identity_canBoTomTat) => {
      closeAll();
      datDangXoa(cb);
    },
    [closeAll],
  );

  const dongXoa = useCallback(() => {
    datDangXoa(null);
    datLyDoXoa("");
    datLoiMayChu("");
  }, []);

  /**
   * Gửi xoá. Dòng có tài khoản, lý do rỗng hay quá dài dừng TẠI ĐÂY (`yeuCauXoa`). Sau 204 đọc lại
   * với ĐÚNG chữ tìm, bộ lọc và con trỏ đang áp (`lanDoc`, không đụng `truyVan`).
   */
  const guiXoa = useCallback(() => {
    if (dangXoa === null || dangGui) return;
    const kq = yeuCauXoa(dangXoa, lyDoXoa);
    if ("loi" in kq) {
      datLoiMayChu(kq.loi);
      return;
    }
    datLoiMayChu("");
    datDangGui(true);
    const person = dangXoa;
    void xoaCanBo(person.id, kq.lyDo)
      .then((ketQua) => {
        if (!ketQua.ok) {
          datLoiMayChu(ketQua.thongBao);
          return;
        }
        setSelected((cu) => cu.filter((s) => s.id !== person.id));
        ghiXong(daXoa(person.full_name));
      })
      .finally(() => datDangGui(false));
  }, [dangGui, dangXoa, ghiXong, lyDoXoa]);

  /* ---- the selection bar --------------------------------------------------------------------- */

  /** "Thêm vào danh bạ Mini App": opens the consent box for the selected people. Never publishes. */
  const openBulk = useCallback(() => {
    closeAll();
    setBulkSelection(consentSelection(selected));
    setBulkKey(crypto.randomUUID());
    setBulkOpen(true);
  }, [closeAll, selected]);

  const closeBulk = useCallback(() => {
    setBulkOpen(false);
    setBulkOutcome(null);
    setBulkError("");
  }, []);

  /** Changing WHAT would be sent: new body, new key. */
  const changeConsent = useCallback((id: string, consentAsked: boolean) => {
    setBulkSelection((cu) => setConsent(cu, id, consentAsked));
    setBulkKey(crypto.randomUUID());
    setBulkError("");
  }, []);

  /**
   * Send the bulk request. Refusals that need no server (nobody ticked, over the cap) stop HERE. On a
   * 200 the table selection is emptied and the register re-read; the dialog stays open only to name
   * the people the server skipped.
   */
  const sendBulk = useCallback(() => {
    if (dangGui) return;
    const req = bulkRequest(bulkSelection);
    if ("error" in req) {
      setBulkError(req.error);
      return;
    }
    setBulkError("");
    datDangGui(true);
    const sent = bulkSelection;
    void publishStaffBulk(req.rows, bulkKey)
      .then((kq) => {
        if (!kq.ok) {
          // Same key kept: a retry of this very body must be recognised as one.
          setBulkError(kq.thongBao);
          return;
        }
        setSelected([]);
        setBulkSelection([]);
        setBulkKey(crypto.randomUUID());
        datLanDoc((n) => n + 1);
        if (kq.duLieu.kind === "replayed") {
          toast.success(BULK_REPLAYED);
          setBulkOpen(false);
          return;
        }
        const results = kq.duLieu.items;
        toast.success(bulkAddedToast(publishedCount(results)));
        if (publishedCount(results) === results.length) setBulkOpen(false);
        else setBulkOutcome({ kind: "lines", lines: bulkResultLines(results, sent) });
      })
      .finally(() => datDangGui(false));
  }, [bulkKey, bulkSelection, dangGui]);

  /**
   * "Rút khỏi danh bạ": there is no bulk route, so the single-person `PUT …/publication` is sent once
   * per selected person ON the Mini App, each with the order it has (`yeuCauRut`). The toast counts
   * what the server changed; refusals are counted and the first server sentence is shown verbatim.
   */
  const withdrawSelected = useCallback(() => {
    if (dangGui) return;
    const targets = selected.filter((cb) => cb.published);
    if (targets.length === 0) {
      toast.info(BULK_NOTHING_TO_WITHDRAW);
      return;
    }
    datDangGui(true);
    void (async () => {
      let done = 0;
      const refused: string[] = [];
      for (const cb of targets) {
        const kq = await datCongKhaiCanBo(cb.id, yeuCauRut(cb));
        if (kq.ok) done += 1;
        else refused.push(kq.thongBao);
      }
      if (done > 0) toast.success(bulkWithdrawnToast(done));
      if (refused.length > 0) toast.error(bulkWithdrawFailedText(refused.length, refused[0] ?? ""));
      setSelected([]);
      datLanDoc((n) => n + 1);
    })().finally(() => datDangGui(false));
  }, [dangGui, selected]);

  const selection = useMemo(
    () =>
      duocCongKhai
        ? {
            selectedIds: new Set(selected.map((s) => s.id)),
            onToggle: (cb: identity_canBoTomTat, on: boolean) => setSelected((cu) => toggleRow(cu, cb, on)),
            onTogglePage: (on: boolean) =>
              setSelected((cu) => togglePage(cu, trangThai.pha === "xong" ? trangThai.trang.items : [], on)),
          }
        : undefined,
    [duocCongKhai, selected, trangThai],
  );

  const hanhDongCongKhai = useMemo(
    () =>
      duocCongKhai
        ? {
            onThem: (cb: identity_canBoTomTat) => moCongKhai({ kieu: "congKhai", canBo: cb }),
            onRut: (cb: identity_canBoTomTat) => moCongKhai({ kieu: "rut", canBo: cb }),
          }
        : undefined,
    [duocCongKhai, moCongKhai],
  );

  /** "Tải lại" after a failed read — the SAME mechanism a completed write uses (`lanDoc`). */
  const docLai = useCallback(() => {
    datTrangThai({ pha: "dangTai" });
    datLanDoc((n) => n + 1);
  }, []);

  const openImport = useCallback(() => {
    closeAll();
    setImportOpen(true);
  }, [closeAll]);

  const figures = tallyFigures(tallies);
  const rows = trangThai.pha === "xong" ? trangThai.trang.items : [];
  // An empty first page IS a count: nobody matches the filter in force.
  const shownLine =
    trangThai.pha === "xong" && rows.length === 0 && !coTrangTruoc(truyVan.nganXep) ? 0 : shownTotal;

  return (
    <section className="min-w-0" aria-labelledby="tieu-de-danh-ba-lien-he">
      <h2 id="tieu-de-danh-ba-lien-he" className="an-thi-giac">
        Danh sách cán bộ
      </h2>

      {/* Both actions need only the tab's own key (`admin.user`): the create route and the three import
          routes declare it (`service-identity/internal/http/routes.go`). */}
      <DirectoryHeading
        actions={
          <>
            <Button
              type="button"
              variant="outline"
              icon={<Upload aria-hidden="true" className="size-4" />}
              onClick={openImport}
            >
              {IMPORT_BUTTON}
            </Button>
            <Button type="button" variant="primary" icon={<Plus aria-hidden="true" className="size-4" />} onClick={openAdd}>
              {ADD_BUTTON}
            </Button>
          </>
        }
      />

      {/* The prototype's three cards, one row from `sm`, from ONE read of `GET /api/v1/staff-counts`. */}
      <div className="mb-5 grid gap-3 sm:grid-cols-3">
        <SummaryCard label={KPI_TOTAL} value={figures.total} />
        <SummaryCard label={KPI_PUBLISHED} value={figures.published} leaf />
        <SummaryCard label={NHAN_SO_KHOI} value={figures.departments} />
      </div>

      {/* Danh mục bộ phận hỏng thì NÓI RA MỘT LẦN Ở ĐÂY, đúng câu của máy chủ. */}
      {soKhoi.pha === "loi" && (
        <p className="text-danger m-0 mb-3 text-[12px] font-medium" role="alert">
          Danh mục khối / đơn vị: {soKhoi.thongBao}
        </p>
      )}

      <HangLoc
        loc={loc}
        boPhan={mucBoPhan}
        tallies={tallies !== null && tallies.ok ? tallies.duLieu : null}
        doiLoc={doiLoc}
        end={
          selection !== undefined && selected.length > 0 ? (
            <SelectionBar
              count={selected.length}
              busy={dangGui}
              onAdd={openBulk}
              onWithdraw={withdrawSelected}
              showDelete={duocXoa}
            />
          ) : undefined
        }
      />

      {trangThai.pha === "dangTai" ? (
        <div role="status">
          <span className="an-thi-giac">Đang tải danh bạ…</span>
          <Skeleton className="h-96 w-full" />
        </div>
      ) : (
        <div className="border-line overflow-hidden rounded-[10px] border border-solid bg-white">
          {trangThai.pha === "loi" ? (
            // The server's `message` verbatim; no branching on `code`, no `trace_id` (`lib/api/goi.ts`).
            <div role="alert">
              <EmptyState
                icon={CloudOff}
                title={LOAD_FAILED_TITLE}
                description={trangThai.thongBao}
                action={
                  <Button type="button" variant="secondary" icon={<RotateCw aria-hidden="true" />} onClick={docLai}>
                    {RELOAD}
                  </Button>
                }
              />
            </div>
          ) : rows.length === 0 ? (
            // Two sentences, never one: "no staff yet" under a mistyped search is a false statement
            // about the whole authority.
            <p className="text-ink-muted m-0 py-12 text-center">{dangLoc(loc) ? KHONG_KHOP_LOC : DANH_BA_RONG}</p>
          ) : (
            <div role="region" tabIndex={0} aria-label="Danh bạ cán bộ của đơn vị" className="overflow-x-auto">
              <BangLienHe
                danhSach={rows}
                traBoPhan={traBoPhan}
                onSua={moSua}
                congKhai={hanhDongCongKhai}
                onXoa={duocXoa ? moXoa : undefined}
                selection={selection}
              />
            </div>
          )}
        </div>
      )}

      {trangThai.pha === "xong" && (
        <div className="mt-3 flex flex-wrap items-center justify-between gap-3">
          {/* The total for the filter in force (tallies, or the search's own count) — never the page
              length. Empty while that figure is unknown: a page length here would be a false total.
              An empty FIRST page is the one exception: the filter matches nobody, so 0 is the total
              (the prototype says "Hiển thị 0 cán bộ.", `StaffDirectoryWorkspace.tsx:404-406`). */}
          <p className="text-ink-muted m-0 text-[12px]">{shownLine === null ? "" : shownCountText(shownLine)}</p>
          {(rows.length > 0 || coTrangTruoc(truyVan.nganXep)) && (
            <DieuHuongTrang
              nganXep={truyVan.nganXep}
              conTroTiep={trangThai.trang.next_cursor}
              conTrangSau={trangThai.trang.has_more}
              diToiTrang={diToiTrang}
            />
          )}
        </div>
      )}

      {contact !== null && (
        <ConfigDialog
          title={contact.editing === null ? CONTACT_ADD_TITLE : CONTACT_EDIT_TITLE}
          description={CONTACT_DESCRIPTION}
          onDismiss={() => {
            if (!dangGui) closeContact();
          }}
        >
          <StaffContactForm
            editing={contact.editing}
            draft={draft}
            setDraft={setDraft}
            showOnMiniApp={showOnMiniApp}
            setShowOnMiniApp={setShowOnMiniApp}
            canPublish={duocCongKhai}
            units={mucBoPhan}
            errors={contactErrors}
            serverError={loiMayChu}
            sending={dangGui}
            onSubmit={sendContact}
            onCancel={closeContact}
          />
        </ConfigDialog>
      )}

      {dangMoCK !== null && (
        <ConfigDialog
          title={dangMoCK.kieu === "congKhai" ? tieuDeCongKhai(dangMoCK.canBo.full_name) : tieuDeRut(dangMoCK.canBo.full_name)}
          hideHeader
          onDismiss={() => {
            if (!dangGui) dongCongKhai();
          }}
        >
          <HopCongKhai
            dangMo={dangMoCK}
            ban={banCK}
            datBan={datBanCK}
            loiMayChu={loiMayChu}
            dangGui={dangGui}
            onGui={guiCongKhai}
            onHuy={dongCongKhai}
          />
        </ConfigDialog>
      )}

      {dangXoa !== null && (
        <ConfigDialog
          title={tieuDeXoa(dangXoa.full_name)}
          hideHeader
          onDismiss={() => {
            if (!dangGui) dongXoa();
          }}
        >
          <HopXoa
            canBo={dangXoa}
            lyDo={lyDoXoa}
            datLyDo={datLyDoXoa}
            loiMayChu={loiMayChu}
            dangGui={dangGui}
            onGui={guiXoa}
            onHuy={dongXoa}
          />
        </ConfigDialog>
      )}

      {/* Gated by `content.update` like the row buttons — convenience only: the server checks the key. */}
      {duocCongKhai && bulkOpen && (
        <ConfigDialog
          title={BULK_TITLE}
          onDismiss={() => {
            if (!dangGui) closeBulk();
          }}
        >
          <BulkConsentForm
            selection={bulkSelection}
            onSetConsent={changeConsent}
            error={bulkError}
            sending={dangGui}
            outcome={bulkOutcome}
            onSubmit={sendBulk}
            onClose={closeBulk}
          />
        </ConfigDialog>
      )}

      {importOpen && (
        <StaffDirectoryImportDialog onImported={() => datLanDoc((n) => n + 1)} onClose={() => setImportOpen(false)} />
      )}
    </section>
  );
}

/** One KPI card — the prototype's markup (`StaffDirectoryWorkspace.tsx:479-500`): no icon, no hint. */
function SummaryCard({ label, value, leaf = false }: { label: string; value: string; leaf?: boolean }) {
  return (
    <div className="border-line shadow-card rounded-card border border-solid bg-white p-4">
      <p className="text-ink-muted m-0 text-[11px] font-semibold tracking-wide uppercase">{label}</p>
      <p className={leaf ? "text-leaf m-0 mt-1.5 text-[20px] font-bold" : "text-navy m-0 mt-1.5 text-[20px] font-bold"}>
        {value}
      </p>
    </div>
  );
}

/**
 * The prototype's selection bar (`StaffDirectoryWorkspace.tsx:224-255`), at the right end of the filter
 * row. "Thêm vào danh bạ Mini App" OPENS the per-person consent box; "Xoá đã chọn" is the disabled "?"
 * of ADR 0068 §14 — there is no bulk soft-delete route (`PHAN_CHUA_DUNG`). No hook of its own.
 */
export function SelectionBar({
  count,
  busy,
  onAdd,
  onWithdraw,
  showDelete,
}: {
  count: number;
  busy: boolean;
  onAdd: () => void;
  onWithdraw: () => void;
  /** Session holds `admin.user.delete` — only then is the pending delete drawn. */
  showDelete: boolean;
}) {
  return (
    <div className="ml-auto flex flex-wrap items-center gap-2">
      <span className="text-ink-muted text-[12px]">{selectedCountText(count)}</span>
      <Button
        type="button"
        variant="primary"
        size="sm"
        disabled={busy}
        icon={<Smartphone aria-hidden="true" className="size-3.5" />}
        onClick={onAdd}
      >
        {BULK_ADD_BUTTON}
      </Button>
      <Button type="button" variant="outline" size="sm" disabled={busy} onClick={onWithdraw}>
        {BULK_WITHDRAW_BUTTON}
      </Button>
      {showDelete && (
        <PendingButton
          info={pendingPart(BULK_DELETE_BUTTON)}
          variant="outline"
          size="sm"
          className="[&>button]:text-danger"
          icon={<Trash2 aria-hidden="true" className="size-3.5" />}
        />
      )}
    </div>
  );
}

/**
 * Hàng lọc — ô tìm, ô khối / đơn vị, ô trạng thái (bản mẫu `StaffDirectoryWorkspace.tsx:186-257`), và
 * ở cuối hàng là thanh chọn (`end`). Ba bộ lọc kết hợp theo AND; đổi bộ lọc nào cũng về trang đầu.
 *
 * Ô TÌM ÁP THEO LÚC GÕ, SAU MỘT NHỊP NGỪNG (`SEARCH_DELAY_MS`), như bản mẫu — không có nút Tìm. Mỗi
 * lần áp vẫn là `POST /api/v1/staff/searches`, chữ nằm trong THÂN; Enter áp ngay.
 *
 * Ô NHẬP KHÔNG CÓ THUỘC TÍNH `name`, VÀ ĐÓ LÀ LỚP CHẶN: trước khi JavaScript chạy xong, Enter trong một
 * `<form>` là trình duyệt tự gửi form — mọi ô CÓ `name` lên URL thành `?ten=chu-da-go`, vào lịch sử
 * trình duyệt (luật 3, cấm #4). `method="post"` là lớp thứ hai. `autoComplete="off"`: trên máy dùng
 * chung, trình duyệt không nhớ họ tên hay số người trước đã tìm.
 *
 * KHÔNG CÓ `maxLength`: thuộc tính ấy đếm đơn vị UTF-16, không phải ký tự. Giới hạn được kiểm lúc áp,
 * bằng đúng phép đếm máy chủ dùng (`chuanHoaTuKhoaTim`).
 */
export function HangLoc({
  loc,
  boPhan,
  tallies = null,
  doiLoc,
  end,
}: {
  loc: LocDanhBa;
  boPhan: readonly MucChon[];
  /** Per-department "(đang hiện/tổng)"; `null` = not read yet, the names alone are shown. */
  tallies?: StaffTallies | null;
  doiLoc: (doi: Partial<LocDanhBa>) => void;
  end?: ReactNode;
}) {
  const [oTim, datOTim] = useState("");
  const [loiTim, datLoiTim] = useState("");

  /**
   * Apply what is typed: a refusal (too long — the list keeps the conditions it had), or a filter
   * change. Words that normalise to the search ALREADY applied change nothing, so no second read goes
   * out for a trailing space.
   */
  const apply = useCallback(
    (typed: string) => {
      const kq = ketQuaGuiTim(typed);
      if ("loi" in kq) {
        datLoiTim(kq.loi);
        return;
      }
      datLoiTim("");
      if ((kq.doi.tuKhoa?.tu ?? null) === (loc.tuKhoa?.tu ?? null)) return;
      doiLoc(kq.doi);
    },
    [doiLoc, loc.tuKhoa],
  );

  useEffect(() => {
    const timer = setTimeout(() => apply(oTim), SEARCH_DELAY_MS);
    return () => clearTimeout(timer);
  }, [apply, oTim]);

  function gui(e: FormEvent<HTMLFormElement>) {
    e.preventDefault();
    apply(oTim);
  }

  return (
    <div className="mb-4 flex min-w-0 flex-wrap items-center gap-2.5">
      <div className="flex min-w-0 max-w-full flex-col">
        <form className="m-0 min-w-0" role="search" method="post" onSubmit={gui}>
          <Field label={NHAN_O_TIM} htmlFor="tim-danh-ba" hideLabel icon={Search} grow="auto" className="w-72 max-w-full">
            <input
              id="tim-danh-ba"
              type="search"
              value={oTim}
              onChange={(e) => datOTim(e.target.value)}
              placeholder={GOI_Y_O_TIM}
              autoComplete="off"
              aria-describedby="loi-tim-danh-ba"
              aria-invalid={loiTim !== ""}
              className="h-9! text-[12.5px]!"
            />
          </Field>
        </form>
        {/* Always in the DOM (a live region added later is not read by every screen reader), but
            zero-height while empty. */}
        <p id="loi-tim-danh-ba" className="text-danger m-0 min-h-0 text-[12px] font-medium [&:not(:empty)]:mt-1.5" role="alert">
          {loiTim}
        </p>
      </div>

      <label htmlFor="loc-khoi-danh-ba" className="an-thi-giac">
        {NHAN_LOC_KHOI}
      </label>
      <select
        id="loc-khoi-danh-ba"
        className={SELECT_CLASS}
        value={loc.boPhan}
        onChange={(e) => doiLoc({ boPhan: maBoPhanLoc(e.target.value, boPhan) })}
      >
        <option value="">{TAT_CA_KHOI}</option>
        {boPhan.map((bp) => (
          <option key={bp.id} value={bp.id}>
            {departmentOptionText(bp.name, bp.id, tallies)}
          </option>
        ))}
      </select>

      {/* The prototype's status select, emitting EXACTLY the three codes ("", "1", "0") through
          `maHienThi` → `doiLoc`. Outside the search `<form>`: a filter code, never typed text. */}
      <label htmlFor="loc-hien-thi-danh-ba" className="an-thi-giac">
        {NHAN_LOC_HIEN_THI}
      </label>
      <select
        id="loc-hien-thi-danh-ba"
        className={SELECT_CLASS}
        value={loc.hienThi}
        onChange={(e) => doiLoc({ hienThi: maHienThi(e.target.value) })}
      >
        {THU_TU_HIEN_THI.map((ma) => (
          <option key={ma} value={ma}>
            {LUA_CHON_HIEN_THI[ma].nhan}
          </option>
        ))}
      </select>

      {end}
    </div>
  );
}

/** Pause after the last keystroke before a search is applied. */
export const SEARCH_DELAY_MS = 300;

/**
 * Phân trang theo con trỏ. KHÔNG CÓ SỐ TRANG: hợp đồng trả `next_cursor` + `has_more`; tổng số của bộ
 * lọc là dòng "Hiển thị n cán bộ." bên cạnh, đọc từ tuyến đếm.
 */
function DieuHuongTrang({
  nganXep,
  conTroTiep,
  conTrangSau,
  diToiTrang,
}: {
  nganXep: NganXepConTro;
  conTroTiep: string;
  conTrangSau: boolean;
  diToiTrang: (toi: NganXepConTro) => void;
}) {
  // Hai điều kiện, không một: `has_more` nói còn trang sau, `next_cursor` là đường đi tới đó.
  const coSau = conTrangSau && conTroTiep !== "";
  return (
    <nav className="dieu-huong-trang m-0" aria-label="Phân trang danh bạ cán bộ">
      <Button
        type="button"
        variant="secondary"
        size="sm"
        icon={<ChevronLeft aria-hidden="true" />}
        disabled={!coTrangTruoc(nganXep)}
        onClick={() => diToiTrang(veTrangTruoc(nganXep))}
      >
        {PAGE_PREVIOUS}
      </Button>
      <Button
        type="button"
        variant="secondary"
        size="sm"
        disabled={!coSau}
        onClick={() => diToiTrang(sangTrangSau(nganXep, conTroTiep))}
      >
        {PAGE_NEXT}
        <ChevronRight aria-hidden="true" />
      </Button>
    </nav>
  );
}
