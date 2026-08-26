package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

type config struct {
	addr      string
	dataDir   string
	selfCheck bool
	timeout   time.Duration
}

func parseConfig(args []string) (config, error) {
	fs := flag.NewFlagSet("海缆维护窗口放行台", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:19081", "HTTP 监听地址")
	dataDir := fs.String("data-dir", "./data", "本地持久化目录")
	selfCheck := fs.Bool("self-check", false, "执行有界全流程自检后退出")
	timeout := fs.Duration("self-check-timeout", 20*time.Second, "自检整体截止时间")
	if err := fs.Parse(args); err != nil {
		return config{}, err
	}
	explicitAddr := false
	fs.Visit(func(f *flag.Flag) {
		if f.Name == "addr" {
			explicitAddr = true
		}
	})
	resolved := *addr
	if !explicitAddr {
		if portText := strings.TrimSpace(os.Getenv("PORT")); portText != "" {
			port, err := strconv.Atoi(portText)
			if err != nil || port < 1 || port > 65535 {
				return config{}, errors.New("PORT 必须是 1 到 65535 的端口号")
			}
			resolved = net.JoinHostPort("127.0.0.1", strconv.Itoa(port))
		}
	}
	if err := validateAddr(resolved); err != nil {
		return config{}, err
	}
	if strings.TrimSpace(*dataDir) == "" {
		return config{}, errors.New("data-dir 不能为空")
	}
	if *timeout < time.Second || *timeout > 2*time.Minute {
		return config{}, errors.New("self-check-timeout 必须在 1 秒到 2 分钟之间")
	}
	return config{addr: resolved, dataDir: *dataDir, selfCheck: *selfCheck, timeout: *timeout}, nil
}

func validateAddr(addr string) error {
	host, portText, err := net.SplitHostPort(addr)
	if err != nil {
		return fmt.Errorf("addr 必须为 host:port: %w", err)
	}
	port, err := strconv.Atoi(portText)
	if err != nil || port < 1 || port > 65535 {
		return errors.New("addr 端口必须在 1 到 65535 之间")
	}
	ip := net.ParseIP(host)
	if host != "localhost" && (ip == nil || !ip.IsLoopback()) {
		return errors.New("addr 必须绑定回环地址")
	}
	return nil
}
