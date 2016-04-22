package kubernetes

import (
	"fmt"
	"time"

	"github.com/containerops/vessel/models"
)

func GetPipelineBussinessRes(pipelineVersion *models.PipelineSpecTemplate, ch chan bool) {
	namespace := pipelineVersion.MetaData.Namespace
	timeout := pipelineVersion.MetaData.TimeoutDuration
	// replicas := pipelineVersion.
	for _, stage := range pipelineVersion.Spec {
		replicas := stage.Replicas
		ipArray := make([]string, replicas)
		err := getPodIp(namespace, stage.Name, ipArray)
		if err != nil {
			ch <- false
			fmt.Printf("xxxxx%v\n", err)
			// fmt.Println("aaaaaaaaaaaaa")
			return
		}

		port := stage.Port
		statusCheckLink := stage.StatusCheckUrl
		statusCheckInterval := stage.StatusCheckInterval
		statusCheckCount := stage.StatusCheckCount
		podsCh := make([]string, replicas)
		for i := 0; i < replicas; i++ {
			checkUrl := fmt.Sprintf("https://%v:%v%v", ipArray[i], port, statusCheckLink)
			t := time.NewTimer(time.Second * time.Duration(timeout))
			go getPodBussinessRes(checkUrl, statusCheckInterval, statusCheckCount, podsCh[i])
		}

		podCh := make(chan bool)
		go wait(replicas, podsCh, podCh)

		select {
		case podRes := <-podCh:
			if podRes == false {
				fmt.Println("bbbbbbbbbbbbbb")
				ch <- false
			}
		case <-t.C:
			fmt.Println("cccccccccccccccccccc")
			ch <- false
		}
	}
	fmt.Println("dddddddddddddddddd")
	ch <- true
}

func getPodBussinessRes(checkUrl string, statusCheckInterval int64, statusCheckCount int, ch chan bool) {
	for i := 0; i < statusCheckCount; i++ {
		if i == 0 && 0 == requestBsRes(checkUrl) {
			ch <- true
			return
		}

		tick := time.NewTimer(time.Duration(statusCheckInterval) * time.Second)
		<-tick.C
		bsRes := requestBsRes(checkUrl)
		if bsRes == 200 {
			ch <- true
			return
		}
	}
	ch <- false
}

// getBsRes : request to checkUrl, get 200:success, 0, ignore, others, failed
func requestBsRes(checkUrl string) int {
	// read res from checkUrl
	return 200
}
