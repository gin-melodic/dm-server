package limiter

import (
	"context"
	"time"

	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcache"
)

type GlobalLimiter struct {
	sem     chan struct{}
	timeout time.Duration
}

// UserAnalysisLimiter User analysis limiter
type UserAnalysisLimiter struct {
	MaxCount   int           `json:"max_count"`
	TimeWindow time.Duration `json:"time_window"`
	Cache      *gcache.Cache `json:"cache"`
}

var (
	global      *GlobalLimiter
	userLimiter *UserAnalysisLimiter
)

// Init Initialize global rate limiter based on config
func Init(ctx context.Context) error {
	maxRunning := g.Cfg().MustGet(ctx, "concurrency.max_running", 3).Int()
	timeoutSec := g.Cfg().MustGet(ctx, "concurrency.acquire_timeout_sec", 1).Int()

	if maxRunning <= 0 {
		maxRunning = 1
	}
	global = &GlobalLimiter{
		sem:     make(chan struct{}, maxRunning),
		timeout: time.Duration(timeoutSec) * time.Second,
	}

	// init user limiter
	maxAnalysisCount := g.Cfg().MustGet(ctx, "concurrency.user.max_analysis_count", 10).Int()
	analysisTimeWindowHours := g.Cfg().MustGet(ctx, "concurrency.user.analysis_time_window_hours", 1).Int()

	userLimiter = &UserAnalysisLimiter{
		MaxCount:   maxAnalysisCount,
		TimeWindow: time.Duration(analysisTimeWindowHours) * time.Hour,
		Cache:      gcache.New(),
	}

	g.Log().Infof(ctx, "[Limiter] initialized, max_running=%d, acquire_timeout=%s", maxRunning, global.timeout)
	g.Log().Infof(ctx, "[UserLimiter] initialized, max_analysis_count=%d, analysis_time_window=%dh",
		maxAnalysisCount, analysisTimeWindowHours)
	return nil
}

// Acquire Request a quota; if cannot acquire within timeout, return false.
func Acquire(ctx context.Context) bool {
	if global == nil {
		// Fallback: no limit when not initialized
		return true
	}
	select {
	case global.sem <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	case <-time.After(global.timeout):
		return false
	}
}

// Release Release one quota
func Release() {
	if global == nil {
		return
	}
	select {
	case <-global.sem:
	default:
	}
}

// CheckUserAnalysisLimit check user analysis limit
func CheckUserAnalysisLimit(ctx context.Context, openid string) bool {
	if userLimiter == nil {
		// Fallback: no limit when not initialized
		return true
	}

	// get user analysis count from cache
	key := "user_analysis_limit:" + openid
	count, _ := userLimiter.Cache.GetOrSetFunc(ctx, key, func(ctx context.Context) (interface{}, error) {
		return 0, nil
	}, userLimiter.TimeWindow)

	// check if over limit
	if count.Int() >= userLimiter.MaxCount {
		return false
	}

	// increase count
	userLimiter.Cache.Set(ctx, key, count.Int()+1, userLimiter.TimeWindow)
	return true
}

// GetUserAnalysisCount get user analysis count
func GetUserAnalysisCount(ctx context.Context, openid string) int {
	if userLimiter == nil {
		return 0
	}

	key := "user_analysis_limit:" + openid
	count, _ := userLimiter.Cache.Get(ctx, key)
	return count.Int()
}
