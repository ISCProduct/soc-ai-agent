package models

// CompanyName は軽量な企業ID/名前構造体（セレクト用）
// JSON出力は {"id":..., "name":...} となる
type CompanyName struct {
	ID   uint   `json:"id"`
	Name string `json:"name"`
}
