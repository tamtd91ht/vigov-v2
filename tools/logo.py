"""Sinh bộ icon của Zalo Mini App từ VECTOR GỐC của logo ViHAT Software.

NGUỒN:  citizen-app/brand/lg_vhs_full.svg  (viewBox 0 0 1455 512)
CHẠY:   python tools/logo.py
RA:     citizen-app/public/icon-<cỡ>.png

VÌ SAO TỰ ĐỌC PATH THAY VÌ DÙNG THƯ VIỆN DỰNG SVG:

    Máy này không có cairosvg / svglib / reportlab, và cài cairo trên Windows là một cuộc phiêu
    lưu. Nhưng các nét tạo nên biểu tượng chỉ dùng M/L/H/V/C/Z — không cung elliptic, không
    gradient, không nét viền, không lỗ theo quy tắc even-odd. Đọc đúng bấy nhiêu lệnh là việc
    nhỏ, và đổi lại **không thêm một phụ thuộc nào vào một kho sắp nộp cho Zalo**.

    Gặp lệnh ngoài tập ấy thì hàm đọc **ném lỗi**, không đoán. Một trình đọc SVG đoán bừa sẽ cho
    ra một logo gần đúng, mà gần đúng là dạng sai khó phát hiện nhất — không ai soi một cái icon.

VÌ SAO CHỈ LẤY BIỂU TƯỢNG, BỎ PHẦN CHỮ:

    Tệp gốc là logo NGANG: biểu tượng ở `x 0..510`, chữ "ViHAT SOFTWARE" từ `x≈575` tới 1455. Ép
    cả logo ngang vào ô vuông thì chữ cao chưa tới 6% chiều cao icon — ở 48px nó là một vệt xám.
    Icon lấy biểu tượng; phần chữ đã nằm ngay trên đầu mọi màn hình trong app.

    Các nét được chọn theo HỘP BAO chứ không theo thứ tự trong tệp. Sửa SVG mà đổi thứ tự nét
    thì cách này vẫn đúng; đánh số cứng thì không, và sẽ hỏng lặng lẽ.

LỊCH SỬ NGẮN, để người sau khỏi đi lại:

    Bản 1 dùng `#1e3150` và gọi đó là màu thương hiệu — SAI, đó là màu khung website. Bản 2 dùng
    đúng hai màu thật (`#00aef4`, `#78bd1a`, đếm từ pixel) nhưng vẽ một biểu tượng DẪN XUẤT, vì
    nguồn khi đó chỉ là ảnh 60×60 không nâng lên 512 sạch được. Bản này dùng vector gốc, nên
    không còn phải dẫn xuất gì — và màu không còn phải đếm, nó nằm sẵn trong thuộc tính `fill`.
"""

from __future__ import annotations

import os
import re

from PIL import Image, ImageDraw

TRANG = (255, 255, 255)

# 512: logo nộp cho Zalo · 192: icon màn hình chính · 180: apple-touch-icon · 48: ca khắc nghiệt.
# 48 có trong danh sách vì nó là cỡ KHẮC NGHIỆT NHẤT — một logo không đọc được ở 48 là logo
# hỏng, và chỉ nhìn bản 512 thì không bao giờ phát hiện ra.
CO = (512, 192, 180, 48)

SIEU = 4               # vẽ ở 4x rồi thu nhỏ — cung tròn và đường chéo không còn bậc thang
X_BIEU_TUONG = 540.0   # biểu tượng nằm bên trái mốc này trong viewBox; chữ nằm bên phải
DOAN_BEZIER = 48       # số đoạn thẳng xấp xỉ một đường bezier bậc ba

_SO = r"[-+]?(?:\d*\.\d+|\d+\.?)(?:[eE][-+]?\d+)?"
_TOKEN = re.compile(rf"([A-Za-z])|({_SO})")
_PATH = re.compile(r'<path\s+d="([^"]+)"\s+fill="(#[0-9a-fA-F]{6})"', re.S)


def _bezier(p0, p1, p2, p3, n=DOAN_BEZIER):
    ra = []
    for i in range(1, n + 1):
        t = i / n
        u = 1 - t
        ra.append((
            u * u * u * p0[0] + 3 * u * u * t * p1[0] + 3 * u * t * t * p2[0] + t ** 3 * p3[0],
            u * u * u * p0[1] + 3 * u * u * t * p1[1] + 3 * u * t * t * p2[1] + t ** 3 * p3[1]))
    return ra


