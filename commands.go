package monerotipbot

import (
	"bytes"
	"fmt"
	"image/png"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/makiuchi-d/gozxing"
	"github.com/makiuchi-d/gozxing/qrcode"
	"github.com/makiuchi-d/gozxing/qrcode/decoder"
	"github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"github.com/monero-ecosystem/go-xmrto-client"
	"github.com/spf13/viper"

	tgbotapi "gopkg.in/telegram-bot-api.v4"
)

func (mtb *MoneroTipBot) parseCommandSTART() error {
	msg := mtb.newReplyMessage(false)
	msg.Text = viper.GetString("welcome_message")
	mtb.reply(msg)
	return mtb.createAccountIfNotExists()
}

func (mtb *MoneroTipBot) parseCommandHELP() error {
	msg := mtb.newReplyMessage(false)

	if len(mtb.message.CommandArguments()) == 0 {
		msg.Text = viper.GetString("help_message")
		return mtb.reply(msg)
	}

	if len(mtb.message.CommandArguments()) > 0 {
		switch mtb.message.CommandArguments() {
		case COMMANDS[TIP]:
			msg.Text = viper.GetString("help_message_TIP")
			return mtb.reply(msg)
		case COMMANDS[SEND]:
			msg.Text = viper.GetString("help_message_SEND")
			return mtb.reply(msg)
		case COMMANDS[GIVEAWAY]:
			msg.Text = viper.GetString("help_message_GIVEAWAY")
			return mtb.reply(msg)
		case COMMANDS[WITHDRAW]:
			msg.Text = viper.GetString("help_message_WITHDRAW")
			return mtb.reply(msg)
		case COMMANDS[BALANCE]:
			msg.Text = viper.GetString("help_message_BALANCE")
			return mtb.reply(msg)
		case COMMANDS[GENERATEQR]:
			msg.Text = viper.GetString("help_message_GENERATEQR")
			return mtb.reply(msg)
		case COMMANDS[XMRTO]:
			msg.Text = viper.GetString("help_message_XMRTO")
			return mtb.reply(msg)
		default:
			msg.Text = "Command not found."
			return mtb.reply(msg)
		}
	}
	return nil
}

