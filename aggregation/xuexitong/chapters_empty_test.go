package xuexitong

import (
	"errors"
	"testing"
)

// 课程本身没有章节内容（老师建了课但没放东西）是正常状态，不是故障。
// 实测 2026-11-01：53 门课里 24 门如此，课程列表接口的 jobcount 同时为 0，
// 两个独立接口互相印证。调用方靠 errors.As 区分它和真正的拉取失败，
// 以免每轮刷课刷出十几条红字把真故障淹没。

func TestChaptersEmptyIsDistinguishable(t *testing.T) {
	err := error(ChaptersEmpty{Account: "13320169850", ChatID: "325150302076929"})

	var target ChaptersEmpty
	if !errors.As(err, &target) {
		t.Fatal("errors.As 应当能识别 ChaptersEmpty")
	}
	if target.Account != "13320169850" || target.ChatID != "325150302076929" {
		t.Fatalf("字段未保留: %+v", target)
	}
}

// 包装后仍然要能识别（调用方可能用 %w 包一层）
func TestChaptersEmptySurvivesWrapping(t *testing.T) {
	inner := ChaptersEmpty{Account: "a", ChatID: "b"}
	wrapped := errors.Join(errors.New("其它上下文"), inner)

	var target ChaptersEmpty
	if !errors.As(wrapped, &target) {
		t.Fatal("包装后 errors.As 仍应识别 ChaptersEmpty")
	}
}

// 真正的故障不能被误判成「章节为空」
func TestOtherErrorsAreNotChaptersEmpty(t *testing.T) {
	for _, err := range []error{
		errors.New("拉取章节失败: status code: 500"),
		errors.New("解析课程章节失败"),
		APIError{Message: "章节卡片缺少 AttachmentSetting"},
		ChapterNotOpened{},
	} {
		var target ChaptersEmpty
		if errors.As(err, &target) {
			t.Fatalf("%v 不应被判定为 ChaptersEmpty", err)
		}
	}
}
