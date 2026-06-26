package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"vpnpanel/internal/audit"
	"vpnpanel/internal/db"
	"vpnpanel/internal/handlers/response"
	"vpnpanel/internal/logger"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"
	"vpnpanel/internal/service"

	"github.com/gin-gonic/gin"
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
	r.POST("/create", s.CreateUser)
	r.GET("/:id", s.GetUser)
	r.POST("/:id/edit", s.UpdateUser)
	r.POST("/:id/edit/status", s.UpdateStatusUser)
	r.POST("/:id/delete", s.DeleteUser)
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
