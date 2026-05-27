package auth

import (
	"context"
	v1 "dm-server/api/user/v1"
	"dm-server/internal/consts"
	"dm-server/internal/dao"
	"dm-server/internal/model"
	"dm-server/internal/model/entity"
	"dm-server/internal/service"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/MicahParks/keyfunc/v2"
	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/golang-jwt/jwt/v5"
)

type sAuth struct{}

var (
	jwksInstance *keyfunc.JWKS
	jwksOnce     sync.Once
)

func New() *sAuth {
	return &sAuth{}
}

func init() {
	service.RegisterAuth(New())
	// Eagerly warm up JWKS in background
	go func() {
		ctx := context.Background()
		if err := New().ensureJWKS(ctx); err != nil {
			glog.Warningf(ctx, "JWKS preload failed, will retry on first request: %v", err)
			// Reset so it retries on next request
			jwksOnce = sync.Once{}
		}
	}()
}

// ─────────────────────────────────────────────────────────
// JWKS — Supabase public key fetching
// ─────────────────────────────────────────────────────────
func (s *sAuth) ensureJWKS(ctx context.Context) error {
	var initErr error
	jwksOnce.Do(func() {
		projectURL := g.Cfg().MustGet(ctx, "supabase.url").String()
		jwksURL := projectURL + "/auth/v1/.well-known/jwks.json"
		glog.Infof(ctx, "Loading Supabase JWKS from: %s", jwksURL)
		jwksInstance, initErr = keyfunc.Get(jwksURL, keyfunc.Options{
			RefreshInterval:   time.Hour,
			RefreshRateLimit:  time.Minute * 5,
			RefreshUnknownKID: true,
		})
	})
	return initErr
}

func (s *sAuth) verifySupabaseJWT(ctx context.Context, tokenString string) (jwt.MapClaims, error) {
	if err := s.ensureJWKS(ctx); err != nil {
		return nil, gerror.Wrap(err, "Failed to load Supabase signing keys")
	}
	token, err := jwt.Parse(tokenString, jwksInstance.Keyfunc)
	if err != nil || !token.Valid {
		return nil, gerror.New("Invalid or expired Supabase token: " + err.Error())
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, gerror.New("Cannot parse token claims")
	}
	return claims, nil
}

// ─────────────────────────────────────────────────────────
// WeChat Login
// ─────────────────────────────────────────────────────────
func (s *sAuth) WechatLogin(ctx context.Context, req *v1.WechatAuthReq) (*v1.WechatAuthRes, error) {
	// get openid from code
	glog.Infof(ctx, "Get openid from code: %s", req.Code)
	openid, err := s.getOpenidFromWechat(ctx, req.Code)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get openid")
	}

	// find or create user (only openid)
	user, err := s.findOrCreateWechatUser(ctx, openid)
	if err != nil {
		return nil, gerror.Wrap(err, "Find or Create User failed")
	}

	// generate JWT token
	token, err := s.generateServerJWT(ctx, user.Id, "", openid, "wechat")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to generate JWT token")
	}
	glog.Infof(ctx, "Generate JWT token: %s", token)

	return &v1.WechatAuthRes{
		OpenId:   openid,
		Token:    token,
		UserInfo: &v1.UserInfo{Id: user.Id, OpenId: user.Openid},
	}, nil
}

// ─────────────────────────────────────────────────────────
// Email Login via Supabase token
// ─────────────────────────────────────────────────────────
func (s *sAuth) EmailLogin(ctx context.Context, req *v1.EmailAuthReq) (*v1.EmailAuthRes, error) {
	// 1. Verify the Supabase access_token using JWKS
	supabaseClaims, err := s.verifySupabaseJWT(ctx, req.AccessToken)
	if err != nil {
		return nil, gerror.Wrap(err, "Invalid Supabase token")
	}

	supabaseUID, _ := supabaseClaims["sub"].(string)
	if supabaseUID == "" {
		return nil, gerror.New("Token missing 'sub' claim")
	}
	email, _ := supabaseClaims["email"].(string)

	// 2. Find or create local MySQL user
	user, err := s.findOrCreateEmailUser(ctx, supabaseUID, email)
	if err != nil {
		return nil, gerror.Wrap(err, "Find or Create User failed")
	}

	// 3. Issue your own server JWT
	token, err := s.generateServerJWT(ctx, user.Id, supabaseUID, "", "email")
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to generate server JWT")
	}

	return &v1.EmailAuthRes{
		Token: token,
		UserInfo: &v1.UserInfo{
			Id:    user.Id,
			Email: email,
		},
	}, nil
}

