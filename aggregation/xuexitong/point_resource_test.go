package xuexitong

import (
	"encoding/json"
	"testing"

	xuexitongApi "github.com/yatori-dev/yatori-go-core/api/xuexitong"
	"github.com/yatori-dev/yatori-go-core/models/ctype"
)

// 这两个 iframe 是 2026-11-01 从真实课程卡片里抓下来的原样片段
// （三维计算机辅助设计 / 中国近现代史纲要），用来锁住「资源型条目」的解析行为。

const (
	liveDownloadFileIframe = `<iframe class="ans-module ans-downloadfile-module" frameborder="0" scrolling="no" ` +
		`data="{&quot;objectid&quot;:&quot;fac5fc4c98eabc2c4f586394f1df8c0d&quot;,&quot;name&quot;:&quot;xx-\u9f7f\u8f6e\u4f20\u52a8.zip&quot;,&quot;type&quot;:&quot;.zip&quot;,&quot;size&quot;:1968707,&quot;hsize&quot;:&quot;1.88 MB&quot;,&quot;mid&quot;:&quot;5705913726361667309334867&quot;}" ` +
		`module="downloadfile">` + "\n" + `</iframe>`

	liveBallChartIframe = `<iframe class="ans-module ans-ballchart-module" frameborder="0" scrolling="no" ` +
		`data="{&quot;keyword&quot;:&quot;\u98ce\u4e91\u53d8\u5e7b\u7684\u516b\u5341\u5e74&quot;,&quot;mid&quot;:&quot;3581014956541513140613131&quot;,&quot;retract&quot;:&quot;false&quot;}" ` +
		`module="ballchart">` + "\n" + `</iframe>`
)

// liveAttachmentSetting 是同一张卡片的 AttachmentSetting.attachments 里与
// downloadfile 对应的条目（实测形态：property 内层带 module/mid/objectid）。
const liveAttachmentSetting = `{
  "hiddenConfig": false,
  "attachments": [
    {"begins":0,"ends":0,"jtoken":"b9b5987a0851a6c829ab9017a1e6f721",
     "property":{"size":325575,"hsize":"317.94 KB","module":"insertdoc","name":"4.11.2 GBLib标准件的应用(4).pdf","mid":"1241745209201661518459219","type":".pdf","objectid":"b5af438aef88b62798627d2d218e4314"},
     "mid":"1241745209201661518459219","objectid":"b5af438aef88b62798627d2d218e4314"},
    {"begins":0,"ends":0,"otherInfo":"nodeId_1002521533-cpi_495504274",
     "property":{"size":1968707,"hsize":"1.88 MB","module":"downloadfile","name":"xx-齿轮传动.zip","mid":"5705913726361667309334867","type":".zip","objectid":"fac5fc4c98eabc2c4f586394f1df8c0d"},
     "mid":"5705913726361667309334867","objectid":"fac5fc4c98eabc2c4f586394f1df8c0d"}
  ]
}`

func TestResourceOnlyModuleNames(t *testing.T) {
	cases := []struct {
		name   string
		iframe string
		want   ctype.CardType
	}{
		{"downloadfile", liveDownloadFileIframe, ctype.DownloadFile},
		{"ballchart", liveBallChartIframe, ctype.BallChart},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			points, err := parseIframeData(c.iframe)
			if err != nil {
				t.Fatalf("parseIframeData: %v", err)
			}
			if len(points) != 1 {
				t.Fatalf("got %d points, want 1", len(points))
			}
			got := points[0].Other["module"]
			if got != string(c.want) {
				t.Fatalf("module = %q, ctype 常量 = %q（必须逐字一致，否则会落进 default 分支）", got, c.want)
			}
			if !c.want.ResourceOnly() {
				t.Fatalf("ResourceOnly(%q) = false", got)
			}
		})
	}
}

// downloadfile 的 data 里没有 jobid/_jobid —— 这是它「不是常规任务点」的判据
func TestDownloadFileHasNoJobID(t *testing.T) {
	points, err := parseIframeData(liveDownloadFileIframe)
	if err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"jobid", "_jobid", "jtoken"} {
		if _, ok := points[0].Data[key]; ok {
			t.Fatalf("data 出现 %q；若学习通开始下发 jobid，说明它变成真任务点，需要走提交接口", key)
		}
	}
	for _, key := range []string{"objectid", "mid", "name", "type", "size"} {
		if _, ok := points[0].Data[key]; !ok {
			t.Fatalf("缺少 %q，save-attachment-downloader 需要这些字段", key)
		}
	}
}

