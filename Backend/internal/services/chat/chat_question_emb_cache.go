package chat

import "sync"

// questionEmbCacheMax は質問側 Embedding のキャッシュ件数の上限。
// 質問は正規化後の定型文が中心なので、上限に達することは通常ない。
const questionEmbCacheMax = 512

// questionEmbCache は回答検証の質問側 Embedding をプロセス内で使い回す。
// ゼロ値で使える。上限に達したら全消去する（ponytail: LRU が必要になったら差し替える）。
type questionEmbCache struct {
	mu sync.Mutex
	m  map[string][]float32
}

func (c *questionEmbCache) get(key string) ([]float32, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.m[key]
	return v, ok
}

func (c *questionEmbCache) put(key string, v []float32) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil || len(c.m) >= questionEmbCacheMax {
		c.m = make(map[string][]float32, questionEmbCacheMax)
	}
	c.m[key] = v
}
