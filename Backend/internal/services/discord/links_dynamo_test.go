package discord

import (
	"context"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	dynamotypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// memDynamo は条件付きの Put / Delete / Transact を、本番と同じ式だけ評価する。
type memDynamo struct {
	mu              sync.Mutex
	items           map[string]map[string]dynamotypes.AttributeValue
	scans           int
	consistentScans int
	puts            int
	panicPK         string
}

func newMemDynamo() *memDynamo {
	return &memDynamo{items: map[string]map[string]dynamotypes.AttributeValue{}}
}

func (m *memDynamo) GetItem(_ context.Context, in *dynamodb.GetItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.GetItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	pk := attrString(in.Key, "pk")
	if item, ok := m.items[pk]; ok {
		return &dynamodb.GetItemOutput{Item: cloneItem(item)}, nil
	}
	return &dynamodb.GetItemOutput{}, nil
}

func (m *memDynamo) PutItem(_ context.Context, in *dynamodb.PutItemInput, _ ...func(*dynamodb.Options)) (*dynamodb.PutItemOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.puts++
	pk := attrString(in.Item, "pk")
	if m.panicPK != "" && pk == m.panicPK {
		panic("put boom")
	}
	if !condOK(aws.ToString(in.ConditionExpression), m.items[pk], in.ExpressionAttributeValues) {
		return nil, &dynamotypes.ConditionalCheckFailedException{}
	}
	m.items[pk] = cloneItem(in.Item)
	return &dynamodb.PutItemOutput{}, nil
}

func (m *memDynamo) Scan(_ context.Context, in *dynamodb.ScanInput, _ ...func(*dynamodb.Options)) (*dynamodb.ScanOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.scans++
	if aws.ToBool(in.ConsistentRead) {
		m.consistentScans++
	}
	var items []map[string]dynamotypes.AttributeValue
	for _, item := range m.items {
		items = append(items, cloneItem(item))
	}
	return &dynamodb.ScanOutput{Items: items}, nil
}

func (m *memDynamo) TransactWriteItems(_ context.Context, in *dynamodb.TransactWriteItemsInput, _ ...func(*dynamodb.Options)) (*dynamodb.TransactWriteItemsOutput, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	type change struct {
		pk   string
		item map[string]dynamotypes.AttributeValue
		del  bool
	}
	var plan []change
	for _, tx := range in.TransactItems {
		switch {
		case tx.Put != nil:
			pk := attrString(tx.Put.Item, "pk")
			if !condOK(aws.ToString(tx.Put.ConditionExpression), m.items[pk], tx.Put.ExpressionAttributeValues) {
				return nil, &dynamotypes.TransactionCanceledException{}
			}
			plan = append(plan, change{pk: pk, item: cloneItem(tx.Put.Item)})
		case tx.Delete != nil:
			pk := attrString(tx.Delete.Key, "pk")
			if !condOK(aws.ToString(tx.Delete.ConditionExpression), m.items[pk], tx.Delete.ExpressionAttributeValues) {
				return nil, &dynamotypes.TransactionCanceledException{}
			}
			plan = append(plan, change{pk: pk, del: true})
		default:
			return nil, &dynamotypes.TransactionCanceledException{}
		}
	}
	for _, c := range plan {
		if c.del {
			delete(m.items, c.pk)
			continue
		}
		m.items[c.pk] = c.item
	}
	return &dynamodb.TransactWriteItemsOutput{}, nil
}

func condOK(expr string, item map[string]dynamotypes.AttributeValue, values map[string]dynamotypes.AttributeValue) bool {
	if expr == "" {
		return true
	}
	for _, part := range strings.Split(expr, " OR ") {
		switch strings.TrimSpace(part) {
		case condAbsent:
			if item == nil {
				return true
			}
		case condExists:
			if item != nil {
				return true
			}
		case "discord_id = :discord":
			if item != nil && attrString(item, "discord_id") == attrString(values, ":discord") {
				return true
			}
		case condSameBacklog:
			if item != nil && attrInt(item, "backlog_id") == attrInt(values, ":prev") {
				return true
			}
		}
	}
	return false
}

func cloneItem(item map[string]dynamotypes.AttributeValue) map[string]dynamotypes.AttributeValue {
	out := make(map[string]dynamotypes.AttributeValue, len(item))
	for k, v := range item {
		out[k] = v
	}
	return out
}

func newDynamoService(legacy *ssmLinkStore) (*LinkService, *memDynamo) {
	db := newMemDynamo()
	return &LinkService{store: newDynamoLinkStoreWith(db, "links", legacy)}, db
}

func TestDynamoLink_100人登録でき同時の別ユーザーも残る(t *testing.T) {
	svc, _ := newDynamoService(nil)
	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			err := svc.Set(context.Background(), "d"+strconv.Itoa(i), Link{BacklogID: i + 1, Name: "人"})
			if err != nil {
				t.Errorf("set %d: %v", i, err)
			}
		}(i)
	}
	wg.Wait()
	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 100 {
		t.Fatalf("残った件数 = %d", len(got))
	}
}

