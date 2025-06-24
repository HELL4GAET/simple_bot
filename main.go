package main

import (
	"fmt"
	"github.com/joho/godotenv"
	"log"
	"os"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

const adminID = 6979520320

var (
	bot         *tgbotapi.BotAPI
	admins      = map[int64]bool{adminID: true}
	users       = make(map[int64]bool)
	history     = make(map[int64][]Message)
	replyTarget = make(map[int64]int64)
	adminState  = make(map[int64]string)
)

type Message struct {
	Text      string
	FileID    string
	MediaType string
	FromAdmin bool
	Timestamp int64
}

func init() {
	// попытка загрузить .env, но ошибки не фатальны
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using environment variables")
	}
}

func main() {

	token := os.Getenv("TELEGRAM_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_TOKEN is not set")
	}

	var err error
	bot, err = tgbotapi.NewBotAPI(token)
	if err != nil {
		panic(err)
	}

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates := bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			handleCallback(update.CallbackQuery)
			continue
		}

		if update.Message == nil {
			continue
		}

		fromID := update.Message.From.ID
		chatID := update.Message.Chat.ID

		if update.Message.IsCommand() {
			switch update.Message.Command() {
			case "start":
				if admins[fromID] {
					sendAdminMenu(fromID)
				} else {
					bot.Send(tgbotapi.NewMessage(chatID, "Здравствуйте! Напишите нам, и мы ответим Вам в ближайшее время!"))
					users[fromID] = true
				}
				continue
			}
		}

		if admins[fromID] {
			handleAdminMessage(fromID, chatID, update.Message)
		} else {
			handleUserMessage(fromID, chatID, update.Message)
		}
	}
}

func sendAdminMenu(adminID int64) {
	adminsBtn := tgbotapi.NewInlineKeyboardButtonData("👥 Список Администраторов", "list_admins")
	addAdminBtn := tgbotapi.NewInlineKeyboardButtonData("➕ Добавить Администратора", "add_admin")
	removeAdminBtn := tgbotapi.NewInlineKeyboardButtonData("➖ Удалить Администратора", "remove_admin")
	usersBtn := tgbotapi.NewInlineKeyboardButtonData("👤 Список пользователей", "list_users")

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(adminsBtn),
		tgbotapi.NewInlineKeyboardRow(addAdminBtn),
		tgbotapi.NewInlineKeyboardRow(removeAdminBtn),
		tgbotapi.NewInlineKeyboardRow(usersBtn),
	)

	msg := tgbotapi.NewMessage(adminID, "⚙️ Меню администратора:")
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func handleCallback(cbq *tgbotapi.CallbackQuery) {
	if !admins[cbq.From.ID] {
		return
	}

	parts := strings.Split(cbq.Data, "|")
	action := parts[0]
	param := ""
	if len(parts) > 1 {
		param = parts[1]
	}

	switch action {
	case "reply":
		handleReplyAction(cbq, param)
	case "history":
		handleHistoryAction(cbq, param)
	case "list_admins":
		handleListAdmins(cbq)
	case "add_admin":
		handleAddAdmin(cbq)
	case "remove_admin":
		handleRemoveAdmin(cbq)
	case "list_users":
		handleListUsers(cbq)
	case "confirm_add_admin":
		handleConfirmAddAdmin(cbq, param)
	case "confirm_remove_admin":
		handleConfirmRemoveAdmin(cbq, param)
	case "user_details":
		handleUserDetails(cbq, param)
	default:
		bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
	}
}

func handleListAdmins(cbq *tgbotapi.CallbackQuery) {
	var adminsList strings.Builder
	adminsList.WriteString("👥 Список администраторов:\n\n")

	for adminID := range admins {
		adminsList.WriteString(fmt.Sprintf("• %d\n", adminID))
	}

	msg := tgbotapi.NewMessage(cbq.From.ID, adminsList.String())
	bot.Send(msg)
	bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
}

func handleAddAdmin(cbq *tgbotapi.CallbackQuery) {
	msg := tgbotapi.NewMessage(cbq.From.ID, "Введите ID пользователя для добавления в администраторы:")
	bot.Send(msg)
	adminState[cbq.From.ID] = "waiting_admin_id_to_add"
	bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
}

func handleRemoveAdmin(cbq *tgbotapi.CallbackQuery) {
	var buttons [][]tgbotapi.InlineKeyboardButton

	for adminID := range admins {
		if adminID == cbq.From.ID {
			continue // Нельзя удалить себя
		}
		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("Удалить %d", adminID),
			fmt.Sprintf("confirm_remove_admin|%d", adminID),
		)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(btn))
	}

	if len(buttons) == 0 {
		msg := tgbotapi.NewMessage(cbq.From.ID, "Нет других администраторов для удаления")
		bot.Send(msg)
	} else {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(cbq.From.ID, "Выберите администратора для удаления:")
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
	}

	bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
}

