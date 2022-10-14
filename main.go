package main

import (
	"goopendrop/awdl"
	"goopendrop/awdl/cpio"
	"goopendrop/ble"
	"goopendrop/owl"
	"goopendrop/utils"
	"os"
	"os/signal"
	"path"
	"path/filepath"

	"strings"
	"sync"
	"syscall"

	"github.com/google/uuid"
)

type DiscoveredDevices struct {
	sync.Mutex
	Devices []awdl.AWDLClientReceiver
}

func main() {
	config, log := initconfig()
	discoverdevices := DiscoveredDevices{}
	receiverChannel := make(chan *awdl.AWDLClientReceiver)
	ackChannel := make(chan *awdl.SuccessBatchSent)
	receivedFile := make(chan *awdl.AirdropReceivedFile)
	awdlClient := initAllRequired(config, log, receiverChannel, receivedFile)

	///////////////////////////////////////////////   Receeived EVENTs Handlers ///////////////////////////////////////////////////////////////////////
	// Successful Transfer Acknowledgment

	go func() {
		for {
			succBatch := <-ackChannel
			log.Infof("Uploaded successfuly to client: %s file of type %s. Number of Files: %d", succBatch.DeviceName, succBatch.FileTypes, len(succBatch.FileNames))
			// INTEGRATION POINT
			// ADD INTEGRATION HERE IF NEEDED, for Example you can inform a server of the successful upload //
			//
			rmpath := path.Join(config.OutboxFolder, succBatch.DeviceName)
			//below just delete content, not the folder itself, thanks to: https://stackoverflow.com/a/52685448
			dir, err := os.ReadDir(rmpath)
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
				// INTEGRATION POINT
				// Handle the discovered device
				// ADD INTEGRATION HERE IF NEEDED, for Example you check if there is available file from you Server, download it and Upload it right away
				// Below will just check outbox folder, if there is a folder name with same discovered device name
				// it will send all the files within that folder
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
			// INTEGRATION POINT
			// You can customized the while thing here,
			// the file received is of CPIO archive
			log.Infof("Received New File from: %s, with IP: %s. File size: %d", rf.DeviceName, rf.DeviceNameIP, len(rf.Data))

			go func() { //TODO: This has too many security Flaws, but for controlled basic usage its fine
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
	// INTEGRATION POINT
	// ADD INTEGRATION HERE IF NEEDED, You can call your own API server to accept or reject /Ask request //
	return true

}
