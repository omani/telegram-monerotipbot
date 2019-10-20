package monerotipbot

import (
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/gabstv/httpdigest"
	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"github.com/omani/go-xmrto-client"
	zmq "github.com/pebbe/zmq4"
	"github.com/spf13/viper"

	statsd "github.com/smira/go-statsd"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

// MoneroTipBot is out Monero Tip Bot
type MoneroTipBot struct {
	walletrpc    wallet.Client
	bot          *tgbotapi.BotAPI
	message      *tgbotapi.Message
	callback     *tgbotapi.CallbackQuery
	from         *tgbotapi.User
	giveaways    []*Giveaway
	qrcodes      []*QRCode
	xmrtoOrders  []*XMRToOrder
	rpcchannel   *zmq.Socket
	statsdclient *statsd.Client
}

var usernameregexp *regexp.Regexp
var mutex sync.RWMutex

// NewBot creates a new monerotipbot instance
func NewBot(conf *Config) (*MoneroTipBot, error) {
	configpath := conf.GetConfig()

	if configpath == "" {
		configpath = filepath.Join(os.Getenv("HOME"), "settings.toml")
	}

	// parse the monerotipbot config file
	viper.SetConfigType("yaml")
	viper.SetConfigName(strings.TrimSuffix(path.Base(configpath), path.Ext(configpath)))
	viper.AddConfigPath(path.Dir(configpath))
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
	})

	err := viper.ReadInConfig()
	if err != nil {
		return nil, err
	}

	bot, err := tgbotapi.NewBotAPI(viper.GetString("telegram_bot_token"))
	if err != nil {
		return nil, err
	}

	if conf.GetDebug() {
		log.Printf("Authorized on account %s", bot.Self.UserName)
		bot.Debug = true
	}

	// this is a regexp matcher for Telegram users
	usernameregexp, err = regexp.Compile("^(?i)[@]?[Aa-zZ]+\\w{4,}")
	if err != nil {
		return nil, err
	}

	// check if giveaway file exists, if not create it and load file
	var giveaways []*Giveaway
	giveawayfilename := viper.GetString("GIVEAWAY_FILE")

	file, err := ioutil.ReadFile(giveawayfilename)
	if err == nil {
		err = json.Unmarshal(file, &giveaways)
		if err != nil {
			return nil, err
		}
	}

	self := &MoneroTipBot{
		bot:       bot,
		giveaways: giveaways,
		// start a wallet client instance with login if login specified in settings
		walletrpc: wallet.New(wallet.Config{
			Address:   viper.GetString("monero_rpc_daemon_url"),
			Transport: httpdigest.New(viper.GetString("monero_rpc_daemon_username"), viper.GetString("monero_rpc_daemon_password")),
		}),
	}

	if viper.GetBool("USE_STATSD") {
		// initiate statsd client
		self.statsdclient = statsd.NewClient(viper.GetString("statsd_address"), statsd.MetricPrefix(viper.GetString("statsd_prefix")), statsd.SendLoopCount(10), statsd.MaxPacketSize(100000))
	}
	// initiate zmq channel for rpc calls to this bot (for now only for broadcasting messages to users)
	responder, _ := zmq.NewSocket(zmq.REP)
	responder.Bind(viper.GetString("rpcchannel_uri"))
	self.rpcchannel = responder

	return self, nil
}

// Run starts the bot in a loop
func (mtb *MoneroTipBot) Run() error {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	updates, err := mtb.bot.GetUpdatesChan(u)
	if err != nil {
		return err
	}

	// track in how many groups this bot is actually used in.
	// in your code editor, search the above comment (whole line) to see what I do with the groupsBotIsIn variable.
	groupsBotIsIn := make(map[int64]*tgbotapi.Chat)
	go groupTrackerLogger(groupsBotIsIn)

	// listen on the ZMQ socket for notifications
	go mtb.listenRPC()
	defer mtb.rpcchannel.Close()

	for update := range updates {
		if update.Message != nil {
			// log the event of the bot joining a group
			if update.Message.NewChatMembers != nil {
				for _, member := range *update.Message.NewChatMembers {
					if member.UserName == viper.GetString("BOT_NAME") && member.IsBot {
						mtb.statsdIncr("bot_group_join.counter", 1)
					}
				}
			}
			// log the event of the bot leaving (being kicked out of) a group
			if update.Message.LeftChatMember != nil {
				if update.Message.LeftChatMember.UserName == viper.GetString("BOT_NAME") && update.Message.LeftChatMember.IsBot {
					mtb.statsdIncr("bot_group_left.counter", 1)
				}
			}

			// track in how many groups this bot is actually used in
			if update.Message.Chat != nil {
				mutex.Lock()
				groupsBotIsIn[update.Message.Chat.ID] = update.Message.Chat
				mutex.Unlock()
			}

			// bots are not allowed to talk to us.
			if update.Message.From.IsBot {
				continue
			}

			// generally assume all update events to be message events
			mtb.message = update.Message
			mtb.from = update.Message.From
		}

		// now filter further: check if this is a message or a callback
		iscallback := false
		if update.CallbackQuery != nil {
			// bots are not allowed to talk to us.
			// notice: we check on the FROM object, not the Message object!
			// on a callback the Message object will always be from the bot!
			if update.CallbackQuery.From.IsBot {
				continue
			}
			iscallback = true
			mtb.callback = update.CallbackQuery
			mtb.from = update.CallbackQuery.From
			// trick: we pass the message referenced in the callback to our message type
			mtb.message = update.CallbackQuery.Message
		}

		if mtb.from == nil {
			mtb.destroy()
			continue
		}

		if !iscallback {
			if mtb.message.IsCommand() {
				// request pre-checks
				err := mtb.requestPreCheck()
				if err != nil {
					mtb.destroy()
					continue
				}

				err = mtb.parseCommand()
				if err != nil {
					mtb.destroy()
					continue
				}
			} else {
				// check if we received a photo (possibly a qr-code)
				if mtb.message.Photo != nil {
					if mtb.message.Caption == "xmr.to" {
						// we received a photo with caption "xmr.to".
						// is the user trying to upload a BTC qr-code to relay to xmr.to? check it here...
						err = mtb.parseXMRToPhoto()
						if err != nil {
							mtb.destroy()
							continue
						}
					} else {
						err = mtb.parsePhoto()
						if err != nil {
							mtb.destroy()
							continue
						}
					}
				}
			}
		}

		// not a command? then handle callbackqueries here
		if mtb.callback != nil {
			// request pre-checks
			err := mtb.requestPreCheck()
			if err != nil {
				mtb.destroy()
				continue
			}

			if strings.HasPrefix(mtb.callback.Data, "giveaway_") {
				err = mtb.processGiveaway()
				if err != nil {
					mtb.destroy()
					continue
				}
			}
			if strings.HasPrefix(mtb.callback.Data, "qrcode_") {
				err = mtb.processQRCode()
				if err != nil {
					mtb.destroy()
					continue
				}
			}
			if strings.HasPrefix(mtb.callback.Data, "xmrto_") {
				err = mtb.processXMRToOrder()
				if err != nil {
					mtb.destroy()
					continue
				}
			}
		}

		/*
			********************************************
			IMPORTANT!!! DESTROY UPDATE REFERENCES HERE!
			********************************************
		*/
		mtb.destroy()
	}

	return nil
}

