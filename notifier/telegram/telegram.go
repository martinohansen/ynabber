package telegram

import (
	"fmt"
	"time"

	"github.com/martinohansen/ynabber"
	tb "gopkg.in/tucnak/telebot.v2"
)

func Notify(user string, message string) error {
	bot, err := tb.NewBot(tb.Settings{
		Token:  ynabber.ConfigLookup("TELEGRAM_TOKEN", ""),
		Poller: &tb.LongPoller{Timeout: 10 * time.Second},
	})
	if err != nil {
		return fmt.Errorf("failed to create bot: %s", err)
	}

	_, err = bot.Send(&tb.User{Username: user}, message)
	if err != nil {
		return fmt.Errorf("failed to send message: %s", err)
	}
	return nil
}
