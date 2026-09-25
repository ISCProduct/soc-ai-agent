// Package houjinbangou は国税庁「法人番号システムWeb-API」を呼び出す。
//
// 法人番号・商号・所在地という登記に基づく事実だけを返すAPIで、gBizINFO と違い
// 財務や調達の情報は持たない。本システムでは主に「企業名しか分かっていない会社の
// 法人番号を特定する」ために使う。法人番号が分かれば gBizINFO 側の同期が動く。
//
// 仕様: https://www.houjin-bangou.nta.go.jp/webapi/
package houjinbangou

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"golang.org/x/text/width"
	"golang.org/x/time/rate"
)

const (
	defaultBaseURL = "https://api.houjin-bangou.nta.go.jp/4"

	// responseTypeXML は type パラメータの値。01=CSV(Shift-JIS) / 02=CSV(UTF-8) / 12=XML。
	// CSVは列定義がレスポンスに含まれず列順の暗黙知に依存するため、XMLを使う。
	responseTypeXML = "12"

	// maxResponseBytes は読み込む上限。名称検索は最大10万件まで該当しうるため、
	// 際限なく読まないようにする。
	maxResponseBytes = 8 << 20 // 8MiB
)

// ErrNotConfigured はアプリケーションIDが未設定であることを表す。
// 呼び出し側がこれを機能の無効化として扱えるよう、エラー値として公開する。
var ErrNotConfigured = errors.New("houjinbangou: アプリケーションIDが未設定です")

// Corporation は法人番号システムが返す1法人。APIのフィールドのうち、
// 企業の特定に使うものだけを持つ。
type Corporation struct {
	CorporateNumber string // 法人番号(13桁)
	Name            string // 商号又は名称
	Furigana        string // フリガナ
	PostCode        string // 郵便番号
	PrefectureName  string
	CityName        string
	StreetNumber    string
	Kind            string // 法人種別コード(101=国の機関, 301=株式会社 など)
	CloseDate       string // 登記記録の閉鎖等年月日。空でなければ現存しない
	UpdateDate      string
}

// Address は都道府県から番地までを連結した住所を返す。
func (c Corporation) Address() string {
	return c.PrefectureName + c.CityName + c.StreetNumber
}

// IsClosed は登記記録が閉鎖済み（解散・合併等）かを返す。
func (c Corporation) IsClosed() bool {
	return strings.TrimSpace(c.CloseDate) != ""
}

// Client は法人番号システムWeb-APIのクライアント。
type Client struct {
	baseURL string
	appID   string
	http    *http.Client
	limiter *rate.Limiter
}

// NewClientFromEnv は環境変数からクライアントを組み立てる。
// HOUJIN_BANGOU_APP_ID が無い場合も *Client は返し、呼び出し時に
// ErrNotConfigured を返す。起動を止めないためで、gBizINFO と同じ扱い。
func NewClientFromEnv() *Client {
	return NewClient(os.Getenv("HOUJIN_BANGOU_APP_ID"), os.Getenv("HOUJIN_BANGOU_BASE_URL"))
}

// NewClient はアプリケーションIDとベースURLを指定してクライアントを作る。
// baseURL が空なら本番のURLを使う（テストで差し替えるための引数）。
func NewClient(appID, baseURL string) *Client {
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		appID:   strings.TrimSpace(appID),
		http:    &http.Client{Timeout: 20 * time.Second},
		// 国税庁は具体的なレート上限を公開していないが、公的APIに連打をかけない。
		// gBizINFO クライアントと同じ 1req/sec に合わせる。
		limiter: rate.NewLimiter(rate.Every(time.Second), 1),
	}
}

// Enabled は呼び出し可能な設定が揃っているかを返す。
func (c *Client) Enabled() bool {
	return c != nil && c.appID != ""
}

// FindByNumber は法人番号を指定して1法人を取得する。該当が無ければ (nil, nil)。
func (c *Client) FindByNumber(ctx context.Context, corporateNumber string) (*Corporation, error) {
	number := strings.TrimSpace(corporateNumber)
	if number == "" {
		return nil, errors.New("houjinbangou: 法人番号が空です")
	}

	corps, err := c.get(ctx, "/num", url.Values{"number": {number}})
	if err != nil {
		return nil, err
	}
	if len(corps) == 0 {
		return nil, nil
	}
	return &corps[0], nil
}

