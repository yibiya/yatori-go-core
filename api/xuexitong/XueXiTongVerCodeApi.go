package xuexitong

import (
	"bytes"
	"crypto/md5"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"image"

	"io/ioutil"

	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
)

var randChar []string = []string{"0", "1", "2", "3", "4", "5", "6", "7", "8", "9", "a", "b", "c", "d", "e", "f", "A", "B", "C", "D", "E", "F"}

// 获取学习通验证码
func (cache *XueXiTUserCache) XueXiTVerificationCodeApi(retry int, lastErr error) (image.Image, error) {
	if retry < 0 {
		return nil, lastErr
	}

	urlStr := "https://mooc1-api.chaoxing.com/processVerifyPng.ac?t=" + fmt.Sprintf("%d", time.Now().UnixMilli())
	method := "GET"

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	//如果开启了IP代理，那么就直接添加代理
	if cache.IpProxySW {
		tr.Proxy = func(req *http.Request) (*url.URL, error) {
			return url.Parse("http://" + cache.ProxyIP) // 设置代理
		}
	}
	client := newRequestClient(tr)
	req, err := http.NewRequest(method, urlStr, nil)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	for _, cookie := range cache.cookies {
		req.AddCookie(cookie)
	}
	req.Header.Add("User-Agent", GetUA("mobile"))
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "mooc1-api.chaoxing.com")
	req.Header.Add("Connection", "keep-alive")

	res, err := client.Do(req)
	if err != nil {
		// 注意：这里原来会回落到 XueXiTChapterVerificationCodeApi，
		// 那是另一套验证码会话（mooc-ans/kaptcha-img），识别出来的码提交到
		// processVerify.ac 必然被拒，等于亲手把重试次数烧光。
		// 同一个端点重试即可。
		return cache.XueXiTVerificationCodeApi(retry-1, err)
	}
	defer res.Body.Close()

	// 直接 decode 图片
	img, _, err := image.Decode(res.Body)
	if err != nil {
		// 不是图片时（例如返回了 202 的挑战页 HTML），重试同一端点
		return cache.XueXiTVerificationCodeApi(retry-1, err)
	}

	return img, nil
}

// 获取学习通验证码
// 获取学习通验证码（直接返回 image.Image）
func (cache *XueXiTUserCache) XueXiTChapterVerificationCodeApi(retry int, lastErr error) (image.Image, error) {
	if retry < 0 {
		return nil, lastErr
	}

	urlStr := "https://mooc1.chaoxing.com/mooc-ans/kaptcha-img/code?" +
		fmt.Sprintf("%d", time.Now().UnixMilli())

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}

	// 代理
	if cache.IpProxySW {
		tr.Proxy = func(req *http.Request) (*url.URL, error) {
			return url.Parse("http://" + cache.ProxyIP)
		}
	}

	client := &http.Client{Transport: tr}

	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, err
	}

	// cookie
	for _, cookie := range cache.cookies {
		req.AddCookie(cookie)
	}

	req.Header.Add("User-Agent", GetUA("mobile"))
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "mooc1-api.chaoxing.com")
	req.Header.Add("Connection", "keep-alive")

	res, err := client.Do(req)
	if err != nil {
		return cache.XueXiTChapterVerificationCodeApi(retry-1, err)
	}
	defer res.Body.Close()

	// 直接 decode 图片
	img, _, err := image.Decode(res.Body)
	if err != nil {
		// 可能是坏图，重试同一个「章节验证码」端点。
		// 不要回落到 XueXiTVerificationCodeApi：那是另一个会话的验证码，
		// 提交到 verifyCode/studychapter 会被拒。
		return cache.XueXiTChapterVerificationCodeApi(retry-1, err)
	}

	return img, nil
}