func (mtb *MoneroTipBot) parseCommandTIP() error {
	msg := mtb.newReplyMessage(true)

	if len(mtb.message.CommandArguments()) == 0 {
		msg.Text = fmt.Sprintf("Please specify username and amount to tip (with optional message): /tip username %f yourmessage goes here", viper.GetFloat64("MIN_TIP_AMOUNT"))
		return mtb.reply(msg)
	}
	var split []string

	if mtb.isReplyToMessage() {
		split = strings.SplitN(mtb.message.CommandArguments(), " ", 2)
		if len(split) < 1 {
			msg.Text = "Need correct amount of command arguments. For tipping reply messages only amount is needed."
			return mtb.reply(msg)
		}
	} else {
		split = strings.SplitN(mtb.message.CommandArguments(), " ", 3)
		if len(split) < 2 {
			msg.Text = "Need correct amount of command arguments."
			return mtb.reply(msg)
		}
		if len(split[0]) == 0 {
			msg.Text = "Need correct amount of command arguments."
			return mtb.reply(msg)
		}
	}

	var message string

	var username string
	var amountstr string
	var casesensitiveusername string
	if mtb.isReplyToMessage() {
		casesensitiveusername = mtb.message.ReplyToMessage.From.UserName
		username = strings.ToLower(mtb.message.ReplyToMessage.From.UserName)
		amountstr = split[0]
		if len(split) > 1 {
			if len(split[1]) > 0 {
				message = split[1]
			}
		}
	} else {
		casesensitiveusername = split[0]
		username = strings.ToLower(split[0])
		amountstr = split[1]
		if len(split) > 2 {
			if len(split[2]) > 0 {
				message = split[2]
			}
		}
	}

	if !usernameregexp.MatchString(username) {
		msg.Text = "That doesn't look like a telegram username. You can only tip users who have a username handle."
		return mtb.reply(msg)
	}

	if strings.HasPrefix(username, "@") {
		username = strings.TrimPrefix(username, "@")
	}

	if strings.ContainsAny(amountstr, ",") {
		amountstr = strings.Replace(amountstr, ",", ".", -1)
	}
	parseamount, err := strconv.ParseFloat(strings.TrimSpace(amountstr), 64)
	if err != nil {
		msg.Text = "Could not parse amount."
		return mtb.reply(msg)
	}
	if parseamount < viper.GetFloat64("MIN_TIP_AMOUNT") {
		if !mtb.message.Chat.IsPrivate() {
			msg.ChatID = mtb.message.Chat.ID
		}
		msg.Text = fmt.Sprintf("Minimum amount for a tip is %f XMR", viper.GetFloat64("MIN_TIP_AMOUNT"))
		return mtb.reply(msg)
	}
	amount := wallet.Float64ToXMR(parseamount)

	if username == mtb.getUsername() {
		msg.Text = "Aww, tipping yourself? How about tipping the developer of this bot?\nMake the dev happy by donating to: ...\n"
		mtb.reply(msg)
		msg.Text = viper.GetString("DEV_DONATION_ADDRESS")
		return mtb.reply(msg)
	}
	if username == strings.ToLower(viper.GetString("BOT_NAME")) {
		msg.Text = "Aww, tipping me? How about tipping the developer of this bot?\nMake the dev happy by donating to: ...\n"
		mtb.reply(msg)
		msg.Text = viper.GetString("DEV_DONATION_ADDRESS")
		return mtb.reply(msg)
	}

	useraccount, err := mtb.getUserAccount()
	if err != nil {
		return err
	}

	// trick: replace current user with the recipient user to invoke a second getUserAccount()
	tmp := mtb.from.UserName
	tmpid := mtb.from.ID
	tmpchatid := mtb.message.Chat.ID
	mtb.from.ID = 0 // we don't know the recipient's ID at this stage
	mtb.from.UserName = username
	recipientaccount, err := mtb.getUserAccount()
	if err != nil {
		return err
	}
	// check if the recipient has a wallet account. if not we will create one.
	if recipientaccount == nil {
		// forbid tipping unknown users (unknown to us. in the wallet) from bot PM
		// this is a feature. not a bug
		if mtb.message.Chat.IsPrivate() {
			// switch back to current user again
			mtb.message.From.UserName = tmp
			mtb.message.From.ID = tmpid
			msg.Text = "User not found. Tipping new users is only possible in group chats."
			return mtb.reply(msg)
		}
		// important:
		// when using this trick, we have to make sure to set chatID to 0
		// otherwise a group chat will get notified about a wallet creation!!!
		mtb.message.Chat.ID = 0 // IMPORTANT!!!

		err = mtb.createAccountIfNotExists()
		if err != nil {
			return err
		}
		// now fetch the recipient account again. this time the wallet must exist.
		recipientaccount, err = mtb.getUserAccount()
		if err != nil {
			return err
		}
		mtb.message.Chat.ID = tmpchatid
	}
	// now switch back to current user again
	mtb.message.From.UserName = tmp
	mtb.message.From.ID = tmpid
	msg = mtb.newReplyMessage(true)

	var destinations []*wallet.Destination

	destinations = append(destinations, &wallet.Destination{
		Amount:  amount,
		Address: recipientaccount.BaseAddress,
	})

	// stat the transfer time
	start := time.Now()
	resp, err := mtb.walletrpc.Transfer(&wallet.RequestTransfer{
		AccountIndex: useraccount.AccountIndex,
		Destinations: destinations,
	})
	if err != nil {
		if !mtb.message.Chat.IsPrivate() {
			msg.ChatID = mtb.message.Chat.ID
		}
		msg.Text = fmt.Sprintf("Tip Error: %s", err)
		return mtb.reply(msg)
	}
	mtb.statsdPrecisionTiming("transaction.time_to_complete", time.Since(start))
	// stat the transaction count
	mtb.statsdIncr("transactions.counter", 1)

	tippermsg := mtb.newReplyMessage(false)
	tippermsg.Text = fmt.Sprintf("You successfully tipped user @%s.", strings.TrimPrefix(casesensitiveusername, "@"))
	tippermsg.Text = fmt.Sprintf("%s\n\nAmount: %f\nFee: %f\nTxHash: <a href='%s%s'>%s</a>", tippermsg.Text, wallet.XMRToFloat64(amount), wallet.XMRToFloat64(resp.Fee), viper.GetString("blockexplorer_url"), resp.TxHash, resp.TxHash)

	getrecipientid := strings.Split(recipientaccount.Label, "@")

	if len(getrecipientid) == 2 {
		if getrecipientid[1] == "0" {
			msg := mtb.newReplyMessage(false)
			// we dont have userID. so fallback to group chat
			msg.ChatID = mtb.message.Chat.ID

			if mtb.message.Chat.IsGroup() || mtb.message.Chat.IsSuperGroup() {
				msg.Text = fmt.Sprintf("@%s, you have been tipped with %f XMR from user @%s.\nPlease PM me (@%s) and click the 'Start' button to complete your account.", username, wallet.XMRToFloat64(amount), mtb.message.From.UserName, viper.GetString("BOT_NAME"))
			}
			if mtb.message.Chat.IsPrivate() {
				msg.Text = fmt.Sprintf("Silently tipped @%s with %f XMR. Notification failed. Please notify the user of starting this bot (@%s).", username, wallet.XMRToFloat64(amount), viper.GetString("BOT_NAME"))
			}
			mtb.reply(msg)
		} else {
			msg := mtb.newReplyMessage(false)

			recipientUserID, err := strconv.ParseInt(getrecipientid[1], 10, 64)
			if err != nil {
				mtb.reply(tippermsg)
				return err
			}
			msg.ChatID = recipientUserID

			if len(message) > 0 {
				// we know recipient user id here: send to user with message included if message exists
				if mtb.isReplyToMessage() {
					msg.Text = fmt.Sprintf("You have been tipped with %f XMR from user @%s\n\n<b>Replied to your message:</b>\n%s\n\n<b>Tip message:</b>\n%s", wallet.XMRToFloat64(amount), mtb.message.From.UserName, mtb.message.ReplyToMessage.Text, message)
				} else {
					msg.Text = fmt.Sprintf("You have been tipped with %f XMR from user @%s\n\n<b>Tip message:</b>\n%s", wallet.XMRToFloat64(amount), mtb.message.From.UserName, message)
				}
			} else {
				// we know recipient user id here: send to user without message, since it does not exist
				if mtb.isReplyToMessage() {
					msg.Text = fmt.Sprintf("You have been tipped with %f XMR from user @%s\n\n<b>Replied to your message:</b>\n%s", wallet.XMRToFloat64(amount), mtb.message.From.UserName, mtb.message.ReplyToMessage.Text)
				} else {
					msg.Text = fmt.Sprintf("You have been tipped with %f XMR from user @%s", wallet.XMRToFloat64(amount), mtb.message.From.UserName)
				}
			}

			err = mtb.reply(msg)
			if err != nil {
				msg.ChatID = mtb.message.Chat.ID
				// success on reaching out to user PM.
				if mtb.message.Chat.IsGroup() || mtb.message.Chat.IsSuperGroup() {
					msg.Text = fmt.Sprintf("@%s, you have been tipped with %f XMR from user @%s.", username, wallet.XMRToFloat64(amount), mtb.message.From.UserName)
				}
				if mtb.message.Chat.IsPrivate() {
					msg.Text = fmt.Sprintf("Silently tipped @%s with %f XMR. Notification failed. Please notify the user of starting this bot (@%s).", username, wallet.XMRToFloat64(amount), viper.GetString("BOT_NAME"))
				}
				mtb.reply(msg)
				return mtb.reply(tippermsg)
			}
			tippermsg.Text = fmt.Sprintf("%s\n\nUser has been notified.", tippermsg.Text)
			return mtb.reply(tippermsg)
		}
	}

	return mtb.reply(tippermsg)
}

