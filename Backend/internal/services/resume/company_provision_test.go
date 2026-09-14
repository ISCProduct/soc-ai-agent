package resume

import (
	"context"
	"errors"
	"testing"

	"Backend/internal/models"
	"Backend/internal/services/company"
	"Backend/internal/services/shared"
)

type provisionerStub struct {
	calls   int
	company *models.Company
	err     error
}

func (p *provisionerStub) ProvisionByName(ctx context.Context, name string) (*models.Company, error) {
	p.calls++
	if p.err != nil {
		return nil, p.err
	}
	return p.company, nil
}

type lookupStub struct {
	exact map[string]*models.Company
}

func (l *lookupStub) FindByName(name string) (*models.Company, error) {
	if c, ok := l.exact[name]; ok {
		return c, nil
	}
	return nil, errors.New("not found")
}
func (l *lookupStub) FindAllActiveNames(q string) ([]models.CompanyName, error) {
	return nil, nil
}

// TestEnsureRealCompany は未登録企業を「拒否」ではなく「取得して登録」に変えたことを検証する（#1124）。
//
// 従来は Web検索で実在確認だけを行い結果を捨てていた。約3万入力トークン払って
// 真偽値しか得られず、企業情報が無いのでレビューは一般論になり、
// 次に同じ企業名が来ればまた払っていた。
func TestEnsureRealCompany(t *testing.T) {
	registered := &models.Company{ID: 1, Name: "株式会社登録済み"}
	provisioned := &models.Company{ID: 2, Name: "株式会社新規"}

	tests := []struct {
		name            string
		input           string
		provisioner     *provisionerStub
		wantName        string
		wantProvisioned int
		wantErr         bool
	}{
		{
			name:  "DBにあれば取得しない",
			input: "株式会社登録済み",
			// 呼ばれたら失敗する設定にして、DBヒット時に取得へ進まないことを固定する
			provisioner: &provisionerStub{err: errors.New("呼ばれてはいけない")},
			wantName:    "株式会社登録済み", wantProvisioned: 0,
		},
		{
			name:        "DBに無ければ取得して登録し、その正式名称を使う",
			input:       "株式会社新規",
			provisioner: &provisionerStub{company: provisioned},
			wantName:    "株式会社新規", wantProvisioned: 1,
		},
		{
			// 推測で企業を作らせない。取得できなければユーザーに案内を返す
			name:            "取得できなければエラー",
			input:           "架空株式会社XYZ999",
			provisioner:     &provisionerStub{err: errors.New("取得失敗")},
			wantProvisioned: 1, wantErr: true,
		},
		{
			name:        "企業名が空なら何もしない",
			input:       "  ",
			provisioner: &provisionerStub{err: errors.New("呼ばれてはいけない")},
			wantName:    "", wantProvisioned: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &ResumeService{}
			svc.SetCompanyValidator(company.NewCompanyValidationService(
				&lookupStub{exact: map[string]*models.Company{"株式会社登録済み": registered}}, nil))
			svc.SetCompanyProvisioner(tt.provisioner)

			got, err := svc.ensureRealCompany(tt.input)

			if tt.wantErr {
				var ve *shared.ValidationError
				if !errors.As(err, &ve) {
					t.Fatalf("want ValidationError, got %v", err)
				}
			} else if err != nil {
				t.Fatalf("unexpected error: %v", err)
			} else if got != tt.wantName {
				t.Errorf("= %q, want %q", got, tt.wantName)
			}

			if tt.provisioner.calls != tt.wantProvisioned {
				t.Errorf("ProvisionByName 呼び出し回数 = %d, want %d", tt.provisioner.calls, tt.wantProvisioned)
			}
		})
	}
}

// 取得器が未注入なら従来どおり案内を返す（落とさない）
func TestEnsureRealCompany_WithoutProvisioner(t *testing.T) {
	svc := &ResumeService{}
	svc.SetCompanyValidator(company.NewCompanyValidationService(&lookupStub{exact: map[string]*models.Company{}}, nil))

	_, err := svc.ensureRealCompany("株式会社未登録")
	var ve *shared.ValidationError
	if !errors.As(err, &ve) {
		t.Fatalf("want ValidationError, got %v", err)
	}
}