// 提交学习通验证码
func (cache *XueXiTUserCache) XueXiTPassVerificationCode(code string, retry int, lastErr error) (bool, error) {
	if retry < 0 {
		return false, lastErr
	}
	urlStr := "https://mooc1-api.chaoxing.com/html/processVerify.ac"
	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	_ = writer.WriteField("app", "0")
	_ = writer.WriteField("ucode", code)
	err := writer.Close()
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	//如果开启了IP代理，那么就直接添加代理
	if cache.IpProxySW {
		tr.Proxy = func(req *http.Request) (*url.URL, error) {
			return url.Parse("http://" + cache.ProxyIP) // 设置代理
		}
	}
	client := &http.Client{
		Transport: tr,
		// 禁止自动重定向
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 返回 http.ErrUseLastResponse 表示不要跟随重定向
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, urlStr, payload)

	if err != nil {
		fmt.Println(err)
		return false, err
	}
	for _, cookie := range cache.cookies {
		req.AddCookie(cookie)
	}
	req.Header.Add("User-Agent", GetUA("mobile"))
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "mooc1-api.chaoxing.com")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Content-Type", "multipart/form-data; boundary=--------------------------114712911338779046453834")

	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return cache.XueXiTPassVerificationCode(code, retry-1, fmt.Errorf("failed to create HTTP request: %v", err))
	}
	defer res.Body.Close()

	// 学习通这个接口的返回码不是固定的 302：
	//   成功 -> 200，直接把原本要访问的页面（章节卡片 HTML）返回
	//   失败 -> 202，返回「请输入图片中的验证码」那个 HTML 挑战页
	// 历史代码只认 302，导致即使验证码输对了也一律判失败，
	// 上层 retryCaptcha 重试 5 次后报「验证码被拒绝」（实测 2026-11-01）。
	body, readErr := ioutil.ReadAll(res.Body)
	if readErr != nil {
		body = nil
	}
	if interpretCaptchaSubmit(res.StatusCode, body) {
		return true, nil
	}
	return false, nil
}

// interpretCaptchaSubmit 判定 /html/processVerify.ac 的响应是否代表「验证码已通过」。
//
// 实测（2026-11-01）：
//
//	通过 -> 200，响应体是原本要访问的页面（章节卡片 HTML）
//	未过 -> 202，响应体是「请输入图片中的验证码」挑战页
//
// 历史上还出现过 302，一并兼容。
//
// 关键点：只靠状态码不够 —— 200 也可能把挑战页回给你，所以还要看响应体。
func interpretCaptchaSubmit(statusCode int, body []byte) bool {
	switch statusCode {
	case http.StatusFound, http.StatusOK:
		return !looksLikeCaptchaChallenge(body)
	}
	return false
}

// looksLikeCaptchaChallenge 判断响应体是不是「还要你再输一次验证码」的挑战页。
// 成功时这个接口会把目标页面整段返回，里面不会有这些标记。
func looksLikeCaptchaChallenge(body []byte) bool {
	s := string(body)
	return strings.Contains(s, "请输入验证码") ||
		strings.Contains(s, "请输入图片中的验证码") ||
		strings.Contains(s, "processVerify.ac")
}

