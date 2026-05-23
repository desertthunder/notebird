package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/fang"
	"github.com/charmbracelet/lipgloss/v2"
	"github.com/desertthunder/notebird/internal/core"
	"github.com/spf13/cobra"
)

func Execute(ctx context.Context, args []string) error {
	cfg := core.NewConfig("127.0.0.1", 7331)
	root := &cobra.Command{
		Use:   "notebird",
		Short: "A tiny local personal wiki.",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd.Context(), cfg)
		},
	}
	root.SetArgs(args)

	root.PersistentFlags().StringVar(&cfg.DataDir, "data-dir", defaultDataDir(), "directory for the SQLite database and local data")
	root.PersistentFlags().StringVar(&cfg.Host, "host", cfg.Host, "HTTP host")
	root.PersistentFlags().IntVar(&cfg.Port, "port", cfg.Port, "HTTP port")

	serve := &cobra.Command{
		Use:   "serve",
		Short: "Run the local Notebird web server",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runServe(cmd.Context(), cfg)
		},
	}
	root.AddCommand(serve)
	return fang.Execute(ctx, root, fang.WithVersion("0.1.0"))
}

func runServe(ctx context.Context, cfg core.Config) error {
	wordmark := lipgloss.NewStyle().Foreground(lipgloss.Color("33")).Bold(true).Render("Notebird")
	fmt.Fprintf(os.Stderr, "%s starting on http://%s:%d\n", wordmark, cfg.Host, cfg.Port)

	app, err := core.New(cfg)
	if err != nil {
		return err
	}
	defer app.Close()
	return app.Serve(ctx)
}

func defaultDataDir() string {
	if base, err := os.UserConfigDir(); err == nil {
		return filepath.Join(base, "notebird")
	}
	return ".notebird"
}