func handleListUsers(cbq *tgbotapi.CallbackQuery) {
	var buttons [][]tgbotapi.InlineKeyboardButton

	for userID := range users {
		btn := tgbotapi.NewInlineKeyboardButtonData(
			fmt.Sprintf("Пользователь %d", userID),
			fmt.Sprintf("user_details|%d", userID),
		)
		buttons = append(buttons, tgbotapi.NewInlineKeyboardRow(btn))
	}

	if len(buttons) == 0 {
		msg := tgbotapi.NewMessage(cbq.From.ID, "Нет активных пользователей")
		bot.Send(msg)
	} else {
		keyboard := tgbotapi.NewInlineKeyboardMarkup(buttons...)
		msg := tgbotapi.NewMessage(cbq.From.ID, "Список пользователей:")
		msg.ReplyMarkup = keyboard
		bot.Send(msg)
	}

	bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
}

func handleUserDetails(cbq *tgbotapi.CallbackQuery, userID string) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return
	}

	replyBtn := tgbotapi.NewInlineKeyboardButtonData("✉ Ответить", fmt.Sprintf("reply|%d", uid))
	historyBtn := tgbotapi.NewInlineKeyboardButtonData("📜 История", fmt.Sprintf("history|%d", uid))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(replyBtn, historyBtn),
	)

	msg := tgbotapi.NewMessage(cbq.From.ID, fmt.Sprintf("Пользователь: %d", uid))
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
	bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
}

func handleConfirmAddAdmin(cbq *tgbotapi.CallbackQuery, userID string) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return
	}

	admins[uid] = true
	msg := tgbotapi.NewMessage(cbq.From.ID, fmt.Sprintf("Пользователь %d добавлен как администратор", uid))
	bot.Send(msg)
	bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
}

func handleConfirmRemoveAdmin(cbq *tgbotapi.CallbackQuery, userID string) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return
	}

	delete(admins, uid)
	msg := tgbotapi.NewMessage(cbq.From.ID, fmt.Sprintf("Пользователь %d удален из администраторов", uid))
	bot.Send(msg)
	bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
}

func handleReplyAction(cbq *tgbotapi.CallbackQuery, userID string) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return
	}

	replyTarget[cbq.From.ID] = uid
	msg := tgbotapi.NewMessage(cbq.From.ID, fmt.Sprintf("Режим ответа пользователю %d. Напишите сообщение:", uid))
	bot.Send(msg)
	bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
}

func handleHistoryAction(cbq *tgbotapi.CallbackQuery, userID string) {
	uid, err := strconv.ParseInt(userID, 10, 64)
	if err != nil {
		return
	}

	var historyText strings.Builder
	historyText.WriteString(fmt.Sprintf("📜 История переписки с пользователем %d:\n\n", uid))

	for _, msg := range history[uid] {
		if msg.FromAdmin {
			historyText.WriteString("👨‍💻 Админ: ")
		} else {
			historyText.WriteString("👤 Пользователь: ")
		}

		if msg.Text != "" {
			historyText.WriteString(msg.Text + "\n")
		} else {
			historyText.WriteString(fmt.Sprintf("[%s]\n", msg.MediaType))
		}
	}

	if len(history[uid]) == 0 {
		historyText.WriteString("История пуста")
	}

	msg := tgbotapi.NewMessage(cbq.From.ID, historyText.String())
	bot.Send(msg)
	bot.Request(tgbotapi.NewCallback(cbq.ID, ""))
}

func handleAdminMessage(adminID, chatID int64, msg *tgbotapi.Message) {
	if state, ok := adminState[adminID]; ok {
		switch state {
		case "waiting_admin_id_to_add":
			uid, err := strconv.ParseInt(msg.Text, 10, 64)
			if err != nil {
				bot.Send(tgbotapi.NewMessage(chatID, "Неверный формат ID. Попробуйте еще раз."))
				return
			}

			confirmBtn := tgbotapi.NewInlineKeyboardButtonData("✅ Подтвердить", fmt.Sprintf("confirm_add_admin|%d", uid))
			cancelBtn := tgbotapi.NewInlineKeyboardButtonData("❌ Отмена", "cancel")

			keyboard := tgbotapi.NewInlineKeyboardMarkup(
				tgbotapi.NewInlineKeyboardRow(confirmBtn, cancelBtn),
			)

			confirmMsg := tgbotapi.NewMessage(chatID, fmt.Sprintf("Добавить пользователя %d как администратора?", uid))
			confirmMsg.ReplyMarkup = keyboard
			bot.Send(confirmMsg)
			delete(adminState, adminID)
		}
		return
	}

	if replyTarget[adminID] != 0 {
		handleAdminReply(adminID, msg)
		return
	}

	sendAdminMenu(adminID)
}

