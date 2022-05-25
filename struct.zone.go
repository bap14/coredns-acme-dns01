package main

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type ZoneFile struct {
	Value             string
	ZoneFileDirectory string

	debug       bool
	nsarecordip string
	record      string
	zoneFile    fs.FileInfo
}

func NewZoneFile() ZoneFile {
	zf := ZoneFile{}
	zf.ZoneFileDirectory = "/coredns/zones.d"
	return zf
}

func (zf *ZoneFile) AddRecord() (err error) {
	if zf.debug {
		fmt.Printf("Adding record '%s'...\n", zf.record)
	}

	err = zf.FindZoneFile()
	if err != nil {
		return err
	}

	err = zf.insertOrUpdateRecord()
	if err != nil {
		return err
	}

	err = zf.updateZoneSerial()
	if err != nil {
		return err
	}

	return nil
}

func (zf *ZoneFile) FindZoneFile(args ...bool) (err error) {
	createIfMissing := true
	if len(args) == 1 {
		createIfMissing = args[0]
	}

	domain := zf.shiftDomain(zf.record)

	if zf.debug {
		fmt.Printf("Looking for zone file '%s'...\n", domain)
	}

	files, err := ioutil.ReadDir(zf.ZoneFileDirectory)
	if err != nil {
		log.Fatalf("Unable to read directory: '%s'\n", zf.ZoneFileDirectory)
	}

	matchedFile, err := zf.searchZoneFilesForOrigin(domain, files)
	if err != nil {
		limit := strings.Count(domain, ".") - 1
		isLocalTLD, err := zf.isLocalTLD(domain)
		if err == nil && isLocalTLD {
			limit++
		}
		for i := 0; i < limit; i++ {
			previousDomain := domain
			domain = zf.shiftDomain(domain)
			if zf.debug {
				fmt.Printf("Looking for zone file '%s'...\n", domain)
			}

			if strings.Compare(previousDomain, domain) == 0 {
				break
			}

			matchedFile, _ = zf.searchZoneFilesForOrigin(domain, files)
			if matchedFile != nil {
				break
			}
		}

		if matchedFile == nil && createIfMissing {
			err = zf.createZoneFile(domain)
			if err != nil {
				log.Fatalf("Failed to create zone file '%s': %s\n", domain, err)
			}
			files, _ = ioutil.ReadDir(zf.ZoneFileDirectory)
			for _, file := range files {
				if file.Name() == "db."+domain {
					matchedFile = file
					break
				}
			}
		}
	}

	if matchedFile != nil {
		zf.zoneFile = matchedFile
		return nil
	}

	return errors.New("unable to find existing zone file")
}

func (zf *ZoneFile) RemoveRecord() (err error) {
	if zf.debug {
		fmt.Printf("Removing record '%s'...\n", zf.record)
	}

	err = zf.FindZoneFile(false)
	if err != nil {
		return err
	}

	err = zf.cleanupRecord()
	if err != nil {
		return err
	}

	err = zf.updateZoneSerial()
	if err != nil {
		return err
	}

	return err
}

func (zf *ZoneFile) SetDebug(flag bool) {
	zf.debug = flag
}

func (zf *ZoneFile) SetNSARecordIP(ip string) (err error) {
	ipAddr := net.ParseIP(ip)
	if ipAddr == nil {
		return errors.New("invalid IPv4 Address provided")
	}
	ipAddr = ipAddr.To4()
	zf.nsarecordip = ipAddr.String()
	return nil
}

func (zf *ZoneFile) SetRecordName(record string) {
	zf.record = strings.TrimRight(record, ".")
}

func (zf *ZoneFile) buildOriginRegex(domain string) (re *regexp.Regexp) {
	return regexp.MustCompile("^" + regexp.QuoteMeta(fmt.Sprintf("$ORIGIN %s.", domain)))
}

