package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	MAX      int
	PROT     string
	URL_PATH string
	HEADER   http.Header
)

func clean() {
	os.RemoveAll("./m3u8_cache/")
	os.MkdirAll("./m3u8_cache", 0755)
}

func GET(URL string, HEADER http.Header) {
}

func MainInit() {
	clean()
	MAX = 5
	PROT = ":8081"
}

func main() {
	MainInit()
	r := gin.Default()
	r.GET("/*all", func(c *gin.Context) {
		pa := c.Request.URL.RequestURI()
		url_name := pa[strings.LastIndex(pa, "/")+1:]
		Url := strings.TrimSpace(c.Query("url"))
		URL_PATH = Url[:strings.LastIndex(Url, "/")+1]
		HEADER = c.Request.Header
		if Url != "" {
		} else {
		}
		c.String(200, "url_name:"+url_name+"\nUrl:"+Url+"\nURL_PATH:"+URL_PATH)
	})

	r.Run(PROT)
}
