package discord

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	linkMetaPK = "meta#migrated"

	// 1人の行と、Backlog ユーザーの占有行。占有行がある間は同じ人を別の Discord に紐づけない。
	condAbsent       = "attribute_not_exists(pk)"
	condExists       = "attribute_exists(pk)"
	condAbsentOrSelf = "attribute_not_exists(pk) OR discord_id = :discord"
	condSameBacklog  = "backlog_id = :prev"
	// GitHub ログイン名の占有行も、読み取ったときのままのときだけ付け替える（同時の /link で古い占有を残さない）
	condNoGitHub   = "attribute_not_exists(github)"
	condSameGitHub = "github = :prevgh"

	// 移行は /link の 2.5 秒とは別枠。Lambda の 10 秒を超えない範囲で、途中で切れても次の回が続きから書く。
	migrationBudget  = 6 * time.Second
	migrationWorkers = 8
)

// dynamoAPI は対応表が使う DynamoDB 操作。テストで差し替える。
type dynamoAPI interface {
	GetItem(ctx context.Context, in *dynamodb.GetItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error)
	PutItem(ctx context.Context, in *dynamodb.PutItemInput, opts ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error)
	Scan(ctx context.Context, in *dynamodb.ScanInput, opts ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error)
	TransactWriteItems(ctx context.Context, in *dynamodb.TransactWriteItemsInput, opts ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error)
}

// dynamoLinkStore は1人を1項目で持つ。同時に別の人が /link しても、相手の行は書き換えない。
type dynamoLinkStore struct {
	db     dynamoAPI
	table  string
	legacy *ssmLinkStore
	// ensureMigrated だけを直列にする。Set 自体は項目単位の条件付き書き込み。
	mu   sync.Mutex
	done bool
}

func newDynamoLinkStore(cfg aws.Config, table string, legacy *ssmLinkStore) *dynamoLinkStore {
	return newDynamoLinkStoreWith(dynamodb.NewFromConfig(cfg), table, legacy)
}

func newDynamoLinkStoreWith(db dynamoAPI, table string, legacy *ssmLinkStore) *dynamoLinkStore {
	return &dynamoLinkStore{db: db, table: table, legacy: legacy}
}

func (d *dynamoLinkStore) List(ctx context.Context) (map[string]Link, error) {
	if err := d.ensureMigrated(ctx); err != nil {
		return nil, err
	}
	items, err := d.scanItems(ctx)
	if err != nil {
		return nil, err
	}
	out := map[string]Link{}
	for _, item := range items {
		pk := attrString(item, "pk")
		if !strings.HasPrefix(pk, "discord#") {
			continue
		}
		out[strings.TrimPrefix(pk, "discord#")] = Link{
			BacklogID: attrInt(item, "backlog_id"),
			Name:      attrString(item, "name"),
			GitHub:    attrString(item, "github"),
		}
	}
	return out, nil
}

func (d *dynamoLinkStore) Set(ctx context.Context, discordID string, l Link) error {
	if err := d.ensureMigrated(ctx); err != nil {
		return err
	}
	current, err := d.get(ctx, discordPK(discordID))
	if err != nil {
		return err
	}
	discordCond := condAbsent
	var discordValues map[string]dynamotypes.AttributeValue
	prevGitHub := attrString(current, "github")
	if current != nil {
		// 読み取った行のままのときだけ上書きする。同じ Discord への同時 /link が、後着で占有行だけ残すのを防ぐ。
		discordValues = map[string]dynamotypes.AttributeValue{":prev": nAttr(attrInt(current, "backlog_id"))}
		if prevGitHub == "" {
			discordCond = condSameBacklog + " AND " + condNoGitHub
		} else {
			discordCond = condSameBacklog + " AND " + condSameGitHub
			discordValues[":prevgh"] = sAttr(prevGitHub)
		}
	}
	items := []dynamotypes.TransactWriteItem{
		d.put(discordItem(discordID, l), discordCond, discordValues),
		d.put(lockItem(l.BacklogID, discordID), condAbsentOrSelf, discordValue(discordID)),
	}
	if current != nil && attrInt(current, "backlog_id") != l.BacklogID {
		items = append(items, d.del(backlogPK(attrInt(current, "backlog_id")), condAbsentOrSelf, discordValue(discordID)))
	}
	// GitHub のログイン名も1人に1つ。大文字小文字は区別しないので、占有行は小文字のキーで持つ
	newGitHub := normalizedGitHubKey(l.GitHub)
	if newGitHub != "" {
		items = append(items, d.put(githubLockItem(newGitHub, discordID), condAbsentOrSelf, discordValue(discordID)))
	}
	if old := normalizedGitHubKey(prevGitHub); old != "" && old != newGitHub {
		items = append(items, d.del(githubPK(old), condAbsentOrSelf, discordValue(discordID)))
	}
	if err := d.transact(ctx, items); err != nil {
		if !errors.Is(err, errTxnCanceled) {
			return err
		}
		owner, oerr := d.get(ctx, backlogPK(l.BacklogID))
		if oerr != nil {
			return oerr
		}
		if who := attrString(owner, "discord_id"); who != "" && who != discordID {
			return fmt.Errorf("%s さんは既に <@%s> に紐づいています。先に /unlink で解除してください", l.Name, who)
		}
		if newGitHub != "" {
			ghOwner, gerr := d.get(ctx, githubPK(newGitHub))
			if gerr != nil {
				return gerr
			}
			if who := attrString(ghOwner, "discord_id"); who != "" && who != discordID {
				return fmt.Errorf("GitHub「%s」は既に <@%s> に紐づいています。先に /unlink で解除してください", l.GitHub, who)
			}
		}
		return fmt.Errorf("対応表の更新に失敗しました")
	}
	return nil
}