func TestEntityFieldsHelperReadsMixedTypes(t *testing.T) {
	data := map[string]interface{}{
		"name":  "xx-齿轮传动.zip",
		"mid":   float64(5705913726361667309334867), // 学习通有时把 mid 下发成数字
		"size":  float64(1968707),
		"type":  ".zip",
		"empty": "",
	}
	if got := stringField(data, "name"); got != "xx-齿轮传动.zip" {
		t.Fatalf("stringField(name) = %q", got)
	}
	if got := stringField(data, "mid"); got == "" {
		t.Fatal("stringField 应当把 float64 形式的 mid 转成字符串")
	}
	if got := stringField(data, "missing"); got != "" {
		t.Fatalf("stringField(missing) = %q, want empty", got)
	}
	if got := intField(data, "size"); got != 1968707 {
		t.Fatalf("intField(size) = %d", got)
	}
	if got := intField(data, "missing"); got != 0 {
		t.Fatalf("intField(missing) = %d", got)
	}
}

// AttachmentsDetection 必须靠 module + objectid 精确命中 downloadfile 那条，
// 不能误取前面的 insertdoc（两者 objectid 不同）。
func TestResourceAttachmentsDetectionPicksDownloadFile(t *testing.T) {
	card := decodeAttachmentSetting(t, liveAttachmentSetting)

	p := &xuexitongApi.PointResourceDto{
		ObjectID: "fac5fc4c98eabc2c4f586394f1df8c0d",
		Type:     ctype.DownloadFile,
	}
	ok, err := p.AttachmentsDetection(card)
	if err != nil {
		t.Fatalf("AttachmentsDetection: %v", err)
	}
	if !ok {
		t.Fatal("未命中 downloadfile 附件")
	}
	if p.Title != "xx-齿轮传动.zip" {
		t.Fatalf("Title = %q", p.Title)
	}
	if p.Mid != "5705913726361667309334867" {
		t.Fatalf("Mid = %q", p.Mid)
	}
	if p.FileType != ".zip" {
		t.Fatalf("FileType = %q；save-attachment-downloader 的 type 参数靠它", p.FileType)
	}
	if p.Size != 1968707 {
		t.Fatalf("Size = %d", p.Size)
	}
}

func TestResourceAttachmentsDetectionMissesUnknownObject(t *testing.T) {
	card := decodeAttachmentSetting(t, liveAttachmentSetting)

	p := &xuexitongApi.PointResourceDto{ObjectID: "does-not-exist", Type: ctype.DownloadFile}
	ok, err := p.AttachmentsDetection(card)
	if err != nil {
		t.Fatalf("AttachmentsDetection 不应因未命中报错: %v", err)
	}
	if ok {
		t.Fatal("未命中的 objectid 不应被当成命中")
	}
	if p.Mid != "" {
		t.Fatalf("未命中时不应回填 Mid，实际 %q", p.Mid)
	}
}

// 所有真实任务点类型都不能被 ResourceOnly 吞掉；未知类型仍走 default 报警
func TestResourceOnlyLeavesTaskPointsAlone(t *testing.T) {
	for _, c := range []ctype.CardType{
		ctype.Video, ctype.Work, ctype.Insertdoc, ctype.Document,
		ctype.InsertBook, ctype.Hyperlink, ctype.Insertlive, ctype.Insertbbs,
		ctype.InsertReadV2, ctype.InsertAudio,
	} {
		if c.ResourceOnly() {
			t.Fatalf("ResourceOnly(%q) = true，真实任务点会被跳过", c)
		}
	}
	if ctype.CardType("something-new").ResourceOnly() {
		t.Fatal("未知类型不应被当成资源型条目静默吞掉")
	}
}

// decodeAttachmentSetting 把 JSON 解成 AttachmentsDetection 期望的
// map[string]interface{} 形态（真实调用链里 parseAttachmentSetting 就是这样返回的）。
func decodeAttachmentSetting(t *testing.T, raw string) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &out); err != nil {
		t.Fatalf("解析 AttachmentSetting 失败: %v", err)
	}
	return out
}
