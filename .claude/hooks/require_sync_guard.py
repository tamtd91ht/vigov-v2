"""Canh việc đồng bộ với kho yêu cầu `../vigov-require` — BA/PM cập nhật, kho này phải theo.

BA và PM cập nhật nghiệp vụ + prototype ở `../vigov-require`. Kho này viết mã theo đó. Khoảng
trống nguy hiểm không phải "chưa ai đọc bên kia" — mà là **một agent viết mã cho một phân hệ
trong khi bản yêu cầu của phân hệ ấy vừa đổi tuần trước và không ai biết**. Mã ấy biên dịch
được, test xanh, và sai so với thứ khách đã chốt. Không có gì đỏ.

BA JOB, MỘT TỆP:

  SessionStart   phiên mới thấy ngay có ghi chú đối chiếu nào CHƯA ĐƯỢC TIẾP NHẬN không
  PreToolUse     chặn đúng khoảnh khắc sắp ghi vào module mà một ghi chú chưa tiếp nhận nhắc tên
  PreToolUse     chấm hình dạng của chính tầng `kb/50-doi-chieu/` — ghi chú và sổ neo

VÌ SAO "ĐÃ TIẾP NHẬN" LÀ SUY RA, KHÔNG PHẢI MỘT CỜ ĐƯỢC GHI.

Cách hiển nhiên là để `neo.json` giữ một danh sách `da_tieu_thu`. Cách ấy hỏng theo đúng kiểu
kho này đã ghi nhận năm lần: một cột trạng thái do người (hay agent) tự đặt sẽ lệch khỏi sự
thật, và bản lệch là bản còn lại. Ai đó đánh dấu "đã tiếp nhận" cho xong việc thì cờ tắt mà
không có việc nào được mở — rào vẫn xanh, vẫn chạy, và đã mù.

Nên câu hỏi được hỏi ngược lại, và hỏi vào thứ không giả được: **sổ tiến độ của module ấy có
trích dẫn tên tệp ghi chú không.** Có trích tức là ai đó đã mở việc thật cho nó. Không trích
thì dù có đánh dấu gì cũng không tính. Cùng một hình dạng luật 10 bất biến 3 dùng cho `quá
hạn`: SUY RA từ dữ kiện, đừng nuôi một cột song song.

GIÁ PHẢI TRẢ MỖI PHIÊN, và đây là ràng buộc người dùng đặt ra: phiên nào KHÔNG có gì mới thì
hook đọc vài tệp nhỏ, thấy sạch, **không in một chữ nào**. Không nạp ngữ cảnh, không tốn token.
Chỉ khi có ghi chú chưa tiếp nhận nó mới lên tiếng.

Ràng buộc: chỉ thư viện chuẩn · nhanh · hỏng thì IM LẶNG cho qua, không bao giờ chặn nhầm vì
chính nó lỗi.
"""

from __future__ import annotations

import json
import os
import re
import sys

sys.path.insert(0, os.path.dirname(os.path.abspath(__file__)))
import _common as c  # noqa: E402

HOOK = "require_sync_guard"

TANG = "kb/50-doi-chieu"
NEO = f"{TANG}/neo.json"
SO_TIEN_DO = "kb/90-ephemeral/tien-do"

# Khoá BẮT BUỘC ở frontmatter một ghi chú đối chiếu.
#
# Bốn khoá đầu là BẰNG CHỨNG, không phải siêu dữ liệu cho đẹp: một ghi chú không nói nó đọc
# branch nào, từ commit nào tới commit nào, là một ghi chú không ai kiểm lại được — và lần
# review sau không biết nối tiếp từ đâu. `anh_huong` là khoá MÁY ĐỌC: thiếu nó thì hook phải
# đoán module từ văn xuôi, tức đoán sai và chặn nhầm.
KHOA_BAT_BUOC = ("branch", "sha_tu", "sha_den", "ngay_review", "anh_huong")

FM = re.compile(r"\A---\r?\n(.*?)\r?\n---\r?\n", re.S)
SHA = re.compile(r"^[0-9a-f]{7,40}$")

