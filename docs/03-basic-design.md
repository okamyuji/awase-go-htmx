# awase 基本設計書

この文書は2026-10-31に作成し、確定版としています。要件の出典は`02-requirements-specification.md`です。

## 1 決定事項の一覧

| 決定 | 検討した案 | 採用 | 理由 | ADR |
|---|---|---|---|---|
| DBドライバ | `modernc.org/sqlite`、`mattn/go-sqlite3` | `modernc.org/sqlite` | CGOなしでarm64向けにビルドでき、同居するboki3と同じ方式になります | ADR-0001 |
| スキーマの適用 | 起動時の埋め込みSQL、外部ツール | 起動時の埋め込みSQL | 単一バイナリで完結し、運用手順が増えません | ADR-0001 |
| 認証方式 | 共有アカウント、個人アカウント、Cloudflare Access | 共有アカウント | 利用者が資格情報をLINEで共有する運用を選びました | ADR-0002 |
| 共有アカウントの設定 | 初回設定画面、環境変数、CLI | `awase passwd`コマンド | 公開直後に第三者が初回設定画面を先に使う危険がありません | ADR-0002 |
| scryptの同時実行 | 制限なし、同時2件まで、同時1件まで | 同時1件まで | 1回の照合で32MiBを使います。128MiBの上限のコンテナで同時2件にすると、10件のログインでOOMにより停止する場合がありました | ADR-0002 |
| 再設定とログインの競合 | 対策なし、セッションにアカウントの版を持たせる | アカウントの版を持たせる | 利用者が対策を選びました | ADR-0002 |
| 別オリジンからの要求の拒否 | トークン方式、標準ライブラリの`http.CrossOriginProtection` | `http.CrossOriginProtection` | Go 1.25以降の標準機能で、トークンの発行と照合が要りません | ADR-0002 |
| 画面の作り | 部分更新を多用、`hx-boost`で全体を差し替え | `hx-boost`で全体を差し替え | 画面ごとのテンプレートだけで済み、JavaScriptなしでも動きます | ADR-0003 |
| htmxの版 | 1系、2.0.11 | 2.0.11 | 応答コードごとの扱いを決める`responseHandling`は2.0で入った設定です | ADR-0003 |
| 層の分け方 | 単一パッケージ、インターフェースで分けた層 | インターフェースで分けた層 | 利用者の指定です。テストで層ごとに差し替えられます | ADR-0003 |
| 配備先 | 専用EC2、feedflowのEC2に同居 | feedflowのEC2に同居 | 利用者の指定です。boki3が同じ方式で動いています | ADR-0004 |
| 入口 | 専用のnginx、feedflowのnginxへ設定を追加 | feedflowのnginxへ`zz-awase.conf`を追加 | 80番と443番はfeedflowのnginxが使っています。名前順で最後に読ませ、既定のサーバーを変えません | ADR-0004 |
| terraformの管理範囲 | EC2を含む全体、Cloudflareと配送だけ | Cloudflareと配送だけ | EC2の状態を2つのterraformで持つと互いに上書きします | ADR-0004 |
| バックアップ | EBSのスナップショット、アプリ内のコピー、取らない | 取らない | 利用者の判断です。日程調整のデータは短期間で使い終わります | ADR-0004 |
| Cloudflareのガード | Access、WAFだけ | WAFだけ | 利用者の指定です。仲間のメールアドレス登録が要りません | ADR-0005 |
| WAFのルールセットの持ち主 | アプリごと、awaseのterraform | awaseのterraform | ゾーンのルールセットは段階ごとに1つしか作れません | ADR-0005 |
| 品質ゲート | 既存のスクリプトを流用、新規に作成 | event-streaming-valkeyのスクリプトを流用 | CRAP値とミューテーションの判定を実績のある手順で行えます | ADR-0006 |
| ミューテーションテストの道具 | gremlins v0.6.0、go-mutesting | gremlins v0.6.0 | 開発用のmacOSに導入済みで、他のリポジトリでも使っています | ADR-0006 |
| ミューテーションの作業領域 | リポジトリの中、リポジトリの外 | リポジトリの外の固定パス | gremlinsはモジュール全体を作業領域へ複製するため、作業領域がモジュールの中にあると自分自身を複製し続けます | ADR-0006 |
| Goの版 | 1.26、1.27 | go.modに`go 1.27.0`、CIは`1.27.x`の最新 | Dependabotの依存グラフの更新はGo 1.27.0で動き、go.modがそれより新しいパッチを要求するとツールチェーンの取得で止まります。CIとDockerfileは1.27系の最新のパッチを使います | なし |
| ホスト名 | 自由 | `awase.okamyuji.work` | feedflowとboki3と同じゾーンに置きます | ADR-0004 |

