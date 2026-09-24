"""Canh tầng tiến độ: `kb/90-ephemeral/tien-do/<module>.json` và tệp sinh `tien-do.md`.

VÌ SAO CÓ HOOK NÀY: AI không có trí nhớ giữa các phiên, nên "đã làm tới đâu, còn nợ gì" chỉ tồn
tại nếu có người GHI nó xuống. Mà ghi tiến độ là việc dễ bỏ qua nhất — nó không làm gì đỏ, không
ai mất gì trong phiên này; cái giá rơi hết vào phiên sau, phiên phải khám phá lại hệ thống và
rút ra kết luận khác. Một quy ước không có hook canh là một quy ước sẽ trôi (luật 9).

BA CÂU HỎI, BA THỜI ĐIỂM — một tệp, vì chúng canh cùng một sự thật:

  PreToolUse    lần ghi này có làm hỏng hay XOÁ MẤT bản ghi của agent khác không
  Stop          phiên này sửa mã của module nào mà không cập nhật tiến độ module ấy
  SessionStart  phiên mới mở ra thấy ngay module nào đang dở

LUẬT NẶNG NHẤT LÀ R6 (mục biến mất). Mô hình ghi đã chọn là *mỗi module một tệp* nên hai agent
không bao giờ mở cùng một tệp — nhưng một agent ghi đè trọn tệp của CHÍNH module mình từ một bản
đọc cũ thì vẫn xoá sạch mục agent trước vừa thêm. Đó đúng là chuyện đã suýt xảy ra ngày
17/09/2026 với `kb/` (xem ban-giao-phien.md §5), và không có gì đỏ khi nó xảy ra.

Hook hỏng KHÔNG được chặn việc: mọi thứ bọc try/except, không đọc được thì cho qua.
"""

from __future__ import annotations

import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "progress_guard"

THU_MUC = "kb/90-ephemeral/tien-do"
TEP_SINH = "kb/90-ephemeral/tien-do.md"

TRANG_THAI = ("chua_lam", "dang_lam", "xong", "treo")

# Module CHUNG cho việc không thuộc đơn vị triển khai nào: chạy migration thật, dọn lịch sử git,
# rà lại cả lớp hook. Không có nó thì những việc ấy bị nhét bừa vào một module để có chỗ đứng,
# và module ấy thành thùng rác.
MODULE_CHUNG = "_chung"

# Thư mục cấp một KHÔNG phải đơn vị triển khai. DANH SÁCH LOẠI TRỪ, cố ý — ban-giao-phien.md §5
# ghi lại năm ca cùng một dạng: danh sách trắng tên thư mục là hình dạng sai cho kho này, vì đơn
# vị triển khai thứ mười một thêm vào sẽ lặng lẽ nằm ngoài mọi phép kiểm. Loại trừ thì hỏng theo
# chiều ngược lại — thừa một cái tên, chứ không sót.
KHONG_PHAI_MODULE = {
    "kb", "docs", "tasks", "tmp", "build", "dist", "gen", "vendor", "node_modules",
    "logs", "scripts",
}


def cac_module(root: str) -> set[str]:
    ra = {MODULE_CHUNG}
    try:
        for ten in os.listdir(root):
            if ten.startswith(".") or ten in KHONG_PHAI_MODULE:
                continue
            if os.path.isdir(os.path.join(root, ten)):
                ra.add(ten)
    except Exception:
        pass
    return ra


def chuan(p: str) -> str:
    return (p or "").replace("\\", "/").lstrip("./")


def la_tep_module(rel: str) -> str | None:
    m = re.search(re.escape(THU_MUC) + r"/([A-Za-z0-9_\-]+)\.json$", rel)
    return m.group(1) if m else None


def cau_hoi(root: str) -> dict:
    try:
        p = os.path.join(root, "kb", "00-foundation", "open-questions.json")
        with open(p, encoding="utf-8") as f:
            return {int(q["id"]): q.get("status", "") for q in json.load(f)}
    except Exception:
        return {}


def ids_tren_dia(path: str) -> list[str]:
    try:
        with open(path, encoding="utf-8") as f:
            return [str(x.get("id")) for x in json.load(f).get("muc", []) if x.get("id")]
    except Exception:
        return []


