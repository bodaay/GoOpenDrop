# GoOpenDrop
a Go Implementation and Enhancement of the Awesome work of seemoo-lab: https://github.com/seemoo-lab/opendrop



I'll Update this README file. THIS PROJECT IN Alpha Stage, use it only as POC



Main features:

* All in one solution, Send/Receive Multiple files and of any type, it will automatically group supported files together, Grouped files will be sent one shot together. For Example, Photos/Videos one group, Then any Apple UType files of same together

* Support for BLE 5.0 since its using BLUEZ with d-bus interface

* It will handle all BLE related work, by sending proper Airdrop beacon to advertise, and by capturing BLE beacons to start broadcasting on Zeroconf network

* built in pure go cpio archive handling, written from scratch for this project




Required APT Packages:
```
sudo apt install libpcap0.8 libev4 bluez
```

I've included Precompiled OWL Binaries, you can use it, or build it from source:

https://github.com/seemoo-lab/owl



You must Do the extraction Step of Keys, this will give the ability to have a verified contact (accept from contact in airdrop settings, and it will show a fixed thumbnail image of type JPEG2000)


Extracted Keys you can put them in "Keys" Folder

Follow this rpeo for extraction:

https://github.com/seemoo-lab/airdrop-keychain-extractor


Verified /Ask Request:

![alt text](https://github.com/bodaay/GoOpenDrop/blob/master/verified.png?raw=true)


in config.json:

make sure you modify the parameter for applyid,email and match it with the same user (apple id) used for keys extraction.

save a contact in the receiving phone with an email matching, then Airdrop will accept request from GoOpenDrop even if its in "contacts only" mode

Add the local username to set the files owner to, since I still cannot figure out how to run this app properly without sudo permission

Sending/Receving Files:

To Send Files, Create a folder in "OUTBOX" with the device name of the receiver, just drop any files there, they will be sent once the device is discovered.

Receving, GoOpenDrop will accept any file and will create a folder in INBOX with the device name of the sender. You can customize the function in which it will accept or reject:

maing.go
```
func checkSender(name string) bool {
	// ADD INTEGRATION HERE IF NEEDED, You can call your own API server to accept or reject /Ask request //
	return true // uncomment this to accept from Everyone

}
```
Tested On:
Wifi Module: Alfa AWUS036ACM

For compatible Wifi Modules with Active Monitoring, check this awesome shortlist:

https://github.com/morrownr/USB-WiFi/blob/main/home/The_Short_List.md
