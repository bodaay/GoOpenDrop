package main

type AppConfig struct {
	InboxFolder               string `json:"inbox_folder"`
	OutboxFolder              string `json:"outbox_folder"`
	OwlWlanDeviceName         string `json:"owl_wlan_dev_name"`
	OwlChannel                string `json:"owl_channel_6_44_149"`
	OwlInterfaceName          string `json:"owl_interface_name"`
	ThumbnalJP2               string `json:"thumbnail_picture_jp2"`
	BLEDevice                 string `json:"ble_device"`
	AirdropAppleID            string `json:"airdrop_appleid"`
	AirdropEmail              string `json:"airdrop_email"`
	AirdropPhone              string `json:"airdrop_phone"`
	AirdropServerHostname     string `json:"airdrop_server_hostname"`
	AirdropServerModel        string `json:"airdrop_server_model"`
	AirdropServerPort         int    `json:"airdrop_server_port"`
	AppleRootCertFile         string `json:"apple_root_cert"`
	ExteractedCert            string `json:"extracted_certififcate"`
	ExtractedCertKey          string `json:"extracted_certkey"`
	ExtractedValidationRecord string `json:"extracted_validation_recoed"`
	DownloadedFilesOwner      string `json:"os_downloadedfiles_owner"`
}
