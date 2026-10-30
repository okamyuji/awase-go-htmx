// Package web テンプレートと静的ファイルをバイナリに埋め込みます。
package web

import "embed"

// FS templates/ と static/ を持つ埋め込みのファイルシステムです。
//
//go:embed templates static
var FS embed.FS
