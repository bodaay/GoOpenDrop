package awdl

import (
	"bytes"
	"context"
	"crypto/md5"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"goopendrop/awdl/cpio"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/grandcat/zeroconf"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
)

type AWDLClient struct {
	owlInterfaceName    string
	owlInterfaceIndex   int
	owlInterfaceAddress []net.Addr
	ThumbnailPicture    []byte
	resolver            *zeroconf.Resolver
	appleroot           []byte
	cert                string
	certKey             string
	validationRecord    []byte
	serviceID           string
	airdropDeviceName   string
	airdropDeviceModel  string
}
type AWDLClientReceiver struct {
	DeviceName  string
	DeviceModel string
	DeviceIP    net.IP
	DevicePort  int
	PlistRecord *APlist
}
type SuccessBatchSent struct {
	DeviceName string
	FileTypes  string
	FileNames  []string
}

func (acr *AWDLClientReceiver) GetDeviceNameHash() string {
	hash := md5.Sum([]byte(acr.DeviceName))
	return hex.EncodeToString(hash[:])

}
func NewAWDLClient(owlInterfaceName string, appleRoot string, thumbnail string, certPath string, certKey string, validationRecordPath string, devName string, devModel string) (*AWDLClient, error) {
	awdlClient := AWDLClient{}
	awdlClient.airdropDeviceName = devName
	awdlClient.airdropDeviceModel = devModel
	// lets load all certificates
	appleRootCertFile, err := os.Open(appleRoot)
	if err != nil {
		return nil, err
	}
	awdlClient.appleroot, err = io.ReadAll(appleRootCertFile)
	if err != nil {
		return nil, err
	}

	awdlClient.ThumbnailPicture, err = os.ReadFile(thumbnail)
	if err != nil {
		log.Warn("failed to load thumbnail picture, there would no image display while asking client to accept")
	}

	awdlClient.cert = certPath
	awdlClient.certKey = certKey
	awdlClient.validationRecord, err = os.ReadFile(validationRecordPath)
	if err != nil {
		return nil, err
	}
	//lets get owl interface first

	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	owl_found := false
	for _, i := range ifaces {
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {

			// log.Printf("%d %v %v\n", i.Index, i.Name, a)
			if i.Name == owlInterfaceName {
				owl_found = true
				awdlClient.owlInterfaceIndex = i.Index
				awdlClient.owlInterfaceName = owlInterfaceName
				awdlClient.owlInterfaceAddress = append(awdlClient.owlInterfaceAddress, a)
			}
		}
	}
	if !owl_found {
		return nil, fmt.Errorf("owl Interface not running: %s", owlInterfaceName)
	}
	//intialize resolver
	interfaces := []net.Interface{
		{
			Index: awdlClient.owlInterfaceIndex,
		},
	}
	options := []zeroconf.ClientOption{
		zeroconf.SelectIPTraffic(zeroconf.IPv6),
		zeroconf.SelectIfaces(interfaces),
	}

	awdlClient.resolver, err = zeroconf.NewResolver(options...)
	if err != nil {
		return nil, err
	}
	rbytes := make([]byte, 6)
	if _, err := rand.Read(rbytes); err != nil {
		return nil, err
	}
	awdlClient.serviceID = hex.EncodeToString(rbytes)
	return &awdlClient, nil
}

func (ac *AWDLClient) StartDiscovery(receiveCH *chan *AWDLClientReceiver) {
	logger := log.New()
	entries := make(chan *zeroconf.ServiceEntry)
	go func(results <-chan *zeroconf.ServiceEntry) {
		for entry := range results {
			// log.Println(entry)
			go func(entry *zeroconf.ServiceEntry) {

				if entry.HostName == ac.airdropDeviceName+".local." { //no need to discover ourselfs. better to do this with ip, but for some reason I'm doing something wrong
					return
				}
				logger.Infof("Fouod New Device with ip: %s and hostname: %s, Trying to Discover it...", entry.AddrIPv6[0].String(), entry.HostName)
				err := sendDiscoverPostREQ(ac, entry.AddrIPv6[0], entry.Port, receiveCH, ac)
				if err != nil {
					logger.Error(err)
				}
				// log.Println(entry.TTL)
				// log.Println(entry.Text)
			}(entry)
		}
	}(entries)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err := ac.resolver.Browse(ctx, "_airdrop._tcp", "local.", entries)

	if err != nil {
		logger.Error("Failed to browse:", err.Error())
	}

	// <-ctx.Done()
	ch := make(chan os.Signal)
	signal.Notify(ch, os.Interrupt, os.Kill) // get notified of all OS signals
	sig := <-ch
	cancel()
	logger.Warn("Received signal [%v]; shutting down AWDL Client...\n", sig)
}

type askBody struct {
	SenderComputerName  string
	BundleID            string
	SenderModelName     string
	SenderID            string
	ConvertMediaFormats bool
	SenderRecordData    []byte
	Items               []string //this is for urls, I may enable it later
	Files               []askFileEntry
	FileIcon            []byte
}
type askFileEntry struct {
	FileName              string
	FileType              string
	FileBomPath           string
	FileIsDirectory       bool
	ConvertedMediaFormats bool
}