// SearchByName は商号又は名称で検索する。
//
// mode=2(部分一致)ではなく mode=1(前方一致)を使う。部分一致は「トヨタ自動車」で
// 「ひだかトヨタ自動車販売」まで拾ってしまい、法人の特定には向かない。
func (c *Client) SearchByName(ctx context.Context, name string) ([]Corporation, error) {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return nil, errors.New("houjinbangou: 商号が空です")
	}

	// 国税庁APIの前方一致は「法人格を除いた商号」に対して行われる。
	// 「株式会社サイバーエージェント」をそのまま投げると 0件 になり、
	// 「サイバーエージェント」なら 15件 返る。呼び出し側が正式名称を
	// 持っていても引けるよう、ここで法人格を落としてから投げる。
	query := stripLegalForms(trimmed)
	if query == "" {
		query = trimmed
	}

	return c.get(ctx, "/name", url.Values{
		"name": {widenForSearch(query)},
		"mode": {"1"}, // 1=前方一致, 2=部分一致
		// 閉鎖された法人も含めて取得し、採用するかは呼び出し側で判断する。
		// ここで絞ると「該当ゼロ」と「解散済みしか無い」を区別できなくなる。
		"close": {"1"},
	})
}

// get はAPIを呼んでXMLを法人の一覧へ変換する。
func (c *Client) get(ctx context.Context, path string, params url.Values) ([]Corporation, error) {
	if !c.Enabled() {
		return nil, ErrNotConfigured
	}
	if err := c.limiter.Wait(ctx); err != nil {
		return nil, err
	}

	params.Set("id", c.appID)
	params.Set("type", responseTypeXML)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path+"?"+params.Encode(), nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("houjinbangou: リクエストに失敗しました: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseBytes))
	if err != nil {
		return nil, fmt.Errorf("houjinbangou: レスポンスの読み取りに失敗しました: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		// アプリケーションIDはログにも載せない。
		return nil, fmt.Errorf("houjinbangou: APIが %d を返しました", resp.StatusCode)
	}

	var doc corporationsXML
	if err := xml.Unmarshal(body, &doc); err != nil {
		return nil, fmt.Errorf("houjinbangou: XMLの解析に失敗しました: %w", err)
	}

	out := make([]Corporation, 0, len(doc.Corporations))
	for _, x := range doc.Corporations {
		out = append(out, Corporation{
			CorporateNumber: strings.TrimSpace(x.CorporateNumber),
			Name:            strings.TrimSpace(x.Name),
			Furigana:        strings.TrimSpace(x.Furigana),
			PostCode:        strings.TrimSpace(x.PostCode),
			PrefectureName:  strings.TrimSpace(x.PrefectureName),
			CityName:        strings.TrimSpace(x.CityName),
			StreetNumber:    strings.TrimSpace(x.StreetNumber),
			Kind:            strings.TrimSpace(x.Kind),
			CloseDate:       strings.TrimSpace(x.CloseDate),
			UpdateDate:      strings.TrimSpace(x.UpdateDate),
		})
	}
	return out, nil
}

// corporationsXML は type=12 のレスポンス構造。
type corporationsXML struct {
	XMLName      xml.Name         `xml:"corporations"`
	Count        int              `xml:"count"`
	Corporations []corporationXML `xml:"corporation"`
}

type corporationXML struct {
	CorporateNumber string `xml:"corporateNumber"`
	Name            string `xml:"name"`
	Furigana        string `xml:"furigana"`
	PostCode        string `xml:"postCode"`
	PrefectureName  string `xml:"prefectureName"`
	CityName        string `xml:"cityName"`
	StreetNumber    string `xml:"streetNumber"`
	Kind            string `xml:"kind"`
	CloseDate       string `xml:"closeDate"`
	UpdateDate      string `xml:"updateDate"`
}

// widenForSearch は検索語の半角英数字・記号を全角へ変換する。
//
// name パラメータに半角文字が混じると、国税庁APIは該当を探す前に
// 「101,商号又は名称には全角文字をUTF-8でエンコードして設定してください。」と
// 400 を返す。登記上の商号は全角で登録されている（ＺＯＺＯ、ＮＴＴデータ 等）ため、
// 変換してから送るのが正しい。変換しないと社名に英数字を含む企業を一切引けない。
func widenForSearch(s string) string {
	return width.Widen.String(s)
}
