go-telegram-monerotipbot
========================

<p align="center">
<img src="https://github.com/omani/go-telegram-monerotipbot/raw/master/assets/img/icon.png" alt="MoneroTipBot" width="200" />
</p>

A telegram Monero tip bot written in go.

___
## Preface
This is the official repository of the Monero tip bot for Telegram, also known under the username [@MoneroTipBot](http://t.me/MoneroTipBot).

## Introduction
What can this bot do?

MoneroTipBot is the one and only, first official Monero wallet for Telegram that started with a [CCS Proposal](https://ccs.getmonero.org) and has been funded by the great Monero community.

MoneroTipBot will be open sourced (under the MIT license) on GitHub.

**ATTENTION/IMPORTANT**

Since the code is open source, beware of MoneroTipBot clones who could potentially scam you.
The only official bot is the bot under the Telegram username [@MoneroTipBot](http://t.me/MoneroTipBot).
If in doubt, please do not hesitate to ask in various Monero groups for the right bot.

We are not responsible - at all - for any usage of the code by other people in any way!

**Notice**:

This is a custodial Monero wallet service. You do not own your private keys. We own them!
Beside the wallet itself, nothing will be stored on the server - where the bot is running - ever. No user data, personal information or otherwise personal data will be stored. The bot is expected to be stable. However, always have in mind that you might lose your money due to a bug or we can get hacked and lose all funds. We cannot guarantee anything. So use this bot on your own risk!

**Warning**:

If you ever think about misusing this bot for more than just tipping small amounts to other users, for example using it as your personal wallet, please be informed that there is absolutely no guarantee - in whatever form - given by the bot and the bot operator. By using this bot you agree that you read and agreed to the LEGAL NOTICE.

**Features**:
- Everything happens on-chain.
- No transaction is stored on disk or a database.
- Always synced wallet. Unlike other wallet clients, there is no need to wait until the wallet is fully synced.
- Group-friendly spam-free messages
- Sensible wallet information will always be sent to user as private message.
- Tip users on Telegram within groups. Optionally send them a message along with the tip.
- Send Monero to regular addresses.
- Receive Monero on regular addresses.
- Make Giveaways within groups.
- Send BTC by sending XMR via xmr.to
- Generate a QR-Code image and share it comfortably with others.
- Make transactions by scanning or uploading a QR-Code image.
- Deposit to account.
- Withdraw from account.
- Clickable TX Hash link to blockexplorer.

**LEGAL NOTICE**:

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

Invite this bot to your group (bot needs no admin rights) and enjoy Monero in Telegram.

For more information about the commands please use the help command.

Happy tipping!

## Help
```
Help Menu:

/tip username amount message
Tip a user with a certain amount and optionally a messageto send along with the tip. All users who started the bot will be notified upon a tip.


/send address amount
Send amount to a regular monero wallet address


/balance
Show your balance.


/giveaway amount
Make a giveaway within a telegram group.


/withdraw address
Withdraw everything from the tip bot wallet to your own wallet address.


/generateqr amount description
Generate a QR-Code image with the desired amount and optionally a description to share with others.

You can also paste a QR-Code image into this PM chat with me and I will try to decode it. This way you can make transactions by scanning QR-Codes.


/xmrto btc-address amount
Needs no further introduction. Those who know xmr.to will love this. :)
Pay the world with Monero via xmr.to from within the MoneroTipBot!


/help command

Show additional information about a specific command.'
```

## Commands
`/help tip`

/tip **username** **amount** *message*

Amount takes no trailing XMR symbol! Tip a user with a certain amount. All users who started the bot will be notified upon a tip.
Optionally you can specify a message to be sent along with the tip. This message will be forwarded to the user if that user has started the bot.

**Notice:**

Make sure the telegram user you want to tip actually exists, otherwise the moneros will be gone!!!

Unknown users to the bot, will be tipped regardless whether or not they exist in Telegram.

You cannot tip users who do not have an account at MoneroTipBot within the bot PM. Those users can only be tipped within a group.

Optionally, you can use the @ sign when giving the username. Exception to this is when you tip on a reply message. Then you don't need a username and only the amount. Amount has to be a number. Use decimals if you need fractional amounts (like 0.1).

___

`/help send`

/send **address** **amount**

Amount takes no trailing XMR symbol! Send a certain amount to a regular monero wallet address. Amount has to be a number. Use decimals if you need fractional amounts (like 0.1).

___

`/help balance`

/balance

Show your current balance. Balance is the total balance.
Unlocked balance is what is free to spend and the number of spent outputs gives you a hint to how many times you can tip.
If balance is locked the output will show the estimated time until unlocked.

___

`/help giveaway`

/giveaway **amount**

Amount takes no trailing XMR symbol! Make a giveaway within a telegram group. This is a group command. If this bot is in a group and you are a member of that group, you can make a giveaway with the amount you want to give away. The first user in that group who clicks on the 'Claim' button will receive that amount and a tip will happen between the giver and the taker.
Amount has to be a number. Use decimals if you need fractional amounts (like 0.1)

___

`/help generateqr`

/generateqr *amount* *description*

Generate a QR-Code image with the desired amount and optionally a description to share with others. The other person can then paste this image into the bot PM and can do a transaction.

Every QR-Code image generated will use a new subaccount address to avoid giving away your main wallet address.

If you don't specify any parameter, it will generate a QR-Code image with just your subaccount address.

The bot will try to parse the QR-Code according to [RFC 3986](https://github.com/monero-project/monero/wiki/URI-Formatting). If the QR-Code is RFC 3986 compliant you will be prompted with the content of the QR-Code (address, amount, recipient, description) and buttons to make the transaction or cancel it.

Example:
/generateqr 10 thank you for the donation. much appreciated! :)

___

`/help withdraw`

/withdraw **address**

Withdraw everything from the tip bot wallet to your own wallet address.

Make sure you double-check the recipient address to make sure you are sending to the right address.

___

`/help xmrto`

/xmrto btc-address amount

MoneroTipBot implements the API of https://xmr.to and makes it possible to send BTC anywhere by sending XMR to their service.
Specify the BTC address you want to send BTC to and the amount. The bot will start an interactive dialog and will guide you through the process.
Pass the /xmrto command without any arguments to see the order parameters of xmr.to.

**Highlight**:

Upload a BITCOIN QR-Code with a correct URI (that is, with an amount encoded) and tag (add a caption to) the image with 'xmr.to' (without the quotes) before uploading it to the bot PM."

___


## Administration/Configuration
This section is for the bot operator.

Configuration is done in the `settings.yml` YAML file.

Overview of the settings file:

#### #Telegram Bot Settings
`telegram_bot_token:`

Token of the Telegram bot. This is the token you get when you create a new bot in telegram via @BotFather.

`rpcchannel_uri: "tcp://127.0.0.1:5555"`

This is the socket for the ZMQ REQ/REP channel for the communication between the bot and external applications. It can be extended to do any kind of stuff.
For now it is being used by the notifier in `cmd/notifier` to send a message to one user or broadcast a message to all users.

`blockexplorer_url: "https://xmrchain.net/tx/"`

This is the link to the XMR blockexplorer when the bot sends the output about a transaction that happened and is used in the TxHash as an HTML anchor link. We use xmrchain.net here.

`BOT_NAME: ""`

The name of your telegram bot (@username, not the display name) as given in the creation process via @BotFather.

`DEV_DONATION_ADDRESS: "88jspkqPmvvc9L3LovdhjoCW2eBSKk4VNTsrdWqB4CYdBfKRWH5yL39bE6NP5Di2Wgix1cxBgKMAiXMbUwCBY3Dk2WvwSSA"`

This wallet address of (preferrably the bot operator) will be used in the message that pops up when a user tries to tip himself/herself.
Or leave it as it is with this wallet address. This is the address of the developer of this bot. He will be happy to receive tips :)

`MIN_TIP_AMOUNT: 0.00042`

The minimum amount that is allowed to tip. You shouldn't set this lower than the fees it requires to do the transaction.

`GIVEAWAY_FILE: "giveaways.json" # absolute path will also work`

This is the path to the file to save giveaways. Since giveaways can last for a long time (if nobody claimed it yet) we want to store them in files, in case the bot goes down or has to be restarted by the bot operator.

`BROADCAST_NOTIFICATION_INTERVAL: 10`

This is the interval broadcast messages will be sent out to users, in seconds. Leave it at 10, since Telegram has restriction on how many times a bot can message users per minute/second, etc. See https://core.telegram.org/bots/faq#how-can-i-message-all-of-my-bot-39s-subscribers-at-once for more information.

`LOGFILE: "monerotipbot.log"`

Telegram does not give us the information about how many groups the bot is in. We use a trick here, we track every `Chat.ID` of every message to track in how many groups the bot *could* possibly be:
```
// track in how many groups this bot is actually used in
if update.Message.Chat != nil {
  mutex.Lock()
  groupsBotIsIn[update.Message.Chat.ID] = update.Message.Chat
  mutex.Unlock()
}
```
As you can see, this is not accurate, and is just for informational purposes to get an estimated value about the group metric. The logfile will show public and private group chatIDs and the chatID of users who private message the bot within the bot PM.


#### #XMRTO service
`xmrto_website: "https://xmr.to"`

This is the third party application we use to make BTC payments available by using XMR transactions. This URL is their website.

`xmrto_btc_blockexplorer: "https://www.blockchain.com/btc/tx"`

This is a BTC blockexplorer for showing BTC TxIDs in the output messages of the xmrto command when the BTC were sent.

#### #Monero Wallet RPC Settings
`monero_rpc_daemon_url: "http://127.0.0.1:6061/json_rpc"`

This is the URL to **YOUR** (the bot operator's) wallet RPC daemon. Everything happens over this RPC wallet daemon.

`monero_rpc_daemon_username: ''`

You should enable HTTP Digest Login on the Monero wallet RPC daemon with the `--rpc-login` parameter, when starting the RPC daemon.

`monero_rpc_daemon_password: ''`

You should enable HTTP Digest Login on the Monero wallet RPC daemon with the `--rpc-login` parameter, when starting the RPC daemon.

`IS_STAGENET_WALLET: false`

Is this bot working with a stagenet wallet? That is, has the Monero wallet RPC damon been started with the `--stagenet` flag? If so, set to true here or things will not work.

#### #Statsd Settings (Metrics Logger)
`USE_STATSD: false`

This bot uses a statsd client to collect some application metrics all over the place (in the code). Set to true if you are running your own instance of a statsd backend. This works perfectly with a statds-graphite-grafana backend and was tested with this docker container https://github.com/kamon-io/docker-grafana-graphite. Follow the instructions there to set up a statsd backend for your bot.

`statsd_address: "127.0.0.1:8125"`

The socket for the statsd backend if you use one.

`statsd_prefix: "mytipbot."`

The prefix for all statsd metrics.

#### #Bot Helper Messages
`welcome_message: ""`

Welcome message to display when a user PMs the bot for the first time and starts it.

`help_message: ""`

The structure of the message of your help menu when a user invokes the `/help` command

`help_message_TIP: ""`

The structure of the message of your help menu when a user invokes the `/help tip` command

`help_message_SEND: ""`

The structure of the message of your help menu when a user invokes the `/help send` command

`help_message_GIVEAWAY: ""`

The structure of the message of your help menu when a user invokes the `/help giveaway` command

`help_message_WITHDRAW: ""`

The structure of the message of your help menu when a user invokes the `/help withdraw` command

`help_message_BALANCE: ""`

The structure of the message of your help menu when a user invokes the `/help balance` command

`help_message_GENERATEQR: ""`

The structure of the message of your help menu when a user invokes the `/help generateqr` command

`help_message_XMRTO: ""`

The structure of the message of your help menu when a user invokes the `/help xmrto` command


This was everything you can specify in your `settings.yml`. Adjust to your needs.

### BotFather Setup (Commands)
When creating your new bot as a Monero Tip Bot, remember to set up the commands of the new bot.

You must at least have these commands, paste them into the BotFather PM when asked to specify the commands:
```
help - Print help
tip - <username> <amount>
send - <address> <amount>
withdraw - <your private wallet address>
balance - Show your current balance
giveaway - <amount>
generateqr - <amount>
xmrto - <btcaddress> <amount>
```

### CMD Runs

> You can compile every main.go you see here if you want and use only the binary. But we will stick to the source code here.

#### Bot
Start the bot and specify the settings file:
```
cd cmd/
go run main.go -c ../settings.yml
```

The bot is considered to be safe in errors. To reduce interruptions while operating it on eg. a VPS, let it run in a loop so it "restarts itself" should there be anything wrong (mostly due to Telegram itself), if at all:
```
cd cmd/
while true; do go run main.go -c ../settings.yml; done
```

#### Notifier
To notify users create a file which holds the message you want to send or broadcast and send it with the notifier:
```
cd cmd/notifier

vi message.txt
...
<write down the message in that file and save it>
...
```
Send to specific user:

`go run main.go -messagefile message.txt -c ../../settings.yml -userid 123456`

Broadcast to all users:

`go run main.go -messagefile message.txt -c ../../settings.yml -broadcast`

You can also redirect the output of this operation to a file:

`go run main.go -messagefile message.txt -c ../../settings.yml -broadcast 2>&1 > broadcast.log`

#### LabelMapper
There is a label mapper to gather all labels from the wallet and save it to a local file. The label mapper just issues a `walletrpc.GetAccounts()` and dumps the object into a json file:

```
cd cmd/labelmapper
go run main.go -c ../../settings.yml
```

It will save it to a file named `labelmap.json`.

*Hint*: If you use the statsd client, you will get labels in your statsd backend, which is more comfortable since you don't have to run this LabelMapper in a cron or by hand.

## Questions
If you have questions or trouble feel free to create an issue in this repository so we can help you.

## Final Notes
**YOU** (the bot operator) are dealing with money here, at least when you are not operating on stagenet. With power comes responsibility.

If you use this bot in your own group, people trust you by depositing their Monero to your wallet. Do not abuse their trust!

- If you create a new wallet, write down and save away the mnemonic seed of that wallet.
- Do regular backups of the wallet. You can broadcast a "scheduled maintenance" message to all users. On that date, stop the bot, stop the wallet-rpc daemon, and make a backup of the wallet file and store it somewhere safe. Restart all services again after that.

## Contribution
* You can fork this, extend it and contribute back.
* You can contribute with pull requests.

## Donations
I love Monero (XMR) and building applications for and on top of Monero.

You can make me happy by donating Monero to the following address:
```
89woiq9b5byQ89SsUL4Bd66MNfReBrTwNEDk9GoacgESjfiGnLSZjTD5x7CcUZba4PBbE3gUJRQyLWD4Akz8554DR4Lcyoj
```

## LICENSE
MIT License
