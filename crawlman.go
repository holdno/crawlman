package crawlman

import (
	"bufio"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/holdno/snowFlakeByGo"
	"golang.org/x/net/html/charset"
	"golang.org/x/text/encoding"
	"golang.org/x/text/transform"
	"io"
	"math/rand"
	"net/http"
	"strings"
	"sync"
	"time"
)

type DomConfig struct {
	Aim    string `json:"aim"`
	Dom    string `json:"dom"`
	Method string `json:"method"`
	Result string `json:"result"`
}

var (
	engine    sync.Map
	idWork, _ = snowFlakeByGo.NewWorker(1)
)

type CrawlmanNode struct {
	Id            int64         `json:"id"`
	Name          string        `json:"name"`
	Interval      time.Duration `json:"interval"`
	UserAgent     []string      `json:"useragent"`
	Url           string        `json:"url"`
	Status        string        `json:"status"`
	List          string        `json:"list"`
	Config        []*DomConfig  `json:"config"`
	ContentConfig []*DomConfig  `json:"content_config"`
	Interface     interface {
		Load(a *Article)
	} `json:"-"`
	lock struct {
		config sync.Mutex
		log    sync.Mutex
	}
	warning  int
	routine  int
	client   *http.Client
	stop     chan struct{}
	wg       sync.WaitGroup
	Response interface{} `json:"response"`
}

// 获取采集节点id
func (c *CrawlmanNode) GetId() int64 {
	if c.Id != 0 {
		return c.Id
	} else {
		return idWork.GetId()
	}
}

// 健康状态监测
func (c *CrawlmanNode) Health() string {
	if c.warning == 0 {
		return "healthy"
	} else if c.warning == 1 {
		return "sub-healthy"
	} else if c.warning == 2 {
		return "unhealthy"
	} else {
		return "healthy"
	}
}

// 列表解析方法
func (c *CrawlmanNode) getList() {
	defer func() {
		if err := recover(); err != nil {
			c.wLog(fmt.Sprintf("panic:%v", err))
			c.warning = 2
			c.Stop()
		}
	}()
	req, _ := http.NewRequest(http.MethodGet, c.Url, nil)
	req.Header.Set("User-Agent", c.UserAgent[rand.Intn(len(c.UserAgent))])
	req.Header.Set("referer", c.Url)
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		c.wLog(fmt.Sprintf("%s无法访问,可能是访问过于频繁IP遭到封禁", c.Url))
		c.warning = 2
		c.Stop()
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 记录故障，网页无法打开
		if c.warning < 2 {
			c.warning = 2
		}
		c.wLog(fmt.Sprintf("目标网站无法打开，%s;", resp.Status))
		return
	}
	e, _ := determineEncoding(resp.Body)
	utf8Reader := transform.NewReader(resp.Body, e.NewDecoder())
	doc, err := goquery.NewDocumentFromReader(utf8Reader)

	if err != nil {
		// 列表采集失败
		if c.warning < 2 {
			c.warning = 2
		}
		fmt.Println(err)
		return
	}

	// var articles []*Article
	// 需要配置一个列表链接的配置
	getContent := 0
	doc.Find(c.List).Each(func(i int, selection *goquery.Selection) {
		if getContent == 0 {
			getContent = 1
		}
		go func() {
			// 获取文章url
			m := new(SyncMap)
			c.getContent(selection, m)

			article := &Article{
				Href:           m.MustGet("href"),
				Title:          m.MustGet("title"),
				Author:         m.MustGet("author"),
				Time:           m.MustGet("time"),
				Keyword:        m.MustGet("keyword"),
				Image:          m.MustGet("image"),
				Content:        m.MustGet("content"),
				Brief:          m.MustGet("brief"),
				SourceWeb:      c.Url,
				SourceWebTitle: c.Name,
			}
			// articles = append(articles, article)
			c.Interface.Load(article)
		}()
	})
	if getContent == 0 {
		// 记录当前站点dom结构发生变化-
		if c.warning < 1 {
			c.warning = 1
		}
		c.wLog("目标站点dom结构发生变化")
		c.Stop()
	}
}

