package ble

import (
	"crypto/sha256"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-cmd/cmd"
	"github.com/godbus/dbus/v5"
	"github.com/muka/go-bluetooth/api"
	"github.com/muka/go-bluetooth/bluez/profile/adapter"
	"github.com/muka/go-bluetooth/bluez/profile/advertising"
	"github.com/muka/go-bluetooth/bluez/profile/device"
	log "github.com/sirupsen/logrus"
)

const appleBit = 0x004C

type ReceivedBLEBeacon struct {
	DeviceMac    string
	DeviceRSSI   int16
	DataReceived []uint8
}

func RestartBluetooth() {

	c := cmd.NewCmd("modprobe", "-r", "btusb") //using modprobe is much better than rfkill block/unblock
	<-c.Start()
	time.Sleep(2500 * time.Millisecond)
	c = cmd.NewCmd("modprobe", "btusb")
	<-c.Start()
	time.Sleep(2500 * time.Millisecond)
}
func getFirstTwoBytesSha25Has(v string) []byte {

	h := sha256.New()
	h.Write([]byte(v))
	sha256 := h.Sum(nil)

	return sha256[:2]
}
func SendAirdropBeaconBlocking(device string, appleID string, email string, phone string) error {
	appleFirstTwo := getFirstTwoBytesSha25Has(appleID)
	emailFirstTwo := getFirstTwoBytesSha25Has(email)
	phoneFirstTwo := getFirstTwoBytesSha25Has(phone)
	adapterID := device
	frames := []byte{}
	extra := []byte{0x5, 0x12, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, appleFirstTwo[0], appleFirstTwo[1], phoneFirstTwo[0], phoneFirstTwo[1], emailFirstTwo[0], emailFirstTwo[1], emailFirstTwo[0], emailFirstTwo[1], 0x0}
	// extra = []byte{0x5, 0x12, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x1, 0xd8, 0x90, 0x7a, 0x89, 0xd8, 0x90, 0xd8, 0x90, 0x0}
	frames = append(frames, extra...)
	props := new(advertising.LEAdvertisement1Properties)
	props.AddManifacturerData(appleBit, frames)
	props.Type = advertising.AdvertisementTypeBroadcast
	props.LocalName = device
	props.Timeout = 0
	props.MinInterval = 200
	props.MaxInterval = 200
	cancel, err := api.ExposeAdvertisement(adapterID, props, uint32(0))
	if err != nil {
		log.Info(err)
		return err
	}
	// defer api.Exit() all Exitws I'm handling from main now

	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM) // get notified of all OS signals
	sig := <-ch
	cancel()
	log.Infof("Received signal [%v]; shutting down...\n", sig)
	return nil
}
func StartBleScan(bledevice string, receivedBeacon chan *ReceivedBLEBeacon) error {
	// defer api.Exit() all Exitws I'm handling from main now
	adapterID := bledevice

	a, err := adapter.GetAdapter(adapterID)
	if err != nil {
		log.Println("0")
		return err
	}

	log.Debug("Flush cached devices")
	err = a.FlushDevices()
	if err != nil {
		log.Println("1")
		return err
	}

	log.Debug("Start discovery")
	discovery, cancel, err := api.Discover(a, nil)
	if err != nil {
		log.Println("2")
		return err
	}

	err = a.SetDiscoveryFilter(map[string]interface{}{
		"Transport": "le",
	})
	if err != nil {
		return err
	}
	err = a.SetDiscoveryFilter(map[string]interface{}{
		"DuplicateData": true,
	}) //this thing is pretty useless
	if err != nil {
		log.Println("3")
		return err
	}

	go func() {

		for ev := range discovery {

			if ev.Type == adapter.DeviceRemoved {
				continue
			}

			dev, err := device.NewDevice1(ev.Path)
			if err != nil {
				log.Errorf("%s: %s", ev.Path, err)
				continue
			}

			if dev == nil {
				log.Errorf("%s: not found", ev.Path)
				continue
			}

			if len(dev.Properties.ManufacturerData) > 0 {

				for k, v := range dev.Properties.ManufacturerData {
					if k == 0x004c { // apple

						temp := v.(dbus.Variant) //casting this object to type dbus.variant
						valuearray := temp.Value().([]uint8)
						if valuearray[0] == 0x05 { //this is airdrop type
							recDevice := &ReceivedBLEBeacon{
								DeviceMac:    dev.Properties.Address,
								DeviceRSSI:   dev.Properties.RSSI,
								DataReceived: valuearray,
							}
							receivedBeacon <- recDevice // inform main

							//we will delete the record in 25 seconds
							go func() {
								time.Sleep(25 * time.Second)
								a.RemoveDevice(dev.Path()) //this is the only way to keep receiving the becaon from same device, since dublicatefilter-off is not working
							}()

						}
					}

				}
			}

		}

	}()

	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, syscall.SIGTERM) // get notified of all OS signals
	sig := <-ch
	cancel()
	api.Exit()
	log.Infof("Received signal [%v]; shutting down BLE SCAN...\n", sig)

	return nil
}