# Thư mục cấp một KHÔNG phải một module có sổ tiến độ. Cùng danh sách với
# `hooks/progress_guard.py` — hai bản sao của một danh sách là hai bản sẽ lệch, nên nếu danh
# sách bên kia đổi thì đổi cả ở đây. Để riêng vì hook kia không xuất nó ra.
KHONG_PHAI_MODULE = {
    "kb", "docs", "tasks", "tmp", "build", "dist", "gen", "vendor", "node_modules",
    "deploy",
}


def goc() -> str:
    return c.project_root()


def cac_module(root: str) -> set[str]:
    """Tên module = thư mục cấp một có thật trên đĩa, trừ danh sách loại."""
    try:
        return {t for t in os.listdir(root)
                if not t.startswith(".")
                and t not in KHONG_PHAI_MODULE
                and os.path.isdir(os.path.join(root, t))}
    except Exception:
        return set()


def module_cua(rel: str, hop_le: set[str]) -> str | None:
    """Module sở hữu đường dẫn này — đoạn đầu tiên, nếu nó là module có thật."""
    segs = [x for x in (rel or "").replace("\\", "/").split("/") if x not in ("", ".")]
    if segs and segs[0] in hop_le:
        return segs[0]
    return None


def doc_fm(duong: str) -> dict:
    """Đọc frontmatter của một ghi chú. CHỈ đầu tệp — ghi chú có thể dài, hook phải nhanh."""
    try:
        with open(duong, encoding="utf-8") as f:
            head = f.read(4000)
    except Exception:
        return {}
    m = FM.search(head)
    if not m:
        return {}
    ra: dict = {}
    for dong in m.group(1).splitlines():
        if ":" not in dong or dong.startswith((" ", "\t", "#")):
            continue
        k, _, v = dong.partition(":")
        ra[k.strip()] = v.strip()
    return ra


def tach_anh_huong(gia_tri: str) -> list[str]:
    """`anh_huong: [service-petitions, web-admin]` -> danh sách tên."""
    return [x.strip().strip("\"'") for x in gia_tri.strip().strip("[]").split(",") if x.strip()]


def ghi_chu(root: str) -> list[tuple[str, dict]]:
    """Mọi ghi chú đối chiếu trên đĩa, kèm frontmatter đã tách."""
    d = os.path.join(root, TANG.replace("/", os.sep))
    if not os.path.isdir(d):
        return []
    ra = []
    for ten in sorted(os.listdir(d)):
        if not ten.endswith(".md") or ten == "README.md":
            continue
        ra.append((ten, doc_fm(os.path.join(d, ten))))
    return ra


def so_co_trich(root: str, module: str, ten_ghi_chu: str) -> bool:
    """Sổ tiến độ của `module` có trích dẫn tên tệp ghi chú này không.

    Đây là toàn bộ định nghĩa của "đã tiếp nhận", và nó cố ý thô: chỉ cần chuỗi tên tệp xuất
    hiện ở đâu đó trong sổ. Chặt hơn — đòi đúng một khoá, đúng một chỗ — sẽ chặn những cách
    ghi hợp lệ mà người viết sổ nghĩ ra, và một rào chặn cái đúng là rào người ta tắt đi.
    """
    p = os.path.join(root, SO_TIEN_DO.replace("/", os.sep), f"{module}.json")
    try:
        with open(p, encoding="utf-8") as f:
            return ten_ghi_chu in f.read()
    except Exception:
        # Module chưa có sổ = chưa ai mở việc nào = chưa tiếp nhận. Hỏng thì ĐÓNG.
        return False


def chua_tiep_nhan(root: str) -> list[tuple[str, list[str]]]:
    """[(tên ghi chú, các module chưa mở việc cho nó)] — rỗng nghĩa là sạch."""
    ra = []
    for ten, fm in ghi_chu(root):
        mods = tach_anh_huong(fm.get("anh_huong", ""))
        thieu = [m for m in mods if not so_co_trich(root, m, ten)]
        if thieu:
            ra.append((ten, thieu))
    return ra


