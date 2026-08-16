# E2E test — `acli-plus confluence page`

Kiểm chứng lệnh Confluence trên site thật. Có 2 cách dùng:

- **Chạy tất cả bằng 1 lệnh** → [`scripts/e2e-confluence.sh`](e2e-confluence.sh)
- **Chạy tay từng case** → bảng bên dưới

Bài này song song với [`e2e-jira.md`](e2e-jira.md) và dùng chung binary, chung
credential — Confluence và Jira là cùng một site, cùng một API token.

---

## 0. Chuẩn bị

### 0.1. Binary

Cần bản đã có nhóm lệnh `confluence page`. Kiểm tra:

```bash
acli-plus confluence page --help
```

Báo `unknown command "confluence"` nghĩa là bản trên PATH cũ hơn source — lệnh
Confluence trước đây nằm thẳng ở gốc (`acli-plus create`). Cài lại:

```bash
make release && make install
```

⚠️ `make install` một mình **không đủ**: [`install.sh`](../install.sh) ưu tiên
binary dựng sẵn trong `dist/` và không cảnh báo khi nó cũ hơn source. `make
release` build lại `dist/` trước, nên phải chạy cả hai.

### 0.2. Space dùng để test

Mọi lệnh dưới đây đã viết sẵn space **`SD`**:

```
https://thanh191997-acli.atlassian.net/wiki/spaces/SD
```

**Không cần export biến gì** — copy dòng nào chạy dòng đó.

Muốn dùng space khác thì tìm-thay đúng chuỗi URL trên. Mở space đó trên web,
copy URL ở thanh địa chỉ; cả hai dạng đều được:

| Dạng | Trang test nằm ở đâu |
|---|---|
| `.../wiki/spaces/DEV` | gốc space |
| `.../wiki/spaces/DEV/pages/98765/Handbook` | con của trang đó |

**Không dùng được** link rút gọn `/wiki/x/AbCd` — acli-plus từ chối và bảo bạn
dán URL đầy đủ.

⚠️ Đây là space rác để test. Đừng trỏ vào space thật: script tạo trang, sửa
trang và xoá trang trong đó.

### 0.3. Những gì không test tự động được

| Thứ | Vì sao |
|---|---|
| Cảnh báo khi trang bị sửa ngoài acli-plus | Phải vào Confluence sửa tay để tạo phiên bản không mang dấu của acli-plus. |
| Xoá vĩnh viễn | Không hỗ trợ theo thiết kế — `delete` chỉ đưa vào thùng rác. |
| Trang **render** ra sao trên web | `view` trả về storage format (XHTML) đúng như đã lưu, chưa có bộ chuyển ngược sang text dễ đọc. Đủ để khẳng định markup đúng, nhưng muốn xem mắt thường thì vẫn mở trang. |

---

## 1. Chạy tất cả bằng 1 lệnh

### An toàn — chỉ parse, đọc và dry-run

Không ghi gì lên Confluence. Chạy cái này trước:

```bash
./scripts/e2e-confluence.sh --parent "https://thanh191997-acli.atlassian.net/wiki/spaces/SD" --read-only
```

### Đầy đủ — có ghi thật

Tạo trang thật dưới `--parent` rồi **tự đưa vào thùng rác khi kết thúc** (kể cả
khi fail giữa chừng, nhờ `trap EXIT`). Hỏi xác nhận trước lần ghi đầu tiên.

```bash
./scripts/e2e-confluence.sh --parent "https://thanh191997-acli.atlassian.net/wiki/spaces/SD"
```

Tuỳ chọn:

```bash
./scripts/e2e-confluence.sh --parent "https://thanh191997-acli.atlassian.net/wiki/spaces/SD" --verbose   # in output từng lệnh
./scripts/e2e-confluence.sh --parent "https://thanh191997-acli.atlassian.net/wiki/spaces/SD" --keep      # giữ trang lại để xem
./scripts/e2e-confluence.sh --parent "https://thanh191997-acli.atlassian.net/wiki/spaces/SD" --yes       # không hỏi (cho CI)
./scripts/e2e-confluence.sh --parent "https://thanh191997-acli.atlassian.net/wiki/spaces/SD" --site acme.atlassian.net
```

Trang script tạo ra đều mang tiền tố `acli-plus-e2e-<timestamp>` trong tiêu đề.
Nếu script bị `kill -9` giữa chừng, tìm theo tiền tố đó trên web mà dọn.

Exit code `0` khi mọi case pass, `1` nếu có case fail. Đừng lấy con số
passed/skipped làm chuẩn — nó đổi theo space và theo số case; chỉ `0 failed` mới
là điều cần đúng.

