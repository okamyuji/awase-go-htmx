# awase の配備

awaseは、feedflowのEC2とnginxに同居させて動かします。awaseのterraformが管理するのは、Cloudflareの資産とEC2への配送だけです。EC2、Elastic IP、セキュリティグループは、feedflowのterraformが管理します。

## 構成

```text
[ブラウザ]
   | HTTPS
[Cloudflare] 日本以外を遮断 / /login の連続要求を遮断 / オリジンの IP を秘匿
   | HTTPS（セキュリティグループは Cloudflare の IP 範囲だけを通します）
[EC2] feedflow の nginx -> /etc/feedflow/conf.d/zz-awase.conf -> awase-app:8080
                                                                  |
                                                  /mnt/feedflow-data/awase/awase.db
```

## 前提

- feedflowのterraformで、EC2とnginxが動いていること
- ローカルにDocker（buildxを含む）とterraform 1.6以降があること
- feedflowのterraformが作ったSSH鍵が`../../../feedflow-go-htmx/deploy/terraform/feedflow_ssh_key.pem`にあること

## 1 秘密の値を用意する

```bash
cd deploy/terraform
cp secrets.auto.tfvars.example secrets.auto.tfvars
```

`secrets.auto.tfvars`に、CloudflareのAPIトークンとElastic IPを記入します。必要な権限は`secrets.auto.tfvars.example`に書いてあります。

## 2 既存のWAFのルールを確かめる

ゾーンのWAFのルールセットは、段階ごとに1つしか作れません。applyの前に、既存のルールセットがあるかを確かめます。

```bash
ZONE_ID=$(curl -s -H "Authorization: Bearer $TOKEN" "https://api.cloudflare.com/client/v4/zones?name=okamyuji.work" | jq -r '.result[0].id')
for phase in http_request_firewall_custom http_ratelimit; do
  curl -s -H "Authorization: Bearer $TOKEN" "https://api.cloudflare.com/client/v4/zones/$ZONE_ID/rulesets/phases/$phase/entrypoint" | jq '{success, rules: [.result.rules[]?.description]}'
done
```

レート制限の`http_ratelimit`は、awaseのterraformがルールセットごと作ります。Freeプランのレート制限は1ルールまでなので、既存のルールがある場合はawaseのルールを足せません。カスタムルールの`http_request_firewall_custom`は、AI Crawl Controlのルールと同居するため、`deploy/scripts/cf-country-rule.sh`がawaseのルールだけを追加します。

## 3 配備する

```bash
terraform init
terraform plan
terraform apply
```

applyは次の順に進みます。

1. ローカルでarm64のイメージをビルドします。
2. EC2へイメージ、compose、nginxの設定、証明書を送ります。
3. EC2でawase-appを起動します。
4. nginxの設定と証明書を差し替えて`nginx -t`を実行します。失敗したら元に戻し、reloadしません。
5. awase-appの`/healthz`の応答を確かめます。

## 4 共有アカウントを設定する

パスワードは標準入力から渡します。コマンドの履歴に残らないよう、ファイルから読み込ませます。

```bash
terraform output -raw passwd_command
```

出力されたコマンドの`<名前>`を書き換え、`password.txt`にパスワードを1行で書いてから実行します。実行が終わったら`password.txt`を消します。同じコマンドを再実行すると、パスワードが置き換わり、すべてのセッションが無効になります。

## 5 取り除く

```bash
terraform destroy
```

nginxからawaseの設定を外して`nginx -t`とreloadを済ませてから、証明書とコンテナを消します。データの`/mnt/feedflow-data/awase`は残ります。カスタムルールからはawaseのルールだけを消します。レート制限のルールセットはゾーンで共有しているので、他のアプリのレート制限も一緒に消えます。

## 注意

- バックアップは取りません。EBSが壊れた場合や、feedflowのterraformでdestroyを実行した場合は、awaseのデータも失われます。
- 配送の途中でSSHの接続が切れると、検証していない設定が残ることがあります。そのときは`terraform apply`をもう一度実行します。
