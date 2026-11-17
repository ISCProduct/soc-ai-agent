package repositories

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"testing"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func newWithdrawalTestDB(t *testing.T) (*gorm.DB, sqlmock.Sqlmock) {
	t.Helper()
	sqlDB, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	db, err := gorm.Open(mysql.New(mysql.Config{
		Conn:                      sqlDB,
		SkipInitializeWithVersion: true,
	}), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		t.Fatalf("gorm: %v", err)
	}
	return db, mock
}

func TestCollectInterviewAndResumeIDs(t *testing.T) {
	db, mock := newWithdrawalTestDB(t)

	mock.ExpectQuery("SELECT `id` FROM `interview_sessions` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(3).AddRow(5))
	mock.ExpectQuery("SELECT `id` FROM `resume_documents` WHERE user_id = \\?").
		WithArgs(uint(9)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(11))
	mock.ExpectQuery("SELECT `id` FROM `resume_reviews` WHERE document_id IN \\(\\?\\)").
		WithArgs(uint(11)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(21))

	interviewIDs, err := collectInterviewSessionIDs(db, 9)
	if err != nil {
		t.Fatalf("collectInterviewSessionIDs: %v", err)
	}
	if len(interviewIDs) != 2 || interviewIDs[0] != 3 || interviewIDs[1] != 5 {
		t.Fatalf("interviewIDs=%v", interviewIDs)
	}

	docIDs, err := collectResumeDocumentIDs(db, 9)
	if err != nil {
		t.Fatalf("collectResumeDocumentIDs: %v", err)
	}
	if len(docIDs) != 1 || docIDs[0] != 11 {
		t.Fatalf("docIDs=%v", docIDs)
	}

	reviewIDs, err := collectResumeReviewIDs(db, docIDs)
	if err != nil {
		t.Fatalf("collectResumeReviewIDs: %v", err)
	}
	if len(reviewIDs) != 1 || reviewIDs[0] != 21 {
		t.Fatalf("reviewIDs=%v", reviewIDs)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("unmet: %v", err)
	}
}

// 物理削除(HardDeleteUser)が消すモデルの一覧を固定する。
//
// 退会の物理削除は、個人情報を消し切るための処理で、対象を黙って減らすと、削除漏れになる。
// 逆に増やすときは、意図したものであることを、この一覧の更新で明示する。
// 並びは削除の順序(子から先に消す)なので、順序も固定する。
var hardDeleteTargets = []string{
	"AIBudgetSetting",
	"UserRefreshToken",
	"SessionValidation",
	"RealtimeUsageLog",
	"InterviewUtterance",
	"InterviewReport",
	"InterviewQuestionState",
	"ResumeReviewItem",
	"ResumeTextBlock",
	"ResumeReview",
	"ChatMessage",
	"AIGeneratedQuestion",
	"ConversationContext",
	"UserWeightScore",
	"UserScoreRecalculationSnapshot",
	"UserAnalysisProgress",
	"UserEmbedding",
	"VariantAssignment",
	"UserCompanyMatch",
	"UserApplicationStatus",
	"CompanyReview",
	"ResumeDocument",
	"InterviewSession",
	"InterviewVideo",
	"RealtimeUsageLog",
	"ScheduleEvent",
	"SkillScore",
	"GitHubRepoSummary",
	"GitHubLanguageStat",
	"GitHubRepo",
	"GitHubProfile",
	"UserGoogleToken",
	"AssessmentResult",
	"AssessmentSession",
	"ESReview",
	"Notification",
	"TeacherStudentGuidance",
	"OrganizationMembership",
	"CollectiveInsightLog",
	"PendingRegistration",
	"User",
}

func TestHardDeleteUserData_Targets(t *testing.T) {
	src, err := os.ReadFile("user_withdrawal_repository.go")
	if err != nil {
		t.Fatal(err)
	}
	f, err := parser.ParseFile(token.NewFileSet(), "user_withdrawal_repository.go", src, 0)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, decl := range f.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "hardDeleteUserData" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			if sel, ok := lit.Type.(*ast.SelectorExpr); ok {
				if pkg, ok := sel.X.(*ast.Ident); ok && pkg.Name == "models" {
					got = append(got, sel.Sel.Name)
				}
			}
			return true
		})
	}
	if !reflect.DeepEqual(got, hardDeleteTargets) {
		t.Fatalf("物理削除の対象が変わった。意図した変更なら hardDeleteTargets を更新すること。\n got : %v\n want: %v", got, hardDeleteTargets)
	}
}
