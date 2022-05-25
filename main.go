package main

import (
	"log"
	"os"
)

var zoneFile ZoneFile

func main() {
	var action string
	var err error
	var record string
	var value string

	debug := false
	nsIpAddress := "172.19.0.53"
	outputDir := "/coredns/zones.d"

	if _, exists := os.LookupEnv("COREDNS_ACME_NS"); exists {
		nsIpAddress = os.Getenv("COREDNS_ACME_NS")
	}

	if _, exists := os.LookupEnv("COREDNS_ACME_DEBUG"); exists {
		debug = true
	}

	if _, exists := os.LookupEnv("COREDNS_ACME_OUT"); exists {
		outputDir = os.Getenv("COREDNS_ACME_OUT")
	}

	action = os.Args[1]
	record = os.Args[2]
	value = os.Args[3]

	zoneFile = NewZoneFile()
	zoneFile.SetDebug(debug)
	err = zoneFile.SetNSARecordIP(nsIpAddress)
	if err != nil {
		if debug {
			log.Println(err)
		}
		os.Exit(2)
	}

	zoneFile.SetRecordName(record)
	zoneFile.Value = value

	if len(outputDir) > 0 {
		zoneFile.ZoneFileDirectory = outputDir
	}

	switch action {
	case "present":
		if debug {
			log.Printf("Received request to add/update record: '%s' with value '%s'", record, value)
		}
		err = zoneFile.AddRecord()
		break
	case "cleanup":
		if debug {
			log.Printf("Received request to cleanup record: '%s' with value '%s'", record, value)
		}
		err = zoneFile.RemoveRecord()
		break
	}

	if err != nil {
		if debug {
			log.Println(err)
		}
		os.Exit(1)
	}

	os.Exit(0)
}