func (d *dynamoLinkStore) Remove(ctx context.Context, discordID string) (bool, error) {
	if err := d.ensureMigrated(ctx); err != nil {
		return false, err
	}
	current, err := d.get(ctx, discordPK(discordID))
	if err != nil {
		return false, err
	}
	if current == nil {
		return false, nil
	}
	items := []dynamotypes.TransactWriteItem{
		d.del(discordPK(discordID), condExists, nil),
		d.del(backlogPK(attrInt(current, "backlog_id")), condAbsentOrSelf, discordValue(discordID)),
	}
	if gh := normalizedGitHubKey(attrString(current, "github")); gh != "" {
		items = append(items, d.del(githubPK(gh), condAbsentOrSelf, discordValue(discordID)))
	}
	if err := d.transact(ctx, items); err != nil {
		if errors.Is(err, errTxnCanceled) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

func (d *dynamoLinkStore) ensureMigrated(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.done {
		return nil
	}
	meta, err := d.get(ctx, linkMetaPK)
	if err != nil {
		return err
	}
	if meta != nil {
		d.done = true
		return nil
	}
	migrateCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), migrationBudget)
	defer cancel()
	if err := d.migrate(migrateCtx); err != nil {
		return err
	}
	d.done = true
	return nil
}

// migrate は SSM の対応表を、まだ無い行だけ DynamoDB へ書く。
// 既にある行は読み飛ばす。条件が失敗した行は上書きしない。全件を毎回直列に書くと、
// コマンドの制限時間でマーカーの前に切れ、次の回も最初からやり直して終わらない。
func (d *dynamoLinkStore) migrate(ctx context.Context) error {
	existing, err := d.scanItems(ctx)
	if err != nil {
		return err
	}
	have := map[string]bool{}
	for _, item := range existing {
		have[attrString(item, "pk")] = true
	}
	var pending []map[string]dynamotypes.AttributeValue
	if d.legacy != nil {
		links, err := d.legacy.List(ctx)
		if err != nil {
			return err
		}
		for id, l := range links {
			if !have[discordPK(id)] {
				pending = append(pending, discordItem(id, l))
			}
			if !have[backlogPK(l.BacklogID)] {
				pending = append(pending, lockItem(l.BacklogID, id))
			}
			if gh := normalizedGitHubKey(l.GitHub); gh != "" && !have[githubPK(gh)] {
				pending = append(pending, githubLockItem(gh, id))
			}
		}
	}
	if err := d.putMissing(ctx, pending); err != nil {
		return err
	}
	if have[linkMetaPK] {
		return nil
	}
	return d.putIfAbsent(ctx, map[string]dynamotypes.AttributeValue{"pk": sAttr(linkMetaPK)})
}

func (d *dynamoLinkStore) scanItems(ctx context.Context) ([]map[string]dynamotypes.AttributeValue, error) {
	var items []map[string]dynamotypes.AttributeValue
	var start map[string]dynamotypes.AttributeValue
	for {
		page, err := d.db.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(d.table),
			ExclusiveStartKey: start,
			ConsistentRead:    aws.Bool(true),
		})
		if err != nil {
			return nil, fmt.Errorf("対応表の取得に失敗しました: %w", err)
		}
		items = append(items, page.Items...)
		if len(page.LastEvaluatedKey) == 0 {
			return items, nil
		}
		start = page.LastEvaluatedKey
	}
}

func (d *dynamoLinkStore) putMissing(ctx context.Context, items []map[string]dynamotypes.AttributeValue) error {
	if len(items) == 0 {
		return nil
	}
	sem := make(chan struct{}, migrationWorkers)
	var wg sync.WaitGroup
	var mu sync.Mutex
	var first error
	for _, item := range items {
		if ctx.Err() != nil {
			break
		}
		wg.Add(1)
		sem <- struct{}{}
		// 成否を first に返すので safego.Go は使わない。
		// panic を握り潰すと移行成功扱いになり、移行済みマーカーを書いてしまう。
		go func(item map[string]dynamotypes.AttributeValue) {
			defer wg.Done()
			defer func() { <-sem }()
			defer func() {
				if r := recover(); r != nil {
					log.Printf("[Discord] 対応表の移行で panic: %v", r)
					mu.Lock()
					if first == nil {
						first = fmt.Errorf("対応表の移行中に異常終了しました: %v", r)
					}
					mu.Unlock()
				}
			}()
			if err := d.putIfAbsent(ctx, item); err != nil {
				mu.Lock()
				if first == nil {
					first = err
				}
				mu.Unlock()
			}
		}(item)
	}
	wg.Wait()
	if first != nil {
		return first
	}
	return ctx.Err()
}

