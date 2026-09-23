package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/db"
	"vpnpanel/internal/handlers/response"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/service"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

type UserController struct {
	users *repository.UserRepo
	vpn   *service.VPNService
	audit *audit.Logger
}

func NewUserController(users *repository.UserRepo, vpnService *service.VPNService, auditLogger *audit.Logger) *UserController {
	return &UserController{users: users, vpn: vpnService, audit: auditLogger}
}

func (s *UserController) Register(r *gin.RouterGroup) {
	r.GET("/all", s.GetAllUsers)
	r.GET("/:id/vpn", s.GetUserVPN)
	r.PUT("/:id/subscription", s.UpdateSubscription)
	r.PUT("/:id/servers/:server_id", s.SetServerAccess)
	r.POST("/:id/servers/:server_id/provision", s.ProvisionServer)
	r.POST("/:id/connections/:connection_id/state", s.SetConnectionState)
	r.POST("/create", s.CreateUser)
	r.GET("/:id", s.GetUser)
	r.POST("/:id/edit", s.UpdateUser)
	r.POST("/:id/edit/status", s.UpdateStatusUser)
	r.POST("/:id/delete", s.DeleteUser)
}

type updateSubscriptionRequest struct {
	Status      string     `json:"status" binding:"required"`
	TariffName  string     `json:"tariff_name"`
	ExpiresAt   *time.Time `json:"expires_at"`
	DeviceLimit int        `json:"device_limit"`
}

type setServerAccessRequest struct {
	Enabled     bool       `json:"enabled"`
	AllowVLESS  bool       `json:"allow_vless"`
	AllowTrojan bool       `json:"allow_trojan"`
	ValidUntil  *time.Time `json:"valid_until"`
}

type provisionServerRequest struct {
	DeviceID *string `json:"device_id"`
	Protocol string  `json:"protocol" binding:"required"`
}

type setConnectionStateRequest struct {
	DesiredState string `json:"desired_state" binding:"required"`
}

func (s *UserController) GetAllUsers(c *gin.Context) {
	users, err := s.users.GetAllUsers()
	if err != nil {
		c.JSON(http.StatusInternalServerError, response.Response{Success: false, Msg: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response.Response{
		Success: true,
		Obj:     users,
	})
}

func (s *UserController) GetUserVPN(c *gin.Context) {
	requestID, _ := c.Get("request_id")
	if s.vpn == nil {
		logger.Error("user vpn details failed", nil, "component", "http_api", "handler", "user_vpn", "operation", "get_user_vpn", "request_id", requestID, "reason", "vpn_service_not_configured")
		c.JSON(http.StatusInternalServerError, response.Response{Success: false, Msg: "vpn service is not configured"})
		return
	}
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		logger.Warn("user vpn details validation failed", "component", "http_api", "handler", "user_vpn", "operation", "get_user_vpn", "request_id", requestID, "reason", "invalid_user_id")
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: "invalid user id"})
		return
	}
	logger.Info("user vpn details service call started", "component", "http_api", "handler", "user_vpn", "operation", "get_user_vpn", "request_id", requestID, "user_id", id)
	details, err := s.vpn.GetUserVPNDetails(uint(id))
	if err != nil {
		logger.Error("user vpn details service call failed", err, "component", "http_api", "handler", "user_vpn", "operation", "get_user_vpn", "request_id", requestID, "user_id", id)
		c.JSON(http.StatusInternalServerError, response.Response{Success: false, Msg: err.Error()})
		return
	}
	profilesCount := len(details.Profiles)
	clientCode := ""
	if details.Client != nil {
		clientCode = details.Client.ClientCode
	}
	logger.Info("user vpn details service call succeeded", "component", "http_api", "handler", "user_vpn", "operation", "get_user_vpn", "request_id", requestID, "user_id", id, "client_code", clientCode, "profiles_count", profilesCount)
	c.JSON(http.StatusOK, response.Response{Success: true, Obj: details})
}

func (s *UserController) UpdateSubscription(c *gin.Context) {
	userID, ok := userIDFromParam(c)
	if !ok {
		return
	}
	var request updateSubscriptionRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: err.Error()})
		return
	}
	subscription, err := s.vpn.UpdateUserSubscription(userID, service.UpdateSubscriptionInput{Status: request.Status, TariffName: request.TariffName, ExpiresAt: request.ExpiresAt, DeviceLimit: request.DeviceLimit})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidSubscription) {
			status = http.StatusBadRequest
		}
		c.JSON(status, response.Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Obj: subscription})
}

func (s *UserController) SetServerAccess(c *gin.Context) {
	userID, ok := userIDFromParam(c)
	if !ok {
		return
	}
	var request setServerAccessRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: err.Error()})
		return
	}
	access, err := s.vpn.SetUserServerAccess(userID, c.Param("server_id"), service.SetServerAccessInput{Enabled: request.Enabled, AllowVLESS: request.AllowVLESS, AllowTrojan: request.AllowTrojan, ValidUntil: request.ValidUntil})
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrInvalidServerAccess) {
			status = http.StatusBadRequest
		}
		c.JSON(status, response.Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusOK, response.Response{Success: true, Obj: access})
}

