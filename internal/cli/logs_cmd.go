package cli

import (
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/highclaw/highclaw/internal/config"
	syslogger "github.com/highclaw/highclaw/internal/system/logger"
	"github.com/spf13/cobra"
)

// logsListCmd 列出所有日志文件
var logsListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all log files",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := resolveLogDir()
		files, err := syslogger.ListLogFiles(dir)
		if err != nil {
			return fmt.Errorf("list log files: %w", err)
		}
		if len(files) == 0 {
			fmt.Printf("No log files found in %s\n", dir)
			return nil
		}

		total, _ := syslogger.TotalSize(dir)
		fmt.Printf("Log files (%d, total %.1f MB):\n\n", len(files), float64(total)/1024/1024)
		for _, f := range files {
			sizeMB := float64(f.Size) / 1024 / 1024
			fmt.Printf("  %-32s  %8.2f MB  %s\n", f.Name, sizeMB, f.ModTime.Local().Format("2006-01-02 15:04:05"))
		}
		fmt.Printf("\nLog directory: %s\n", dir)
		return nil
	},
}

// logsCleanCmd 清理过期日志
var logsCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up old log files",
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, _ := config.Load()
		if cfg == nil {
			cfg = config.Default()
		}

		maxAge := cfg.Log.MaxAgeDays
		if maxAge <= 0 {
			maxAge = 30
		}

		stderrEnabled := true
		if cfg.Log.StderrEnabled != nil {
			stderrEnabled = *cfg.Log.StderrEnabled
		}

		mgr, err := syslogger.New(syslogger.Config{
			Dir:           cfg.Log.Dir,
			MaxAgeDays:    maxAge,
			MaxSizeMB:     cfg.Log.MaxSizeMB,
			StderrEnabled: stderrEnabled,
		})
		if err != nil {
			return fmt.Errorf("init logger: %w", err)
		}
		defer mgr.Close()

		removed, err := mgr.Cleanup()
		if err != nil {
			return fmt.Errorf("cleanup logs: %w", err)
		}
		if removed == 0 {
			fmt.Println("No expired log files to clean.")
		} else {
			fmt.Printf("Removed %d expired log files (older than %d days)\n", removed, maxAge)
		}
		return nil
	},
}

// logsStatusCmd 显示日志系统状态
var logsStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show log system status",
	RunE: func(cmd *cobra.Command, args []string) error {
		dir := resolveLogDir()
		files, _ := syslogger.ListLogFiles(dir)
		total, _ := syslogger.TotalSize(dir)

		fmt.Println("Log System Status:")
		fmt.Println()
		fmt.Printf("  Directory:    %s\n", dir)
		fmt.Printf("  Total files:  %d\n", len(files))
		fmt.Printf("  Total size:   %.2f MB\n", float64(total)/1024/1024)
		if len(files) > 0 {
			fmt.Printf("  Latest file:  %s\n", files[0].Name)
			fmt.Printf("  Latest time:  %s\n", files[0].ModTime.Local().Format("2006-01-02 15:04:05"))
		}

		cfg, _ := config.Load()
		if cfg == nil {
			cfg = config.Default()
		}
		fmt.Printf("  Max age:      %d days\n", cfg.Log.MaxAgeDays)
		fmt.Printf("  Max size:     %d MB per file\n", cfg.Log.MaxSizeMB)
		fmt.Printf("  Log level:    %s\n", cfg.Log.Level)
		return nil
	},
}

func init() {
	// 增强原有 logsCmd（已在 commands.go 中定义）
	logsCmd.AddCommand(logsListCmd)
	logsCmd.AddCommand(logsCleanCmd)
	logsCmd.AddCommand(logsStatusCmd)
}

// enhancedLogsTail 增强版 tail，使用日志目录中最新文件
func enhancedLogsTail(lines int, follow bool) error {
	dir := resolveLogDir()
	files, err := syslogger.ListLogFiles(dir)
	if err != nil || len(files) == 0 {
		// 回退到旧的单文件日志
		return tailLogFile(logFilePath(), lines, follow)
	}

	latest := files[0].Path
	result, err := syslogger.TailFile(latest, lines)
	if err != nil {
		return err
	}
	for _, line := range result {
		fmt.Println(line)
	}

	if !follow {
		return nil
	}

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)

	done := make(chan struct{})
	go func() {
		<-stop
		close(done)
	}()

	return syslogger.FollowFile(latest, os.Stdout, done)
}

// enhancedLogsQuery 增强版查询，搜索所有日志文件
func enhancedLogsQuery(pattern string) error {
	dir := resolveLogDir()
	files, err := syslogger.ListLogFiles(dir)
	if err != nil || len(files) == 0 {
		return queryLogFile(logFilePath(), pattern)
	}

	totalMatches := 0
	for _, f := range files {
		matches, err := syslogger.QueryFile(f.Path, pattern)
		if err != nil {
			continue
		}
		if len(matches) > 0 {
			fmt.Printf("--- %s (%d matches) ---\n", f.Name, len(matches))
			for _, line := range matches {
				fmt.Println(line)
			}
			totalMatches += len(matches)
		}
	}
	fmt.Printf("\nTotal matches: %d across %d files\n", totalMatches, len(files))
	return nil
}

func resolveLogDir() string {
	cfg, _ := config.Load()
	if cfg != nil && strings.TrimSpace(cfg.Log.Dir) != "" {
		return cfg.Log.Dir
	}
	return syslogger.DefaultConfig().Dir
}
