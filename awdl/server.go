package awdl

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/grandcat/zeroconf"
	log "github.com/sirupsen/logrus"
	"howett.net/plist"
)

type AskVerify func(string) bool

type AirdropReceivedFile struct {
	DeviceNameIP string
	DeviceName   string
	Data         []byte
	FilesList    []string
}

type AskClientMapping map[string]string

type AWDLServer struct {
	owlInterfaceName      string
	owlInterface          net.Interface
	owlInterfaceIndex     int
	owlInterfaceAddress   []net.Addr
	ThumbnailPicture      []byte
	appleroot             []byte
	cert                  string
	certKey               string
	validationRecord      []byte
	serviceID             string
	airdropDeviceName     string
	airdropDeviceModel    string
	AskClients            AskClientMapping
	airdropServerPort     int
	DiscoverFixedResponse []byte
	AskFixedResponse      []byte
	GinRouter             *gin.Engine
	serviceRunning        bool
	serviceserver         *zeroconf.Server
}

func getRandomServiceID() string {
	rbytes := make([]byte, 6)
	if _, err := rand.Read(rbytes); err != nil {
		return ""
	}
	return hex.EncodeToString(rbytes)
}
func NewAWDLServer(owlInterfaceName string, appleRoot string, certPath string, certKey string, validationRecordPath string, devName string, devModel string, devPort int) (*AWDLServer, error) {
	awdlServer := AWDLServer{}
	// lets load all certificates
	awdlServer.airdropDeviceName = devName
	awdlServer.airdropDeviceModel = devModel
	awdlServer.airdropServerPort = devPort
	appleRootCertFile, err := os.Open(appleRoot)
	if err != nil {
		return nil, err
	}
	awdlServer.appleroot, err = ioutil.ReadAll(appleRootCertFile)
	if err != nil {
		return nil, err
	}

	awdlServer.cert = certPath
	awdlServer.certKey = certKey
	awdlServer.validationRecord, err = os.ReadFile(validationRecordPath)
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
				awdlServer.owlInterfaceIndex = i.Index
				awdlServer.owlInterfaceName = owlInterfaceName
				awdlServer.owlInterfaceAddress = append(awdlServer.owlInterfaceAddress, a)
				awdlServer.owlInterface = i
				break
			}
		}
	}
	if !owl_found {
		return nil, fmt.Errorf("owl Interface not running: %s", owlInterfaceName)
	}

	awdlServer.serviceID = getRandomServiceID()

	//Prepare Discover Fixed Response
	var b bytes.Buffer
	encoder := plist.NewBinaryEncoder(&b)

	type QuickPlist struct {
		ReceiverComputerName      string
		ReceiverModelName         string
		ReceiverMediaCapabilities []byte
		ReceiverRecordData        []byte
	}
	q := QuickPlist{
		ReceiverComputerName:      awdlServer.airdropDeviceName,
		ReceiverModelName:         awdlServer.airdropDeviceModel,
		ReceiverMediaCapabilities: []byte("{\"Version\":1}"),
		ReceiverRecordData:        awdlServer.validationRecord,
	}
	err = encoder.Encode(q)
	if err != nil {
		log.Error(err)
	}

	awdlServer.DiscoverFixedResponse = b.Bytes()

	//preapare fixed ask response
	type askResponse struct {
		ReceiverModelName    string
		ReceiverComputerName string
	}
	ar := askResponse{
		ReceiverModelName:    awdlServer.airdropDeviceModel,
		ReceiverComputerName: awdlServer.airdropDeviceName,
	}
	var ab bytes.Buffer
	encoderAsk := plist.NewBinaryEncoder(&ab)
	err = encoderAsk.Encode(ar)
	if err != nil {
		log.Error(err)
	}
	awdlServer.AskFixedResponse = b.Bytes()
	// ioutil.WriteFile("tempDis.plist", awdlServer.DiscoverFixedResponse, os.ModePerm)
	awdlServer.AskClients = make(AskClientMapping)
	return &awdlServer, nil
}

func (aws *AWDLServer) RegisterServer() {
	//flag 136
	//this is "or" of SUPPORTS_DISCOVER_MAYBE 0x80 | SUPPORTS_MIXED_TYPES 0x08
	//As Always, Thanks for the awesome work of opendrop for making things eaiser for us
	if aws.serviceRunning {
		if aws.serviceserver != nil {
			aws.serviceserver.TTL(120)
			aws.serviceserver.SetText([]string{"flags=136"}) //this will let me re-anounce the service, TODO: MUST FIX, there is a chance of race condition, we need to use mutex
		}
		return
	}
	ifaces := make([]net.Interface, 1)
	ifaces = append(ifaces, aws.owlInterface)
	aws.serviceID = getRandomServiceID()
	server, err := zeroconf.RegisterProxy(aws.serviceID, "_airdrop._tcp", "local.", aws.airdropServerPort, aws.airdropDeviceName, []string{aws.owlInterfaceAddress[0].String()[:24]}, []string{"flags=136"}, ifaces)
	if err != nil {
		log.Error(err)
	}
	aws.serviceserver = server
	aws.serviceRunning = true
	server.TTL(120)
	defer aws.serviceserver.Shutdown()

	// Clean exit.
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt, syscall.SIGTERM)
	select {
	case <-sig:
		log.Warn("shutting down airdrop service")
		aws.serviceserver.Shutdown()
		// Exit by user
	case <-time.After(time.Second * 120):
		log.Warn("Service Time out")
		aws.serviceRunning = false
		aws.serviceserver = nil
		// Exit by timeout
	}

}

