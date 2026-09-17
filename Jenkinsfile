// ViGov — pipeline dựng và đóng ảnh.
//
// PHẠM VI DỪNG Ở ẢNH. Pipeline này sinh mã, chạy cổng kiểm, đóng chín ảnh và đẩy lên
// registry. Nó KHÔNG triển khai, không `kubectl apply`, không đụng cụm k8s — phần đó do đội
// devops phụ trách. Thứ nó bàn giao là một thẻ ảnh BẤT BIẾN.
//
// ─────────────────────────────────────────────────────────────────────────────────────────
// MÁY CHỦ BUILD CẦN CÓ: go (1.26+), buf, node (22+), python3, docker, và một trình biên dịch
// C — `go test -race` cần cgo. Giai đoạn đầu tiên kiểm từng thứ và nói tên cái thiếu, thay
// vì để pipeline đổ ở giữa chừng với một thông báo không đọc được.
// ─────────────────────────────────────────────────────────────────────────────────────────

// Khai ngoài `pipeline {}` để hai giai đoạn dùng CHUNG một định nghĩa "nhánh chính".
//
// `when { branch 'main' }` chỉ hoạt động trong job MULTIBRANCH. Với một job pipeline thường,
// `BRANCH_NAME` không được gán, điều kiện luôn sai, và hậu quả là pipeline chạy xanh từ đầu
// đến cuối mà KHÔNG đóng một ảnh nào — một lượt build thành công không sinh ra gì, kiểu hỏng
// mất nhiều thời gian nhất để nhận ra vì không có gì đỏ cả.
boolean laNhanhChinh() {
  String b = env.BRANCH_NAME ?: env.GIT_BRANCH ?: ''
  // Rỗng = job pipeline thường. Kho này cam kết commit thẳng lên main (CLAUDE.md, mục GIT),
  // nên một job thường ở đây luôn là main.
  return b == '' || b == 'main' || b.endsWith('/main')
}

