package main

import (
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

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

// TestGrab 寻找并预约一个位置
func TestGrab(t *testing.T) {
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
		devId := grabber.findOneVacantSeat()
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

// TestIsInLibrary 当前是否在图书馆
func TestIsInLibrary(t *testing.T) {
	initViper()
	conf := GrabberConfig{}
	err := viper.UnmarshalKey("grabber", &conf)
	if err != nil {
		panic(err)
	}
	grabber := NewGrabber(conf.Areas, false, conf.StartTime, conf.EndTime)
	grabber.startFlushClient(conf.Username, conf.Password, time.Second*10)
	ot := grabber.isInLibrary(conf.IsInLibraryName)
	if ot != nil {
		fmt.Printf("在图书馆的%s，%s - %s\n", ot.Title, ot.Start, ot.End)
	} else {
		fmt.Println("不在图书馆")
	}
}
