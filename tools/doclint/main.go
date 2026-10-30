// doclint docs 配下の設計文書に対する機械チェックのSSOTです。
// アプリのカバレッジとミューテーションの対象から外すため、独立したモジュールにしています。
package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type finding struct {
	severity string
	path     string
	line     int
	message  string
}

var (
	jpClass    = `[ぁ-んァ-ヿ一-鿿、。]`
	boundaryRe = regexp.MustCompile(jpClass + ` [0-9A-Za-z]|[0-9A-Za-z] ` + jpClass)
	colonEndRe = regexp.MustCompile(`[ぁ-ん一-鿿ァ-ヿ][:：]\s*$`)
	emphasisRe = regexp.MustCompile(`\*\*`)
	tbdRe      = regexp.MustCompile(`(?i)\bTBD\b|\bTODO\b|後で決める|未定とする|将来検討`)
	// 見出しとラベル行は境界スペース検査の対象外です。
	headingRe = regexp.MustCompile(`^#{1,6}\s`)
	// 箇条書きは「見出し語 本文」の形で、見出し語の後ろの半角スペース1つを区切りとして許します。
	listLabelRe  = regexp.MustCompile(`^\s*(?:[-*]|\d+\.)\s+\S+ `)
	fenceRe      = regexp.MustCompile("^\\s*```")
	inlineCodeRe = regexp.MustCompile("`[^`]*`")
	urlRe        = regexp.MustCompile(`https?://\S+`)
	// 用語統一です。英字の前後に英字が続く複合語では発火させません。
	termRules = []struct {
		re        *regexp.Regexp
		canonical string
	}{
		{regexp.MustCompile(`(?:^|[^A-Za-z])(?:sqlite3?|Sqlite)(?:[^A-Za-z0-9]|$)`), "SQLite"},
		{regexp.MustCompile(`(?:^|[^A-Za-z])(?:HTMX|Htmx)(?:[^A-Za-z]|$)`), "htmx"},
		{regexp.MustCompile(`(?:^|[^A-Za-z])(?:cloudflare|CloudFlare)(?:[^A-Za-z]|$)`), "Cloudflare"},
		{regexp.MustCompile(`(?:^|[^A-Za-z])(?:golang|GoLang)(?:[^A-Za-z]|$)`), "Go"},
	}
)

func lintFile(path string) []finding {
	data, err := os.ReadFile(path) // #nosec G304 -- 開発用ツールで、path は WalkDir が返した値です
	if err != nil {
		return []finding{{"Critical", path, 0, "読み取り失敗: " + err.Error()}}
	}
	var out []finding
	inFence := false
	for i, line := range strings.Split(string(data), "\n") {
		n := i + 1
		if fenceRe.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		for _, f := range lintLine(line) {
			f.path, f.line = path, n
			out = append(out, f)
		}
	}
	return out
}

func lintLine(line string) []finding {
	// インラインコードと URL は引用と識別子なので、全ルールの検査対象から外します。
	stripped := urlRe.ReplaceAllString(inlineCodeRe.ReplaceAllString(line, ""), "")
	var out []finding
	if emphasisRe.MatchString(stripped) {
		out = append(out, finding{severity: "High", message: "アスタリスク2つの強調は使わない"})
	}
	if colonEndRe.MatchString(stripped) {
		out = append(out, finding{severity: "Medium", message: "日本語文末のコロンは使わない"})
	}
	if tbdRe.MatchString(stripped) {
		out = append(out, finding{severity: "Critical", message: "先送りマーカー(TBD/TODO/後で決める/未定とする/将来検討)は残さない"})
	}
	if body := listLabelRe.ReplaceAllString(stripped, ""); !headingRe.MatchString(line) && boundaryRe.MatchString(body) {
		out = append(out, finding{severity: "High", message: "日本語と英数字の境界に半角スペースを入れない: " + boundaryRe.FindString(body)})
	}
	for _, tr := range termRules {
		if tr.re.MatchString(stripped) {
			out = append(out, finding{severity: "High", message: "用語統一: " + tr.canonical + " を使う"})
		}
	}
	return out
}

func main() {
	root := "docs"
	if len(os.Args) > 1 {
		root = os.Args[1]
	}
	var all []finding
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "superpowers" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			all = append(all, lintFile(path)...)
		}
		return nil
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "walk error:", err)
		os.Exit(2)
	}
	counts := map[string]int{}
	for _, f := range all {
		counts[f.severity]++
		fmt.Printf("[%s] %s:%d %s\n", f.severity, f.path, f.line, f.message)
	}
	fmt.Printf("Critical %d / High %d / Medium %d / Low %d\n",
		counts["Critical"], counts["High"], counts["Medium"], counts["Low"])
	if counts["Critical"] > 0 || counts["High"] > 0 || counts["Medium"] > 0 {
		os.Exit(1)
	}
}
