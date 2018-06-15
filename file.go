package crawlman

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

var DBbuffer interface {
	Read(id int64) (*CrawlmanNode, error)
	ReadAll() ([]*CrawlmanNode, error)
	Write(node *CrawlmanNode) error
	Delete(id int64) error
}

const filePath = "./crawlmanConfig/"

func GetAllConfig() *CrawlmanNodes {
	if DBbuffer != nil {
		nodeSlice, err := DBbuffer.ReadAll()
		if err != nil {
			panic(err)
		}
		for _, v := range nodeSlice {
			nodes.Set(v.Id, v)
		}
	} else {
		files, err := ioutil.ReadDir(filePath)
		if err != nil {
			// panic(err)
			err := os.Mkdir(filePath, os.ModePerm)
			if err != nil {
				fmt.Printf("mkdir failed![%v]\n", err)
				panic(err)
			}
		}
		for _, f := range files {
			filename := filePath + f.Name()
			if !f.IsDir() && getExt(filename) == "json" {
				// 这个操作会将所有配置信息写入nodes变量中
				LoadConfig(filename)
			}
		}
	}
	return nodes
}

//获取得文件的扩展名，最后一个.后面的内容
func getExt(f string) (ext string) {
	//  fmt.Println("ext:", f)
	index := strings.LastIndex(f, ".")
	data := []byte(f)
	for i := index + 1; i < len(data); i++ {
		ext = ext + string([]byte{data[i]})
	}
	return
}

// 判断文件夹是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

// 删除采集节点
func (c *CrawlmanNode) delete() error {
	if DBbuffer != nil {
		err := DBbuffer.Delete(c.Id)
		if err == nil {
			nodes.Delete(c.Id)
		}
		return err
	}
	var err error
	c.lock.config.Lock()
	defer c.lock.config.Unlock()
	ok, _ := PathExists(filePath)
	if !ok {
		return errors.New("path not found")
	}
	filename := fmt.Sprintf("%s%d.json", filePath, c.Id)
	if checkFileIsExist(filename) {
		err = os.Remove(filename)
		if err != nil {
			return err
		}
		nodes.Delete(c.Id)
		return nil
	}
	// 判断log目录是否存在
	// 是的话删除整个log目录
	ok, _ = PathExists(fmt.Sprintf("%slog/%d", filePath, c.Id))
	if ok {
		os.RemoveAll(fmt.Sprintf("%slog/%d", filePath, c.Id))
	}
	return errors.New("file not exist")
}

// 保存配置到配置文件
func (c *CrawlmanNode) toFile() error {
	j, err := ToJson(c)
	if err != nil {
		return err
	}
	if DBbuffer != nil {
		err = DBbuffer.Write(c)
		if err != nil {
			return err
		}
		return nil
	}
	var f *os.File
	c.lock.config.Lock()
	defer c.lock.config.Unlock()
	ok, _ := PathExists(filePath)
	if !ok {
		err = os.Mkdir(filePath, os.ModePerm)
		if err != nil {
			fmt.Printf("mkdir failed![%v]\n", err)
			return err
		}
	}
	filename := fmt.Sprintf("%s%d.json", filePath, c.Id)
	if checkFileIsExist(filename) { //如果文件存在
		f, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_TRUNC, 0666) //打开文件
	} else {
		f, err = os.Create(filename) //创建文件
	}
	if err != nil {
		return err
	}

	w := bufio.NewWriter(f) //创建新的 Writer 对象
	_, err = w.WriteString(j)
	if err != nil {
		return err
	}
	w.Flush()
	f.Close()
	return nil
}

// 将日志写入文件
func (c *CrawlmanNode) wLog(content string) {
	if content == "" {
		return
	}
	go func() {
		content = fmt.Sprintf("%s,%s\n", time.Now().Format("2006-01-02 15:04:05"), content)
		var f *os.File
		var err error
		c.lock.log.Lock()
		defer c.lock.log.Unlock()
		path := fmt.Sprintf("%slog/%d/", filePath, c.Id)
		ok, _ := PathExists(path)
		if !ok {
			err := os.MkdirAll(path, os.ModePerm)
			if err != nil {
				fmt.Printf("mkdir failed![%v]\n", err)
				panic(err)
			}
		}
		filename := path + time.Now().Format("2006-01-02") + ".json"
		if checkFileIsExist(filename) { //如果文件存在
			f, err = os.OpenFile(filename, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666) //打开文件
		} else {
			f, err = os.Create(filename) //创建文件
		}
		if err != nil {
			fmt.Println(err)
			return
		}
		w := bufio.NewWriter(f) //创建新的 Writer 对象
		_, err = w.WriteString(content)
		if err != nil {
			panic(err)
		}
		w.Flush()
		f.Close()
		fmt.Print(content)
	}()
}

func checkFileIsExist(filePath string) bool {
	var exist = true
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

func JsonToNode(j []byte) *CrawlmanNode {
	var a *CrawlmanNode
	err := json.Unmarshal(j, &a)
	if err != nil {
		// 文件内容格式出问题
		return nil
	}
	return a
}

func LoadConfig(filename string) (*CrawlmanNode, error) {
	var (
		f   *os.File
		err error
	)
	if checkFileIsExist(filename) { //如果文件存在
		f, err = os.OpenFile(filename, os.O_RDONLY, 0666) //打开文件
	} else {
		f, err = os.Create(filename) //创建文件
	}
	if err != nil {
		return nil, err
	}
	content, err := ioutil.ReadAll(f)
	if err != nil {
		return nil, err
	}
	nodeStruct = JsonToNode(content)
	if nodeStruct != nil {
		nodeStruct.client = &http.Client{Timeout: time.Duration(5) * time.Second}
		nodes.Set(nodeStruct.Id, nodeStruct)
	} else {
		panic("从配置文件中加载采集节点失败")
	}

	return nodeStruct, nil
}
