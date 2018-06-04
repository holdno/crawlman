package crawlman

import (
	"fmt"
	"testing"
)

type A struct {
}

func (a *A) Load(article *Article) {
	fmt.Println(article)
}

func TestCrawlerMan(t *testing.T) {
	NewCrawlerNode()
	// nodes := GetAllConfig()
	var a = new(A)
	StartAll(a)
}
