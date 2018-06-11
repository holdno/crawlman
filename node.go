package crawlman

import (
	"encoding/json"
	"sync"
)

var (
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

func (m *CrawlmanNodes) MustGet(key int64) *CrawlmanNode {
	value, ok := m.Map.Load(key)
	if ok {
		return value.(*CrawlmanNode)
	} else {
		return nil
	}
}

func (m *CrawlmanNodes) Get(key int64) (*CrawlmanNode, bool) {
	value, ok := m.Map.Load(key)
	if ok {
		return value.(*CrawlmanNode), ok
	} else {
		return nil, ok
	}
}

func (m *CrawlmanNodes) Set(key int64, value *CrawlmanNode) {
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

func (m *CrawlmanNodes) Range(f func(k int64, v *CrawlmanNode) bool) {
	m.Map.Range(func(key, value interface{}) bool {
		ok := f(key.(int64), value.(*CrawlmanNode))
		return ok
	})
}

func (m *CrawlmanNodes) Delete(key int64) {
	m.Map.Delete(key)
}

func toJson(c *CrawlmanNode) (string, error) {
	jsonbyte, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(jsonbyte), nil
}

func mapToStruct(id int64) []*CrawlmanNode {
	var c []*CrawlmanNode
	nodes.MustGet(id)
	nodes.Range(func(k int64, v *CrawlmanNode) bool {
		c = append(c, v)
		return true
	})
	return c
}
