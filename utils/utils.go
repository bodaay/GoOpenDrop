package utils

import (
	"log"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
)

// Credit to https://stackoverflow.com/a/73864967
func ChownRecursively(root string, OwnerName string) error {
	userid, err := user.Lookup(OwnerName)
	if err != nil {
		return err
	}
	uVar, err := strconv.Atoi(userid.Uid)
	if err != nil {
		return err
	}
	gVar, err := strconv.Atoi(userid.Gid)
	if err != nil {
		return err
	}
	err = filepath.Walk(root,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			err = os.Chown(path, uVar, gVar)
			if err != nil {
				return err
			} else {
				// fmt.Printf("File ownership of %s changed.\n", path)
			}
			return nil
		})
	if err != nil {
		return err
	}
	return nil
}

func IsRoot() bool {
	currentUser, err := user.Current()
	if err != nil {
		log.Fatalf("[isRoot] Unable to get current user: %s", err)
	}
	return currentUser.Username == "root"
}