TAIL = [
    "  Lược đồ một mục — năm khoá bắt buộc:",
    "",
    '    {"id": "<kebab>", "viec": "<một dòng>",',
    '     "trang_thai": "chua_lam|dang_lam|xong|treo",',
    '     "bang_chung": "<file:line hoặc lệnh đã chạy — BẮT BUỘC khi xong>",',
    '     "no_confirm": [<id câu hỏi còn OPEN>], "tiep_theo": "<bước kế>"}',
    "",
    '  Khoá tuỳ chọn thứ sáu: "menu": "<slug>" hoặc ["<slug>", …] — slug của',
    "  docs/ui-ux/NN-<slug>.md. Gom mục của nhiều module về một menu (`/develop-*`).",
    "",
    "  → Thủ tục: `/progress` · Luật 9: .claude/rules/critical/9-knowledge-single-source.md",
]


# --------------------------------------------------------------------------
# PreToolUse
# --------------------------------------------------------------------------

def kiem_ghi(data: dict) -> None:
    tool = c.tool_of(data)
    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path:
        return
    rel = chuan(path)
    root = c.project_root()

    # R1 — tệp sinh, không ai gõ tay vào
    if rel.endswith(TEP_SINH):
        c.block(HOOK, f"ghi tay vào tệp SINH RA — {TEP_SINH}",
                ["tệp này do `python tools/tien_do.py` sinh; sửa tay mất sạch lần sinh sau"],
                [f"  Sửa `{THU_MUC}/<module>.json` của module mình, rồi chạy `make kb`.",
                 "",
                 "  Một tệp cho NHIỀU agent cùng ghi chính là hình dạng ROUTING.md §3 cấm:",
                 "  lần ghi sau âm thầm xoá lần ghi trước. Mỗi module một tệp thì không ai",
                 "  chạm tệp ai, và tệp đọc vẫn chỉ có một.",
                 "",
                 "  → Thủ tục: `/progress`"],
                tool=tool, path=path)

    ten = la_tep_module(rel)
    if ten is None:
        return

    # R2 — tên tệp phải là một module có thật
    hop_le = cac_module(root)
    if ten not in hop_le:
        c.block(HOOK, f"module không tồn tại — {ten}",
                [f"'{ten}' không phải thư mục cấp một nào, cũng không phải `{MODULE_CHUNG}`"],
                ["  Tên tệp LÀ tên module, và module là một đơn vị triển khai có thật trên đĩa.",
                 f"  Việc không thuộc đơn vị nào thì vào `{MODULE_CHUNG}.json`.",
                 "",
                 "  Có: " + ", ".join(sorted(hop_le)[:12])],
                tool=tool, path=path)

    noi_dung = c.noi_dung_sau_sua(ti)
    if not noi_dung.strip():
        return

    # R3 — JSON phải đọc được
    try:
        d = json.loads(noi_dung)
    except Exception as ex:
        c.block(HOOK, f"JSON hỏng — {rel}", [str(ex)[:160]],
                ["  Tệp tiến độ hỏng là tiến độ BIẾN MẤT với phiên sau, không phải một lỗi cú pháp.",
                 ""] + TAIL, tool=tool, path=path)
        return

    loi: list[str] = []
    if d.get("module") != ten:
        loi.append(f"khoá `module` là '{d.get('module')}', phải là '{ten}' — đúng tên tệp")
    if not re.match(r"^\d{4}-\d{2}-\d{2}$", str(d.get("cap_nhat", ""))):
        loi.append("thiếu `cap_nhat` dạng YYYY-MM-DD — hạn của tệp đọc tính từ khoá này")
    muc = d.get("muc")
    if not isinstance(muc, list):
        loi.append("thiếu mảng `muc`")
        muc = []

    # R4 + R5 — từng mục, và id không được trùng
    da_thay: set[str] = set()
    tt_cau = cau_hoi(root)
    for i, x in enumerate(muc):
        if not isinstance(x, dict):
            loi.append(f"muc[{i}] không phải object")
            continue
        mid = str(x.get("id", ""))
        nhan = mid or f"muc[{i}]"
        if not re.match(r"^[a-z0-9]+(-[a-z0-9]+)*$", mid):
            loi.append(f"{nhan}: `id` phải là kebab-case, không rỗng")
        elif mid in da_thay:
            loi.append(f"{nhan}: `id` trùng trong cùng tệp — hai mục một tên là hai mục lẫn nhau")
        da_thay.add(mid)
        if not str(x.get("viec", "")).strip():
            loi.append(f"{nhan}: thiếu `viec`")
        tt = x.get("trang_thai")
        if tt not in TRANG_THAI:
            loi.append(f"{nhan}: `trang_thai` '{tt}' ngoài {'|'.join(TRANG_THAI)}")
        if tt == "xong" and not str(x.get("bang_chung", "")).strip():
            loi.append(f"{nhan}: khai `xong` mà không có `bang_chung`")
        # R7 — nợ confirm LIÊN KẾT tới nguồn chuẩn, không chép, và không trỏ câu đã chốt
        for q in x.get("no_confirm") or []:
            try:
                qi = int(q)
            except Exception:
                loi.append(f"{nhan}: `no_confirm` phải là số hiệu câu hỏi")
                continue
            if tt_cau and qi not in tt_cau:
                loi.append(f"{nhan}: câu #{qi} không có trong open-questions.json")
            elif tt_cau.get(qi) == "DECIDED":
                loi.append(f"{nhan}: câu #{qi} đã DECIDED — không còn chặn được gì")
        # R8 — `menu` (tuỳ chọn) là LIÊN KẾT tới một đặc tả có thật, không phải nhãn tự đặt.
        # Nó là trục để `tien_do.py` gom mục của NHIỀU module về một menu; một slug gõ sai thì
        # mục ấy lặng lẽ rơi khỏi mục menu của nó, và phiên sau chạy `/develop-*` không thấy nó.
        if "menu" in x:
            ds = x["menu"] if isinstance(x["menu"], list) else [x["menu"]]
            cac = c.cac_menu(root)
            for s in ds:
                if not isinstance(s, str) or (cac and s not in cac):
                    loi.append(f"{nhan}: `menu` '{s}' không phải slug nào trong {c.MENU_DIR}/ "
                               f"— có: {', '.join(sorted(cac))}")

    if loi:
        c.block(HOOK, f"tiến độ sai lược đồ — {rel}", loi,
                ["  Một mục khai `xong` mà không có bằng chứng là thứ phiên sau tin rồi đi tiếp",
                 "  trên nền không có gì. `bang_chung` là file:line, hoặc lệnh đã chạy và kết quả.",
                 ""] + TAIL, tool=tool, path=path)

    # R6 — KHÔNG được làm biến mất mục đã có trên đĩa
    cu = ids_tren_dia(os.path.join(root, rel.replace("/", os.sep)))
    mat = [x for x in cu if x not in da_thay]
    if mat:
        c.block(HOOK, f"lần ghi này XOÁ {len(mat)} mục đang có — {rel}",
                [f"biến mất: {x}" for x in mat],
                ["  Gần như chắc chắn đây là ghi đè trọn tệp từ một bản đọc CŨ: agent khác đã",
                 "  thêm mục sau lúc bạn đọc. Ngày 17/09/2026 đúng thao tác này suýt xoá sáu chỗ",
                 "  sửa của một phiên song song (ban-giao-phien.md §5).",
                 "",
                 "  Đọc lại tệp trên đĩa, rồi dùng Edit cho đúng mục mình đổi.",
                 "  Việc đã xong thì đổi `trang_thai` thành `xong` — KHÔNG xoá mục đi: 'đã từng",
                 "  phải làm' là thứ phiên sau cần biết.",
                 "",
                 "  Một mục thực sự sai và phải bỏ: nói với người dùng trước (luật 7)."],
                tool=tool, path=path)


