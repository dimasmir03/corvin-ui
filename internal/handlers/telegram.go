package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/db"
	"vpnpanel/internal/handlers/response"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TelegramController struct {
	repo *repository.TelegramRepo
}

func NewTelegramController(r *gin.RouterGroup) *TelegramController {
	telegramController := &TelegramController{repo: repository.NewTelegramRepo(db.DB)}
	telegramController.Routes(r)
	return telegramController
}

func (s TelegramController) Routes(r *gin.RouterGroup) {
	r.POST("/user/create", s.CreateUser)
	r.GET("/user/:tg_id", s.GetUser)
	r.POST("/vpn/create", s.CreateVpn)
	r.GET("/vpn/:tg_id", s.GetVpn)
	r.GET("/allusers", s.GetAllUsers)
	r.POST("/complaints/create", s.CreateComplaint)
	r.POST("/complaints/:id/update", s.UpdateComplaint)
}

func (s TelegramController) CreateUser(c *gin.Context) {
	var dto response.CreateUserDTO

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, Response{false, err.Error(), nil})
		return
	}

	user, err := s.repo.CreateUser(models.Telegram{
		TgID:      dto.TgID,
		Username:  dto.Username,
		Firstname: dto.Firstname,
		Lastname:  dto.Lastname,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Msg:     "user created",
		Obj: response.ClientDTO{
			ID:        uint(user.ID),
			TgID:      user.TgID,
			Username:  user.Username,
			Firstname: user.Firstname,
			Lastname:  user.Lastname,
		},
	})
}

func (s TelegramController) GetUser(c *gin.Context) {
	tgID := c.Param("tg_id")
	user, err := s.repo.GetUser(tgID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusNotFound, Response{
				Success: false,
				Msg:     "record not found",
				Obj:     nil,
			})
			return
		}
		c.JSON(http.StatusInternalServerError, Response{
			Success: false,
			Msg:     err.Error(),
			Obj:     nil,
		})
		return
	}
	c.JSON(http.StatusOK, Response{
		Success: true,
		Msg:     "",
		Obj:     user,
	})
}

func (s TelegramController) CreateVpn(c *gin.Context) {
	var dto response.CreateVpnDTO

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, Response{false, err.Error(), nil})
		return
	}

	// link generation (заглушка)
	// link := "https://vpn.example.com/profile/" + fmt.Sprint(dto.TgID)

	vlesParams := utils.GenVlessLink(dto.TgID)
	vpn, err := s.repo.CreateVpn(dto.TgID, vlesParams.UID, vlesParams.Link)

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
		return
	}

	// Отправляем в RabbitMQ
	task := broker.CreateUserTask{
		UserID:     dto.TgID,
		Username:   vlesParams.Name,
		UUID:       vlesParams.UID,
		PBK:        vlesParams.PBK,
		SID:        vlesParams.SID,
		SPX:        vlesParams.SPX,
		Flow:       vlesParams.Flow,
		Encryption: vlesParams.Encryption,
	}

	if err := broker.GlobalProducer.PublishCreateUser(task); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{
			Success: false,
			Msg:     "Failed to send user create task in broker:" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Msg:     "vpn created",
		Obj: response.VpnResult{
			TgID: dto.TgID,
			Link: vpn.Link,
		},
	})
}

func (s TelegramController) GetVpn(c *gin.Context) {
	tgID, err := strconv.ParseInt(c.Param("tg_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{false, err.Error(), nil})
		return
	}

	vpn, err := s.repo.GetVpn(tgID)
	if err != nil {
		c.JSON(http.StatusOK, Response{false, err.Error(), nil})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Msg:     "",
		Obj: response.VpnResult{
			TgID: tgID,
			Link: vpn.Link,
		},
	})
}

func (s TelegramController) GetAllUsers(c *gin.Context) {
	users, err := s.repo.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
		return
	}

	var out []response.ClientDTO
	for _, u := range users {
		out = append(out, response.ClientDTO{
			TgID:      u.TgID,
			Username:  u.Username,
			Firstname: u.Firstname,
			Lastname:  u.Lastname,
		})
	}

	c.JSON(http.StatusOK, Response{true, "", out})
}

func (s TelegramController) CreateComplaint(c *gin.Context) {
	var dto response.CreateComplaintDTO

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, Response{false, err.Error(), nil})
		return
	}

	com, err := s.repo.CreateComplaint(dto.TgID, dto.Username, dto.Text)

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Msg:     "complaint created",
		Obj: map[string]uint{
			"complaintId": com.ID,
		},
	})
}

func (s TelegramController) UpdateComplaint(c *gin.Context) {
	var dto response.UpdateComplaintDTO

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, Response{false, err.Error(), nil})
		return
	}

	err := s.repo.UpdateComplaint(dto.ComplaintID, dto.AdminReply, dto.Status)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Obj: map[string]uint{
			"complaintId": dto.ComplaintID,
		},
	})
}

type Response struct {
	Success bool   `json:"success"`
	Msg     string `json:"msg"`
	Obj     any    `json:"obj"`
}
