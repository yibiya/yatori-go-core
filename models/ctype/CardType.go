package ctype

type CardType string

const (
	Video        CardType = "insertvideo"  // 视屏
	Work         CardType = "work"         //作业
	Insertdoc    CardType = "insertdoc"    //内置文档
	Document     CardType = "document"     //文档
	InsertBook   CardType = "insertbook"   //内置书
	Hyperlink    CardType = "hyperlink"    //外部链接
	Insertlive   CardType = "insertlive"   //直播
	Insertbbs    CardType = "insertbbs"    //讨论
	InsertReadV2 CardType = "insertreadV2" //阅读任务点
	InsertAudio  CardType = "insertaudio"  //音频任务点

	// 以下是章节里出现但「不是任务点」的资源模块。它们的 iframe data 里
	// 没有 jobid/_jobid/jtoken，学习通后台没有可上报的 job，提交只会得到
	// 「无效的请求参数」，因此识别出来静默跳过，不要当成未知类型报警。
	// 见 2026-10-31 日志: 三维计算机辅助设计的 .prt/.drw 附件、
	// 中国近现代史纲要的「史实」pdf 附件、洋务运动的知识点球图。
	DownloadFile CardType = "downloadfile" //资料下载附件（pdf/prt/drw 等）
	BallChart    CardType = "ballchart"    //知识点球图（可视化资源）
)

// ResourceOnly 报告该 module 是否属于「章节里会出现但不是任务点」的资源模块。
//
// 这类 iframe 的 data 只有 objectid/name/mid/size/type，没有 jobid、_jobid
// 或 jtoken，学习通后台也没有对应的提交接口 —— 提交只会得到「无效的请求参数」。
// 因此解析时直接跳过，不要落到「未知的任务点类型」分支。
func (c CardType) ResourceOnly() bool {
	switch c {
	case DownloadFile, BallChart:
		return true
	}
	return false
}
