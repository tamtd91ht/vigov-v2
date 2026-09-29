/**
 * DANH BẠ CÁN BỘ XÃ — giao diện theo bản mẫu `vi-gov/zalo-miniapp` (`features/directory/DirectoryPage`):
 * ô tìm, mỗi cán bộ một thẻ có ô chữ cái đầu, nút gọi tròn bên phải.
 *
 * DỮ LIỆU VÀ QUY TẮC GIỮ NGUYÊN của `StaffDirectoryScreen.tsx`: chỉ người đã đồng ý công khai (#12), số
 * gọi qua `tel:` (`dialTarget`) — mở màn quay số của máy, không đi qua mạng, không log, không lưu. Tìm
 * kiếm lọc NGAY TRÊN MÁY trong danh sách đã tải; từ khoá không gửi đi đâu.
 *
 * Không có nút "nhắn Zalo" như bản mẫu: chưa có lời gọi nền tảng nào cho việc ấy ở nửa này.
 */
import { useEffect, useMemo, useRef, useState } from "react";

import { communeStaffDirectory } from "../api/vigov-client";
import type { PublicStaff } from "../api/public-contract";
import { dialTarget, afterDirectoryLoad, type DirectoryState } from "./StaffDirectoryScreen";
import { DIRECTORY, DIRECTORY_UNIT_HEAD, COMMUNE_APP_UI, COMMUNE_APP_REPORTS, COMMUNE_APP_SCREENS } from "./copy";

import { Icon } from "./Icon";
import { StatusBlock } from "./commune-frame";
import { TextField } from "./input-field";

/** Chữ cái đầu của TÊN GỌI (từ cuối) — quy ước của bản mẫu. */
function initialOf(full_name: string): string {
  const last = full_name.trim().split(/\s+/).pop() ?? "";
  return (last[0] ?? "?").toUpperCase();
}

/** Bỏ dấu tiếng Việt và hoa thường — người lớn tuổi hay gõ không dấu ("chu tich"). */
export function stripDiacritics(s: string): string {
  return s.normalize("NFD").replace(/[̀-ͯ]/g, "").replace(/đ/g, "d").replace(/Đ/g, "D").toLowerCase();
}

/**
 * Lọc trên máy, không phân biệt hoa thường, không phân biệt dấu, và theo SỐ điện thoại (bỏ dấu cách, chấm
 * trong số) — như danh bạ của prototype. Không dấu hiệu nào của từ khoá rời khỏi máy.
 */
export function filterStaff(list: readonly PublicStaff[], keyword: string): readonly PublicStaff[] {
  const q = stripDiacritics(keyword.trim());
  if (q === "") return list;
  const number = q.replace(/\D/g, "");
  // Chỉ so theo số khi từ khoá TOÀN là số (cho phép dấu cách, chấm, gạch) — "tổ 3" không khớp mọi số có chữ 3.
  const by_number = number.length >= 3 && /^[\d\s.\-+]+$/.test(q);
  return list.filter(
    (staff) =>
      // The units a person heads are searchable too: "ha lam" finds the Trưởng thôn Hà Lam.
      [staff.full_name, staff.position, staff.org_unit, ...staff.residential_units_headed].some((o) => stripDiacritics(o).includes(q)) ||
      (by_number && [staff.office_phone, staff.mobile].some((o) => o.replace(/\D/g, "").includes(number))),
  );
}

/**
 * The commune's own order first: people with a `display_order` ascending, then everybody without one in
 * the order the server sent. STABLE — equal positions keep the server's order. The server already sends
 * this order (`danh_ba_cong_khai.go:54-56`); sorting here is the guard for the day it does not, so the
 * commune's chosen order is what the citizen sees either way. PURE.
 */
export function byDisplayOrder(list: readonly PublicStaff[]): PublicStaff[] {
  return list
    .map((staff, i) => ({ staff, i }))
    .sort((a, b) => {
      const x = a.staff.display_order;
      const y = b.staff.display_order;
      if (x !== null && y !== null && x !== y) return x - y;
      if (x !== null && y === null) return -1;
      if (x === null && y !== null) return 1;
      return a.i - b.i;
    })
    .map((e) => e.staff);
}

/**
 * "Trưởng thôn Hà Lam" for a unit this person heads. The unit name is the commune's own, and may already
 * begin with its kind ("Thôn Hà Lam", "Tổ dân phố 3") — then the line is "Trưởng thôn Hà Lam" / "Trưởng tổ
 * dân phố 3", not "Trưởng thôn Thôn Hà Lam". Otherwise it reads "Trưởng thôn <tên>". PURE.
 */
export function unitHeadLine(unit: string): string {
  const u = unit.trim();
  if (/^(thôn|tổ dân phố|tổ|khu phố|ấp|bản|xóm)(?=\s|$)/i.test(u)) {
    return `${DIRECTORY_UNIT_HEAD.head_of} ${u.charAt(0).toLocaleLowerCase("vi")}${u.slice(1)}`;
  }
  return `${DIRECTORY_UNIT_HEAD.head_of_village} ${u}`;
}

