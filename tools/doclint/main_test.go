package main

import "testing"

func TestLintLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want int
	}{
		{"正しい表記は通す", "DBはSQLiteを使います。", 0},
		{"インラインコード内の識別子は通す", "ドライバは `modernc.org/sqlite` です。", 0},
		{"URL は通す", "https://example.com/sqlite を参照します。", 0},
		{"英字複合語の一部は通す", "mysqlite3xという名前は対象外です。", 0},
		{"小文字の sqlite3 を検出する", "DBはsqlite3です。", 1},
		{"強調を検出する", "これは**重要**です。", 1},
		{"文末コロンを検出する", "手順は次のとおり:", 1},
		{"先送りを検出する", "認証方式はTBDです。", 1},
		{"境界スペースを検出する", "これは Go で書きます。", 1},
		{"見出しの境界スペースは通す", "## 1 Go の版", 0},
		{"箇条書きの見出し語の区切りは通す", "3. 配備先 feedflowと同じホストです。", 0},
		{"箇条書きの本文中の境界スペースは検出する", "- 配備先 これは feedflow です。", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := lintLine(tt.line); len(got) != tt.want {
				t.Errorf("lintLine(%q) = %v, want %d findings", tt.line, got, tt.want)
			}
		})
	}
}
