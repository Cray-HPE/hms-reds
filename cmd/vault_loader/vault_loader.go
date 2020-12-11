// Copyright 2019 Cray Inc. All Rights Reserved.

package main

import (
	"encoding/json"
	"fmt"
	"os"
	"time"

	"stash.us.cray.com/HMS/hms-reds/internal/model"
	securestorage "stash.us.cray.com/HMS/hms-securestorage"
)

func main() {
	var secureStorage securestorage.SecureStorage
	var credStorage *model.RedsCredStore

	defaultNodeCredentials, ok := os.LookupEnv("VAULT_REDFISH_BMC_DEFAULTS")
	if !ok {
		panic("Value not set for VAULT_REDFISH_BMC_DEFAULTS")
	}

	defaultSwitchCredentials, ok := os.LookupEnv("VAULT_REDFISH_SWITCH_DEFAULTS")
	if !ok {
		panic("Value not set for VAULT_REDFISH_SWITCH_DEFAULTS")
	}

	// Setup Vault. It's kind of a big deal, so we'll wait forever for this to work.
	fmt.Println("Connecting to Vault...")
	for {
		var err error
		// Start a connection to Vault
		if secureStorage, err = securestorage.NewVaultAdapter(""); err != nil {
			fmt.Printf("Unable to connect to Vault (%s)...trying again in 5 seconds.\n", err)
			time.Sleep(5 * time.Second)
		} else {
			fmt.Println("Connected to Vault")
			credStorage = model.NewRedsCredStore("secret/reds-creds", secureStorage)
			break
		}
	}

	// Node BMC defaults
	var nodeCredentials map[string]model.RedsCredentials
	err := json.Unmarshal([]byte(defaultNodeCredentials), &nodeCredentials)
	if err != nil {
		fmt.Printf("Unable to unmarshal defaults for node BMCs: %s", err)
	}

	// Uncomment the following lines if you want to debug what is getting put into Vault.
	//prettyCredentials, _ := json.MarshalIndent(nodeCredentials, "\t", "   ")
	//fmt.Printf("Loading:\n\t%s\n\n", prettyCredentials)

	err = credStorage.StoreDefaultCredentials(nodeCredentials)
	if err != nil {
		fmt.Printf("Unable to store defaults for node BMCs: %s", err)
	}


	// Switch defaults
	var switchCredentials model.SwitchCredentials
	err = json.Unmarshal([]byte(defaultSwitchCredentials), &switchCredentials)
	if err != nil {
		fmt.Printf("Unable to unmarshal defaults for switches: %s", err)
	}

	// Uncomment the following lines if you want to debug what is getting put into Vault.
	//prettyCredentials, _ = json.MarshalIndent(switchCredentials, "\t", "   ")
	//fmt.Printf("Loading:\n\t%s\n\n", prettyCredentials)

	err = credStorage.StoreDefaultSwitchCredentials(switchCredentials)
	if err != nil {
		fmt.Printf("Unable to store defaults for switches: %s", err)
	}

	fmt.Println("Done.")
}
