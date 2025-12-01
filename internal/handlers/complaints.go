package handlers

import (
	"net/http"
	"strconv"

	"vpnpanel/internal/broker"
	"vpnpanel/internal/handlers/response"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
)

type ComplaintsController struct {
	repo *repository.ComplaintRepository
}

func NewComplaintsController(repo *repository.ComplaintRepository) *ComplaintsController {
	return &ComplaintsController{repo: repo}
}

func (s ComplaintsController) Register(r *gin.RouterGroup) {
	r.GET("/all", s.getAll)
	r.GET("/:id", s.getByID)
	r.POST("/create", s.create)
	r.POST("/:id/delete", s.delete)
	// r.POST("/:id/update", s.update)
	r.POST("/:id/reply", s.reply)
}

func (s ComplaintsController) getAll(c *gin.Context) {
	complaints, err := s.repo.GetAll()
	if err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Failed to get all complaints"})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Obj: complaints})
}

func (s ComplaintsController) getByID(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Invalid ID"})
		return
	}
	complaint, err := s.repo.GetByID(uint(id))
	if err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Failed to get complaint"})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Obj: complaint})
}

func (s ComplaintsController) create(c *gin.Context) {
	var dto response.CreateComplaintDTO
	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Invalid request body: " + err.Error()})
		return
	}

	complaint := &models.Complaint{
		TgID:     dto.TgID,
		Username: dto.Username,
		Text:     dto.Text,
		Status:   "new",
	}
	if err := s.repo.Create(complaint); err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Failed to create complaint:" + err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Obj: complaint})
}

func (s ComplaintsController) reply(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Invalid ID"})
		return
	}

	var body struct {
		Reply string `json:"reply"`
	}

	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Invalid body"})
		return
	}

	if err := s.repo.UpdateReply(uint(id), body.Reply); err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Failed to update complaint"})
		return
	}

	complaint, _ := s.repo.GetByID(uint(id))
	task := broker.ComplaintReplyTask{
		ComplaintID: complaint.ID,
		TgID:        complaint.TgID,
		UserID:      complaint.UserID,
		Reply:       body.Reply,
	}

	if err := broker.GlobalProducer.PublishComplaintReply(task); err != nil {
		c.JSON(http.StatusOK, response.Response{
			Success: false,
			Msg:     "Failed to send reply via broker:" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response.Response{Success: true})
}

func (s ComplaintsController) delete(c *gin.Context) {
	idStr := c.Param("id")
	id, err := strconv.ParseUint(idStr, 10, 64)
	if err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Invalid ID"})
		return
	}
	if err := s.repo.Delete(uint(id)); err != nil {
		c.JSON(http.StatusOK, response.Response{Success: false, Msg: "Failed to delete complaint"})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Msg: "Complaint deleted successfully", Obj: nil})
}
