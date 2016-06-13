package pipeline

import (
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/containerops/vessel/models"
	"github.com/containerops/vessel/module/dependence"
	"github.com/containerops/vessel/module/etcd"
	"github.com/containerops/vessel/module/scheduler"
	"github.com/containerops/vessel/utils"
	"github.com/containerops/vessel/utils/timer"
)

// StartPipeline start pipeline with PipelineSpecTemplate
func StartPipeline(pipelineTemplate *models.PipelineSpecTemplate) []byte {
	pipeline := pipelineTemplate.MetaData
	stageSpec := pipelineTemplate.Spec
	if status, err := etcd.GetPipelineStatus(pipeline); err == nil && status != "Deleted" {
		detail := fmt.Sprintf("Pipeline = %v in namespane = %v is already exist", pipeline.Name, pipeline.Namespace)
		bytes, _ := formatOutputBytes(pipelineTemplate, pipeline, nil, detail)
		return bytes
	}

	for _, stage := range stageSpec {
		stage.Namespace = pipeline.Namespace
		stage.PipelineName = pipeline.Name
		status, err := etcd.GetStageStatus(stage)
		if err == nil && status != "Deleted" {
			detail := fmt.Sprintf("Stage = %v in namespane = %v is already exist", stage.Name, stage.Namespace)
			bytes, _ := formatOutputBytes(pipelineTemplate, pipeline, nil, detail)
			return bytes
		}
	}

	executorMap, err := dependence.ParsePipelineTemplate(pipelineTemplate)
	if err != nil {
		bytes, _ := formatOutputBytes(pipelineTemplate, pipeline, nil, err.Error())
		return bytes
	}

	if err := etcd.SavePipeline(pipeline); err != nil {
		bytes, _ := formatOutputBytes(pipelineTemplate, pipeline, nil, err.Error())
		return bytes
	}

	pipeline.Status = models.StateReady
	if err := etcd.SetPipelineStatus(pipeline); err != nil {
		bytes, _ := formatOutputBytes(pipelineTemplate, pipeline, nil, err.Error())
		return bytes
	}

	schedulingRes := scheduler.StartStage(executorMap, timer.InitHourglass(time.Duration(pipeline.TimeoutDuration)*time.Second))
	bytes, success := formatOutputBytes(pipelineTemplate, pipeline, schedulingRes, "")
	etcd.SetCreationTimestamp(pipeline)
	if success {
		pipeline.Status = models.StateRunning
		etcd.SetPipelineStatus(pipeline)
	} else {
		//rollback by pipeline failed
		go scheduler.StopStage(executorMap, timer.InitHourglass(time.Duration(pipeline.TimeoutDuration)*time.Second))
	}
	log.Printf("Start pipeline name = %v in namespace '%v' is over", pipeline.Namespace, pipeline.Name)
	log.Print("Start job is done")
	return bytes
}

// StopPipeline stop pipeline with PipelineSpecTemplate
func StopPipeline(pipelineTemplate *models.PipelineSpecTemplate) []byte {
	pipeline := pipelineTemplate.MetaData
	stageSpec := pipelineTemplate.Spec
	if status, err := etcd.GetPipelineStatus(pipeline); err != nil || status == "Deleted" {
		detail := fmt.Sprintf("Pipeline = %v in namespane = %v is not start", pipeline.Name, pipeline.Namespace)
		bytes, _ := formatOutputBytes(pipelineTemplate, pipeline, nil, detail)
		return bytes
	}

	for _, stage := range stageSpec {
		stage.Namespace = pipeline.Namespace
		stage.PipelineName = pipeline.Name
		status, err := etcd.GetStageStatus(stage)
		if err != nil || status == "Deleted" {
			detail := fmt.Sprintf("Stage = %v in namespane = %v is not start", stage.Name, stage.Namespace)
			bytes, _ := formatOutputBytes(pipelineTemplate, pipeline, nil, detail)
			return bytes
		}
	}

	executorMap, err := dependence.ParsePipelineTemplate(pipelineTemplate)
	if err != nil {
		bytes, _ := formatOutputBytes(pipelineTemplate, pipeline, nil, err.Error())
		return bytes
	}

	schedulingRes := scheduler.StopStage(executorMap, timer.InitHourglass(time.Duration(pipeline.TimeoutDuration)*time.Second))
	bytes, success := formatOutputBytes(pipelineTemplate, pipeline, schedulingRes, "")
	if success {
		pipeline.Status = models.StateDeleted
		etcd.SetPipelineStatus(pipeline)
	}
	log.Printf("Delete pipeline name = %v in namespace '%v' is over", pipeline.Namespace, pipeline.Name)
	log.Print("Delete job is done")
	return bytes
}

func formatOutputBytes(pipelineTemplate *models.PipelineSpecTemplate, pipeline *models.Pipeline, schedulingRes []*models.ExecutedResult, pipelineDetail string) ([]byte, bool) {
	resultList := []interface{}{}
	status := models.ResultFailed
	if pipelineDetail == "" {
		status = models.ResultSuccess
		for _, result := range schedulingRes {
			resultList = append(resultList, result.Result)
			if status != result.Status {
				status = result.Status
				break
			}
		}
	}

	output := &models.PipelineResult{
		Namespace:      pipeline.Namespace,
		Name:           pipeline.Name,
		WorkspaceID:    1000,
		ProjectID:      2000,
		PipelineID:     utils.UUID(),
		PipelineDetail: pipelineDetail,
		Details:        resultList,
		PipelineSpec:   pipelineTemplate,
		Status:         status,
	}

	bytes, err := json.Marshal(output)
	if err != nil {
		log.Println(err)
	}
	return bytes, status == models.ResultSuccess
}
