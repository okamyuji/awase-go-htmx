# ADR-0001 SQLiteと`modernc.org/sqlite`で保存する

ステータス 採用

## 状況

awaseのデータは、イベント、候補、回答、共有アカウント、セッションです。利用者は5名程度で、データ量も小さく収まります。利用者はDBにSQLiteを指定しました。配備先のEC2はarm64で、同居するfeedflowとboki3はCGOを使わずにビルドしています。

## 決定

DBドライバに`modernc.org/sqlite`を使います。スキーマのSQLはバイナリに埋め込み、起動時に`PRAGMA user_version`で適用済みの版を確かめてから、未適用のSQLを順に実行します。

## 理由

`modernc.org/sqlite`はGoだけで書かれているため、`CGO_ENABLED=0`のままarm64向けにクロスビルドできます。boki3が同じドライバで本番稼働しているので、配備と運用の手順をそろえられます。`mattn/go-sqlite3`はCGOが必要で、DockerfileとCIにCコンパイラを足す必要があります。

## 結果

バイナリは数MB大きくなります。外部のマイグレーションツールは使わず、スキーマの変更はSQLファイルの追加で行います。