# --------------------------------------------------------------------------
# Stop
# --------------------------------------------------------------------------

MA = (".go", ".ts", ".tsx", ".sql", ".proto", ".json", ".py")


def module_cua(rel: str, hop_le: set[str]) -> str | None:
    segs = [s for s in chuan(rel).split("/") if s]
    return segs[0] if segs and segs[0] in hop_le else None


def doc_phien(transcript: str, hop_le: set[str]) -> tuple[set[str], set[str]]:
    """(module có mã bị sửa, module có tiến độ được ghi)."""
    sua: set[str] = set()
    ghi: set[str] = set()
    if not transcript or not os.path.exists(transcript):
        return sua, ghi
    try:
        with open(transcript, encoding="utf-8", errors="ignore") as f:
            for line in f:
                if not line.strip():
                    continue
                if not any(t in line for t in ('"Edit"', '"Write"', '"MultiEdit"')):
                    continue
                for m in re.finditer(r'"file_path"\s*:\s*"([^"]+)"', line):
                    rel = chuan(m.group(1).replace("\\\\", "/"))
                    ten = la_tep_module(rel)
                    if ten:
                        ghi.add(ten)
                        continue
                    if not rel.endswith(MA):
                        continue
                    mod = module_cua(rel, hop_le)
                    if mod:
                        sua.add(mod)
    except Exception:
        pass
    return sua, ghi


