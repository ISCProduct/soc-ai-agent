package houjinbangou_test

// 国税庁 法人番号システムWeb-API クライアントのテスト。
// 実行: cd Backend && go test ./internal/services/houjinbangou/... -v

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"Backend/internal/services/houjinbangou"
)

// 実APIが返すXMLをそのまま貼ったもの（type=12）。
const numResponseXML = `<?xml version="1.0" encoding="UTF-8"?><corporations><lastUpdateDate>2026-09-16</lastUpdateDate><count>1</count><divideNumber>1</divideNumber><divideSize>1</divideSize><corporation><sequenceNumber>1</sequenceNumber><corporateNumber>7000012050002</corporateNumber><process>01</process><correct>1</correct><updateDate>2018-04-02</updateDate><changeDate>2015-10-05</changeDate><name>国税庁</name><nameImageId/><kind>101</kind><prefectureName>東京都</prefectureName><cityName>千代田区</cityName><streetNumber>霞が関３丁目１－１</streetNumber><addressImageId/><prefectureCode>13</prefectureCode><cityCode>101</cityCode><postCode>1000013</postCode><addressOutside/><addressOutsideImageId/><closeDate/><closeCause/><successorCorporateNumber/><changeCause/><assignmentDate>2015-10-05</assignmentDate><latest>1</latest><enName>National Tax Agency</enName><enPrefectureName>Tokyo</enPrefectureName><enCityName>3-1-1 Kasumigaseki, Chiyoda-ku</enCityName><enAddressOutside/><furigana>コクゼイチョウ</furigana><hihyoji>0</hihyoji></corporation></corporations>`

// corpXML はテスト用に1法人分のXMLを組み立てる。
func corpXML(number, name, closeDate string) string {
	return `<corporation><corporateNumber>` + number + `</corporateNumber><name>` + name +
		`</name><kind>301</kind><prefectureName>東京都</prefectureName><cityName>港区</cityName>` +
		`<streetNumber>1-1</streetNumber><postCode>1000001</postCode><closeDate>` + closeDate +
		`</closeDate><furigana/><updateDate>2025-01-01</updateDate></corporation>`
}

// corpAt は都道府県・市区町村を指定して1法人分のXMLを組み立てる。
func corpAt(number, name, pref, city string) string {
	return `<corporation><corporateNumber>` + number + `</corporateNumber><name>` + name +
		`</name><kind>301</kind><prefectureName>` + pref + `</prefectureName><cityName>` + city +
		`</cityName><streetNumber>1-1</streetNumber><postCode>1000001</postCode><closeDate/>` +
		`<furigana/><updateDate>2025-01-01</updateDate></corporation>`
}

func wrap(inner string) string {
	return `<?xml version="1.0" encoding="UTF-8"?><corporations><count>1</count>` + inner + `</corporations>`
}

// newStub はXMLを返すテストサーバとクライアントを用意する。
func newStub(t *testing.T, body string, status int) (*houjinbangou.Client, *[]string) {
	t.Helper()
	var gotQueries []string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQueries = append(gotQueries, r.URL.Path+"?"+r.URL.RawQuery)
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)

	return houjinbangou.NewClient("test-app-id", srv.URL), &gotQueries
}

func TestFindByNumber(t *testing.T) {
	client, queries := newStub(t, numResponseXML, http.StatusOK)

	corp, err := client.FindByNumber(context.Background(), "7000012050002")
	if err != nil {
		t.Fatalf("エラーが返った: %v", err)
	}
	if corp == nil {
		t.Fatal("法人が返らなかった")
	}

	if corp.CorporateNumber != "7000012050002" {
		t.Errorf("法人番号が違う: %q", corp.CorporateNumber)
	}
	if corp.Name != "国税庁" {
		t.Errorf("商号が違う: %q", corp.Name)
	}
	if got, want := corp.Address(), "東京都千代田区霞が関３丁目１－１"; got != want {
		t.Errorf("住所が違う: got %q, want %q", got, want)
	}
	if corp.IsClosed() {
		t.Error("closeDate が空なので閉鎖扱いにしてはいけない")
	}

	// アプリケーションIDと type=12(XML) が必ず載ること
	q := (*queries)[0]
	for _, want := range []string{"id=test-app-id", "type=12", "number=7000012050002"} {
		if !strings.Contains(q, want) {
			t.Errorf("クエリに %q が無い: %s", want, q)
		}
	}
}

