package owl

import (
	"net"

	"github.com/go-cmd/cmd"

	"github.com/sirupsen/logrus"
)

func OwlIsReady(owlname string) (bool, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return false, err
	}
	for _, i := range ifaces {
		if err != nil {
			continue
		}
		if i.Name == owlname {
			return true, nil
		}

	}
	return false, nil
}
func KillOwl() {
	// return
	c := cmd.NewCmd("pkill", "owl")
	<-c.Start()
}

func StartOwlInterface(wlanDevName string, owlChannel string, OwlName string, logger *logrus.Logger) error {
	// return nil
	KillOwl()

	c := cmd.NewCmd("ip", "link", "set", "dev", wlanDevName, "down")
	s := <-c.Start()
	if s.Error != nil {
		return s.Error
	}
	owlcmd := cmd.NewCmd("owl", "-i", wlanDevName, "-h", OwlName, "-c", owlChannel)
	owlcmd.Stderr = nil
	owlcmd.Stdout = nil

	statusChan := owlcmd.Start() // non-blocking

	// ticker := time.NewTicker(10 * time.Millisecond)

	// // Print last line of stdout every 2s
	// go func() {
	// 	for range ticker.C {
	// 		status := owlcmd.Status()
	// 		if len(status.Stdout) > 0 {
	// 			logger.Info(status.Stdout)
	// 		}

	// 	}
	// }()

	// Block waiting for command to exit, be stopped, or be killed
	<-statusChan
	return nil
}
