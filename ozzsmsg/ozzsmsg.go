package ozzsmsg

import (
	"encoding/xml"
	"fmt"
	"log"
	"os"
)

type OSSWiz struct {
	IntroMessages struct {
		WelcomeMessage string `xml:"welcomemessage"`
	} `xml:"intromessages"`
	SystemMsg string `xml:"systemmsg"`
}

func Ozzsgetmsg() (*OSSWiz, error) {
	xmlPath := os.Getenv("XMLMESSAGE")
	if xmlPath == "" {
		return nil, fmt.Errorf("XMLMESSAGE environment variable not set")
	}

	data, err := os.ReadFile(xmlPath)
	if err != nil {
		return nil, err
	}

	var messages OSSWiz
	if err := xml.Unmarshal(data, &messages); err != nil {
		return nil, err
	}
	return &messages, nil
}

func xmain() {
	messages, err := Ozzsgetmsg()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Welcome Message: %s\n", messages.IntroMessages.WelcomeMessage)
	fmt.Printf("System Message: %s\n", messages.SystemMsg)
}
