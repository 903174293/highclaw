package cli

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/highclaw/highclaw/internal/config"
	"github.com/highclaw/highclaw/internal/system/tasklog"
	"github.com/spf13/cobra"
)

var (
	tasksLimit   int
	tasksOffset  int
	tasksAction  string
	tasksModule  string
	tasksChannel string
	tasksStatus  string
	tasksSearch  string
	tasksSince   string
	tasksUntil   string
	tasksSort    string
	tasksMaxAge  int
	tasksMaxN    int
)

// --- Tasks 命令组 ---

var tasksCmd = &cobra.Command{
	Use:   "tasks",
	Short: "Manage task audit log (activity history)",
	Long: `View and manage the task audit log.
All user operations (chat, CRUD, tool execution, config changes) are recorded here.`,
}

var tasksListCmd = &cobra.Command{
	Use:   "list",
	Short: "List recent task records",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := ensureTaskStore()
		if err != nil {
			return err
		}
		defer store.Close()

		sortDesc := true
		sortBy := "created_at"
		if tasksSort != "" {
			if strings.HasPrefix(tasksSort, "+") {
				sortDesc = false
				sortBy = strings.TrimPrefix(tasksSort, "+")
			} else if strings.HasPrefix(tasksSort, "-") {
				sortBy = strings.TrimPrefix(tasksSort, "-")
			} else {
				sortBy = tasksSort
			}
		}
		records, total, err := store.Query(tasklog.QueryParams{
			Action:   tasksAction,
			Module:   tasksModule,
			Channel:  tasksChannel,
			Status:   tasksStatus,
			Search:   tasksSearch,
			Since:    tasksSince,
			Until:    tasksUntil,
			SortBy:   sortBy,
			SortDesc: sortDesc,
			Limit:    tasksLimit,
			Offset:   tasksOffset,
		})
		if err != nil {
			return fmt.Errorf("query tasks: %w", err)
		}

		if len(records) == 0 {
			fmt.Println("No task records found.")
			return nil
		}

		fmt.Printf("Task records (%d/%d):\n\n", len(records), total)
		for _, r := range records {
			ts := formatTaskTime(r.CreatedAt)
			req := truncateString(r.RequestBody, 60)
			resp := truncateString(r.ResponseBody, 40)
			fmt.Printf("  #%-6d [%s] %-8s %-10s %s\n", r.ID, ts, r.Action, r.Module, r.Status)
			if req != "" {
				fmt.Printf("          req: %s\n", req)
			}
			if resp != "" {
				fmt.Printf("          res: %s\n", resp)
			}
			if r.DurationMs > 0 {
				fmt.Printf("          duration: %dms", r.DurationMs)
				if r.TokensInput > 0 || r.TokensOutput > 0 {
					fmt.Printf("  tokens: %d/%d", r.TokensInput, r.TokensOutput)
				}
				fmt.Println()
			}
		}

		if total > tasksOffset+tasksLimit {
			fmt.Printf("\n  ... %d more records. Use --offset %d to see next page.\n", total-tasksOffset-tasksLimit, tasksOffset+tasksLimit)
		}
		return nil
	},
}

var tasksGetCmd = &cobra.Command{
	Use:   "get [id]",
	Short: "Get task record details by ID",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := ensureTaskStore()
		if err != nil {
			return err
		}
		defer store.Close()

		var id int64
		if _, err := fmt.Sscanf(args[0], "%d", &id); err != nil {
			return fmt.Errorf("invalid task ID: %s", args[0])
		}

		rec, err := store.Get(id)
		if err != nil {
			return fmt.Errorf("get task: %w", err)
		}
		if rec == nil {
			return fmt.Errorf("task #%d not found", id)
		}

		data, _ := json.MarshalIndent(rec, "", "  ")
		fmt.Println(string(data))
		return nil
	},
}

var tasksSearchCmd = &cobra.Command{
	Use:   "search [query]",
	Short: "Full-text search task records",
	Args:  cobra.MinimumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := ensureTaskStore()
		if err != nil {
			return err
		}
		defer store.Close()

		query := strings.Join(args, " ")
		records, total, err := store.Query(tasklog.QueryParams{
			Search: query,
			Limit:  tasksLimit,
			Offset: tasksOffset,
		})
		if err != nil {
			return fmt.Errorf("search tasks: %w", err)
		}

		if len(records) == 0 {
			fmt.Println("No matching task records found.")
			return nil
		}

		fmt.Printf("Search results for %q (%d/%d):\n\n", query, len(records), total)
		for _, r := range records {
			ts := formatTaskTime(r.CreatedAt)
			fmt.Printf("  #%-6d [%s] %-8s %-10s %s\n", r.ID, ts, r.Action, r.Module, r.Status)
			fmt.Printf("          req: %s\n", truncateString(r.RequestBody, 80))
		}
		return nil
	},
}

var tasksStatsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show task log statistics",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := ensureTaskStore()
		if err != nil {
			return err
		}
		defer store.Close()

		stats, err := store.GetStats()
		if err != nil {
			return fmt.Errorf("get stats: %w", err)
		}

		fmt.Println("Task Log Statistics:")
		fmt.Println()
		fmt.Printf("  Total records:      %d\n", stats.TotalRecords)
		fmt.Printf("  Total tokens (in):  %d\n", stats.TotalTokensIn)
		fmt.Printf("  Total tokens (out): %d\n", stats.TotalTokensOut)
		fmt.Printf("  Avg duration:       %.0fms\n", stats.AvgDurationMs)
		if stats.EarliestRecord != "" {
			fmt.Printf("  Earliest record:    %s\n", formatTaskTime(stats.EarliestRecord))
		}
		if stats.LatestRecord != "" {
			fmt.Printf("  Latest record:      %s\n", formatTaskTime(stats.LatestRecord))
		}

		if len(stats.ByAction) > 0 {
			fmt.Println("\n  By Action:")
			for k, v := range stats.ByAction {
				fmt.Printf("    %-12s %d\n", k, v)
			}
		}
		if len(stats.ByModule) > 0 {
			fmt.Println("\n  By Module:")
			for k, v := range stats.ByModule {
				fmt.Printf("    %-12s %d\n", k, v)
			}
		}
		if len(stats.ByStatus) > 0 {
			fmt.Println("\n  By Status:")
			for k, v := range stats.ByStatus {
				fmt.Printf("    %-12s %d\n", k, v)
			}
		}

		fmt.Printf("\n  Database: %s\n", tasklogDBPath())
		return nil
	},
}

var tasksCleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean up old task records",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := ensureTaskStore()
		if err != nil {
			return err
		}
		defer store.Close()

		maxAge := tasksMaxAge
		if maxAge <= 0 {
			maxAge = 90
		}
		maxN := tasksMaxN
		if maxN <= 0 {
			maxN = 100000
		}

		deleted, err := store.Cleanup(maxAge, maxN)
		if err != nil {
			return fmt.Errorf("cleanup tasks: %w", err)
		}

		if deleted == 0 {
			fmt.Println("No records to clean.")
		} else {
			fmt.Printf("Cleaned %d task records (max-age=%d days, max-records=%d)\n", deleted, maxAge, maxN)
		}
		return nil
	},
}

var tasksCountCmd = &cobra.Command{
	Use:   "count",
	Short: "Show total task record count",
	RunE: func(cmd *cobra.Command, args []string) error {
		store, err := ensureTaskStore()
		if err != nil {
			return err
		}
		defer store.Close()

		cnt, err := store.Count()
		if err != nil {
			return err
		}
		fmt.Printf("Total task records: %d\n", cnt)
		return nil
	},
}

func init() {
	tasksListCmd.Flags().IntVar(&tasksLimit, "limit", 20, "Max records to return")
	tasksListCmd.Flags().IntVar(&tasksOffset, "offset", 0, "Offset for pagination")
	tasksListCmd.Flags().StringVar(&tasksAction, "action", "", "Filter by action type")
	tasksListCmd.Flags().StringVar(&tasksModule, "module", "", "Filter by module")
	tasksListCmd.Flags().StringVar(&tasksChannel, "channel", "", "Filter by channel")
	tasksListCmd.Flags().StringVar(&tasksStatus, "status", "", "Filter by status")
	tasksListCmd.Flags().StringVar(&tasksSince, "since", "", "Filter records created after this time (RFC3339, e.g. 2025-01-01T00:00:00Z)")
	tasksListCmd.Flags().StringVar(&tasksUntil, "until", "", "Filter records created before this time (RFC3339)")
	tasksListCmd.Flags().StringVar(&tasksSort, "sort", "-created_at", "Sort field with direction: -created_at, +duration_ms, -tokens_input, +action")

	tasksSearchCmd.Flags().IntVar(&tasksLimit, "limit", 20, "Max results")
	tasksSearchCmd.Flags().IntVar(&tasksOffset, "offset", 0, "Offset")

	tasksCleanCmd.Flags().IntVar(&tasksMaxAge, "max-age", 90, "Max age in days")
	tasksCleanCmd.Flags().IntVar(&tasksMaxN, "max-records", 100000, "Max total records to keep")

	tasksCmd.AddCommand(tasksListCmd)
	tasksCmd.AddCommand(tasksGetCmd)
	tasksCmd.AddCommand(tasksSearchCmd)
	tasksCmd.AddCommand(tasksStatsCmd)
	tasksCmd.AddCommand(tasksCleanCmd)
	tasksCmd.AddCommand(tasksCountCmd)
}

func ensureTaskStore() (*tasklog.Store, error) {
	cfg, err := config.Load()
	if err != nil {
		cfg = config.Default()
	}
	tlCfg := tasklog.Config{
		Dir:        cfg.TaskLog.Dir,
		MaxAgeDays: cfg.TaskLog.MaxAgeDays,
		MaxRecords: cfg.TaskLog.MaxRecords,
		Enabled:    true,
	}
	store, err := tasklog.NewStore(tlCfg)
	if err != nil {
		return nil, fmt.Errorf("open task log: %w", err)
	}
	return store, nil
}

func tasklogDBPath() string {
	cfg, _ := config.Load()
	if cfg == nil {
		cfg = config.Default()
	}
	dir := cfg.TaskLog.Dir
	if dir == "" {
		dir = config.ConfigDir() + "/state"
	}
	return dir + "/tasks.db"
}

func formatTaskTime(s string) string {
	t, err := time.Parse(time.RFC3339Nano, s)
	if err != nil {
		return s
	}
	return t.Local().Format("2006-01-02 15:04:05")
}
