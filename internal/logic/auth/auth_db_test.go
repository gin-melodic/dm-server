package auth

import (
	"context"
	"testing"
	"time"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/consts"

	_ "github.com/gogf/gf/contrib/drivers/mysql/v2"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/gcfg"
	"github.com/gogf/gf/v2/util/guid"
)

func TestUserLogicDatabaseFlow(t *testing.T) {
	ctx := context.Background()
	configureAuthTestDB(t)
	if _, err := g.DB().GetValue(ctx, "SELECT 1"); err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if _, err := g.DB().GetValue(ctx, "SELECT emotion FROM dreams LIMIT 0"); err != nil {
		t.Skipf("dream API schema changes are not applied: %v", err)
	}

	userID := createAuthTestUser(t, ctx)
	t.Cleanup(func() { cleanupAuthTestUser(ctx, userID) })
	userCtx := context.WithValue(ctx, consts.CtxUserId, userID)
	svc := New()

	defaults, err := svc.GetUserSettings(userCtx, &v1.GetUserSettingsReq{})
	if err != nil {
		t.Fatalf("GetUserSettings defaults failed: %v", err)
	}
	if defaults.Language != "zh-CN" || defaults.PrivacyMode != "private" || defaults.StorageMode != "local_cache" {
		t.Fatalf("unexpected defaults: %#v", defaults)
	}

	enabled := true
	updated, err := svc.UpdateUserSettings(userCtx, &v1.UpdateUserSettingsReq{UserSettings: v1.UserSettings{
		Language:             "en-US",
		PrivacyMode:          "cloud_sync",
		DreamReminderEnabled: &enabled,
		DreamReminderTime:    "08:30",
		StorageMode:          "cloud_sync",
	}})
	if err != nil {
		t.Fatalf("UpdateUserSettings insert/upsert failed: %v", err)
	}
	if updated.Language != "en-US" || updated.DreamReminderTime != "08:30" {
		t.Fatalf("unexpected updated settings: %#v", updated)
	}

	disabled := false
	updated, err = svc.UpdateUserSettings(userCtx, &v1.UpdateUserSettingsReq{UserSettings: v1.UserSettings{
		DreamReminderEnabled: &disabled,
	}})
	if err != nil {
		t.Fatalf("UpdateUserSettings disable reminder failed: %v", err)
	}
	if updated.DreamReminderTime != "" {
		t.Fatalf("expected disabled reminder to clear time, got %#v", updated)
	}
	if _, err := svc.UpdateUserSettings(userCtx, &v1.UpdateUserSettingsReq{UserSettings: v1.UserSettings{PrivacyMode: "public"}}); err == nil {
		t.Fatal("expected invalid privacy mode to fail")
	}

	insertAuthProfileDream(t, ctx, userID, "fear", "dark,shadow")
	insertAuthProfileDream(t, ctx, userID, "peaceful", "guide,teacher")
	insertAuthProfileDream(t, ctx, userID, "happy", "family,love")
	profile, err := svc.GetPsycheProfile(userCtx, &v1.GetPsycheProfileReq{})
	if err != nil {
		t.Fatalf("GetPsycheProfile failed: %v", err)
	}
	if profile.IntegrationLevel != "moderate" || len(profile.Archetypes) != 5 || profile.DominantArchetype == "" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}

func configureAuthTestDB(t *testing.T) {
	t.Helper()
	if adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile); ok {
		if err := adapter.SetPath("../../../manifest/config"); err != nil {
			t.Fatalf("set config path: %v", err)
		}
	}
}

func createAuthTestUser(t *testing.T, ctx context.Context) uint64 {
	t.Helper()
	id, err := g.DB().Model("users").Data(g.Map{
		"openid": "auth_logic_test_" + guid.S(),
	}).InsertAndGetId()
	if err != nil {
		t.Fatalf("insert test user: %v", err)
	}
	return uint64(id)
}

func cleanupAuthTestUser(ctx context.Context, userID uint64) {
	_, _ = g.DB().Exec(ctx, "DELETE FROM analysis_sessions WHERE dream_id IN (SELECT id FROM dreams WHERE user_id = ?)", userID)
	_, _ = g.DB().Model("dreams").Where("user_id", userID).Delete()
	_, _ = g.DB().Model("user_settings").Where("user_id", userID).Delete()
	_, _ = g.DB().Model("users").Where("id", userID).Delete()
}

func insertAuthProfileDream(t *testing.T, ctx context.Context, userID uint64, emotion, tags string) {
	t.Helper()
	_, err := g.DB().Model("dreams").Data(g.Map{
		"user_id":          userID,
		"title":            "profile dream",
		"content":          "profile dream content",
		"dream_date":       time.Now().Format("2006-01-02"),
		"tags":             tags,
		"emotion":          emotion,
		"status":           "completed",
		"confidence_score": 0.86,
	}).Insert()
	if err != nil {
		t.Fatalf("insert profile dream: %v", err)
	}
}
