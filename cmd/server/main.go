package main

import (
	"context"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"cablewindow/internal/httpui"
	"cablewindow/internal/store"
	"cablewindow/internal/workflow"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stderr, nil))
	cfg, err := parseConfig(os.Args[1:])
	if err != nil {
		logger.Error("配置无效", "error", err)
		os.Exit(2)
	}
	if cfg.selfCheck {
		ctx, cancel := context.WithTimeout(context.Background(), cfg.timeout)
		defer cancel()
		if err := runSelfCheck(ctx, cfg, logger); err != nil {
			logger.Error("自检失败", "error", err)
			os.Exit(1)
		}
		logger.Info("自检通过")
		return
	}
	if err := runServer(cfg, logger); err != nil {
		logger.Error("服务退出", "error", err)
		os.Exit(1)
	}
}

func buildHandler(dataDir string, logger *slog.Logger, clock workflow.Clock) (http.Handler, error) {
	repo, err := store.Open(dataDir)
	if err != nil {
		return nil, err
	}
	service := workflow.New(repo, clock)
	return httpui.New(service, logger), nil
}

func runServer(cfg config, logger *slog.Logger) error {
	handler, err := buildHandler(cfg.dataDir, logger, nil)
	if err != nil {
		return err
	}
	listener, err := net.Listen("tcp", cfg.addr)
	if err != nil {
		return err
	}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 15 * time.Second, WriteTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second}
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	serveErr := make(chan error, 1)
	go func() { serveErr <- server.Serve(listener) }()
	logger.Info("服务已启动", "addr", listener.Addr().String(), "data_dir", cfg.dataDir)
	select {
	case sig := <-stop:
		logger.Info("收到退出信号", "signal", sig.String())
	case err := <-serveErr:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(ctx)
}