def neo_lech(root: str) -> list[str]:
    """Branch nào có ghi chú mới hơn neo — tức ĐÃ GHI CHÚ MÀ QUÊN DỜI NEO.

    LỖ NÀY LỘ RA KHI CHẠY THẬT, KHÔNG PHẢI KHI THIẾT KẾ. Ghi chú (bước 5) và dời neo (bước 6)
    là HAI thao tác, nên phiên chết giữa hai bước để lại một neo cũ hơn thứ đã đọc — và lần
    review sau đọc lại đúng khoảng vừa đọc. Trùng lặp thì tốn, nhưng cái đắt hơn là người đọc
    ghi chú thứ hai không phân biệt được nó là phần MỚI hay phần đã đọc rồi.

    Suy ra, không nuôi cờ: so `sha_den` của ghi chú mới nhất mỗi branch với `sha_den` trong
    `neo.json`. Khớp thì im. Hai con số cho một câu hỏi thì phải có chỗ đối chiếu chúng, nếu
    không cái lệch là cái không ai thấy.
    """
    try:
        with open(os.path.join(root, NEO.replace("/", os.sep)), encoding="utf-8") as f:
            br = (json.load(f) or {}).get("branches") or {}
    except Exception:
        return []

    # Ghi chú mới nhất của mỗi branch, theo `ngay_review` rồi tên tệp (tên mang ngày ở đầu).
    moi: dict[str, tuple[str, str, str]] = {}
    for ten, fm in ghi_chu(root):
        b, s = fm.get("branch", "").strip(), fm.get("sha_den", "").strip().strip("\"'")
        if not b or not s:
            continue
        khoa = (fm.get("ngay_review", ""), ten)
        if b not in moi or khoa > (moi[b][0], moi[b][1]):
            moi[b] = (khoa[0], ten, s)

    ra = []
    for b, (_, ten, sha) in sorted(moi.items()):
        neo_sha = str((br.get(b) or {}).get("sha_den", "")).strip()
        if neo_sha and neo_sha != sha:
            ra.append(f"{b}: ghi chú `{ten}` đọc tới {sha}, neo còn ở {neo_sha}")
        elif not neo_sha:
            ra.append(f"{b}: có ghi chú `{ten}` (tới {sha}) nhưng neo CHƯA CÓ branch này")
    return ra


# --------------------------------------------------------------------------
# SessionStart — im lặng khi sạch
# --------------------------------------------------------------------------

def bao_dau_phien(root: str) -> None:
    no = chua_tiep_nhan(root)
    lech = neo_lech(root)
    if not no and not lech:
        return                                    # SẠCH -> không in gì, không tốn token

    if no:
        print(f"[ViGov] {len(no)} ghi chú đối chiếu `../vigov-require` CHƯA ĐƯỢC TIẾP NHẬN:")
        for ten, mods in no[:4]:
            print(f"[ViGov]   {TANG}/{ten} -> chưa mở việc cho: {', '.join(mods)}")
        if len(no) > 4:
            print(f"[ViGov]   ... còn {len(no) - 4} ghi chú nữa")
        print("[ViGov] Đọc ghi chú RỒI mở việc vào sổ module ấy trước khi viết mã cho nó.")

    if lech:
        print("[ViGov] NEO LỆCH — lần đối chiếu sau sẽ ĐỌC TRÙNG khoảng đã đọc:")
        for d in lech[:3]:
            print(f"[ViGov]   {d}")
        print(f"[ViGov] Dời `sha_den` trong {NEO} cho khớp ghi chú mới nhất.")


# --------------------------------------------------------------------------
# PreToolUse — hình dạng của chính tầng kb/50-doi-chieu/
# --------------------------------------------------------------------------