## 2 構成

```text
[ブラウザ]
   | HTTPS
   v
[Cloudflareエッジ] 日本以外を遮断 / /loginの連続要求を遮断 / オリジンIPを秘匿
   | HTTPS (セキュリティグループはCloudflareのIP範囲だけを通します)
   v
[EC2 t4g] feedflowのcomposeが持つnginx
   |  awase.okamyuji.work  -> awase-app:8080 (要求時に名前解決します)
   |  feedflow.okamyuji.work -> app:8080
   |  boki3のホスト名 -> boki3-app:8080
   v
[awase-app] Goの単一バイナリ / テンプレートと静的ファイルとスキーマを埋め込み
   |
[SQLite] /mnt/feedflow-data/awase/awase.db (EBS)
```

## 3 パッケージと層

| パッケージ | 役割 | 依存してよい先 |
|---|---|---|
| `internal/domain` | エンティティ、入力の検証、集計 | 標準ライブラリだけ |
| `internal/port` | リポジトリとサービスのインターフェース、時計、ID生成、パスワードハッシュ | `domain` |
| `internal/service` | ユースケース。イベント、回答、認証 | `domain`、`port` |
| `internal/store` | `port`のリポジトリをSQLiteで実装し、スキーマを適用します | `domain`、`port` |
| `internal/auth` | scryptのパスワードハッシュ、ログイン失敗の制限 | `port` |
| `internal/handler` | HTTPのハンドラ、ミドルウェア、テンプレートの描画 | `domain`、`port` |
| `web` | テンプレートと静的ファイルの埋め込み | なし |
| `cmd/awase` | 設定の読み込み、各層の組み立て、`serve`と`passwd`の実行 | すべて |

ハンドラは`port`のサービスのインターフェースだけを受け取り、サービスは`port`のリポジトリのインターフェースだけを受け取ります。具体的な型を組み立てるのは`cmd/awase`だけです。

## 4 ドメインモデル

### 4.1 エンティティ

| 型 | 項目 |
|---|---|
| `Event` | ID、タイトル、メモ、候補の並び、作成日時、更新日時 |
| `Candidate` | ID、表示順、ラベル |
| `Answer` | ID、イベントID、名前、コメント、候補IDごとの出欠、作成日時、更新日時 |
| `Choice` | `No`(×)、`Maybe`(△)、`Yes`(○)の3値 |
| `Tally` | 候補ごとの○、△、×の人数と、強調するかどうか |

### 4.2 規則

- タイトル、メモ、候補、名前、コメントは前後の空白を取り除いた値を検証し、その値を保存します。
- 候補の入力は改行で分け、各行の前後の空白を取り除き、空行を捨てます。
- 入力の制約は要件定義書の3章に従います。`domain`の検証関数は、違反した項目名と理由の組をすべて返します。
- 候補を追加するときは、既存の候補と追加分の合計が50件以下であることを確かめます。
- 候補が1件だけのイベントでは、その候補を削除できません。
- 回答を保存するときは、送られた候補IDの集合が、そのイベントの候補IDの集合と等しいことを確かめます。等しくない場合は422を返して保存しません。この検証で、すべての候補に出欠が付いていることと、別のイベントの候補が混ざっていないことを同時に確かめます。
- 集計は、○の人数が最大の候補をすべて強調します。最大値が0の場合はどの候補も強調しません。
- 回答に出欠のない候補は未回答として数えず、画面に「−」を出します。

### 4.3 識別子

- イベントIDは`crypto/rand.Text()`で作ります。URLに出るため推測できない値にします。
- 候補と回答はSQLiteの整数の主キーを使います。経路では必ずイベントIDと組にして検索し、別のイベントの候補や回答を操作できないようにします。

## 5 データ

日時はUTCのUNIX秒で保存し、画面では日本時間で表示します。接続ごとに外部キー制約、WALモード、5秒のビジー待ちを有効にします。

