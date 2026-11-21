package consultation

import "Backend/internal/models"

// teacherGuidanceRef は担当校の確認を済ませたスレッド。
type teacherGuidanceRef struct {
	guidance *models.TeacherStudentGuidance
}
