// Package l1catalog は L1 カタログのシード CSV を実行ファイルに埋め込む。
// CSV の作り方は automation/l1_catalog/generate.py を参照。
package l1catalog

import _ "embed"

// SeedCSV は gBizINFO から作った L1 カタログ（約800社）。
//
//go:embed l1_catalog_seed.csv
var SeedCSV string