```sql
CREATE TABLE account (
  id            INTEGER PRIMARY KEY CHECK (id = 1),
  username      TEXT    NOT NULL,
  password_hash TEXT    NOT NULL,
  version       INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL
);

CREATE TABLE sessions (
  token_hash      BLOB    PRIMARY KEY,
  account_version INTEGER NOT NULL,
  created_at      INTEGER NOT NULL,
  expires_at      INTEGER NOT NULL
);

CREATE TABLE events (
  id         TEXT    PRIMARY KEY,
  title      TEXT    NOT NULL,
  memo       TEXT    NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE candidates (
  id       INTEGER PRIMARY KEY,
  event_id TEXT    NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  position INTEGER NOT NULL,
  label    TEXT    NOT NULL
);

CREATE TABLE answers (
  id         INTEGER PRIMARY KEY,
  event_id   TEXT    NOT NULL REFERENCES events(id) ON DELETE CASCADE,
  name       TEXT    NOT NULL,
  comment    TEXT    NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL
);

CREATE TABLE choices (
  answer_id    INTEGER NOT NULL REFERENCES answers(id) ON DELETE CASCADE,
  candidate_id INTEGER NOT NULL REFERENCES candidates(id) ON DELETE CASCADE,
  value        INTEGER NOT NULL CHECK (value IN (0, 1, 2)),
  PRIMARY KEY (answer_id, candidate_id)
);

CREATE INDEX candidates_event ON candidates(event_id, position);
CREATE INDEX answers_event ON answers(event_id, id);
```

- `account`は1行だけを持ちます。`awase passwd`は同じトランザクションで`account`を置き換えて`version`を1増やし、`sessions`を全件削除します。
- セッションは、作成時のアカウントの版を`account_version`に持ちます。認証ミドルウェアは、`account_version`が現在の`account.version`と等しいセッションだけを有効とします。ログインの照合中に`awase passwd`が確定した場合も、照合前に読んだ版でセッションを作るため、そのセッションは無効になります。
- 回答の追加、変更、削除と候補の追加、削除は、同じトランザクションでイベントの`updated_at`を更新します。
- 期限切れのセッションは、ログインのたびに削除します。

## 6 画面と経路

すべての経路はGo 1.22以降の`http.ServeMux`のメソッド付きパターンで登録します。状態を変える要求はすべてPOSTで受け、成功したら303で表示用の経路へ移動させます。

| メソッドと経路 | 画面または処理 | 要件 |
|---|---|---|
| `GET /login` | ログイン画面 | FR-01 |
| `POST /login` | ログイン | FR-01、FR-03 |
| `POST /logout` | ログアウト | FR-02 |
| `GET /{$}` | イベント一覧 | FR-05 |
| `GET /events/new` | イベント作成画面 | FR-06 |
| `POST /events` | イベント作成 | FR-06 |
| `GET /events/{id}` | イベント表示と回答の入力欄 | FR-07、FR-08 |
| `GET /events/{id}/edit` | イベント編集画面 | FR-11 |
| `POST /events/{id}` | タイトルとメモの変更 | FR-11 |
| `POST /events/{id}/candidates` | 候補の追加 | FR-11 |
| `POST /events/{id}/candidates/{cid}/delete` | 候補の削除 | FR-11 |
| `POST /events/{id}/delete` | イベントの削除 | FR-12 |
| `POST /events/{id}/answers` | 回答の追加 | FR-08 |
| `GET /events/{id}/answers/{aid}/edit` | 回答の編集画面 | FR-09 |
| `POST /events/{id}/answers/{aid}` | 回答の変更 | FR-09 |
| `POST /events/{id}/answers/{aid}/delete` | 回答の削除 | FR-10 |
| `GET /healthz` | 稼働確認。認証なしで`ok`を返します | NFR-09 |
| `GET /static/` | 埋め込みの静的ファイル | なし |

### 6.1 htmxの使い方

- htmxは2.0.11を使います。npmは使わず、配布ファイルを`web/static`に置いて埋め込みます。
- `body`に`hx-boost="true"`を付け、リンクとフォームをhtmxの要求に変えます。サーバーは常に画面全体のHTMLを返します。JavaScriptが動かない場合も、通常のリンクとフォームとして同じ経路が動きます。
- htmxは既定でインラインの`style`要素を挿入し、7.4節のCSPに違反します。`htmx-config`で`includeIndicatorStyles`を`false`にし、読み込み中の表示は`web/static`のCSSで定義します。
- htmxは既定で4xxと5xxの応答を画面に反映しません。`htmx-config`の`responseHandling`で4xxと5xxも差し替える設定にし、入力エラーの422、ログイン制限の429、存在しないイベントの404を利用者に見せます。

### 6.2 表の配置

- PCの幅では、候補を行、回答者を列とする表を出します。
- スマートフォンの幅では、表を横にスクロールできるようにし、候補の列と人数の列を左側に固定します。
- 寸法はremで指定し、固定の最大幅を使いません。

## 7 認証とセキュリティ

### 7.1 共有アカウント

