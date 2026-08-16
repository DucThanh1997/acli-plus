# E2E test — `acli-plus jira`

Kiểm chứng lệnh Jira trên site thật. Có 2 cách dùng:

- **Chạy tất cả bằng 1 lệnh** → [`scripts/e2e-jira.sh`](e2e-jira.sh)
- **Chạy tay từng case** → bảng bên dưới, mỗi case là 1 lệnh copy-paste được

---

## 0. Chuẩn bị

### 0.1. Cài binary — làm 1 lần duy nhất

Mọi lệnh trong tài liệu này đã viết sẵn giá trị thật (`acli-plus`, project `KAN`,
type `Task`). **Không cần export biến gì cả** — copy dòng nào chạy dòng đó.

Điều kiện duy nhất: `acli-plus` trên PATH phải là bản đã có lệnh `jira`. Kiểm tra:

```bash
acli-plus jira project list
```

Ra bảng project là xong, bỏ qua phần còn lại của mục này — nó vừa xác nhận binary
đúng bản, vừa xác nhận credential còn dùng được.

Nếu báo `unknown command "jira"` thì bản trên PATH là bản cũ, cài lại:

```bash
make release && make install
```

⚠️ `make install` một mình **không đủ**: [`install.sh`](../install.sh) ưu tiên
binary dựng sẵn trong `dist/` và không hề cảnh báo khi thứ trong đó cũ hơn
source — nó sẽ lặng lẽ cài lại đúng bản cũ, và bạn vẫn nhận `unknown command
"jira"` dù vừa mới "cài". `make release` build lại `dist/` trước, nên phải chạy
cả hai.

