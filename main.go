package main

import (
	"encoding/json"
	"fmt"
	"github.com/spf13/pflag"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/anaskhan96/soup"
	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
)

type Grabber struct {
	authClient *http.Client
	areas      []string // 目标区域
	isTomorrow bool     // 是否是明天, false：默认抢今天
	start      string   // 目标起始时间
	end        string   // 目标结束时间
	searchUrl  string
	grabUrl    string
}

type GrabberConfig struct {
	Areas      []string `yaml:"areas"`
	IsTomorrow bool     `yaml:"isTomorrow"`
	StartTime  string   `yaml:"starTime"`
	EndTime    string   `yaml:"endTime"`
	Username   string   `yaml:"username"`
	Password   string   `yaml:"password"`
}

func main() {
	initViper()
	conf := GrabberConfig{}
	err := viper.UnmarshalKey("grabber", &conf)
	if err != nil {
		panic(err)
	}
	grabber := NewGrabber(conf.Areas, conf.IsTomorrow, conf.StartTime, conf.EndTime)
	grabber.startFlushClient(conf.Username, conf.Password, time.Second*10)
	for {
		// 扫描出空位置
		devId := grabber.searchVacantSeats()
		if devId == "" {
			time.Sleep(time.Second * 1)
			continue
		}
		// 选上
		grabber.grab(devId)
		// 二次成功验证
		if grabber.grabSuccess() {
			// 结束
			fmt.Println("=============抢座成功=============")
			break
		}
		// 二次验证失败，继续
	}
}

func initViper() {
	cfile := pflag.String("config", "config.yaml", "配置文件路径")
	pflag.Parse()

	viper.SetConfigType("yaml")
	viper.SetConfigFile(*cfile)
	err := viper.ReadInConfig()
	if err != nil {
		panic(err)
	}
}

func (g *Grabber) startFlushClient(username, password string, dur time.Duration) {
	authClient := g.getClient(username, password)
	g.setAuthClient(authClient)
	go func() {
		for {
			authClient := g.getClient(username, password)
			g.setAuthClient(authClient)
			time.Sleep(dur)
		}
	}()
}

func (g *Grabber) setAuthClient(authClient *http.Client) {
	g.authClient = authClient
}

func NewGrabber(areas []string, isTomorrow bool, start string, end string) *Grabber {
	return &Grabber{
		areas:      areas,
		isTomorrow: isTomorrow,
		start:      start,
		end:        end,
		searchUrl:  "http://kjyy.ccnu.edu.cn/ClientWeb/pro/ajax/device.aspx",
		grabUrl:    "http://kjyy.ccnu.edu.cn/ClientWeb/pro/ajax/reserve.aspx",
	}
}

type searchResp struct {
	Data []struct {
		Title string        `json:"title"`
		Ts    []interface{} `json:"ts"`
		DevId string        `json:"devId"`
	}
}

func (g *Grabber) searchVacantSeats() string { // 得到一个空闲座位号
	for _, area := range g.areas {
		dateTime := time.Now()
		if g.isTomorrow {
			dateTime = dateTime.Add(time.Hour * 24)
		}
		year, month, day := dateTime.Date()

		params := url.Values{}
		params.Set("byType", "devcls")
		params.Set("classkind", "8")
		params.Set("display", "fp")
		params.Set("md", "d")
		params.Set("room_id", area)
		params.Set("purpose", "")
		params.Set("selectOpenAty", "")
		params.Set("cld_name", "default")
		params.Set("date", fmt.Sprintf("%d-%02d-%02d", year, month, day))
		params.Set("fr_start", g.start)
		params.Set("fr_end", g.end)
		params.Set("act", "get_rsv_sta")
		params.Set("_", "16698463729090")
		parsedSearchUrl, _ := url.Parse(g.searchUrl)
		cookies := g.authClient.Jar.Cookies(parsedSearchUrl)

		client, bodyData := resty.New(), &searchResp{}
		_, _ = client.SetCookies(cookies).R().SetQueryParamsFromValues(params).SetResult(&bodyData).Get(g.searchUrl)

		for _, locationInfo := range bodyData.Data {
			if len(locationInfo.Ts) != 0 {
				continue
			}
			return locationInfo.DevId
		}
	}
	return ""
}

func (g *Grabber) grab(devId string) {
	dateTime := time.Now()
	if g.isTomorrow {
		dateTime = dateTime.Add(time.Hour * 24)
	}
	year, month, day := dateTime.Date()

	params := url.Values{}
	params.Set("dialogid", "")
	params.Set("dev_id", devId)
	params.Set("lab_id", "")
	params.Set("kind_id", "")
	params.Set("room_id", "")
	params.Set("type", "dev")
	params.Set("prop", "")
	params.Set("test_id", "")
	params.Set("term", "")
	params.Set("Vnumber", "")
	params.Set("classkind", "")
	params.Set("test_name", "")
	params.Set("start", fmt.Sprintf("%d-%02d-%02d %s", year, month, day, g.start))
	params.Set("end", fmt.Sprintf("%d-%02d-%02d %s", year, month, day, g.end))
	params.Set("start_time", "1000")
	params.Set("end_time", "2200")
	params.Set("up_file", "")
	params.Set("memo", "")
	params.Set("act", "set_resv")
	params.Set("_", "17048114508")

	parsedUrl, _ := url.Parse(g.grabUrl)
	cookies := g.authClient.Jar.Cookies(parsedUrl)

	client := resty.New()
	_, _ = client.SetCookies(cookies).R().
		SetQueryParamsFromValues(params).
		SetHeader("Accept", "application/json, text/javascript, */*; q=0.01").
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6").
		SetHeader("Connection", "keep-alive").
		SetHeader("Referer", "http://kjyy.ccnu.edu.cn/clientweb/xcus/ic2/Default.aspx").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetHeader("X-Requested-With", "XMLHttpRequest").
		Get(parsedUrl.String())
}