/**
 * Nhóm theo BỘ PHẬN, theo thứ tự máy chủ trả (không sắp lại). Bản mẫu chia "Lãnh đạo UBND xã" /
 * "Bộ phận chuyên môn" bằng một trường nhóm ViGov không có; `org_unit` là thứ xã thật sự nhập. Người
 * không ghi bộ phận vào nhóm "Cán bộ khác" ở cuối. THUẦN.
 */
export function groupByOrgUnit(
  list: readonly PublicStaff[],
  other_label: string,
): { org_unit: string; staff: PublicStaff[] }[] {
  const group: { org_unit: string; staff: PublicStaff[] }[] = [];
  const other: PublicStaff[] = [];
  for (const staff of list) {
    const org_unit = staff.org_unit.trim();
    if (org_unit === "") {
      other.push(staff);
      continue;
    }
    const found = group.find((n) => n.org_unit === org_unit);
    if (found) found.staff.push(staff);
    else group.push({ org_unit: org_unit, staff: [staff] });
  }
  if (other.length > 0) group.push({ org_unit: other_label, staff: other });
  return group;
}

function CommuneStaffCard({ staff }: { staff: PublicStaff }) {
  // Số cơ quan trước: công khai theo bản chất; di động chỉ khi người ấy đồng ý (#12).
  const number = staff.office_phone.trim() !== "" ? staff.office_phone : staff.mobile;
  const target = number.trim() === "" ? null : dialTarget(number);
  return (
    <li className="xa-the xa-can-bo">
      <span className="xa-can-bo__chu-dau" aria-hidden="true">
        {initialOf(staff.full_name)}
      </span>
      <span className="xa-can-bo__chu">
        <strong className="xa-can-bo__ten">{staff.full_name}</strong>
        {staff.position !== "" && <span>{staff.position}</span>}
        {staff.residential_units_headed.map((unit) => (
          <span key={unit}>{unitHeadLine(unit)}</span>
        ))}
        {staff.office_phone.trim() !== "" && (
          <span className="xa-phu">
            {DIRECTORY.office_phone}: {staff.office_phone.trim()}
          </span>
        )}
        {staff.mobile.trim() !== "" && (
          <span className="xa-phu">
            {DIRECTORY.mobile}: {staff.mobile.trim()}
          </span>
        )}
        {staff.has_zalo && <span className="xa-phu">{DIRECTORY.has_zalo}</span>}
      </span>
      {target !== null && (
        <a className="xa-can-bo__goi" href={target} aria-label={COMMUNE_APP_UI.call_person(staff.full_name)}>
          <Icon name="phone" size={22} />
          <span>{COMMUNE_APP_UI.call}</span>
        </a>
      )}
    </li>
  );
}

const ERROR_TEXT = {
  "loi-mang": DIRECTORY.network_error,
  "loi-may-chu": DIRECTORY.server_error,
  "khong-hop-le": COMMUNE_APP_REPORTS.directory_invalid,
} as const;

export function CommuneDirectoryBody(props: { page: DirectoryState; onLoad: () => void }) {
  const [keyword, setKeyword] = useState("");
  const { page } = props;
  const filtered = useMemo(
    () => (page.kind === "xong" ? filterStaff(byDisplayOrder(page.staff), keyword) : []),
    [page, keyword],
  );

  if (page.kind === "dang-tai") return <StatusBlock icon="users" text={DIRECTORY.loading} loading />;
  if (page.kind === "loi") {
    return (
      <StatusBlock
        icon="alert"
        error
        text={ERROR_TEXT[page.error]}
        button={page.error !== "khong-hop-le" ? { label: DIRECTORY.retry_button, onPress: props.onLoad } : undefined}
      />
    );
  }
  if (page.staff.length === 0) return <StatusBlock icon="users" text={COMMUNE_APP_REPORTS.directory_empty} />;

  return (
    <>
      <div className="xa-tim">
        <TextField
          id="xa-tim-can-bo"
          label={COMMUNE_APP_UI.search_directory}
          value={keyword}
          max={60}
          onChange={setKeyword}
        />
      </div>
      {filtered.length === 0 ? (
        <StatusBlock icon="users" text={COMMUNE_APP_UI.staff_not_found} />
      ) : (
        groupByOrgUnit(filtered, COMMUNE_APP_SCREENS.group_other).map((n) => (
          <section key={n.org_unit} className="xa-nhom">
            <h2 className="xa-dau-khoi__tieu-de xa-nhom__tieu-de">{n.org_unit}</h2>
            <ul className="xa-ds">
              {n.staff.map((staff, i) => (
                // Không có mã trong hợp đồng công khai (cố ý); danh sách không sắp lại, nên khoá là vị trí
                // trong nhóm cộng tên — đủ ổn cho một lần xem.
                <CommuneStaffCard key={`${i}-${staff.full_name}`} staff={staff} />
              ))}
            </ul>
          </section>
        ))
      )}
    </>
  );
}

export function useCommuneDirectory(domain: string) {
  const [page, setPage] = useState<DirectoryState>({ kind: "dang-tai" });
  const loaded = useRef(false);

  async function load() {
    setPage({ kind: "dang-tai" });
    setPage(afterDirectoryLoad(await communeStaffDirectory(domain)));
  }

  useEffect(() => {
    if (loaded.current) return;
    loaded.current = true;
    void load();
  }, []);

  return { page, reload: () => void load() };
}