- `awase passwd -username <名前>`は標準入力の1行目をパスワードとして読みます。行末の改行(LFとCRLF)だけを取り除き、空白は取り除きません。
- ユーザー名は前後の空白を取り除いて1〜64文字とし、取り除いた値を保存します。パスワードは12文字以上とします。文字数はUnicodeのコードポイントで数えます。違反した場合、コマンドは理由を出して失敗し、DBを変更しません。
- パスワードは`golang.org/x/crypto/scrypt`でハッシュ化します。パラメータはN=32768、r=8、p=1とし、16バイトのソルトと一緒に1つの文字列へ符号化して保存します。
- ログインのパスワードは入力された値をそのまま照合します。ユーザー名は前後の空白を取り除いて照合します。
- ユーザー名の照合は`crypto/subtle.ConstantTimeCompare`で行います。ユーザー名が違う場合とアカウントが未設定の場合も、固定のダミーのハッシュに対してscryptを計算し、同じ失敗の応答を返します。
- scryptの計算は、容量1のチャネルで同時に1件までに制限します。2件目以降の照合は、空きが出るまで待ちます。
- scryptだけを動かすプログラムで、10件のログインを同時に送った場合のメモリを測りました。条件は、Linuxのコンテナを`--memory=128m --memory-swap=128m`で動かし、cgroupの`memory.peak`を観測するというものです。
  - 同時1件では、`GOMEMLIMIT=96MiB`の有無にかかわらず、ピークは約67〜71MiBでした。
  - 同時2件では、OOMで停止する場合と、ピークが約110MiBになる場合がありました。
- 8.2節の`GOMEMLIMIT=96MiB`は、HTTPとSQLiteの分を含めたヒープに余裕を持たせるために付けます。

### 7.2 セッション

- ログインに成功すると、`crypto/rand.Text()`でトークンを作り、Cookie`awase_session`に入れます。DBにはトークンのSHA-256と、照合に使ったアカウントの版を保存します。
- Cookieには`HttpOnly`、`SameSite=Lax`、`Path=/`、`Secure`、30日の`Max-Age`を付けます。`Secure`は既定で付け、環境変数`AWASE_COOKIE_SECURE=false`のときだけ外します。この指定はローカル環境での開発に使うもので、本番の設定には書きません。
- 認証ミドルウェアは`/login`、`/healthz`、`/static/`以外のすべての経路で、Cookieのトークンを照合します。照合できない場合はログイン画面へ303で移動させます。要求がGETの場合だけ、元のパスとクエリを`next`に入れます。POSTの経路をGETで開き直すと405になるため、POSTでは`next`を付けません。
- `next`は、クエリとフォームを復号した後の文字列で判定します。次の条件をすべて満たす値だけを受け付け、満たさない場合はイベント一覧へ移動させます。
  - `url.Parse`が成功します。この関数は制御文字を含む値を失敗にします。
  - 解析結果のSchemeとHostが空です。
  - 元の文字列が`/`で始まり、`//`で始まらず、`\`を含みません。

### 7.3 ログイン失敗の制限

- `auth`パッケージがIPアドレスごとの失敗回数をメモリに持ちます。失敗が15分間に10回に達したIPアドレスは、その時点から15分間ログインを拒否されます。拒否中の`POST /login`には資格情報を照合せずに429を返します。`GET /login`は回数に数えず、拒否もしません。
- 試行の判定と加算は、照合の前に同じロックの中で行います。同時に送られた試行も、上限を超えて照合されません。成功した試行は記録から消します。
- 利用者の数が少ないため、保持はプロセス内の`map`とミューテックスで行い、照合のたびに期限切れの項目を捨てます。プロセスを再起動すると回数は消えます。
- IPアドレスは、nginxが設定する`X-Real-IP`から取ります。nginxはCloudflareの`CF-Connecting-IP`から実際の接続元を復元しています。awase-appはホストにポートを公開しないため、nginxを経由しない要求は届きません。`X-Real-IP`がない場合は`RemoteAddr`を使います。

### 7.4 その他の対策

- 状態を変える要求は`http.CrossOriginProtection`で包み、別オリジンからの要求に403を返します。
- アプリは応答に`Content-Security-Policy: default-src 'self'`、`X-Content-Type-Options: nosniff`、`Referrer-Policy: same-origin`、`X-Frame-Options: DENY`を付けます。nginxはこれらを重ねて付けず、`Strict-Transport-Security`だけを付けます。重ねると、`Referrer-Policy`は後ろの値だけが使われ、CSPは両方が適用されるため、アプリが決めた方針と違う結果になります。
- htmxの設定は`meta`要素で渡し、インラインのスクリプトを使いません。
- テンプレートは`html/template`で描画し、SQLはすべてプレースホルダで組み立てます。
- 秘密情報は持ちません。セッションはDBで管理するため、署名用の鍵が要りません。
- ログは`log/slog`のJSON形式で標準出力に出します。パスワード、Cookie、トークンはログに出しません。

## 8 配備

### 8.1 前提

feedflowのterraformが作った次の資産を使います。awaseはこれらを変更せず、自分の資産を追加するだけにします。

- EC2とElastic IP、Cloudflareに限定したセキュリティグループ
- composeプロジェクト`feedflow`と、そのネットワーク`feedflow_internal`
- `/etc/feedflow/conf.d`を読み込むnginxと、`/etc/feedflow/tls`の証明書置き場
- `/mnt/feedflow-data`にマウントしたデータ用のEBS

2026-10-31にホストで確認した値は次のとおりです。メモリは916MiBで、利用可能な量は537MiBでした。ネットワーク`feedflow_internal`が存在し、conf.dには`feedflow.conf`と`boki3.conf`がありました。

### 8.2 コンテナ

- サービス名は`awase-app`とします。同じネットワークにいるfeedflowの`app`とboki3の`boki3-app`と名前が重ならないようにするためです。
- 実行イメージはdistrolessのnonroot(uid 65532)にし、バイナリだけを置きます。
- メモリの上限を128MiBとし、環境変数`GOMEMLIMIT`を96MiBにします。上限を超えた場合、停止するのはawase-appだけです。
- `restart: unless-stopped`を指定し、メモリ不足などで停止した場合はDockerが再起動します。
- ホストへはポートを公開せず、`feedflow_internal`へ外部ネットワークとして参加します。
- データは`/mnt/feedflow-data/awase`を`/data`へマウントし、DBのパスは`/data/awase.db`とします。

### 8.3 nginxの設定

`zz-awase.conf`を`/etc/feedflow/conf.d`へ追加します。nginxはconf.dをファイル名の順に読み、どのサーバー名にも一致しない要求を最初に読んだサーバーへ渡します。`zz-`を付けて最後に読ませることで、既定のサーバーを変えません。内容はboki3の設定と同じ形にします。

- 上流は`resolver 127.0.0.11 valid=30s;`と変数付きの`proxy_pass http://$awase_upstream;`で要求時に名前解決します。awase-appが止まっていてもnginxは起動とreloadに成功し、awaseのホスト名だけが502を返します。
- `real_ip_header`はfeedflow.confがhttpコンテキストで宣言しているため、`zz-awase.conf`では`server`コンテキストの中にだけ書きます。
- 付けるセキュリティヘッダは`Strict-Transport-Security`だけです。
- 証明書は`/etc/feedflow/tls/awase.crt`と`awase.key`に置きます。feedflowの証明書は使いません。

