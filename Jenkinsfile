// ViGov — cổng kiểm TOÀN KHO. Tệp này KHÔNG đóng ảnh nào.
//
// ─────────────────────────────────────────────────────────────────────────────────────────
// VÌ SAO TỆP NÀY VẪN TỒN TẠI KHI MỖI DỊCH VỤ ĐÃ CÓ PIPELINE RIÊNG
//
// Mỗi dịch vụ tự quyết build cái gì và khi nào: `service-<tên>/Jenkinsfile`. Web cũng vậy:
// `web-admin/Jenkinsfile`. Một thay đổi ở `comms` không còn bắt `finance` sinh ảnh
// mới rồi triển khai lại.
//
// Nhưng có một nhóm bất biến KHÔNG THUỘC VỀ DỊCH VỤ NÀO CẢ:
//
//   · 7 bất biến cấu trúc của bộ não (.claude/) — luật nào có hook nào, agent nào có thật
//   · 90 ca tự kiểm hook — bằng chứng lớp thực thi còn chạy
//   · bất biến an toàn của 9 Dockerfile + 9 Jenkinsfile (tools/check_build.py)
//   · `buf lint` trên proto/ — hợp đồng GIỮA các dịch vụ, không của bên nào
//   · `gofmt`, `go vet`, `go build`, `go test -race` trên TOÀN kho, gồm core/ và tools/
//
// Nhét chúng vào tám pipeline dịch vụ nghĩa là chạy tám lần một việc. Và tệ hơn: nếu chúng
// CHỈ chạy ở pipeline dịch vụ, thì một thay đổi chỉ đụng `.claude/` hoặc `tools/` sẽ không
// kích hoạt pipeline nào — bộ não trôi mà không có gì đỏ.
//
// Nên phân chia theo QUYỀN SỞ HỮU chứ không theo tiện tay:
//   dịch vụ sở hữu ảnh của nó  ·  kho sở hữu bất biến của kho.
// ─────────────────────────────────────────────────────────────────────────────────────────
//
// MÁY CHỦ BUILD CẦN: go (1.26+), buf, node (22+), python3, và một trình biên dịch C —
// `go test -race` cần cgo. KHÔNG cần docker: tệp này không đóng ảnh.