func (mtb *MoneroTipBot) destroy() {
	mtb.message = nil
	mtb.callback = nil
	mtb.from = nil
	// save the wallet (IMPORTANT!)
	mtb.walletrpc.Store()
	// // destroy ZMQ socket
	// mtb.rpcchannel.Close()
	// // destroy statsdclient
	// mtb.statsdclient.Close()
}

func (mtb *MoneroTipBot) hasUserNameHandle() bool {
	if len(mtb.from.UserName) == 0 {
		reply := &Message{
			Format: true,
			ChatID: mtb.getReplyID(),
		}
		if mtb.message.Chat.IsPrivate() {
			reply.Text = "You have no username set. Please set a username handle in your Telegram settings to be able to interact with this bot. Type in /start again if you are ready."
		} else {
			reply.Text = "You have no username set. Please set a username handle in your Telegram settings to be able to interact with this bot."
		}
		mtb.reply(reply)
		return false
	}

	return true
}

func (mtb *MoneroTipBot) reply(msg *Message) error {
	botmsg := tgbotapi.NewMessage(msg.ChatID, "")

	if msg.Format {
		botmsg.ParseMode = "Markdown"
		botmsg.Text = fmt.Sprintf("```\n%s```", msg.Text)
	} else {
		botmsg.ParseMode = "HTML"
		botmsg.Text = fmt.Sprintf("%s", msg.Text)
	}

	_, err := mtb.bot.Send(botmsg)
	mtb.statsdIncr("botreplymessages.counter", 1)
	return err
}

func (mtb *MoneroTipBot) requestPreCheck() error {
	if !mtb.hasUserNameHandle() {
		return errors.New("No username handle")
	}
	msg := mtb.newReplyMessage(false)

	// get user's wallet account
	useraccount, err := mtb.getUserAccount()
	if err != nil {
		msg.Text = fmt.Sprintf("Error: %s", err)
		mtb.reply(msg)
		return err
	}

	if useraccount == nil {
		err := mtb.createAccountIfNotExists()
		if err != nil {
			return err
		}
	}

	return nil
}

func (mtb *MoneroTipBot) createAccountIfNotExists() error {
	msg := mtb.newReplyMessage(true)

	// get user's wallet account
	useraccount, err := mtb.getUserAccount()
	if err != nil {
		return err
	}
	if useraccount != nil {
		msg.Text = fmt.Sprintf("Account completed. Welcome %s", mtb.getUsername())
		mtb.reply(msg)
		return err
	}

	return mtb.createAccount()
}

func (mtb *MoneroTipBot) createAccount() error {
	resp, err := mtb.walletrpc.CreateAccount(&wallet.RequestCreateAccount{
		Label: fmt.Sprintf("%s@%d", strings.ToLower(mtb.getUsername()), mtb.getUsernameID()),
	})
	if err != nil {
		mtb.reply(&Message{
			Text:   fmt.Sprintf("Error while creating account: %s", err),
			Format: true,
			ChatID: mtb.getUsernameID(),
		})
		return err
	}
	// stat account creation
	mtb.statsdIncr("account_created.counter", 1)

	if mtb.message.Chat.IsPrivate() {
		msg := &Message{
			Format: true,
			ChatID: mtb.getReplyID(),
		}
		msg.Text = fmt.Sprintf("Address has been created for user: %s", mtb.getUsername())
		mtb.reply(msg)
		msg.Text = fmt.Sprintf("Please deposit the amount you wish to your newly created address: %s", resp.Address)
		mtb.reply(msg)
		msg.Text = fmt.Sprintf("This address is dedicated to your user only. Only the user with user id #%d (which is you) can control it. Telegram assigns a new user ID for new accounts. So make sure you withdraw all your funds, should you ever decide to delete your telegram account.", mtb.getUsernameID())
		mtb.reply(msg)
	}

	return nil
}

