package crawlman

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"
	"time"
)

const filePath = "./crawlmanConfig/"

func GetAllConfig() *CrawlmanNodes {
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

func (c *CrawlmanNode) toFile() {
	var f *os.File
	var err error
	mu.Lock()
	defer mu.Unlock()
	ok, _ := PathExists(filePath)
	if !ok {
		err := os.Mkdir(filePath, os.ModePerm)
		if err != nil {
			fmt.Printf("mkdir failed![%v]\n", err)
			panic(err)
		}
	}
	filename := filePath + c.Id + ".json"
	if checkFileIsExist(filename) { //如果文件存在
		f, err = os.OpenFile(filename, os.O_APPEND|os.O_WRONLY|os.O_TRUNC, 0666) //打开文件
	} else {
		f, err = os.Create(filename) //创建文件
	}
	if err != nil {
		fmt.Println(err)
		return
	}
	j, err := toJson(c)
	if err != nil {
		panic(err)
	}
	w := bufio.NewWriter(f) //创建新的 Writer 对象
	_, err = w.WriteString(j)
	if err != nil {
		panic(err)
	}
	w.Flush()
	f.Close()
}

func checkFileIsExist(filePath string) bool {
	var exist = true
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		exist = false
	}
	return exist
}

func jsonToStruct(j []byte) *CrawlmanNode {
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
	nodeStruct = jsonToStruct(content)
	nodeStruct.client = &http.Client{Timeout: time.Duration(5) * time.Second}
	nodes.Set(nodeStruct.getId(), nodeStruct)
	return nodeStruct, nil
}
