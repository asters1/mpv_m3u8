package main

import (
	"crypto/aes"
	"crypto/cipher"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/gin-gonic/gin"
)

var (
	MAX        int
	PROT       string
	URL_PATH   string
	HEADER     http.Header
	KEY        string
	KEY_METHOD string
	KEY_URI    string
	KEY_IV     string
	KEY_SWITCH bool
	TS_LIST    map[int]string
	TS_LIST_F  map[string]int
	TS_PATH    string
)

func clean() {
	os.RemoveAll("./m3u8_cache/")
	os.MkdirAll("./m3u8_cache", 0755)
}

func GET(URL string, HEADER http.Header) (*http.Response, error) {
	fmt.Println(URL)
	client := &http.Client{}
	requset, _ := http.NewRequest(
		http.MethodGet,
		URL,
		nil,
	)
	resp, err := client.Do(requset)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

// 解析URI和IV
func parseLineParameters(line string) map[string]string {
	linePattern := regexp.MustCompile(`([a-zA-Z-]+)=("[^"]+"|[^",]+)`)
	r := linePattern.FindAllStringSubmatch(line, -1)
	params := make(map[string]string)
	for _, arr := range r {
		params[arr[1]] = strings.Trim(arr[2], "\"")
	}
	return params
}

func JX(path string, resp *http.Response) error {
	body_bit, _ := ioutil.ReadAll(resp.Body)
	// fmt.Println(string(body_bit))
	// fmt.Println(resp.Header)
	defer resp.Body.Close()
	if resp.Header.Get("content-type") == "application/vnd.apple.mpegurl" {
		KEY_SWITCH = false
		TS_LIST = nil
		TS_LIST_F = nil
		TS_LIST = make(map[int]string)
		TS_LIST_F = make(map[string]int)
		j := 0
		f, _ := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0755)
		defer f.Close()
		m3u8_slice := strings.Split(string(body_bit), "\n")
		for i := 0; i < len(m3u8_slice); i++ {
			text := strings.TrimSpace(m3u8_slice[i])
			if (!(KEY_SWITCH)) && (strings.HasPrefix(text, "#EXT-X-KEY")) {
				KEY_SWITCH = true
				m := parseLineParameters(text)
				KEY_METHOD = m["METHOD"]
				KEY_URI = m["URI"]
				KEY_IV = m["IV"]
				if !(strings.HasPrefix(KEY_URI, "http")) {
					if strings.HasPrefix(KEY_URI, "/") {
						KEY_URI = URL_PATH + KEY_URI[1:]
					} else {
						KEY_URI = URL_PATH + KEY_URI
					}
					key_resp, err := GET(KEY_URI, HEADER)
					if err != nil {
						return err
					}
					key_bit, _ := ioutil.ReadAll(key_resp.Body)
					defer resp.Body.Close()
					KEY = string(key_bit)
				}
				text = ""
			}

			if !(strings.HasPrefix(text, "#")) {
				if strings.HasPrefix(text, "http") {
					text = "http://localhos" + PROT + "/?url=" + text
				}
				TS_LIST[j] = text
				TS_LIST_F[text] = j
				j++
			}
			f.WriteString(text + "\n")

		}
	} else {

		if KEY_SWITCH {
			b, err := AES128Decrypt(body_bit, []byte(KEY), []byte(KEY_IV))
			if err != nil {
				return err
			}
			body_bit = b
			// Some TS files do not start with SyncByte 0x47,
			// 一些 ts 文件不以同步字节 0x47 开头，
			//	they can not be played after merging,
			// 合并后不能播放，
			// Need to remove the bytes before the SyncByte 0x47(71).
			// 需要删除同步字节 0x47(71) 之前的字节。
		}

		syncByte := uint8(71) // 0x47
		bLen := len(body_bit)
		for j := 0; j < bLen; j++ {
			if body_bit[j] == syncByte {
				//			fmt.Println(bytes[:j])
				body_bit = body_bit[j:]
				break
			}
		}
		ioutil.WriteFile(path, body_bit, 0755)
	}
	return nil
}

func AES128Decrypt(crypted, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	blockSize := block.BlockSize()
	if len(iv) == 0 {
		iv = key
	}
	blockMode := cipher.NewCBCDecrypter(block, iv[:blockSize])
	origData := make([]byte, len(crypted))
	blockMode.CryptBlocks(origData, crypted)
	length := len(origData)
	unPadding := int(origData[length-1])
	origData = origData[:(length - unPadding)]
	return origData, nil
}

func DownloadTs(name string) {
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
		count := strings.Count(pa, "/")
		if count > 1 && !(strings.HasPrefix(pa, "/http")) {
			TS_PATH = pa[1:]
		} else {
			TS_PATH = url_name
		}
		fmt.Println("=====\nTS-Path:" + TS_PATH + "\n=======")
		Url := strings.TrimSpace(c.Query("url"))
		if Url != "" {
			URL_PATH = Url[:strings.LastIndex(Url, "/")+1]
			if strings.Index(Url, ".m3u") != -1 {
				clean()
			}
			HEADER = c.Request.Header
			resp, err := GET(Url, HEADER)
			if err != nil {
				c.String(460, "网络请求失败!请检查你的网络或者URL~")
				return

			}
			err = JX("./m3u8_cache/"+url_name, resp)
			if err != nil {
				c.String(460, "网络请求失败!请检查你的网络或者URL~")
				return
			}

			c.Header("Content-Length", "-1")
			c.Request.Header.Del("Range")
			c.File("./m3u8_cache/" + url_name)
			return
		} else {
			u := URL_PATH + TS_PATH
			resp, err := GET(u, HEADER)
			if err != nil {
				c.String(460, "网络请求失败!请检查你的网络或者URL~")
				return

			}
			err = JX("./m3u8_cache/"+url_name, resp)
			if err != nil {
				c.String(460, "网络请求失败!请检查你的网络或者URL~")
				return
			}
			c.File("./m3u8_cache/" + url_name)
		}
	})

	r.Run(PROT)
}
