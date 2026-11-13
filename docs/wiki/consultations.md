# 教員と学生の相談（#2028）

教員の案内は一方向だったので、学生が返信・質問できるようにする。

## しくみ

- 案内（`teacher_student_guidances`）を1つのスレッドとし、発言を `guidance_messages`（migration 000069）に積む
- 学生からの質問は、種類 `question` の案内（`teacher_user_id = 0`）として作り、最初の発言も `guidance_messages` に入れる。担当校の先生なら誰でも答えられる
- `read_at` は相手側が読んだ日時。学生がスレッドを開くと先生の発言が、先生が開くか返信すると学生の発言が既読になる
- 学生が返信したスレッドは、閉じていても開き直す（先生の返答を見落とさないように）
- 本文は1000文字まで

## API

| 主体 | メソッド | パス | 内容 |
|---|---|---|---|
| 学生 | POST | `/api/user/consultations` | 先生への質問のスレッドを作る |
| 学生 | GET | `/api/user/consultations/unread` | スレッドごとの、先生の未読の発言の数 |
| 学生 | GET | `/api/user/consultations/:id` | 本人のスレッド（他人のは 404）。先生の発言を既読にする |
| 学生 | POST | `/api/user/consultations/:id/messages` | 本人のスレッドに返信 |
| 先生 | GET | `/api/admin/teacher/consultations?school_id=` | 受信一覧（担当校スコープ。学生の発言があるスレッドを新しい順に） |
| 先生 | GET | `/api/admin/teacher/consultations/:id` | スレッドを見る（学生の発言を既読にする） |
| 先生 | POST | `/api/admin/teacher/consultations/:id/messages` | 返信 |

先生のスレッドの2本は `school_id` を持たないので、コントローラでスレッドの学生の学校と担当校を突き合わせ、
担当校以外は 403 にする（`EnsureAdminSchoolAccessForUser`）。

## 画面

- 学生: `/consultations`（質問の投稿、やり取りの一覧と未読の印、スレッドのダイアログ）。ホームの案内の「先生に返信する」と、プロフィールのカードから開く
- 先生: `/admin/consultations`（受信一覧と未読の印、スレッドのダイアログ）。管理画面の「教員業務」から開く

## 未対応（別の PR）

- 通知センター（#2033、PR #2186）との連携。#2186 のマージ後に、先生の返信・学生の発言で通知を出す