### 8.4 配送の手順

terraformの`null_resource`がSSHで次の順に実行します。どこかで失敗した場合、手順はそこで止まり、feedflowのnginxは直前の設定のまま動き続けます。

1. feedflowの`nginx`コンテナが稼働中で、ネットワーク`feedflow_internal`が存在することを確かめます。
2. `/mnt/feedflow-data/awase`を作り、所有者を`65532:65532`にします。
3. ローカルでビルドしたarm64のイメージを`docker load`で取り込みます。EC2の上ではビルドしません。メモリを使い切って同居するアプリを止めないためです。
4. `/home/ec2-user/awase`でawase-appを起動します。
5. 既存の`zz-awase.conf`、`awase.crt`、`awase.key`を退避します。
6. 新しい証明書、鍵、`zz-awase.conf`を置き、鍵の権限を600にします。
7. `nginx -t`を実行します。失敗した場合は、退避した3つのファイルを戻します。退避がないファイルは消します。そのうえでreloadせずに失敗で終わります。
8. `nginx -s reload`を実行し、退避したファイルを消します。
9. awase-appのコンテナのIPアドレスを`docker inspect`で調べ、ホストからそのアドレスの`/healthz`へ接続して応答を確かめます。awase-appはポートを公開していないためです。

手順6の後、手順7の前にSSHの接続が切れると、検証していない設定と証明書が残ります。設定が壊れていれば、次にnginxが再起動したときにfeedflowも止まります。起こる見込みが低いため、利用者の判断で対策は入れません。配送が失敗した場合は、`terraform apply`を再実行して退避と検証からやり直します。

### 8.5 Cloudflare

awaseのterraformは次のCloudflareの資産を持ちます。ゾーンのSSLモードはfeedflowのterraformの管理下にあるので、awaseの側では触れない方針です。

- Aレコード `awase.okamyuji.work`をfeedflowのElastic IPへ向け、プロキシを有効にします。
- Origin CA証明書 `awase.okamyuji.work`用に発行します。
- カスタムルール 式`(http.host eq "awase.okamyuji.work" and not ip.src.country in {"JP"})`に一致した要求をblockします。
- レート制限ルール 式`(http.request.uri.path eq "/login")`に一致した要求をIPアドレスごとに数え、10秒間に10回を超えると続く10秒間blockします。