Không muốn cài đè lên PATH thì `make build`, rồi thay `acli-plus` trong mọi lệnh
bằng đường dẫn **tuyệt đối** tới `bin/acli-plus` — [mục
10](#10-cấu-hình-riêng-theo-repo-acli-plusyaml) có `cd` sang thư mục khác nên
đường dẫn tương đối `./bin/acli-plus` sẽ chết ở đó.

### 0.1b. Site hoặc project của bạn khác?

Doc viết theo site đã đăng ký sẵn (`thanh191997-acli.atlassian.net`). Khác thì
tìm-thay 3 thứ:

| Trong doc | Thay bằng |
|---|---|
| `KAN` | project key của bạn — xem [mục 0.3](#03-chọn-project-để-test) |
| `Task` | một type hợp lệ của project đó |
| `<key A>`, `<key B>` | key hai ticket do chính [mục 6](#6-sửa-workflow-người-phụ-trách) tạo ra |

### 0.2. Có cần setup lại không?

**Không cần.** Credential trong `~/.config/acli-plus/credentials.yaml` dùng chung
cho Confluence và Jira — cùng site, cùng account, cùng API token. Bạn đã đăng ký
`thanh191997-acli.atlassian.net` rồi nên các lệnh Jira chạy luôn được, đã kiểm
chứng thực tế.

Nếu muốn xác nhận token có quyền Jira thì chạy setup **bằng binary mới**:

```bash
acli-plus setup
```

Bản mới hỏi `Atlassian site URL` (bản cũ ghi `Confluence site URL`) và in thêm
dòng cuối:

```
Reachable on this site: Confluence, Jira
```

Thiếu chữ `Jira` nghĩa là token không có quyền Jira → tạo token mới tại
<https://id.atlassian.com/manage-profile/security/api-tokens>. Nhập lại đúng
site/email/token cũ là được, nó ghi đè đúng host chứ không tạo trùng.

### 0.2b. Config hai tầng — token chung, site riêng theo repo

Dễ nhầm nên nói rõ: **chỉ token là dùng chung**, còn chọn site vẫn là per-project.

| Tầng | Ở đâu | Chung hay riêng |
|---|---|---|
| Token (bí mật) | `~/.config/acli-plus/credentials.yaml`, key theo host | **Chung** — nhiều repo cùng site dùng chung 1 token, và token **không bao giờ** được ghi vào file trong repo (tránh commit lên git) |
| Site + default (không bí mật) | `acli-plus.yaml` trong repo, commit được | **Riêng từng repo** — repo A trỏ site client A, repo B trỏ client B |

Thứ tự chọn site:

1. host trong URL truyền vào (vd `.../browse/KAN-1`)
2. `--site`
3. biến môi trường `ACLI_PLUS_SITE`
4. `site:` trong `acli-plus.yaml` của thư mục hiện tại
5. site duy nhất đã đăng ký, nếu chỉ có đúng 1

Bước 5 tồn tại vì lệnh Jira không có URL để suy ra host — không có nó thì `jira
project list` cũng phải kèm `--site`. Khi đăng ký **nhiều site**, bước 5 không
chạy: acli-plus từ chối đoán và liệt kê các site để bạn chọn. Nên muốn cố định
site cho một repo thì cứ tạo `acli-plus.yaml` — nó thắng bước 5. Xem
[mục 10](#10-cấu-hình-riêng-theo-repo-acli-plusyaml).

### 0.3. Chọn project để test

Cần **1 project Jira mà bạn được phép tạo/xoá ticket** — dùng project rác, đừng
dùng project thật. Xem có gì:

```bash
acli-plus jira project list
```

Trên site của bạn hiện có đúng một project:

```
KEY  NAME       TYPE      STYLE     LEAD
KAN  acli-plus  software  next-gen  Trần Thành
```

Doc dùng luôn `KAN` nên các lệnh bên dưới copy là chạy. Project khác thì tìm-thay
`KAN` như ở [mục 0.1b](#01b-site-hoặc-project-của-bạn-khác).

Xem project có những type nào — sai type thì Jira **chỉ** báo `issuetype: Specify
a valid issue type`, không hề liệt kê cái đúng, nên cứ lấy danh sách trước:

```bash
acli-plus jira project view KAN --json | python3 -c 'import json,sys; [print(t["name"], "| subtask =", t["subtask"]) for t in json.load(sys.stdin)["issueTypes"]]'
```

Project `KAN` trả về:

```
Epic | subtask = False
Subtask | subtask = True
Task | subtask = False
Story | subtask = False
Bug | subtask = False
```

Cột `subtask` quyết định `--parent` dùng ở tầng nào — xem [case 4.6](#4-tạo-work-item).

### 0.4. Những gì không test được tự động

| Thứ | Vì sao |
|---|---|
| `workitem archive` / `unarchive` | Cần Jira **Premium/Enterprise**. Site thường sẽ báo `not available on your Jira plan` — đó là kết quả đúng. |
| `comment-visibility --type/--value` | Cần group hoặc project role có thật trên site. Script chỉ test nhánh `--public`. |
| `attachment-delete` | Cần file đính kèm sẵn (acli-plus không upload được, giống acli). |
| `project create/delete`, `field create/delete` | Cần quyền admin và để lại rác trên site. Xem [mục 9](#9-project-và-field-cần-quyền-admin). |
| `create -e/--editor` | Cần tương tác `$EDITOR`. Xem [case 4.7](#4-tạo-work-item). |

---

## 1. Chạy tất cả bằng 1 lệnh

### An toàn tuyệt đối — chỉ đọc và dry-run

Không ghi gì lên Jira. Chạy cái này trước.

```bash
./scripts/e2e-jira.sh --project KAN --read-only
```

### Đầy đủ — có ghi thật

Tạo ticket thật trong `--project` rồi **tự xoá sạch khi kết thúc** (kể cả khi
fail giữa chừng, nhờ `trap EXIT`). Nó hỏi xác nhận trước lần ghi đầu tiên.

```bash
./scripts/e2e-jira.sh --project KAN
```

Tuỳ chọn:

```bash
./scripts/e2e-jira.sh --project KAN --verbose        # in output từng lệnh
./scripts/e2e-jira.sh --project KAN --keep           # giữ lại ticket để xem
./scripts/e2e-jira.sh --project KAN --yes            # không hỏi (cho CI)
./scripts/e2e-jira.sh --project KAN --type Story     # type khác
./scripts/e2e-jira.sh --project KAN --site acme.atlassian.net
```

Kết quả cuối có dạng:

```
== Summary
30 passed, 0 failed, 1 skipped
```

Con số trên là lần chạy `--read-only` với `--project KAN`; chạy đầy đủ sẽ nhiều
hơn. **Đừng lấy con số làm chuẩn** — nó đổi theo site và theo số case được thêm
vào script. Chỉ `0 failed` mới là điều cần đúng.

Số case SKIP thay đổi theo site: board Kanban không có sprint (đưa `--project
SCR` thì case sprint hết SKIP), project rỗng thì không có gì để `edit`/`delete`
dry-run. SKIP là bình thường, chỉ FAIL mới đáng lo.

Exit code `0` khi mọi case pass, `1` nếu có case fail. Ticket script tạo ra đều
có label `acli-plus-e2e` — nếu script bị `kill -9` giữa chừng, dọn tay bằng:

```bash
acli-plus jira workitem delete --jql "project = KAN AND labels = acli-plus-e2e" --yes
```

---

## 2. Đọc — không ghi gì

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 2.1 | `acli-plus jira project list` | Bảng `KEY NAME TYPE STYLE LEAD`, có project của bạn |
| 2.2 | `acli-plus jira project list --json` | JSON thô từ API |
| 2.3 | `acli-plus jira project list --csv` | CSV có dòng header |
| 2.4 | `acli-plus jira project list --query team --type software` | Chỉ project khớp |
| 2.5 | `acli-plus jira project view KAN` | Khối `Key/Name/Id/Type/Style/Lead/URL` |
| 2.6 | `acli-plus jira field list` | Bảng `ID NAME TYPE CUSTOM`, có `summary`, `labels` |
| 2.7 | `acli-plus jira field list --custom` | Chỉ field custom (`customfield_*`) |
| 2.8 | `acli-plus jira field list --query point` | Lọc theo tên — đây là cách tìm `customfield_NNNNN` |
| 2.9 | `acli-plus jira board search --project KAN` | Bảng `ID NAME TYPE PROJECT`, hoặc `no results` nếu project không có board |
| 2.10 | `acli-plus jira board list-sprints --board 1` | Sprint của board (thay `1` bằng ID ở 2.9). **Board Kanban** → `this board does not use sprints (Kanban boards have none): board 1` — đây là kết quả **đúng**, sprint là khái niệm của Scrum. Board `KAN` của bạn là Kanban nên sẽ ra thông báo này |
| 2.11 | `acli-plus jira board list-sprints --board 2` | Board `2` (`SCR board`) là Scrum → ra `1  SCR Sprint 1  future`. Đây là ca đối chứng của 2.10 |
| 2.12 | `acli-plus jira board list-sprints --board 2 --state active` | Lọc theo state; sprint chưa start thì `future`, nên `--state active` trả rỗng cho tới khi bạn start sprint trên web |
| 2.12b | `acli-plus jira sprint list-workitems --sprint 1` | Ticket trong sprint — rỗng cho tới khi đưa ticket vào, xem [mục 2b](#2b-đưa-ticket-vào-sprint) |
| 2.13 | `acli-plus jira filter list` | Filter của bạn + favourite |
| 2.14 | `acli-plus jira filter list --favourites` | Chỉ favourite |
| 2.15 | `acli-plus jira filter search --name sprint` | Filter toàn site khớp tên |
| 2.16 | `acli-plus jira dashboard search` | Bảng dashboard |
| 2.17 | `acli-plus jira workitem link --list-types` | `NAME INWARD OUTWARD` — thường có `Blocks`, `Relates` |

### 2b. Đưa ticket vào sprint

Field `Sprint` của Jira bất đối xứng: **đọc** ra là mảng object, nhưng **ghi**
chỉ nhận một số trần. Nó khai schema là `array[json]`, nên `--field` — vốn suy ra
hình dạng từ schema — luôn gói lại thành mảng và bị Jira từ chối:

| Lệnh | Gửi lên | Kết quả |
|---|---|---|
| `--field "Sprint=1"` | `["1"]` | `400 The Sprint (id) must be a number` |
| `--field-json "Sprint=1"` | `1` | ✅ `updated SCR-1` |

Nên dùng `--field-json`, cờ này gửi nguyên văn JSON bạn viết, không suy diễn:

```bash
acli-plus jira workitem create -p SCR -t Task -s "e2e sprint"
acli-plus jira workitem edit <key vừa tạo> --field-json "Sprint=1"
acli-plus jira sprint list-workitems --sprint 1
```

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 2b.1 | `acli-plus jira workitem edit <key> --field-json "Sprint=1"` | `updated SCR-N` |
| 2b.2 | `acli-plus jira sprint list-workitems --sprint 1` | Ticket vừa gán đã xuất hiện |
| 2b.3 | `acli-plus jira workitem edit <key> --field "Sprint=1"` | `400 ... The Sprint (id) must be a number` — cho thấy vì sao cần `--field-json` |
| 2b.4 | `acli-plus jira workitem edit <key> --field-json 'Labels=["x","y"]'` | `updated SCR-N`, `view --fields labels` ra `x, y` — cờ này gửi mọi hình dạng JSON, không riêng số. Lưu ý `--field-json "Sprint=[1]"` **vẫn** bị Jira từ chối: cờ gửi đúng `[1]` như bạn viết, nhưng field Sprint chỉ nhận số trần |
| 2b.5 | `acli-plus jira workitem edit <key> --field-json "Sprint={oops"` | `field Sprint: --field-json expects valid JSON, got "{oops"` — chặn tại chỗ, không gọi API |

**Tìm kiếm** — chú ý API mới không trả tổng số, nên mặc định chỉ 1 trang:

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 2.18 | `acli-plus jira workitem search --jql "project = KAN"` | Bảng `KEY TYPE STATUS ASSIGNEE SUMMARY` |
| 2.19 | `acli-plus jira workitem search --jql "project = KAN" --limit 3` | Đúng 3 dòng dữ liệu |
| 2.20 | `acli-plus jira workitem search --jql "project = KAN" --paginate` | Đi hết mọi trang |
| 2.21 | `acli-plus jira workitem search --jql "assignee = currentUser()" --paginate` | Ticket của bạn |
| 2.22 | `acli-plus jira workitem search --jql "project = KAN" --fields summary,status` | Chỉ fetch 2 field |
| 2.23 | `acli-plus jira workitem search --jql "project = KAN" --csv > /tmp/out.csv` | File CSV mở được bằng Excel |
| 2.24 | `acli-plus jira workitem search --jql "project = KAN" --limit 1 --json` | JSON đầy đủ, có `"fields"` |

---

## 3. Dry-run — kiểm tra "nói mà không làm"

Đếm số ticket trước và sau, phải bằng nhau.

```bash
acli-plus jira workitem search --jql "project = KAN" --paginate | wc -l
```

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 3.1 | `acli-plus jira workitem create -p KAN -t Task -s "dry run" --dry-run` | `[dry-run] create Task "dry run" in KAN`, **không** tạo ticket |
| 3.2 | `acli-plus jira workitem edit --jql "project = KAN" -s "x" --dry-run` | `[dry-run] edit <key A>, <key B> (summary)` |
| 3.3 | `acli-plus jira workitem delete --jql "project = KAN" --dry-run` | `[dry-run] delete ...` và **không hỏi xác nhận** |
| 3.4 | Chạy lại lệnh đếm ở trên | Số dòng **không đổi** |

---

## 4. Tạo work item

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 4.1 | `acli-plus jira workitem create -p KAN -t Task -s "e2e basic"` | `created KAN-N "e2e basic" -> https://.../browse/KAN-N` |
| 4.2 | `acli-plus jira workitem create -p KAN -t Task -s "e2e assigned" -a @me` | Tạo xong, assignee là bạn |
| 4.3 | `acli-plus jira workitem create -p KAN -t Task -s "e2e full" -a @me -l backend,q3 --priority High --due 2026-12-31` | Đủ label/priority/due |
| 4.4 | Markdown trong `-d` | Để riêng ngay dưới bảng — chuỗi test có backtick |
| 4.5 | Tạo từ file Markdown | Cần `/tmp/story.md`, mà file đó được tạo ở [mục 5](#5-markdown--adf) — chạy trọn mục 5, nó gồm đúng lệnh này |
| 4.6 | Tạo ticket con bằng `--parent` | Để riêng ngay dưới bảng — `--parent` có hai tầng, dễ dính lỗi 400 |
| 4.7 | `acli-plus jira workitem create -p KAN -t Task -e` | Mở `$EDITOR`; **dòng đầu = summary**, phần còn lại = description. Lưu & thoát → tạo ticket. Xoá sạch nội dung → báo `nothing was written in the editor` |

**4.4 — Markdown trong `-d`.** Case này để riêng ra khỏi bảng vì chuỗi test có
backtick, mà backtick thì không sống được trong ô bảng markdown:

```bash
acli-plus jira workitem create -p KAN -t Task -s "e2e md" \
  -d 'Có **đậm**, *nghiêng*, `code` và [link](https://example.com).'
```

Kỳ vọng: mở ticket trên web → thấy định dạng thật, không phải chữ `**` thô.

**4.6 — `--parent`.** Jira team-managed có hai tầng cha con khác nhau, và dùng
nhầm tầng thì chỉ nhận được `400 ... Please select valid parent issue.` chứ
không nói tầng nào sai:

| Type đang tạo | Cha hợp lệ phải là |
|---|---|
| `Subtask` | một `Task` / `Story` / `Bug` |
| `Task` / `Story` / `Bug` | một `Epic` |
| `Epic` | không có cha |

Lấy key một Task đang tồn tại rồi tạo subtask dưới nó:

```bash
acli-plus jira workitem search --jql "project = KAN AND type = Task" --limit 1 --csv
acli-plus jira workitem create -p KAN -t Subtask -s "e2e sub" --parent <key vừa lấy>
```

Type nào có thật trong project thì xem [mục 0.3](#03-chọn-project-để-test) —
project `KAN` có `Epic`, `Task`, `Story`, `Bug`, `Subtask`.

⚠️ Bắt buộc **nháy đơn** cho `-d`. Nháy kép thì shell hiểu `` `code` `` là
command substitution: nó đi chạy lệnh tên `code` rồi nhét output vào mô tả — tuỳ
máy mà ra treo shell, mô tả rỗng, hay chạy nhầm thứ khác. Nháy đơn giữ nguyên
văn mọi ký tự.

**Field tuỳ ý** — đây là phần đáng test nhất. Custom field khác nhau theo từng
site, nên xem site mình có gì trước khi chạy 4.9–4.11:

```bash
acli-plus jira field list --custom --csv
```

Site `KAN` chỉ có 8 custom field, **không** có `Story Points` và **không** có
`customfield_10016` — hai thứ hay xuất hiện trong ví dụ trên mạng.

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 4.8 | `acli-plus jira workitem create -p KAN -t Task -s "e2e field" --field "Labels=a,b"` | Gọi field bằng **tên hiển thị**, tự map sang id `labels` và tách mảng |
| 4.9 | `acli-plus jira workitem create -p KAN -t Task -s "e2e sp" --field "Story Points=5"` | ✅ **Trên site KAN, PASS của case này chính là lỗi** `field not found: "Story Points"` — site không có field đó, và thứ đang test là acli-plus chặn tên field sai **trước khi** gọi API |
| 4.9b | `acli-plus jira workitem create -p KAN -t Task -s "e2e num" --field "Original estimate=3600"` | Nhánh dương của 4.9 bằng field số **có thật** trên site: set được giá trị số |
| 4.10 | `acli-plus jira workitem create -p KAN -t Task -s "e2e raw" --field 'labels=["x","y"]'` | Truyền thẳng JSON — đường thoát cho field lạ |
| 4.11 | `acli-plus jira workitem create -p KAN -t Task -s "e2e id" --field "customfield_10015=2026-12-31"` | Gọi thẳng bằng id cũng được (`customfield_10015` = `Start date`) |

**Tạo hàng loạt:**

Template do `--generate-json` in ra dùng project mẫu `"TEAM"` và field
`"Story Points"` — site này không có cả hai, nên `sed` sửa luôn cho khỏi phải mở
file ra vá tay:

```bash
acli-plus jira workitem create-bulk --generate-json \
  | sed -e 's/"TEAM"/"KAN"/' -e 's/"Story Points": "3"//' > /tmp/bulk.json
acli-plus jira workitem create-bulk --from-json /tmp/bulk.json --dry-run
acli-plus jira workitem create-bulk --from-json /tmp/bulk.json
```

Site có `Story Points` thì bỏ vế `sed` thứ hai đi để test luôn field đó.

| # | Kỳ vọng |
|---|---|
| 4.12 | `--generate-json` in ra template JSON hợp lệ |
| 4.13 | `--dry-run` báo `[dry-run] create 1 work items` |
| 4.14 | Chạy thật → `created 1 work items: KAN-N` |
| 4.15 | Thiếu field bắt buộc ở 1 phần tử → báo `item 2: ...` chỉ rõ phần tử nào hỏng, **các phần tử thành công vẫn được liệt kê** |

---

## 5. Markdown → ADF

Đây là phần riêng của acli-plus (Jira v3 bắt buộc ADF). Tạo file:

````bash
cat > /tmp/story.md <<'MD'
---
title: e2e from file
---

Intro với **bold**, *italic* và `inline code`.

## A heading

- first bullet
- second bullet

1. numbered one
2. numbered two

- [x] a done task
- [ ] a pending task

> a quoted line

```go
fmt.Println("hello")
```

| col a | col b |
|---|---|
| 1 | 2 |

A [link](https://example.com).

---
MD
````

```bash
acli-plus jira workitem create -p KAN -t Task --from-file /tmp/story.md
acli-plus jira workitem view <key mà lệnh trên vừa in ra>
```

| # | Kỳ vọng khi `view` |
|---|---|
| 5.1 | `Summary: e2e from file` — lấy từ frontmatter, **không** nằm trong description |
| 5.2 | `## A heading` |
| 5.3 | `- first bullet` |
| 5.4 | `1. numbered one` |
| 5.5 | `[x] a done task` và `[ ] a pending task` |
| 5.6 | `> a quoted line` |
| 5.7 | Code block giữ nguyên `fmt.Println("hello")` |
| 5.8 | Bảng thành `col a \| col b` |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 5.9 | Mở ticket trên web Jira | Định dạng render **thật** (heading to, checkbox bấm được, code có nền), không phải text thô |
| 5.10 | `echo '![alt](https://example.com/a.png)' > /tmp/img.md` rồi `create --description-file /tmp/img.md` | Ảnh thành link + `warning: image rendered as a link (ADF needs an uploaded media id)` trên stderr |
| 5.11 | `echo '{"type":"doc","version":1,"content":[]}' > /tmp/raw.json` rồi `create --description-file /tmp/raw.json -s x` | File đã là ADF → truyền thẳng, không convert |

---

## 6. Sửa, workflow, người phụ trách

Mục 6–8 cần ticket có thật. Key ticket **đổi theo từng lần chạy** và [mục
12](#12-dọn-dẹp) xoá sạch chúng, nên doc không viết cứng key được — viết cứng thì
lần sau copy ra chỉ nhận `work item not found`. Tạo sẵn hai ticket mang label
riêng để cả ba mục dùng chung:

```bash
acli-plus jira workitem create -p KAN -t Task -s "e2e A" -l e2e-muc6
acli-plus jira workitem create -p KAN -t Task -s "e2e B" -l e2e-muc6
```

Mỗi lệnh in ra key của nó. Từ đây:

- **`<key A>`** và **`<key B>`** trong bảng → thay bằng hai key vừa in ra
- lệnh nào có `--jql "... labels = e2e-muc6"` → **copy chạy thẳng**, không phải thay gì

Quên mất key thì lấy lại:

```bash
acli-plus jira workitem search --jql "project = KAN AND labels = e2e-muc6" --csv
```

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 6.1 | `acli-plus jira workitem view <key A>` | Khối field + description |
| 6.2 | `acli-plus jira workitem view <key A> --fields summary,status` | Chỉ 2 field |
| 6.3 | `acli-plus jira workitem view <key A> --json` | JSON thô, có custom field |
| 6.4 | `acli-plus jira workitem view "https://<host>/browse/<key A>"` | URL dùng thay key được, và **URL tự chọn site** |
| 6.5 | `acli-plus jira workitem view KAN-22x` | Lỗi `not a work item key ...` — chặn ngay, không gọi API |
| 6.6 | `acli-plus jira workitem edit <key A> -s "đã sửa"` | `updated KAN-N` |
| 6.7 | `acli-plus jira workitem edit <key A> --field "Labels=x,y"` | Ghi đè label |
| 6.8 | `acli-plus jira workitem edit <key A> --priority Low --no-notify` | Không gửi mail cho watcher |
| 6.9 | `acli-plus jira workitem edit --key <key A>,<key B> -l chung` | Sửa nhiều ticket 1 lệnh |
| 6.10 | `acli-plus jira workitem edit --jql "project = KAN AND labels = e2e-muc6" -l y` | Chọn theo JQL |
| 6.11 | `acli-plus jira workitem edit <key A>` | Lỗi `nothing to change: pass at least one field flag` |

**Chuyển trạng thái:**

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 6.12 | `acli-plus jira workitem transition <key A> --list` | Bảng `ID NAME MOVES TO SCREEN` — chỉ những transition **hợp lệ từ trạng thái hiện tại**. Board `KAN` trả về 4 dòng: `To Do`, `In Progress`, `In Review`, `Done` |
| 6.13 | `acli-plus jira workitem transition <key A> --status "In Progress"` | `moved KAN-N to "In Progress"` |
| 6.14 | `acli-plus jira workitem transition <key A> --status "Không Tồn Tại"` | Lỗi `KAN-N: no transition to that status is available from the current status (available: To Do, In Progress, In Review, Done)` — **có liệt kê cái đúng**, khác hẳn lỗi 400 trống trơn của API |
| 6.15 | `acli-plus jira workitem transition --jql "project = KAN AND labels = e2e-muc6" --status Done` | Chuyển hàng loạt theo JQL |

**Gán người:**

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 6.16 | `acli-plus jira workitem assign <key A> --assignee @me` | Gán cho bạn |
| 6.17 | `acli-plus jira workitem assign <key A> --assignee your@email.com` | Tìm theo email |
| 6.18 | `acli-plus jira workitem assign <key A> --assignee ""` | Bỏ gán → `assigned KAN-N to nobody` |
| 6.19 | `acli-plus jira workitem assign <key A> --assignee "Tên Trùng"` | Nếu trùng nhiều người → `"..." matches N accounts; pass an account id instead: ...` — **không đoán bừa** |
| 6.20 | `acli-plus jira workitem assign <key A> --assignee ghost@nowhere.com` | `user not found: ghost@nowhere.com` |

---

## 7. Comment, link, watcher, attachment

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 7.1 | `acli-plus jira workitem comment-create <key A> -b "Comment có **markdown**."` | `commented on KAN-N (comment 12345)` |
| 7.2 | `acli-plus jira workitem comment-create <key A> --body-file /tmp/story.md` | Comment dài từ file Markdown |
| 7.3 | `acli-plus jira workitem comment-list <key A>` | Mỗi comment: `id  thời gian  tác giả` rồi nội dung |
| 7.4 | `acli-plus jira workitem comment-list <key A> --json` | JSON |
| 7.5 | `acli-plus jira workitem comment-update <key A> --id 12345 -b "Sửa rồi."` | Nội dung đổi |
| 7.6 | `acli-plus jira workitem comment-visibility <key A> --id 12345 --type role --value Administrators` | Chỉ role đó đọc được; `comment-list` hiện `[role: Administrators]` |
| 7.7 | `acli-plus jira workitem comment-visibility <key A> --id 12345 --public` | Gỡ giới hạn — **nội dung comment phải giữ nguyên** (đây là bug dễ gặp: Jira update ghi đè cả body) |
| 7.8 | `acli-plus jira workitem comment-delete <key A> --id 12345` | Hỏi xác nhận rồi xoá |
| 7.9 | `acli-plus jira workitem comment-delete <key A> --id 12345 --yes` | Xoá không hỏi |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 7.10 | `acli-plus jira workitem link --list-types` | Danh sách link type của site |
| 7.11 | `acli-plus jira workitem link --type Relates --inward <key A> --outward <key B>` | `linked <key A> -> <key B> (Relates)` |
| 7.12 | `acli-plus jira workitem link --type blocks --inward <key A> --outward <key B>` | Chữ thường vẫn nhận, chuẩn hoá thành `Blocks` |
| 7.13 | `acli-plus jira workitem link --type "is blocked by" --inward <key A> --outward <key B>` | Nhận cả cụm từ chiều inward/outward |
| 7.14 | `acli-plus jira workitem link --type Sai --inward <key A> --outward <key B>` | `issue link type not found: "Sai" (available: Blocks, Relates, ...)` |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 7.15 | `acli-plus jira workitem watcher-list <key A>` | Bảng `ACCOUNT ID NAME EMAIL`; thường có bạn |
| 7.16 | `acli-plus jira workitem watcher-remove <key A> --watcher @me` | Bỏ theo dõi; chạy lại 7.15 để xác nhận |
| 7.17 | `acli-plus jira workitem attachment-list <key A>` | `no results` nếu chưa có file |
| 7.18 | Đính kèm 1 file qua web Jira, rồi chạy lại 7.17 | Bảng `ID FILENAME SIZE TYPE AUTHOR CREATED`, size dạng `2.0 KB` |
| 7.19 | `acli-plus jira workitem attachment-delete --id <id ở 7.18>` | Hỏi xác nhận rồi xoá |

---

## 8. Clone, xoá, archive

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 8.1 | `acli-plus jira workitem clone <key A>` | `cloned into KAN-N`, summary có tiền tố `CLONE -` — **ghi lại key này cho 8.4** |
| 8.2 | `acli-plus jira workitem clone <key A> --prefix "COPY -"` | Tiền tố tuỳ ý |
| 8.3 | `acli-plus jira workitem clone <key A> --prefix "" -p KHAC` | Clone sang project khác (flag ghi đè giá trị copy) |
| 8.4 | `acli-plus jira workitem view <key mà 8.1 in ra>` | Bản clone giữ description **có định dạng**, không phải text phẳng. Không copy status/comment/attachment/link |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 8.5 | `acli-plus jira workitem archive <key A>` | Có Premium → `archived KAN-N`. Không có → `this operation is not available on your Jira plan: archive requires Jira Premium or Enterprise` (**không phải lỗi 404 khó hiểu**) |
| 8.6 | `acli-plus jira workitem unarchive <key A>` | Khôi phục (chỉ khi 8.5 thành công) |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 8.7 | `acli-plus jira workitem delete <key A>` rồi gõ `n` | `aborted; no changes made` — ticket còn nguyên |
| 8.8 | `acli-plus jira workitem delete <key A>` rồi gõ `y` | Xoá. Lưu ý prompt ghi rõ **This cannot be undone** — khác Confluence, ticket Jira không vào thùng rác |
| 8.9 | `acli-plus jira workitem delete <key A> --yes` | Xoá không hỏi |
| 8.10 | `acli-plus jira workitem delete <key A> --with-subtasks --yes` | Xoá kèm subtask (không có cờ này Jira từ chối xoá cha còn subtask) |
| 8.11 | `acli-plus jira workitem delete --jql "project = KAN AND labels = acli-plus-e2e" --yes` | Xoá hàng loạt theo JQL — dùng để dọn rác |

---

## 9. Project và field (cần quyền admin)

Chỉ chạy nếu bạn là Jira admin **và** chấp nhận để lại/xoá thứ thật trên site.

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 9.1 | `acli-plus jira project create --key E2E --name "E2E scratch" --type software --dry-run` | `[dry-run] create software project E2E (E2E scratch)` |
| 9.2 | `acli-plus jira project create --key E2E --name "E2E scratch"` | Tạo thật; lead mặc định là bạn |
| 9.3 | `acli-plus jira project update E2E --name "E2E đổi tên"` | `updated project E2E` |
| 9.4 | `acli-plus jira project view E2E` | Tên mới |
| 9.5 | `acli-plus jira project archive E2E` | Cần Premium, ngược lại báo giới hạn plan |
| 9.6 | `acli-plus jira project restore E2E` | Khôi phục |
| 9.7 | `acli-plus jira project delete E2E` | Hỏi xác nhận, mặc định vào thùng rác (`It moves to the trash and can be restored.`) |
| 9.8 | `acli-plus jira project delete E2E --no-undo --yes` | Xoá thẳng, prompt ghi `This cannot be undone.` |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 9.9 | `acli-plus jira field create --name "E2E Field" --type com.atlassian.jira.plugin.system.customfieldtypes:textfield` | `created custom field "E2E Field" (customfield_NNNNN)` |
| 9.10 | `acli-plus jira field list --query "E2E Field"` | Thấy field vừa tạo |
| 9.11 | `acli-plus jira field delete "E2E Field"` | Hỏi xác nhận rồi cho vào thùng rác |
| 9.12 | `acli-plus jira field cancel-delete "E2E Field"` | Khôi phục |
| 9.13 | `acli-plus jira field delete "Summary"` | `Summary is a system field and cannot be deleted` |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 9.14 | `acli-plus jira filter add-favourite 10001` | `favourited filter(s) ...` |
| 9.15 | `acli-plus jira filter add-favourite "Tên filter"` | Gọi bằng tên cũng được |
| 9.16 | `acli-plus jira filter change-owner 10001 --owner someone@acme.com` | Chuyển chủ sở hữu |

---

## 10. Cấu hình riêng theo repo (`acli-plus.yaml`)

Đây là tầng per-project ở [mục 0.2b](#02b-config-hai-tầng--token-chung-site-riêng-theo-repo):
mỗi repo tự khai site và default của nó, **không chứa token**, commit thoải mái.

```bash
cat > acli-plus.yaml <<EOF
site: https://<host của bạn>
jira_project: KAN
jira_board: 1
EOF
```

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 10.1 | `acli-plus jira workitem create -t Task -s "không cần -p"` | Lấy project từ `jira_project` |
| 10.2 | `acli-plus jira board list-sprints` | Lấy board từ `jira_board` |
| 10.3 | `acli-plus jira board search` | Lọc theo `jira_project` |
| 10.4 | Xoá `acli-plus.yaml`, chạy lại 10.1 | `--project is required to create a work item` |

**Kiểm chứng nhiều repo trỏ nhiều site khác nhau** — đây là kịch bản mà design
gốc muốn phục vụ (mỗi client một site, token nằm chung một store):

```bash
mkdir -p /tmp/repo-a /tmp/repo-b
printf 'site: https://site-a.atlassian.net\njira_project: AAA\n' > /tmp/repo-a/acli-plus.yaml
printf 'site: https://site-b.atlassian.net\njira_project: BBB\n' > /tmp/repo-b/acli-plus.yaml
```

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 10.5 | `cd /tmp/repo-a && acli-plus jira project list` | Đi tới `site-a`; chưa đăng ký thì báo `no credentials for host ... (host site-a.atlassian.net)` — chứng tỏ nó đọc đúng file của repo A |
| 10.6 | `cd /tmp/repo-b && acli-plus jira project list` | Đi tới `site-b` — cùng binary, khác repo, khác site |
| 10.7 | `cd /tmp/repo-a && acli-plus jira project list --site <host thật>` | `--site` thắng file của repo |
| 10.8 | `grep -r token /tmp/repo-*/acli-plus.yaml` | Không có kết quả — token **không bao giờ** nằm trong file repo |

Nhớ `rm acli-plus.yaml` sau khi test nếu không muốn commit vào repo này.

---

## 11. Lỗi và exit code

| # | Lệnh | Kỳ vọng | Exit |
|---|---|---|---|
| 11.1 | `acli-plus jira workitem view "KAN-1" --site chua-dang-ky.example.com` | `no credentials for host; run 'acli-plus setup' (host chua-dang-ky.example.com)` | 1 |
| 11.1b | Đăng ký ≥2 site rồi chạy `acli-plus jira project list` | `several sites are registered; pass --site to choose one (registered: a..., b...)` — không đoán bừa | 1 |
| 11.2 | `acli-plus jira workitem view NOTAKEY` | `not a work item key (expected e.g. TEAM-123)...` | 1 |
| 11.3 | `acli-plus jira workitem view "KAN-999999"` | `work item not found: KAN-999999` | 1 |
| 11.4 | `acli-plus jira workitem search` | `a JQL query is required (--jql)` | 1 |
| 11.5 | `acli-plus jira workitem search --jql "cú pháp sai !!!"` | Lỗi JQL của Jira được in nguyên văn | 1 |
| 11.6 | `acli-plus jira workitem create -p KAN -s "thiếu type"` | `--type is required to create a work item` | 1 |
| 11.7 | `acli-plus jira workitem edit -s x` | `no work items selected: pass a key, --key, or --jql` | 1 |
| 11.7b | `acli-plus jira workitem edit --jql "project = KAN AND labels = khong-ton-tai" -s x` | `the JQL query matched no work items: ...` — **khác** 11.7: query chạy được nhưng không khớp gì | 1 |
| 11.8 | `acli-plus jira workitem create -p KAN -t Task -s x --field "Không Có=1"` | `field not found: "Không Có"` | 1 |
| 11.9 | `acli-plus jira workitem create -p KAN -t Task -s x --field "Original estimate=abc"` | `field Original estimate: expects a number, got "abc"` — dùng field số **có thật** trên site, chứ `Story Points` sẽ dừng sớm hơn ở `field not found` và không chạm tới nhánh kiểm kiểu | 1 |
| 11.10 | `acli-plus jira workitem create -p KAN -t Task -s x --field "thiếu bằng"` | `--field expects name=value` | 1 |
| 11.11 | `acli-plus jira workitem comment-update <key A> -b x` | `--id is required (see 'comment-list' for comment ids)` | 1 |

Kiểm tra exit code:

```bash
acli-plus jira workitem view NOTAKEY; echo "exit=$?"     # phải là 1
acli-plus jira project list > /dev/null; echo "exit=$?"  # phải là 0
```

Kiểm tra stdout/stderr tách nhau (quan trọng khi pipe):

```bash
acli-plus jira workitem search --jql "project = KAN" --csv 2>/dev/null | head -3
# → chỉ CSV sạch, warning và "no results" nằm ở stderr
```

---

## 12. Dọn dẹp

Ticket sinh ra từ ba nguồn khác nhau, dọn cả ba:

```bash
# 1. ticket do scripts/e2e-jira.sh tạo
acli-plus jira workitem delete --jql "project = KAN AND labels = acli-plus-e2e" --yes

# 2. hai ticket A/B của mục 6
acli-plus jira workitem delete --jql "project = KAN AND labels = e2e-muc6" --yes
```

Ticket tạo tay ở mục 4 **không mang label nào**, chỉ nhận ra được qua summary.
Xem trước rồi hãy xoá — `~` là so khớp gần đúng, dễ quét trúng ticket thật:

```bash
acli-plus jira workitem search --jql "project = KAN AND summary ~ 'e2e'" --paginate
```

Đúng toàn ticket rác thì xoá:

```bash
acli-plus jira workitem delete --jql "project = KAN AND summary ~ 'e2e'" --yes
```
