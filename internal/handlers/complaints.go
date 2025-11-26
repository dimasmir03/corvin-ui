package handlers

import (
	"log"
	"net/http"
	"strconv"

	"vpnpanel/internal/broker"
	"vpnpanel/internal/db"
	"vpnpanel/internal/handlers/response"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
)

type CommplaintsController struct {
	repo *repository.ComplaintRepository
}

func NewComplaintsController(r *gin.RouterGroup) *CommplaintsController {
	apiController := &CommplaintsController{repo: repository.NewComplaintRepo(db.DB)}
	apiController.Routes(r)
	return apiController
}

func (s CommplaintsController) Routes(r *gin.RouterGroup) {
	r.GET("/all", s.getAllComplaints)
	r.GET("/:id", s.getComplaint)
	r.POST("/create", s.createComplaint)
	r.POST("/:id/delete", s.deleteComplaint)
	r.POST("/:id/update", s.updateComplaint)
	r.POST("/:id/reply", s.replyComplaint)
}

func (s CommplaintsController) getAllComplaints(c *gin.Context) {
	complaints, err := s.repo.GetAllComplaints()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{Success: false, Msg: "Failed to get all complaints"})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Msg: "", Obj: complaints})
}

func (s CommplaintsController) getComplaint(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: "Invalid ID"})
		return
	}
	complaint, err := s.repo.GetByIDComplaint(uint(id))
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{Success: false, Msg: "Failed to get complaint"})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Msg: "", Obj: complaint})
}

func (s CommplaintsController) createComplaint(c *gin.Context) {
	var complaint models.Complaint
	if err := c.BindJSON(&complaint); err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Invalid request body: " + err.Error()})
		return
	}
	if err := s.repo.CreateComplaint(&complaint); err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Failed to create complaint:" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Msg: "", Obj: complaint})
}

func (s CommplaintsController) deleteComplaint(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: "Invalid ID"})
		return
	}
	if err := s.repo.DeleteComplaint(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{Success: false, Msg: "Failed to delete complaint"})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Msg: "Complaint deleted successfully", Obj: nil})
}

func (s CommplaintsController) updateComplaint(c *gin.Context) {
	// idStr := c.Param("id")
	// id, err := strconv.ParseUint(idStr, 10, 64)
	// if err != nil {
	// 	c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: "Invalid ID"})
	// 	return
	// }
	var complaint models.Complaint
	if err := c.BindJSON(&complaint); err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: "Invalid request body"})
		return
	}
	if err := s.repo.UpdateComplaint(&complaint); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{Success: false, Msg: "Failed to update complaint"})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Msg: "", Obj: complaint})

}

// replyComplaint
func (s CommplaintsController) replyComplaint(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: "Invalid ID"})
		return
	}

	var body struct {
		Reply string `json:"reply"`
	}

	if err := c.BindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: "Invalid body"})
		return
	}

	// сохраняем ответ в БД
	if err := s.repo.UpdateReply(uint(id), body.Reply); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{Success: false, Msg: "Failed to update complaint"})
		return
	}

	complaint, err := s.repo.GetByIDComplaint(uint(id))
	if err != nil {
		c.JSON(http.StatusNotFound, response.Response{Success: false, Msg: "Complaint not found"})
		return
	}

	// Отправляем в RabbitMQ
	task := broker.ComplaintReplyTask{
		ComplaintID: complaint.ID,
		TgID:        complaint.TgID,
		UserID:      complaint.UserID,
		Reply:       body.Reply,
	}

	if err := broker.GlobalProducer.PublishComplaintReply(task); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{
			Success: false,
			Msg:     "Failed to send reply via broker:" + err.Error(),
		})
		return
	}

	log.Println("ответ вроде отправлен")

	c.JSON(http.StatusOK, response.Response{Success: true})
}