func TestFindByNumber_該当なしはnilを返す(t *testing.T) {
	client, _ := newStub(t, `<?xml version="1.0"?><corporations><count>0</count></corporations>`, http.StatusOK)

	corp, err := client.FindByNumber(context.Background(), "0000000000000")
	if err != nil {
		t.Fatalf("該当なしはエラーにしない: %v", err)
	}
	if corp != nil {
		t.Errorf("該当なしなのに法人が返った: %+v", corp)
	}
}

func TestClient_未設定ならErrNotConfigured(t *testing.T) {
	client := houjinbangou.NewClient("", "")
	if client.Enabled() {
		t.Fatal("アプリケーションIDが無いのに Enabled になっている")
	}

	_, err := client.FindByNumber(context.Background(), "7000012050002")
	if err != houjinbangou.ErrNotConfigured {
		t.Errorf("ErrNotConfigured を返すべき: %v", err)
	}
}

func TestClient_APIエラーでIDを漏らさない(t *testing.T) {
	client, _ := newStub(t, "forbidden", http.StatusForbidden)

	_, err := client.FindByNumber(context.Background(), "7000012050002")
	if err == nil {
		t.Fatal("エラーが返るべき")
	}
	// アプリケーションIDはシークレット。エラーメッセージはログや画面に出るので載せない。
	if strings.Contains(err.Error(), "test-app-id") {
		t.Errorf("エラーにアプリケーションIDが含まれている: %v", err)
	}
}