// GetUserInfo Get user information
// Used after login to get detailed user info
func (s *sAuth) GetUserInfo(ctx context.Context, req *v1.GetUserInfoReq) (*v1.GetUserInfoRes, error) {
	userId := ctx.Value(consts.CtxUserId)
	if userId == nil {
		return nil, gerror.New("User not logged in")
	}
	userIdUint64, ok := userId.(uint64)
	if !ok {
		return nil, gerror.New("Invalid user ID type")
	}

	// find user by id
	var user *entity.Users
	err := dao.Users.Ctx(ctx).Where("id", userIdUint64).Where("deleted_at IS NULL").Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "Database error")
	}
	if user == nil {
		return nil, gerror.New("User does not exist")
	}
	return &v1.GetUserInfoRes{
		UserInfo: &v1.UserInfo{
			Id:       user.Id,
			OpenId:   user.Openid,
			Email:    user.Email,
			Nickname: user.Nickname,
			Avatar:   user.AvatarUrl,
		},
	}, nil
}

// UpdateUserInfo Update user information
func (s *sAuth) UpdateUserInfo(ctx context.Context, req *v1.UpdateUserInfoReq) (*v1.UpdateUserInfoRes, error) {
	userId := ctx.Value(consts.CtxUserId)
	if userId == nil {
		return nil, gerror.New("User not logged in")
	}
	userIdUint64, ok := userId.(uint64)
	if !ok {
		return nil, gerror.New("Invalid user ID type")
	}
	_, err := dao.Users.Ctx(ctx).Data(g.Map{
		"nickname":   req.Nickname,
		"avatar_url": req.Avatar,
		"updated_at": gtime.Now().FormatTo("2006-01-02 15:04:05"),
	}).Where("id", userIdUint64).Update()
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to update user info")
	}
	var user *entity.Users
	err = dao.Users.Ctx(ctx).Where("id", userIdUint64).Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "Database error")
	}
	if user == nil {
		return nil, gerror.New("User does not exist")
	}
	return &v1.UpdateUserInfoRes{
		UserInfo: &v1.UserInfo{
			Id:       user.Id,
			OpenId:   user.Openid,
			Email:    user.Email,
			Nickname: user.Nickname,
			Avatar:   user.AvatarUrl,
		},
	}, nil
}

func (s *sAuth) GetUserSettings(ctx context.Context, req *v1.GetUserSettingsReq) (*v1.GetUserSettingsRes, error) {
	userID, err := getAuthContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := loadUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	res := v1.GetUserSettingsRes(*settings)
	return &res, nil
}

func (s *sAuth) UpdateUserSettings(ctx context.Context, req *v1.UpdateUserSettingsReq) (*v1.UpdateUserSettingsRes, error) {
	userID, err := getAuthContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	settings, err := loadUserSettings(ctx, userID)
	if err != nil {
		return nil, err
	}
	if req.Language != "" {
		settings.Language = req.Language
	}
	if req.PrivacyMode != "" {
		settings.PrivacyMode = req.PrivacyMode
	}
	if req.DreamReminderEnabled != nil {
		settings.DreamReminderEnabled = req.DreamReminderEnabled
	}
	if req.DreamReminderTime != "" {
		settings.DreamReminderTime = req.DreamReminderTime
	}
	if req.StorageMode != "" {
		settings.StorageMode = req.StorageMode
	}
	if err := saveUserSettings(ctx, userID, settings); err != nil {
		return nil, err
	}
	res := v1.UpdateUserSettingsRes(*settings)
	return &res, nil
}

