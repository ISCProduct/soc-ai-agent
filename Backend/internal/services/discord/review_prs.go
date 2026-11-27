package discord

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// WaitingPR は朝のリマインドに載せる、レビュー待ちの PR。
type WaitingPR struct {
	Number    int
	Title     string
	URL       string
	Reviewers []string
}

// pullSnapshot は「レビュー待ちか」を決めるのに必要な PR の断面。
type pullSnapshot struct {
	Number    int
	Title     string
	URL       string
	Draft     bool
	HeadSHA   string
	Requested []string
	Reviews   []pullReview
}

type pullReview struct {
	Login string
	State string
	SHA   string
}

// ReviewLister はレビュー待ち PR の取得。テストで差し替える。
type ReviewLister interface {
	Waiting(ctx context.Context) ([]WaitingPR, error)
}

// SelectWaiting は開いている PR から、朝のリマインドに載せるものを選ぶ。
// Draft は除く。最新のコミットに対する変更要求があり、その後の更新が無いものも除く。
// 載せる相手が誰もいない PR は出さない。
func SelectWaiting(prs []pullSnapshot) []WaitingPR {
	var out []WaitingPR
	for _, pr := range prs {
		if pr.Draft || changesRequestedOnHead(pr) {
			continue
		}
		reviewers := uniqueLogins(pr.Requested)
		if len(reviewers) == 0 {
			reviewers = staleChangeReviewers(pr)
		}
		if len(reviewers) == 0 {
			continue
		}
		out = append(out, WaitingPR{
			Number:    pr.Number,
			Title:     pr.Title,
			URL:       pr.URL,
			Reviewers: reviewers,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Number < out[j].Number })
	return out
}

// opinionatedState は、レビュアーの「意見」を表す状態か。
// COMMENTED / PENDING は意見ではないので、承認や変更要求を上書きしない。
func opinionatedState(state string) bool {
	switch state {
	case "APPROVED", "CHANGES_REQUESTED", "DISMISSED":
		return true
	}
	return false
}

// latestOpinions はレビュアーごとの最新の意見（APPROVED / CHANGES_REQUESTED / DISMISSED）を返す。
// レビューは履歴なので、変更要求のあとに承認が付いていれば、その人の現在の意見は承認。
// Reviews は古い順に並んでいる前提で、同じ人は後ろのものが勝つ。キーは小文字のログイン名。
func latestOpinions(reviews []pullReview) map[string]pullReview {
	latest := map[string]pullReview{}
	for _, r := range reviews {
		key := strings.ToLower(strings.TrimSpace(r.Login))
		if key == "" || !opinionatedState(r.State) {
			continue
		}
		latest[key] = r
	}
	return latest
}

// changesRequestedOnHead は、最新のコミットに対して変更要求が「今も」付いているか。
// 同じ人が後から承認や却下をしていれば、変更要求とは数えない。
func changesRequestedOnHead(pr pullSnapshot) bool {
	if pr.HeadSHA == "" {
		return false
	}
	for _, r := range latestOpinions(pr.Reviews) {
		if r.State == "CHANGES_REQUESTED" && r.SHA == pr.HeadSHA {
			return true
		}
	}
	return false
}

// staleChangeReviewers は、更新前のコミットに変更要求を出したまま、まだ見直していない人。
// 変更要求のあとに承認した人は含めない。
func staleChangeReviewers(pr pullSnapshot) []string {
	var logins []string
	for _, r := range latestOpinions(pr.Reviews) {
		if r.State == "CHANGES_REQUESTED" && r.SHA != pr.HeadSHA {
			logins = append(logins, r.Login)
		}
	}
	return uniqueLogins(logins)
}

func uniqueLogins(in []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, login := range in {
		key := strings.ToLower(strings.TrimSpace(login))
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, strings.TrimSpace(login))
	}
	sort.Slice(out, func(i, j int) bool { return strings.ToLower(out[i]) < strings.ToLower(out[j]) })
	return out
}

// GitHubReviewClient は読み取り専用トークンでレビュー待ち PR を取る。
// 配信用の GITHUB_DISPATCH_TOKEN は使わない。同じ値だったら無効にする。
type GitHubReviewClient struct {
	token    string
	owner    string
	repo     string
	endpoint string
	http     *http.Client
}

// NewGitHubReviewClientFromEnv はトークンかリポジトリが無ければ nil を返す。
// 朝のリマインド本体は、このときレビュー待ちの節を出さずに続行する。
func NewGitHubReviewClientFromEnv() *GitHubReviewClient {
	token := strings.TrimSpace(os.Getenv("GITHUB_PR_READ_TOKEN"))
	repo := strings.TrimSpace(os.Getenv("GITHUB_PR_READ_REPO"))
	if repo == "" {
		repo = strings.TrimSpace(os.Getenv("GITHUB_DISPATCH_REPO"))
	}
	dispatch := strings.TrimSpace(os.Getenv("GITHUB_DISPATCH_TOKEN"))
	if token == "" || repo == "" {
		return nil
	}
	if dispatch != "" && token == dispatch {
		log.Printf("[Reminder] GITHUB_PR_READ_TOKEN が配信用トークンと同じです。レビュー待ちは載せません")
		return nil
	}
	owner, name, ok := strings.Cut(repo, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		log.Printf("[Reminder] GITHUB_PR_READ_REPO は owner/repo の形にしてください")
		return nil
	}
	return &GitHubReviewClient{
		token:    token,
		owner:    owner,
		repo:     name,
		endpoint: "https://api.github.com/graphql",
		http:     &http.Client{Timeout: 4 * time.Second},
	}
}