pipeline {
  agent any

  options {
    timestamps()
    timeout(time: 40, unit: 'MINUTES')
    buildDiscarder(logRotator(numToKeepStr: '30'))
    disableConcurrentBuilds()
  }

  stages {

    stage('Kiểm công cụ') {
      steps {
        // `make` và `gcc` vào danh sách sau lượt chạy thật đầu tiên (2026-09-21).
        //   · `make`: mọi stage dưới đây gọi `make proto` / `make check`. Thiếu nó thì lỗi là
        //     "make: not found" ở giữa một stage, trông như mục tiêu hỏng chứ không như máy
        //     chủ thiếu công cụ.
        //   · `gcc`: `make check` chạy `go test -race`, mà `-race` cần cgo. Thiếu nó thì lỗi
        //     là "race is only supported ... with cgo" — trông như mã hỏng.
        sh '''
          set -eu
          thieu=""
          for cc in go buf node npm python3 make gcc; do
            command -v "$cc" >/dev/null 2>&1 || thieu="$thieu $cc"
          done
          if [ -n "$thieu" ]; then
            echo ""
            echo "MÁY CHỦ BUILD THIẾU:$thieu"
            echo "Dừng ở đây thay vì đổ giữa chừng với một lỗi không đọc được."
            echo ""
            exit 1
          fi
          go version; buf --version; node --version
        '''
      }
    }

    stage('Sinh mã từ .proto') {
      steps {
        // gen/ nằm trong .gitignore và mã nguồn import nó; buf.gen.yaml dùng plugin TỪ XA
        // nên bước này cần mạng ra buf.build — lý do nó ở máy chủ build chứ không ở trong
        // Dockerfile, nơi mạng thường bị chặn và cũng nên bị chặn.
        sh 'make proto'
        sh 'test -d core/gen || { echo "make proto chạy xong mà không có core/gen/"; exit 1; }'
      }
    }

    stage('Phụ thuộc web') {
      steps {
        // Chạy TRƯỚC cổng kiểm, có chủ ý. Mục `web` của `make check` BỎ QUA phần kiểm
        // TypeScript khi không có node_modules — nó báo to, nhưng vẫn là bỏ qua, và một cổng
        // kiểm có thể bỏ qua chính là thứ dự án này liên tục gặp: thứ trông như biện pháp mà
        // không phải biện pháp. Cài trước thì nó không còn đường bỏ qua.
        dir('web-admin') {
          sh 'npm ci'
        }
      }
    }

    stage('Cổng kiểm') {
      steps {
        // MỘT ĐỊNH NGHĨA DUY NHẤT của "đã kiểm": `make check`. Pipeline cố ý KHÔNG liệt kê
        // lại từng bước — hai danh sách sẽ lệch, và bản lỏng hơn là bản chạy trên CI trong
        // khi mọi người tin vào bản chặt hơn (luật 9).
        sh 'make check'
      }
    }
  }

  post {
    success {
      echo 'Bất biến toàn kho: xanh. Ảnh do pipeline của TỪNG dịch vụ đóng — xem ' +
           'service-<tên>/Jenkinsfile và web-admin/Jenkinsfile.'
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────────────────
// QUY ƯỚC THẺ ẢNH — nêu MỘT LẦN ở đây, và `tools/check_build.py` giữ cho chín pipeline kia
// không ai phá.
//
// Mọi ảnh chỉ mang đúng một thẻ: commit đã sinh ra nó. Không `latest`, không thẻ di động nào.
//
// Một thẻ di động (`latest`, `main`, `prod`) nghĩa là hai pod cùng một manifest có thể đang
// chạy hai đoạn mã khác nhau, tuỳ lúc nào chúng kéo ảnh. Trong một hệ thống lưu hồ sơ hành
// chính có giá trị pháp lý, câu "bản nào đang chạy lúc đó" phải trả lời được bằng một mã
// commit, không phải bằng suy đoán từ thời điểm kéo ảnh.
//
// Vì mỗi dịch vụ nay dựng độc lập, HAI DỊCH VỤ Ở HAI COMMIT KHÁC NHAU LÀ BÌNH THƯỜNG — đó
// chính là điểm của việc tách. Nhưng vì tám dịch vụ dùng chung `core/`, một dịch vụ không
// được dựng lại là một dịch vụ đang chạy `core/` cũ. Đó là lý do danh sách đường kích hoạt
// trong mỗi pipeline dịch vụ PHẢI có `core/**`, `proto/**`, `go.work`.
//
// Cái giá là manifest phải sửa thẻ mỗi lần triển khai. Đó chính là điều mong muốn: một lần
// triển khai phải là một thay đổi ai đó nhìn thấy được.
// ─────────────────────────────────────────────────────────────────────────────────────────
//
// VÌ SAO KHÔNG GỌI PLUGIN DOCKER PIPELINE — nêu MỘT LẦN ở đây; tám pipeline đóng ảnh
// (`web-admin` + bảy dịch vụ) chỉ trỏ tới mục này, không chép nó xuống.
//
// Tám tệp ấy từng viết `docker.withRegistry { docker.build(...).push() }`. Hai lệnh đó thuộc
// plugin **Docker Pipeline**, và MÁY CHỦ JENKINS THẬT KHÔNG CÓ NÓ. Phát hiện ngày 2026-09-21
// trên kho `vihat-miniapp` (dùng chung máy chủ này), và triệu chứng đáng nhớ vì nó không
// giống một lỗi thiếu plugin:
//
//     Invalid agent type "docker" specified. Must be one of [any, label, none]
//
// Ba tên ấy là những loại agent Jenkins core tự biết. Danh sách chỉ có ba tên nghĩa là KHÔNG
// plugin nào đăng ký thêm loại nào — và lỗi xảy ra lúc BIÊN DỊCH Jenkinsfile, trước khi lượt
// build kịp chạy một dòng, nên trong log không có stage nào để lần theo.
//
// Máy chủ Jenkins dùng chung với dự án khác, nên "cài thêm plugin" không phải quyết định của
// kho này. Đổi lại, tám tệp tự làm ba việc plugin vốn làm hộ — mỗi việc một cái bẫy riêng:
//
//   1. `DOCKER_CONFIG` RIÊNG TỪNG LƯỢT BUILD. `docker login` mặc định ghi vào
//      `~/.docker/config.json` của user `jenkins` — MỘT tệp dùng chung cho mọi job trên máy.
//      Không tách thì lượt đăng xuất của job này đá văng phiên đăng nhập của job dự án khác
//      đang đẩy ảnh giữa chừng, và triệu chứng bên kia là một lỗi 401 không lý do.
//
//   2. `--password-stdin` VÀ `set +x`. Bước `sh` của Jenkins chạy `sh -xe`, mà `-x` in ra ĐỐI
//      SỐ ĐÃ KHAI TRIỂN: `echo "$REG_PASS"` sẽ hiện nguyên mật khẩu registry trong log. Bộ lọc
//      che của Jenkins bắt được, nhưng một bí mật đã ra tới chỗ cần bộ lọc thì chỉ còn đúng
//      một lớp giữa nó và log — luật 8, và một khoá registry rò là rò cho MỌI xã.
//      Cùng lý do: script để trong nháy ĐƠN, giá trị vào bằng biến môi trường. Nội suy Groovy
//      một bí mật vào chuỗi script là đưa nó ra ngoài tầm che.
//
//   3. `docker image rm` SAU KHI ĐẨY. Máy dùng chung, mỗi commit một thẻ mới; không dọn thì
//      đĩa của người khác đầy vì kho này. Chỉ bỏ THẺ — các tầng nằm lại trong cache.
//
// Và `post { always }` xoá `config.json`: một lượt hỏng GIỮA login và push là đúng lượt để
// lại token đăng nhập registry nằm trên đĩa máy chủ.
//
// `withCredentials` thì vẫn dùng — nó thuộc `credentials-binding`, plugin có trong mọi bản
// cài Jenkins tiêu chuẩn. Không có nó thì không còn đường nào lấy bí mật mà không phạm luật 8.
// ─────────────────────────────────────────────────────────────────────────────────────────
