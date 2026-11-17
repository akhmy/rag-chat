package telegram

//func (h *handler) handleStart(ctx context.Context, sdk *bot.Bot, upd *models.Update) {
//	if _, err := sdk.SendMessage(ctx, &bot.SendMessageParams{
//		ChatID: upd.Message.Chat.ID,
//		Text:   "Choose an action:",
//		ReplyMarkup: models.ReplyKeyboardMarkup{
//			Keyboard: [][]models.KeyboardButton{
//				{
//					{Text: "/sessions"},
//				},
//			},
//			ResizeKeyboard: true,
//		},
//	}); err != nil {
//		replyError(ctx, sdk, upd.Message.Chat.ID, "failed to send message")
//	}
//}