def cham_ghi_chu(rel: str, tai_lieu: str, tool: str, path: str) -> None:
    m = FM.search(tai_lieu)
    if not m:
        c.block(HOOK, f"ghi chú đối chiếu không có frontmatter — {rel}",
                ["không thấy khối `---` ở đầu tệp"],
                ["  Ghi chú đối chiếu phải nói nó ĐỌC TỪ ĐÂU, nếu không nó là lời kể chứ không",
                 "  phải bằng chứng — và lần review sau không biết nối tiếp từ commit nào.",
                 "",
                 "    ---",
                 "    id: doi-chieu-<ngay>-<branch>",
                 "    tier: T2",
                 "    source: CURATED",
                 "    owner: architecture",
                 "    branch: main",
                 "    sha_tu: <sha đã review tới ở lần trước>",
                 "    sha_den: <sha mới nhất lần này>",
                 "    ngay_review: 2026-09-23",
                 "    anh_huong: [service-petitions, web-admin]",
                 "    ---",
                 "",
                 "  → Thủ tục: `/doi-chieu-require`"],
                tool=tool, path=path)

    fm = {}
    for dong in m.group(1).splitlines():
        if ":" in dong and not dong.startswith((" ", "\t", "#")):
            k, _, v = dong.partition(":")
            fm[k.strip()] = v.strip()

    thieu = [k for k in KHOA_BAT_BUOC if not fm.get(k)]
    if thieu:
        c.block(HOOK, f"ghi chú đối chiếu thiếu khoá bắt buộc — {rel}",
                [f"thiếu: {', '.join(thieu)}"],
                ["  `branch` · `sha_tu` · `sha_den` · `ngay_review` là BẰNG CHỨNG: không có chúng",
                 "  thì không ai kiểm lại được ghi chú này đọc từ đâu, và lần sau không biết nối",
                 "  tiếp từ commit nào — tức là bỏ sót commit mà không ai biết đã bỏ sót.",
                 "",
                 "  `anh_huong` là khoá MÁY ĐỌC. Thiếu nó thì rào phải đoán module từ văn xuôi,",
                 "  và một rào đoán là một rào chặn nhầm. Rỗng thật thì ghi `anh_huong: []`.",
                 "",
                 "  → Thủ tục: `/doi-chieu-require`"],
                tool=tool, path=path)

    xau = [f"{k}: '{fm[k]}' không phải dạng SHA" for k in ("sha_tu", "sha_den")
           if not SHA.match(fm[k].strip().strip("\"'"))]
    if xau:
        c.block(HOOK, f"SHA không đúng dạng — {rel}", xau,
                ["  Neo phải là commit THẬT của `../vigov-require`, viết bằng hex 7–40 ký tự.",
                 "  Một neo bịa là một neo làm lần review sau bắt đầu từ chỗ không có thật, và",
                 "  mọi commit giữa hai điểm ấy biến mất mà không ai thấy thiếu.",
                 "",
                 "  Lấy đúng SHA:  git -C ../vigov-require rev-parse origin/<branch>"],
                tool=tool, path=path)


def cham_neo(rel: str, tai_lieu: str, tool: str, path: str) -> None:
    try:
        d = json.loads(tai_lieu)
    except Exception as e:
        c.block(HOOK, f"sổ neo không phải JSON hợp lệ — {rel}", [str(e)[:120]],
                ["  Sổ neo là thứ lần review sau đọc để biết bắt đầu từ đâu. JSON hỏng ở đây",
                 "  nghĩa là lần sau đọc lại toàn bộ lịch sử, hoặc tệ hơn, bỏ qua im lặng."],
                tool=tool, path=path)
        return

    loi = []
    br = d.get("branches")
    if not isinstance(br, dict):
        loi.append("thiếu khoá `branches` dạng đối tượng")
    else:
        for ten, v in br.items():
            if not isinstance(v, dict):
                loi.append(f"branch '{ten}': phải là đối tượng")
                continue
            s = str(v.get("sha_den", "")).strip()
            if not SHA.match(s):
                loi.append(f"branch '{ten}': `sha_den` = '{s}' không phải dạng SHA")
            if not str(v.get("ngay_review", "")).strip():
                loi.append(f"branch '{ten}': thiếu `ngay_review`")
    if loi:
        c.block(HOOK, f"sổ neo sai hình dạng — {rel}", loi,
                ["  Hình dạng đúng:",
                 "",
                 '    {"branches": {"main": {"sha_den": "<sha>", "ngay_review": "2026-09-23"}}}',
                 "",
                 "  → Thủ tục: `/doi-chieu-require`"],
                tool=tool, path=path)


