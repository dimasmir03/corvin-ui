package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/broker"
	"vpnpanel/internal/handlers/response"
	"vpnpanel/internal/jobsvc"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/service"
	"vpnpanel/internal/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type TelegramController struct {
	teleRepo     *repository.TelegramRepo
	usersService *service.UsersService
	vpnService   *service.VPNService
	storage      *repository.StorageRepo
	jobs         *jobsvc.Service
	audit        *audit.Logger
}

func NewTelegramController(
	repo *repository.StorageRepo,
	teleRepo *repository.TelegramRepo,
	usersService *service.UsersService,
	vpnService *service.VPNService,
	jobs *jobsvc.Service,
	auditLogger *audit.Logger,
) *TelegramController {
	return &TelegramController{
		storage:      repo,
		teleRepo:     teleRepo,
		usersService: usersService,
		vpnService:   vpnService,
		jobs:         jobs,
		audit:        auditLogger,
	}
}

func (s TelegramController) Register(r *gin.RouterGroup) {
	r.POST("/user/create", s.CreateUser)
	r.GET("/user/:tg_id", s.GetUser)
	r.POST("/vpn/create", s.CreateVpn)
	r.POST("/vpn/create/:protocol", s.CreateVpnProtocol)
	r.GET("/vpn/:tg_id/", s.GetVpn)
	r.GET("/vpn/:tg_id/:protocol", s.GetVpnLinkByProtocol)
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

	user, err := s.usersService.EnsureTelegramUser(service.TelegramUserInput{
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
	tgID, err := strconv.ParseInt(c.Param("tg_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{
			Success: false,
			Msg:     err.Error(),
			Obj:     nil,
		})
		return
	}

	user, err := s.usersService.GetTelegramByTgID(tgID)
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

	vlessParams := utils.GenVlessLink(dto.TgID)

	trojanParams := utils.GenTrojanLink(dto.TgID)

	telegram, err := s.teleRepo.GetTelegramByTgID(dto.TgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
		return
	}

	vpn, err := s.teleRepo.CreateVpn(repository.CreateVpnParams{
		UserID:     telegram.UserID,
		UUID:       vlessParams.UID,
		Status:     "active",
		VlessLink:  vlessParams.Link,
		TrojanLink: trojanParams.Link,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
		return
	}

	if s.jobs != nil {
		batch, jobs, err := s.jobs.CreateUserConfig(jobsvc.CreateUserConfigInput{
			UserID:            telegram.UserID,
			TechnicalClientID: vlessParams.UID,
			Protocols:         []string{"vless", "trojan"},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.Response{
				Success: false,
				Msg:     "Failed to create vpn jobs:" + err.Error(),
			})
			return
		}
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorTelegramUser,
			ActorID:    audit.StringID(dto.TgID),
			Action:     "vpn.client.created",
			EntityType: "job_batch",
			EntityID:   audit.StringID(batch.ID),
			Status:     audit.StatusSuccess,
			Message:    "vpn create jobs queued",
			Metadata: map[string]any{
				"jobs_count": len(jobs),
				"user_id":    telegram.UserID,
			},
		})
	}

	// Отправляем legacy task в RabbitMQ
	task := broker.CreateUserTask{
		UserID:     dto.TgID,
		Username:   vlessParams.Name,
		UUID:       vlessParams.UID,
		PBK:        vlessParams.PBK,
		SID:        vlessParams.SID,
		SPX:        vlessParams.SPX,
		Flow:       vlessParams.Flow,
		Encryption: vlessParams.Encryption,

		Type:     trojanParams.Type,
		Security: trojanParams.Security,
		Fp:       trojanParams.Fp,
		Alpn:     trojanParams.Alpn,
		Sni:      trojanParams.Sni,
		Password: trojanParams.Password,
	}

	if err := broker.GlobalProducer.PublishCreateUser(task); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{
			Success: false,
			Msg:     "Failed to send create user task in broker:" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Msg:     "vpn created",
		Obj: response.VpnResult{
			TgID:       dto.TgID,
			VlessLink:  vpn.VlessLink,
			TrojanLink: vpn.TrojanLink,
		},
	})
}