func (mtb *MoneroTipBot) parseCommandSEND() error {
	msg := mtb.newReplyMessage(true)

	if len(mtb.message.CommandArguments()) == 0 {
		msg.Text = "Please specify an address and amount to send: /send ADDRESSHERE 0.00042"
		return mtb.reply(msg)
	}
	split := strings.Split(mtb.message.CommandArguments(), " ")
	if len(split) != 2 {
		msg.Text = "Need correct amount of command arguments."
		return mtb.reply(msg)
	}
	if len(split[0]) == 0 {
		msg.Text = "Need correct amount of command arguments."
		return mtb.reply(msg)
	}
	if len(split[1]) == 0 {
		msg.Text = "Need correct amount of command arguments."
		return mtb.reply(msg)
	}

	destinationaddress := split[0]

	if strings.ContainsAny(split[1], ",") {
		split[1] = strings.Replace(split[1], ",", ".", -1)
	}

	parseamount, err := strconv.ParseFloat(strings.TrimSpace(split[1]), 64)
	if err != nil {
		msg.Text = "Could not parse the amount. Aborted"
		return mtb.reply(msg)
	}
	if parseamount == 0 {
		msg.Text = "Amount is zero. Aborted"
		return mtb.reply(msg)
	}

	amount := wallet.Float64ToXMR(parseamount)

	var destinations []*wallet.Destination
	destinations = append(destinations, &wallet.Destination{
		Amount:  amount,
		Address: destinationaddress,
	})

	useraccount, err := mtb.getUserAccount()
	if err != nil {
		return err
	}

	// stat the transfer time
	start := time.Now()
	resp, err := mtb.walletrpc.Transfer(&wallet.RequestTransfer{
		AccountIndex: useraccount.AccountIndex,
		Destinations: destinations,
	})
	if err != nil {
		msg.Text = fmt.Sprintf("Error: %s", err)
		return mtb.reply(msg)
	}
	mtb.statsdPrecisionTiming("transaction.time_to_complete", time.Since(start))
	// stat the transaction count
	mtb.statsdIncr("transactions.counter", 1)

	msg.Text = fmt.Sprintf("Successfully sent %f to address %s", wallet.XMRToFloat64(amount), destinationaddress)
	mtb.reply(msg)
	msg.Format = false
	msg.Text = fmt.Sprintf("Amount: %f\nFee: %f\nTxHash: <a href='%s%s'>%s</a>", wallet.XMRToFloat64(amount), wallet.XMRToFloat64(resp.Fee), viper.GetString("blockexplorer_url"), resp.TxHash, resp.TxHash)
	mtb.reply(msg)

	return nil
}