func (s *sAuth) GetPsycheProfile(ctx context.Context, req *v1.GetPsycheProfileReq) (*v1.GetPsycheProfileRes, error) {
	userID, err := getAuthContextUserID(ctx)
	if err != nil {
		return nil, err
	}
	total, err := g.DB().Model("dreams").
		Where("user_id = ? AND deleted_at IS NULL", userID).
		Count()
	if err != nil {
		return nil, gerror.Wrap(err, "failed to count dreams")
	}
	score := float64(35)
	level := "low"
	description := "Keep recording dreams to build a richer psyche profile."
	if total >= 3 {
		score = 62
		level = "moderate"
		description = "Your recent dreams show emerging symbolic continuity and self-reflection."
	}
	if total >= 10 {
		score = 84
		level = "high"
		description = "Your dream journal shows stable integration across recurring themes."
	}
	profile := v1.GetPsycheProfileRes{
		IntegrationScore:       score,
		IntegrationLevel:       level,
		IntegrationDescription: description,
		Archetypes: []v1.ArchetypeProfileItem{
			{Type: "self", Name: "Self", Score: score, Description: "Wholeness and inner alignment"},
			{Type: "persona", Name: "Persona", Score: 48, Description: "Social identity and outward roles"},
			{Type: "shadow", Name: "Shadow", Score: 42, Description: "Unintegrated emotions and avoided themes"},
			{Type: "anima", Name: "Anima", Score: 38, Description: "Inner feeling and symbolic imagination"},
			{Type: "sage", Name: "Sage", Score: 44, Description: "Guidance, insight, and meaning-making"},
		},
		DominantArchetype: "self",
		UpdatedAt:         gtime.Now().Format("Y-m-d H:i:s"),
	}
	return &profile, nil
}

// ─────────────────────────────────────────────────────────
// Internal helpers
// ─────────────────────────────────────────────────────────

func getAuthContextUserID(ctx context.Context) (uint64, error) {
	userId := ctx.Value(consts.CtxUserId)
	if userId == nil {
		return 0, gerror.New("User not logged in")
	}
	userIdUint64, ok := userId.(uint64)
	if !ok || userIdUint64 == 0 {
		return 0, gerror.New("Invalid user ID type")
	}
	return userIdUint64, nil
}

func defaultUserSettings() *v1.UserSettings {
	enabled := false
	return &v1.UserSettings{
		Language:             "zh-CN",
		PrivacyMode:          "private",
		DreamReminderEnabled: &enabled,
		DreamReminderTime:    "",
		StorageMode:          "local_cache",
	}
}

func loadUserSettings(ctx context.Context, userID uint64) (*v1.UserSettings, error) {
	settings := defaultUserSettings()
	var row struct {
		Language             string `orm:"language"`
		PrivacyMode          string `orm:"privacy_mode"`
		DreamReminderEnabled *bool  `orm:"dream_reminder_enabled"`
		DreamReminderTime    string `orm:"dream_reminder_time"`
		StorageMode          string `orm:"storage_mode"`
	}
	err := g.DB().Model("user_settings").Where("user_id", userID).Scan(&row)
	if err != nil {
		return nil, gerror.Wrap(err, "failed to load user settings")
	}
	if row.Language == "" && row.PrivacyMode == "" && row.StorageMode == "" {
		return settings, nil
	}
	if row.Language != "" {
		settings.Language = row.Language
	}
	if row.PrivacyMode != "" {
		settings.PrivacyMode = row.PrivacyMode
	}
	if row.DreamReminderEnabled != nil {
		settings.DreamReminderEnabled = row.DreamReminderEnabled
	}
	settings.DreamReminderTime = row.DreamReminderTime
	if row.StorageMode != "" {
		settings.StorageMode = row.StorageMode
	}
	return settings, nil
}

func saveUserSettings(ctx context.Context, userID uint64, settings *v1.UserSettings) error {
	enabled := false
	if settings.DreamReminderEnabled != nil {
		enabled = *settings.DreamReminderEnabled
	}
	data := g.Map{
		"user_id":                userID,
		"language":               settings.Language,
		"privacy_mode":           settings.PrivacyMode,
		"dream_reminder_enabled": enabled,
		"dream_reminder_time":    settings.DreamReminderTime,
		"storage_mode":           settings.StorageMode,
		"updated_at":             gtime.Now().Format("Y-m-d H:i:s"),
	}
	count, err := g.DB().Model("user_settings").Where("user_id", userID).Count()
	if err != nil {
		return gerror.Wrap(err, "failed to check user settings")
	}
	if count == 0 {
		_, err = g.DB().Model("user_settings").Data(data).Insert()
	} else {
		delete(data, "user_id")
		_, err = g.DB().Model("user_settings").Where("user_id", userID).Data(data).Update()
	}
	if err != nil {
		return gerror.Wrap(err, "failed to save user settings")
	}
	return nil
}

