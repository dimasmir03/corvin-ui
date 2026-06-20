package telegrambot

import (
	"embed"
	"fmt"
	"path/filepath"

	telebot "gopkg.in/telebot.v4"
)

//go:embed assets/instruction/*
var instructionAssets embed.FS

func (b *Bot) handleInstruction(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender == nil {
		b.logger.Error("instruction opened failed", nil, "reason", "sender is nil")
		return b.send(c, msgVPNFetchFailed)
	}

	b.logger.Info("instruction opened", "tg_id", sender.ID)
	return b.sendInstructionStep(c, sender.ID, 0)
}

func (b *Bot) handleInstructionNext(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		b.logger.Error("instruction next failed", nil, "reason", "sender is nil")
		return nil
	}

	step := b.state.GetInstructionStep(sender.ID)
	if step >= len(instructionSteps)-1 {
		return b.respond(c, &telebot.CallbackResponse{Text: msgInstructionLast})
	}
	b.respondToCallback(c)
	if err := b.sendInstructionStep(c, sender.ID, step+1); err != nil {
		b.logger.Error("instruction next failed", err, "tg_id", sender.ID, "step", step)
		return err
	}
	return nil
}

func (b *Bot) handleInstructionPrev(c telebot.Context) error {
	sender := c.Sender()
	if sender == nil {
		b.logger.Error("instruction prev failed", nil, "reason", "sender is nil")
		return nil
	}

	step := b.state.GetInstructionStep(sender.ID)
	if step <= 0 {
		return b.respond(c, &telebot.CallbackResponse{Text: msgInstructionFirst})
	}
	b.respondToCallback(c)
	if err := b.sendInstructionStep(c, sender.ID, step-1); err != nil {
		b.logger.Error("instruction prev failed", err, "tg_id", sender.ID, "step", step)
		return err
	}
	return nil
}

func (b *Bot) handleInstructionMenu(c telebot.Context) error {
	b.respondToCallback(c)

	sender := c.Sender()
	if sender != nil {
		b.state.ClearInstruction(sender.ID)
	}
	return b.send(c, msgStartMenu, startMenu())
}

func (b *Bot) sendInstructionStep(c telebot.Context, tgID int64, step int) error {
	if len(instructionSteps) == 0 {
		return b.send(c, msgInstructionComingSoon)
	}
	if step < 0 {
		step = 0
	}
	if step >= len(instructionSteps) {
		step = len(instructionSteps) - 1
	}

	b.state.SetInstructionStep(tgID, step)
	current := instructionSteps[step]
	menu := instructionMenu(step)

	if current.ImagePath == "" {
		return b.send(c, current.Text, telebot.ModeHTML, menu)
	}

	file, err := instructionAssets.Open(instructionAssetPath(current.ImagePath))
	if err != nil {
		b.logger.Error("instruction image send failed", err, "tg_id", tgID, "step", step)
		return b.send(c, current.Text, telebot.ModeHTML, menu)
	}
	defer file.Close()

	photo := &telebot.Photo{
		File:    telebot.FromReader(file),
		Caption: current.Text,
	}

	if err := b.send(c, photo, telebot.ModeHTML, menu); err != nil {
		b.logger.Error("instruction image send failed", err, "tg_id", tgID, "step", step)
		return b.send(c, current.Text, telebot.ModeHTML, menu)
	}
	return nil
}

func instructionAssetPath(filename string) string {
	return filepath.ToSlash(fmt.Sprintf("assets/instruction/%s", filename))
}
