package aiq

import (
	"sort"
	"sync"
	"testing"
	"time"
)

// 背景（实测 2026-11-01）：多账号同时刷课 + AI 答题时，请求以 0 秒间隔
// 连发，被上游 new-api 网关判为「批量探测模型」并自动封号。
// 这里锁住限速器的行为：相邻两次调用之间确实被拉开了间隔。

func resetPacer(t *testing.T) {
	t.Helper()
	prev := SetAICallInterval(0)
	aiCallMu.Lock()
	aiLastCallTime = time.Time{}
	aiCallMu.Unlock()
	t.Cleanup(func() { SetAICallInterval(prev) })
}

func TestWaitAISlotEnforcesInterval(t *testing.T) {
	resetPacer(t)
	defer SetAICallInterval(0)

	const interval = 120 * time.Millisecond
	SetAICallInterval(interval)

	start := time.Now()
	waitAISlot() // 第一次不该等
	first := time.Since(start)
	if first > interval/2 {
		t.Fatalf("第一次调用不应等待，实际等了 %v", first)
	}

	start = time.Now()
	waitAISlot() // 第二次应至少等一个间隔
	second := time.Since(start)
	if second < interval*4/5 {
		t.Fatalf("第二次调用间隔不足：等了 %v，期望至少 %v", second, interval)
	}
}

// 并发调用必须被排到不同时间点，而不是同时醒来一起冲
func TestWaitAISlotStaggersConcurrentCalls(t *testing.T) {
	resetPacer(t)
	const interval = 100 * time.Millisecond
	SetAICallInterval(interval)
	defer SetAICallInterval(0)

	const n = 3
	var wg sync.WaitGroup
	stamps := make([]time.Time, n)
	start := time.Now()
	for i := range n {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			waitAISlot()
			stamps[i] = time.Now()
		}(i)
	}
	wg.Wait()
	elapsed := time.Since(start)

	// 3 次调用至少应跨越 (n-1) 个间隔
	want := time.Duration(n-1) * interval
	if elapsed < want*9/10 {
		t.Fatalf("并发调用未被摊平：总耗时 %v，期望至少 %v", elapsed, want)
	}

	// 任意两次相邻调用之间都不该短于间隔的一半（排序后检查）
	sorted := append([]time.Time(nil), stamps...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Before(sorted[j]) })
	for i := 1; i < len(sorted); i++ {
		gap := sorted[i].Sub(sorted[i-1])
		if gap < interval/2 {
			t.Fatalf("第 %d 与第 %d 次调用仅相隔 %v，未达到限速目的", i-1, i, gap)
		}
	}
}

func TestSetAICallIntervalZeroDisables(t *testing.T) {
	resetPacer(t)
	SetAICallInterval(0)

	start := time.Now()
	for range 5 {
		waitAISlot()
	}
	if elapsed := time.Since(start); elapsed > 50*time.Millisecond {
		t.Fatalf("间隔设为 0 时应完全不等待，实际耗时 %v", elapsed)
	}
}

func TestAICallIntervalGetterMatchesSetter(t *testing.T) {
	resetPacer(t)
	want := 250 * time.Millisecond
	SetAICallInterval(want)
	if got := AICallInterval(); got != want {
		t.Fatalf("AICallInterval() = %v, want %v", got, want)
	}
	// 负数应被夹到 0
	SetAICallInterval(-time.Second)
	if got := AICallInterval(); got != 0 {
		t.Fatalf("负间隔应被夹到 0，实际 %v", got)
	}
}