def doc_path(d: str) -> list[list[tuple[float, float]]]:
    """Đọc thuộc tính `d` thành các nét đa giác. Chỉ M/L/H/V/C/Z — gặp lệnh khác thì NÉM LỖI."""
    tok = _TOKEN.findall(d)
    i, lenh = 0, ""
    x = y = 0.0
    bat_dau = (0.0, 0.0)
    net: list[tuple[float, float]] = []
    tat_ca: list[list[tuple[float, float]]] = []

    def so() -> float:
        nonlocal i
        while i < len(tok) and not tok[i][1]:
            i += 1
        v = float(tok[i][1])
        i += 1
        return v

    while i < len(tok):
        if tok[i][0]:
            lenh = tok[i][0]
            i += 1
            if lenh in "Zz":
                if net:
                    tat_ca.append(net)
                net = []
                x, y = bat_dau
                continue
        if lenh in "Mm":
            nx, ny = so(), so()
            x, y = (x + nx, y + ny) if lenh == "m" else (nx, ny)
            if net:
                tat_ca.append(net)
            net = [(x, y)]
            bat_dau = (x, y)
            # Toạ độ tiếp theo sau một lệnh M là LINETO, không phải moveto nữa — quy tắc SVG,
            # và bỏ qua nó thì mỗi cặp số sau M sinh ra một nét rỗng.
            lenh = "l" if lenh == "m" else "L"
        elif lenh in "Ll":
            nx, ny = so(), so()
            x, y = (x + nx, y + ny) if lenh == "l" else (nx, ny)
            net.append((x, y))
        elif lenh in "Hh":
            nx = so()
            x = x + nx if lenh == "h" else nx
            net.append((x, y))
        elif lenh in "Vv":
            ny = so()
            y = y + ny if lenh == "v" else ny
            net.append((x, y))
        elif lenh in "Cc":
            a, b, c, e, f, g = (so() for _ in range(6))
            if lenh == "c":
                p1, p2, p3 = (x + a, y + b), (x + c, y + e), (x + f, y + g)
            else:
                p1, p2, p3 = (a, b), (c, e), (f, g)
            net.extend(_bezier((x, y), p1, p2, p3))
            x, y = p3
        else:
            raise ValueError(f"tools/logo.py: lệnh SVG chưa hỗ trợ: {lenh!r}. "
                             f"Bổ sung vào doc_path() thay vì đoán.")
    if net:
        tat_ca.append(net)
    return tat_ca


def net_bieu_tuong(duong_svg: str):
    """Các nét thuộc BIỂU TƯỢNG, kèm màu đọc từ `fill`. Lọc theo hộp bao, không theo thứ tự."""
    with open(duong_svg, encoding="utf-8") as f:
        svg = f.read()
    ra = []
    for d, mau in _PATH.findall(svg):
        rgb = tuple(int(mau[k:k + 2], 16) for k in (1, 3, 5))
        for net in doc_path(d):
            if max(p[0] for p in net) < X_BIEU_TUONG:
                ra.append((net, rgb))
    if not ra:
        raise ValueError("tools/logo.py: không tìm thấy nét nào của biểu tượng — SVG đã đổi?")
    return ra


def ve(co: int, net):
    s = co * SIEU
    tat_x = [p[0] for n, _ in net for p in n]
    tat_y = [p[1] for n, _ in net for p in n]
    w, h = max(tat_x) - min(tat_x), max(tat_y) - min(tat_y)

    # Lề 13%: biểu tượng cần chỗ thở trong ô bo góc, và đó cũng là lề iOS/Android chừa cho icon.
    le = s * 0.13
    ty_le = min((s - 2 * le) / w, (s - 2 * le) / h)
    dx = (s - w * ty_le) / 2 - min(tat_x) * ty_le
    dy = (s - h * ty_le) / 2 - min(tat_y) * ty_le

    anh = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    d = ImageDraw.Draw(anh)
    # Nền trắng bo góc: logo thật sống trên nền trắng. Một ô màu đặc bắt mắt hơn trên màn hình
    # chính, nhưng đó là một thương hiệu khác với thương hiệu công ty đang dùng.
    d.rounded_rectangle([0, 0, s - 1, s - 1], radius=int(s * 0.22), fill=TRANG + (255,))
    for poly, rgb in net:
        d.polygon([(p[0] * ty_le + dx, p[1] * ty_le + dy) for p in poly], fill=rgb + (255,))
    return anh.resize((co, co), Image.LANCZOS)


def main() -> None:
    goc = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    net = net_bieu_tuong(os.path.join(goc, "citizen-app", "brand", "lg_vhs_full.svg"))
    ra = os.path.join(goc, "citizen-app", "public")
    os.makedirs(ra, exist_ok=True)
    for co in CO:
        duong = os.path.join(ra, f"icon-{co}.png")
        ve(co, net).save(duong, "PNG", optimize=True)
        print(f"logo: {os.path.relpath(duong, goc).replace(os.sep, '/')}  {co}x{co}")


if __name__ == "__main__":
    main()
