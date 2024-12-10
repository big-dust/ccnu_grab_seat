package main

import (
	"github.com/spf13/viper"
	"os"
	"testing"
	"time"
)

// TestFindVacantSeats 寻找空闲座位写入文件
func TestFindVacantSeats(t *testing.T) {
	initViper()
	conf := GrabberConfig{}
	err := viper.UnmarshalKey("grabber", &conf)
	if err != nil {
		panic(err)
	}
	grabber := NewGrabber(conf.Areas, conf.IsTomorrow, conf.StartTime, conf.EndTime)
	grabber.startFlushClient(conf.Username, conf.Password, time.Second*10)
	seats := grabber.findVacantSeats()
	// 将seats按照合适的格式写入文件
	out, err := os.OpenFile("seats.csv", os.O_CREATE|os.O_WRONLY, 0666)
	if err != nil {
		panic(err)
	}
	// 写入表头：座位号
	out.WriteString("座位号")
	for _, seat := range seats {
		out.WriteString("\n")
		out.WriteString(seat.Title)
	}
}
