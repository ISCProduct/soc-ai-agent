package discord

import (
	"context"
	"errors"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
)

type stagingStubSSM struct {
	value    string
	getErr   error
	putErr   error
	putCalls []string
}

func (s *stagingStubSSM) GetParameter(_ context.Context, in *ssm.GetParameterInput, _ ...func(*ssm.Options)) (*ssm.GetParameterOutput, error) {
	if s.getErr != nil {
		return nil, s.getErr
	}
	return &ssm.GetParameterOutput{Parameter: &ssmtypes.Parameter{Value: aws.String(s.value)}}, nil
}

func (s *stagingStubSSM) PutParameter(_ context.Context, in *ssm.PutParameterInput, _ ...func(*ssm.Options)) (*ssm.PutParameterOutput, error) {
	if s.putErr != nil {
		return nil, s.putErr
	}
	s.putCalls = append(s.putCalls, aws.ToString(in.Value))
	return &ssm.PutParameterOutput{}, nil
}

func TestParseStagingState(t *testing.T) {
	tests := []struct {
		in      string
		want    string
		wantErr bool
	}{
		{"on", StagingStateOn, false},
		{"off", StagingStateOff, false},
		{"ON", StagingStateOn, false},
		{"  off  ", StagingStateOff, false},
		{"auto", "", true},
		{"", "", true},
		{"yes", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			got, err := ParseStagingState(tt.in)
			if tt.wantErr {
				if err == nil {
					t.Errorf("不正な値 %q を受理した", tt.in)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Errorf("= %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

// 判断できないときは on に倒すこと。
// off に倒すと、パラメータ未作成や手書きミスでステージングが理由不明に落ちる。
func TestGetState_FallsBackToOn(t *testing.T) {
	tests := []struct {
		name string
		stub *stagingStubSSM
	}{
		{"パラメータが無い", &stagingStubSSM{getErr: &ssmtypes.ParameterNotFound{}}},
		{"未知の値", &stagingStubSSM{value: "maybe"}},
		{"空文字", &stagingStubSSM{value: ""}},
		{"auto と書かれている", &stagingStubSSM{value: "auto"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			svc := &StagingUptimeService{client: tt.stub, parameterName: "/test"}
			got, err := svc.GetState(context.Background())
			if err != nil {
				t.Fatalf("err: %v", err)
			}
			if got != StagingStateOn {
				t.Errorf("= %q, want on（判断できないときは止めない）", got)
			}
		})
	}
}

// 明示的に off と書かれているときだけ off を返す。
func TestGetState_RespectsExplicitOff(t *testing.T) {
	svc := &StagingUptimeService{client: &stagingStubSSM{value: "off"}, parameterName: "/test"}
	got, err := svc.GetState(context.Background())
	if err != nil || got != StagingStateOff {
		t.Errorf("= %q, %v; want off", got, err)
	}
}

// SSM自体が壊れているときはエラーを返す。
// 「取得できない」を勝手に on と解釈すると、障害に気づけない。
func TestGetState_PropagatesRealError(t *testing.T) {
	svc := &StagingUptimeService{
		client:        &stagingStubSSM{getErr: errors.New("access denied")},
		parameterName: "/test",
	}
	if _, err := svc.GetState(context.Background()); err == nil {
		t.Error("SSMエラーを握り潰した")
	}
}

func TestSetState(t *testing.T) {
	stub := &stagingStubSSM{}
	svc := &StagingUptimeService{client: stub, parameterName: "/test"}

	if err := svc.SetState(context.Background(), "OFF"); err != nil {
		t.Fatalf("err: %v", err)
	}
	if len(stub.putCalls) != 1 || stub.putCalls[0] != "off" {
		t.Errorf("書き込み内容 = %v, want [off]（正規化されること）", stub.putCalls)
	}

	// 不正な値は書き込まない
	if err := svc.SetState(context.Background(), "auto"); err == nil {
		t.Error("不正な値を書き込もうとした")
	}
	if len(stub.putCalls) != 1 {
		t.Errorf("不正な値でも書き込んだ: %v", stub.putCalls)
	}
}
