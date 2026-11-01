package xuexitong

import (
	"net/http"
	"testing"
)

// 2026-11-01 实测学习通 /html/processVerify.ac 的返回：
//   验证码正确 -> 200，响应体是原本要访问的页面（章节卡片 HTML）
//   验证码错误 -> 202，响应体是「请输入图片中的验证码」挑战页
//
// 历史代码只认 302，于是无论输对输错都判失败，上层重试 5 次后报
// 「图片验证码重试 5 次后仍未通过: 验证码被拒绝」。

const captchaChallengePage = `<!doctype html><html><head><title>提示页面</title></head><body>
<div class="yzmTips"><p>【9010】操作异常，请输入图片中的验证码</p></div>
<form action="/html/processVerify.ac" onsubmit="return /[0-9a-zA-Z]{4}/g.test(1)">
<input type="hidden" name="app" value="0"/>
<input type="text" id="ucode" name="ucode" maxlength="4">
<img id="ccc" src='/processVerifyPng.ac?t=1'>
</form></body></html>`

const cardPageOnSuccess = `<html xmlns="http://www.w3.org/1999/xhtml"><script>
    _HOST_ = "//mooc1-api.chaoxing.com";
</script><head></head><body><div class="ans-module">ok</div></body></html>`

func TestInterpretCaptchaSubmit(t *testing.T) {
	cases := []struct {
		name   string
		status int
		body   string
		want   bool
	}{
		{"200 + 正常页面 -> 通过", http.StatusOK, cardPageOnSuccess, true},
		{"202 + 挑战页 -> 未通过", http.StatusAccepted, captchaChallengePage, false},
		{"200 + 挑战页 -> 未通过（不能只看状态码）", http.StatusOK, captchaChallengePage, false},
		{"302 -> 兼容旧行为，通过", http.StatusFound, "", true},
		{"500 -> 未通过", http.StatusInternalServerError, "", false},
		{"200 空体 -> 通过", http.StatusOK, "", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := interpretCaptchaSubmit(c.status, []byte(c.body)); got != c.want {
				t.Fatalf("status=%d 判定 = %v, want %v", c.status, got, c.want)
			}
		})
	}
}

func TestLooksLikeCaptchaChallenge(t *testing.T) {
	cases := []struct {
		name string
		body string
		want bool
	}{
		{"请输入验证码", "请先请输入验证码", true},
		{"请输入图片中的验证码", "【9010】操作异常，请输入图片中的验证码", true},
		{"表单 action", `<form action="/html/processVerify.ac">`, true},
		{"正常卡片页", cardPageOnSuccess, false},
		{"空响应", "", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := looksLikeCaptchaChallenge([]byte(c.body)); got != c.want {
				t.Fatalf("looksLikeCaptchaChallenge(%q) = %v, want %v", c.name, got, c.want)
			}
		})
	}
}
