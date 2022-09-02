package main

import (
	"fmt"

	"EnclaveLauncher/launcher"

	log "github.com/sirupsen/logrus"
)

func main() {
	client, instanceID, err := launcher.SetupInstance("marlin-one")
	if err != nil {
		log.Error("SetupInstance eror: ", err)
		return
	}

	output, err := launcher.RunEnclave(client)
	if err != nil {
		log.Error(err)
		return
	} else {
		fmt.Println(output)
	}

	err = launcher.TearDown(client, *instanceID, "marlin-one")
	if err != nil {
		log.Error(err)
		return
	}
}