Freeプランのレート制限で式に使える項目は、パスとVerified Botだけです。そのため、このルールはfeedflowとboki3の`/login`への要求も同じ条件で遮断します。通常のログインは10秒間に10回の要求に届かないので、他のアプリの利用者がログインを拒まれることはありません。Cloudflareのレート制限はデータセンターごとに数え、数秒の遅れがあるため、遮断が始まる回数は厳密ではありません。

ゾーンのルールセットは段階ごとに1つしか作れません。2026-10-31にAPIで確かめたところ、カスタムルールの段階にはAI Crawl Controlが作ったルールのルールセットがあり、レート制限の段階にはルールセットがありませんでした。そのため、2つの段階を次のように扱います。

- レート制限 awaseのterraformが、ゾーンで共有する資産としてルールセットを持ちます。この扱いはADR-0004の管理範囲の例外です。
- カスタムルール ルールセットごとは持たず、awaseの国別遮断のルールだけを`deploy/scripts/cf-country-rule.sh`がルール単位のAPIで追加と削除をします。terraformは`null_resource`からこのスクリプトを呼びます。AI Crawl Controlのルールには触れません。

- P5の最初に、Cloudflare APIで2つの段階に既存のルールセットがあるかを確かめます。Freeプランのレート制限は1ルールまでなので、レート制限の段階に既存のルールが1件でもあれば、awaseのルールを足せません。その場合は作業を止め、利用者に判断を求めます。
- 後からfeedflowやboki3がレート制限のルールを足す場合は、awaseのterraformの定義に加えます。
- APIトークンには、feedflowのトークンの権限に加えて、ゾーンのWAFのルールセットを編集する権限が要ります。権限名はP5でCloudflareの文書と実際の`terraform plan`で確かめます。

### 8.6 運用

- 共有アカウントは、ホストの`/home/ec2-user/awase`で`sudo docker compose exec -T awase-app /awase passwd -username <名前>`を実行し、標準入力でパスワードを渡して設定します。
- バックアップは取りません。EBSが壊れた場合や、feedflowのterraformで`destroy`を実行してEC2とEBSが消えた場合は、awaseのデータも失われます。
- awaseのterraformで`destroy`を実行すると、次の順に取り除きます。取り除く手順は、配送の`null_resource`とは別の`null_resource`に置きます。配送のリソースは再配備のたびに作り直され、そのたびにdestroy時のプロビジョナが走るためです。取り除く手順のリソースは、接続先とSSH鍵のパスだけを`triggers`に持ち、プロビジョナからは`self.triggers`だけを参照します。配送のリソースは`depends_on`で取り除く手順のリソースに依存させます。この向きにすると、取り除く手順のリソースは配送より先に作られるので、初回の配送が失敗してもstateに残り、destroyで手順が実行されます。このリソースは作成時に何も実行しないため、配送の失敗で汚染された状態にもなりません。配送のリソースの`triggers`にも同じ接続先と鍵のパスを入れます。接続先や鍵のパスが変わって取り除く手順が走った場合に、配送も続けて実行され、awaseが外れたままにならないようにするためです。各手順は、ファイルやコンテナがすでにない状態で再実行しても成功するように書きます。
  1. `zz-awase.conf`を退避してから消します。
  2. `nginx -t`を実行します。失敗した場合は退避した設定を戻し、失敗で終わります。
  3. `nginx -s reload`を実行します。
  4. 証明書と鍵を消し、awase-appのコンテナを止めて消します。
  5. データの`/mnt/feedflow-data/awase`は消さずに残します。
- `destroy`はレート制限のルールセットを消し、カスタムルールからawaseのルールだけを消します。他のアプリがレート制限のルールセットに加えたルールも一緒に消えます。

## 9 品質ゲート

| 道具 | 内容 | 実行の契機 |
|---|---|---|
| `scripts/quality-gate.sh` | gofmt、`go fix -diff`、go vet、staticcheck、golangci-lint、govulncheck、`go test -race -shuffle=on`、カバレッジ80%以上、CRAP値、ミューテーションの残骸の検出、ビルド、gitleaks、doclint | pre-commitと手動 |
| `scripts/crap.sh` | 全関数のCRAP値を計算し、15以上があれば失敗します | 品質ゲート |
| `scripts/mutation.sh` | gremlins v0.6.0を実行し、KILLEDと変異後にビルドできないもの以外が1件でもあれば失敗します | 手動。実装の最後に実行します |
| `scripts/check-mutation-residue.sh` | ミューテーションの作業領域と結果ファイルが残っていれば失敗します | 品質ゲートとpre-commit |
| 共通CI | go vet、ビルド、`go test -race -shuffle=on -count=1`、gitleaks | push |

