#!/usr/bin/env python3
"""Đối chiếu bảng map biến môi trường ở `deploy/cau-hinh/README.md` với mã và với sổ đăng ký.

LỚP LỖI: các bảng ở `deploy/cau-hinh/README.md` là thứ DUY NHẤT trong kho trả lời câu "biến này do
ConfigMap hay Secret cấp, và pod nào dừng nếu thiếu nó". `core/config` không biết phần đầu — nó chỉ
đọc `os.Getenv`. `.env.example` không biết — nó chỉ giữ chỗ. `hooks/env_contract_guard.py` cũng
không, và nó nói thẳng điều đó trong phần WHAT IT DELIBERATELY DOES NOT CHECK.

Vì thế bảng ấy là tài liệu viết tay, và tài liệu viết tay cạnh một danh sách sinh ra từ mã là đúng
hình dạng sẽ trôi: thêm một biến vào `config.Load` là việc năm giây, cập nhật bảng là việc người ta
định làm sau. Lúc bảng thiếu một dòng, người vận hành đọc nó và tin rằng đã khai đủ — rồi pod
`CrashLoopBackOff` với một cái tên không có trong tài liệu nào.

Cổng này không kiểm NỘI DUNG của cột "k8s cấp bằng" — không gì quyết được điều đó từ mã. Nó kiểm đúng
thứ quyết được:

  1. bảng có đúng những biến mà `core/config` thật sự đọc, và `.env.example` có dòng cho từng biến;
  2. mỗi `os.Getenv` trong `core/config` có một lời gọi `r.read("TÊN", …, <mức>, <nhóm>…)` nói nó
     bắt buộc tới đâu và thuộc nhóm nào — biến đọc ngoài `r.read` là biến không ai quyết;
  3. cột "Bắt buộc" khớp với mã (từ 29/09/2026, `config.Uses`): biến mà ít nhất một dịch vụ khai
     nhóm của nó VÀ mức là `requiredEverywhere`/`requiredInProd` thì ô phải bắt đầu bằng `có`
     (`có (prod)` cho `requiredInProd`) và gọi tên ĐÚNG những dịch vụ ấy (hoặc `mọi dịch vụ` khi
     là tất cả); biến không dịch vụ nào bắt buộc thì ô KHÔNG được bắt đầu bằng `có`.

Dịch vụ khai nhóm nào: `config.Uses(...)` trong `service-*/cmd/server/main.go`. Biến nền (không nhóm:
ENV, DATABASE_DSN, DANGEROUS_AUTH_BYPASS) thuộc mọi dịch vụ.

VÌ SAO NÓ ĐỨNG CẠNH `hooks/env_contract_guard.py` chứ không thay: hook chặn lúc GHI và chỉ nhìn một
tệp. Nó không bao giờ đọc lại `deploy/cau-hinh/README.md`, nên một biến thêm hôm nay và một bảng quên
cập nhật hôm qua là hai tệp nó không bao giờ nhìn cùng lúc.

Chạy:  python tools/check_env_map.py
Mã thoát: 0 = khớp · 1 = lệch, hoặc không đọc được một trong các tệp nguồn.
"""

from __future__ import annotations

import glob
import io
import os
import re
import sys

ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

CONFIG_DIR = os.path.join(ROOT, "core", "config")
ENV_MAU = os.path.join(ROOT, ".env.example")
README = os.path.join(ROOT, "deploy", "cau-hinh", "README.md")
MAINS = os.path.join(ROOT, "service-*", "cmd", "server", "main.go")

# `os.Getenv("X")` / `os.LookupEnv("X")` — cùng biểu thức hook dùng, cố ý: hai nơi đọc cùng
# một thứ mà bằng hai biểu thức khác nhau là hai câu trả lời sẽ lệch.
DOC_ENV = re.compile(r"os\.(?:Getenv|LookupEnv)\s*\(\s*\"([A-Z0-9_]+)\"\s*\)")