---

## 2. Parse URL — không gọi API

Những ca này hỏng ngay ở bước đọc tham số, chưa chạm tới mạng.

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 2.1 | `acli-plus confluence page delete "https://thanh191997-acli.atlassian.net/wiki/x/AbCd" --yes` | `short /wiki/x/ links are not supported; paste the full page URL` |
| 2.2 | `acli-plus confluence page delete "" --yes` | `not a recognizable Confluence URL or page id` |
| 2.3 | `acli-plus confluence page delete 999999999 --yes` | `page not found: 999999999` — id trần hợp lệ về cú pháp, chỉ là không tồn tại |

⚠️ **Một bất đối xứng đáng biết:** link mà acli-plus **in ra** sau khi ghi có
dạng `.../wiki/pages/viewpage.action?pageId=N`, và chính parser của nó **không**
nhận lại dạng đó — `viewpage.action` bị đọc thành page id, kết quả là
`confluence api 400`. Nên khi viết script, hãy lấy **số** `pageId=` ra mà dùng,
đừng dán nguyên link:

```bash
acli-plus confluence page delete 120033 --yes          # đúng
```

Script `e2e-confluence.sh` ghim hành vi này ở phase 1, để nếu sau này có sửa
phía nào thì nó hiện ra thành một case fail chứ không im lặng trôi qua.

---

## 3. Dry-run — nói mà không làm

```bash
printf 'nội dung thử\n' > /tmp/e2e-page.md
```

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 3.1 | `acli-plus confluence page create /tmp/e2e-page.md "https://thanh191997-acli.atlassian.net/wiki/spaces/SD" --dry-run` | `[dry-run] created "e2e-page" -> …`, và **không** có trang nào mọc ra |
| 3.2 | `acli-plus confluence page update /tmp/e2e-page.md <pageId> --dry-run` | `[dry-run] updated …` |
| 3.3 | `acli-plus confluence page delete <pageId> --dry-run --yes` | `[dry-run] deleted (moved to trash) …` |

---

## 4. Tạo trang và đọc lại

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 4.1 | `acli-plus confluence page create /tmp/e2e-page.md "https://thanh191997-acli.atlassian.net/wiki/spaces/SD"` | `created "e2e-page" -> https://…?pageId=N`. Ghi lại số `N` |
| 4.2 | Chạy lại **đúng lệnh 4.1** | `updated`, và `pageId` **y hệt** lần trước — `create` là upsert theo tiêu đề, không đẻ trang trùng |

Đây là hành vi dễ hiểu nhầm nhất: `create` chạy lần hai **không** tạo bản sao.
Nó tìm trang con cùng tiêu đề dưới cùng parent, thấy thì cập nhật.

**Đọc lại để kiểm chứng nội dung.** Không có `view` thì mọi khẳng định chỉ dừng
ở "lệnh chạy không lỗi":

```bash
acli-plus confluence page view <pageId>
```

In ra khối `Id/Title/Space id/Parent id/Version/URL`, rồi tới thân bài ở dạng
**storage format** (XHTML) đúng như Confluence đang lưu. Thêm `--json` nếu muốn
máy đọc.

Markdown được chuyển thành thẻ, nên đây là cách đối chiếu:

| Markdown | Storage format |
|---|---|
| `**bold**` | `<strong>bold</strong>` |
| `# Tiêu đề` | `<h1>Tiêu đề</h1>` |
| `## Mục con` | `<h2>Mục con</h2>` |
| `- a` | `<ul><li>a</li></ul>` |
| `[link](https://example.com)` | `<a href="https://example.com">link</a>` |

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 4.3 | `acli-plus confluence page view <pageId>` | Thân bài là XHTML, **không** còn ký tự Markdown thô như `**` |
| 4.4 | `acli-plus confluence page view <pageId> --json` | JSON có `"Body"` |
| 4.5 | `acli-plus confluence page view "https://thanh191997-acli.atlassian.net/wiki/spaces/SD"` | `view needs a page URL (with a page id) or a page id` — space không trỏ tới một trang cụ thể |
| 4.6 | `acli-plus confluence page view 999999999` | `page not found: 999999999` |

---

## 5. Tiêu đề trang lấy từ đâu

Ba quy tắc, theo thứ tự.