流用元の`crap.sh`はモジュールのパスをevent-streaming-valkeyの名前で直書きしているため、awaseのモジュールのパスに直します。pre-commitのフックはリポジトリに含まれないので、`make precommit-install`で`pre-commit install`を実行します。

### 9.1 ミューテーションの残骸を残さない仕組み

gremlinsは`os.TempDir()`の下に`gremlins-`で始まるディレクトリを作り、モジュール全体をそこへ複製します。複製は除外の条件を持たないため、作業領域をモジュールの中に置くと、gremlinsは作業領域自身を複製し続けて失敗します。`scripts/mutation.sh`は次の手順で動き、複製を必ず消します。

1. 作業領域を`${XDG_CACHE_HOME:-$HOME/.cache}/awase-mutation`に決めて`mkdir -p`で作り、`TMPDIR`をそこへ向けます。gremlinsとテストが作る一時ファイルは、すべてこの中に集まります。
2. 関数`cleanup`を`trap`で`EXIT`に登録します。`cleanup`は作業領域を削除し、削除後に作業領域が残っていれば終了コードを1にします。`INT`、`TERM`、`HUP`の`trap`は、0以外の終了コードで`exit`します。gremlinsは中断されると終了コード0で終わるため、シグナルを受けたスクリプトがその先へ進まないようにします。
3. gremlinsを`GOFLAGS=-count=1`と`--timeout-coefficient 10`を付けて実行し、結果のJSONを作業領域の中に書きます。テストのキャッシュが当たると、gremlinsが時間切れの基準にする初回の計測が短くなり、変異がすべて時間切れになるためです。
4. 結果のJSONをjqで読み、KILLEDと変異後にビルドできないもの以外が1件でもあれば失敗にします。SKIPPEDも失敗として数えます。gremlinsは中断されたときと結果が0件のときにJSONを書かないため、JSONがない場合も失敗にします。gremlinsの効率の閾値は、テストが届かない変異と時間切れの変異を分母に入れないため、この判定には使いません。

`SIGKILL`のように`trap`で受けられない終了の場合、作業領域は残ったままになります。この場合は`scripts/check-mutation-residue.sh`が品質ゲートとpre-commitを失敗させ、`make mutation-clean`を実行するまでコミットできません。この検出は、作業領域と、リポジトリ直下の`gremlins.json`と`gremlins-*`を調べます。

### 9.2 テストの方針

- 機能はテストを先に書いてから実装します。
- `store`のテストは`t.TempDir()`に作った実ファイルのSQLiteで行います。
- `handler`のテストは`httptest`でサービスの偽物を差し込みます。
- `awase passwd`はセッションも全件削除するため、版の判定を実装し忘れても実動作では気づけません。版1のセッションを作り、`account.version`を2にすると拒否されることを単体テストで確かめます。登録したすべての経路を未認証で開き、`/login`、`/healthz`、`/static/`以外がログイン画面への303になることを確かめるテストを置きます。
- 起動処理は`run`関数に分け、`main`は`run`を呼ぶだけにします。`run`は引数と標準入出力を受け取り、テストから呼べるようにします。

## 10 実装フェーズと完了条件

各フェーズの最後に、実際にawaseを動かして結果を観測します。観測した出力を報告に引用します。

