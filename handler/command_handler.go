package handler

import (
	"context"
	"fmt"
	"github.com/Rhymond/go-money"
	"github.com/aattwwss/telegram-expense-bot/domain"
	"github.com/aattwwss/telegram-expense-bot/message"
	"github.com/aattwwss/telegram-expense-bot/repo"
	"github.com/aattwwss/telegram-expense-bot/util"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/rs/zerolog/log"
	"math"
	"strconv"
	"time"
)

const (
	userExistsMsg            = "Welcome back! These are the summary of your transactions: \n"
	errorFindingUserMsg      = "Sorry there is a problem fetching your information.\n"
	errorCreatingUserMsg     = "Sorry there is a problem signing you up.\n"
	signUpSuccessMsg         = "Congratulations! We can get you started right away!\n"
	helpMsg                  = "Type /start to register.\nType <category>, <price>, [date]\n"
	cannotRecogniseAmountMsg = "I don't recognise that amount of money :(\n"

	transactionHeaderHTMLMsg = "<b>Summary\n</b>"

	transactionTypeInlineColSize = 2
)

type CommandHandler struct {
	transactionRepo     repo.TransactionRepo
	transactionTypeRepo repo.TransactionTypeRepo
	categoryRepo        repo.CategoryRepo
	statRepo            repo.StatRepo
	userRepo            repo.UserRepo
}

func NewCommandHandler(userRepo repo.UserRepo, transactionRepo repo.TransactionRepo, transactionTypeRepo repo.TransactionTypeRepo, categoryRepo repo.CategoryRepo, statRepo repo.StatRepo) CommandHandler {
	return CommandHandler{
		userRepo:            userRepo,
		transactionRepo:     transactionRepo,
		transactionTypeRepo: transactionTypeRepo,
		categoryRepo:        categoryRepo,
		statRepo:            statRepo,
	}
}

func (handler CommandHandler) Start(ctx context.Context, msg *tgbotapi.MessageConfig, update tgbotapi.Update) {
	msg.Text = "Welcome to your expense tracker!\n"

	teleUser := update.SentFrom()

	dbUser, err := handler.userRepo.FindUserById(ctx, teleUser.ID)
	if err != nil {
		log.Error().Msgf("error finding user: %v", err)
		msg.Text += errorFindingUserMsg
		return
	}

	if dbUser != nil {
		log.Info().Msgf("User already exists. id: %v", dbUser.Id)
		msg.Text += userExistsMsg
		return
	}

	loc, _ := time.LoadLocation("Asia/Singapore")

	user := domain.User{
		Id:       teleUser.ID,
		Locale:   "en",
		Currency: money.GetCurrency("SGD"),
		Location: loc,
	}

	err = handler.userRepo.Add(ctx, user)
	if err != nil {
		log.Error().Msgf("error adding user: %v", err)
		msg.Text += errorCreatingUserMsg
		return
	}
	msg.Text += signUpSuccessMsg
	return
}

func (handler CommandHandler) Help(ctx context.Context, msg *tgbotapi.MessageConfig, update tgbotapi.Update) {
	msg.Text = helpMsg
	return
}

func (handler CommandHandler) Transact(ctx context.Context, msg *tgbotapi.MessageConfig, update tgbotapi.Update) {
	float, err := strconv.ParseFloat(update.Message.Text, 64)
	if err != nil {
		msg.Text = cannotRecogniseAmountMsg
		return
	}

	amount := money.NewFromFloat(float, money.SGD)
	msg.Text = fmt.Sprintf(message.TransactionTypeReplyMsg+"%v", amount.AsMajorUnits())

	transactionTypes, err := handler.transactionTypeRepo.GetAll(ctx)
	if err != nil {
		msg.Text = message.GenericErrReplyMsg
		return
	}

	msg.ReplyMarkup = tgbotapi.InlineKeyboardMarkup{InlineKeyboard: newTransactionTypesKeyboard(transactionTypes, transactionTypeInlineColSize)}

}

func (handler CommandHandler) Stat(ctx context.Context, msg *tgbotapi.MessageConfig, update tgbotapi.Update) {
	userId := update.SentFrom().ID
	user, err := handler.userRepo.FindUserById(ctx, userId)
	if err != nil {
		log.Error().Msgf("Error finding user for stats: %v", err)
		msg.Text = message.GenericErrReplyMsg
		return
	}
	if user == nil {
		log.Error().Msgf("User not found for stats: %v", userId)
		msg.Text = message.GenericErrReplyMsg
		return
	}

	param := repo.GetMonthlySearchParam{
		Location:           *user.Location,
		PrevMonthIntervals: 3,
		UserId:             userId,
	}

	summaries, err := handler.statRepo.GetMonthly(ctx, param)
	if err != nil {
		log.Error().Msgf("%v", err)
		msg.Text = message.GenericErrReplyMsg
		return
	}

	msg.Text = transactionHeaderHTMLMsg
	msg.Text += summaries.GenerateReportText()

	msg.ParseMode = tgbotapi.ModeHTML
	return
}

func newTransactionTypesKeyboard(transactionTypes []domain.TransactionType, colSize int) [][]tgbotapi.InlineKeyboardButton {
	var configs []util.InlineKeyboardConfig
	for _, transactionTye := range transactionTypes {
		config := util.NewInlineKeyboardConfig(transactionTye.Name, util.CallbackDataSerialize(transactionTye, transactionTye.Id))
		configs = append(configs, config)
	}

	return util.NewInlineKeyboard(configs, colSize)
}

func roundUpDivision(dividend int, divisor int) int {
	quotient := float64(dividend) / float64(divisor)
	quotientCeiling := math.Ceil(quotient)
	return int(quotientCeiling)
}