func (mtb *MoneroTipBot) parseCommandGIVEAWAY() error {
	msg := mtb.newReplyMessage(true)

	if !mtb.message.Chat.IsGroup() && !mtb.message.Chat.IsSuperGroup() {
		msg.Text = "Giveaways can only be made in groups. Aborting."
		return mtb.reply(msg)
	}

	if len(mtb.message.CommandArguments()) == 0 {
		msg.Text = "Please specify the amount to give away: /giveaway 0.00042"
		return mtb.reply(msg)
	}

	amountstr := mtb.message.CommandArguments()
	if strings.ContainsAny(amountstr, ",") {
		amountstr = strings.Replace(amountstr, ",", ".", -1)
	}
	parseamount, err := strconv.ParseFloat(strings.TrimSpace(amountstr), 64)
	if err != nil {
		msg.Text = "Could not parse the amount. Aborted"
		return mtb.reply(msg)
	}
	if parseamount < viper.GetFloat64("MIN_TIP_AMOUNT") {
		if !mtb.message.Chat.IsPrivate() {
			msg.ChatID = mtb.message.Chat.ID
		}
		msg.Text = fmt.Sprintf("Minimum amount for a tip is %f XMR", viper.GetFloat64("MIN_TIP_AMOUNT"))
		return mtb.reply(msg)
	}

	useraccount, err := mtb.getUserAccount()
	if err != nil {
		return err
	}

	if useraccount.UnlockedBalance < wallet.Float64ToXMR(parseamount) {
		if !mtb.message.Chat.IsPrivate() {
			msg.ChatID = mtb.message.Chat.ID
		}
		msg.Text = "Error: Not enough unlocked money."
		return mtb.reply(msg)
	}

	giveawaytext := fmt.Sprintf("User @%s is giving %f XMR away. Click the 'Claim' button to claim it.", mtb.message.From.UserName, parseamount)
	claim := tgbotapi.NewInlineKeyboardButtonData("Claim", "giveaway_claim")
	cancel := tgbotapi.NewInlineKeyboardButtonData("Cancel", "giveaway_cancel")
	markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(claim, cancel))
	giveawaymsg := tgbotapi.NewMessage(mtb.message.Chat.ID, "")
	giveawaymsg.ReplyMarkup = markup
	giveawaymsg.Text = giveawaytext
	resp, _ := mtb.bot.Send(giveawaymsg)

	giveaway := &Giveaway{
		Message: &resp,
		From:    mtb.message,
		Sender:  useraccount,
		Amount:  wallet.Float64ToXMR(parseamount),
	}
	mtb.giveaways = append(mtb.giveaways, giveaway)
	return mtb.saveGiveawayToFile()
}