// 内容解析方法
func (c *CrawlmanNode) getContent(selection *goquery.Selection, m *SyncMap) {
	var (
		content string
		exist   bool
	)
	for _, v := range c.Config {
		if v.Dom != "" && v.Result != "" {
			node := selection.Find(v.Dom)
			if v.Method != "" {
				content, exist = node.Attr(v.Method)
				if !exist {
					// 通知列表数据结构发生了变化
					if c.warning < 1 {
						c.warning = 1
					}
					c.wLog("列表数据结构发生了变化")
					return
				}
			} else if v.Method == "" {
				if v.Result == "text" {
					content = node.Text()
					if content == "" {
						fmt.Println("text:", content)
					}
				}
			}
			if v.Aim == "href" || v.Aim == "image" {
				if !strings.Contains(content, "://") {
					// lastStr := c.Url[len(c.Url)-1 : len(c.Url)]
					var (
						url  string
						http string
					)
					if strings.Contains(c.Url, "http://") {
						url = strings.Replace(c.Url, "http://", "", -1)
						http = "http://"
					} else if strings.Contains(c.Url, "https://") {
						url = strings.Replace(c.Url, "https://", "", -1)
						http = "https://"
					} else {
						url = c.Url
						http = "http://"
					}

					urls := strings.Split(url, "/")
					content = fmt.Sprintf("%s%s%s", http, urls[0], content)
					//if lastStr == "/" {
					//	content = fmt.Sprintf("%s%s", c.Url[0:len(c.Url)-1], content)
					//} else {
					//	content = fmt.Sprintf("%s%s", c.Url, content)
					//}
				}
			}
			if v.Aim == "href" && content != "" {
				req, _ := http.NewRequest(http.MethodGet, content, nil)
				req.Header.Set("User-Agent", c.UserAgent[rand.Intn(len(c.UserAgent))])
				req.Header.Set("referer", c.Url)
				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					fmt.Println(err)
					if c.warning < 2 {
						c.warning = 2
					}
					c.wLog(fmt.Sprintf("目标网站无法打开,%s;", content))
					return
				}
				defer resp.Body.Close()
				e, _ := determineEncoding(resp.Body)
				utf8Reader := transform.NewReader(resp.Body, e.NewDecoder())
				doc, err := goquery.NewDocumentFromReader(utf8Reader)
				if err != nil {
					// 列表采集失败
					fmt.Println(err)
					if c.warning < 2 {
						c.warning = 2
					}
					c.wLog(fmt.Sprintf("内容采集失败,%v;", err))
					return
				}
				for _, vv := range c.ContentConfig {
					var content string
					if vv.Dom != "" && vv.Result != "" {
						node := doc.Find(vv.Dom)
						if vv.Method != "" {
							content, exist = node.Attr(vv.Result)
							if !exist {
								// 通知列表数据结构发生了变化
								c.wLog(fmt.Sprintf("%s列表数据结构发生变化:%s", vv.Aim, vv.Dom))
							}
						} else if vv.Method == "" {
							if vv.Result == "text" {
								content = node.Text()
								if content == "" {
									fmt.Println("text:", content)
								}
							} else if vv.Result == "html" {
								var str = strings.Builder{}
								var exist = false
								node.Find("p,pre").Each(func(index int, selection *goquery.Selection) {
									exist = true
									// 增加图片采集
									img, ok := selection.Find("img").Attr("src")
									if ok {
										str.WriteString("<p><img class='wscnph' src='")
										str.WriteString(img)
										str.WriteString("'></p>")
										if m.MustGet("image") == "" {
											m.Set("image", img)
										}
									}
									text := strings.TrimSpace(selection.Text())
									if text != "" {
										str.WriteString("<p>")
										str.WriteString(text)
										str.WriteString("</p>")
									}
								})
								if exist {
									content = str.String()
								} else {
									content, err = node.Html()
									if err != nil {
										if c.warning < 2 {
											c.warning = 2
										}
										c.wLog(fmt.Sprintf("富文本内容采集失败,%v;", err))
										return
									}
								}
							}
						}
					}
					m.Set(vv.Aim, strings.TrimSpace(content))
				}
			}
			m.Set(v.Aim, strings.TrimSpace(content))
		}
	}
}