| フェーズ | 内容 | 実動作での完了条件 |
|---|---|---|
| P1(骨格) | go.mod、`cmd/awase`、`/healthz`、品質ゲートのスクリプト、Makefile、pre-commit、golangci-lintの設定 | `go run ./cmd/awase`を起動し、`curl`で`/healthz`が200と`ok`を返します。標準出力のログがJSONです。`make precommit-install`の後、作業領域を作ると品質ゲートと`git commit`が失敗し、消すと成功します |
| P2(ドメインと永続化) | `domain`、`port`、`store`、スキーマ、`awase passwd` | `awase passwd`を実行すると、`sqlite3`で全テーブルと`account`の1行が見え、`version`が1です。2回目の実行で`version`が2になります。11文字のパスワード、空のユーザー名、空白だけのユーザー名、65文字のユーザー名では失敗し、`account`は変わりません。前後に空白を付けたユーザー名は、空白を取り除いた値で保存されます。CRLFで終わるパスワードを渡すと、CRを含まない値でハッシュ化されます |
| P3(認証) | ログイン、ログアウト、セッション、ログイン失敗の制限、別オリジンの拒否、セキュリティヘッダ | 起動したawaseに`curl`で次を確かめます。アカウントを設定する前はログインできません。成功のログインが303と`HttpOnly`、`SameSite=Lax`、`Secure`付きのCookieを返します。未認証で`GET /events/{id}`を開いてからログインすると、そのページへ移動します。誤った資格情報では失敗の表示が出ます。10回失敗の後は正しい資格情報でも429です。別の`Origin`のPOSTが403です。応答に7.4節のヘッダが1つずつ付きます。`next`に`https://example.com`と`/%09/example.com`を渡すと一覧へ移動します。未認証の各経路はログイン画面へ移動し、POSTでは`next`が付きません。ログアウト後と`awase passwd`の再実行後は、以前のCookieがログイン画面へ移動させられます。linux/arm64向けにビルドしたバイナリをalpineのコンテナで`--memory=128m --memory-swap=128m`と`GOMEMLIMIT=96MiB`を付けて起動し、誤った資格情報のログインを10件同時に送ります。このとき、`docker exec`で読んだ`/sys/fs/cgroup/memory.peak`が96MiB未満で、`OOMKilled`が`false`です。96MiBを閾値にするのは、同時2件の実装を見分けるためです |
| P4(画面) | イベントと回答のすべての画面 | Playwrightで次を確かめ、PCとスマートフォンの幅で画面を撮ります。ログイン、イベント作成、2名の回答、回答の変更と削除、タイトルとメモの変更、候補の追加と削除、イベントの削除、ログアウトを行い、表と人数と強調表示を確かめます。101文字のタイトルが422と理由の表示になります。合計51件になる候補の追加と、最後の1件の候補の削除が拒否されます。削除したイベントのURLは404です。一覧が新しい順に並び、候補の数、回答の数、最終更新日時が出ます。別のイベントの候補IDを混ぜた回答、候補が欠けた回答、余分な候補を足した回答の送信は422です。別のイベントの候補IDや回答IDを経路に入れた削除と変更は404です。日時が日本時間で表示されます。ローカル環境で一覧とイベントの画面の応答時間を`curl -w`で計り、500ミリ秒以内です |
| P5(配備) | Dockerfile、compose、nginxの設定、terraform | 次を確かめます。`https://awase.okamyuji.work`でログインでき、Cookieに`Secure`が付きます。ホストからawase-appのコンテナのIPアドレスへ、ログインで得たCookieを`Cookie`ヘッダで付けて、一覧とイベントの画面へ5件同時に要求します。応答がすべて200で、応答時間がすべて500ミリ秒以内です。`docker inspect`でメモリ上限が134217728、再起動の方針が`unless-stopped`、環境変数`GOMEMLIMIT`が`96MiB`です。awase-appを止めた状態で、feedflowとboki3のURLが応答し、`nginx -t`が成功します。どのサーバー名にも一致しない要求の応答が、配備の前と同じboki3です。ローカル環境のnginxのコンテナで、壊れた`zz-awase.conf`を置く配送手順を実行すると設定が戻り、destroyの手順を実行すると`nginx -t`が成功します。`GET /login`へ10秒間に続けて要求すると429が返ります。アプリはGETを失敗として数えないため、この429はCloudflareのものです。オリジンのIPアドレスへの直接の接続はタイムアウトします。WAFの2つのルールがCloudflare APIで登録済みです |
| P6(ミューテーション) | ミューテーションテストと仕上げ | `make mutation`を前面で実行し、`scripts/mutation.sh`がすべてのミューテーションをKILLEDと判定し、終了後に作業領域がありません。実行中にプロセスグループへ`INT`を送ると、スクリプトは0以外で終わり、作業領域が残りません |

P5の`terraform apply`はCloudflareと本番ホストを変更します。実行の前に利用者の了承を得ます。日本以外からの遮断は、日本国内からは実際に試せないため、ルールの式がAPIで登録されていることを確かめます。

## 11 要件との対応

| 要件 | 設計 |
|---|---|
| FR-01〜FR-04 | 5章、6章、7章 |
| FR-05〜FR-12 | 4章、5章、6章 |
| NFR-01 | 5章のインデックス、10章のP4とP5の計測 |
| NFR-02 | 7章 |
| NFR-03 | 8.5節 |
| NFR-04 | 7.1節のscryptの制限、8.2節〜8.4節、8.6節 |
| NFR-05 | 2章、8.2節、8.6節 |
| NFR-06 | 9章 |
| NFR-07 | 3章 |
| NFR-08 | 6.2節、5章 |
| NFR-09 | 6章の`/healthz`、7.4節のログ |
