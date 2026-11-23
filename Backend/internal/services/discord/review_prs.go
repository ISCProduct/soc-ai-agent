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

func changesRequestedOnHead(pr pullSnapshot) bool {
	for _, r := range pr.Reviews {
		if r.State == "CHANGES_REQUESTED" && r.SHA == pr.HeadSHA && pr.HeadSHA != "" {
			return true
		}
	}
	return false
}

func staleChangeReviewers(pr pullSnapshot) []string {
	var logins []string
	for _, r := range pr.Reviews {
		if r.State == "CHANGES_REQUESTED" && r.SHA != pr.HeadSHA && r.Login != "" {
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
		for _, rev := range n.Reviews.Nodes {
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

const reviewQuery = `query($owner:String!,$name:String!){
  repository(owner:$owner, name:$name) {
    pullRequests(states:OPEN, first:50, orderBy:{field:UPDATED_AT, direction:DESC}) {
      nodes {
        number title url isDraft headRefOid
        reviewRequests(first:10) {
          nodes { requestedReviewer { ... on User { login } } }
        }
        reviews(last:10) {
          nodes { state author { login } commit { oid } }
        }
      }
    }
  }
}`

type reviewGQL struct {
	Data struct {
		Repository struct {
			PullRequests struct {
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
					Reviews struct {
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
				}
			}
		}
	}
	Errors []struct {
		Message string
	}
}
