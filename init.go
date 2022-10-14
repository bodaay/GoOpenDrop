package main

import (
	"encoding/json"
	"goopendrop/awdl"
	"goopendrop/ble"
	"goopendrop/owl"
	"goopendrop/utils"
	"log"
	"os"
	"time"

	"github.com/sirupsen/logrus"
)

func initconfig() (*AppConfig, *logrus.Logger) {
	configfile := "config.json"

	if len(os.Args) > 1 {
		configfile = os.Args[1]
	}

	cfbytes, err := os.ReadFile(configfile)
	if err != nil {
		panic(err)
	}
	config := AppConfig{}
	err = json.Unmarshal(cfbytes, &config)
	if err != nil {
		panic(err)
	}
	if !utils.IsRoot() {
		log.Fatal("Currently, this app only works if running as root if dont_run_owl set to false, sorry...")
	}

	//check inbox and outbox folders
	err = os.MkdirAll(config.InboxFolder, os.ModePerm)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(config.OutboxFolder, os.ModePerm)
	if err != nil {
		panic(err)
	}
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.Infof("Config File: %s\n", configfile)
	log.Infof("Using Wifi Interface: %s", config.OwlWlanDeviceName)

	return &config, log
}

func initAllRequired(config *AppConfig, log *logrus.Logger, receiverChannel chan *awdl.AWDLClientReceiver, receivedFile chan *awdl.AirdropReceivedFile) *awdl.AWDLClient {
	///////////////////////////////////////////////   Making sure Owl, BLE, Gin Server Are running Before Processding //////////////////////////////////
	//Run Owl Interface
	go owl.StartOwlInterface(config.OwlWlanDeviceName, config.OwlChannel, config.AWDLInterfaceName, log)

	var err error
	//for loop to wait for owl interface to be ready
	log.Warnf("Waiting for AWDL Interface (%s) to Start", config.AWDLInterfaceName)
	for {
		ready, err := owl.OwlIsReady(config.AWDLInterfaceName)
		if err != nil {
			log.Error(err)
		}
		if ready {
			break
		}
		time.Sleep(1000 * time.Millisecond)
	}

	//Restart BLE
	receivedAiroDropBeacon := make(chan *ble.ReceivedBLEBeacon)
	ble.RestartBluetooth()
	//Initial BLE AirDrop BLE Beacon Broadcasts, we are not leaving until these two are working
	//Below code for BLE, its really bad, but it works
	go func() {
		RequiresRestart := false
		for {
			go func() {
				err = ble.StartBleScan(config.BLEDevice, receivedAiroDropBeacon)
				if err != nil {
					log.Println("ERROR IN BLLUETOOTH SCAN")
					RequiresRestart = true
				}
			}()
			time.Sleep(1 * time.Second)
			go func() {
				err := ble.SendAirdropBeaconBlocking(config.BLEDevice, config.AirdropAppleID, config.AirdropPhone, config.AirdropEmail)
				if err != nil {
					RequiresRestart = true
					log.Println("ERROR IN BLLUETOOTH BROADCAST")
				}
			}()
			for {
				if RequiresRestart {
					RequiresRestart = false
					ble.RestartBluetooth()
					time.Sleep(2 * time.Second)
					break
				}
				time.Sleep(1 * time.Second)
			}

		}

	}()

	awdlServer, err := awdl.NewAWDLServer(config.AWDLInterfaceName, config.AppleRootCertFile, config.ExteractedCert, config.ExtractedCertKey, config.ExtractedValidationRecord, config.AirdropServerHostname, config.AirdropServerModel, config.AirdropServerPort)
	if err != nil {
		log.Error(err)
	}
	//Routines to wait for Airdrop BLE Beacons
	go func() {
		for {
			dev := <-receivedAiroDropBeacon
			log.Infof("Airdrop Beacons Received from address=%s, with rssi=%d", dev.DeviceMac, dev.DeviceRSSI)
			go awdlServer.RegisterServer()
		}

	}()

	go func() {
		for {
			err := awdlServer.StartWebServer(checkSender, receivedFile)
			if err == nil {
				break
			}
			log.Error(err)
			time.Sleep(500 * time.Millisecond)
		}
	}()

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	///////////////////////////////////////////////   AWDL Client Serivce Browser and Discovery //////////////////////////////////////////////////////

	//owl interface take time to start, TODO: change this to proper check if interface is ready
	//Initial AWDL Client For Discovery and Sending Files
	awdlClient, err := awdl.NewAWDLClient(config.AWDLInterfaceName, config.AppleRootCertFile, config.ThumbnalJP2, config.ExteractedCert, config.ExtractedCertKey, config.ExtractedValidationRecord, config.AirdropServerHostname, config.AirdropServerModel)
	if err != nil {
		panic(err)
	}

	go awdlClient.StartDiscovery(&receiverChannel)

	return awdlClient
}
