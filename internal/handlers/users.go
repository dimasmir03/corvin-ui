package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"vpnpanel/internal/db"
	"vpnpanel/internal/models"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
)

type UserController struct{}

func NewUserController(r *gin.RouterGroup) *UserController {
	userController := &UserController{}
	userController.Routes(r)
	return userController
}

func (s *UserController) Routes(r *gin.RouterGroup) {
	r.GET("/users", s.GetAllUsers)
	r.POST("/users/new", s.CreateUser)
	r.POST("/users/edit/:id", s.UpdateUser)
	r.POST("/users/delete/:id", s.DeleteUser)
}

// GetAllUsers retrieves all users stored in the database and returns them as a JSON object.
// The response will contain a single key 'users' with a value of an array of user objects.
func (s *UserController) GetAllUsers(c *gin.Context) {
	var users []models.User
	db.DB.Find(&users)
	c.JSON(http.StatusOK, gin.H{"users": users})
}

// CreateUser creates a new user in the database, hashing the provided password and associating the user with the provided server IDs.
// The user information is passed as a JSON object in the request body.
// The server IDs are passed as a JSON array in the 'servers' key of the request context.
// The function returns a JSON object with the error message if an error occurs during user creation or server association.
func (s *UserController) CreateUser(c *gin.Context) {
	var user models.User
	if err := c.Bind(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	user.Password = string(hash)

	if err := db.DB.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	servers := c.MustGet("servers").([]string)
	db.DB.Where("user_id = ?", user.ID).Delete(&models.UserServer{})
	for _, sid := range servers {
		id, err := strconv.Atoi(sid)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		db.DB.Create(&models.UserServer{UserID: user.ID, ServerID: uint(id)})
	}

	c.Redirect(http.StatusSeeOther, "/users")
}

// UpdateUser updates an existing user by ID, changing their username and password as well as their server associations.
// The ID of the user to be updated is passed as a URL parameter.
// The updated user information is passed as a JSON object in the request body.
func (s *UserController) UpdateUser(c *gin.Context) {
	id, exists := c.Get("id")
	if !exists {
		c.Error(errors.New("id is required"))
		return
	}

	var user models.User
	db.DB.First(&user, id)

	if err := c.Bind(&user); err != nil {
		c.Error(err)
		return
	}

	if user.Password != "" {
		hash, err := bcrypt.GenerateFromPassword([]byte(user.Password), bcrypt.DefaultCost)
		if err != nil {
			c.Error(err)
			return
		}
		user.Password = string(hash)
	}

	db.DB.Save(&user)
	serverIDs := c.Request.Form["servers"] // массив выбранных ID

	db.DB.Where("user_id = ?", user.ID).Delete(&models.UserServer{})
	for _, sid := range serverIDs {
		id, err := strconv.Atoi(sid)
		if err != nil {
			c.Error(err)
			return
		}
		db.DB.Create(&models.UserServer{UserID: user.ID, ServerID: uint(id)})
	}

	c.Redirect(http.StatusSeeOther, "/users")
}

// DeleteUser deletes a user by ID and redirects to the user list page.
// The ID of the user to be deleted is passed as a URL parameter.
func (s *UserController) DeleteUser(c *gin.Context) {
	id, exists := c.Get("id")
	if !exists {
		c.Error(errors.New("id is required"))
		return
	}
	db.DB.Delete(&models.User{}, id)
	c.Redirect(http.StatusSeeOther, "/users")
}
