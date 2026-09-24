---
id: domain-boundaries
tier: T0
source: CURATED
owner: architecture
derived_from_commit: 184869f
expires: null
owns_facts:
  - "ranh giới giữa các service và lý do cắt ở đó"
  - "service nào được gọi thẳng service nào"
  - "quy tắc đặt tên service"
  - "dạng tên sự kiện: petitions.received.v1"
---

# Miền nghiệp vụ và ranh giới service

**Tệp này trả lời câu VÌ SAO.** Danh sách service hiện có, cổng, phụ thuộc thực tế nằm ở
`kb/30-indexes/services.json` — **tầng GENERATED**, luôn đúng, không chép sang đây.

## Nguyên tắc cắt ranh giới

| # | Nguyên tắc | Vì sao |
|---|---|---|
| 1 | Cắt theo **miền nghiệp vụ hành chính**, không theo tầng kỹ thuật | Một thay đổi nghiệp vụ nên chạm đúng một service |
| 2 | Service sở hữu **dữ liệu** của miền mình, không chỉ sở hữu mã | Sở hữu mã mà chung CSDL là monolith phân tán |
| 3 | Ranh giới đi theo **nhịp thay đổi**, không theo kích thước | Thứ đổi cùng nhau thì ở cùng nhau |
| 4 | Ranh giới đi theo **ranh giới trách nhiệm hành chính** | Văn thư, một cửa, thanh tra là các bộ phận khác nhau ngoài đời |

## Vì sao KHÔNG cắt nhỏ hơn

Cấp xã có quy mô nhỏ: vài chục cán bộ, vài nghìn hồ sơ một năm. Cắt quá nhỏ thì chi phí
vận hành (triển khai, giám sát, truy vết) vượt lợi ích, và mỗi luồng nghiệp vụ phải đi qua
nhiều lời gọi mạng cho một việc mà một giao dịch làm được.

**Phép thử trước khi tách service mới:** luồng nghiệp vụ này có cần **nhất quán mạnh** với
service hiện có không? Có → **đừng tách**, vì tách là chấp nhận nhất quán cuối cùng và phải
viết bù trừ. Không → tách được.

## Đường đi hợp lệ giữa các service

| Từ → tới | Cách | Khi nào |
|---|---|---|
| bất kỳ → bất kỳ | **gRPC** qua hợp đồng | Cần dữ liệu ngay, chấp nhận phụ thuộc lúc chạy |
| bất kỳ → bất kỳ | **Sự kiện** | Chấp nhận trễ; bên nhận giữ bản sao đọc của riêng mình |
| bất kỳ → CSDL của người khác | **KHÔNG BAO GIỜ** | — |

Lời gọi gRPC mang xã đi ra sao, vì sao mặc định theo lô, và chuyện gì xảy ra khi `platform`
không tới được: `kb/10-decisions/0012-grpc-boundary-contract.md`.

Thêm một đường gọi mới là thêm một cạnh vào đồ thị phụ thuộc. Trước khi thêm, tra
`kb/30-indexes/dependencies.json` xem có tạo vòng không — vòng phụ thuộc đồng bộ là chỗ hệ
thống sẽ kẹt khi một service chậm.

## Đặt tên

- **Định danh máy đọc dùng tiếng Anh** — tên service, proto package, import path, tên trường,
  và **đoạn đường dẫn URL**. Văn xuôi tài liệu (`kb/*.md`) giữ tiếng Việt.
- Service theo **miền nghiệp vụ**, không theo màn hình: `documents`, `petitions`,
  `finance`. Không có `admin-service` — "admin" là giao diện, không phải miền.
- Sự kiện: `<miền số nhiều>.<việc đã xảy ra, tiếng Anh>.<phiên bản>` — `petitions.received.v1`
- Tên ở **thì quá khứ**: sự kiện mô tả việc **đã xảy ra**, không phải lệnh
- **Ngoại lệ có chủ đích:** khi thuật ngữ hành chính không có bản dịch đúng, giữ nguyên khái
  niệm và chú thích. `PhanAnh` / `KhieuNai` / `ToCao` là **ba thứ khác nhau về pháp lý**;
  gộp cả ba thành `complaint` là làm mất phân biệt đó. → `kb/00-foundation/ubiquitous-language.md`

Ranh giới ngôn ngữ đầy đủ — vì sao đường dẫn tiếng Anh nhưng **giá trị enum giữ tiếng Việt**,
và vì sao tên sự kiện là dạng trên: `kb/10-decisions/0011-contract-surface-language.md`.

## Bảy service

Cắt theo **bộ phận chịu trách nhiệm trong một UBND xã** — xem lý do đầy đủ ở
`kb/10-decisions/0001-service-decomposition.md`.

`dossiers` (một cửa) đã **gỡ khỏi kho ngày 20/09/2026**: khách chốt hồ sơ một cửa không
thuộc phạm vi hợp đồng. Bằng chứng khảo sát và đường quay lại ở ADR 0001 §Bổ sung 2026-09-20.

| Service | Bộ phận ngoài đời |
|---|---|
| `platform` | Nền tảng — nhà cung cấp vận hành, **chỉ siêu dữ liệu** |
| `identity` | Tổ chức – cán bộ, định danh công dân toàn nền tảng, và **đơn vị dân cư** (thôn / tổ dân phố) |
| `documents` | Văn thư — văn bản đến/đi, đơn thư (ADR 0039) |
| `petitions` | Tiếp dân — phản ánh và nhiệm vụ phát sinh |
| `finance` | Tài chính – kế toán — dự toán, giải ngân |
| `comms` | Thông tin – truyền thông — tin bài, truyền thanh, bản đồ, thông báo |
| `reporting` | Read model — **không sở hữu dữ liệu gốc nào** |

**Hai thứ cố ý KHÔNG phải service:** nhật ký thao tác (`core/audit`) và lưu trữ tệp
(`core/storage`). Lý do ở ADR 0001.

**`identity` giữ thêm đơn vị dân cư — mở rộng công khai, không phải lệ.** Thôn / tổ dân phố là
đơn vị **trong bộ máy xã**, đổi cùng nhịp với sơ đồ tổ chức, nên nằm cạnh nó. Kèm theo là hai
danh mục treo vào bộ máy ấy: `Loại đơn vị dân cư` và `Khối nhiệm vụ`. Ghi ở đây vì một ranh
giới nới ra trong im lặng là ranh giới không còn ai kiểm được — vì sao và cái giá phải trả:
`kb/10-decisions/0024-so-huu-danh-muc-tham-chieu.md`.

## Danh mục tham chiếu thuộc về ai

Màn hình `Cấu hình → Danh mục` gom mười nhóm vào một chỗ vì **người quản trị** muốn sửa chúng ở
một chỗ. Đó là sự thật về giao diện. Ở tầng dữ liệu, mỗi nhóm đi theo miền dùng nó, và service
của miền ấy sở hữu — chủ sở hữu từng nhóm, ba nhóm còn đang chờ khách, và phép thử để biết một
thứ có phải danh mục hay không: **ADR 0024**.

→ Thuật ngữ nghiệp vụ và ánh xạ tên tài nguyên URL: `kb/00-foundation/ubiquitous-language.md`
→ Ai sở hữu thực thể nào: `kb/30-indexes/data-ownership.json` (GENERATED)
→ Luật 2: `.claude/rules/critical/2-service-boundary.md`
