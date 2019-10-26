package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/monero-ecosystem/go-monero-rpc-client/wallet"
	"github.com/spf13/viper"

	monerotipbot "github.com/monero-ecosystem/telegram-monerotipbot"
	zmq "github.com/pebbe/zmq4"
	"github.com/sirupsen/logrus"
)

var (
	userID      int64
	broadcast   bool
	messagefile string
	configpath  string
)

func main() {
	flag.Int64Var(&userID, "userid", 0, "User ID to send the message to.")
	flag.BoolVar(&broadcast, "broadcast", false, "Broadcast to all users if set.")
	flag.StringVar(&messagefile, "messagefile", "", "Message FILE to read message from.")
	flag.StringVar(&configpath, "c", "", "")

	flag.Parse()
	if !flag.Parsed() {
		log.Fatal("Couldn't parse cli arguments. Aborted")
	}

	if len(messagefile) == 0 {
		log.Fatal("Need a message FILE to read message from.")
	}
	if broadcast && userID > 0 {
		log.Fatal("Either send to user or do a broadcast. Not both.")
	}
	if !broadcast && userID == 0 {
		log.Fatal("Need either a user ID or the broadcast option set.")
	}

	if configpath == "" {
		configpath = filepath.Join(os.Getenv("HOME"), "settings.toml")
	}

	viper.SetConfigType("yaml")
	viper.SetConfigName(strings.TrimSuffix(path.Base(configpath), path.Ext(configpath)))
	viper.AddConfigPath(path.Dir(configpath))
	viper.WatchConfig()
	viper.OnConfigChange(func(e fsnotify.Event) {
		fmt.Println("Config file changed:", e.Name)
	})
	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal(err)
	}

	file, err := ioutil.ReadFile(messagefile)
	if err != nil {
		log.Fatal(err)
	}

	var userids []string
	if broadcast {
		walletrpc := wallet.New(wallet.Config{
			Address: viper.GetString("monero_rpc_daemon_url"),
		})

		accounts, err := walletrpc.GetAccounts(&wallet.RequestGetAccounts{})
		if err != nil {
			log.Fatal(err)
		}
		for _, account := range accounts.SubaddressAccounts {
			split := strings.Split(account.Label, "@")
			if len(split) < 2 {
				continue
			}
			if len(split[1]) == 0 {
				continue
			}
			userids = append(userids, split[1])
		}
	}

	// prepare ZMQ socket
	requester, _ := zmq.NewSocket(zmq.REQ)
	requester.Connect(viper.GetString("rpcchannel_uri"))
	time.Sleep(time.Second)
	defer requester.Close()

	errors := 0
	success := 0

	if broadcast {
		count := viper.GetInt("BROADCAST_NOTIFICATION_INTERVAL")

		for _, userid := range userids {
			id, err := strconv.ParseInt(userid, 10, 64)
			if err != nil {
				log.Fatal(err)
			}

			// skip all zero userids
			if id == 0 {
				continue
			}

			mynotification := &monerotipbot.Notification{
				Message: string(file),
				UserID:  id,
			}

			notificationbytes, err := json.Marshal(mynotification)
			if err != nil {
				log.Fatal(err)
			}
			_, err = requester.SendBytes(notificationbytes, 0)
			if err != nil {
				log.Fatal(err)
			}

			// Wait for reply:
			reply, _ := requester.RecvBytes(0)
			err = json.Unmarshal(reply, mynotification)
			if err != nil {
				log.Fatal(err)
			}
			logrus.Infof("UserID: %d - Sent: %t - Error: %s", mynotification.UserID, mynotification.Sent, mynotification.Error)

			if mynotification.Sent {
				success++
			}
			if !mynotification.Sent {
				errors++
			}

			count--
			if count == 0 {
				time.Sleep(time.Second * 5)
				count = viper.GetInt("BROADCAST_NOTIFICATION_INTERVAL")
			}
		}

		log.Printf("Success: %d - Errors: %d", success, errors)
	} else {
		mynotification := &monerotipbot.Notification{
			Message: string(file),
			UserID:  userID,
		}

		notificationbytes, err := json.Marshal(mynotification)
		if err != nil {
			log.Fatal(err)
		}
		_, err = requester.SendBytes(notificationbytes, 0)
		if err != nil {
			log.Fatal(err)
		}

		// Wait for reply:
		reply, _ := requester.RecvBytes(0)
		err = json.Unmarshal(reply, mynotification)
		if err != nil {
			log.Fatal(err)
		}
		logrus.Infof("%#v", mynotification)
	}
}
