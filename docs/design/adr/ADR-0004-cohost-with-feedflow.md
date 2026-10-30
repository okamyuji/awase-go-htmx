# ADR-0004 feedflowのEC2とnginxに同居する

ステータス 採用

## 状況

利用者は、feedflowと同じAWS上のホストでawaseを動かすよう指定しました。このホストでは、feedflowのcomposeが持つnginxが80番と443番を使い、boki3がすでに同じ方式で同居しています。

## 決定

- awaseのcomposeはアプリのコンテナ`awase-app`だけを持ち、ネットワーク`feedflow_internal`へ参加します。
- feedflowのnginxが読む`/etc/feedflow/conf.d`へ`zz-awase.conf`を追加します。名前順で最後に読ませ、既定のサーバーを変えません。上流は要求時に名前解決します。
- 設定と証明書を置く前に既存のものを退避し、`nginx -t`が失敗したらすべて戻してreloadしません。
- awaseを取り除くときは、設定を消して`nginx -t`とreloadを行ってから、証明書を消します。取り除く手順は配送とは別のterraformのリソースに置き、再配備のたびに走らないようにします。配送のリソースを取り除く手順のリソースに依存させ、初回の配送が失敗してもdestroyで取り除けるようにします。
- バックアップは取りません。
- awaseのterraformは、Cloudflareの資産とホストへの配送だけを管理します。EC2、Elastic IP、セキュリティグループはfeedflowのterraformが管理します。例外として、ゾーンで共有するWAFのルールセットはawaseのterraformが持ちます(ADR-0005)。
- ホスト名は`awase.okamyuji.work`とします。

## 理由

nginxは起動時に上流の名前を解決できないと起動に失敗します。要求時の解決にすれば、awase-appが止まっていてもnginxは動き続け、feedflowとboki3は影響を受けません。設定の誤りも`nginx -t`で止めれば、次にnginxが再起動したときにfeedflowを巻き込みません。証明書だけが消えて設定が残ると、nginxは証明書を読めずに起動に失敗します。このため、取り除くときは設定を先に消します。EC2を2つのterraformの状態で持つと、片方のapplyがもう片方の変更を上書きします。

## 結果

feedflowのterraformで`destroy`を実行すると、awaseとそのデータも消えます。バックアップを取らないので、このときのデータは戻せません。利用者は、日程調整のデータが短期間で使い終わることを理由に、この扱いを選びました。awaseはfeedflowのnginxとネットワークの名前に依存するため、feedflow側でこれらの名前を変える場合は、awaseの設定も合わせて変えます。
