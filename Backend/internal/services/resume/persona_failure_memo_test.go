package resume

import (
	"errors"
	"testing"
	"time"

	"Backend/internal/models"
)

// TestLookupCompanyBrief_PersonaFailureIsNotRetried は、生成に失敗した企業を
// 短期間は再試行しないことを検証する（#1124）。
//
// lookupCompanyBriefFromCache は1レビューで最大2回呼ばれる。記録しないと
// レビューのたびに AI コールを撃ち続けることになり、失敗が続く企業ほど高くつく。
func TestLookupCompanyBrief_PersonaFailureIsNotRetried(t *testing.T) {
	comp := &models.Company{ID: 7, Name: "株式会社テスト", Industry: "情報通信業"}
	persona := &personaStub{err: errors.New("ai down")}

	svc := &ResumeService{}
	svc.SetCompanyRepo(&briefReaderStub{company: comp})
	svc.SetPersonaEnsurer(persona)

	for i := range 3 {
		if brief := svc.lookupCompanyBriefFromCache("株式会社テスト"); brief == "" {
			t.Fatalf("%d回目: brief が空（生成に失敗してもレビューは続行させる）", i+1)
		}
	}

	if persona.calls != 1 {
		t.Errorf("FetchAndSavePersona 呼び出し回数 = %d, want 1（失敗を記録せず再試行している）", persona.calls)
	}
}

// 記録は企業ごと。ある企業の失敗が別企業の生成まで止めてはいけない。
func TestPersonaFailureMemo_IsPerCompany(t *testing.T) {
	svc := &ResumeService{}
	svc.markPersonaFailed(7)

	if !svc.recentlyFailedPersona(7) {
		t.Error("記録した企業が再試行対象になっている")
	}
	if svc.recentlyFailedPersona(8) {
		t.Error("記録していない企業まで止めている")
	}
}

// TTL を過ぎたら再試行する。恒久的にブロックすると、
// 一時的な障害で落ちた企業が永久に重視傾向を持てなくなる。
func TestPersonaFailureMemo_ExpiresAfterTTL(t *testing.T) {
	svc := &ResumeService{}
	svc.markPersonaFailed(7)

	// TTL を跨いだ状態を直接作る（時間を待たない）。
	svc.personaMu.Lock()
	svc.personaFailures[7] = time.Now().Add(-personaFailureTTL - time.Second)
	svc.personaMu.Unlock()

	if svc.recentlyFailedPersona(7) {
		t.Error("TTL を過ぎても再試行しない（一時障害で永久にブロックされる）")
	}
}

// 古い記録は次の記録時に掃除される。放置すると企業数ぶん際限なく溜まる。
func TestPersonaFailureMemo_EvictsStaleEntries(t *testing.T) {
	svc := &ResumeService{}
	svc.markPersonaFailed(7)

	svc.personaMu.Lock()
	svc.personaFailures[7] = time.Now().Add(-personaFailureTTL - time.Second)
	svc.personaMu.Unlock()

	svc.markPersonaFailed(8)

	svc.personaMu.RLock()
	_, stillThere := svc.personaFailures[7]
	size := len(svc.personaFailures)
	svc.personaMu.RUnlock()

	if stillThere {
		t.Error("期限切れの記録が残っている")
	}
	if size != 1 {
		t.Errorf("記録数 = %d, want 1", size)
	}
}
