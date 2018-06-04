package crawlman

import (
	"bufio"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
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

var UserAgent = []string{
	"Mozilla/5.0 (Macintosh; Intel Mac OS X 10_13_2) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/64.0.3282.140 Safari/537.36"}
var wg sync.WaitGroup

type DomConfig struct {
	Aim    string
	Dom    string
	Method string
	Point  string
}

type CrawlmanNode struct {
	Id            string        `json:"-"`
	Name          string        `json:"name"`
	Interval      time.Duration `json:"interval"`
	client        *http.Client  `json:"-"`
	UserAgent     []string      `json:"useragent"`
	Url           string        `json:"url"`
	Status        string        `json:"status"`
	stop          chan struct{}
	routine       int
	List          string       `json:"list"`
	Config        []*DomConfig `json:"config"`
	ContentConfig []*DomConfig `json:"content_config"`
	system        interface {
		Load(a *Article)
	}
}

func (c *CrawlmanNode) getId() string {
	return base64.StdEncoding.EncodeToString([]byte(c.Name))
}

func (c *CrawlmanNode) getList() {
	req, _ := http.NewRequest(http.MethodGet, c.Url, nil)
	req.Header.Set("User-Agent", c.UserAgent[rand.Intn(len(c.UserAgent))])
	req.Header.Set("referer", c.Url)
	resp, err := c.client.Do(req)
	if err != nil {
		fmt.Println(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		// 记录故障，网页无法打开
		fmt.Println("网络请求状态码:", resp.StatusCode)
		return
	}
	e, _ := determineEncoding(resp.Body)
	utf8Reader := transform.NewReader(resp.Body, e.NewDecoder())
	doc, err := goquery.NewDocumentFromReader(utf8Reader)

	if err != nil {
		// 列表采集失败
		fmt.Println(err)
	}

	// var articles []*Article
	// 需要配置一个列表链接的配置
	getContent := 0
	doc.Find(c.List).Each(func(i int, selection *goquery.Selection) {
		if getContent == 0 {
			getContent = 1
		}
		// 获取文章url
		m := new(SyncMap)
		c.getContent(selection, m)

		article := &Article{
			Href:    m.MustGet("href"),
			Title:   m.MustGet("title"),
			Author:  m.MustGet("author"),
			Time:    m.MustGet("time"),
			Keyword: m.MustGet("keyword"),
			Image:   m.MustGet("image"),
			Content: m.MustGet("content"),
			Brief:   m.MustGet("brief"),
		}
		// articles = append(articles, article)
		c.system.Load(article)
	})
	if getContent == 0 {
		// 记录当前站点dom结构发生变化-
		fmt.Println("目标站点dom结构发生变化")
		c.Stop()
	}
}

func (crawler *CrawlmanNode) getContent(selection *goquery.Selection, m *SyncMap) {
	var (
		content string
		exist   bool
	)
	for _, v := range crawler.Config {
		if v.Dom != "" && v.Point != "" {
			node := selection.Find(v.Dom)
			if v.Method == "attr" {
				content, exist = node.Attr(v.Point)
				if !exist {
					// 通知列表数据结构发生了变化
				}
			} else if v.Method == "" {
				if v.Point == "text" {
					content = node.Text()
					if content == "" {
						fmt.Println("text:", content)
					}
				}
			}
			if v.Aim == "href" && content != "" {
				go func(content string) {
					req, _ := http.NewRequest(http.MethodGet, content, nil)
					req.Header.Set("User-Agent", crawler.UserAgent[rand.Intn(len(crawler.UserAgent))])
					req.Header.Set("referer", crawler.Url)
					resp, err := crawler.client.Do(req)
					if err != nil {
						fmt.Println(err)
						return
					}
					doc, err := goquery.NewDocumentFromReader(resp.Body)
					if err != nil {
						// 列表采集失败
						return
					}
					defer resp.Body.Close()
					for _, v := range crawler.ContentConfig {
						if v.Dom != "" && v.Point != "" {
							node := doc.Find(v.Dom)
							if v.Method == "attr" {
								content, exist = node.Attr(v.Point)
								if !exist {
									// 通知列表数据结构发生了变化
								}
							} else if v.Method == "" {
								if v.Point == "text" {
									content = node.Text()
									if content == "" {
										fmt.Println("text:", content)
									}
								} else if v.Point == "html" {
									content, err = node.Html()
									if err != nil {
										fmt.Println("详情采集失败")
									}
								}
							}
						}
						m.Set(v.Aim, strings.TrimSpace(content))
					}
				}(content)
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
	if a == nil {
		return errors.New("请配置Load接口")
	}
	// TODO 判断当前node是否已经存在定时任务
	c.system = a
	c.stop = make(chan struct{})
	c.Id = c.getId()
	nodes.Set(c.Id, c)
	c.toFile()
	go c.start()
	return nil
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
	if c.routine != 1 {
		c.Status = "open"
		c.routine = 1
		ticker := time.NewTicker(c.Interval * time.Second)
		for {
			select {
			case <-ticker.C:
				fmt.Println("start crawler")
				c.getList()
			case <-c.stop:
				c.routine = 0
				ticker.Stop()
			}
		}
	}
}

// 启动全部采集任务
func StartAll(a interface{ Load(a *Article) }) {
	configs := GetAllConfig()
	configs.Range(func(k string, v *CrawlmanNode) bool {
		v.system = a
		go v.start()
		return true
	})
}

// 停止全部采集任务
func StopAll() {
	nodes.Range(func(k string, v *CrawlmanNode) bool {
		v.Stop()
		return true
	})
}

func (c *CrawlmanNode) Stop() {
	if c.routine != 0 {
		c.Status = "close"
		close(c.stop)
	}
}

func determineEncoding(r io.Reader) (encoding.Encoding, string) {
	bytes, err := bufio.NewReader(r).Peek(1024)
	if err != nil {
		panic(err)
	}
	e, n, c := charset.DetermineEncoding(bytes, "")
	fmt.Println(e, n, c)
	return e, n
}
