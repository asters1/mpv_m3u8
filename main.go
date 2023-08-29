package main

import (
	"os"

	"github.com/gin-gonic/gin"
)

var (
	MAX  int
	PROT string
)

func main() {
	clean()
	r := gin.Default()
	r.GET("/*all", func(c *gin.Context) {
	})
	r.Run(PROT)
}

func clean() {
	os.RemoveAll("./m3u8_cache/")
	os.MkdirAll("./m3u8_cache", 0755)
}
