package client

import "time"

var (
	delta = time.Second
)

// 评估器，用于评估StreamGroup的负载情况，并根据负载情况调整StreamGroup中的容器数量
// 指标为错误率, 错误率超过一定阈值时，释放容器，低于一定阈值时，创建容器
type GroupEvaluator struct {
	groupRewardLimit      float64
	containerPenaltyLimit float64
}

func NewEvaluator(groupRewardLimit, containerPenaltyLimit float64) *GroupEvaluator {
	return &GroupEvaluator{
		groupRewardLimit:      groupRewardLimit,
		containerPenaltyLimit: containerPenaltyLimit,
	}
}

// 开始评估group
func (e *GroupEvaluator) Evaluate(group *StreamGroup) {
	// 评估间隔, 初始为60s, 会根据group下的container运行情况动态调整, 最小为1s
	multiple := 60
	for {
		time.Sleep(delta * time.Duration(max(1, multiple)))
		PM, EM, n := float64(0), float64(0), group.GetNofSC()
		for i := int64(0); i < n; i++ {
			container := group.GetSC()
			// 如果容器已经关闭，则释放
			if container.IsClosed() {
				group.ReleaseSC(container)
				continue
			}
			tPM, tEM := container.GetNofPM(), container.GetNofEM()
			PM += tPM
			EM += tEM
			if tEM/tPM > e.containerPenaltyLimit {
				group.ReleaseSC(container)
			} else {
				group.PutSC(container)
			}
		}
		if EM/PM < e.groupRewardLimit {
			group.CreateSC()
			multiple++
		} else {
			multiple--
		}
	}
}

// 关闭group下的所有container
func (e *GroupEvaluator) Close(group *StreamGroup) {
	for i := int64(0); i < group.GetNofSC(); i++ {
		group.ReleaseSC(group.GetSC())
	}
}
