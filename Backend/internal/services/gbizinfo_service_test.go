package services

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(req *http.Request) (*http.Response, error) {
	return f(req)
}

func TestGBizInfoServiceFetchProcurementsKeepsJointSignatures(t *testing.T) {
	service := &GBizInfoService{
		baseURL: "https://gbiz.example",
		token:   "test-token",
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v2/hojin/1234567890123/procurement" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			if got := r.Header.Get("X-hojinInfo-api-token"); got != "test-token" {
				t.Fatalf("unexpected token: %s", got)
			}
			body := `{
			"hojin-infos": [{
				"procurement": [{
					"title": "共同開発業務",
					"date_of_order": "2026-01-15",
					"amount": 1000000,
					"government_departments": "デジタル庁",
					"joint_signatures": ["株式会社パートナーA", " 株式会社パートナーB "]
				}]
			}]
		}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})},
	}

	rows, err := service.fetchProcurements(context.Background(), "1234567890123", 1)
	if err != nil {
		t.Fatalf("fetchProcurements returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 procurement, got %d", len(rows))
	}

	names := parseJointSignatures(rows[0].JointSignatures)
	if len(names) != 2 || names[0] != "株式会社パートナーA" || names[1] != "株式会社パートナーB" {
		t.Fatalf("unexpected joint signatures: %#v raw=%s", names, rows[0].JointSignatures)
	}
}

func TestGBizInfoServiceFetchSubsidiesKeepsJointSignatures(t *testing.T) {
	service := &GBizInfoService{
		baseURL: "https://gbiz.example",
		token:   "test-token",
		client: &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
			if r.URL.Path != "/v2/hojin/1234567890123/subsidy" {
				t.Fatalf("unexpected path: %s", r.URL.Path)
			}
			body := `{
			"hojin-infos": [{
				"subsidy": [{
					"title": "共同実証事業",
					"date_of_approval": "2026-02-01",
					"amount": "500000",
					"government_departments": "経済産業省",
					"target": "実証",
					"note": "",
					"joint_signatures": ["株式会社共同先"]
				}]
			}]
		}`
			return &http.Response{
				StatusCode: http.StatusOK,
				Header:     http.Header{"Content-Type": []string{"application/json"}},
				Body:       io.NopCloser(strings.NewReader(body)),
			}, nil
		})},
	}

	rows, err := service.fetchSubsidies(context.Background(), "1234567890123", 1)
	if err != nil {
		t.Fatalf("fetchSubsidies returned error: %v", err)
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 subsidy, got %d", len(rows))
	}

	names := parseJointSignatures(rows[0].JointSignatures)
	if len(names) != 1 || names[0] != "株式会社共同先" {
		t.Fatalf("unexpected joint signatures: %#v raw=%s", names, rows[0].JointSignatures)
	}
}