// 提交学习通验证码(章节内）
func (cache *XueXiTUserCache) XueXiTPassCahpterVerificationCode(code string, retry int, lastErr error) (bool, error) {
	if retry < 0 {
		return false, lastErr
	}
	//urlStr := "https://mooc1.chaoxing.com/mooc-ans/kaptcha-img/ajaxValidate2"
	urlStr := "https://mooc1.chaoxing.com/mooc-ans/verifyCode/studychapter"
	method := "POST"

	payload := &bytes.Buffer{}
	writer := multipart.NewWriter(payload)
	//_ = writer.WriteField("app", "0")
	_ = writer.WriteField("code", code)
	err := writer.Close()
	if err != nil {
		fmt.Println(err)
		return false, err
	}

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	//如果开启了IP代理，那么就直接添加代理
	if cache.IpProxySW {
		tr.Proxy = func(req *http.Request) (*url.URL, error) {
			return url.Parse("http://" + cache.ProxyIP) // 设置代理
		}
	}
	client := &http.Client{
		Transport: tr,
		// 禁止自动重定向
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// 返回 http.ErrUseLastResponse 表示不要跟随重定向
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequest(method, urlStr, payload)

	if err != nil {
		fmt.Println(err)
		return false, err
	}
	for _, cookie := range cache.cookies {
		req.AddCookie(cookie)
	}
	req.Header.Add("User-Agent", GetUA("mobile"))
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "mooc1-api.chaoxing.com")
	req.Header.Add("Connection", "keep-alive")
	req.Header.Add("Content-Type", "multipart/form-data; boundary=--------------------------114712911338779046453834")

	req.Header.Set("Content-Type", writer.FormDataContentType())
	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return cache.XueXiTPassCahpterVerificationCode(code, retry-1, err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	// 这里原来写成 `if body != {"status":true} { return true }`，判断完全反了：
	// 验证码输错（返回 {"status":false}）会被当成成功，输对反而继续重试。
	// 结果就是永远刷不过，上层重试到上限后报「验证码被拒绝」（2026-11-01 实测）。
	return strings.TrimSpace(string(body)) == `{"status":true}`, nil
}

// XueXiTSliderVerificationCodeApi 获取学习通滑块验证码相关信息，返回信息cx_captcha_function({"t":1764584640340,"captchaId":"Ew0z9skxsLzVKQjmeObQiRVLxkxbPkRF"})
func (cache *XueXiTUserCache) XueXiTSliderVerificationCodeApi(captchaId string, retry int, lastErr error) (string, error) {

	urlStr := "https://captcha.chaoxing.com/captcha/get/conf?callback=cx_captcha_function&captchaId=" + captchaId + "&_=" + fmt.Sprintf("%d", time.Now().UnixMilli())
	method := "GET"

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}

	//如果开启了IP代理，那么就直接添加代理
	if cache.IpProxySW {
		tr.Proxy = func(req *http.Request) (*url.URL, error) {
			return url.Parse("http://" + cache.ProxyIP) // 设置代理
		}
	}
	client := newRequestClient(tr)
	req, err := http.NewRequest(method, urlStr, nil)

	if err != nil {
		fmt.Println(err)
		return "", err
	}
	req.Header.Add("User-Agent", GetUA("mobile"))
	req.Header.Add("Accept-Language", "zh-CN,en-US;q=0.9")
	req.Header.Add("X-Requested-With", "com.chaoxing.mobile")
	for _, cookie := range cache.cookies {
		req.AddCookie(cookie)
	}
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "captcha.chaoxing.com")
	req.Header.Add("Connection", "keep-alive")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return cache.XueXiTSliderVerificationCodeApi(captchaId, retry-1, err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	//fmt.Println(string(body))
	return string(body), nil
}

// 拉取验证码图片等信息
func (cache *XueXiTUserCache) XueXiTSliderVerificationImgApi(captchaId, serverTime, referer string, retry int, lastErr error) (string, error) {
	// 计算 MD5
	captchaKeyHash := md5.Sum([]byte(serverTime + uuid.New().String()))
	captchaKey := hex.EncodeToString(captchaKeyHash[:])

	//ivHash := md5.Sum([]byte(fmt.Sprintf("%s%s%d%s", captchaId, "slide", time.Now().UnixMilli(), uuid.New().String())))
	//iv := hex.EncodeToString(ivHash[:])

	// 计算token
	sum := md5.Sum([]byte(fmt.Sprintf("%s%s%s%s", serverTime, captchaId, "slide", captchaKey)))
	md5hex := hex.EncodeToString(sum[:])
	serverTimeInt, _ := strconv.ParseInt(serverTime, 10, 64)
	token := fmt.Sprintf("%s:%d", md5hex, serverTimeInt+300000)
	version := "1.1.20"
	urlStr := "https://captcha.chaoxing.com/captcha/get/verification/image?callback=cx_captcha_function&captchaId=" + captchaId + "&type=slide&version=" + version + "&captchaKey=" + captchaKey + "&token=" + token + "&referer=" + url.QueryEscape(referer)
	method := "GET"

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}

	//如果开启了IP代理，那么就直接添加代理
	if cache.IpProxySW {
		tr.Proxy = func(req *http.Request) (*url.URL, error) {
			return url.Parse("http://" + cache.ProxyIP) // 设置代理
		}
	}
	client := &http.Client{
		Transport: tr,
	}
	req, err := http.NewRequest(method, urlStr, nil)

	if err != nil {
		fmt.Println(err)
		return "", err
	}
	req.Header.Add("User-Agent", GetUA("mobile"))
	req.Header.Add("Accept-Language", "zh-CN,en-US;q=0.9")
	req.Header.Add("X-Requested-With", "com.chaoxing.mobile")
	for _, cookie := range cache.cookies {
		req.AddCookie(cookie)
	}
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "captcha.chaoxing.com")
	req.Header.Add("Connection", "keep-alive")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return cache.XueXiTSliderVerificationImgApi(captchaId, serverTime, referer, retry-1, err)
	}
	//fmt.Println(string(body))
	return string(body), nil
}