def kiem_ket_phien(data: dict) -> None:
    if data.get("stop_hook_active"):
        return
    root = c.project_root()
    hop_le = cac_module(root)
    sua, ghi = doc_phien(data.get("transcript_path") or "", hop_le)
    thieu = sorted(sua - ghi)
    if not thieu:
        return
    c.warn(HOOK, f"phiên này sửa mã của {len(thieu)} module mà không cập nhật tiến độ",
           [f"{m} -> {THU_MUC}/{m}.json chưa được ghi" for m in thieu],
           ["  Đây là thứ duy nhất phiên sau có để đi tiếp. Không ghi thì phiên sau đọc mã và",
            "  đoán lại — chậm, tốn, và ra kết luận khác lần này.",
            "",
            "  Với mỗi module: mở tệp trên, đổi mục vừa làm sang `dang_lam`/`xong` kèm",
            "  `bang_chung`, thêm mục mới nếu phát sinh, rồi `make kb`.",
            "",
            "  Sửa xong không đẻ ra việc gì để ghi (đổi chú thích, sửa lỗi chính tả): nói thẳng",
            "  câu ấy ra rồi dừng. Đừng ghi một mục rỗng cho qua cổng.",
            "",
            "  → Thủ tục: `/progress`"],
           tool="Stop", path="")


# --------------------------------------------------------------------------
# SessionStart
# --------------------------------------------------------------------------

def tom_tat() -> None:
    root = c.project_root()
    thu_muc = os.path.join(root, THU_MUC.replace("/", os.sep))
    if not os.path.isdir(thu_muc):
        return
    dang: list[str] = []
    no: set[int] = set()
    dem = 0
    for ten in sorted(os.listdir(thu_muc)):
        if not ten.endswith(".json"):
            continue
        try:
            with open(os.path.join(thu_muc, ten), encoding="utf-8") as f:
                d = json.load(f)
        except Exception:
            continue
        for x in d.get("muc", []):
            dem += 1
            for q in x.get("no_confirm") or []:
                try:
                    no.add(int(q))
                except Exception:
                    pass
            if x.get("trang_thai") == "dang_lam":
                # Cắt ngắn có chủ ý: đây là BẢNG ĐÈN đầu phiên, không phải nội dung. Một dòng
                # dài ba trăm ký tự trong băng-rôn khởi động là dòng không ai đọc hết.
                keo = str(x.get("tiep_theo") or x.get("viec") or "")
                if len(keo) > 100:
                    keo = keo[:97].rstrip() + "..."
                dang.append(f"{d.get('module')}/{x.get('id')} — {keo}")
    if not dem:
        return
    out = [f"[ViGov] Tiến độ: {dem} mục · {len(dang)} ĐANG LÀM · {len(no)} câu hỏi còn chặn. "
           f"Chi tiết: {TEP_SINH}"]
    for d in dang[:5]:
        out.append(f"[ViGov]   đang làm: {d}")
    if len(dang) > 5:
        out.append(f"[ViGov]   ... còn {len(dang) - 5} mục nữa")
    out.append("[ViGov] Sửa mã module nào thì cập nhật " + THU_MUC + "/<module>.json trước khi kết phiên.")
    print("\n".join(out))


def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    # Bộ tự kiểm (tools/test_hooks.py) gửi payload không có `hook_event_name`. Mặc định về
    # PreToolUse là cố ý: đó là nhánh CHẶN, tức nhánh phải có ca test.
    ev = data.get("hook_event_name") or ("SessionStart" if not c.input_of(data) else "PreToolUse")
    try:
        if ev == "SessionStart":
            tom_tat()
        elif ev == "Stop":
            kiem_ket_phien(data)
        else:
            kiem_ghi(data)
    except SystemExit:
        raise
    except Exception:
        sys.exit(0)          # hook hỏng không bao giờ được chặn việc


if __name__ == "__main__":
    main()