func (mtb *MoneroTipBot) parseCommandWITHDRAW() error {
	msg := mtb.newReplyMessage(true)

	if len(mtb.message.CommandArguments()) == 0 {
		msg.Text = "Please specify destination address to withdraw to: /withdraw ADDRESSHERE"
		return mtb.reply(msg)
	}

	withdrawaddress := mtb.message.CommandArguments()

	validaddr, err := mtb.walletrpc.ValidateAddress(&wallet.RequestValidateAddress{Address: withdrawaddress, AnyNetType: viper.GetBool("IS_STAGENET_WALLET")})
	if err != nil {
		msg.Text = "Something went wrong. Could not validate address."
		return mtb.reply(msg)
	}
	if !validaddr.Valid {
		msg.Text = "This is not a valid monero address. Aborted."
		return mtb.reply(msg)
	}

	useraccount, err := mtb.getUserAccount()
	if err != nil {
		return err
	}

	// stat the transfer time
	start := time.Now()
	resp, err := mtb.walletrpc.SweepAll(&wallet.RequestSweepAll{
		AccountIndex: useraccount.AccountIndex,
		Address:      mtb.message.CommandArguments(),
	})
	if err != nil {
		msg.Text = fmt.Sprintf("Error: %s", err)
		return mtb.reply(msg)
	}
	mtb.statsdPrecisionTiming("transaction.time_to_complete", time.Since(start))
	// stat the transaction count
	mtb.statsdIncr("transactions.counter", 1)

	msg.Format = false
	msg.Text = fmt.Sprintf("Withdraw successful.\n\nAmount: %f\nFee: %f\nTxHash: <a href='%s%s'>%s</a>\n", wallet.XMRToFloat64(resp.AmountList[0]), wallet.XMRToFloat64(resp.FeeList[0]), viper.GetString("blockexplorer_url"), resp.TxHashList[0], resp.TxHashList[0])
	return mtb.reply(msg)
}