func (mtb *MoneroTipBot) parseCommand() error {
	// let's do our filtering of group and PM commands in this section.
	// basically, everything is a PM command, except GIVEAWAY and TIP.
	// filter accordingly.
	msg := mtb.newReplyMessage(true)
	msg.Text = "This command is only available in the bot PM. Please don't spam the group."

	switch mtb.message.Command() {
	case COMMANDS[START]:
		if !mtb.message.Chat.IsPrivate() {
			return mtb.reply(msg)
		}
		// stat this command invocation
		mtb.statsdIncr("commands.START.counter", 1)
		return mtb.parseCommandSTART()
	case COMMANDS[HELP]:
		if !mtb.message.Chat.IsPrivate() {
			return mtb.reply(msg)
		}
		// stat this command invocation
		mtb.statsdIncr("commands.HELP.counter", 1)
		return mtb.parseCommandHELP()
	case COMMANDS[TIP]:
		// stat this command invocation
		mtb.statsdIncr("commands.TIP.counter", 1)
		return mtb.parseCommandTIP()
	case COMMANDS[SEND]:
		if !mtb.message.Chat.IsPrivate() {
			return mtb.reply(msg)
		}
		// stat this command invocation
		mtb.statsdIncr("commands.SEND.counter", 1)
		return mtb.parseCommandSEND()
	case COMMANDS[GIVEAWAY]:
		// stat this command invocation
		mtb.statsdIncr("commands.GIVEAWAY.counter", 1)
		return mtb.parseCommandGIVEAWAY()
	case COMMANDS[WITHDRAW]:
		if !mtb.message.Chat.IsPrivate() {
			return mtb.reply(msg)
		}
		// stat this command invocation
		mtb.statsdIncr("commands.WITHDRAW.counter", 1)
		return mtb.parseCommandWITHDRAW()
	case COMMANDS[BALANCE]:
		if !mtb.message.Chat.IsPrivate() {
			return mtb.reply(msg)
		}
		// stat this command invocation
		mtb.statsdIncr("commands.BALANCE.counter", 1)
		return mtb.parseCommandBALANCE()
	case COMMANDS[GENERATEQR]:
		if !mtb.message.Chat.IsPrivate() {
			return mtb.reply(msg)
		}
		// stat this command invocation
		mtb.statsdIncr("commands.GENERATEQR.counter", 1)
		return mtb.parseCommandGENERATEQR()
	case COMMANDS[XMRTO]:
		if !mtb.message.Chat.IsPrivate() {
			return mtb.reply(msg)
		}
		// stat this command invocation
		mtb.statsdIncr("commands.XMRTO.counter", 1)
		return mtb.parseCommandXMRTO()
	}

	return nil
}

func (mtb *MoneroTipBot) getUserAccount() (*Account, error) {
	msg := mtb.newReplyMessage(true)

	accounts, err := mtb.walletrpc.GetAccounts(&wallet.RequestGetAccounts{})
	if err != nil {
		msg.Text = fmt.Sprintf("Error while retrieving accounts: %s", err)
		mtb.reply(msg)
		return nil, err
	}
	// stat this command invocation
	mtb.statsdGauge("user_accounts.counter", int64(len(accounts.SubaddressAccounts)))
	// stat the total balance
	mtb.statsdFGauge("total_balance.counter", wallet.XMRToFloat64(accounts.TotalBalance))
	mtb.statsdFGauge("total_unlocked_balance.counter", wallet.XMRToFloat64(accounts.TotalUnlockedBalance))

	var label string
	// we see user's ID. it means we already know the user.
	label = fmt.Sprintf("%s@", strings.ToLower(mtb.getUsername()))

	for _, address := range accounts.SubaddressAccounts {
		split := strings.Split(address.Label, "@")
		if len(split) == 2 {
			userid, err := strconv.ParseInt(split[1], 10, 64)
			if err == nil {
				// stat labels so we have them stored somewhere else than only in local files
				mtb.statsdGauge(fmt.Sprintf("account_labels_usernames.%s", split[0]), userid)
				mtb.statsdGauge(fmt.Sprintf("account_labels.%d", userid), int64(address.AccountIndex))
				mtb.statsdFGauge(fmt.Sprintf("account_balance_per_label.%d", userid), wallet.XMRToFloat64(address.Balance))
			}
		}

		if strings.Contains(address.Label, label) {
			useraccount := &Account{
				AccountIndex:    address.AccountIndex,
				Balance:         address.Balance,
				BaseAddress:     address.BaseAddress,
				Label:           address.Label,
				Tag:             address.Tag,
				UnlockedBalance: address.UnlockedBalance,
			}

			// label the account of a known user on every request. this is a cheap operation
			if mtb.isKnownUser() {
				mtb.walletrpc.LabelAccount(&wallet.RequestLabelAccount{
					AccountIndex: address.AccountIndex,
					Label:        fmt.Sprintf("%s@%d", strings.ToLower(mtb.getUsername()), mtb.getUsernameID()),
				})
			}

			return useraccount, nil
		}
	}

	return nil, nil
}

func (mtb *MoneroTipBot) getReplyID() int64 {
	if mtb.isKnownUser() {
		return int64(mtb.from.ID)
	}
	return mtb.message.Chat.ID
}

func (mtb *MoneroTipBot) getUsernameID() int64 {
	if mtb.isKnownUser() {
		return int64(mtb.from.ID)
	}
	return int64(mtb.message.From.ID)
}

func (mtb *MoneroTipBot) getUsername() string {
	return mtb.from.UserName
}

func (mtb *MoneroTipBot) isReplyToMessage() bool {
	if mtb.message.ReplyToMessage != nil {
		mtb.statsdIncr("isreplytomessage.counter", 1)
		return true
	}
	return false
}

