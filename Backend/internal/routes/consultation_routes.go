package routes

import (
	consultationcontrollers "Backend/internal/controllers/consultation"
	"Backend/internal/repositories"
	"Backend/internal/services/auth"
	"Backend/internal/services/school"

	"github.com/labstack/echo/v4"
)

// SetupConsultationRoutes は教員と学生の相談（#2028）の API を登録する。
//
//   - 学生: EchoUserAuth の下。本人の案内だけを扱う
//   - 教員の受信一覧: 担当校スコープ（EchoAdminSchoolScope、school_id 必須）の下
//   - 教員のスレッド: school_id を持たないので、コントローラでスレッドの学生の学校と担当校を突き合わせる（越境は 403）
//
// SetupAdminRoutes の外に置くので、admin_routes の分類表には載らない（assessment_routes と同じ）。
func SetupConsultationRoutes(
	api *echo.Group,
	c *consultationcontrollers.Controller,
	userSecret string,
	access auth.UserAccessGuard,
	orgs OrganizationIDResolver,
	userRepo *repositories.UserRepository,
	schools *school.SchoolService,
	adminSecret string,
) {
	student := api.Group("/user/consultations", EchoUserAuth(userSecret, access, orgs))
	student.POST("", c.Ask)
	student.GET("/unread", c.StudentUnread)
	student.GET("/:id", c.StudentThread)
	student.POST("/:id/messages", c.StudentReply)

	inbox := api.Group("/admin/teacher/consultations", EchoAdminAuth(userRepo, adminSecret))
	inbox.GET("", c.Inbox, EchoAdminSchoolScope(schools))
	inbox.GET("/:id", c.TeacherThread)
	inbox.POST("/:id/messages", c.TeacherReply)
}
