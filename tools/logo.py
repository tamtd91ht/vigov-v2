"""Sinh bộ icon của Zalo Mini App từ MÃ, không phải từ một tệp ảnh ai đó kéo vào kho.

VÌ SAO LÀ MỘT TRÌNH SINH CHỨ KHÔNG PHẢI MẤY TỆP PNG:

    Một PNG nằm trong kho là một tệp nhị phân không ai đọc được, không ai sửa được, và không ai
    trả lời được câu "màu này lấy ở đâu ra". Khi thương hiệu đổi một sắc navy, hoặc Zalo đòi
    thêm một cỡ, thứ phải sửa là tệp này — một dòng — chứ không phải mở Photoshop rồi xuất lại
    năm tệp và hy vọng cả năm cùng đúng.

    Màu `#1e3150` không do ai chọn cho đẹp: nó là màu thương hiệu đọc được từ `mask-icon` và
    `msapplication-TileColor` của vihatsoftware.com, và nó cũng là `headerColor` trong
    `citizen-app/app-config.json`. MỘT nguồn cho một sự thật (luật 9): đổi ở đây thì phải đổi
    cả hai chỗ kia, và tệp này nói rõ ra để người sau biết còn chỗ nào.

CHẠY:  python tools/logo.py
RA:    citizen-app/public/icon-<cỡ>.png

`public/` chứ không phải `assets/`: Vite chép nguyên `public/` vào bundle mà không băm tên, nên
`index.html` trỏ tới `./icon-192.png` là đường dẫn còn đúng sau khi dựng. Đặt ở `assets/` thì
tệp không lên bundle và favicon hỏng đúng trên máy thật — nơi không ai kiểm lại.

Bộ cỡ chọn theo thứ Zalo và trình duyệt thật sự đòi, không phải cho đủ bộ. 48 nằm trong danh
sách vì nó là cỡ KHẮC NGHIỆT NHẤT — một logo không đọc được ở 48 là một logo hỏng, và chỉ nhìn
bản 512 thì không bao giờ phát hiện ra.
"""

from __future__ import annotations

import os
import sys

from PIL import Image, ImageDraw, ImageFont

NAVY = (30, 49, 80)          # #1e3150 — màu thương hiệu, xem chú thích đầu tệp
TRANG = (255, 255, 255)

# 512: logo nộp cho Zalo · 192: icon màn hình chính · 180: apple-touch-icon · 48: ca khắc nghiệt
CO = (512, 192, 180, 48)

# Font đậm, xếp theo thứ tự ưu tiên. KHÔNG dùng font mặc định của Pillow làm phương án cuối:
# nó là font bitmap cỡ cố định, phóng lên 512 thì răng cưa — và một logo răng cưa nộp lên Zalo
# là thứ người duyệt nhìn thấy trước cả nội dung app.
FONT_UNG_VIEN = (
    "C:/Windows/Fonts/seguibl.ttf",    # Segoe UI Black
    "C:/Windows/Fonts/ariblk.ttf",     # Arial Black
    "C:/Windows/Fonts/arialbd.ttf",    # Arial Bold
    "/usr/share/fonts/truetype/dejavu/DejaVuSans-Bold.ttf",
)


def tim_font(co_chu: int) -> ImageFont.FreeTypeFont:
    for duong in FONT_UNG_VIEN:
        if os.path.exists(duong):
            return ImageFont.truetype(duong, co_chu)
    sys.exit("tools/logo.py: không tìm thấy font đậm nào. Thêm đường dẫn vào FONT_UNG_VIEN.")


def ve(co: int) -> Image.Image:
    """Vẽ ở 4x rồi thu nhỏ — khử răng cưa cho cả góc bo lẫn nét chữ.

    Vẽ thẳng ở cỡ đích thì góc bo bị bậc thang, rõ nhất ở 48.
    """
    s = co * 4
    anh = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    d = ImageDraw.Draw(anh)

    # Bo góc 22% — xấp xỉ squircle của iOS. Thấp hơn trông như ảnh chụp màn hình, cao hơn thành
    # hình tròn và mất đi cảm giác "đây là một ứng dụng".
    d.rounded_rectangle([0, 0, s - 1, s - 1], radius=int(s * 0.22), fill=NAVY)

    chu = "VHS"
    # Cỡ chữ theo TỶ LỆ, không phải số tuyệt đối, để mọi cỡ icon trông là một logo chứ không
    # phải bốn logo họ hàng.
    font = tim_font(int(s * 0.30))

    # Giãn chữ: ba chữ cái đậm dính nhau thành một khối đen ở cỡ nhỏ. Giãn ra thì ở 48 vẫn tách.
    gian = int(s * 0.022)
    rong = [d.textlength(k, font=font) for k in chu]
    tong = sum(rong) + gian * (len(chu) - 1)

    hop = d.textbbox((0, 0), chu, font=font)
    cao = hop[3] - hop[1]
    x = (s - tong) / 2
    # Nhích lên trên một chút: khối chữ + gạch chân cân bằng quang học quanh tâm hình, không
    # phải quanh tâm của riêng khối chữ.
    y = (s - cao) / 2 - hop[1] - s * 0.045
    for i, k in enumerate(chu):
        d.text((x, y), k, font=font, fill=TRANG)
        x += rong[i] + gian

    # Gạch chân: neo khối chữ xuống, và cho logo một nét ngang để mắt bám ở cỡ nhỏ.
    day = max(2, int(s * 0.028))
    rong_gach = tong * 0.86
    y_gach = (s + cao) / 2 + s * 0.045
    d.rounded_rectangle(
        [(s - rong_gach) / 2, y_gach, (s + rong_gach) / 2, y_gach + day],
        radius=day / 2, fill=TRANG)

    return anh.resize((co, co), Image.LANCZOS)


def main() -> None:
    goc = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))
    ra = os.path.join(goc, "citizen-app", "public")
    os.makedirs(ra, exist_ok=True)
    for co in CO:
        duong = os.path.join(ra, f"icon-{co}.png")
        ve(co).save(duong, "PNG", optimize=True)
        print(f"logo: {os.path.relpath(duong, goc).replace(os.sep, '/')}  {co}x{co}")


if __name__ == "__main__":
    main()
