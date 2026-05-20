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
	"time"

	"github.com/gogf/gf/v2/errors/gerror"
	"github.com/gogf/gf/v2/frame/g"
	"github.com/gogf/gf/v2/os/glog"
	"github.com/gogf/gf/v2/os/gtime"
	"github.com/gogf/gf/v2/util/gconv"
	"github.com/golang-jwt/jwt/v4"
)

type sAuth struct{}

func New() *sAuth {
	return &sAuth{}
}

func init() {
	service.RegisterAuth(New())
}

// WechatLogin WeChat login
func (s *sAuth) WechatLogin(ctx context.Context, req *v1.WechatAuthReq) (*v1.WechatAuthRes, error) {
	// get openid from code
	glog.Infof(ctx, "Get openid from code: %s", req.Code)
	openid, err := s.getOpenidFromWechat(ctx, req.Code)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get openid")
	}

	// find or create user (only openid)
	user, err := s.findOrCreateUser(ctx, openid)
	if err != nil {
		return nil, gerror.Wrap(err, "Find or Create User failed")
	}

	// generate JWT token
	token, err := s.generateJWT(ctx, user.Id, openid)
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

// GetUserInfo Get user information
// Used after login to get detailed user info
func (s *sAuth) GetUserInfo(ctx context.Context, req *v1.GetUserInfoReq) (*v1.GetUserInfoRes, error) {
	// get user id from context
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
			Nickname: user.Nickname,
			Avatar:   user.AvatarUrl,
		},
	}, nil
}

// findOrCreateUser Find or create user
func (s *sAuth) findOrCreateUser(ctx context.Context, openid string) (*entity.Users, error) {
	// find user by openid
	var user *entity.Users
	err := dao.Users.Ctx(ctx).Where("openid", openid).Where("deleted_at IS NULL").Scan(&user)
	if err != nil {
		return nil, gerror.Wrap(err, "Database error")
	}
	// return user if exists
	if user != nil {
		return user, nil
	}

	// create user
	user = &entity.Users{
		Openid: openid,
	}
	rs, err := dao.Users.Ctx(ctx).Insert(user)
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to create user")
	}
	lastId, err := rs.LastInsertId()
	if err != nil {
		return nil, gerror.Wrap(err, "Failed to get last insert ID")
	}
	user.Id = uint64(lastId)
	user.CreatedAt = gtime.Now().FormatTo("2006-01-02 15:04:05")
	user.UpdatedAt = gtime.Now().FormatTo("2006-01-02 15:04:05")
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

// generateJWT Generate JWT token
func (s *sAuth) generateJWT(ctx context.Context, userID uint64, openid string) (string, error) {
	secret := g.Cfg().MustGet(ctx, "jwt.secret").String()
	timeout := g.Cfg().MustGet(ctx, "jwt.timeout").Int()

	exp := time.Now().Add(time.Duration(timeout) * time.Second).Unix()

	claims := jwt.MapClaims{
		"userId": userID,
		"openid": openid,
		"exp":    exp,
		"iat":    time.Now().Unix(),
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

	if err != nil {
		return nil, gerror.Wrap(err, "Token parse failed")
	}

	if !token.Valid {
		return nil, gerror.New("Invalid token")
	}

	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return nil, gerror.New("Invalid token format")
	}

	return &model.AuthClaims{
		ID:     gconv.Uint64(claims["userId"]),
		OpenID: gconv.String(claims["openid"]),
	}, nil
}
