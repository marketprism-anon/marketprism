package main

import (
	"os"
	"os/exec"
	"time"

	"github.com/chainBoostScale/ChainBoost/onet/log"
)

func main() {
	result := make(chan string)
	var err error
	wait := 60 * time.Minute
	go func() {
		cmd := exec.Command("cd remote; ./users -kill=false")
		if err = cmd.Run(); err != nil {
			log.Lvl1("Couldn't run the simulation:", err)
		}
		result <- "finished"
	}()
	go func() {
		select {
		case msg := <-result:
			if msg == "finished" {
				log.LLvl1("simulation finished successfully!")
			}
			log.LLvl1("Received out-of-line message", msg)
		case <-time.After(wait):
			log.LLvl1("Quitting after waiting", wait)
		}

		if err := Cleanup(); err != nil {
			log.LLvl1("Couldn't cleanup platform:", err)
		}
	}()
}

func Cleanup() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Fatal(err)
	}
	log.LLvl1(homeDir)
	err = exec.Command("pkill", "-9", "users").Run()
	if err != nil {
		log.LLvl1("Error stopping ./users:", err)
	} else {
		log.LLvl1("users on gateway are cleaned")
	}
	return err
}
