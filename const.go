package monerotipbot

const (
	// START command for starting the bot
	START = string(iota + 1)
	// HELP command for showing usage of bot
	HELP
	// TIP command for tipping users
	TIP
	// SEND command for making normal transactions
	SEND
	// GIVEAWAY command for giveaways
	GIVEAWAY
	// WITHDRAW command for withdraws
	WITHDRAW
	// BALANCE command for showing wallet balance
	BALANCE
	// GENERATEQR command for generating QR-Codes (images)
	GENERATEQR
)

var (
	// COMMANDS defines all Telegram commands this bot has
	COMMANDS = map[string]string{
		START:      "start",
		HELP:       "help",
		TIP:        "tip",
		SEND:       "send",
		GIVEAWAY:   "giveaway",
		WITHDRAW:   "withdraw",
		BALANCE:    "balance",
		GENERATEQR: "generateqr",
	}
)
