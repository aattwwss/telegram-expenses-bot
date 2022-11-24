package main

import (
	"context"
	"github.com/aattwwss/telegram-expense-bot/config"
	"github.com/aattwwss/telegram-expense-bot/dao"
	"github.com/aattwwss/telegram-expense-bot/db"
	"github.com/aattwwss/telegram-expense-bot/handler"
	"github.com/caarlos0/env/v6"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/joho/godotenv"
	"github.com/rs/zerolog/log"
	"os"
)

func handleCallback(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update, commandHandler *handler.CallbackHandler) {
	editMsgConfig := tgbotapi.EditMessageReplyMarkupConfig{
		BaseEdit: tgbotapi.BaseEdit{
			ChatID:      update.CallbackQuery.Message.Chat.ID,
			MessageID:   update.CallbackQuery.Message.MessageID,
			ReplyMarkup: nil,
		},
	}
	if _, err := bot.Request(editMsgConfig); err != nil {
		log.Error().Msgf("handleMessage error: %v", err)
	}

	msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, update.CallbackQuery.Data)

	//And finally, send a message containing the data received.
	if _, err := bot.Send(msg); err != nil {
		log.Error().Msgf("handleMessage error: %v", err)
	}
}
func handleMessage(ctx context.Context, bot *tgbotapi.BotAPI, update tgbotapi.Update, commandHandler *handler.CommandHandler) {
	log.Info().Msgf("Received: %v", update.Message.Text)

	msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
	if update.Message.IsCommand() { // ignore any non-command Messages
		// Create a new MessageConfig. We don't have text yet,
		// so we leave it empty.
		// Extract the command from the Message.
		switch update.Message.Command() {
		case "start":
			commandHandler.Start(ctx, &msg, update)
		case "help":
			commandHandler.Help(ctx, &msg, update)
		default:
			commandHandler.Help(ctx, &msg, update)
		}
	} else {
		commandHandler.Transact(ctx, &msg, update)
	}
	// Send the message.
	if _, err := bot.Send(msg); err != nil {
		log.Error().Msgf("handleMessage error: %v", err)
	}
}

func loadEnv() error {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" || appEnv == "dev" {
		err := godotenv.Load(".env.local")
		if err != nil {
			return err
		}
	}

	err := godotenv.Load()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	ctx := context.Background()
	//zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	err := loadEnv()
	if err != nil {
		log.Fatal().Msg("Error loading .env files")
	}
	cfg := config.EnvConfig{}
	if err := env.Parse(&cfg); err != nil {
		log.Fatal().Err(err)
	}
	dbLoaded, _ := db.LoadDB(ctx, cfg)

	userDAO := dao.NewUserDao(dbLoaded)
	transactionDAO := dao.NewTransactionDAO(dbLoaded)
	commandHandler := handler.NewCommandHandler(userDAO, transactionDAO)
	callbackHandler := handler.NewCallbackHandler(userDAO, transactionDAO)

	bot, err := tgbotapi.NewBotAPI(cfg.TelegramApiToken)
	if err != nil {
		log.Fatal().Err(err)
	}

	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := bot.GetUpdatesChan(u)

	for i := 0; i < 2; i++ {
		go func(bot *tgbotapi.BotAPI, update <-chan tgbotapi.Update) {
			for update := range updates {
				if update.Message != nil {
					handleMessage(ctx, bot, update, &commandHandler)
				} else if update.CallbackQuery != nil {
					handleCallback(ctx, bot, update, &callbackHandler)
				}
			}
		}(bot, updates)
	}
	select {}
}
