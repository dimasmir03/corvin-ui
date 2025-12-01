package handlers

import (
	"errors"
	"net/http"
	"vpnpanel/internal/db"
	"vpnpanel/internal/handlers/response"
	"vpnpanel/internal/models"
	"vpnpanel/internal/repository"

	"github.com/gin-gonic/gin"
)

type UserController struct {
	users *repository.UserRepo
}

func NewUserController(users *repository.UserRepo) *UserController {
	return &UserController{users: users}
}

func (s *UserController) Register(r *gin.RouterGroup) {
	r.GET("/all", s.GetAllUsers)
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
	// 	log.Printf("Failed to read response.Response body: %v\n", err)
	// }
	// // req url
	// log.Println("Request URL:", c.Request.URL.String())

	// // req header X-API-KEY
	// log.Println("Request Header X-API-KEY:", c.Request.Header.Get("X-API-KEY"))

	// // log.Println("response.Response status code:", c.Request.StatusCode)
	// // response.Response body as string
	// log.Printf("response.Response body: %s\n", string(body))
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
	user.Status = userStatus.Status
	db.DB.Save(&user)
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