func (g *Grabber) getClient(username, password string) *http.Client {
	resp, err := soup.Get("https://account.ccnu.edu.cn/cas/login?service=http://kjyy.ccnu.edu.cn/loginall.aspx?page=")
	if err != nil {
		log.Fatalf("Failed to get login page: %v", err)
	}
	doc := soup.HTMLParse(resp)
	jsessionID := doc.Find("body", "id", "cas").FindAll("script")[2].Attrs()["src"]
	ltValue := doc.Find("div", "class", "logo").FindAll("input")[2].Attrs()["value"]

	jar, _ := cookiejar.New(&cookiejar.Options{})
	client := &http.Client{
		Jar:     jar,
		Timeout: 5 * time.Second,
	}

	jsessionID = jsessionID[26:]
	loginURL := fmt.Sprintf("https://account.ccnu.edu.cn/cas/login;jsessionid=%v?service=http://kjyy.ccnu.edu.cn/loginall.aspx?page=", jsessionID)
	formData := fmt.Sprintf("username=%v&password=%v&lt=%v&execution=e1s1&_eventId=submit&submit=登录", username, password, ltValue)
	body := strings.NewReader(formData)

	req, _ := http.NewRequest("POST", loginURL, body)
	req.Header = http.Header{
		"Accept":                    {"text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8"},
		"Accept-Encoding":           {"gzip, deflate, br"},
		"Accept-Language":           {"zh-CN,zh;q=0.8,zh-TW;q=0.7,zh-HK;q=0.5,en-US;q=0.3,en;q=0.2"},
		"Cache-Control":             {"max-age=0"},
		"Connection":                {"keep-alive"},
		"Content-Length":            {"162"},
		"Content-Type":              {"application/x-www-form-urlencoded"},
		"Cookie":                    {"JSESSIONID=" + jsessionID},
		"Host":                      {"account.ccnu.edu.cn"},
		"Origin":                    {"https://account.ccnu.edu.cn"},
		"Referer":                   {"https://account.ccnu.edu.cn/cas/login?service=http://kjyy.ccnu.edu.cn/loginall.aspx?page="},
		"Sec-Fetch-Dest":            {"document"},
		"Sec-Fetch-Mode":            {"navigate"},
		"Sec-Fetch-Site":            {"same-origin"},
		"Sec-Fetch-User":            {"?1"},
		"Upgrade-Insecure-Requests": {"1"},
		"User-Agent":                {"Mozilla/5.0 (Windows NT 10.0; Win64; x64; rv:107.0) Gecko/20100101 Firefox/107.0"},
		"sec-ch-ua":                 {""},
		"sec-ch-ua-mobile":          {"?0"},
		"sec-ch-ua-platform":        {"Windows"},
	}

	res, err := client.Do(req)
	if err != nil {
		log.Fatalf("Failed to execute login request: %v", err)
	}
	defer res.Body.Close()

	return client
}

func (g *Grabber) grabSuccess() bool {
	params := url.Values{}
	params.Set("act", "get_History_resv")
	params.Set("strat", "90")
	params.Set("StatFlag", "New")
	params.Set("_", "1704815632495")

	parsedUrl, _ := url.Parse("http://kjyy.ccnu.edu.cn/ClientWeb/pro/ajax/center.aspx")
	cookies := g.authClient.Jar.Cookies(parsedUrl)

	client := resty.New()
	resp, err := client.SetCookies(cookies).R().
		SetQueryParamsFromValues(params).
		SetHeader("Accept", "application/json, text/javascript, */*; q=0.01").
		SetHeader("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.8,en-GB;q=0.7,en-US;q=0.6").
		SetHeader("Connection", "keep-alive").
		SetHeader("Referer", "http://kjyy.ccnu.edu.cn/clientweb/xcus/ic2/Default.aspx").
		SetHeader("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36 Edg/120.0.0.0").
		SetHeader("X-Requested-With", "XMLHttpRequest").
		Get(parsedUrl.String())

	if err != nil {
		log.Fatalf("Failed to execute request: %v", err)
	}

	var response map[string]interface{}
	if err := json.Unmarshal(resp.Body(), &response); err != nil {
		log.Fatalf("Failed to unmarshal response: %v", err)
	}

	msg, ok := response["msg"].(string)
	if !ok {
		log.Fatalf("Unexpected response format")
	}

	return len(msg) > len("<tbody date='2024-01-09 13:53' state='1082265730")+10
}