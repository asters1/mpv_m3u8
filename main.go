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
	"time"

	"github.com/gin-gonic/gin"
)

func clean() {
	os.RemoveAll("./m3u8_cache/")
	os.MkdirAll("./m3u8_cache", 0755)
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

func IsExists(path string) bool {
	_, err := os.Stat(path)
	if err == nil {
		return true
	} else {
		return false
	}
}

var (
	// 除去host的所有路径，第一个字符为/
	REQ_URI string
	// URL从开头到最后/的路径，最后一个字符为/
	URL_PATH string
	// 最大进程数
	MAX int
	// gin的端口
	PROT string
	// 请求头
	HEADER http.Header
	// m3u8的KEY需要请求得到
	KEY string
	// m3u8的方法，一般为NONE或者AES-128
	KEY_METHOD string
	// m3u8的地址
	KEY_URI string
	// m3u8的偏移量
	KEY_IV string
	// 是否有KEY
	KEY_SWITCH bool
	// TS的顺查和逆查
	TS_LIST   map[int]string
	TS_LIST_F map[string]int
	// 获得m3u8时的TIME
	GET_M3U8_TIME int
	// 解析m3u8时复制TIME
	GET_TS_TIME int
)

func MainInit() {
	clean()
	MAX = 5
	PROT = ":8081"
	GET_M3U8_TIME = 0
	GET_TS_TIME = 0
}

func JX(path string, resp *http.Response) error {
	body_bit, _ := ioutil.ReadAll(resp.Body)
	defer resp.Body.Close()
	if (resp.Header.Get("content-type") == "application/vnd.apple.mpegurl") || (strings.Index(resp.Request.URL.RequestURI(), ".m3u") != -1) {
		GET_TS_TIME = GET_M3U8_TIME
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
			text := strings.TrimSpace(strings.TrimSpace(m3u8_slice[i]))
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
				if (strings.HasPrefix(text, "http")) || (!strings.HasPrefix(text, "/")) {
					if text != "" {
						text = "/" + text
					}
				}
				TS_LIST[j] = text
				TS_LIST_F[text] = j
				j++
			}
			f.WriteString(text + "\n")
		}
	} else {
		if GET_M3U8_TIME != GET_TS_TIME {
			return nil
		}
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

func GET(URL string, HEADER http.Header) (*http.Response, error) {
	// fmt.Println("GET()==:" + URL)
	if strings.HasPrefix(URL, "/") {
		if strings.HasPrefix(URL, "/http") {
			URL = URL[1:]
		} else {
			URL = URL_PATH + URL[1:]
		}
	} else if !strings.HasPrefix(URL, "http") {
		err := fmt.Errorf("URL格式不正确!!\nURL:" + URL)
		// fmt.Println(err)
		return nil, err
	}

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

func DownloadTs(URL string, HEADER http.Header) {
	uname := URL[strings.LastIndex(URL, "/")+1:]
	if IsExists("./m3u8_cache/" + uname) {
		return
	}

	resp, err := GET(URL, HEADER)
	if err != nil {
		return
	}
	JX("./m3u8_cache/"+uname, resp)
}

func main() {
	MainInit()
	r := gin.Default()
	r.GET("/*all", func(c *gin.Context) {
		REQ_URI = c.Request.URL.RequestURI()

		File_Name := REQ_URI[strings.LastIndex(REQ_URI, "/")+1:]
		Url := strings.TrimSpace(c.Query("url"))
		if Url != "" {
			GET_M3U8_TIME = int(time.Now().UnixNano())
			// 清空./m3u8_cache/
			clean()
			// 重置URL_PATH,TS_LIST,TS_LIST_F
			URL_PATH = Url[:strings.LastIndex(Url, "/")+1]
			TS_LIST = nil
			TS_LIST_F = nil
			// 获得请求头
			HEADER = c.Request.Header
			resp, err := GET(Url, HEADER)
			if err != nil {
				// fmt.Println(err)
				if err != nil {
					fmt.Println(err.Error())
					c.String(460, err.Error())

					return
				}
			}
			err = JX("./m3u8_cache/"+File_Name, resp)
			c.Request.Header.Del("Range")
			c.File("./m3u8_cache/" + File_Name)
			return
		} else {
			c.Header("content-type", "video/mp2t")
			index := TS_LIST_F[REQ_URI]
			// fmt.Println(index)
			for i := 0; i < MAX; i++ {
				u, t := TS_LIST[index+i]
				// fmt.Println("t:", t)
				if t {
					// fmt.Println(u)
					go DownloadTs(u, HEADER)
					fmt.Println("进程")
				}
			}
			if IsExists("./m3u8_cache/" + File_Name) {
				c.File("./m3u8_cache/" + File_Name)
				return
			}

			resp, err := GET(REQ_URI, HEADER)
			if err != nil {
				fmt.Println(err.Error())
				c.String(460, err.Error())
				return

			}
			err = JX("./m3u8_cache/"+File_Name, resp)
			if err != nil {
				fmt.Println(err.Error())
				c.String(460, err.Error())
				return
			}
			c.Request.Header.Del("Range")
			c.File("./m3u8_cache/" + File_Name)
			return
		}

		// fmt.Println(HEADER)
		// c.String(200, "REQ_URI:"+REQ_URI+"\nFile_Name:"+File_Name+"\nURL_PATH:"+URL_PATH)
	})
	r.Run(PROT)
}
