package utils

import "fmt"

func FormatUserList(users []map[string]interface{}) string {
	result := "Список пользователей:\n"
	for _, user := range users {
		result += fmt.Sprintf("- %s (Лимит: %v)\n", user["name"], user["limit"])
	}
	return result
}
