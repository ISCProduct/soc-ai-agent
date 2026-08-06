package company

import (
	"fmt"

	"golang.org/x/sync/singleflight"
)

// CompanySearchFlight は企業キー単位で Write Search を単一化する（R4）。
type CompanySearchFlight struct {
	group singleflight.Group
}

func NewCompanySearchFlight() *CompanySearchFlight {
	return &CompanySearchFlight{}
}

// Do は op+companyKey で同時実行を1つにまとめる。
func (f *CompanySearchFlight) Do(op, companyKey string, fn func() (any, error)) (any, error) {
	if f == nil {
		return fn()
	}
	key := stringsKey(op, companyKey)
	v, err, _ := f.group.Do(key, fn)
	return v, err
}

func stringsKey(op, companyKey string) string {
	if companyKey == "" {
		companyKey = "_"
	}
	return fmt.Sprintf("%s:%s", op, companyKey)
}
