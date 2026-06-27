package auth

import (
	"context"
	"testing"
	"time"

	v1 "dm-server/api/user/v1"
	"dm-server/internal/consts"

	_ "github.com/gogf/gf/contrib/drivers/pgsql/v2"
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
	if defaults.Language != "en" || defaults.PrivacyMode != "private" || defaults.StorageMode != "local_cache" {
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
	if updated.Language != "en" || updated.DreamReminderTime != "08:30" {
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

	emptyProfile, err := svc.GetPsycheProfile(userCtx, &v1.GetPsycheProfileReq{})
	if err != nil {
		t.Fatalf("GetPsycheProfile empty failed: %v", err)
	}
	if emptyProfile.IntegrationScore != 0 || emptyProfile.HasProfileData || emptyProfile.CompletedDreamCount != 0 {
		t.Fatalf("expected empty profile with zero score, got %#v", emptyProfile)
	}

	insertAuthProfileDream(t, ctx, userID, "fear", "dark,shadow")
	insertAuthProfileDream(t, ctx, userID, "peaceful", "guide,teacher")
	insertAuthProfileDream(t, ctx, userID, "happy", "family,love")
	profile, err := svc.GetPsycheProfile(userCtx, &v1.GetPsycheProfileReq{})
	if err != nil {
		t.Fatalf("GetPsycheProfile failed: %v", err)
	}
	if !profile.HasProfileData || profile.CompletedDreamCount != 3 {
		t.Fatalf("expected profile data count, got %#v", profile)
	}
	if profile.IntegrationLevel != "moderate" || len(profile.Archetypes) != 5 || profile.DominantArchetype == "" {
		t.Fatalf("unexpected profile: %#v", profile)
	}
}

func TestAppleLoginDatabaseFlow(t *testing.T) {
	ctx := context.Background()
	configureAuthTestDB(t)
	if _, err := g.DB().GetValue(ctx, "SELECT 1"); err != nil {
		t.Skipf("database unavailable: %v", err)
	}
	if _, err := g.DB().GetValue(ctx, "SELECT supabase_uid FROM users LIMIT 0"); err != nil {
		t.Skipf("apple auth schema changes are not applied: %v", err)
	}

	const supabaseUID = "22222222-2222-2222-2222-222222222222"
	const email = "apple-db@example.com"

	setJWTAuthTestConfig(t)
	t.Cleanup(func() {
		_, _ = g.DB().Model("users").Where("supabase_uid", supabaseUID).Delete()
	})

	svc := New()
	first, err := svc.AppleLogin(ctx, &v1.AppleAuthReq{SupabaseUid: supabaseUID, Email: email})
	if err != nil {
		t.Fatalf("AppleLogin create failed: %v", err)
	}
	if first.Token == "" || first.UserInfo == nil || first.UserInfo.Id == 0 || first.UserInfo.Email != email {
		t.Fatalf("unexpected first login response: %#v", first)
	}

	second, err := svc.AppleLogin(ctx, &v1.AppleAuthReq{SupabaseUid: supabaseUID, Email: email})
	if err != nil {
		t.Fatalf("AppleLogin reuse failed: %v", err)
	}
	if second.UserInfo == nil || second.UserInfo.Id != first.UserInfo.Id {
		t.Fatalf("expected reused user id %d, got %#v", first.UserInfo.Id, second.UserInfo)
	}

	claims, err := svc.VerifyJWT(ctx, second.Token)
	if err != nil {
		t.Fatalf("VerifyJWT failed: %v", err)
	}
	if claims.ID != first.UserInfo.Id || claims.SupabaseUID != supabaseUID || claims.AuthProvider != "apple" {
		t.Fatalf("unexpected claims: %#v", claims)
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

func setJWTAuthTestConfig(t *testing.T) {
	t.Helper()
	adapter, ok := g.Cfg().GetAdapter().(*gcfg.AdapterFile)
	if !ok {
		t.Fatal("expected file config adapter")
	}
	if err := adapter.Set("jwt.secret", "auth-test-secret"); err != nil {
		t.Fatalf("set jwt.secret: %v", err)
	}
	if err := adapter.Set("jwt.timeout", "3600"); err != nil {
		t.Fatalf("set jwt.timeout: %v", err)
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