func (s TelegramController) CreateVpnProtocol(c *gin.Context) {
	protocol := c.Param("protocol")
	var dto response.CreateVpnDTO

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusBadRequest, Response{false, err.Error(), nil})
		return
	}

	var vlessParams utils.VlessParams
	var trojanParams utils.TrojanParams
	switch protocol {
	case "vless":
		vlessParams = utils.GenVlessLink(dto.TgID)
	case "trojan":
		trojanParams = utils.GenTrojanLink(dto.TgID)
	}

	telegram, err := s.teleRepo.GetTelegramByTgID(dto.TgID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
		return
	}

	switch protocol {
	case "vless":
		_, err := s.teleRepo.CreateVpnProtocol(repository.CreateVpnParams{
			TgID:      telegram.TgID,
			UserID:    telegram.UserID,
			VlessLink: vlessParams.Link,
		}, protocol)

		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
			return
		}

	case "trojan":
		_, err := s.teleRepo.CreateVpnProtocol(repository.CreateVpnParams{
			TgID:       telegram.TgID,
			UserID:     telegram.UserID,
			TrojanLink: trojanParams.Link,
		}, protocol)

		if err != nil {
			c.JSON(http.StatusInternalServerError, Response{false, err.Error(), nil})
			return
		}

	}

	var Username string
	switch protocol {
	case "vless":
		Username = vlessParams.Name
	case "trojan":
		Username = trojanParams.Name
	}

	if s.jobs != nil {
		technicalClientID := vlessParams.UID
		if protocol == "trojan" {
			technicalClientID = trojanParams.Password
		}
		batch, jobs, err := s.jobs.CreateUserConfig(jobsvc.CreateUserConfigInput{
			UserID:            telegram.UserID,
			TechnicalClientID: technicalClientID,
			Protocols:         []string{protocol},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, response.Response{
				Success: false,
				Msg:     "Failed to create vpn jobs:" + err.Error(),
			})
			return
		}
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorTelegramUser,
			ActorID:    audit.StringID(dto.TgID),
			Action:     "vpn.client.created",
			EntityType: "job_batch",
			EntityID:   audit.StringID(batch.ID),
			Status:     audit.StatusSuccess,
			Message:    "vpn protocol create jobs queued",
			Metadata: map[string]any{
				"jobs_count": len(jobs),
				"protocol":   protocol,
				"user_id":    telegram.UserID,
			},
		})
	}

	// Отправляем legacy task в RabbitMQ
	task := broker.CreateUserTask{
		UserID:     dto.TgID,
		Username:   Username,
		UUID:       vlessParams.UID,
		PBK:        vlessParams.PBK,
		SID:        vlessParams.SID,
		SPX:        vlessParams.SPX,
		Flow:       vlessParams.Flow,
		Encryption: vlessParams.Encryption,

		Type:     trojanParams.Type,
		Security: trojanParams.Security,
		Fp:       trojanParams.Fp,
		Alpn:     trojanParams.Alpn,
		Sni:      trojanParams.Sni,
		Password: trojanParams.Password,
	}

	if err := broker.GlobalProducer.PublishCreateUser(task); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{
			Success: false,
			Msg:     "Failed to send create user task in broker:" + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Msg:     "vpn created",
	})
}

func (s TelegramController) GetVpn(c *gin.Context) {
	tgID, err := strconv.ParseInt(c.Param("tg_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{false, err.Error(), nil})
		return
	}

	vpn, err := s.vpnService.GetVPNByTelegramID(tgID)
	if err != nil {
		c.JSON(http.StatusOK, Response{false, err.Error(), nil})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Msg:     "",
		Obj: response.VpnResult{
			TgID:       tgID,
			VlessLink:  vpn.VlessLink,
			TrojanLink: vpn.TrojanLink,
		},
	})
}

func (s TelegramController) GetVpnLinkByProtocol(c *gin.Context) {
	tgID, err := strconv.ParseInt(c.Param("tg_id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, Response{false, "invalid tg_id", nil})
		return
	}

	protocol := c.Param("protocol")

	link, err := s.vpnService.GetVPNLinkByProtocol(tgID, protocol)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, Response{
			false,
			"user not found",
			nil,
		})
		return
	}
	if err != nil {
		c.JSON(http.StatusInternalServerError, Response{
			false,
			"error",
			nil,
		})
		return
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Msg:     "",
		Obj: map[string]interface{}{
			"link": link, // "" или ссылка
		},
	})
}

func (s TelegramController) GetAllUsers(c *gin.Context) {
	users, err := s.usersService.GetAllTelegramUsers()
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

	if err := c.ShouldBind(&dto); err != nil {
		c.JSON(http.StatusOK, Response{false, err.Error(), nil})
		return
	}

	com, err := s.teleRepo.CreateComplaint(dto.TgID, dto.Username, dto.Text)

	if err != nil {
		c.JSON(http.StatusOK, Response{false, "failed create complaint in db:" + err.Error(), nil})
		return
	}

	if dto.HasPhoto {
		fileHeader, err := c.FormFile("photo")
		if err != nil && err != http.ErrMissingFile {
			c.JSON(http.StatusOK, Response{
				false,
				"failed to read photo",
				nil,
			})
			return
		}

		var photoURL string
		// Если фото есть — загружаем в MinIO
		if fileHeader != nil {
			src, err := fileHeader.Open()
			if err != nil {
				c.JSON(http.StatusOK, Response{false, "failed to open:" + err.Error(), nil})
				return
			}
			defer src.Close()

			objectName := fmt.Sprintf("%d_%s", com.ID, fileHeader.Filename)

			photoURL, err = s.storage.UploadFile(src, objectName, fileHeader.Header.Get("Content-Type"))
			if err != nil {
				c.JSON(http.StatusOK, Response{false, "failed to upload photo:" + err.Error(), nil})
				return
			}
		}

		err = s.teleRepo.UpdateComplaintPhotoURL(com.ID, photoURL)
		if err != nil {
			c.JSON(http.StatusOK, Response{false, "failed to update photo url:" + err.Error(), nil})
			return
		}
	}

	c.JSON(http.StatusOK, Response{
		Success: true,
		Obj: map[string]uint{
			"complaint_id": com.ID,
		},
	})
}

func (s TelegramController) UpdateComplaint(c *gin.Context) {
	var dto response.UpdateComplaintDTO

	if err := c.ShouldBindJSON(&dto); err != nil {
		c.JSON(http.StatusOK, Response{false, err.Error(), nil})
		return
	}

	complaint, err := s.teleRepo.UpdateComplaint(dto.ComplaintID, dto.AdminReply, dto.Status)
	if err != nil {
		c.JSON(http.StatusOK, Response{false, err.Error(), nil})
		return
	}

	// Отправляем в RabbitMQ
	task := broker.ComplaintReplyTask{
		ComplaintID: complaint.ID,
		TgID:        complaint.TgID,
		UserID:      complaint.UserID,
		Reply:       complaint.Reply,
	}

	if err := broker.GlobalProducer.PublishComplaintReply(task); err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{
			Success: false,
			Msg:     "Failed to send reply via broker:" + err.Error(),
		})
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