func (d *dynamoLinkStore) putIfAbsent(ctx context.Context, item map[string]dynamotypes.AttributeValue) error {
	_, err := d.db.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           aws.String(d.table),
		Item:                item,
		ConditionExpression: aws.String(condAbsent),
	})
	if err == nil {
		return nil
	}
	var exists *dynamotypes.ConditionalCheckFailedException
	if errors.As(err, &exists) {
		return nil
	}
	return fmt.Errorf("対応表の移行に失敗しました: %w", err)
}

func (d *dynamoLinkStore) get(ctx context.Context, pk string) (map[string]dynamotypes.AttributeValue, error) {
	out, err := d.db.GetItem(ctx, &dynamodb.GetItemInput{
		TableName:      aws.String(d.table),
		Key:            map[string]dynamotypes.AttributeValue{"pk": sAttr(pk)},
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, fmt.Errorf("対応表の取得に失敗しました: %w", err)
	}
	if len(out.Item) == 0 {
		return nil, nil
	}
	return out.Item, nil
}

func (d *dynamoLinkStore) transact(ctx context.Context, items []dynamotypes.TransactWriteItem) error {
	_, err := d.db.TransactWriteItems(ctx, &dynamodb.TransactWriteItemsInput{TransactItems: items})
	if err == nil {
		return nil
	}
	var canceled *dynamotypes.TransactionCanceledException
	if errors.As(err, &canceled) {
		return errTxnCanceled
	}
	return fmt.Errorf("対応表の更新に失敗しました: %w", err)
}

func (d *dynamoLinkStore) put(item map[string]dynamotypes.AttributeValue, condition string, values map[string]dynamotypes.AttributeValue) dynamotypes.TransactWriteItem {
	p := &dynamotypes.Put{TableName: aws.String(d.table), Item: item}
	if condition != "" {
		p.ConditionExpression = aws.String(condition)
		p.ExpressionAttributeValues = values
	}
	return dynamotypes.TransactWriteItem{Put: p}
}

func (d *dynamoLinkStore) del(pk, condition string, values map[string]dynamotypes.AttributeValue) dynamotypes.TransactWriteItem {
	return dynamotypes.TransactWriteItem{Delete: &dynamotypes.Delete{
		TableName:                 aws.String(d.table),
		Key:                       map[string]dynamotypes.AttributeValue{"pk": sAttr(pk)},
		ConditionExpression:       aws.String(condition),
		ExpressionAttributeValues: values,
	}}
}

var errTxnCanceled = errors.New("対応表の条件付き更新が競合しました")

func discordPK(id string) string { return "discord#" + id }
func backlogPK(id int) string    { return "backlog#" + strconv.Itoa(id) }
func githubPK(key string) string { return "github#" + key }

func discordItem(discordID string, l Link) map[string]dynamotypes.AttributeValue {
	item := map[string]dynamotypes.AttributeValue{
		"pk":         sAttr(discordPK(discordID)),
		"backlog_id": nAttr(l.BacklogID),
		"name":       sAttr(l.Name),
	}
	if l.GitHub != "" {
		item["github"] = sAttr(l.GitHub)
	}
	return item
}

func githubLockItem(key, discordID string) map[string]dynamotypes.AttributeValue {
	return map[string]dynamotypes.AttributeValue{
		"pk":         sAttr(githubPK(key)),
		"discord_id": sAttr(discordID),
	}
}

func lockItem(backlogID int, discordID string) map[string]dynamotypes.AttributeValue {
	return map[string]dynamotypes.AttributeValue{
		"pk":         sAttr(backlogPK(backlogID)),
		"discord_id": sAttr(discordID),
	}
}

func discordValue(discordID string) map[string]dynamotypes.AttributeValue {
	return map[string]dynamotypes.AttributeValue{":discord": sAttr(discordID)}
}

func sAttr(v string) dynamotypes.AttributeValue {
	return &dynamotypes.AttributeValueMemberS{Value: v}
}

func nAttr(v int) dynamotypes.AttributeValue {
	return &dynamotypes.AttributeValueMemberN{Value: strconv.Itoa(v)}
}

func attrString(item map[string]dynamotypes.AttributeValue, key string) string {
	v, ok := item[key].(*dynamotypes.AttributeValueMemberS)
	if !ok || v == nil {
		return ""
	}
	return v.Value
}

func attrInt(item map[string]dynamotypes.AttributeValue, key string) int {
	v, ok := item[key].(*dynamotypes.AttributeValueMemberN)
	if !ok || v == nil {
		return 0
	}
	n, _ := strconv.Atoi(v.Value)
	return n
}
