package main

import (
	"encoding/json"
	"goopendrop/awdl"
	"goopendrop/awdl/cpio"
	"goopendrop/ble"
	"goopendrop/owl"
	"goopendrop/utils"
	"io/ioutil"
	"log"
	"os"
	"os/signal"
	"path"
	"path/filepath"

	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

type DiscoveredDevices struct {
	sync.Mutex
	Devices []awdl.AWDLClientReceiver
}

func main() {
	if !utils.IsRoot() {
		log.Fatal("Currently, this app only works if running as root, sorry...")
	}
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

	//check inbox and outbox folders
	err = os.MkdirAll(config.InboxFolder, os.ModePerm)
	if err != nil {
		panic(err)
	}
	err = os.MkdirAll(config.OutboxFolder, os.ModePerm)
	if err != nil {
		panic(err)
	} //Initiate Logger
	log := logrus.New()
	log.SetOutput(os.Stdout)
	log.Infof("Config File: %s\n", configfile)
	log.Infof("Using Wifi Interface: %s", config.OwlWlanDeviceName)
	///////////////////////////////////////////////   Making sure Owl, BLE, Gin Server Are running Before Processding //////////////////////////////////
	//Run Owl Interface
	go owl.StartOwlInterface(config.OwlWlanDeviceName, config.OwlChannel, config.OwlInterfaceName, log)
	//for loop to wait for owl interface to be ready
	log.Warnf("Waiting for Owl Interface (%s) to Start", config.OwlInterfaceName)
	for {
		ready, err := owl.OwlIsReady(config.OwlInterfaceName)
		if err != nil {
			log.Error(err)
		}
		if ready {
			break
		}
		time.Sleep(500 * time.Millisecond)
	}

	//Restart BLE
	receivedAiroDropBeacon := make(chan *ble.ReceivedBLEBeacon)
	ble.RestartBluetooth()
	//Initial BLE AirDrop BLE Beacon Broadcasts, we are not leaving until these two are working
	//Below code for BLE is shit,but kind of stable
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

	awdlServer, err := awdl.NewAWDLServer(config.OwlInterfaceName, config.AppleRootCertFile, config.ExteractedCert, config.ExtractedCertKey, config.ExtractedValidationRecord, config.AirdropServerHostname, config.AirdropServerModel, config.AirdropServerPort)
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
	receivedFile := make(chan *awdl.AirdropReceivedFile)
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
	awdlClient, err := awdl.NewAWDLClient(config.OwlInterfaceName, config.AppleRootCertFile, config.ThumbnalJP2, config.ExteractedCert, config.ExtractedCertKey, config.ExtractedValidationRecord, config.AirdropServerHostname, config.AirdropServerModel)
	if err != nil {
		panic(err)
	}
	receiverChannel := make(chan *awdl.AWDLClientReceiver)
	ackChannel := make(chan *awdl.SuccessBatchSent)
	go awdlClient.StartDiscovery(&receiverChannel)
	discoverdevices := DiscoveredDevices{}

	///////////////////////////////////////////////   Receeived EVENTs Handlers ///////////////////////////////////////////////////////////////////////
	// Successful Transfer Acknowledgment

	go func() {
		for {
			succBatch := <-ackChannel
			log.Infof("Uploaded successfuly to client: %s file of type %s. Number of Files: %d", succBatch.DeviceName, succBatch.FileTypes, len(succBatch.FileNames))
			// ADD INTEGRATION HERE IF NEEDED, for Example you can inform a server of the successful upload //
			//
			rmpath := path.Join(config.OutboxFolder, succBatch.DeviceName)
			//below just delete content, not the folder itself, thanks to: https://stackoverflow.com/a/52685448
			dir, err := ioutil.ReadDir(rmpath)
			if err != nil {
				log.Error(err)
				continue
			}
			for _, d := range dir {
				os.RemoveAll(path.Join([]string{rmpath, d.Name()}...))
			}
		}
	}()
	//Main routines to Handle Devices Discovery and Auto Uploading Files
	go func() {
		for {
			detectedClient := <-receiverChannel
			discoverdevices.Lock()
			deviceExists := false
			for _, d := range discoverdevices.Devices {
				if d.DeviceName == detectedClient.DeviceName {
					d.DeviceIP = detectedClient.DeviceIP
					d.DevicePort = detectedClient.DevicePort
					d.PlistRecord = detectedClient.PlistRecord
					deviceExists = true
					break
				}
			}

			if !deviceExists {
				discoverdevices.Devices = append(discoverdevices.Devices, *detectedClient)
				log.Infof("Adding New Discovered Device: %s, Model: %s", detectedClient.DeviceName, detectedClient.DeviceModel)

			} else {
				log.Warnf("Updated Existing Found Device: %s", detectedClient.DeviceName)
			}
			discoverdevices.Unlock() //TODO: check if its safe to unlock it at this point
			go func() {
				// ADD INTEGRATION HERE IF NEEDED, for Example you check if there is available file from you Server, download it and Upload it right away //
				userPath := path.Join(config.OutboxFolder, detectedClient.DeviceName)
				_, err := os.Stat(userPath)
				if err != nil {
					log.Infof("No Files Pending for user: %s", detectedClient.DeviceName)
					return
				}
				cpioUpload := cpio.CpioArchive{}
				cpioUpload.AddAllInFolder(userPath, true)
				//Check if the arvice has valid files
				validFiles := 0
				for _, e := range cpioUpload.Entries {
					if e.IsDirectory {
						continue
					}
					validFiles += 1
				}
				if validFiles == 0 {
					return
				}
				err = awdl.SendAskAndUpload(detectedClient, &ackChannel, &cpioUpload, awdlClient)
				if err != nil {
					log.Error(err)
				}

			}()

		}
	}()

	//Receive File Handler Routine
	go func() {
		for {

			rf := <-receivedFile
			log.Infof("Received New File from: %s, with IP: %s. File size: %d", rf.DeviceName, rf.DeviceNameIP, len(rf.Data))

			go func() { //TODO: This has too many security Flaws, for POC its fine
				randomUUid := uuid.New()
				extractDestination := path.Join(config.InboxFolder, rf.DeviceName, randomUUid.String())
				os.MkdirAll(extractDestination, os.ModePerm)
				cpiodownloaded, err := cpio.LoadCpioArchiveBytes(rf.Data)
				if err != nil {
					log.Error(err)
					return
				}
				for _, e := range cpiodownloaded.Entries {
					fpath := path.Join(extractDestination, filepath.Base(e.FileName))
					if e.IsDirectory {
						continue
					}
					if strings.HasPrefix(filepath.Base(e.FileName), ".") {
						continue //skip weird . files
					}
					if e.FileName == "TRAILER!!!" {
						continue
					}
					os.WriteFile(fpath, e.FileContent, os.ModePerm)
					utils.ChownRecursively(extractDestination, config.DownloadedFilesOwner)
				}
			}()

		}

	}()

	///////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////////

	//we will stop at the with os signals
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM) // get notified of all OS signals
	sig := <-ch
	owl.KillOwl()
	log.Infof("Received signal [%v]; shutting down...\n", sig)
	//reset bluetooth adapter
	ble.RestartBluetooth()
}

func checkSender(name string) bool {
	// ADD INTEGRATION HERE IF NEEDED, You can call your own API server to accept or reject /Ask request //
	return true // uncomment this to accept from Everyone

}
