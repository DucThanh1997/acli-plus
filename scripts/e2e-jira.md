# E2E test — `acli-plus jira`

Kiểm chứng lệnh Jira trên site thật. Có 2 cách dùng:

- **Chạy tất cả bằng 1 lệnh** → [`scripts/e2e-jira.sh`](e2e-jira.sh)
- **Chạy tay từng case** → bảng bên dưới, mỗi case là 1 lệnh copy-paste được

---

## 0. Chuẩn bị

### 0.1. Build binary mới — làm trước tiên

⚠️ **`acli-plus` trong PATH (`/usr/local/bin`) là bản cũ, chưa có lệnh `jira`.**
Kiểm tra nhanh:

```bash
acli-plus jira --help    # bản cũ: "unknown command"
```

Build bản mới:

```bash
make build
export ACLI=./bin/acli-plus
```

Muốn dùng thẳng `acli-plus` ở mọi nơi thì cài đè rồi `export ACLI=acli-plus`:

```bash
./install.sh
```

Tất cả lệnh trong tài liệu này dùng `$ACLI`.

### 0.2. Có cần setup lại không?

**Không cần.** Credential trong `~/.config/acli-plus/credentials.yaml` dùng chung
cho Confluence và Jira — cùng site, cùng account, cùng API token. Bạn đã đăng ký
`thanh191997-acli.atlassian.net` rồi nên các lệnh Jira chạy luôn được, đã kiểm
chứng thực tế.

Nếu muốn xác nhận token có quyền Jira thì chạy setup **bằng binary mới**:

```bash
$ACLI setup
```

Bản mới hỏi `Atlassian site URL` (bản cũ ghi `Confluence site URL`) và in thêm
dòng cuối:

```
Reachable on this site: Confluence, Jira
```

Thiếu chữ `Jira` nghĩa là token không có quyền Jira → tạo token mới tại
<https://id.atlassian.com/manage-profile/security/api-tokens>. Nhập lại đúng
site/email/token cũ là được, nó ghi đè đúng host chứ không tạo trùng.

**Không cần khai báo site ở đâu cả.** Lệnh Jira không có URL để suy ra host, nên
khi chỉ có đúng 1 site đã đăng ký, acli-plus tự dùng site đó. Chỉ khi đăng ký
nhiều site mới cần `--site`.

### 0.3. Chọn project để test

Cần **1 project Jira mà bạn được phép tạo/xoá ticket** — dùng project rác, đừng
dùng project thật. Xem có gì:

```bash
$ACLI jira project list
```

Trên site của bạn hiện có đúng một project:

```
KEY  NAME       TYPE      STYLE     LEAD
KAN  acli-plus  software  next-gen  Trần Thành
```

Đặt biến cho tiện copy-paste cả bài:

```bash
export P=KAN           # project key của bạn
export TYPE=Task       # đổi nếu project không có type "Task"
```

