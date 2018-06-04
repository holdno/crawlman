package crawlman

import "sync"

type Article struct {
	Title          string `json:"title"`            // 标题
	Author         string `json:"author"`           // 作者
	Source         string `json:"source"`           // 来源
	Href           string `json:"href"`             // 文章链接
	SourceWeb      string `json:"source_web"`       // 来源链接
	SourceWebTitle string `json:"source_web_title"` // 来源站点名称
	Content        string `json:"content"`          // 文章详情
	Time           string `json:"time"`             // 发布时间
	Brief          string `json:"brief"`            // 简介
	Keyword        string `json:"keyword"`          // 关键字
	Image          string `json:"image"`            // 封面图
}

type SyncMap struct {
	Map sync.Map
}

func (m *SyncMap) Len() int {
	var i int
	m.Map.Range(func(k, v interface{}) bool {
		i++
		return true
	})
	return i
}

func (m *SyncMap) MustGet(key string) string {
	value, ok := m.Map.Load(key)
	if ok {
		return value.(string)
	} else {
		return ""
	}
}

func (m *SyncMap) Get(key string) (string, bool) {
	value, ok := m.Map.Load(key)
	if ok {
		return value.(string), ok
	} else {
		return "", ok
	}
}

func (m *SyncMap) Set(key string, value string) {
	m.Map.Store(key, value)
}

func (m *SyncMap) Range(f func(k string, v string) bool) {
	m.Map.Range(func(key, value interface{}) bool {
		ok := f(key.(string), value.(string))
		return ok
	})
}

func (m *SyncMap) Delete(key string) {
	m.Map.Delete(key)
}
