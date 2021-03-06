package music

import (
	"github.com/tidwall/gjson"
	"io/ioutil"
	"net/http"
	"net/url"
)

func QueryNeteaseMusic(musicName string) string {
	client := http.Client{}
	req, err := http.NewRequest("GET", "http://music.163.com/api/search/get?type=1&s="+url.QueryEscape(musicName), nil)
	if err != nil {
		return ""
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/87.0.4280.88 Safari/537.36 Edg/87.0.664.66")
	res, err := client.Do(req)
	if err != nil {
		return ""
	}
	data, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return ""
	}
	return gjson.ParseBytes(data).Get("result.songs.0.id").String()
}
