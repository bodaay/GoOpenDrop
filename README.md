# GoOpenDrop
a Go Implementation and Enhancement of the Awesome work of seemoo-lab: https://github.com/seemoo-lab/opendrop

I'll Update this README file

just quick info

Main features:

* All in one solution, Send/Receive Multiple files and of any type, it will automatically group supported files together, Grouped files will be sent one shot together. For Example, Photos/Videos one group, Then any Apple UType files of same together

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