func handleAdminReply(adminID int64, msg *tgbotapi.Message) {
	userID := replyTarget[adminID]
	replyTarget[adminID] = 0

	message := Message{
		Text:      msg.Text,
		FromAdmin: true,
	}

	if msg.Photo != nil {
		photo := msg.Photo[len(msg.Photo)-1]
		message.FileID = photo.FileID
		message.MediaType = "photo"
	} else if msg.Audio != nil {
		message.FileID = msg.Audio.FileID
		message.MediaType = "audio"
	} else if msg.Document != nil {
		message.FileID = msg.Document.FileID
		message.MediaType = "document"
	} else if msg.Voice != nil {
		message.FileID = msg.Voice.FileID
		message.MediaType = "voice"
	}

	history[userID] = append(history[userID], message)
	sendToUser(userID, message)
	bot.Send(tgbotapi.NewMessage(adminID, "Ответ отправлен"))
	sendAdminMenu(adminID)
}

func handleUserMessage(userID, chatID int64, msg *tgbotapi.Message) {
	users[userID] = true

	message := Message{}
	if msg.Text != "" {
		message.Text = msg.Text
	} else if msg.Photo != nil {
		photo := msg.Photo[len(msg.Photo)-1]
		message.FileID = photo.FileID
		message.MediaType = "photo"
	} else if msg.Audio != nil {
		message.FileID = msg.Audio.FileID
		message.MediaType = "audio"
	} else if msg.Document != nil {
		message.FileID = msg.Document.FileID
		message.MediaType = "document"
	} else if msg.Voice != nil {
		message.FileID = msg.Voice.FileID
		message.MediaType = "voice"
	}

	history[userID] = append(history[userID], message)

	for admin := range admins {
		forwardToAdmin(admin, chatID, msg)
		sendAdminControls(admin, userID)
	}

	if msg.Text != "" {
		bot.Send(tgbotapi.NewMessage(chatID, "Ваше сообщение получено. Спасибо!"))
	}
}

func forwardToAdmin(adminID int64, chatID int64, msg *tgbotapi.Message) {
	fwd := tgbotapi.NewForward(adminID, chatID, msg.MessageID)
	bot.Send(fwd)
}

func sendAdminControls(adminID, userID int64) {
	replyBtn := tgbotapi.NewInlineKeyboardButtonData("✉ Ответить", fmt.Sprintf("reply|%d", userID))
	historyBtn := tgbotapi.NewInlineKeyboardButtonData("📜 История", fmt.Sprintf("history|%d", userID))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(replyBtn, historyBtn),
	)

	msg := tgbotapi.NewMessage(adminID, fmt.Sprintf("Пользователь %d написал:", userID))
	msg.ReplyMarkup = keyboard
	bot.Send(msg)
}

func sendToUser(userID int64, msg Message) {
	// Если сообщение текстовое
	if msg.Text != "" {
		// Отправляем текстовое сообщение
		bot.Send(tgbotapi.NewMessage(userID, msg.Text))
		return
	}

	// Если сообщение содержит фото
	if msg.MediaType == "photo" {
		// Отправляем фото
		bot.Send(tgbotapi.NewPhoto(userID, tgbotapi.FileID(msg.FileID)))
		return
	}

	// Если сообщение содержит аудио
	if msg.MediaType == "audio" {
		// Отправляем аудио
		bot.Send(tgbotapi.NewAudio(userID, tgbotapi.FileID(msg.FileID)))
		return
	}

	// Если сообщение содержит документ
	if msg.MediaType == "document" {
		// Отправляем документ
		bot.Send(tgbotapi.NewDocument(userID, tgbotapi.FileID(msg.FileID)))
		return
	}

	// Если сообщение содержит голосовое сообщение
	if msg.MediaType == "voice" {
		// Отправляем голосовое сообщение
		bot.Send(tgbotapi.NewVoice(userID, tgbotapi.FileID(msg.FileID)))
		return
	}

	// Если неизвестный тип медиа
	fmt.Println("Неизвестный тип медиа:", msg.MediaType)
}