// 请求并获取图片
func (cache *XueXiTUserCache) PullSliderImgApi(imgUrl string, retry int, lastErr error) (image.Image, error) {
	if retry < 0 {
		return nil, lastErr
	}
	method := "GET"

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	client := &http.Client{
		Transport: tr,
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return fmt.Errorf("滑块图片请求不允许重定向")
		},
	}
	req, err := http.NewRequest(method, imgUrl, nil)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	req.Header.Add("User-Agent", GetUA("mobile"))
	req.Header.Add("Accept-Language", "zh-CN,en-US;q=0.9")
	req.Header.Add("X-Requested-With", "com.chaoxing.mobile")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "captcha.chaoxing.com")
	req.Header.Add("Connection", "keep-alive")

	resp, err := client.Do(req)
	if err != nil {
		return cache.PullSliderImgApi(imgUrl, retry-1, fmt.Errorf("请求失败: %v", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("HTTP 状态码异常: %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("图片解码失败: %v", err)
	}

	return img, nil
}

// 过滑块接口
func (cache *XueXiTUserCache) PassSliderApi(captchaId, token, xPoint, runEnv string, retry int, lastErr error) (string, error) {
	if retry < 0 {
		return "", lastErr
	}
	urlStr := "https://captcha.chaoxing.com/captcha/check/verification/result?callback=cx_captcha_function&captchaId=" + captchaId + "&type=slide&token=" + token + "&textClickArr=" + url.QueryEscape(`[{"x":`+xPoint+`}]`) + "&coordinate=" + url.QueryEscape(`[]`) + "&runEnv=10&version=1.1.20&t=a&iv=cdd9bfb9e7805d0d2d5f1ad4498f70e1&_=1764584636040"
	method := "GET"

	tr := &http.Transport{
		TLSClientConfig: &tls.Config{},
	}
	client := newRequestClient(tr)
	req, err := http.NewRequest(method, urlStr, nil)

	if err != nil {
		fmt.Println(err)
		return "", err
	}
	req.Header.Add("User-Agent", "Mozilla/5.0 (Linux; Android 8.1.0; MI 5X Build/OPM1.171019.019; wv) AppleWebKit/537.36 (KHTML, like Gecko) Version/4.0 Chrome/71.0.3578.99 Mobile Safari/537.36 (schild:ce5175d20950c8ee955fb03246f762da) (device:MI 5X) Language/zh_CN com.chaoxing.mobile/ChaoXingStudy_3_6.7.2_android_phone_10936_311 (@Kalimdor)_76c82452584d47e39ab79aa54ea86554")
	req.Header.Add("Accept-Language", "zh-CN,en-US;q=0.9")
	req.Header.Add("X-Requested-With", "com.chaoxing.mobile")
	req.Header.Add("Accept", "*/*")
	req.Header.Add("Host", "captcha.chaoxing.com")
	req.Header.Add("Connection", "keep-alive")

	res, err := client.Do(req)
	if err != nil {
		fmt.Println(err)
		return cache.PassSliderApi(captchaId, token, xPoint, runEnv, retry-1, err)
	}
	defer res.Body.Close()

	body, err := ioutil.ReadAll(res.Body)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	if err := validateResponse("提交滑块验证码", res, body); err != nil {
		return "", err
	}
	//fmt.Println(string(body))
	return string(body), nil
}