func (s *UserController) ProvisionServer(c *gin.Context) {
	userID, ok := userIDFromParam(c)
	if !ok {
		return
	}
	var request provisionServerRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: err.Error()})
		return
	}
	result, err := s.vpn.ProvisionUserServer(userID, request.DeviceID, c.Param("server_id"), request.Protocol)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, service.ErrUnsupportedProtocol) || errors.Is(err, service.ErrInvalidServerAccess) || errors.Is(err, service.ErrNoMatchingServers) || errors.Is(err, service.ErrSubscriptionInactive) || errors.Is(err, service.ErrDeviceRevoked) {
			status = http.StatusBadRequest
		}
		c.JSON(status, response.Response{Success: false, Msg: err.Error()})
		return
	}
	status := http.StatusAccepted
	if result.CommandsCount == 0 {
		status = http.StatusOK
	}
	c.JSON(status, response.Response{Success: true, Obj: result})
}

func (s *UserController) SetConnectionState(c *gin.Context) {
	userID, ok := userIDFromParam(c)
	if !ok {
		return
	}
	connectionID, err := strconv.ParseUint(c.Param("connection_id"), 10, 64)
	if err != nil || connectionID == 0 {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: "invalid connection id"})
		return
	}
	var request setConnectionStateRequest
	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: err.Error()})
		return
	}
	result, err := s.vpn.RequestConnectionState(userID, uint(connectionID), request.DesiredState)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, response.Response{Success: false, Msg: "connection not found"})
		return
	}
	if err != nil {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, response.Response{Success: true, Obj: result})
}

func userIDFromParam(c *gin.Context) (uint, bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 64)
	if err != nil || id == 0 {
		c.JSON(http.StatusBadRequest, response.Response{Success: false, Msg: "invalid user id"})
		return 0, false
	}
	return uint(id), true
}

func (s *UserController) CreateUser(c *gin.Context) {
	// var user models.User
	// if err := c.Bind(&user); err != nil {
	// 	c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	// 	return
	// }

	// hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	// if err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	// 	return
	// }

	// user.Password = string(hash)

	// if err := db.DB.Create(&user).Error; err != nil {
	// 	c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
	// 	return
	// }

	// servers := c.MustGet("servers").([]string)
	// db.DB.Where("user_id = ?", user.ID).Delete(&models.UserServer{})
	// for _, sid := range servers {
	// 	id, err := strconv.Atoi(sid)
	// 	if err != nil {
	// 		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
	// 		return
	// 	}
	// 	db.DB.Create(&models.UserServer{UserID: user.ID, ServerID: uint(id)})
	// }

	// c.Redirect(http.StatusSeeOther, "/users")
}

func (s *UserController) GetUser(c *gin.Context) {

}

func (s *UserController) UpdateUser(c *gin.Context) {
	// id, exists := c.Get("id")
	// if !exists {
	// 	c.Error(errors.New("id is required"))
	// 	return
	// }

	// var user models.User
	// db.DB.First(&user, id)

	// if err := c.Bind(&user); err != nil {
	// 	c.Error(err)
	// 	return
	// }

	// if user.Password != "" {
	// 	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	// 	if err != nil {
	// 		c.Error(err)
	// 		return
	// 	}
	// 	user.Password = string(hash)
	// }

	// db.DB.Save(&user)
	// serverIDs := c.Request.Form["servers"] // массив выбранных ID

	// db.DB.Where("user_id = ?", user.ID).Delete(&models.UserServer{})
	// for _, sid := range serverIDs {
	// 	id, err := strconv.Atoi(sid)
	// 	if err != nil {
	// 		c.Error(err)
	// 		return
	// 	}
	// 	db.DB.Create(&models.UserServer{UserID: user.ID, ServerID: uint(id)})
	// }

	// c.Redirect(http.StatusSeeOther, "/users")
}

func (s *UserController) UpdateStatusUser(c *gin.Context) {
	id := c.Param("id")

	///////////////////////////
	// DEBUG BLOCK ////////////
	////////////////////////////
	// body, err := io.ReadAll(c.Request.Body)
	// if err != nil {
	// 	logger.Printf("Failed to read response.Response body: %v\n", err)
	// }
	// // req url
	// logger.Println("Request URL:", c.Request.URL.String())

	// // req header X-API-KEY
	// logger.Println("Request Header X-API-KEY:", c.Request.Header.Get("X-API-KEY"))

	// // logger.Println("response.Response status code:", c.Request.StatusCode)
	// // response.Response body as string
	// logger.Printf("response.Response body: %s\n", string(body))
	/////////////////////////////

	var userStatus struct {
		Status bool `json:"status"`
	}
	if err := c.BindJSON(&userStatus); err != nil {
		c.JSON(http.StatusOK,
			response.Response{
				Success: false,
				Msg:     err.Error(),
			},
		)
		return
	}
	var user models.User
	db.DB.First(&user, id)
	if user.ID == 0 {
		c.JSON(
			http.StatusBadRequest,
			response.Response{
				Success: false,
				Msg:     "user not found",
			},
		)
		return
	}
	oldStatus := user.Status
	user.Status = userStatus.Status
	db.DB.Save(&user)
	if oldStatus && !user.Status {
		_ = s.audit.Log(audit.Event{
			ActorType:  audit.ActorAdmin,
			Action:     "user.disabled",
			EntityType: "user",
			EntityID:   audit.StringID(user.ID),
			Status:     audit.StatusSuccess,
			Message:    "user disabled",
			IP:         c.ClientIP(),
			UserAgent:  c.Request.UserAgent(),
		})
	}
	c.JSON(http.StatusOK, response.Response{Success: true})
}

func (s *UserController) DeleteUser(c *gin.Context) {
	id, exists := c.Get("id")
	if !exists {
		c.Error(errors.New("id is required"))
		return
	}
	db.DB.Delete(&models.User{}, id)
	c.Redirect(http.StatusSeeOther, "/users")
}