func SendAskAndUpload(receiver *AWDLClientReceiver, ackSuccess *chan *SuccessBatchSent, archive *cpio.CpioArchive, acr *AWDLClient) error {
	var buf bytes.Buffer
	// buf, _ := os.Create("test.plist")
	plistEnc := plist.NewBinaryEncoder(&buf)
	abody := askBody{}

	abody.SenderComputerName = acr.airdropDeviceName
	abody.SenderModelName = acr.airdropDeviceModel
	abody.BundleID = "com.apple.finder"
	abody.SenderID = acr.serviceID
	abody.ConvertMediaFormats = false
	abody.SenderRecordData = acr.validationRecord
	if acr.ThumbnailPicture != nil {
		abody.FileIcon = acr.ThumbnailPicture
	}
	//we have so separate the archive into groups of file base on their mime type
	// with our testing, we found out that images and videos can be sent togther
	// anything else, can only be sent if they are of same type
	AppleUTypesProcessed := make(map[string][]cpio.CpioArchiveEntry, 0)
	// First process all images and videos together
	for _, e := range archive.Entries {
		if e.IsDirectory {
			continue
		}
		if e.FileName == "TRAILER!!!" {
			continue
		}
		if strings.Contains(e.FileMime, "image/") || strings.Contains(e.FileMime, "video/") {
			AppleUTypesProcessed["images/videos"] = append(AppleUTypesProcessed["images/videos"], e)
		} else {
			AppleUTypesProcessed[e.FileAppleUType] = append(AppleUTypesProcessed[e.FileAppleUType], e)
		}
		// AppleUTypesProcessed[e.FileAppleUType] = append(AppleUTypesProcessed["images/videos"], e)
	}
	for appleutype, entries := range AppleUTypesProcessed {
		log.Infof("Asking Client %s To Accept File os Type: %s, Number of Files %d", receiver.DeviceName, appleutype, len(entries))
		abody.Files = nil //clear it
		uploadCpio := cpio.CpioArchive{}
		for _, e := range entries {
			abf := askFileEntry{}
			abf.ConvertedMediaFormats = false
			abf.FileIsDirectory = e.IsDirectory
			abf.FileType = e.FileAppleUType
			abf.FileBomPath = e.FileName
			abf.FileName = filepath.Base(e.FileName)
			abody.Files = append(abody.Files, abf)
			uploadCpio.AddEntry(&e)
		}
		buf.Reset()
		plistEnc.Encode(abody)
		cert, err := tls.LoadX509KeyPair(acr.cert, acr.certKey)
		if err != nil {
			return err
		}
		caCertPool := x509.NewCertPool()
		caCertPool.AppendCertsFromPEM(acr.appleroot)
		tr := &http.Transport{
			TLSClientConfig: &tls.Config{
				Certificates:       []tls.Certificate{cert},
				RootCAs:            caCertPool,
				InsecureSkipVerify: true,
			},
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, //disable http2
		}
		client := &http.Client{
			Timeout:   600 * time.Second,
			Transport: tr,
		}
		v6addresss := fmt.Sprintf("https://[%s", receiver.DeviceIP.String()) + "%25" + acr.owlInterfaceName + fmt.Sprintf("]:%d", receiver.DevicePort)
		url := v6addresss + "/Ask"
		// log.Println(url)

		r, err := http.NewRequest("POST", url, &buf)
		if err != nil {
			return err
		}
		// os.WriteFile("temp.plist", buf.Bytes(), os.ModePerm)
		r.Header.Add("Content-Type", "application/octet-stream")
		r.Header.Add("Connection", "keep-alive")
		r.Header.Add("Accept", "*/*")
		r.Header.Add("User-Agent", "AirDrop/1.0")
		r.Header.Add("Accept-Language", "en-us")
		r.Header.Add("Accept-Encoding", "br, gzip, deflate")
		//ioutil.WriteFile("temp.plist", buf.Bytes(), os.ModePerm)
		resp, err := client.Do(r)
		if err != nil {
			return err
		}
		r.Close = false       //must be false, to reuse the connection once we get accepted
		io.ReadAll(resp.Body) //this help clearing the connection even if we don't need the data
		resp.Body.Close()     //close it so we can reuse the same connection

		if resp.StatusCode != http.StatusOK {
			log.Errorf("Client Rejected or Failed, HTTP code: %d", resp.StatusCode)
			// return fmt.Errorf("client Declined or request timed out, HTTP Error Code: %d", resp.StatusCode)
			continue //go to next batch of files
		}

		if resp.StatusCode == 200 { //The key thing here, we need connection reuse, so we have to read all old response, dont use req.close=true. The file transfer should happen within same connection, wasted 8 hours to figure this out
			uploadurl := v6addresss + "/Upload"

			log.Infof("Client %s Accepted Airdrop Request", receiver.DeviceName)
			uploadedgzipped, err := uploadCpio.WriteByteArchive(true)
			if err != nil {
				return err
			}
			buf := bytes.NewReader(uploadedgzipped)
			buf.Seek(0, 0)
			uploadreq, err := http.NewRequest("POST", uploadurl, buf)
			if err != nil {
				return err
			}
			uploadreq.Header.Add("Content-Type", "application/x-cpio")
			uploadreq.Header.Add("Connection", "keep-alive")
			uploadreq.Header.Add("Accept", "*/*")
			uploadreq.Header.Add("User-Agent", "AirDrop/1.0")
			uploadreq.Header.Add("Accept-Language", "en-us")
			uploadreq.Header.Add("Accept-Encoding", "br, gzip, deflate")
			resp, err := client.Do(uploadreq)
			if err != nil {
				return err
			}
			if resp.StatusCode != http.StatusOK {
				return fmt.Errorf("failed To Upload, HTTP Status Code: %d", resp.StatusCode)
			}
			ioutil.ReadAll(resp.Body) //this help clearing the connection even if we don't need the data
			resp.Body.Close()
			if resp.StatusCode != 200 {
				return fmt.Errorf("failed to upload to client:%s", receiver.DeviceName)
			}
			//all good, notify main to send acknowledgments
			success := SuccessBatchSent{}
			success.DeviceName = receiver.DeviceName
			success.FileTypes = appleutype
			for _, e := range entries {
				success.FileNames = append(success.FileNames, e.FileName)
			}
			*ackSuccess <- &success
		}
	}

	return nil
}