func (mtb *MoneroTipBot) newReplyMessage(format bool) *Message {
	return &Message{
		Format: format,
		ChatID: mtb.getReplyID(),
	}
}

func (mtb *MoneroTipBot) isKnownUser() bool {
	if mtb.from.ID != 0 {
		return true
	}
	return false
}

func (mtb *MoneroTipBot) processGiveaway() error {
	switch mtb.callback.Data {
	case "giveaway_claim":
		claimer := mtb.callback.From.UserName

		for i, giveaway := range mtb.giveaways {
			if giveaway.Message.MessageID == mtb.message.MessageID {
				if giveaway.From.From.UserName == mtb.callback.From.UserName {
					mtb.bot.AnswerCallbackQuery(tgbotapi.CallbackConfig{
						CallbackQueryID: mtb.callback.ID,
						Text:            "Can't claim your own giveaway.",
					})
					return errors.New("Claimer is giver")
				}

				claimeraccount, err := mtb.getUserAccount()
				if err != nil {
					return err
				}

				var destinations []*wallet.Destination
				destinations = append(destinations, &wallet.Destination{
					Amount:  giveaway.Amount,
					Address: claimeraccount.BaseAddress,
				})
				// stat the transfer time
				start := time.Now()
				resp, err := mtb.walletrpc.Transfer(&wallet.RequestTransfer{
					AccountIndex: giveaway.Sender.AccountIndex,
					Destinations: destinations,
				})
				if err != nil {
					edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), giveaway.Message.MessageID, fmt.Sprintf("User @%s is giving %f XMR away.\n\n...<b>%s</b>", giveaway.From.From.UserName, wallet.XMRToFloat64(giveaway.Amount), err))
					edit.ParseMode = "HTML"
					mtb.bot.Send(edit)
					return err
				}
				mtb.statsdPrecisionTiming("transaction.time_to_complete", time.Since(start))
				// stat the transaction count
				mtb.statsdIncr("transactions.counter", 1)

				mtb.giveaways = append(mtb.giveaways[:i], mtb.giveaways[i+1:]...)
				mtb.saveGiveawayToFile()

				tippermsg := mtb.newReplyMessage(false)
				// replace the chatID with the giver. else we notify the taker.
				tippermsg.ChatID = int64(giveaway.From.From.ID)
				tippermsg.Text = fmt.Sprintf("You successfully tipped user @%s.", strings.TrimPrefix(claimer, "@"))
				tippermsg.Text = fmt.Sprintf("%s\n\nAmount: %s\nFee: %s\nTxHash: <a href='%s%s'>%s</a>", tippermsg.Text, wallet.XMRToDecimal(resp.Amount), wallet.XMRToDecimal(resp.Fee), viper.GetString("blockexplorer_url"), resp.TxHash, resp.TxHash)

				if giveaway.From.From.ID != 0 {
					edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), giveaway.Message.MessageID, fmt.Sprintf("User @%s is giving %f XMR away.\n\n%f XMR given from @%s to @%s.", giveaway.From.From.UserName, wallet.XMRToFloat64(giveaway.Amount), wallet.XMRToFloat64(giveaway.Amount), giveaway.From.From.UserName, claimer))
					mtb.bot.Send(edit)

					msg := mtb.newReplyMessage(false)
					msg.Text = fmt.Sprintf("You have been tipped with %f XMR from user @%s", wallet.XMRToFloat64(giveaway.Amount), giveaway.From.From.UserName)
					err := mtb.reply(msg)
					if err != nil {
						edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), giveaway.Message.MessageID, fmt.Sprintf("User @%s is giving %f XMR away.\n\n%f XMR given from @%s to @%s.\n\n@%s, you have been tipped.\nPlease PM me (@%s) and click the 'Start' button to complete your account.", giveaway.From.From.UserName, wallet.XMRToFloat64(resp.Amount), wallet.XMRToFloat64(resp.Amount), giveaway.From.From.UserName, claimer, claimer, viper.GetString("BOT_NAME")))
						mtb.bot.Send(edit)
						// send notification to giver here
						return mtb.reply(tippermsg)
					}
					// send notification to giver here. with user has been notified
					tippermsg.Text = fmt.Sprintf("%s\n\nUser has been notified.", tippermsg.Text)
					return mtb.reply(tippermsg)
				}

				edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), giveaway.Message.MessageID, fmt.Sprintf("User @%s is giving %f XMR away.\n\n%f XMR given from @%s to @%s.\n\n@%s, you have been tipped.\nPlease PM me (@%s) and click the 'Start' button to complete your account.", giveaway.From.From.UserName, wallet.XMRToFloat64(resp.Amount), wallet.XMRToFloat64(resp.Amount), giveaway.From.From.UserName, claimer, claimer, viper.GetString("BOT_NAME")))
				edit.ParseMode = "HTML"
				mtb.bot.Send(edit)

				// final send because giveaway.From.From.ID was 0.
				return mtb.reply(tippermsg)
			}
		}
	case "giveaway_cancel":
		if mtb.giveaways == nil {
			return nil
		}

		for i, giveaway := range mtb.giveaways {
			if giveaway.Message.MessageID == mtb.message.MessageID {
				if giveaway.From.From.UserName == mtb.callback.From.UserName {
					edit := tgbotapi.NewEditMessageText(int64(mtb.message.Chat.ID), giveaway.Message.MessageID, fmt.Sprintf("User @%s is giving %f XMR away\n\n...<b>Canceled!</b>", giveaway.From.From.UserName, wallet.XMRToFloat64(giveaway.Amount)))
					edit.ParseMode = "HTML"
					mtb.bot.Send(edit)

					mtb.giveaways = append(mtb.giveaways[:i], mtb.giveaways[i+1:]...)
					mtb.saveGiveawayToFile()
					return nil
				}
				mtb.bot.AnswerCallbackQuery(tgbotapi.CallbackConfig{
					CallbackQueryID: mtb.callback.ID,
					Text:            "Not your giveaway.",
				})
				return nil
			}
		}
	default:
		return errors.New("Could not parse CallbackQuery")
	}

	return nil
}