func (mtb *MoneroTipBot) parseCommandBALANCE() error {
	msg := mtb.newReplyMessage(true)
	useraccount, err := mtb.getUserAccount()
	if err != nil {
		return err
	}
	balances, err := mtb.walletrpc.GetBalance(&wallet.RequestGetBalance{
		AccountIndex: useraccount.AccountIndex,
	})
	if err != nil {
		msg.Text = fmt.Sprintf("Error: %s", err)
		return mtb.reply(msg)
	}

	if balances.PerSubaddress == nil {
		msg.Text = "Account has no funds. No balance to show."
		mtb.reply(msg)
		msg.Text = useraccount.BaseAddress
		return mtb.reply(msg)
	}

	totalbalance := 0
	totalunlockedbalance := 0
	totalnumunspentoutputs := 0
	var blockstounlock []int

	for _, balance := range balances.PerSubaddress {
		totalbalance = totalbalance + int(balance.Balance)
		totalunlockedbalance = totalunlockedbalance + int(balance.UnlockedBalance)
		totalnumunspentoutputs = totalnumunspentoutputs + int(balance.NumUnspentOutputs)

		if balance.BlocksToUnlock > 0 {
			blockstounlock = append(blockstounlock, int(balance.BlocksToUnlock))
		}
	}

	var sumblockstounlock int
	if len(blockstounlock) > 0 {
		sort.Ints(blockstounlock)
		sumblockstounlock = blockstounlock[len(blockstounlock)-1]
	}

	if sumblockstounlock > 0 {
		msg.Text = fmt.Sprintf("Balance: %f\nUnlocked Balance: %f\nBlocks To Unlock (accumulated): %d (~%d minutes)\nUnspent Outputs: %d\nAddress: ...", wallet.XMRToFloat64(uint64(totalbalance)), wallet.XMRToFloat64(uint64(totalunlockedbalance)), sumblockstounlock, sumblockstounlock*2, totalnumunspentoutputs)
		mtb.reply(msg)
	} else {
		msg.Text = fmt.Sprintf("Balance: %f\nUnlocked Balance: %f\nBlocks To Unlock: %d\nUnspent Outputs: %d\nAddress: ...", wallet.XMRToFloat64(uint64(totalbalance)), wallet.XMRToFloat64(uint64(totalunlockedbalance)), sumblockstounlock, totalnumunspentoutputs)
		mtb.reply(msg)
	}
	msg.Text = useraccount.BaseAddress
	mtb.reply(msg)

	return nil
}

func (mtb *MoneroTipBot) parseCommandGENERATEQR() error {
	msg := mtb.newReplyMessage(true)

	var encodestring string

	if len(mtb.message.CommandArguments()) == 0 {
		useraccount, err := mtb.getUserAccount()
		if err != nil {
			return err
		}

		resp, err := mtb.walletrpc.CreateAddress(&wallet.RequestCreateAddress{AccountIndex: useraccount.AccountIndex, Label: fmt.Sprintf("%d", mtb.from.ID)})
		if err != nil {
			return err
		}

		encodestring := fmt.Sprintf("monero:%s", resp.Address)

		qrReader := qrcode.NewQRCodeWriter()
		hints := map[gozxing.EncodeHintType]interface{}{gozxing.EncodeHintType_ERROR_CORRECTION: decoder.ErrorCorrectionLevel_L}
		qrcode, err := qrReader.Encode(encodestring, gozxing.BarcodeFormat_QR_CODE, 400, 400, hints)
		if err != nil {
			return err
		}

		buf := new(bytes.Buffer)
		png.Encode(buf, qrcode)

		fb := tgbotapi.FileBytes{Name: "image.png", Bytes: buf.Bytes()}
		photomsg := tgbotapi.NewPhotoUpload(mtb.getReplyID(), fb)
		_, err = mtb.bot.Send(photomsg)
		if err != nil {
			return err
		}

		return nil
	}

	split := strings.SplitN(mtb.message.CommandArguments(), " ", 2)
	if len(split) < 1 {
		msg.Text = "Need correct amount of command arguments."
		return mtb.reply(msg)
	}
	if len(split[0]) == 0 {
		msg.Text = "Need correct amount of command arguments."
		return mtb.reply(msg)
	}
	amountstr := split[0]
	var description string
	if len(split) > 1 {
		if len(split[1]) > 0 {
			description = split[1]
		}
	}

	parseamount, err := strconv.ParseFloat(strings.TrimSpace(amountstr), 64)
	if err != nil {
		msg.Text = "Could not parse amount."
		return mtb.reply(msg)
	}

	useraccount, err := mtb.getUserAccount()
	if err != nil {
		return err
	}

	resp, err := mtb.walletrpc.CreateAddress(&wallet.RequestCreateAddress{AccountIndex: useraccount.AccountIndex, Label: fmt.Sprintf("%d", mtb.from.ID)})
	if err != nil {
		return err
	}

	encodestring = fmt.Sprintf("monero:%s?tx_amount=%f&recipient_name=@%s (telegram)&tx_description=%s\n\nPowered by @%s", resp.Address, parseamount, mtb.from.UserName, description, viper.GetString("BOT_NAME"))

	qrReader := qrcode.NewQRCodeWriter()
	hints := map[gozxing.EncodeHintType]interface{}{gozxing.EncodeHintType_ERROR_CORRECTION: decoder.ErrorCorrectionLevel_L}
	qrcode, err := qrReader.Encode(encodestring, gozxing.BarcodeFormat_QR_CODE, 400, 400, hints)
	if err != nil {
		return err
	}

	buf := new(bytes.Buffer)
	png.Encode(buf, qrcode)

	fb := tgbotapi.FileBytes{Name: "image.png", Bytes: buf.Bytes()}
	photomsg := tgbotapi.NewPhotoUpload(mtb.getReplyID(), fb)
	_, err = mtb.bot.Send(photomsg)
	if err != nil {
		return err
	}

	mtb.statsdIncr("qrcode_generated.counter", 1)

	return nil
}