Kiểm tra type nào hợp lệ: mở project trong Jira, hoặc thử `create --dry-run` rồi
`create` thật — nếu sai type, Jira trả lỗi có liệt kê type hợp lệ.

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
./scripts/e2e-jira.sh --project $P --read-only
```

### Đầy đủ — có ghi thật

Tạo ticket thật trong `--project` rồi **tự xoá sạch khi kết thúc** (kể cả khi
fail giữa chừng, nhờ `trap EXIT`). Nó hỏi xác nhận trước lần ghi đầu tiên.

```bash
./scripts/e2e-jira.sh --project $P
```

Tuỳ chọn:

```bash
./scripts/e2e-jira.sh --project $P --verbose        # in output từng lệnh
./scripts/e2e-jira.sh --project $P --keep           # giữ lại ticket để xem
./scripts/e2e-jira.sh --project $P --yes            # không hỏi (cho CI)
./scripts/e2e-jira.sh --project $P --type Story     # type khác
./scripts/e2e-jira.sh --project $P --site acme.atlassian.net
```

Kết quả cuối:

```
== Summary
29 passed, 0 failed, 3 skipped
```

Số case SKIP thay đổi theo site: board Kanban không có sprint, project rỗng thì
không có gì để `edit`/`delete` dry-run. SKIP là bình thường, chỉ FAIL mới đáng lo.

Exit code `0` khi mọi case pass, `1` nếu có case fail. Ticket script tạo ra đều
có label `acli-plus-e2e` — nếu script bị `kill -9` giữa chừng, dọn tay bằng:

```bash
$ACLI jira workitem delete --jql "project = $P AND labels = acli-plus-e2e" --yes
```

---

## 2. Đọc — không ghi gì

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 2.1 | `$ACLI jira project list` | Bảng `KEY NAME TYPE STYLE LEAD`, có project của bạn |
| 2.2 | `$ACLI jira project list --json` | JSON thô từ API |
| 2.3 | `$ACLI jira project list --csv` | CSV có dòng header |
| 2.4 | `$ACLI jira project list --query team --type software` | Chỉ project khớp |
| 2.5 | `$ACLI jira project view $P` | Khối `Key/Name/Id/Type/Style/Lead/URL` |
| 2.6 | `$ACLI jira field list` | Bảng `ID NAME TYPE CUSTOM`, có `summary`, `labels` |
| 2.7 | `$ACLI jira field list --custom` | Chỉ field custom (`customfield_*`) |
| 2.8 | `$ACLI jira field list --query point` | Lọc theo tên — đây là cách tìm `customfield_NNNNN` |
| 2.9 | `$ACLI jira board search --project $P` | Bảng `ID NAME TYPE PROJECT`, hoặc `no results` nếu project không có board |
| 2.10 | `$ACLI jira board list-sprints --board 1` | Sprint của board (thay `1` bằng ID ở 2.9). **Board Kanban** → `this board does not use sprints (Kanban boards have none): board 1` — đây là kết quả **đúng**, sprint là khái niệm của Scrum. Board `KAN` của bạn là Kanban nên sẽ ra thông báo này |
| 2.11 | `$ACLI jira board list-sprints --board 1 --state active` | Chỉ sprint đang chạy (cần board Scrum) |
| 2.12 | `$ACLI jira sprint list-workitems --sprint 1` | Ticket trong sprint (cần board Scrum) |
| 2.13 | `$ACLI jira filter list` | Filter của bạn + favourite |
| 2.14 | `$ACLI jira filter list --favourites` | Chỉ favourite |
| 2.15 | `$ACLI jira filter search --name sprint` | Filter toàn site khớp tên |
| 2.16 | `$ACLI jira dashboard search` | Bảng dashboard |
| 2.17 | `$ACLI jira workitem link --list-types` | `NAME INWARD OUTWARD` — thường có `Blocks`, `Relates` |

**Tìm kiếm** — chú ý API mới không trả tổng số, nên mặc định chỉ 1 trang:

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 2.18 | `$ACLI jira workitem search --jql "project = $P"` | Bảng `KEY TYPE STATUS ASSIGNEE SUMMARY` |
| 2.19 | `$ACLI jira workitem search --jql "project = $P" --limit 3` | Đúng 3 dòng dữ liệu |
| 2.20 | `$ACLI jira workitem search --jql "project = $P" --paginate` | Đi hết mọi trang |
| 2.21 | `$ACLI jira workitem search --jql "assignee = currentUser()" --paginate` | Ticket của bạn |
| 2.22 | `$ACLI jira workitem search --jql "project = $P" --fields summary,status` | Chỉ fetch 2 field |
| 2.23 | `$ACLI jira workitem search --jql "project = $P" --csv > /tmp/out.csv` | File CSV mở được bằng Excel |
| 2.24 | `$ACLI jira workitem search --jql "project = $P" --limit 1 --json` | JSON đầy đủ, có `"fields"` |

---

## 3. Dry-run — kiểm tra "nói mà không làm"

Đếm số ticket trước và sau, phải bằng nhau.

```bash
$ACLI jira workitem search --jql "project = $P" --paginate | wc -l
```

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 3.1 | `$ACLI jira workitem create -p $P -t $TYPE -s "dry run" --dry-run` | `[dry-run] create Task "dry run" in TEAM`, **không** tạo ticket |
| 3.2 | `$ACLI jira workitem edit --jql "project = $P" -s "x" --dry-run` | `[dry-run] edit TEAM-1, TEAM-2 (summary)` |
| 3.3 | `$ACLI jira workitem delete --jql "project = $P" --dry-run` | `[dry-run] delete ...` và **không hỏi xác nhận** |
| 3.4 | Chạy lại lệnh đếm ở trên | Số dòng **không đổi** |

---

## 4. Tạo work item

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 4.1 | `$ACLI jira workitem create -p $P -t $TYPE -s "e2e basic"` | `created TEAM-N "e2e basic" -> https://.../browse/TEAM-N` |
| 4.2 | `$ACLI jira workitem create -p $P -t $TYPE -s "e2e assigned" -a @me` | Tạo xong, assignee là bạn |
| 4.3 | `$ACLI jira workitem create -p $P -t $TYPE -s "e2e full" -a @me -l backend,q3 --priority High --due 2026-12-31` | Đủ label/priority/due |
| 4.4 | `$ACLI jira workitem create -p $P -t $TYPE -s "e2e md" -d "Có **đậm**, *nghiêng*, \`code\` và [link](https://example.com)."` | Mở ticket trên web → thấy định dạng thật, không phải chữ `**` |
| 4.5 | `$ACLI jira workitem create -p $P -t $TYPE --from-file /tmp/story.md` | Summary lấy từ frontmatter `title:` hoặc H1 — xem [mục 5](#5-markdown--adf) |
| 4.6 | `$ACLI jira workitem create -p $P -t $TYPE -s "e2e sub" --parent TEAM-1` | Tạo con của TEAM-1 (type phải là Subtask nếu project yêu cầu) |
| 4.7 | `$ACLI jira workitem create -p $P -t $TYPE -e` | Mở `$EDITOR`; **dòng đầu = summary**, phần còn lại = description. Lưu & thoát → tạo ticket. Xoá sạch nội dung → báo `nothing was written in the editor` |

**Field tuỳ ý** — đây là phần đáng test nhất:

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 4.8 | `$ACLI jira workitem create -p $P -t $TYPE -s "e2e field" --field "Labels=a,b"` | Gọi field bằng **tên hiển thị**, tự map sang id `labels` và tách mảng |
| 4.9 | `$ACLI jira workitem create -p $P -t $TYPE -s "e2e sp" --field "Story Points=5"` | Nếu site có field này → set số `5`. Không có → `field not found: "Story Points"` |
| 4.10 | `$ACLI jira workitem create -p $P -t $TYPE -s "e2e raw" --field 'labels=["x","y"]'` | Truyền thẳng JSON — đường thoát cho field lạ |
| 4.11 | `$ACLI jira workitem create -p $P -t $TYPE -s "e2e id" --field "customfield_10016=3"` | Gọi thẳng bằng id cũng được |

**Tạo hàng loạt:**

```bash
$ACLI jira workitem create-bulk --generate-json > /tmp/bulk.json
# sửa "TEAM" thành project của bạn, bỏ "Story Points" nếu site không có
$ACLI jira workitem create-bulk --from-json /tmp/bulk.json --dry-run
$ACLI jira workitem create-bulk --from-json /tmp/bulk.json
```

| # | Kỳ vọng |
|---|---|
| 4.12 | `--generate-json` in ra template JSON hợp lệ |
| 4.13 | `--dry-run` báo `[dry-run] create 1 work items` |
| 4.14 | Chạy thật → `created 1 work items: TEAM-N` |
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
$ACLI jira workitem create -p $P -t $TYPE --from-file /tmp/story.md
$ACLI jira workitem view TEAM-N        # thay N
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

Đặt `export K=TEAM-N` (ticket tạo ở mục 4).

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 6.1 | `$ACLI jira workitem view $K` | Khối field + description |
| 6.2 | `$ACLI jira workitem view $K --fields summary,status` | Chỉ 2 field |
| 6.3 | `$ACLI jira workitem view $K --json` | JSON thô, có custom field |
| 6.4 | `$ACLI jira workitem view "https://<host>/browse/$K"` | URL dùng thay key được, và **URL tự chọn site** |
| 6.5 | `$ACLI jira workitem view ${K}x` | Lỗi `not a work item key ...` — chặn ngay, không gọi API |
| 6.6 | `$ACLI jira workitem edit $K -s "đã sửa"` | `updated TEAM-N` |
| 6.7 | `$ACLI jira workitem edit $K --field "Labels=x,y"` | Ghi đè label |
| 6.8 | `$ACLI jira workitem edit $K --priority Low --no-notify` | Không gửi mail cho watcher |
| 6.9 | `$ACLI jira workitem edit --key TEAM-1,TEAM-2 -l chung` | Sửa nhiều ticket 1 lệnh |
| 6.10 | `$ACLI jira workitem edit --jql "project = $P AND labels = x" -l y` | Chọn theo JQL |
| 6.11 | `$ACLI jira workitem edit $K` | Lỗi `nothing to change: pass at least one field flag` |

**Chuyển trạng thái:**

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 6.12 | `$ACLI jira workitem transition $K --list` | Bảng `ID NAME MOVES TO SCREEN` — chỉ những transition **hợp lệ từ trạng thái hiện tại** |
| 6.13 | `$ACLI jira workitem transition $K --status "In Progress"` | `moved TEAM-N to "In Progress"` |
| 6.14 | `$ACLI jira workitem transition $K --status "Không Tồn Tại"` | Lỗi `no transition to that status is available ... (available: In Progress, Done)` — **có liệt kê cái đúng** |
| 6.15 | `$ACLI jira workitem transition --jql "project = $P AND labels = x" --status Done` | Chuyển hàng loạt theo JQL |

**Gán người:**

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 6.16 | `$ACLI jira workitem assign $K --assignee @me` | Gán cho bạn |
| 6.17 | `$ACLI jira workitem assign $K --assignee your@email.com` | Tìm theo email |
| 6.18 | `$ACLI jira workitem assign $K --assignee ""` | Bỏ gán → `assigned TEAM-N to nobody` |
| 6.19 | `$ACLI jira workitem assign $K --assignee "Tên Trùng"` | Nếu trùng nhiều người → `"..." matches N accounts; pass an account id instead: ...` — **không đoán bừa** |
| 6.20 | `$ACLI jira workitem assign $K --assignee ghost@nowhere.com` | `user not found: ghost@nowhere.com` |

---

## 7. Comment, link, watcher, attachment

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 7.1 | `$ACLI jira workitem comment-create $K -b "Comment có **markdown**."` | `commented on TEAM-N (comment 12345)` |
| 7.2 | `$ACLI jira workitem comment-create $K --body-file /tmp/story.md` | Comment dài từ file Markdown |
| 7.3 | `$ACLI jira workitem comment-list $K` | Mỗi comment: `id  thời gian  tác giả` rồi nội dung |
| 7.4 | `$ACLI jira workitem comment-list $K --json` | JSON |
| 7.5 | `$ACLI jira workitem comment-update $K --id 12345 -b "Sửa rồi."` | Nội dung đổi |
| 7.6 | `$ACLI jira workitem comment-visibility $K --id 12345 --type role --value Administrators` | Chỉ role đó đọc được; `comment-list` hiện `[role: Administrators]` |
| 7.7 | `$ACLI jira workitem comment-visibility $K --id 12345 --public` | Gỡ giới hạn — **nội dung comment phải giữ nguyên** (đây là bug dễ gặp: Jira update ghi đè cả body) |
| 7.8 | `$ACLI jira workitem comment-delete $K --id 12345` | Hỏi xác nhận rồi xoá |
| 7.9 | `$ACLI jira workitem comment-delete $K --id 12345 --yes` | Xoá không hỏi |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 7.10 | `$ACLI jira workitem link --list-types` | Danh sách link type của site |
| 7.11 | `$ACLI jira workitem link --type Relates --inward TEAM-1 --outward TEAM-2` | `linked TEAM-1 -> TEAM-2 (Relates)` |
| 7.12 | `$ACLI jira workitem link --type blocks --inward TEAM-1 --outward TEAM-2` | Chữ thường vẫn nhận, chuẩn hoá thành `Blocks` |
| 7.13 | `$ACLI jira workitem link --type "is blocked by" --inward TEAM-1 --outward TEAM-2` | Nhận cả cụm từ chiều inward/outward |
| 7.14 | `$ACLI jira workitem link --type Sai --inward TEAM-1 --outward TEAM-2` | `issue link type not found: "Sai" (available: Blocks, Relates, ...)` |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 7.15 | `$ACLI jira workitem watcher-list $K` | Bảng `ACCOUNT ID NAME EMAIL`; thường có bạn |
| 7.16 | `$ACLI jira workitem watcher-remove $K --watcher @me` | Bỏ theo dõi; chạy lại 7.15 để xác nhận |
| 7.17 | `$ACLI jira workitem attachment-list $K` | `no results` nếu chưa có file |
| 7.18 | Đính kèm 1 file qua web Jira, rồi chạy lại 7.17 | Bảng `ID FILENAME SIZE TYPE AUTHOR CREATED`, size dạng `2.0 KB` |
| 7.19 | `$ACLI jira workitem attachment-delete --id <id ở 7.18>` | Hỏi xác nhận rồi xoá |

---

## 8. Clone, xoá, archive

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 8.1 | `$ACLI jira workitem clone $K` | `cloned into TEAM-M`, summary có tiền tố `CLONE -` |
| 8.2 | `$ACLI jira workitem clone $K --prefix "COPY -"` | Tiền tố tuỳ ý |
| 8.3 | `$ACLI jira workitem clone $K --prefix "" -p KHAC` | Clone sang project khác (flag ghi đè giá trị copy) |
| 8.4 | `$ACLI jira workitem view TEAM-M` | Bản clone giữ description **có định dạng**, không phải text phẳng. Không copy status/comment/attachment/link |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 8.5 | `$ACLI jira workitem archive $K` | Có Premium → `archived TEAM-N`. Không có → `this operation is not available on your Jira plan: archive requires Jira Premium or Enterprise` (**không phải lỗi 404 khó hiểu**) |
| 8.6 | `$ACLI jira workitem unarchive $K` | Khôi phục (chỉ khi 8.5 thành công) |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 8.7 | `$ACLI jira workitem delete $K` rồi gõ `n` | `aborted; no changes made` — ticket còn nguyên |
| 8.8 | `$ACLI jira workitem delete $K` rồi gõ `y` | Xoá. Lưu ý prompt ghi rõ **This cannot be undone** — khác Confluence, ticket Jira không vào thùng rác |
| 8.9 | `$ACLI jira workitem delete $K --yes` | Xoá không hỏi |
| 8.10 | `$ACLI jira workitem delete TEAM-1 --with-subtasks --yes` | Xoá kèm subtask (không có cờ này Jira từ chối xoá cha còn subtask) |
| 8.11 | `$ACLI jira workitem delete --jql "project = $P AND labels = acli-plus-e2e" --yes` | Xoá hàng loạt theo JQL — dùng để dọn rác |

---

## 9. Project và field (cần quyền admin)

Chỉ chạy nếu bạn là Jira admin **và** chấp nhận để lại/xoá thứ thật trên site.

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 9.1 | `$ACLI jira project create --key E2E --name "E2E scratch" --type software --dry-run` | `[dry-run] create software project E2E (E2E scratch)` |
| 9.2 | `$ACLI jira project create --key E2E --name "E2E scratch"` | Tạo thật; lead mặc định là bạn |
| 9.3 | `$ACLI jira project update E2E --name "E2E đổi tên"` | `updated project E2E` |
| 9.4 | `$ACLI jira project view E2E` | Tên mới |
| 9.5 | `$ACLI jira project archive E2E` | Cần Premium, ngược lại báo giới hạn plan |
| 9.6 | `$ACLI jira project restore E2E` | Khôi phục |
| 9.7 | `$ACLI jira project delete E2E` | Hỏi xác nhận, mặc định vào thùng rác (`It moves to the trash and can be restored.`) |
| 9.8 | `$ACLI jira project delete E2E --no-undo --yes` | Xoá thẳng, prompt ghi `This cannot be undone.` |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 9.9 | `$ACLI jira field create --name "E2E Field" --type com.atlassian.jira.plugin.system.customfieldtypes:textfield` | `created custom field "E2E Field" (customfield_NNNNN)` |
| 9.10 | `$ACLI jira field list --query "E2E Field"` | Thấy field vừa tạo |
| 9.11 | `$ACLI jira field delete "E2E Field"` | Hỏi xác nhận rồi cho vào thùng rác |
| 9.12 | `$ACLI jira field cancel-delete "E2E Field"` | Khôi phục |
| 9.13 | `$ACLI jira field delete "Summary"` | `Summary is a system field and cannot be deleted` |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 9.14 | `$ACLI jira filter add-favourite 10001` | `favourited filter(s) ...` |
| 9.15 | `$ACLI jira filter add-favourite "Tên filter"` | Gọi bằng tên cũng được |
| 9.16 | `$ACLI jira filter change-owner 10001 --owner someone@acme.com` | Chuyển chủ sở hữu |

---

## 10. Cấu hình theo repo

```bash
cat > acli-plus.yaml <<EOF
site: https://<host của bạn>
jira_project: $P
jira_board: 1
EOF
```

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 10.1 | `$ACLI jira workitem create -t $TYPE -s "không cần -p"` | Lấy project từ `jira_project` |
| 10.2 | `$ACLI jira board list-sprints` | Lấy board từ `jira_board` |
| 10.3 | `$ACLI jira board search` | Lọc theo `jira_project` |
| 10.4 | Xoá `acli-plus.yaml`, chạy lại 10.1 | `--project is required to create a work item` |

Nhớ xoá `acli-plus.yaml` sau khi test nếu không muốn commit.

---

## 11. Lỗi và exit code

| # | Lệnh | Kỳ vọng | Exit |
|---|---|---|---|
| 11.1 | `$ACLI jira workitem view $P-1 --site chua-dang-ky.example.com` | `no credentials for host; run 'acli-plus setup' (host chua-dang-ky.example.com)` | 1 |
| 11.1b | Đăng ký ≥2 site rồi chạy `$ACLI jira project list` | `several sites are registered; pass --site to choose one (registered: a..., b...)` — không đoán bừa | 1 |
| 11.2 | `$ACLI jira workitem view NOTAKEY` | `not a work item key (expected e.g. TEAM-123)...` | 1 |
| 11.3 | `$ACLI jira workitem view $P-999999` | `work item not found: TEAM-999999` | 1 |
| 11.4 | `$ACLI jira workitem search` | `a JQL query is required (--jql)` | 1 |
| 11.5 | `$ACLI jira workitem search --jql "cú pháp sai !!!"` | Lỗi JQL của Jira được in nguyên văn | 1 |
| 11.6 | `$ACLI jira workitem create -p $P -s "thiếu type"` | `--type is required to create a work item` | 1 |
| 11.7 | `$ACLI jira workitem edit -s x` | `no work items selected: pass a key, --key, or --jql` | 1 |
| 11.7b | `$ACLI jira workitem edit --jql "project = $P AND labels = khong-ton-tai" -s x` | `the JQL query matched no work items: ...` — **khác** 11.7: query chạy được nhưng không khớp gì | 1 |
| 11.8 | `$ACLI jira workitem create -p $P -t $TYPE -s x --field "Không Có=1"` | `field not found: "Không Có"` | 1 |
| 11.9 | `$ACLI jira workitem create -p $P -t $TYPE -s x --field "Story Points=abc"` | `field Story Points: expects a number, got "abc"` | 1 |
| 11.10 | `$ACLI jira workitem create -p $P -t $TYPE -s x --field "thiếu bằng"` | `--field expects name=value` | 1 |
| 11.11 | `$ACLI jira workitem comment-update $K -b x` | `--id is required (see 'comment-list' for comment ids)` | 1 |

Kiểm tra exit code:

```bash
$ACLI jira workitem view NOTAKEY; echo "exit=$?"     # phải là 1
$ACLI jira project list > /dev/null; echo "exit=$?"  # phải là 0
```

Kiểm tra stdout/stderr tách nhau (quan trọng khi pipe):

```bash
$ACLI jira workitem search --jql "project = $P" --csv 2>/dev/null | head -3
# → chỉ CSV sạch, warning và "no results" nằm ở stderr
```

---

## 12. Dọn dẹp

```bash
# xoá mọi ticket do test tạo ra
$ACLI jira workitem delete --jql "project = $P AND labels = acli-plus-e2e" --yes

# kiểm tra còn sót không
$ACLI jira workitem search --jql "project = $P AND summary ~ 'e2e'" --paginate
```
