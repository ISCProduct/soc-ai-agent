package discord

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"

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
	out := map[string]Link{}
	var start map[string]dynamotypes.AttributeValue
	for {
		page, err := d.db.Scan(ctx, &dynamodb.ScanInput{
			TableName:         aws.String(d.table),
			ExclusiveStartKey: start,
		})
		if err != nil {
			return nil, fmt.Errorf("対応表の取得に失敗しました: %w", err)
		}
		for _, item := range page.Items {
			pk := attrString(item, "pk")
			if !strings.HasPrefix(pk, "discord#") {
				continue
			}
			out[strings.TrimPrefix(pk, "discord#")] = Link{
				BacklogID: attrInt(item, "backlog_id"),
				Name:      attrString(item, "name"),
			}
		}
		if len(page.LastEvaluatedKey) == 0 {
			return out, nil
		}
		start = page.LastEvaluatedKey
	}
}

func (d *dynamoLinkStore) Set(ctx context.Context, discordID string, l Link) error {
	if err := d.ensureMigrated(ctx); err != nil {
		return err
	}
	current, err := d.get(ctx, discordPK(discordID))
	if err != nil {
		return err
	}
	items := []dynamotypes.TransactWriteItem{
		d.put(discordItem(discordID, l), "", nil),
		d.put(lockItem(l.BacklogID, discordID), condAbsentOrSelf, discordValue(discordID)),
	}
	if current != nil && attrInt(current, "backlog_id") != l.BacklogID {
		items = append(items, d.del(backlogPK(attrInt(current, "backlog_id")), condAbsentOrSelf, discordValue(discordID)))
	}
	if err := d.transact(ctx, items); err != nil {
		if !errors.Is(err, errTxnCanceled) {
			return err
		}
		owner, oerr := d.get(ctx, backlogPK(l.BacklogID))
		if oerr != nil {
			return oerr
		}
		who := attrString(owner, "discord_id")
		if who == "" || who == discordID {
			return fmt.Errorf("対応表の更新に失敗しました")
		}
		return fmt.Errorf("%s さんは既に <@%s> に紐づいています。先に /unlink で解除してください", l.Name, who)
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
	if d.legacy != nil {
		links, err := d.legacy.List(ctx)
		if err != nil {
			return err
		}
		for id, l := range links {
			// 移行中に DynamoDB へ書かれた行は上書きしない。
			if err := d.putIfAbsent(ctx, discordItem(id, l)); err != nil {
				return err
			}
			if err := d.putIfAbsent(ctx, lockItem(l.BacklogID, id)); err != nil {
				return err
			}
		}
	}
	if err := d.putIfAbsent(ctx, map[string]dynamotypes.AttributeValue{"pk": sAttr(linkMetaPK)}); err != nil {
		return err
	}
	d.done = true
	return nil
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

func discordItem(discordID string, l Link) map[string]dynamotypes.AttributeValue {
	return map[string]dynamotypes.AttributeValue{
		"pk":         sAttr(discordPK(discordID)),
		"backlog_id": nAttr(l.BacklogID),
		"name":       sAttr(l.Name),
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
