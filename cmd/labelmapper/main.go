// LabelMapper is a helper to save the current structure of the wallet into a file.
// Labels are wallet-bound. Means they are not on the blockchain.
// That implies that we will lose all labels whenever the wallet crashes or gets lost for whatever reasons.
// The LabelMapper helps us to make a "backup" of all labels within the wallet so that we are able to relabel a new wallet, should we ever have to do it.

// Run this scrript as a cron job and backup the produced json file to somewhere secure (ideally not on the same server where this wallet is running)

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/omani/go-monero-rpc-client/wallet"
	"github.com/spf13/viper"
)

var configpath string

func main() {
	flag.StringVar(&configpath, "c", "", "")

	flag.Parse()
	if !flag.Parsed() {
		log.Fatal("Couldn't parse cli arguments. Aborted")
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

	walletrpc := wallet.New(wallet.Config{
		Address: viper.GetString("monero_rpc_daemon_url"),
	})

	accounts, err := walletrpc.GetAccounts(&wallet.RequestGetAccounts{})
	if err != nil {
		log.Fatal(err)
	}
	file, _ := json.MarshalIndent(accounts, "", " ")

	_ = os.WriteFile("labelmap.json", file, 0644)
}
