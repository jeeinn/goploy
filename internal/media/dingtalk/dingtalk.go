package dingtalk

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/zhenorzz/goploy/config"
	"github.com/zhenorzz/goploy/internal/media/dingtalk/api"
	"github.com/zhenorzz/goploy/internal/media/dingtalk/api/access_token"
	"github.com/zhenorzz/goploy/internal/media/dingtalk/api/contact"
	"github.com/zhenorzz/goploy/internal/media/dingtalk/api/get_user_by_mobile"
	"github.com/zhenorzz/goploy/internal/media/dingtalk/api/user_access_token"
	"github.com/zhenorzz/goploy/internal/media/dingtalk/cache"
	"io"
	"net/http"
	"net/url"
	"time"
)

type Dingtalk struct {
	Key    string
	Secret string
	Client *http.Client
	Cache  *cache.AccessTokenCache
	Method string
	Api    string
	Query  url.Values
	Body   interface{}
	Resp   interface{}
	Token  string
}

func (d *Dingtalk) Login(authCode string, redirectUri string) (string, error) {
	d.Key = config.Toml.Dingtalk.AppKey
	d.Secret = config.Toml.Dingtalk.AppSecret
	d.Client = &http.Client{}
	d.Cache = cache.GetCache()

	userAccessTokenInfo, err := d.GetUserAccessToken(authCode)
	if err != nil {
		return "", err
	}

	contactUserInfo, err := d.GetContactUser(userAccessTokenInfo.AccessToken)
	if err != nil {
		return "", err
	}

	mobileUserId, err := d.GetUserIdByMobile(contactUserInfo.Mobile)
	if err != nil {
		return "", err
	}

	if mobileUserId.Result.Userid == "" {
		return "", errors.New("please scan the code again after joining the dingtalk company")
	}

	return contactUserInfo.Mobile, nil
}

func (d *Dingtalk) Request() (err error) {
	var (
		req            *http.Request
		resp           *http.Response
		responseData   []byte
		commonResponse api.CommonResponse
	)

	uri, _ := url.Parse(d.Api)
	uri.RawQuery = d.Query.Encode()

	if d.Body != nil {
		b, _ := json.Marshal(d.Body)
		req, _ = http.NewRequest(d.Method, uri.String(), bytes.NewBuffer(b))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req, _ = http.NewRequest(d.Method, uri.String(), nil)
	}

	if d.Token != "" {
		req.Header.Set("x-acs-dingtalk-access-token", d.Token)
	}

	if resp, err = d.Client.Do(req); err != nil {
		return err
	}

	defer resp.Body.Close()

	if responseData, err = io.ReadAll(resp.Body); err != nil {
		return err
	}

	if err = json.Unmarshal(responseData, &commonResponse); err != nil {
		return err
	}

	if commonResponse.Code != "" {
		return errors.New(fmt.Sprintf("api return error, code: %s, message: %s, request_id: %s", commonResponse.Code, commonResponse.Message, commonResponse.RequestId))
	} else if commonResponse.ErrCode != 0 {
		return errors.New(fmt.Sprintf("api return error, code: %v, message: %s, request_id: %s", commonResponse.ErrCode, commonResponse.ErrMsg, commonResponse.OldRequestId))
	}

	if err = json.Unmarshal(responseData, d.Resp); err != nil {
		return err
	}

	return nil
}

func (d *Dingtalk) GetUserAccessToken(authCode string) (resp user_access_token.Response, err error) {
	d.Method = http.MethodPost
	d.Api = user_access_token.Url
	d.Query = nil
	d.Token = ""
	d.Body = user_access_token.Request{
		ClientId:     d.Key,
		ClientSecret: d.Secret,
		Code:         authCode,
		GrandType:    "authorization_code",
	}
	d.Resp = &resp

	return resp, d.Request()
}

func (d *Dingtalk) GetContactUser(userAccessToken string) (resp contact.Response, err error) {
	d.Method = http.MethodGet
	d.Api = contact.Url
	d.Query = nil
	d.Token = userAccessToken
	d.Body = nil
	d.Resp = &resp

	return resp, d.Request()
}

func (d *Dingtalk) GetUserIdByMobile(mobile string) (resp get_user_by_mobile.Response, err error) {
	accessToken, err := d.GetAccessToken()
	if err != nil {
		return resp, err
	}

	d.Method = http.MethodPost
	d.Api = get_user_by_mobile.Url
	d.Query = url.Values{}
	d.Query.Set("access_token", accessToken)
	d.Token = ""
	d.Body = get_user_by_mobile.Request{
		Mobile: mobile,
	}
	d.Resp = &resp

	return resp, d.Request()
}

func (d *Dingtalk) GetAccessToken() (accessToken string, err error) {
	accessToken, ok := d.Cache.Get(d.Key)
	if !ok {
		var resp access_token.Response

		d.Method = http.MethodPost
		d.Api = access_token.Url
		d.Query = nil
		d.Body = access_token.Request{
			AppKey:    d.Key,
			AppSecret: d.Secret,
		}
		d.Resp = &resp

		if err = d.Request(); err != nil {
			return "", err
		}

		d.Cache.Set(d.Key, resp.AccessToken, time.Duration(resp.ExpireIn))

		accessToken = resp.AccessToken
	}

	return accessToken, nil
}