func TestDynamoLink_同じBacklogユーザーは拒否し付け替えで占有を外す(t *testing.T) {
	svc, _ := newDynamoService(nil)
	ctx := context.Background()
	if err := svc.Set(ctx, "111", Link{BacklogID: 1, Name: "大橋和幸"}); err != nil {
		t.Fatal(err)
	}
	err := svc.Set(ctx, "222", Link{BacklogID: 1, Name: "大橋和幸"})
	if err == nil || !strings.Contains(err.Error(), "既に <@111>") {
		t.Fatalf("二重登録を拒否しない: %v", err)
	}
	if err := svc.Set(ctx, "111", Link{BacklogID: 9, Name: "大橋和幸"}); err != nil {
		t.Fatal(err)
	}
	if err := svc.Set(ctx, "222", Link{BacklogID: 1, Name: "大橋和幸"}); err != nil {
		t.Fatalf("付け替え後の旧IDを使えない: %v", err)
	}
	ok, err := svc.Remove(ctx, "222")
	if err != nil || !ok {
		t.Fatalf("解除できない: %v %v", ok, err)
	}
	ok, err = svc.Remove(ctx, "222")
	if err != nil || ok {
		t.Fatalf("未登録の解除が成功扱い: %v %v", ok, err)
	}
}

func TestDynamoLink_既存のSSMを移行し壊れた表は空にしない(t *testing.T) {
	raw := `{"111":{"id":1,"name":"大橋和幸"}}`
	legacy := &ssmLinkStore{client: &memSSM{value: &raw}, paramName: "/p/links"}
	svc, db := newDynamoService(legacy)
	// 移行より先に DynamoDB へ入った行は、SSM の古い値で上書きしない。
	if err := db.putDiscord("111", Link{BacklogID: 1, Name: "新しい名前"}); err != nil {
		t.Fatal(err)
	}
	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got["111"].Name != "新しい名前" {
		t.Fatalf("移行が新しい行を上書きした: %+v", got["111"])
	}

	broken := "{not json"
	bad := &LinkService{store: newDynamoLinkStoreWith(newMemDynamo(), "links", &ssmLinkStore{client: &memSSM{value: &broken}, paramName: "/p"})}
	if _, err := bad.List(context.Background()); err == nil || !strings.Contains(err.Error(), "形式が不正") {
		t.Fatalf("壊れた表を空として読んだ: %v", err)
	}
}

func TestDynamoLink_同じDiscordへの同時更新は占有を残さない(t *testing.T) {
	svc, db := newDynamoService(nil)
	var wg sync.WaitGroup
	for _, backlogID := range []int{1, 2} {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			_ = svc.Set(context.Background(), "same", Link{BacklogID: id, Name: "人"})
		}(backlogID)
	}
	wg.Wait()
	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("残った Discord 行 = %d", len(got))
	}
	owner := got["same"].BacklogID
	other := 1
	if owner == 1 {
		other = 2
	}
	out, err := db.GetItem(context.Background(), &dynamodb.GetItemInput{Key: map[string]dynamotypes.AttributeValue{"pk": sAttr(backlogPK(other))}})
	if err != nil {
		t.Fatal(err)
	}
	if len(out.Item) != 0 {
		t.Fatalf("使われていない Backlog の占有が残った: %d", other)
	}
	own, err := db.GetItem(context.Background(), &dynamodb.GetItemInput{Key: map[string]dynamotypes.AttributeValue{"pk": sAttr(backlogPK(owner))}})
	if err != nil {
		t.Fatal(err)
	}
	if attrString(own.Item, "discord_id") != "same" {
		t.Fatalf("表示と占有が一致しない: %+v", own.Item)
	}
}

func TestDynamoLink_一覧は強整合で読む(t *testing.T) {
	svc, db := newDynamoService(nil)
	if _, err := svc.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if db.consistentScans == 0 || db.consistentScans != db.scans {
		t.Fatalf("強整合 %d / scan %d", db.consistentScans, db.scans)
	}
}

func TestDynamoLink_移行は足りない行だけ書く(t *testing.T) {
	raw := `{"111":{"id":1,"name":"大橋和幸"},"222":{"id":2,"name":"山田太郎"}}`
	legacy := &ssmLinkStore{client: &memSSM{value: &raw}, paramName: "/p/links"}
	svc, db := newDynamoService(legacy)
	if err := db.putDiscord("111", Link{BacklogID: 1, Name: "新しい名前"}); err != nil {
		t.Fatal(err)
	}
	if err := db.putLock(1, "111"); err != nil {
		t.Fatal(err)
	}
	before := db.puts
	got, err := svc.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got["111"].Name != "新しい名前" {
		t.Fatalf("既存の行を上書きした: %+v", got["111"])
	}
	if got["222"].Name != "山田太郎" {
		t.Fatalf("足りない行を移していない: %+v", got["222"])
	}
	// 111 の行と占有は既にある。222 の行と占有と、移行済みマーカーだけを書く。
	if wrote := db.puts - before; wrote != 3 {
		t.Fatalf("書き込み回数 = %d, want 3", wrote)
	}
}

func TestPutMissing_panicは失敗にして待つ(t *testing.T) {
	db := newMemDynamo()
	db.panicPK = "discord#1"
	store := newDynamoLinkStoreWith(db, "links", nil)
	err := store.putMissing(context.Background(), []map[string]dynamotypes.AttributeValue{
		{"pk": sAttr("discord#1")},
		{"pk": sAttr("discord#2")},
	})
	if err == nil || !strings.Contains(err.Error(), "異常終了") {
		t.Fatalf("err = %v", err)
	}
}

func (m *memDynamo) putDiscord(id string, l Link) error {
	_, err := m.PutItem(context.Background(), &dynamodb.PutItemInput{Item: discordItem(id, l)})
	return err
}

func (m *memDynamo) putLock(backlogID int, discordID string) error {
	_, err := m.PutItem(context.Background(), &dynamodb.PutItemInput{Item: lockItem(backlogID, discordID)})
	return err
}