func (mtb *MoneroTipBot) processQRCode() error {
	switch mtb.callback.Data {
	case "qrcode_tx_send":
		if mtb.qrcodes == nil {
			return nil
		}

		msg := mtb.newReplyMessage(true)

		for i, qrcode := range mtb.qrcodes {
			if qrcode.Message.MessageID == mtb.message.MessageID {
				if qrcode.From.From.UserName == mtb.callback.From.UserName {
					useraccount, err := mtb.getUserAccount()

					var destinations []*wallet.Destination

					destinations = append(destinations, &wallet.Destination{
						Amount:  qrcode.Amount,
						Address: qrcode.ParseURI.URI.Address,
					})

					// stat the transfer time
					start := time.Now()
					resp, err := mtb.walletrpc.Transfer(&wallet.RequestTransfer{
						AccountIndex: useraccount.AccountIndex,
						Destinations: destinations,
					})
					if err != nil {
						edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), mtb.callback.Message.MessageID, fmt.Sprintf("%s\n\n...<b>%s! Aborted.</b>", mtb.callback.Message.Text, err))
						edit.ParseMode = "HTML"
						mtb.bot.Send(edit)
						return err
					}
					mtb.statsdPrecisionTiming("transaction.time_to_complete", time.Since(start))
					// stat the transaction count
					mtb.statsdIncr("transactions.counter", 1)

					edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), mtb.callback.Message.MessageID, fmt.Sprintf("%s\n\n...<b>Transaction complete!</b>", mtb.callback.Message.Text))
					edit.ParseMode = "HTML"
					mtb.bot.Send(edit)

					msg.Format = false
					msg.Text = fmt.Sprintf("Amount: %s\nFee: %s\nTxHash: <a href='%s%s'>%s", wallet.XMRToDecimal(resp.Amount), wallet.XMRToDecimal(resp.Fee), viper.GetString("blockexplorer_url"), resp.TxHash, resp.TxHash)
					mtb.reply(msg)

					mtb.qrcodes = append(mtb.qrcodes[:i], mtb.qrcodes[i+1:]...)

					return nil
				}
			}
		}
	case "qrcode_tx_cancel":
		if mtb.qrcodes == nil {
			return nil
		}

		for i, qrcode := range mtb.qrcodes {
			if qrcode.Message.MessageID == mtb.message.MessageID {
				if qrcode.From.From.UserName == mtb.callback.From.UserName {
					edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), mtb.callback.Message.MessageID, fmt.Sprintf("%s\n\n...<b>Canceled!</b>", mtb.callback.Message.Text))
					edit.ParseMode = "HTML"
					mtb.bot.Send(edit)

					mtb.qrcodes = append(mtb.qrcodes[:i], mtb.qrcodes[i+1:]...)
					return nil
				}
			}
		}
	}

	return nil
}