# `r.read("X", <giá trị thô>, <mức>, <nhóm>, ...)` trong `config.Load` — có thể xuống dòng.
DOC_NHOM = re.compile(
    r"\br\.read\(\s*\"([A-Z0-9_]+)\"\s*,.+?,\s*(requiredEverywhere|requiredInProd|optional)\s*((?:,\s*\w+\s*)*)\)",
    re.S,
)
BAT_BUOC = {"requiredEverywhere", "requiredInProd"}

# `config.Uses(config.A, config.B, ...)` trong main của một dịch vụ.
KHAI_NHOM = re.compile(r"config\.Uses\(([^)]*)\)", re.S)
TEN_NHOM = re.compile(r"config\.(\w+)")

# Dòng bảng: `| \`TEN_BIEN\` | <bắt buộc> | <k8s cấp bằng> | ...`
DONG_BANG = re.compile(r"^\|\s*`([A-Z0-9_]+)`\s*\|([^|]*)\|")

# Dòng khai trong `.env.example`.
DONG_ENV = re.compile(r"^([A-Z0-9_]+)=")

# Biến chỉ dùng cho phép kiểm, không dịch vụ nào đọc lúc chạy. Nó có dòng trong `.env.example`
# một cách chính đáng, nhưng nó KHÔNG thuộc bảng map: không pod nào cần nó.
CHI_DE_KIEM = {"VIGOV_TEST_DSN"}


def _doc(duong_dan: str) -> str:
    with io.open(duong_dan, encoding="utf-8") as f:
        return f.read()


def _bang_trong_readme(noi_dung: str) -> dict[str, str]:
    """Trả về {tên biến: ô 'Bắt buộc'} lấy từ mọi bảng map."""
    ra: dict[str, str] = {}
    for dong in noi_dung.splitlines():
        m = DONG_BANG.match(dong)
        if m:
            ra[m.group(1)] = m.group(2).strip()
    return ra


def _ma_config() -> str:
    """Mọi tệp .go không phải test của core/config, nối lại."""
    tep = sorted(
        f for f in glob.glob(os.path.join(CONFIG_DIR, "*.go")) if not f.endswith("_test.go")
    )
    return "\n".join(_doc(f) for f in tep)


def _khai_bao_dich_vu() -> dict[str, set[str]]:
    """{tên dịch vụ: tập nhóm nó khai} — đọc từ config.Uses của từng main."""
    ra: dict[str, set[str]] = {}
    for f in sorted(glob.glob(MAINS)):
        ten = os.path.basename(os.path.dirname(os.path.dirname(os.path.dirname(f))))
        ten = ten[len("service-"):]
        m = KHAI_NHOM.search(_doc(f))
        ra[ten] = set(TEN_NHOM.findall(m.group(1))) if m else set()
    return ra


