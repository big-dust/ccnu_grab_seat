package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"

	"github.com/anaskhan96/soup"
	"github.com/go-resty/resty/v2"
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
	Areas           []string `yaml:"areas"`
	IsTomorrow      bool     `yaml:"isTomorrow"`
	StartTime       string   `yaml:"starTime"`
	EndTime         string   `yaml:"endTime"`
	Username        string   `yaml:"username"`
	Password        string   `yaml:"password"`
	IsInLibraryName string   `yaml:"isInLibraryName"`
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

type seat struct {
	Title string `json:"title"`
	Ts    []ts   `json:"ts"`
	DevId string `json:"devId"`
}

type ts struct { // 预约信息
	Start string `json:"start"`
	End   string `json:"end"`
	Owner string `json:"owner"`
	State string `json:"state"`
}

type searchResp struct {
	Data []seat
}

func (g *Grabber) findOneVacantSeat() string { // 得到一个空闲座位号
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
			isConflict := false
			for _, t := range locationInfo.Ts {
				// t.Start, t.End, 的结构都是2024-12-10 08:20这样的
				// 需要忽略前面的日期部分，只比较时间部分
				start, end := t.Start[len(t.Start)-5:len(t.Start)], t.End[len(t.End)-5:len(t.End)]
				if !(g.start > end || g.end < start) {
					// 冲突，该座位不能预约
					isConflict = true
					break
				}
			}
			if !isConflict {
				// 不冲突
				return locationInfo.DevId
			}
		}
	}
	return ""
}

func (g *Grabber) findVacantSeats() []seat { // 得到所有空闲座位
	vacantSeats := make([]seat, 0)
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
			isConflict := false
			for _, t := range locationInfo.Ts {
				// t.Start, t.End, 的结构都是2024-12-10 08:20这样的
				// 需要忽略前面的日期部分，只比较时间部分
				start, end := t.Start[len(t.Start)-5:len(t.Start)], t.End[len(t.End)-5:len(t.End)]
				if start < g.start && g.start < end || start < g.end && g.end < end {
					// 冲突，该座位不能预约
					isConflict = true
					break
				}
			}
			if !isConflict {
				// 不冲突
				vacantSeats = append(vacantSeats, locationInfo)
			}
		}
	}
	return vacantSeats
}

type occupant struct {
	Title string
	Name  string
	Start string
	End   string
}

func (g *Grabber) isInLibrary(name string) *occupant {
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
			for _, t := range locationInfo.Ts {
				if t.Owner == name && t.State == "doing" {
					return &occupant{
						Title: locationInfo.Title,
						Name:  name,
						Start: t.Start[len(t.Start)-5:],
						End:   t.End[len(t.End)-5:],
					}
				}
			}
		}
	}
	return nil
}

func (g *Grabber) seatToName(seat string) []ts {
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
			if locationInfo.Title == seat {
				return locationInfo.Ts
			}
		}
	}
	return nil
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