func (mtb *MoneroTipBot) processXMRToOrder() error {
	switch mtb.callback.Data {
	case "xmrto_tx_send":
		for i, xmrtoOrder := range mtb.xmrtoOrders {
			if xmrtoOrder.Message.MessageID == mtb.message.MessageID {
				if xmrtoOrder.From.From.UserName != mtb.callback.From.UserName {
					mtb.bot.AnswerCallbackQuery(tgbotapi.CallbackConfig{
						CallbackQueryID: mtb.callback.ID,
						Text:            "Something went wrong!",
					})
					return errors.New("Something went wrong")
				}

				// initiate a new xmrto client
				client := xmrto.New(&xmrto.Config{Testnet: viper.GetBool("IS_STAGENET_WALLET")})
				orderstatus, err := client.GetOrderStatus(&xmrto.RequestGetOrderStatus{UUID: xmrtoOrder.Order.UUID})
				if err != nil {
					mtb.bot.AnswerCallbackQuery(tgbotapi.CallbackConfig{
						CallbackQueryID: mtb.callback.ID,
						Text:            "Something went wrong!",
					})
					return errors.New("Something went wrong")
				}

				// timeout if less than 60 seconds left before xmrto gives us a timeout. (just in case the send might delay)
				// we rather let the user try it again than let him lose money here and contact xmr.to support email :)
				if orderstatus.SecondsTillTimeout <= 60 {
					edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), xmrtoOrder.Message.MessageID, "")
					edit.Text = fmt.Sprintf("%s\n\n...<b>Timed out!</b>", xmrtoOrder.Message.Text)
					edit.ParseMode = "HTML"
					mtb.bot.Send(edit)
					return nil
				}

				useraccount, err := mtb.getUserAccount()
				if err != nil {
					return err
				}

				var destinations []*wallet.Destination
				destinations = append(destinations, &wallet.Destination{
					Amount:  wallet.Float64ToXMR(xmrtoOrder.Order.XMRAmountTotal),
					Address: xmrtoOrder.Order.XMRReceivingSubAddress,
				})
				// stat the transfer time
				start := time.Now()
				resp, err := mtb.walletrpc.Transfer(&wallet.RequestTransfer{
					AccountIndex: useraccount.AccountIndex,
					Destinations: destinations,
				})
				if err != nil {
					edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), xmrtoOrder.Message.MessageID, "")
					edit.Text = fmt.Sprintf("%s\n\n...<b>%s</b>", xmrtoOrder.Message.Text, err)
					edit.ParseMode = "HTML"
					mtb.bot.Send(edit)
					return err
				}
				mtb.statsdPrecisionTiming("transaction.time_to_complete", time.Since(start))
				// stat the transaction count
				mtb.statsdIncr("transactions.counter", 1)

				mtb.xmrtoOrders = append(mtb.xmrtoOrders[:i], mtb.xmrtoOrders[i+1:]...)

				edit := tgbotapi.NewEditMessageText(int64(mtb.callback.Message.Chat.ID), xmrtoOrder.Message.MessageID, "")
				edit.Text = fmt.Sprintf("%s\n\n...<b>Transaction complete!</b>", xmrtoOrder.Message.Text)
				edit.ParseMode = "HTML"
				mtb.bot.Send(edit)

				msg := mtb.newReplyMessage(false)
				// replace the chatID with the giver. else we notify the taker.
				msg.ChatID = int64(xmrtoOrder.From.From.ID)
				msg.Text = fmt.Sprintf("Amount: %s\nFee: %s\nTxHash: <a href='%s%s'>%s</a>", wallet.XMRToDecimal(resp.Amount), wallet.XMRToDecimal(resp.Fee), viper.GetString("blockexplorer_url"), resp.TxHash, resp.TxHash)
				mtb.reply(msg)

				go monitorXMRToOrder(xmrtoOrder, mtb.bot)
			}
		}
	case "xmrto_tx_cancel":
		if mtb.xmrtoOrders == nil {
			return nil
		}

		for i, xmrtoOrder := range mtb.xmrtoOrders {
			if xmrtoOrder.Message.MessageID == mtb.message.MessageID {
				if xmrtoOrder.From.From.UserName == mtb.callback.From.UserName {
					edit := tgbotapi.NewEditMessageText(int64(mtb.message.Chat.ID), xmrtoOrder.Message.MessageID, "")
					edit.Text = fmt.Sprintf("%s\n\n...<b>Canceled!</b>", xmrtoOrder.Message.Text)
					edit.ParseMode = "HTML"
					mtb.bot.Send(edit)

					mtb.xmrtoOrders = append(mtb.xmrtoOrders[:i], mtb.xmrtoOrders[i+1:]...)
					return nil
				}
				mtb.bot.AnswerCallbackQuery(tgbotapi.CallbackConfig{
					CallbackQueryID: mtb.callback.ID,
					Text:            "Something went wrong!",
				})
				return nil
			}
		}
	default:
		return errors.New("Could not parse CallbackQuery")
	}

	return nil
}

func (mtb *MoneroTipBot) saveGiveawayToFile() error {
	file, err := json.MarshalIndent(mtb.giveaways, "", " ")
	if err != nil {
		return err
	}

	return ioutil.WriteFile(viper.GetString("GIVEAWAY_FILE"), file, 0644)
}

func (mtb *MoneroTipBot) parseXMRToPhoto() error {
	if !mtb.message.Chat.IsPrivate() {
		// do nothing, since this is not in my PM
		return nil
	}

	// received a photo.
	msg := mtb.newReplyMessage(true)

	file, err := mtb.bot.GetFile(tgbotapi.FileConfig{FileID: (*mtb.message.Photo)[len((*mtb.message.Photo))-1].FileID})
	if err != nil {
		msg.Text = "Weird. Couldn't get uploaded image path from telegram servers. This shouldn't happen."
		return mtb.reply(msg)
	}

	res, err := http.Get(file.Link(viper.GetString("telegram_bot_token")))
	if err != nil {
		msg.Text = "Couldn't download uploaded image from telegram servers. Try again later."
		return mtb.reply(msg)
	}

	now := time.Now().Unix()
	//open a file for writing
	savefile, err := os.Create(fmt.Sprintf("/tmp/xmrto_qrcode.jpg_%d", now))
	if err != nil {
		msg.Text = "Could not create image file. Try again later."
		return mtb.reply(msg)
	}
	_, err = io.Copy(savefile, res.Body)
	if err != nil {
		msg.Text = "Could not save image to file. Try again later."
		return mtb.reply(msg)
	}
	savefile.Close()
	defer os.Remove(savefile.Name())

	qrimage, err := os.Open(fmt.Sprintf("/tmp/xmrto_qrcode.jpg_%d", now))
	if err != nil {
		msg.Text = "Could not open image file. Try again later."
		return mtb.reply(msg)
	}

	img, _, err := image.Decode(qrimage)
	if err != nil {
		msg.Text = "Could decode image."
		return mtb.reply(msg)
	}

	// prepare BinaryBitmap
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		msg.Text = "Could not open image file. Try again later."
		return mtb.reply(msg)
	}

	// decode image
	qrReader := qrcode.NewQRCodeReader()
	harder := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
	result, err := qrReader.Decode(bmp, harder)
	if err != nil {
		msg.Text = "Could not detect QR-Code in image."
		return mtb.reply(msg)
	}

	parseuri, err := url.ParseRequestURI(result.String())
	if err != nil {
		msg.Text = "Could not parse URI. Aborting."
		return mtb.reply(msg)
	}

	values := parseuri.Query()
	if len(values.Get("amount")) == 0 {
		msg.Text = "Could not found amount in URI. Can't proceed from here."
		return mtb.reply(msg)
	}

	/*********************************************************************
		 trick:
		 fake a command here to pipe it straight into parseCommandXMRTo()
	*********************************************************************/
	entity := tgbotapi.MessageEntity{
		Length: 6,
		Offset: 0,
		Type:   "bot_command",
	}
	var entities []tgbotapi.MessageEntity
	entities = append(entities, entity)
	mtb.message.Entities = &entities
	mtb.message.Text = fmt.Sprintf("xmr.to %s %s", parseuri.Opaque, values.Get("amount"))
	return mtb.parseCommandXMRTO()
}

