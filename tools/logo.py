"""Sinh bộ icon của Zalo Mini App từ MÃ, không phải từ một tệp ảnh ai đó kéo vào kho.

VÌ SAO LÀ MỘT TRÌNH SINH CHỨ KHÔNG PHẢI MẤY TỆP PNG:

    Một PNG nằm trong kho là một tệp nhị phân không ai đọc được, không ai sửa được, và không ai
    trả lời được câu "màu này lấy ở đâu ra". Khi thương hiệu đổi một sắc, hoặc Zalo đòi thêm một
    cỡ, thứ phải sửa là tệp này — một dòng — chứ không phải mở Photoshop rồi xuất lại năm tệp và
    hy vọng cả năm cùng đúng.

MÀU LẤY TỪ ĐÂU, và đây là một đính chính:

    Bản đầu của tệp này dùng `#1e3150` và gọi đó là "màu thương hiệu". SAI. `#1e3150` là màu
    KHUNG WEBSITE — `mask-icon` và `msapplication-TileColor` của vihatsoftware.com — và nó cũng
    là `headerColor` trong `citizen-app/app-config.json`. Nó không xuất hiện ở đâu trong logo.

    Hai màu thương hiệu THẬT đọc bằng cách đếm pixel trên chính tệp logo
    (`citizen-app/brand/logo-goc.png`), không phải bằng mắt: `#00aef4` xanh dương và `#78bd1a` xanh
    lá. Đó là hai màu duy nhất trong logo ngoài trắng; mọi giá trị khác chỉ là pixel khử răng cưa.

VÌ SAO KHÔNG DÙNG THẲNG BIỂU TƯỢNG THẬT, và đây là giới hạn phải nói ra:

    Biểu tượng trong `citizen-app/brand/logo-goc.png` chỉ **60×60 pixel**. Nâng lên 512 là gấp 8,5 lần. Đã thử cách
    tốt nhất có thể — vẽ hình tròn bằng toán cho viền sắc tuyệt đối, rồi truy dải trắng âm bản
    từ ảnh gốc — và kết quả vẫn răng cưa rõ, kèm vệt xanh dương viền quanh mảng xanh lá. Không
    đạt để nộp.

    Nên icon này là một biểu tượng DẪN XUẤT, không phải bản sao: giữ cấu trúc nhận ra được của
    thương hiệu — hình tròn, xanh dương trên, xanh lá dưới, một dải trắng âm bản cắt ngang — với
    hình học vẽ được chính xác ở mọi cỡ. Nó cố ý KHÔNG bắt chước đường cắt thật, vì một bản sao
    sai lệch của logo một công ty có thật tệ hơn hẳn một biểu tượng dẫn xuất rõ ràng.

    → Có tệp vector gốc (SVG/AI/EPS) hoặc PNG ≥512 thì thay được trong một lượt, và NÊN thay.

CHẠY:  python tools/logo.py
RA:    citizen-app/public/icon-<cỡ>.png

`public/` chứ không phải `assets/`: Vite chép nguyên `public/` vào bundle mà không băm tên, nên
`index.html` trỏ tới `./icon-192.png` là đường dẫn còn đúng sau khi dựng. Đặt ở `assets/` thì
tệp không lên bundle và favicon hỏng đúng trên máy thật — nơi không ai kiểm lại.
"""

from __future__ import annotations

import os

from PIL import Image, ImageDraw

XANH_DUONG = (0, 174, 244)   # #00aef4 — đếm từ pixel của brand/logo-goc.png
XANH_LA = (120, 189, 26)     # #78bd1a — đếm từ pixel của brand/logo-goc.png
TRANG = (255, 255, 255)

# 512: logo nộp cho Zalo · 192: icon màn hình chính · 180: apple-touch-icon · 48: ca khắc nghiệt
CO = (512, 192, 180, 48)

SIEU = 8   # vẽ ở 8x rồi thu nhỏ — đường chéo và cung tròn không có bậc thang ở 48


def ve(co: int) -> Image.Image:
    s = co * SIEU
    anh = Image.new("RGBA", (s, s), TRANG + (255,))
    d = ImageDraw.Draw(anh)

    # Nền trắng bo góc: logo thật sống trên nền trắng. Một ô màu đặc sẽ đẹp hơn trên màn hình
    # chính, nhưng nó là một thương hiệu khác với thương hiệu công ty đang dùng.
    nen = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    ImageDraw.Draw(nen).rounded_rectangle(
        [0, 0, s - 1, s - 1], radius=int(s * 0.22), fill=TRANG + (255,))

    # Hình tròn, lề 14% — đủ thở trong ô bo góc, đủ to để đọc ở 48.
    le = s * 0.14
    hop = [le, le, s - le, s - le]

    dai = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    dd = ImageDraw.Draw(dai)
    dd.ellipse(hop, fill=XANH_DUONG + (255,))

    # Nửa dưới xanh lá: cắt bằng đa giác thay vì pieslice, để mép trên của mảng xanh lá là một
    # ĐƯỜNG THẲNG trùng với dải trắng — pieslice sẽ để lại một cung mỏng lệch màu ở chỗ nối.
    giua = s * 0.50
    dd.rectangle([0, giua, s, s], fill=(0, 0, 0, 0))
    la = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    ImageDraw.Draw(la).ellipse(hop, fill=XANH_LA + (255,))
    ImageDraw.Draw(la).rectangle([0, 0, s, giua], fill=(0, 0, 0, 0))
    dai = Image.alpha_composite(dai, la)

    # Dải trắng âm bản: một nhát CHÉO cắt ngang hình tròn. Đây là phần DẪN XUẤT — logo thật có
    # đường cắt phức tạp hơn, và bắt chước nó từ ảnh 60px cho ra bản sao sai lệch.
    #
    # Chéo chứ không phải chữ V, và thử cả hai mới thấy vì sao: một chữ V đối xứng đọc thành mũi
    # tên tải xuống, và hai nhánh của nó cắt vào rìa dưới hình tròn để lại mấy mảnh vụn rời. Một
    # nhát chéo giữ đúng thứ mắt nhận ra ở logo thật — vòng tròn hai màu bị một nhát trắng xẻ
    # nghiêng — mà vẽ chính xác được ở mọi cỡ.
    dv = ImageDraw.Draw(dai)
    day = s * 0.085                     # bề dày nhát cắt
    trai, phai = -s * 0.05, s * 1.05    # vượt ra ngoài để nhát cắt xuyên hết đường tròn
    cao_trai, cao_phai = s * 0.46, s * 0.58   # nghiêng xuống phải, theo hướng logo thật
    dv.polygon([(trai, cao_trai), (phai, cao_phai),
                (phai, cao_phai + day), (trai, cao_trai + day)], fill=(0, 0, 0, 0))

    # Nhát thứ hai, mảnh hơn, song song — nhịp hai vạch, thứ làm biểu tượng không đọc thành một
    # quả bóng bị gạch đúng một nét.
    lech = s * 0.135
    day2 = day * 0.45
    dv.polygon([(trai, cao_trai + day + lech), (phai, cao_phai + day + lech),
                (phai, cao_phai + day + lech + day2), (trai, cao_trai + day + lech + day2)],
               fill=(0, 0, 0, 0))

    anh = Image.alpha_composite(nen, dai)
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
