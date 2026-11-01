package point

import (
	"strings"

	"github.com/thedevsaddam/gojsonq"
	xuexitong2 "github.com/yatori-dev/yatori-go-core/aggregation/xuexitong"
	"github.com/yatori-dev/yatori-go-core/api/xuexitong"
	"github.com/yatori-dev/yatori-go-core/models/ctype"
	log2 "github.com/yatori-dev/yatori-go-core/utils/log"
)

// ExecuteResource 处理资源型条目（资料下载 / 知识点球图）。
//
// 这两类在章节卡片里没有 jobid，学习通也没有 /ananas/job/xxx 提交接口，
// 所以不能用文档/视频那套「提交完成」的写法。实测（2026-11-01）可用的只有：
//
//	GET https://mooc1.chaoxing.com/mooc-ans/knowledge/save-attachment-downloader
//	    ?mid=&type=&clazzId=&courseId=&nodeId=&cpi=
//
// 参数缺一个就回 {"result":false,"errorMsg":"参数错误"}，全给对回 {"status":true}。
// 前端点「下载」时调的就是它（ananas/videojs-ext/recordDownloadInfo.js），
// 所以这是唯一被学习通认可的「访问」上报方式。
//
// 注意两点，避免调用方误判：
//  1. 返回 {"status":true} 只代表「已记录访问」，不代表任务点立即变绿；
//     实测课程 job 计数与章节 finishcount 不一定马上变化，服务端另有判定。
//  2. ballchart（知识点球图）没有对应接口，直接跳过。
func ExecuteResource(cache *xuexitong.XueXiTUserCache, p *xuexitong.PointResourceDto) (string, error) {
	if p == nil {
		return "", nil
	}
	if p.Type != ctype.DownloadFile {
		// ballchart 没有可用的上报接口，静默跳过
		return "", nil
	}
	if p.Mid == "" || p.FileType == "" {
		return "", nil
	}

	report, err := cache.ResourceDownloaderReport(p, 3, nil)
	if err != nil && strings.Contains(err.Error(), "status code: 500") {
		xuexitong2.ReLogin(cache) //重登
		report, err = cache.ResourceDownloaderReport(p, 3, nil)
	}
	if err != nil {
		return "", err
	}

	ok := gojsonq.New().JSONString(report).Find("status")
	if ok == true {
		log2.Print(log2.DEBUG, "(", p.Title, ")资料下载已记录")
	} else {
		log2.Print(log2.DEBUG, "(", p.Title, ")资料下载记录失败：", gojsonq.New().JSONString(report).Find("errorMsg"))
	}
	return report, nil
}

// HasReportableContent 报告该资源条目是否存在可上报的接口。
// 调用方可以用它决定日志措辞（跳过的 vs 处理失败的）。
func HasReportableContent(p *xuexitong.PointResourceDto) bool {
	return p != nil && p.Type == ctype.DownloadFile && p.Mid != "" && p.FileType != ""
}