func (aws *AWDLServer) StartWebServer(checkVerify AskVerify, receivedFilleChan chan *AirdropReceivedFile) error {

	// aws.EchoServer = echo.New()
	r := gin.Default()
	aws.GinRouter = r
	// aws.EchoServer.HideBanner = true
	// aws.EchoServer.Logger.SetLevel(echolog.OFF) // only print Errors
	// aws.EchoServer.Use(middleware.Logger())
	// aws.EchoServer.Use(middleware.BodyDumpWithConfig(middleware.BodyDumpConfig{
	// 	Handler: func(ctx echo.Context, b1, b2 []byte) {
	// 		log.Println(ctx.Request().RequestURI)
	// 	},
	// }))
	aws.GinRouter.POST("/Discover", func(c *gin.Context) {
		// log.Info(c.Request().Body)
		c.Request.ProtoMajor = 1
		c.Request.ProtoMinor = 1
		c.Request.Close = true

		c.Data(200, "application/octet-stream", aws.DiscoverFixedResponse)

	})
	aws.GinRouter.HEAD("/", func(c *gin.Context) {
		c.Request.ProtoMajor = 1
		c.Request.ProtoMinor = 1
		// log.Warn("Head Called")
		c.Request.Close = true
		c.Header("Connection", "close")
		c.Header("Content-Length", "0")
		c.AbortWithStatus(200)

	})

	aws.GinRouter.POST("/Ask", func(c *gin.Context) {
		// c.Response().Header().Set("")
		c.Request.ProtoMajor = 1
		c.Request.ProtoMinor = 1
		body := c.Request.Body
		if body != nil {
			bodybytes, err := ioutil.ReadAll(body)
			defer body.Close()
			if err != nil {
				log.Error(err)
				c.AbortWithStatus(401)
			}
			// ioutil.WriteFile("ask.plist", bodybytes, os.ModePerm)
			b := bytes.NewReader(bodybytes)
			decoder := plist.NewDecoder(b)
			var data interface{}
			decoder.Decode(&data)
			sender := ""
			for k, v := range data.(map[string]interface{}) {
				if k == "SenderComputerName" {

					sender = v.(string)
					break
				}
			}

			if sender != "" {
				if checkVerify(sender) { // if he is allowed to send us
					aws.AskClients[c.ClientIP()] = sender
					log.Infof("Accepted File Transfer Request From: %s", sender)
					c.Data(200, "application/octet-stream", aws.AskFixedResponse)
					return
				}
			}

		}
		c.AbortWithStatus(401)

	})
	aws.GinRouter.POST("/Upload", func(c *gin.Context) {
		c.Request.ProtoMajor = 1
		c.Request.ProtoMinor = 1

		body := c.Request.Body

		c.Request.Close = true
		// log.Warn(c.Request.Proto)
		defer body.Close()
		if val, ok := aws.AskClients[c.ClientIP()]; ok {
			log.Infof("Downlading File From: %s", val)

			dataReceived, err := io.ReadAll(body)
			// b := make([]byte, 100)
			// body.Read(b)

			if err != nil {
				c.AbortWithStatus(500)
			}
			newFR := AirdropReceivedFile{
				DeviceNameIP: c.ClientIP(),
				DeviceName:   val,
				Data:         dataReceived,
			}
			// ioutil.WriteFile("receivedData.cpio.gz", dataReceived, os.ModePerm)
			receivedFilleChan <- &newFR
			c.Header("Connection", "close")
			c.Header("Content-Length", "0")
			c.AbortWithStatus(200)
			return
		}
		c.Header("Connection", "close")
		c.Header("Content-Length", "0")
		c.AbortWithStatus(401)

	})
	//I want it to be listening only via ipv6 on owl interface
	v6addresss := fmt.Sprintf("[%s", aws.owlInterfaceAddress[0].String()[:24]) + "%" + aws.owlInterfaceName + fmt.Sprintf("]:%d", aws.airdropServerPort)
	// log.Println(v6addresss)
	// aws.HttpServer = &http.Server{
	// 	Addr:    v6addresss,
	// 	Handler: aws.GinRouter, // set Echo as handler
	// 	// TLSConfig: &tls.Config{
	// 	// 	//MinVersion: 1, // customize TLS configuration
	// 	// },

	// 	// 	ReadTimeout:  15 * time.Second, // use custom timeouts
	// 	// 	WriteTimeout: 15 * time.Second,
	// }

	err := aws.GinRouter.RunTLS(v6addresss, aws.cert, aws.certKey)
	if err != nil {
		return err
	}

	return nil
}
