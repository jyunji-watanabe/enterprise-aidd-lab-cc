# 経費精算システム (Minimum Enterprise)

[enterprise-aidd-lab / Specs/MinimumEnterprise/Spec.md](https://github.com/jyunji-watanabe/enterprise-aidd-lab/blob/861a633cd040b02a6f139e640c39d0f6cbe968b4/Specs/MinimumEnterprise/Spec.md) の実装です。
社内ユーザーが経費を申請し、上長（部門長）と経理が承認・精算確定するWebアプリケーションです。

| レイヤ | 技術 |
| --- | --- |
| Backend | Go 1.24 / 標準 `net/http`（ルーティング） / `modernc.org/sqlite`（cgo不要の純Go SQLite） / bcrypt |
| Frontend | React 19 + TypeScript 5.9 / Vite 7 / React Router 7 |
| DB | SQLite（起動時に組み込みマイグレーションを自動適用、空DBなら初期データを自動投入） |
| 品質ハーネス | golangci-lint v2・go vet / ESLint（typescript-eslint strict・型情報あり）・Prettier・tsc / Go test・Vitest + Testing Library / Playwright |

---

## 1. クイックスタート

### 必要なもの
- Go 1.24+、Node.js 22+（`make` を使う場合は GNU make）
- E2E テストを実行する場合は Playwright の Chromium（`npx playwright install chromium`）

### 単一コマンドで起動
```bash
make setup   # 依存関係のインストール（初回のみ）
make run     # フロントをビルドし、http://localhost:8080 で起動（空DBなら初期データ投入）
```
DB は `backend/data/expense.db` に作成されます。`make reset-db` で削除すると、次回起動時に再シードされます。

### docker compose で起動
```bash
docker compose up --build   # http://localhost:8080
```
（Dockerfile はマルチステージ構成：Node でフロントをビルド → Go で静的バイナリをビルド → alpine に同梱。DB は `expense-data` ボリュームに保存）

### 開発モード（ホットリロード）
```bash
make dev-backend    # API: :8080
make dev-frontend   # Vite: http://localhost:5173 （/api を :8080 にプロキシ）
```

---

## 2. テスト用ログインアカウント

全員パスワードは **`Passw0rd!`** です。

| ユーザーID | 氏名 | 部門 | ロール |
| --- | --- | --- | --- |
| `employee1` | 山田 太郎 | 営業部 (SALES) | Employee（一般社員） |
| `employee2` | 佐藤 花子 | 開発部 (DEV) | Employee（一般社員） |
| `manager1` | 鈴木 一郎 | 営業部 (SALES) | Manager（承認者/部門長） |
| `manager2` | 高橋 次郎 | 開発部 (DEV) | Manager（承認者/部門長） |
| `admin1` | 田中 経理 | 経理部 (ACCT) | Admin（経理担当/管理者） |

### 初期シードデータ
- 部門: 営業部 / 開発部 / 経理部
- 勘定科目: 6110 旅費交通費、6120 交際費（上限 ¥50,000）、6130 会議費（上限 ¥10,000）、6140 消耗品費（上限 ¥100,000）、6150 通信費、6190 雑費（旧・**無効**）
- 申請 7件（下書き・申請中×2・一次承認済・精算確定×2・差戻し）。
  うち「申請中のまま10日」「一次承認済のまま9日」の2件はリマインドバッチの対象になります。
- シードはサービス層経由で投入するため、承認履歴・監査ログも実運用と同じ形で記録されます。

---

## 3. 動作確認手順

1. `employee1` でログイン →「新規申請」→ 明細に **35,000円・摘要「新幹線代」** を入力 →「保存して提出」
   → 「30,000円以上の明細は摘要を50文字以上」エラー（画面のヒント＋サーバー側 422）。摘要を50文字以上にして再提出 → 「申請中」
2. `manager1` でログイン →「承認待ち」→ 申請を開く → コメント未入力では承認ボタンが押せない → コメントを入れて「承認」→「一次承認済」
3. `admin1` でログイン →「承認待ち」→ 申請を開く → コメントを入れて「精算確定」
4. 詳細画面の「経費精算書（印刷）」→ 申請番号・承認履歴・明細・合計・捺印欄（承認者名＋タイムスタンプ）を表示。「印刷 / PDF保存」でブラウザの印刷ダイアログから PDF 保存（印刷用CSSで A4 レイアウト）
5. `admin1`「帳票・バッチ」→ 期間を指定して「CSVダウンロード」（精算確定データ、UTF-8 BOM付き）
6. `admin1`「帳票・バッチ」→「リマインドバッチを実行」→ 対象2件のリマインド（ダミーメール）ログが表示される
   CLI からは `make remind`（または `bin/expense remind -db <db>`）。ダミーメールは標準出力に `[DUMMY MAIL] to=...` として出力され、`reminder_logs` テーブルにも記録されます。
7. `admin1`「マスタ管理」で勘定科目・部門・ユーザーを追加/更新/削除 →「監査ログ」で操作前後の JSON・IP・User-Agent を確認、「改ざん検証」でハッシュチェーンを検証
8. RBAC の確認: `employee1` のセッションで `GET /api/users` 等の管理APIを直接叩くと **403 Forbidden**

```bash
# curl での RBAC 確認例
curl -s -c /tmp/c.txt -H 'Content-Type: application/json' \
  -d '{"userId":"employee1","password":"Passw0rd!"}' http://localhost:8080/api/auth/login >/dev/null
curl -s -o /dev/null -w '%{http_code}\n' -b /tmp/c.txt http://localhost:8080/api/users   # => 403
```

---

## 4. テスト・静的解析の実行手順

| コマンド | 内容 |
| --- | --- |
| `make lint` | **Backend**: `go vet` + golangci-lint（errcheck, govet, staticcheck, unused, gosec, revive, errorlint, gocritic, sqlclosecheck, rowserrcheck, bodyclose, noctx, unparam ほか / 設定: `backend/.golangci.yml`）<br>**Frontend**: ESLint（`typescript-eslint` strictTypeChecked + react-hooks）＋ Prettier チェック |
| `make typecheck` | `tsc -b --noEmit`（strict, `noUncheckedIndexedAccess`） |
| `make test` | **Backend**: `go test -race ./...`（ドメイン単体・ストア・サービス統合・HTTP API 統合）<br>**Frontend**: Vitest + Testing Library（ドメインヘルパ、APIクライアント、コンポーネント、ルーティング/権限） |
| `make coverage` | 両方のカバレッジ |
| `make e2e` | Playwright。フロントをビルドし、Go サーバーを**新規シード済みDB**で起動して Chromium で実行（`frontend/e2e/`） |
| `make check` | 上記すべて（CI と同等） |

CI（`.github/workflows/ci.yml`）では backend / frontend / e2e の3ジョブで同じチェックを実行します。

### 主なテスト観点
- **状態遷移**（`backend/internal/domain/status_test.go`, `policy_test.go`）: 7つの許可遷移の正常系、5ステータス×5操作の全組み合わせで許可遷移がちょうど7つであること、`REJECTED → SETTLED` / `DRAFT → SETTLED` / `SUBMITTED → SETTLED` の拒絶、終端状態、ロール×部門×本人の権限判定
- **業務ルール**（`validation_test.go`）: 0円以下の提出不可、30,000円ちょうどで49文字NG/50文字OK、空白水増し不可、科目上限・無効科目、コメント必須
- **トランザクション整合性**（`service_test.go`）: 失敗した遷移で監査ログ・履歴・ステータスが一切変わらないこと、承認履歴と監査ログの記録
- **RBAC**（`httpapi/server_test.go`）: Employee/Manager で全管理APIを直接叩いて 403、未認証は 401、他部門 Manager の承認は 403、不正遷移は 409、クロスオリジンPOSTは 403
- **監査ログ**（`store_test.go`）: UPDATE/DELETE がトリガーで拒否されること、ハッシュチェーンで改ざん行を検出すること
- **E2E**: 申請→業務ルールエラー→修正提出→他部門長403→承認（コメント必須）→精算→帳票/PDF→CSV、差戻し後の精算拒否、ログイン失敗の監査、管理画面/APIの403、マスタ管理＋監査ログ＋改ざん検証、リマインドバッチ

---

## 5. アーキテクチャ

```
backend/
  cmd/expense/            エントリポイント: serve / seed / remind サブコマンド
  internal/domain/        純粋なドメインロジック（DB・HTTP 非依存）
    status.go               状態機械（許可遷移テーブル）
    policy.go               権限判定（閲覧・編集・遷移の可否、AllowedActions）
    role.go                 ロールとパーミッション
    validation.go           業務ルール（提出時/下書き時のバリデーション）
  internal/store/         SQLite リポジトリ、埋め込みマイグレーション、監査ログ（ハッシュチェーン）
  internal/service/       ユースケース。1操作 = 1トランザクション（変更＋承認履歴＋監査ログ）
  internal/httpapi/       REST API、認証ミドルウェア、エラー→HTTPステータス変換、SPA 配信
  internal/seed/          初期データ（サービス層経由で投入）
frontend/
  src/api/                型付き API クライアント
  src/domain/             表示ラベル・書式・入力ヒント（純関数、ユニットテスト対象）
  src/components, pages/  画面。操作ボタンはサーバーが返す allowedActions のみを描画
  e2e/                    Playwright
```

### ドメインロジックの分離
- 状態遷移と権限判定は `internal/domain` に集約し、サービス層は必ず `domain.AuthorizeTransition` を通してから更新します。
- API は申請詳細に `allowedActions` / `canEdit` を含めて返し、**フロントはそれを描画するだけ**（ロール・ステータスによる分岐をUIに持たない）。フロントの30,000円ルール等は即時フィードバック用のヒントで、最終判定は常にサーバーです。
- 画面のメニュー表示制御は `/api/auth/me` が返す permissions に基づく表示上の制御で、API側でも同じ権限を必ず検査します。

### 状態機械
```
          submit               approve                    settle
  DRAFT ────────────▶ SUBMITTED ─────────▶ APPROVED_BY_MGR ─────────▶ SETTLED
    ▲                  │    │                 │       │
    │     withdraw     │    │ reject          │       │ reject
    ├──────────────────┘    ▼                 │       ▼
    │                    REJECTED ◀───────────┼───────┘
    └─────────────────── withdraw ────────────┘
```

| 操作 | 遷移 | 実行可能者 |
| --- | --- | --- |
| submit（提出） | DRAFT → SUBMITTED | 申請者本人（業務ルール検証あり） |
| withdraw（取下げ） | SUBMITTED / APPROVED_BY_MGR → DRAFT | 申請者本人 |
| approve（一次承認） | SUBMITTED → APPROVED_BY_MGR | 申請者と**同じ部門**の Manager（本人以外）、コメント必須 |
| reject（差戻し） | SUBMITTED → REJECTED | 同部門 Manager（本人以外）、コメント必須 |
| reject（差戻し） | APPROVED_BY_MGR → REJECTED | Admin（本人以外）、コメント必須 |
| settle（精算確定） | APPROVED_BY_MGR → SETTLED | Admin（本人以外）、コメント必須 |

上記以外の組み合わせはすべて **409 Conflict**、ロール/担当外は **403 Forbidden**。ステータス更新は `WHERE status = <現在値>` の比較更新で、同時操作による二重遷移も検出します。

### RBAC
| 機能 | Employee | Manager | Admin |
| --- | :-: | :-: | :-: |
| 自身の申請の作成・下書き保存・提出・取下げ・照会 | ✓ | ✓ | ✓ |
| 自部門メンバーの申請の照会・一次承認・差戻し | | ✓ | |
| 全申請の照会 / 最終承認（精算確定）・差戻し | | | ✓ |
| マスタ管理 / 監査ログ照会 / CSV出力 / バッチ手動実行 | | | ✓ |

### トランザクションと監査ログ
- 申請の作成・更新（ヘッダ＋明細の洗い替え）、状態遷移（ステータス＋承認履歴）、マスタCRUD は、**監査ログの追記を含めて同一トランザクション**で実行します。どこかで失敗すれば全てロールバックされます。
- 記録項目: タイムスタンプ、実行ユーザーID、操作種別、対象リソース（種別/ID）、操作前JSON、操作後JSON、IPアドレス、User-Agent
- 記録対象: ログイン成功/失敗・ログアウト、申請の作成/更新/提出/承認/差戻し/取下げ/精算確定、マスタの作成/更新/削除、バッチ実行、CSV出力
- 改ざん対策:
  1. SQLite トリガーで `audit_logs` への UPDATE/DELETE を拒否（追記専用）
  2. 各行が「直前行のハッシュ＋自身の内容」の SHA-256 を持つハッシュチェーン。DBファイルを直接書き換えても「改ざん検証」（`GET /api/admin/audit-logs/verify`）で検出

### その他の設計
- 認証: bcrypt でハッシュ化したパスワード、ランダムトークンのセッション（DBにはトークンのSHA-256のみ保存）、`HttpOnly` + `SameSite=Strict` Cookie。状態変更リクエストは Origin ヘッダ検査で CSRF を二重に防止
- ユーザー存在有無がレスポンス時間から推測されないよう、未知ユーザーでもダミーハッシュと比較
- 領収書: 任意のファイル（最大5MB）を DB に保存。ダウンロードはアップロード者、またはその申請を閲覧できるユーザーのみ
- CSV: 精算日（Asia/Tokyo）で期間指定、明細1行=1レコード。表計算ソフトの数式インジェクション対策として `= + - @` 始まりの摘要をエスケープ
- 日時はUTCで保存し、帳票・CSV・画面は Asia/Tokyo で表示

### 主な API
| Method | Path | 権限 |
| --- | --- | --- |
| POST | `/api/auth/login`, `/api/auth/logout` / GET `/api/auth/me` | - / ログイン |
| GET | `/api/expenses?scope=mine\|approvals\|all&status=` | ログイン（`all` は Admin） |
| POST / GET / PUT | `/api/expenses`, `/api/expenses/{id}` | 申請者（閲覧は担当者も） |
| POST | `/api/expenses/{id}/{submit\|withdraw\|approve\|reject\|settle}` | 状態機械＋権限判定 |
| POST / GET | `/api/attachments`, `/api/attachments/{id}` | ログイン |
| GET | `/api/accounts`, `/api/departments` | ログイン（参照用） |
| POST/PUT/DELETE | `/api/accounts…`, `/api/departments…`, `/api/users…`（GET `/api/users` 含む） | Admin |
| GET | `/api/admin/audit-logs`, `/api/admin/audit-logs/verify` | Admin |
| GET | `/api/admin/exports/settled.csv?from=YYYY-MM-DD&to=YYYY-MM-DD` | Admin |
| POST / GET | `/api/admin/batch/reminders` | Admin |

---

## 6. 仕様の解釈・前提

- **差戻し（REJECTED）は終端状態**としました。仕様の遷移表に REJECTED からの遷移が無いため、厳密に従っています（再申請は新規作成）。
- **一次承認は申請者と同じ部門の Manager**、最終承認（精算確定）と一次承認後の差戻しは Admin が行います。Admin は一次承認を代行できません。
- **自己承認の禁止**: 自分の申請を自分で承認・差戻し・精算確定することはできません。
- **コメント必須**は承認・差戻しに加え、最終承認である精算確定にも適用しています。
- **0円以下は提出不可**は合計金額で判定し、加えて各明細も1円以上を必須としています。下書き保存時は構造チェック（日付形式・科目存在・負数禁止・文字数上限）のみで、業務ルールは提出時に検証します。
- **摘要50文字**は前後の空白を除いた文字数（Unicode コードポイント数）で判定します。
- 勘定科目の**上限金額**は1明細あたりの上限として提出時に検証し、**無効な科目**は提出不可です。使用中の科目・所属者のいる部門・申請のあるユーザーは削除できません（科目は無効化で対応）。
- PDF は「印刷用HTMLレイアウト」方式です（ブラウザの印刷→PDF保存。E2E で `page.pdf()` による生成も確認）。
- バッチは「最終ステータス変更から7日以上」経過した SUBMITTED / APPROVED_BY_MGR を対象に、SUBMITTED は同部門の Manager、APPROVED_BY_MGR は Admin 宛てのリマインドを記録します（定期実行は cron 等から `expense remind` を呼び出す想定）。