// 该方法用于将crawlman注册进采集列表，如果不调用该方法，新的配置将会丢失
// 传入的接口a好比是crawlman办公的工具，上战场不带枪可不行
func (c *CrawlmanNode) JoinJob(a interface{ Load(a *Article) }) error {
	if c.Name == "" {
		return errors.New("目标站点名称不能为空")
	}
	if c.List == "" {
		return errors.New("目标站点列表节点配置不能为空")
	}
	if c.Config == nil {
		return errors.New("目标站点列表采集配置不能为空")
	}
	//if c.ContentConfig == nil {
	//	return errors.New("目标站点内容采集配置不能为空")
	//}
	if a == nil {
		return errors.New("请配置Load接口")
	}
	// TODO 判断当前node是否已经存在定时任务
	c.Interface = a
	c.stop = make(chan struct{})
	if c.Id == 0 {
		c.Id = c.GetId()
	}
	nodes.Set(c.Id, c)
	if c.Status == "open" {
		go c.start()
	} else {
		c.toFile()
	}
	return nil
}

func Start(id int64) bool {
	node, exist := GetNode(id)
	if exist {
		go node.start()
		return true
	}
	return false
}

// 新建节点
func NewCrawlerNode() *CrawlmanNode {
	c := &CrawlmanNode{
		Status: "open",
		client: &http.Client{Timeout: time.Duration(5) * time.Second},
		stop:   make(chan struct{}),
	}
	return c
}

func (c *CrawlmanNode) start() {
	nodes, ok := engine.Load(c.Id)
	if ok {
		nodes.(*CrawlmanNode).Stop()
	}
	c.stop = make(chan struct{})
	c.wLog(c.Name + ",crawler start")
	c.Status = "open"
	engine.Store(c.Id, 1)
	c.toFile()
	ticker := time.NewTicker(c.Interval * time.Second)
	for {
		select {
		case <-ticker.C:
			c.getList()
		case <-c.stop:
			engine.Delete(c.Id)
			c.wLog(c.Name + ",crawler stop")
			ticker.Stop()
			return
		}
	}
}

// 启动全部采集任务
func StartAll(a interface{ Load(a *Article) }) {
	configs := GetAllConfig()
	configs.Range(func(k int64, v *CrawlmanNode) bool {
		v.Interface = a
		go v.start()
		return true
	})
}

// 停止全部采集任务
func StopAll() {
	nodes.Range(func(k int64, v *CrawlmanNode) bool {
		v.Stop()
		return true
	})
}

func (c *CrawlmanNode) Stop() {
	c.Status = "close"
	c.toFile()
	close(c.stop)
}

func (c *CrawlmanNode) Error(str string) {
	c.warning = 2
	c.wLog(str)
}

func determineEncoding(r io.Reader) (encoding.Encoding, string) {
	bytes, err := bufio.NewReader(r).Peek(1024)
	if err != nil {
		panic(err)
	}
	e, n, _ := charset.DetermineEncoding(bytes, "")
	return e, n
}

func GetNode(id int64) (*CrawlmanNode, bool) {
	node, ok := nodes.Get(id)
	if !ok {
		return nil, ok
	}
	return node, ok
}

func GetNodes() *CrawlmanNodes {
	return nodes
}

func Delete(id int64) error {
	node, ok := nodes.Get(id)
	if !ok {
		return errors.New("id not exist")
	}
	_, ok = engine.Load(id)
	if ok {
		node.Stop()
	}
	engine.Delete(id)
	err := node.delete()
	if err != nil {
		return err
	}
	return nil
}
