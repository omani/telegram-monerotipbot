package monerotipbot

import (
	"github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"github.com/monero-ecosystem/go-xmrto-client"
	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

// Notification will be used for broadcasting messages to users or all users the bot knows
type Notification struct {
	// UserID is the user id
	UserID int64 `json:"UserID"`
	// Message is the message
	Message string `json:"Message"`
	// Sent indicates if the message has been sent to the user
	Sent bool `json:"Sent"`
	// Error is set to true if message could not be sent due to an error
	Error string `json:"Error"`
}

// Message represents a reply message in Telegram
type Message struct {
	Text   string
	Format bool
	ChatID int64
}

// Tip represents a tip
type Tip struct {
	Sender struct {
		IUsername string
		Username  string
		Amount    uint64
	}
	Recipient struct {
		IUsername string
		Username  string
		Amount    uint64
	}
}

// Giveaway is a json tagged struct to save it as a file on disk and represents giveaways made by users
type Giveaway struct {
	Message *tgbotapi.Message `json:"message"`
	From    *tgbotapi.Message `json:"from"`
	Sender  *Account          `json:"sender"`
	Amount  uint64            `json:"amount"`
}

// QRCode will always be in memory. No need to save it on disk. Represents a QR-Code object
type QRCode struct {
	Message  *tgbotapi.Message
	From     *tgbotapi.Message
	ParseURI *wallet.ResponseParseURI
	Amount   uint64
}

// XMRToOrder will always be in memory due to xmr.to's 5 minute timeout (too short worth to save it to disk). Represents an XMRTO Order that has been created
type XMRToOrder struct {
	Message *tgbotapi.Message
	From    *tgbotapi.Message
	Order   *xmrto.ResponseGetOrderStatus
}

// Account is a 1:1 copy of the Account struct in the go-monero-rpc-client
type Account struct {
	// Index of the account.
	AccountIndex uint64
	// Balance of the account (locked or unlocked).
	Balance uint64
	// Base64 representation of the first subaddress in the account.
	BaseAddress string
	// (Optional) Label of the account.
	Label string
	// (Optional) Tag for filtering accounts.
	Tag string
	// Unlocked balance for the account.
	UnlockedBalance uint64
}