// findOrCreateUser Find or create user
func (s *sAuth) findOrCreateWechatUser(ctx context.Context, openid string) (*entity.Users, error) {
	var user *entity.Users
	err := dao.Users.Ctx(ctx).Where("openid", openid).Where("deleted_at IS NULL").Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "Database error")
	}
	if user != nil {
		return user, nil
	}
	user = &entity.Users{
		Openid:       openid,
		AuthProvider: "wechat",
	}
	rs, err := dao.Users.Ctx(ctx).Insert(user)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to create user")
	}
	lastId, _ := rs.LastInsertId()
	user.Id = uint64(lastId)
	return user, nil
}

func (s *sAuth) findOrCreateEmailUser(ctx context.Context, supabaseUID, email string) (*entity.Users, error) {
	var user *entity.Users
	err := dao.Users.Ctx(ctx).
		Where("supabase_uid", supabaseUID).
		Where("deleted_at IS NULL").
		Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "Database error")
	}
	if user != nil {
		// Sync email if it changed on Supabase side
		if email != "" && user.Email != email {
			_, _ = dao.Users.Ctx(ctx).
				Data(g.Map{"email": email, "updated_at": gtime.Now().FormatTo("2006-01-02 15:04:05")}).
				Where("id", user.Id).Update()
			user.Email = email
		}
		return user, nil
	}
	// Create new user
	user = &entity.Users{
		SupabaseUid:  supabaseUID,
		Email:        email,
		AuthProvider: "email",
	}
	rs, err := dao.Users.Ctx(ctx).Insert(user)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to create user")
	}
	lastId, _ := rs.LastInsertId()
	user.Id = uint64(lastId)
	return user, nil
}

// getOpenidFromWechat Get openid from WeChat
func (s *sAuth) getOpenidFromWechat(ctx context.Context, code string) (string, error) {
	// WeChat mini program config (should be read from config file)
	appID := g.Cfg().MustGet(ctx, "wechat.appId").String()
	appSecret := g.Cfg().MustGet(ctx, "wechat.appSecret").String()

	url := fmt.Sprintf("https://api.weixin.qq.com/sns/jscode2session?appid=%s&secret=%s&js_code=%s&grant_type=authorization_code",
		appID, appSecret, code)

	client := g.Client()
	response, err := client.Get(ctx, url)
	if err != nil {
		return "", gerror.Wrap(err, "Failed to request WeChat API")
	}
	defer response.Close()

	var result struct {
		Openid     string `json:"openid"`
		SessionKey string `json:"session_key"`
		Unionid    string `json:"unionid"`
		Errcode    int    `json:"errcode"`
		Errmsg     string `json:"errmsg"`
	}

	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return "", gerror.Wrap(err, "Failed to parse WeChat response")
	}

	if result.Errcode != 0 {
		return "", gerror.Newf("WeChat API error: %d %s", result.Errcode, result.Errmsg)
	}

	if result.Openid == "" {
		return "", gerror.New("Failed to get openid")
	}

	return result.Openid, nil
}

// generateServerJWT issue server JWT after auth
func (s *sAuth) generateServerJWT(ctx context.Context, userID uint64, supabaseUID, openid, provider string) (string, error) {
	secret := g.Cfg().MustGet(ctx, "jwt.secret").String()
	timeout := g.Cfg().MustGet(ctx, "jwt.timeout").Int()
	exp := time.Now().Add(time.Duration(timeout) * time.Second).Unix()
	claims := jwt.MapClaims{
		"userId":      userID,
		"supabaseUid": supabaseUID,
		"openid":      openid,
		"provider":    provider,
		"exp":         exp,
		"iat":         time.Now().Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

// VerifyJWT Verify JWT token
func (s *sAuth) VerifyJWT(ctx context.Context, tokenString string) (*model.AuthClaims, error) {
	secret := g.Cfg().MustGet(ctx, "jwt.secret").String()
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
		return []byte(secret), nil
	})
	if err != nil || !token.Valid {
		return nil, gerror.New("Invalid token")
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, gerror.New("Invalid token format")
	}
	return &model.AuthClaims{
		ID:           gconv.Uint64(claims["userId"]),
		SupabaseUID:  gconv.String(claims["supabaseUid"]),
		OpenID:       gconv.String(claims["openid"]),
		AuthProvider: gconv.String(claims["provider"]),
	}, nil
}