func (c *GitHubReviewClient) Waiting(ctx context.Context) ([]WaitingPR, error) {
	if c == nil {
		return nil, nil
	}
	body, err := json.Marshal(map[string]any{
		"query": reviewQuery,
		"variables": map[string]string{
			"owner": c.owner,
			"name":  c.repo,
		},
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "soc-ai-agent-reminder")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, errors.New("GitHub への接続に失敗しました")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, errors.New("GitHub の応答を読めませんでした")
	}
	if resp.StatusCode/100 != 2 {
		return nil, fmt.Errorf("GitHub が %d を返しました", resp.StatusCode)
	}
	var parsed reviewGQL
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return nil, errors.New("GitHub の応答を解釈できませんでした")
	}
	if len(parsed.Errors) > 0 {
		return nil, errors.New("GitHub の問い合わせに失敗しました")
	}
	if parsed.Data.Repository.PullRequests.PageInfo.HasNextPage {
		// 先頭 reviewQueryLimit 件までしか見ていない。古い更新の PR はリマインドに載らない。
		log.Printf("[Reminder] 開いている PR が %d 件を超えたため、古い PR は載せていません", reviewQueryLimit)
	}
	var snaps []pullSnapshot
	for _, n := range parsed.Data.Repository.PullRequests.Nodes {
		snap := pullSnapshot{
			Number:  n.Number,
			Title:   n.Title,
			URL:     n.URL,
			Draft:   n.IsDraft,
			HeadSHA: n.HeadRefOid,
		}
		for _, reqn := range n.ReviewRequests.Nodes {
			if login := reqn.RequestedReviewer.Login; login != "" {
				snap.Requested = append(snap.Requested, login)
			}
		}
		// 履歴（古い順）に、ユーザーごとの最新の意見を後ろに足す。
		// 履歴は直近10件しか取らないので、それより古い変更要求は最新の意見の側で拾う。
		// latestOpinions は同じ人なら後ろが勝つため、最新の意見が履歴と食い違うときは最新が採用される。
		for _, rev := range n.Reviews.Nodes {
			snap.Reviews = append(snap.Reviews, pullReview{
				Login: rev.Author.Login,
				State: rev.State,
				SHA:   rev.Commit.Oid,
			})
		}
		for _, rev := range n.LatestOpinionatedReviews.Nodes {
			snap.Reviews = append(snap.Reviews, pullReview{
				Login: rev.Author.Login,
				State: rev.State,
				SHA:   rev.Commit.Oid,
			})
		}
		snaps = append(snaps, snap)
	}
	return SelectWaiting(snaps), nil
}

// reviewQueryLimit は一度に見る開いている PR の数（reviewQuery の first と揃える）。
const reviewQueryLimit = 50

// reviewQuery の注意:
//   - reviewRequests は User だけを取る。チーム宛てのレビュー依頼（Team）は login を持たないので
//     載らない。チームにだけ依頼した PR は「依頼先なし」として出さない。
//   - reviews は履歴、latestOpinionatedReviews はユーザーごとの最新の意見。
//     後者だけが「変更要求のあとに承認された」を直接表す。
const reviewQuery = `query($owner:String!,$name:String!){
  repository(owner:$owner, name:$name) {
    pullRequests(states:OPEN, first:50, orderBy:{field:UPDATED_AT, direction:DESC}) {
      pageInfo { hasNextPage }
      nodes {
        number title url isDraft headRefOid
        reviewRequests(first:10) {
          nodes { requestedReviewer { ... on User { login } } }
        }
        reviews(last:10) {
          nodes { state author { login } commit { oid } }
        }
        latestOpinionatedReviews(first:20) {
          nodes { state author { login } commit { oid } }
        }
      }
    }
  }
}`

type reviewNodesGQL struct {
	Nodes []struct {
		State  string
		Author struct {
			Login string
		}
		Commit struct {
			Oid string
		}
	}
}

type reviewGQL struct {
	Data struct {
		Repository struct {
			PullRequests struct {
				PageInfo struct {
					HasNextPage bool
				}
				Nodes []struct {
					Number         int
					Title          string
					URL            string
					IsDraft        bool
					HeadRefOid     string
					ReviewRequests struct {
						Nodes []struct {
							RequestedReviewer struct {
								Login string
							}
						}
					}
					Reviews                  reviewNodesGQL
					LatestOpinionatedReviews reviewNodesGQL
				}
			}
		}
	}
	Errors []struct {
		Message string
	}
}