func TestResolveCorporateNumber(t *testing.T) {
	tests := []struct {
		name       string
		query      string
		location   string // 所在地ヒント（空でもよい）
		body       string
		wantNumber string // 空なら「一意に決まらない」を期待
	}{
		{
			name:       "法人格の有無が違っても一致させる",
			query:      "トヨタ自動車",
			body:       wrap(corpXML("1180301018771", "トヨタ自動車株式会社", "")),
			wantNumber: "1180301018771",
		},
		{
			name:  "前方一致で拾った別法人は採用しない",
			query: "トヨタ自動車",
			body: wrap(corpXML("1180301018771", "トヨタ自動車東京販売株式会社", "") +
				corpXML("2180301018770", "トヨタ自動車九州株式会社", "")),
			wantNumber: "",
		},
		{
			name:  "完全一致が1件だけなら他に候補があっても採用する",
			query: "トヨタ自動車",
			body: wrap(corpXML("1180301018771", "トヨタ自動車株式会社", "") +
				corpXML("2180301018770", "トヨタ自動車九州株式会社", "")),
			wantNumber: "1180301018771",
		},
		{
			name:       "解散済みの法人は採用しない",
			query:      "テスト商事",
			body:       wrap(corpXML("9999999999999", "テスト商事株式会社", "2020-03-31")),
			wantNumber: "",
		},
		{
			name:  "同名の現存法人が複数あるときは決めない",
			query: "大和",
			body: wrap(corpXML("1111111111111", "株式会社大和", "") +
				corpXML("2222222222222", "大和株式会社", "")),
			wantNumber: "",
		},
		{
			// 「株式会社ＺＯＺＯ」は実際に2社ある。商号だけでは決まらない。
			name:     "同名が複数でも所在地で絞れれば採用する",
			query:    "ゾゾ",
			location: "千葉県千葉市美浜区",
			body: wrap(corpAt("4040001010503", "株式会社ゾゾ", "千葉県", "千葉市美浜区") +
				corpAt("4180001113671", "株式会社ゾゾ", "東京都", "渋谷区")),
			wantNumber: "4040001010503",
		},
		{
			name:     "所在地ヒントが無ければ同名複数は決めない",
			query:    "ゾゾ",
			location: "",
			body: wrap(corpAt("4040001010503", "株式会社ゾゾ", "千葉県", "千葉市美浜区") +
				corpAt("4180001113671", "株式会社ゾゾ", "東京都", "渋谷区")),
			wantNumber: "",
		},
		{
			name:     "所在地ヒントがどれとも合わなければ決めない",
			query:    "ゾゾ",
			location: "大阪府大阪市",
			body: wrap(corpAt("4040001010503", "株式会社ゾゾ", "千葉県", "千葉市美浜区") +
				corpAt("4180001113671", "株式会社ゾゾ", "東京都", "渋谷区")),
			wantNumber: "",
		},
		{
			// 実例。「freee株式会社」は法人格を落とすと台東区の
			// 「株式会社Ｆｒｅｅｅ」に完全一致する(本物は品川区)。
			// 候補が1件でも所在地が食い違えば採用しない。
			name:       "候補が1件でも所在地が食い違えば採用しない",
			query:      "Ｆｒｅｅｅ株式会社",
			location:   "東京都品川区大崎",
			body:       wrap(corpAt("5010401130076", "株式会社Ｆｒｅｅｅ", "東京都", "台東区")),
			wantNumber: "",
		},
		{
			name:       "所在地が一致すれば採用する",
			query:      "Ｆｒｅｅｅ株式会社",
			location:   "東京都台東区台東４丁目",
			body:       wrap(corpAt("5010401130076", "株式会社Ｆｒｅｅｅ", "東京都", "台東区")),
			wantNumber: "5010401130076",
		},
		{
			name:       "所在地ヒントが都道府県だけなら市区町村は問わない",
			query:      "テスト商事",
			location:   "東京都",
			body:       wrap(corpAt("1111111111111", "テスト商事株式会社", "東京都", "台東区")),
			wantNumber: "1111111111111",
		},
		{
			name:       "所在地ヒントが無ければ従来どおり採用する",
			query:      "テスト商事",
			location:   "",
			body:       wrap(corpAt("1111111111111", "テスト商事株式会社", "東京都", "台東区")),
			wantNumber: "1111111111111",
		},
		{
			name:       "都道府県が違えば採用しない",
			query:      "テスト商事",
			location:   "大阪府大阪市",
			body:       wrap(corpAt("1111111111111", "テスト商事株式会社", "東京都", "台東区")),
			wantNumber: "",
		},
		{
			name:       "該当ゼロ",
			query:      "存在しない会社",
			body:       `<?xml version="1.0"?><corporations><count>0</count></corporations>`,
			wantNumber: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client, _ := newStub(t, tt.body, http.StatusOK)

			corp, err := client.ResolveCorporateNumber(context.Background(), tt.query, tt.location)
			if err != nil {
				t.Fatalf("エラーが返った: %v", err)
			}

			if tt.wantNumber == "" {
				if corp != nil {
					t.Errorf("一意に決まらないはずが %s を採用した", corp.CorporateNumber)
				}
				return
			}
			if corp == nil {
				t.Fatalf("法人番号 %s を採用すべきだが nil が返った", tt.wantNumber)
			}
			if corp.CorporateNumber != tt.wantNumber {
				t.Errorf("法人番号が違う: got %s, want %s", corp.CorporateNumber, tt.wantNumber)
			}
		})
	}
}

func TestSearchByName_前方一致で問い合わせる(t *testing.T) {
	client, queries := newStub(t, wrap(corpXML("1180301018771", "トヨタ自動車株式会社", "")), http.StatusOK)

	if _, err := client.SearchByName(context.Background(), "トヨタ自動車"); err != nil {
		t.Fatalf("エラーが返った: %v", err)
	}

	// mode=2(部分一致)は「ひだかトヨタ自動車販売」まで拾って特定に使えない
	if !strings.Contains((*queries)[0], "mode=1") {
		t.Errorf("前方一致(mode=1)で問い合わせるべき: %s", (*queries)[0])
	}
}
