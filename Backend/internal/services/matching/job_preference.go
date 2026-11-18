package matching

import "os"

// 本人の希望（選んだ職種）の反映（SOCAIAGENT-670）。
//
// 推薦は本人の傾向と企業の重視点の近さだけで決まり、本人が選んだ職種は使われていなかった。
// 選んだ職種の募集がある企業は、適合度を JobMatchBonus だけ上げる。加点だけで、除外はしない
// （募集データが無い企業を不利にしない。職種を選んでいない人・募集データが無い企業は、従来どおり）。
const JobMatchBonus = 8.0

// JobPreferenceReader は、本人が選んだ職種と、その職種の募集がある企業を返す。任意の依存（未注入なら加点しない）。
type JobPreferenceReader interface {
	// SessionJobCategoryID は、セッションで本人が選んだ職種。選んでいなければ 0。
	SessionJobCategoryID(sessionID string) (uint, error)
	// CompanyIDsHiringCategory は、companyIDs のうち、その職種（または親子の職種）の募集がある企業。
	CompanyIDsHiringCategory(companyIDs []uint, jobCategoryID uint) (map[uint]bool, error)
}

// SetJobPreferenceReader は、選んだ職種の加点を有効にする。
func (s *MatchingService) SetJobPreferenceReader(r JobPreferenceReader) { s.jobPref = r }

// jobBonusEnabled は、職種の加点を使うか。MATCHING_JOB_BONUS=off で切り戻せる。
func jobBonusEnabled() bool { return os.Getenv("MATCHING_JOB_BONUS") != "off" }

// applyJobBonus は、加点した適合度（100 を上限）。
func applyJobBonus(score float64) float64 {
	if score+JobMatchBonus > 100 {
		return 100
	}
	return score + JobMatchBonus
}