func sendDiscoverPostREQ(awdlClient *AWDLClient, deviceAddress net.IP, devicePort int, mainChannelDisocveredDevice *chan *AWDLClientReceiver, acr *AWDLClient) error {
	cert, err := tls.LoadX509KeyPair(acr.cert, acr.certKey)
	if err != nil {
		return err
	}
	caCertPool := x509.NewCertPool()
	caCertPool.AppendCertsFromPEM(acr.appleroot)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{
			Certificates:       []tls.Certificate{cert},
			RootCAs:            caCertPool,
			InsecureSkipVerify: true,
		},
		TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{}, //disable http2
	}

	client := &http.Client{
		Timeout:   60 * time.Second,
		Transport: tr,
	}

	// http request
	v6addresss := fmt.Sprintf("https://[%s", deviceAddress.String()) + "%25" + awdlClient.owlInterfaceName + fmt.Sprintf("]:%d", devicePort)
	url := v6addresss + "/Discover"
	// log.Println(url)
	var b bytes.Buffer
	encoder := plist.NewBinaryEncoder(&b)
	encoder.Encode(map[string][]byte{"SenderRecordData": awdlClient.validationRecord})
	r, err := http.NewRequest("POST", url, &b)
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/octet-stream")
	r.Header.Add("Connection", "keep-alive")
	r.Header.Add("Accept", "*/*")
	r.Header.Add("User-Agent", "AirDrop/1.0")
	r.Header.Add("Accept-Language", "en-us")
	r.Header.Add("Accept-Encoding", "br, gzip, deflate")
	r.Close = true //for discovery, better to close once we read the response, no connection reuse
	resp, err := client.Do(r)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to discover device")
	}
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return err
	}
	//ioutil.WriteFile("received.plist", body, os.ModePerm)
	rrr := bytes.NewReader(body)
	// ioutil.WriteFile("test.bin", body, os.ModePerm)
	// decoder := plist.NewDecoder(rrr)
	//
	d := plist.NewDecoder(rrr)
	var data interface{}
	// var aplist APlist
	d.Decode(&data)
	if err != nil {
		return err
	}
	if data == nil {
		return fmt.Errorf("bad Client!!, Really Bad")
	}
	aplist := APlist{}
	for k, v := range data.(map[string]interface{}) {
		if k == "ReceiverComputerName" {
			aplist.ReceiverComputerName = v.(string)
		} else if k == "ReceiverModelName" {
			aplist.ReceiverModelName = v.(string)
		} else if k == "ReceiverMediaCapabilities" {
			aplist.ReceiverMediaCapabilitiesBytes = v.([]byte)
		}

	}
	err = json.Unmarshal(aplist.ReceiverMediaCapabilitiesBytes, &aplist.ReceiverMediaCapabilitiesRecords)
	if err != nil {
		return err
	}
	newAWDLReciver := AWDLClientReceiver{}
	newAWDLReciver.DeviceName = strings.Trim(aplist.ReceiverComputerName, " \r\n")
	newAWDLReciver.DeviceModel = aplist.ReceiverModelName
	newAWDLReciver.DeviceIP = deviceAddress
	newAWDLReciver.DevicePort = devicePort
	newAWDLReciver.PlistRecord = &aplist
	*mainChannelDisocveredDevice <- &newAWDLReciver // pass it to main
	//up to here all good, time to actually send back to main that we discovered new device
	return nil
}
