package utils

import (
	"strconv"

	"gopkg.in/telebot.v4"
)

func GetEmail(user *telebot.User) string {
	return user.FirstName + user.LastName + strconv.FormatInt(user.ID, 10)
}