pipeline {
  agent any

  options {
    timestamps()
    timeout(time: 45, unit: 'MINUTES')
    buildDiscarder(logRotator(numToKeepStr: '30'))
    // Hai lượt cùng lúc trên một máy chủ sẽ giẫm lên gen/ và node_modules của nhau.
    disableConcurrentBuilds()
  }

  parameters {
    // Harbor. KHÔNG ghi cứng trong tệp này vì nó khác nhau giữa các môi trường, và một địa
    // chỉ ghi cứng là địa chỉ sẽ có người sửa vội vào một buổi tối.
    string(name: 'REGISTRY', defaultValue: 'registry.vihat.vn',
           description: 'Máy chủ Harbor, không kèm https://')
    string(name: 'PROJECT', defaultValue: 'vigov',
           description: 'Tên project trong Harbor — ảnh sẽ là <REGISTRY>/<PROJECT>/vigov-<tên>')
    string(name: 'REGISTRY_CRED', defaultValue: 'harbor-vigov',
           description: 'ID của credentials (username/password) trong Jenkins. CHỈ LÀ ID, ' +
                        'không bao giờ là giá trị thật — luật 8, bất biến 1.')
  }

  environment {
    // BuildKit khai tường minh chứ không trông vào mặc định của máy chủ: hai Dockerfile đều
    // dùng `RUN --mount=type=cache`, và nếu BuildKit tắt thì chúng đổ ở một lỗi cú pháp khó
    // hiểu thay vì chạy chậm.
    DOCKER_BUILDKIT = '1'
    // TAG (thẻ ảnh = commit) KHÔNG đặt ở đây mà đặt trong giai đoạn đầu bằng `git rev-parse`.
    // `env.GIT_COMMIT` do plugin git gán, và thời điểm gán phụ thuộc kiểu job — đọc nó trong
    // khối environment có lúc ra rỗng, và một thẻ ảnh rỗng thì không hỏng ngay: nó đẩy lên
    // registry một thẻ vô nghĩa. Hỏi thẳng git thì lúc nào cũng đúng.
    // Xem phần "VÌ SAO KHÔNG CÓ THẺ `latest`" ở cuối tệp.
  }

  stages {

    stage('Kiểm công cụ') {
      steps {
        sh '''
          set -eu
          thieu=""
          for cc in go buf node npm python3 docker; do
            command -v "$cc" >/dev/null 2>&1 || thieu="$thieu $cc"
          done
          if [ -n "$thieu" ]; then
            echo ""
            echo "MÁY CHỦ BUILD THIẾU:$thieu"
            echo "Pipeline dừng ở đây thay vì đổ giữa chừng với một lỗi không đọc được."
            echo ""
            exit 1
          fi
          echo "go     : $(go version)"
          echo "buf    : $(buf --version)"
          echo "node   : $(node --version)"
          echo "docker : $(docker version --format '{{.Server.Version}}')"
        '''
        script {
          env.TAG = sh(script: 'git rev-parse --short=12 HEAD', returnStdout: true).trim()
          if (!env.TAG) {
            error('Không lấy được commit hiện tại — không có thẻ ảnh nào để đặt.')
          }
          echo "commit : ${env.TAG}"

          // DANH SÁCH DỊCH VỤ ĐỌC TỪ ĐĨA, KHÔNG CHÉP TAY.
          //
          // Một danh sách gõ tay trong tệp này là một danh sách sẽ lệch với kho mã. Hậu quả
          // không phải một lỗi đỏ: dịch vụ thứ chín thêm vào mà quên sửa ở đây sẽ đơn giản
          // KHÔNG có ảnh, pipeline vẫn xanh, và người ta chỉ phát hiện lúc triển khai khi
          // thiếu mất một thành phần (luật 9, bất biến 1).
          env.DICH_VU = sh(
            script: 'ls -d services/*/cmd/server | cut -d/ -f2 | sort',
            returnStdout: true).trim()
          if (!env.DICH_VU) {
            error('Không thấy dịch vụ nào dưới services/*/cmd/server.')
          }
          echo "dịch vụ: ${env.DICH_VU.split(/\s+/).join(' ')}"
        }
      }
    }

    stage('Sinh mã từ .proto') {
      steps {
        // gen/ NẰM TRONG .gitignore và mã nguồn import nó, nên một bản checkout sạch không
        // build được cho tới khi bước này chạy. buf.gen.yaml dùng plugin TỪ XA, tức bước này
        // cần mạng ra buf.build — lý do nó ở đây chứ không ở trong Dockerfile, nơi mạng
        // thường bị chặn và cũng nên bị chặn.
        sh 'make proto'
        sh 'test -d gen || { echo "make proto chạy xong mà không có gen/"; exit 1; }'
      }
    }

    stage('Phụ thuộc web') {
      steps {
        // Chạy TRƯỚC cổng kiểm, có chủ ý. Mục `web` của `make check` sẽ BỎ QUA phần kiểm
        // TypeScript khi không có node_modules — nó báo to, nhưng vẫn là bỏ qua, và một cổng
        // kiểm có thể bỏ qua chính là thứ dự án này liên tục gặp: thứ trông như biện pháp mà
        // không phải biện pháp. Cài trước thì nó không còn đường bỏ qua.
        dir('apps/commune-admin') {
          sh 'npm ci'
        }
      }
    }

    stage('Cổng kiểm') {
      steps {
        // MỘT ĐỊNH NGHĨA DUY NHẤT của "đã kiểm": `make check`. Pipeline cố ý KHÔNG liệt kê
        // lại từng bước ở đây — hai danh sách sẽ lệch, và bản lỏng hơn là bản sẽ chạy trên CI
        // trong khi mọi người tin vào bản chặt hơn (luật 9).
        //
        // Gồm: 7 bất biến bộ não · 90 ca tự kiểm hook · gofmt · go vet · buf lint · go build ·
        // go test -race · typecheck + test + đối chiếu hợp đồng của web.
        sh 'make check'
      }
    }

    stage('Đóng ảnh') {
      when { expression { laNhanhChinh() } }
      steps {
        script {
          def dichVu = env.DICH_VU.split(/\s+/)

          docker.withRegistry("https://${params.REGISTRY}", params.REGISTRY_CRED) {

            // TUẦN TỰ, KHÔNG SONG SONG, và đây là lựa chọn chứ không phải thiếu sót. Chín
            // lượt biên dịch Go cùng lúc trên một máy chủ chỉ tranh nhau CPU chứ không nhanh
            // hơn, trong khi chạy tuần tự thì tầng `go mod download` của ảnh đầu tiên được
            // tám ảnh sau dùng lại — phần lâu nhất chỉ trả giá một lần.
            for (svc in dichVu) {
              def ten = "${params.REGISTRY}/${params.PROJECT}/vigov-${svc}"
              def anh = docker.build(
                "${ten}:${env.TAG}",
                "--build-arg SERVICE=${svc} --build-arg VERSION=${env.TAG} " +
                "-f build/go.Dockerfile .")
              anh.push()
              echo "đã đẩy ${ten}:${env.TAG}"
            }

            def tenWeb = "${params.REGISTRY}/${params.PROJECT}/vigov-commune-admin"
            def anhWeb = docker.build(
              "${tenWeb}:${env.TAG}",
              "--build-arg VERSION=${env.TAG} -f build/web.Dockerfile .")
            anhWeb.push()
            echo "đã đẩy ${tenWeb}:${env.TAG}"
          }
        }
      }
    }

    stage('Bàn giao cho devops') {
      when { expression { laNhanhChinh() } }
      steps {
        script {
          def moi = (env.DICH_VU.split(/\s+/).collect { "vigov-${it}" }
                     + ['vigov-commune-admin'])
          echo """
─────────────────────────────────────────────────────────────────────
ẢNH ĐÃ SẴN SÀNG — commit ${env.TAG}

  ${params.REGISTRY}/${params.PROJECT}/<tên>:${env.TAG}

  ${moi.join('\n  ')}

Ghi chú cho người viết manifest k8s:
  · Probe dùng httpGet /healthz cổng 8080. Ảnh Go KHÔNG có shell, nên exec probe
    sẽ không chạy được.
  · Cổng 9090 là gRPC giữa các dịch vụ: giữ trong cụm, đừng đưa ra Ingress.
  · Ảnh chạy không phải root (Go: UID 65532 · web: UID 1000). Đặt runAsNonRoot: true.
  · MIGRATION TỰ CHẠY LÚC KHỞI ĐỘNG, trong một pg_advisory_lock, mỗi tệp một giao dịch,
    và dịch vụ TỪ CHỐI khởi động nếu migration lỗi. Không cần Job riêng. Nhưng vì thế,
    đừng đặt initialDelaySeconds của liveness quá ngắn ở lần triển khai đầu.
  · Mọi giá trị RIÊNG CỦA TỪNG XÃ được đọc lúc chạy từ cơ sở dữ liệu, không phải từ biến
    môi trường và không nằm trong ảnh. Cùng một ảnh phục vụ mọi xã.
  · Biến môi trường cần khai: xem .env.example. Chúng là hằng số của NỀN TẢNG.
─────────────────────────────────────────────────────────────────────
"""
        }
      }
    }
  }

  post {
    always {
      // Ảnh đã nằm trên registry rồi; bản sao cục bộ chỉ còn là đĩa bị ăn dần cho tới khi
      // máy chủ build đầy — và một máy chủ build đầy đĩa đổ ở những chỗ chẳng liên quan gì.
      // Chỉ gỡ đúng những thẻ lượt này tạo ra, không đụng cache tầng.
      sh '''
        set +e
        # Thẻ rỗng thì KHÔNG dọn gì cả. Một lượt đổ trước khi đặt được TAG sẽ để biến này
        # rỗng, và một mẫu tìm kiếm rỗng khớp nhiều hơn hẳn thứ định khớp.
        [ -n "${TAG:-}" ] || exit 0
        for t in $(docker image ls --format '{{.Repository}}:{{.Tag}}' 2>/dev/null \
                   | grep ":${TAG}\\$"); do
          docker image rm "$t" >/dev/null 2>&1
        done
        exit 0
      '''
    }
  }
}

// ─────────────────────────────────────────────────────────────────────────────────────────
// VÌ SAO KHÔNG CÓ THẺ `latest`, VÀ KHÔNG CÓ THẺ NÀO DI ĐỘNG
//
// Mọi ảnh chỉ mang đúng một thẻ: commit đã sinh ra nó. Một thẻ di động (`latest`, `main`,
// `prod`) nghĩa là hai pod cùng một manifest có thể đang chạy hai đoạn mã khác nhau, tuỳ lúc
// nào chúng kéo ảnh. Trong một hệ thống lưu hồ sơ hành chính có giá trị pháp lý, câu hỏi
// "bản nào đang chạy lúc đó" phải trả lời được bằng một mã commit, chứ không phải bằng suy
// đoán từ thời điểm kéo ảnh.
//
// Cái giá là manifest phải sửa thẻ mỗi lần triển khai. Đó chính là điều mong muốn: một lần
// triển khai phải là một thay đổi ai đó nhìn thấy được.
// ─────────────────────────────────────────────────────────────────────────────────────────