def main() -> int:
    for f in (CONFIG_DIR, ENV_MAU, README):
        if not os.path.exists(f):
            print(f"[ĐỎ] không đọc được {os.path.relpath(f, ROOT)}")
            return 1

    cau_hinh = _ma_config()
    ma_doc = set(DOC_ENV.findall(cau_hinh))

    phan_loai: dict[str, tuple[str, set[str]]] = {}
    for ten, muc, nhom in DOC_NHOM.findall(cau_hinh):
        phan_loai[ten] = (muc, set(re.findall(r"\w+", nhom)))

    dich_vu = _khai_bao_dich_vu()
    if not dich_vu:
        print("[ĐỎ] không tìm thấy service-*/cmd/server/main.go nào — phép kiểm không có gì để so")
        return 1

    env_mau = {
        m.group(1)
        for dong in _doc(ENV_MAU).splitlines()
        if (m := DONG_ENV.match(dong))
    }

    bang = _bang_trong_readme(_doc(README))

    vi_pham: list[str] = []

    for ten in sorted(ma_doc - set(bang)):
        vi_pham.append(
            f"`core/config` đọc {ten} nhưng bảng map ở deploy/cau-hinh/README.md KHÔNG có dòng nào. "
            f"Người vận hành không biết phải khai nó ở ConfigMap hay Secret."
        )

    for ten in sorted(set(bang) - ma_doc - CHI_DE_KIEM):
        vi_pham.append(
            f"bảng map có {ten} nhưng `core/config` không đọc biến nào tên ấy — "
            f"đổi tên trong mã mà quên bảng, hoặc một dòng đã chết."
        )

    for ten in sorted(ma_doc - env_mau):
        vi_pham.append(
            f"`core/config` đọc {ten} nhưng `.env.example` không có dòng nào (luật 11, bất "
            f"biến 6). Biến ngoài sổ đăng ký là biến người sau phát hiện từ một stack trace."
        )

    for ten in sorted(ma_doc - set(phan_loai)):
        vi_pham.append(
            f"`core/config` đọc {ten} nhưng không qua `r.read(\"{ten}\", …, <mức>, <nhóm>)` — "
            f"không ai quyết nó bắt buộc tới đâu, và dịch vụ không khai nhóm vẫn đọc nó."
        )

    tat_ca = set(dich_vu)
    so_bat_buoc = 0
    for ten in sorted(ma_doc & set(phan_loai)):
        muc, nhom = phan_loai[ten]
        if muc in BAT_BUOC:
            can = tat_ca if not nhom else {dv for dv, khai in dich_vu.items() if khai & nhom}
        else:
            can = set()
        o = bang.get(ten)
        if o is None:
            continue  # đã báo ở vòng trên
        o_sach = o.replace("*", "").strip()
        if not can:
            if o_sach.startswith("có"):
                vi_pham.append(
                    f"{ten}: bảng ghi Bắt buộc {o!r} nhưng không dịch vụ nào bắt buộc nó "
                    f"(mức {muc}, nhóm {sorted(nhom) or 'nền'}) — người vận hành sẽ khai thừa."
                )
            continue
        so_bat_buoc += 1
        muon_dau = "có (prod)" if muc == "requiredInProd" else "có —"
        if not o_sach.startswith(muon_dau):
            vi_pham.append(
                f"{ten}: mức {muc} nên ô Bắt buộc phải bắt đầu bằng {muon_dau!r}, bảng ghi {o!r}."
            )
        if can == tat_ca and "mọi dịch vụ" in o_sach:
            continue
        ghi = {dv for dv in tat_ca if re.search(rf"\b{re.escape(dv)}\b", o_sach)}
        if ghi != can:
            vi_pham.append(
                f"{ten}: bắt buộc cho {sorted(can)} nhưng ô Bắt buộc ghi {sorted(ghi)} ({o!r})."
            )

    if vi_pham:
        print("[ĐỎ] bảng map biến môi trường lệch với mã")
        for v in vi_pham:
            print(f"        {v}")
        print()
        print("        Sửa ở deploy/cau-hinh/README.md mục 1–4 — và nhớ cột 'k8s cấp bằng': phép thử là")
        print("        'in ra một dòng log thì có đau không', không phải 'có nhạy cảm không'.")
        print("        Một DSN có mật khẩu là Secret dù nó trông như một địa chỉ.")
        print("        Dependency HOÀN TOÀN MỚI (Kafka chẳng hạn) là STOP CONDITION của luật")
        print("        11 câu 1: hỏi chủ cụm trước, đừng tự đặt tên rồi tự chọn ConfigMap.")
        return 1

    print(
        f"[PASS] map biến môi trường — {len(ma_doc)} biến `core/config` đọc · "
        f"{so_bat_buoc} bắt buộc cho ít nhất một trong {len(dich_vu)} dịch vụ · tất cả có dòng "
        f"trong `.env.example` và trong bảng map của deploy/cau-hinh/README.md"
    )
    return 0


if __name__ == "__main__":
    sys.exit(main())