func (mtb *MoneroTipBot) parsePhoto() error {
	if !mtb.message.Chat.IsPrivate() {
		// do nothing, since this is not in my PM
		return nil
	}

	// received a photo.
	msg := mtb.newReplyMessage(true)

	file, err := mtb.bot.GetFile(tgbotapi.FileConfig{FileID: (*mtb.message.Photo)[len((*mtb.message.Photo))-1].FileID})
	if err != nil {
		msg.Text = "Weird. Couldn't get uploaded image path from telegram servers. This shouldn't happen."
		return mtb.reply(msg)
	}

	res, err := http.Get(file.Link(viper.GetString("telegram_bot_token")))
	if err != nil {
		msg.Text = "Couldn't download uploaded image from telegram servers. Try again later."
		return mtb.reply(msg)
	}

	now := time.Now().Unix()
	//open a file for writing
	savefile, err := os.Create(fmt.Sprintf("/tmp/qrcode.jpg_%d", now))
	if err != nil {
		msg.Text = "Could not create image file. Try again later."
		return mtb.reply(msg)
	}
	_, err = io.Copy(savefile, res.Body)
	if err != nil {
		msg.Text = "Could not save image to file. Try again later."
		return mtb.reply(msg)
	}
	savefile.Close()
	defer os.Remove(savefile.Name())

	qrimage, err := os.Open(fmt.Sprintf("/tmp/qrcode.jpg_%d", now))
	if err != nil {
		msg.Text = "Could not open image file. Try again later."
		return mtb.reply(msg)
	}

	img, _, err := image.Decode(qrimage)
	if err != nil {
		msg.Text = "Could decode image."
		return mtb.reply(msg)
	}

	// prepare BinaryBitmap
	bmp, err := gozxing.NewBinaryBitmapFromImage(img)
	if err != nil {
		msg.Text = "Could not open image file. Try again later."
		return mtb.reply(msg)
	}

	// decode image
	qrReader := qrcode.NewQRCodeReader()
	harder := map[gozxing.DecodeHintType]interface{}{gozxing.DecodeHintType_TRY_HARDER: true}
	result, err := qrReader.Decode(bmp, harder)
	if err != nil {
		msg.Text = "Could not detect QR-Code in image."
		return mtb.reply(msg)
	}

	parseuri, err := mtb.walletrpc.ParseURI(&wallet.RequestParseURI{URI: result.String()})
	if err == nil {
		if parseuri.URI.Amount == 0 {
			msg.Text = parseuri.URI.Address
			return mtb.reply(msg)
		}
		out := fmt.Sprintf("Address: %s\nAmount: %f XMR\nRecipient: %s\n\nDescription:\n%s", parseuri.URI.Address, wallet.XMRToFloat64(parseuri.URI.Amount), parseuri.URI.RecipientName, parseuri.URI.TxDescription)
		claim := tgbotapi.NewInlineKeyboardButtonData("Send", "qrcode_tx_send")
		cancel := tgbotapi.NewInlineKeyboardButtonData("Cancel", "qrcode_tx_cancel")
		markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(claim, cancel))
		msg := tgbotapi.NewMessage(mtb.message.Chat.ID, "")
		msg.ReplyMarkup = markup
		msg.Text = out
		resp, _ := mtb.bot.Send(msg)
		mtb.statsdIncr("qrcode_parsed.counter", 1)

		qrcode := &QRCode{
			Message:  &resp,
			From:     mtb.message,
			ParseURI: parseuri,
			Amount:   parseuri.URI.Amount,
		}
		mtb.qrcodes = append(mtb.qrcodes, qrcode)
	} else {
		msg.Text = "URI is not RFC 3986 compliant. Checking if string is a valid monero address."
		mtb.reply(msg)
		resp, err := mtb.walletrpc.ValidateAddress(&wallet.RequestValidateAddress{Address: result.String(), AnyNetType: viper.GetBool("IS_STAGENET_WALLET")})
		if err != nil {
			msg.Text = fmt.Sprintf("%s", err)
			return mtb.reply(msg)
		}
		if resp.Valid {
			msg.Text = "Found a valid monero address. Please copy and paste the following message back to me after filling in the right amount."
			mtb.reply(msg)
			msg.Format = false
			msg.Text = fmt.Sprintf("/send %s AMOUNTHERE", result)
			mtb.statsdIncr("qrcode_parsed.counter", 1)
			return mtb.reply(msg)
		}
		msg.Text = "Address is not valid. Aborted."
		mtb.statsdIncr("qrcode_invalid.counter", 1)
		return mtb.reply(msg)
	}

	return nil
}

func (mtb *MoneroTipBot) listenRPC() {
	notification := &Notification{}
	for {
		data, err := mtb.rpcchannel.RecvBytes(0)
		if err != nil {
			data, err := prepareErrorNotification(notification, err)
			if err != nil {
				continue
			}
			mtb.rpcchannel.SendBytes(data, 0)
			continue
		}
		err = json.Unmarshal(data, notification)
		if err != nil {
			data, err := prepareErrorNotification(notification, err)
			if err != nil {
				continue
			}
			mtb.rpcchannel.SendBytes(data, 0)
			continue
		}

		// process the notification message
		msg := &Message{
			Format: false,
			ChatID: notification.UserID,
			Text:   notification.Message,
		}
		err = mtb.reply(msg)
		if err != nil {
			data, err := prepareErrorNotification(notification, err)
			if err != nil {
				continue
			}
			mtb.rpcchannel.SendBytes(data, 0)
			continue
		}

		notification.Sent = true
		data, err = json.Marshal(notification)
		if err != nil {
			data, err := prepareErrorNotification(notification, err)
			if err != nil {
				continue
			}
			mtb.rpcchannel.SendBytes(data, 0)
			continue
		}
		_, err = mtb.rpcchannel.SendBytes(data, 0)
		if err != nil {
			continue
		}
	}
}

