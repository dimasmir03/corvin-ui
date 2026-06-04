package handlers

import (
	"net/http"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/jobsvc"

	"github.com/gin-gonic/gin"
)

type JobsController struct {
	jobs *jobsvc.Service
}

func NewJobsController(jobs *jobsvc.Service) *JobsController {
	return &JobsController{jobs: jobs}
}

func (s *JobsController) Register(r *gin.RouterGroup) {
	r.POST("/result", s.ApplyResult)
}

func (s *JobsController) ApplyResult(c *gin.Context) {
	var event broker.JobResultEvent
	if err := c.ShouldBindJSON(&event); err != nil {
		c.JSON(http.StatusBadRequest, Response{Success: false, Msg: err.Error()})
		return
	}

	batch, job, err := s.jobs.ApplyResult(event)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{Success: false, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, Response{Success: true, Obj: gin.H{
		"batch_id":     batch.ID,
		"batch_status": batch.Status,
		"job_id":       job.ID,
		"job_status":   job.Status,
	}})
}
