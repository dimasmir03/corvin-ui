package telegrambot

func (b *Bot) isAdmin(tgID int64) bool {
	if b == nil || len(b.adminIDs) == 0 {
		return false
	}

	for _, adminID := range b.adminIDs {
		if adminID == tgID {
			return true
		}
	}
	return false
}