func prepareErrorNotification(notification *Notification, err error) ([]byte, error) {
	notification.Sent = false
	notification.Error = err.Error()
	data, err := json.Marshal(notification)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func groupTrackerLogger(tracker map[int64]*tgbotapi.Chat) {
	for {
		file, err := os.OpenFile(viper.GetString("LOGFILE"), os.O_RDWR|os.O_CREATE, 0666)
		if err != nil {
			break
		}
		mutex.Lock()
		for k, v := range tracker {
			file.WriteString(fmt.Sprintf("Group ID: %d - Group Object: %#v\n", k, v))
		}
		mutex.Unlock()
		file.Close()

		time.Sleep(time.Minute * 60)
	}
}

func (mtb *MoneroTipBot) statsdIncr(stat string, count int64, tags ...statsd.Tag) {
	if viper.GetBool("USE_STATSD") {
		mtb.statsdclient.Incr(stat, count, tags...)
	}
}

func (mtb *MoneroTipBot) statsdGauge(stat string, value int64, tags ...statsd.Tag) {
	if viper.GetBool("USE_STATSD") {
		mtb.statsdclient.Gauge(stat, value, tags...)
	}
}

func (mtb *MoneroTipBot) statsdFGauge(stat string, value float64, tags ...statsd.Tag) {
	if viper.GetBool("USE_STATSD") {
		mtb.statsdclient.FGauge(stat, value, tags...)
	}
}

func (mtb *MoneroTipBot) statsdPrecisionTiming(stat string, delta time.Duration, tags ...statsd.Tag) {
	if viper.GetBool("USE_STATSD") {
		mtb.statsdclient.PrecisionTiming(stat, delta, tags...)
	}
}

func monitorXMRToOrder(order *XMRToOrder, bot *tgbotapi.BotAPI) error {
	ticker := time.NewTicker(1 * time.Second)
	msg := tgbotapi.NewMessage(int64(order.From.From.ID), "")
	msg.ParseMode = "HTML"
	msg.Text = "Starting background process to monitor state changes of your XMRTO order.\nYou will receive a notification for each state change.\n\nMost important states:\n<b>UNPAID</b>: waiting for XMR payment from you.\n<b>PAID_UNCONFIRMED</b>: XMR transaction seen in mempool, waiting for confirmations.\n<b>PAID</b>: XMR transaction has enough confirmations.\n<b>BTC_SENT</b>: XMRTO sent the BTC out. From here you wait for the BTC network to confirm your BTC transaction.\n\nMonitoring. Stand by..."
	bot.Send(msg)
	laststate := order.Order.State
	start := time.Now()

	for {
		select {
		case <-ticker.C:
			// fmt.Println("Tick at", t)
			if time.Since(start).Seconds() > 3600 {
				msg.Text = fmt.Sprintf("Didn't receive any order state change for 1 hour. Giving up monitoring.\n\nTry to track your order manually at %s/track by using the secret-key (UUID) <b>%s</b>.\n\nIf you experience any issues please consider contacting the support team of xmr.to!", viper.GetString("xmrto_website"), order.Order.UUID)
				bot.Send(msg)
				return nil
			}

			// initiate a new xmrto client
			client := xmrto.New(&xmrto.Config{Testnet: viper.GetBool("IS_STAGENET_WALLET")})
			orderstatus, err := client.GetOrderStatus(&xmrto.RequestGetOrderStatus{UUID: order.Order.UUID})
			if err != nil {
				msg.Text = fmt.Sprintf("XMRTO API Error: %s", err)
				bot.Send(msg)
				return err
			}

			if orderstatus.State != laststate {
				if orderstatus.State == "BTC_SENT" {
					msg.Text = fmt.Sprintf("(<b>%s</b>):\n\nstate change detected:\nYour order has now state <b>%s</b>\n\nState: <b>%s</b>\nUUID: %s\nBTCAmount: %f\nBTCDestAddress: %s\nBTCTransactionID: %s\nXMRNumConfirmationsRemaining: %d", orderstatus.UUID, orderstatus.State, orderstatus.State, orderstatus.UUID, orderstatus.BTCAmount, orderstatus.BTCDestAddress, orderstatus.BTCTransactionID, orderstatus.XMRNumConfirmationsRemaining)
				} else {
					msg.Text = fmt.Sprintf("(<b>%s</b>)\n\nstate change detected:\nYour order has now state <b>%s</b>", orderstatus.UUID, orderstatus.State)
				}
				bot.Send(msg)
				laststate = orderstatus.State
			}
			if orderstatus.State == "BTC_SENT" {
				// xmrto has sent the btc. we can stop the monitoring process here.
				msg.Text = fmt.Sprintf("(<b>%s</b>)\n\nCongratulations!\n\n%f BTC have been sent to BTC destination address %s.\n\nTrack your BTC transaction ID <a href='%s/%s'>%s</a>.\n\nXMRTO order complete.\n\nGood bye.\n\nPowered by @%s via https://xmr.to", orderstatus.UUID, orderstatus.BTCAmount, orderstatus.BTCDestAddress, viper.GetString("xmrto_btc_blockexplorer"), orderstatus.BTCTransactionID, orderstatus.BTCTransactionID, viper.GetString("BOT_NAME"))
				bot.Send(msg)
				return nil
			}
		}
	}
}
