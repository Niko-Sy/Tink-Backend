// Package migration embeds immutable SQL so release commands do not depend on source paths.
package migration

import "embed"

//go:embed *.sql
var Files embed.FS
