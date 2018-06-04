package crawlman

import (
	"encoding/json"
	"sync"
)

var (
	mu         sync.Mutex
	nodeStruct *CrawlmanNode
	nodes      = new(CrawlmanNodes)
)

type CrawlmanNodes struct {
	Map sync.Map
}

func (m *CrawlmanNodes) Len() int {
	var i int
	m.Map.Range(func(k, v interface{}) bool {
		i++
		return true
	})
	return i
}

func (m *CrawlmanNodes) MustGet(key string) *CrawlmanNode {
	value, ok := m.Map.Load(key)
	if ok {
		return value.(*CrawlmanNode)
	} else {
		return nil
	}
}

func (m *CrawlmanNodes) Get(key string) (*CrawlmanNode, bool) {
	value, ok := m.Map.Load(key)
	if ok {
		return value.(*CrawlmanNode), ok
	} else {
		return nil, ok
	}
}

func (m *CrawlmanNodes) Set(key string, value *CrawlmanNode) {
	m.Map.Store(key, value)
}

func (m *CrawlmanNodes) ToSlice() []*CrawlmanNode {
	var s []*CrawlmanNode
	m.Map.Range(func(key, value interface{}) bool {
		s = append(s, value.(*CrawlmanNode))
		return true
	})
	return s
}

func (m *CrawlmanNodes) Range(f func(k string, v *CrawlmanNode) bool) {
	m.Map.Range(func(key, value interface{}) bool {
		ok := f(key.(string), value.(*CrawlmanNode))
		return ok
	})
}

func (m *CrawlmanNodes) Delete(key string) {
	m.Map.Delete(key)
}

func toJson(c *CrawlmanNode) (string, error) {
	jsonbyte, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(jsonbyte), nil
}

func mapToStruct(id string) []*CrawlmanNode {
	var c []*CrawlmanNode
	nodes.MustGet(id)
	nodes.Range(func(k string, v *CrawlmanNode) bool {
		c = append(c, v)
		return true
	})
	return c
}
