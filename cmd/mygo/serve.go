package main

import (
	"context"
	"fmt"
	"mygo/internal/domain/config"
	"mygo/internal/serve"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	cfg, _ := config.Load("./site.yaml")
	if err := cfg.Validate(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(2)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	s, err := serve.New(cfg, ".mygo/index.db", cfg.Build.ThemeDir, cfg.Site.Theme)
	if err != nil {
		fmt.Fprintln(os.Stderr, "serve init error:", err.Error())
		os.Exit(1)
	}
	defer s.Close()

	if err := s.ListenAndServe(ctx, ":8080"); err != nil {
		fmt.Fprintln(os.Stderr, "serve error:", err.Error())
		os.Exit(1)
	}
}
