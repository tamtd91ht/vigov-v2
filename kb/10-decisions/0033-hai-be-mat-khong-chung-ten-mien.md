---
id: 0033-hai-be-mat-khong-chung-ten-mien
tier: T1
source: CURATED
owner: architecture
derived_from_commit: 3bc42e0
expires: null
owns_facts:
  - "`POST /api/v1/sessions` là HAI tuyến khác nhau trên hai máy chủ, và đó là thực trạng chứ không phải nhầm lẫn"
  - "citizen-app gọi hai máy chủ; openapi.json và tools/ingress CỐ Ý không bao tuyến của backend thương mại"
  - "ràng buộc: bề mặt ViGov và bề mặt thương mại không bao giờ dùng chung một tên miền, chốt 22/09/2026"
---

# 0033. Hai bề mặt không bao giờ chung tên miền

**Trạng thái:** đã chốt · **Ngày:** 2026-09-22 · **Nối tiếp ADR 0031 · 0032** · **ADR 0020 giữ nguyên**

## Bối cảnh — một sự thật không suy ra được từ kho này

`citizen-app` chạy trong Mini App và gọi **hai máy chủ khác nhau**. Điều đó đã đúng từ ADR 0031
và 0032; thứ chưa ai viết ra là **hệ quả của nó trên không gian đường dẫn**:

| Cùng một đường dẫn | ViGov (kho này) | `vihat-miniapp` (kho anh em) |
|---|---|---|
| `POST /api/v1/sessions` | *"Đăng nhập bằng **email và mật khẩu**, mở một phiên làm việc"* — **cán bộ** | đăng nhập **một chạm bằng số Zalo**, đổi `accessToken` + `phoneToken` — **khách hàng doanh nghiệp** |
| `POST /api/v1/requests` | *không tồn tại* | yêu cầu tư vấn / gọi lại của khách doanh nghiệp |

**Hai mô hình xác thực khác hẳn nhau, trùng khít đường dẫn.** Không phải nhầm lẫn: mỗi bên chọn
`sessions` một cách hợp lý trong hệ thống của mình, và hai hệ thống ấy chưa bao giờ đứng cạnh
nhau để ai nhận ra.

**Đây là điều (1), và là điều duy nhất trong ADR này không suy ra được từ mã trong kho này.**
Bộ sinh hợp đồng của ViGov đọc từ khai báo route Go ở đây, nên nó không thể biết bên kia có gì.

## Quyết định — ĐÓNG LỰA CHỌN, không đổi tên

**Bề mặt ViGov và bề mặt thương mại không bao giờ được đặt sau cùng một tên miền.**

Không tuyến nào bị đổi tên. `sessions` đã chốt ở ADR 0020, `citizen-app/src/features/dang-nhap/
hop-dong.ts` là tệp duy nhất giữ nó, và Mini App đang chờ nộp Zalo. Đổi một hợp đồng đã chốt để
phòng một quyết định chưa ai ra là trả tiền hôm nay cho việc chưa xảy ra.

Tiền tố trùng là **triệu chứng** nói cho ta biết hai bề mặt phải tách ở tầng tên miền — **không
phải nguyên nhân** cần đi sửa.

## Vì sao ghép chung tên miền là hố của LUẬT 1, không phải bất tiện định tuyến

Đây là điều (3), và là lý do quyết định này không phải chuyện thẩm mỹ.

Luật 1 bất biến 3: **xã được suy ra từ `Host` ở rìa NGOÀI CÙNG, trước mọi handler; không phân
giải được thì 404.** Backend thương mại **không có khái niệm xã nào cả** — `internal/httpapi/
api.go` của nó không nhắc `tenant` một lần nào (đo 22/09/2026).

Ghép chung một tên miền buộc phải chọn một trong hai, và cả hai đều hỏng:

| Lối ra | Vỡ ở đâu |
|---|---|
| Cho tuyến thương mại **đi vòng** qua bước suy xã | Đúng **luật 1, cấm #1**: một ngoại lệ trên đường cô lập — và nó nằm ngay trên đường đi của **mọi** xã, không phải ở một góc |
| **Gán** cho tuyến thương mại một xã vô nghĩa | Một bề mặt **không thuộc xã nào** bỗng trông như đã được phân xã trong **mọi vết kiểm toán**. Về sau không phân biệt được vết nào là của một xã thật |

Cái giá của việc đóng lựa chọn này hôm nay bằng không. Cái giá của việc phát hiện nó sau khi một
xã đã chạy thật là sửa đường cô lập trên dữ liệu đang chạy.

## Hệ quả cho hai bộ sinh — điều (2)

`kb/20-contracts/openapi.json` và `tools/ingress` **CỐ Ý không bao** `POST /api/v1/sessions` của
backend thương mại lẫn `POST /api/v1/requests`. Cả hai bộ sinh đọc **từ khai báo route Go trong
kho này**, nên chúng không thể phát minh ra hai tuyến ấy.

**Rủi ro thật nằm ở chiều ngược lại**, và nó là lý do ADR này tồn tại: một phiên sau thấy
`citizen-app` gọi một đường **không có trong `openapi.json`** rồi *"sửa cho khớp"*. Đó là lần hợp
nhất sai, và nó sẽ trông hoàn toàn hợp lý — đúng hình dạng mà ADR 0032 đã cảnh báo một lần.

Quy ước tiền tố `my-` cho rìa công dân (`kb/00-foundation/ubiquitous-language.md`) **không áp**
cho nửa thương mại: `requests` cố ý không mang `my-` và cố ý không mượn từ vựng kênh công dân.
Mượn từ vựng của hệ thống kia là bước đầu để người sau tưởng hai thứ là một.

## ĐIỀU KIỆN DỪNG — hỏi người dùng, đừng tự quyết

1. **Một đề nghị đặt hai bề mặt sau cùng một tên miền**, kể cả "tạm thời, để thử". Không có hình
   dạng tạm thời nào cho một ngoại lệ trên đường cô lập
2. **Một tuyến thứ ba trùng đường dẫn giữa hai kho.** Hai lần là trùng hợp, ba lần là không gian
   tên đang thực sự va nhau và câu hỏi đổi tên phải được hỏi lại
3. **Backend thương mại mọc ra khái niệm xã.** Lúc ấy bảng so sánh ở trên hết đúng, và ranh giới
   giữa hai hệ thống phải được vẽ lại từ đầu

→ ADR 0020 (hợp đồng tuyến phát hành phiên) · ADR 0031 · ADR 0032 (phép thử: khoá nào ký nó)
→ Luật 1 bất biến 3, cấm #1: `.claude/rules/critical/1-tenant-isolation.md`