func (mtb *MoneroTipBot) parseCommandXMRTO() error {
	// initiate a new xmrto client
	client := xmrto.New(&xmrto.Config{Testnet: viper.GetBool("IS_STAGENET_WALLET")})

	if len(mtb.message.CommandArguments()) == 0 {
		msg := mtb.newReplyMessage(false)
		getorder, err := client.GetOrderParameters()
		if err != nil {
			msg.Text = fmt.Sprintf("XMRTO API Error: %s", err)
			mtb.reply(msg)
			return err
		}
		mtb.statsdIncr("xmrto_getorderparams.counter", 1)
		msg.Text = fmt.Sprintf("XMR.TO Current Order Parameters\n\nPrice: %f\nLowerLimit: %f\nUpperLimit: %f\nZeroConfEnabled: %t\nZeroConfMaxAmount: %f", getorder.Price, getorder.LowerLimit, getorder.UpperLimit, getorder.ZeroConfEnabled, getorder.ZeroConfMaxAmount)
		return mtb.reply(msg)
	}

	msg := mtb.newReplyMessage(false)
	split := strings.SplitN(mtb.message.CommandArguments(), " ", 2)
	if len(split) < 1 {
		msg.Text = "Need correct amount of command arguments."
		return mtb.reply(msg)
	}

	if len(split) != 2 {
		msg.Text = "Need correct amount of command arguments."
		return mtb.reply(msg)
	}

	btcaddress := split[0]
	amountstr := split[1]

	parseamount, err := strconv.ParseFloat(strings.TrimSpace(amountstr), 64)
	if err != nil {
		msg.Text = "Could not parse amount."
		return mtb.reply(msg)
	}

	if parseamount <= 0 {
		msg.Text = "Amount must be positive."
		return mtb.reply(msg)
	}

	useraccount, err := mtb.getUserAccount()
	if err != nil {
		msg.Text = fmt.Sprintf("Error: %s", err)
		return mtb.reply(msg)
	}

	if useraccount == nil {
		msg.Text = "Oops. Something went wrong. This should not happen!"
		return mtb.reply(msg)
	}

	// prepare message object here. will use for multiple replies in edit mode.
	newmsg := tgbotapi.NewMessage(mtb.message.Chat.ID, "")

	// let's create an order with 0.001 btc.
	createorder, err := client.CreateOrder(&xmrto.RequestCreateOrder{
		BTCAmount:      parseamount,
		BTCDestAddress: btcaddress,
	})
	if err != nil {
		msg.Text = fmt.Sprintf("XMRTO API Error: %s", err)
		return mtb.reply(msg)
	}
	mtb.statsdIncr("xmrto_createorder.counter", 1)

	// give xmrto request time to settle down...
	newmsg.Text = "Request sent. Waiting 3 seconds for order to reach XMRTO nodes. Stand by..."
	chattable, err := mtb.bot.Send(newmsg)
	if err != nil {
		return err
	}

	time.Sleep(time.Second * 3)

	str1 := fmt.Sprintf("-- XMR.TO Reponse --\nBTCAmount: %f\nBTCDestAddress: %s\nState: %s\nUUID: %s", createorder.BTCAmount, createorder.BTCDestAddress, createorder.State, createorder.UUID)
	str2 := fmt.Sprintf("Fetching order details from XMRTO API with secret-key: %s", createorder.UUID)
	edit := tgbotapi.NewEditMessageText(int64(chattable.Chat.ID), chattable.MessageID, "")
	edit.Text = fmt.Sprintf("%s\n\n%s\n\n%s", chattable.Text, str1, str2)
	edit.ParseMode = "HTML"
	chattable, err = mtb.bot.Send(edit)
	if err != nil {
		return err
	}

	time.Sleep(time.Second * 1)

	// now check the order state with the secret-key
	// we received from xmr.to for this particular order.
	orderstatus, err := client.GetOrderStatus(&xmrto.RequestGetOrderStatus{UUID: createorder.UUID})
	if err != nil {
		msg.Text = fmt.Sprintf("XMRTO API Error: %s", err)
		return mtb.reply(msg)
	}
	mtb.statsdIncr("xmrto_orderstatus.counter", 1)

	if useraccount.UnlockedBalance <= wallet.Float64ToXMR(orderstatus.XMRAmountTotal) {
		msg.Text = fmt.Sprintf("Insufficient funds. You need at least %f XMR in your unlocked balance.", parseamount)
		return mtb.reply(msg)
	}

	out := fmt.Sprintf("%s\n\n-- XMR.TO Reponse --\nState: <b>%s</b>\nUUID: <b>%s</b>\nBTCAmount: %f\nBTCDestAddress: %s\nCreatedAT: %s\nExpiresAT: %s\nSecondsTillTimeout: %d\nXMRAmountTotal: %f XMR\nXMRPriceBTC: %f BTC\nXMRReceivingSubAddress: %s",
		chattable.Text,
		orderstatus.State,
		orderstatus.UUID,
		orderstatus.BTCAmount,
		orderstatus.BTCDestAddress,
		orderstatus.CreatedAT,
		orderstatus.ExpiresAT,
		orderstatus.SecondsTillTimeout,
		orderstatus.XMRAmountTotal,
		orderstatus.XMRPriceBTC,
		orderstatus.XMRReceivingSubAddress,
	)

	out = fmt.Sprintf("%s\n\n---\nNeed to deposit %f XMR to above <i>XMRReceivingSubAddress</i> wallet to relay the requested BTC.\n\nTo complete your request I will send %f XMR from your account to the given address above. Please confirm or cancel.\n\n<b>You have less than 4 minutes to confirm this transaction.</b>\n", out, orderstatus.XMRAmountTotal, orderstatus.XMRAmountTotal)

	claim := tgbotapi.NewInlineKeyboardButtonData("Send", "xmrto_tx_send")
	cancel := tgbotapi.NewInlineKeyboardButtonData("Cancel", "xmrto_tx_cancel")
	markup := tgbotapi.NewInlineKeyboardMarkup(tgbotapi.NewInlineKeyboardRow(claim, cancel))

	edit = tgbotapi.NewEditMessageText(int64(chattable.Chat.ID), chattable.MessageID, "")
	edit.Text = fmt.Sprintf("%s\n\n%s", edit.Text, out)
	edit.ParseMode = "HTML"
	edit.ReplyMarkup = &markup
	chattable, err = mtb.bot.Send(edit)
	if err != nil {
		return err
	}

	mtb.statsdIncr("xmrto_tx.counter", 1)

	xmrtoOrder := &XMRToOrder{
		Message: &chattable,
		From:    mtb.message,
		Order:   orderstatus,
	}
	mtb.xmrtoOrders = append(mtb.xmrtoOrders, xmrtoOrder)

	return nil
}