# --------------------------------------------------------------------------
# PreToolUse — chặn đúng khoảnh khắc sắp viết mã
# --------------------------------------------------------------------------

def chan_khi_viet_ma(root: str, rel: str, tool: str, path: str) -> None:
    hop_le = cac_module(root)
    mod = module_cua(rel, hop_le)
    if not mod:
        return

    no = [(ten, mods) for ten, mods in chua_tiep_nhan(root) if mod in mods]
    if not no:
        return                                    # module này sạch -> im lặng

    c.block(HOOK, f"`{mod}` có yêu cầu mới từ BA/PM chưa được tiếp nhận",
            [f"{TANG}/{ten}" for ten, _ in no],
            [f"  Bản yêu cầu của `{mod}` ở `../vigov-require` đã đổi, và chưa ai mở việc cho",
             "  thay đổi ấy. Viết mã lúc này là viết theo bản cũ: nó biên dịch được, test xanh,",
             "  và lệch với thứ khách đã chốt — không có gì đỏ để ai nhận ra.",
             "",
             "  ĐƯỜNG ĐI ĐÚNG, hai bước:",
             f"    1. Đọc ghi chú trên. Nó ghi rõ commit nào đổi gì ở phân hệ nào.",
             f"    2. Mở việc vào `{SO_TIEN_DO}/{mod}.json` — trong `bang_chung` TRÍCH TÊN TỆP",
             "       ghi chú. Đó chính là thứ rào này đọc để biết đã tiếp nhận (`/progress`).",
             "",
             "  Ghi chú hoá ra KHÔNG liên quan tới `" + mod + "`? Sửa `anh_huong` ở frontmatter",
             "  của nó — đừng mở một việc giả để rào im.",
             "",
             "  → Thủ tục: `/doi-chieu-require` · Kỹ năng: `skills/doi-chieu-require`"],
            tool=tool, path=path)


# --------------------------------------------------------------------------

def main() -> None:
    c.utf8_streams()
    data = c.read_input()
    root = goc()

    su_kien = data.get("hook_event_name") or ""
    tool = c.tool_of(data)

    # SessionStart: không có tool_name.
    if su_kien == "SessionStart" or (not tool and not data.get("tool_input")):
        bao_dau_phien(root)
        sys.exit(0)

    if tool not in ("Edit", "Write", "MultiEdit", "NotebookEdit"):
        sys.exit(0)

    ti = c.input_of(data)
    path = c.path_of(ti)
    if not path:
        sys.exit(0)

    rel = os.path.relpath(path, root).replace("\\", "/") if os.path.isabs(path) else path
    rel = rel.replace("\\", "/").lstrip("./")

    # Kho khác thì không phải việc của rào này.
    if c.ngoai_du_an(path):
        sys.exit(0)

    tai_lieu = c.noi_dung_sau_sua(ti)

    if rel == NEO:
        cham_neo(rel, tai_lieu, tool, path)
        sys.exit(0)

    if rel.startswith(TANG + "/") and rel.endswith(".md") and not rel.endswith("README.md"):
        cham_ghi_chu(rel, tai_lieu, tool, path)
        sys.exit(0)

    # Ghi vào SỔ TIẾN ĐỘ chính là đường thoát của lần chặn — không bao giờ chặn nó.
    if rel.startswith(SO_TIEN_DO + "/"):
        sys.exit(0)

    chan_khi_viet_ma(root, rel, tool, path)
    sys.exit(0)


if __name__ == "__main__":
    try:
        main()
    except SystemExit:
        raise
    except Exception:
        # Rào hỏng thì để công việc chạy tiếp. Một rào tự nó lỗi mà chặn cả phiên là thứ
        # người ta gỡ ra, và gỡ xong thì không còn rào nào.
        sys.exit(0)