func (zf *ZoneFile) cleanupRecord() (err error) {
	fileName := path.Join(zf.ZoneFileDirectory, zf.zoneFile.Name())
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	baseDomain := strings.Replace(zf.zoneFile.Name(), "db.", "", 1)
	record := strings.Replace(zf.record, "."+baseDomain, "", 1)
	recordFound := false

	lines := strings.Split(string(contents), "\n")
	var newLines []string
	for _, line := range lines {
		if strings.Contains(line, record+" IN TXT") {
			recordFound = true
			continue
		}

		newLines = append(newLines, line)
	}

	if !recordFound {
		return errors.New("unable to find existing record")
	}

	newContents := strings.Join(newLines, "\n")
	err = ioutil.WriteFile(fileName, []byte(newContents), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (zf *ZoneFile) createZoneFile(domain string) (err error) {
	template := "$TTL 60\n"
	template += fmt.Sprintf("$ORIGIN %s.\n", domain)
	template += fmt.Sprintf("@        3600 IN SOA a.ns.%s. devenv.%s. (\n", domain, domain)
	template += fmt.Sprintf("                     %s00    ; serial\n", time.Now().Format("20060102"))
	template += "                     86400         ; refresh\n"
	template += "                     3600          ; retry\n"
	template += "                     604800        ; expire\n"
	template += "                     86400         ; expire\n"
	template += "                     )\n"
	template += fmt.Sprintf("         3600 IN NS   a.ns.%s.\n", domain)
	template += fmt.Sprintf("         3600 IN NS   b.ns.%s.\n", domain)
	template += fmt.Sprintf("a.ns          IN A    %s\n", zf.nsarecordip)
	template += fmt.Sprintf("b.ns          IN A    %s\n", zf.nsarecordip)

	filePath := path.Join(zf.ZoneFileDirectory, "db."+domain)
	err = ioutil.WriteFile(filePath, []byte(template), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (zf *ZoneFile) generateRecordLine(record string) string {
	return fmt.Sprintf("%s IN TXT \"%s\"", record, zf.Value)
}

func (zf *ZoneFile) insertOrUpdateRecord() (err error) {
	fileName := path.Join(zf.ZoneFileDirectory, zf.zoneFile.Name())
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		return err
	}

	baseDomain := strings.Replace(zf.zoneFile.Name(), "db.", "", 1)
	record := strings.Replace(zf.record, "."+baseDomain, "", 1)
	recordUpdated := false

	lines := strings.Split(string(contents), "\n")
	for n, line := range lines {
		if strings.Contains(line, record) {
			lines[n] = zf.generateRecordLine(record)
			recordUpdated = true
		}
	}

	if !recordUpdated {
		lines = append(lines, zf.generateRecordLine(record))
	}

	newContents := strings.Join(lines, "\n")
	err = ioutil.WriteFile(fileName, []byte(newContents), 0644)
	if err != nil {
		return err
	}

	return nil
}

func (zf *ZoneFile) isLocalTLD(domain string) (result bool, err error) {
	re := regexp.MustCompile("\\.lan[0-9]*$")
	result, err = regexp.Match(re.String(), []byte(domain))
	return result, err
}

func (zf *ZoneFile) searchZoneFilesForOrigin(domain string, files []fs.FileInfo) (matchedFile fs.FileInfo, err error) {
	var scanner *bufio.Scanner

	re := zf.buildOriginRegex(domain)

	for _, file := range files {
		f, err := os.Open(path.Join(zf.ZoneFileDirectory, file.Name()))
		if err != nil {
			if zf.debug {
				log.Printf("Failed to open file '%s': %s", file.Name(), err.Error())
			}
			continue
		}
		scanner = bufio.NewScanner(f)
		for scanner.Scan() {
			line := scanner.Text()
			if strings.Contains(line, "$ORIGIN") {
				if zf.debug {
					fmt.Printf("Checking resource record line '%s'\n", line)
				}
				match := re.Match(scanner.Bytes())
				if match {
					_ = f.Close()
					return file, nil
				}
				break
			}
		}
		_ = f.Close()
	}

	return nil, errors.New("search returned no results")
}

func (zf *ZoneFile) shiftDomain(domain string) (value string) {
	parts := strings.Split(domain, ".")
	parts = parts[1:]
	value = strings.Join(parts, ".")
	return value
}

func (zf *ZoneFile) updateZoneSerial() (err error) {
	zoneFile := path.Join(zf.ZoneFileDirectory, zf.zoneFile.Name())
	contents, err := ioutil.ReadFile(zoneFile)
	if err != nil {
		return err
	}

	lines := strings.Split(string(contents), "\n")
	re := regexp.MustCompile("^(?P<prefix>[ \t]*)(?P<serial>[0-9]+)[ \t]*;[ \t]*serial")
	for n, line := range lines {
		matched := re.FindStringSubmatch(line)
		if len(matched) == 0 || len(matched[1]) == 0 {
			continue
		}

		serial, err := strconv.ParseInt(matched[2], 10, 64)
		if err != nil {
			return err
		}

		serial++
		lines[n] = fmt.Sprintf("%s%s ; serial", matched[1], strconv.FormatInt(serial, 10))
		break
	}

	newContents := strings.Join(lines, "\n")
	err = ioutil.WriteFile(zoneFile, []byte(newContents), 0644)
	if err != nil {
		return err
	}

	return nil
}