```bash
printf -- '---\ntitle: Tiêu đề từ frontmatter\n---\n\nthân bài\n' > /tmp/e2e-fm.md
printf 'không có frontmatter\n' > /tmp/e2e-byname.md
printf '# Heading dẫn đầu\n\nthân bài\n' > /tmp/e2e-h1.md
```

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 5.1 | `acli-plus confluence page create /tmp/e2e-fm.md "https://thanh191997-acli.atlassian.net/wiki/spaces/SD"` | Tiêu đề là `Tiêu đề từ frontmatter` |
| 5.2 | `acli-plus confluence page create /tmp/e2e-byname.md "https://thanh191997-acli.atlassian.net/wiki/spaces/SD"` | Tiêu đề là `e2e-byname` — tên file bỏ đuôi |
| 5.3 | `acli-plus confluence page create /tmp/e2e-h1.md "https://thanh191997-acli.atlassian.net/wiki/spaces/SD"` | Tiêu đề là `e2e-h1`, **không** phải `Heading dẫn đầu` |
| 5.4 | `acli-plus confluence page view <pageId của 5.3>` | Thân bài chứa `<h1>Heading dẫn đầu</h1>` |

5.4 kiểm tra vế còn lại của quy tắc: H1 **không** được lấy làm tiêu đề, nhưng
cũng **không** được cắt khỏi thân bài. Bỏ sót vế này thì một bản sửa làm mất H1
vẫn qua được 5.3.

---

## 6. Sửa trang

⚠️ **Trang vừa tạo xong vẫn bị hỏi khi update.** Đây là điểm gợn thật, không
phải bạn làm sai:

```
Page "e2e-page" (id 3211321, version 1) was modified outside acli-plus. Overwrite? [y/N]:
```

Lý do: acli-plus nhận ra bài viết của chính nó bằng **version message**, mà API
create của Confluence **không nhận** trường đó — trang mới luôn là version 1 với
message rỗng, không phân biệt được với trang gõ tay trên web. Chỉ từ lần update
thứ hai trở đi mới hết hỏi, vì lúc đó version đã mang dấu.

Nên mọi `update` trong `e2e-confluence.sh` đều kèm `--yes`. Chạy tay cũng vậy
nếu không muốn bị hỏi.

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 6.0 | `acli-plus confluence page update /tmp/e2e-page.md <pageId>` (không `--yes`) | Hỏi `modified outside acli-plus`; gõ `n` → `aborted; no changes made` |
| 6.1 | `acli-plus confluence page update /tmp/e2e-page.md <pageId> --yes` | `updated`, `pageId` không đổi |
| 6.1b | `acli-plus confluence page view <pageId>` | Nội dung mới có mặt, nội dung cũ **biến mất** — update là ghi đè chứ không nối thêm. `Version` tăng đúng 1 |
| 6.2 | `acli-plus confluence page update /tmp/e2e-fm.md <pageId> --yes` | Trang bị **đổi tên** thành tiêu đề trong file, nhưng vẫn **cùng `pageId`** |
| 6.3 | `acli-plus confluence page update /tmp/e2e-page.md "https://thanh191997-acli.atlassian.net/wiki/spaces/SD/pages/<id đã xoá>/x"` | Trang không còn → nội dung được chèn vào **space** trong URL, kèm warning báo đã fallback |
| 6.4 | Sửa trang trên web, rồi chạy lại 6.1 | Cảnh báo trang bị sửa ngoài acli-plus và **hỏi xác nhận** trước khi đè |
| 6.5 | Lặp 6.4 với `--force` | Đè thẳng, không hỏi |

6.4 và 6.5 phải làm tay vì cần một lần sửa thật trong giao diện Confluence.

---

## 7. Xoá trang

`delete` chỉ đưa vào thùng rác — khôi phục được, khác hẳn ticket Jira là xoá vĩnh viễn.

| # | Lệnh | Kỳ vọng |
|---|---|---|
| 7.1 | `acli-plus confluence page delete <pageId>` rồi gõ `n` | `aborted; no changes made` — trang còn nguyên |
| 7.2 | `acli-plus confluence page delete <pageId>` rồi gõ `y` | `deleted (moved to trash) "…"` |
| 7.3 | Chạy lại 7.2 với cùng id | `page not found: <pageId>` |
| 7.4 | `acli-plus confluence page delete <pageId> --yes` | Xoá không hỏi |

---

## 8. Dọn dẹp

Script tự dọn. Làm tay thì xoá theo id đã ghi lại:

```bash
acli-plus confluence page delete <pageId> --yes
```

Trang chạy tay ở mục 3–7 không mang tiền tố nào, nên phải tự nhớ id — hoặc tìm
theo tiêu đề (`e2e-page`, `e2e-fm`, `e2e-byname`, `e2e-h1`) trong space trên web.

Xoá nhầm cũng khôi phục được: Space settings → Content tools → Trash.
