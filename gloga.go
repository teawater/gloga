package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/koding/multiconfig"
)

const supportedLineFormat = "[IWEF]mmdd hh:mm:ss.uuuuuu threadid file:line] msg"
const dateLayout = "20060102 15:04:05.999999999 MST"

type Log struct {
	Stat     string
	Date     time.Time
	ThreadId uint64
	File     string
	Line     uint64
	Msg      string
	Log      string
}

const (
	dateEarly = iota
	dateLate
	dateOK
)

func checkLogDate(t time.Time) int {
	if aConf.StartDate != (time.Time{}) && t.Before(aConf.StartDate) {
		return dateEarly
	}
	if aConf.StopDate != (time.Time{}) && t.After(aConf.StopDate) {
		return dateLate
	}
	return dateOK
}

func parseLog(year, zone string, logfile string, callback func(Log) error) error {
	fp, err := os.Open(logfile)
	if err != nil {
		return fmt.Errorf("os.Open error: %s", err.Error())
	}
	defer fp.Close()

	reLineFormat, err := regexp.Compile(`^Log line format: (.*)$`)
	if err != nil {
		return fmt.Errorf("regexp.Compile error: %s", err.Error())
	}
	reLine, err := regexp.Compile(`^([IWEF])(\d\d\d\d\s\d\d:\d\d:\d\d.\d\d\d\d\d\d)\s+(\d+)\s+(.+):(\d+)\]\s+(.*)$`)
	if err != nil {
		return fmt.Errorf("regexp.Compile error: %s", err.Error())
	}

	scanner := bufio.NewScanner(fp)
	gotLineFormat := false
	var prevlog Log
	gotLog := false
	for scanner.Scan() {
		if !gotLineFormat {
			line := reLineFormat.FindSubmatch(scanner.Bytes())
			if len(line) == 2 {
				if string(line[1]) != supportedLineFormat {
					return fmt.Errorf("The format of %s is not supported", logfile)
				}
			}
			gotLineFormat = true
		} else {
			line := reLine.FindSubmatch(scanner.Bytes())
			if len(line) == 7 {
				date, err := time.Parse(dateLayout, year+string(line[2])+" "+zone)
				if err != nil {
					log.Panicf("time.Parse %s: %s", string(line[2]), err.Error())
				}

				threadid, err := strconv.ParseUint(string(line[3]), 10, 64)
				if err != nil {
					log.Panicf("strconv.ParseUint %s: %s", string(line[3]), err.Error())
				}

				linenum, err := strconv.ParseUint(string(line[5]), 10, 64)
				if err != nil {
					log.Panicf("strconv.ParseUint %s: %s", string(line[5]), err.Error())
				}

				//Now, current line is a right log.
				if gotLog {
					res := checkLogDate(prevlog.Date)
					if res == dateOK {
						err = callback(prevlog)
						if err != nil {
							return fmt.Errorf("callback func got %s", err.Error())
						}
					}
					if res == dateLate {
						return nil
					}
				}

				prevlog = Log{
					Stat:     string(line[1]),
					Date:     date,
					ThreadId: threadid,
					File:     string(line[4]),
					Line:     linenum,
					Msg:      string(line[6]),
					Log:      scanner.Text(),
				}
				gotLog = true
			} else {
				log.Printf("Got unsupport format line %s", scanner.Text())
				if gotLog {
					prevlog.Msg += scanner.Text()
					prevlog.Log += scanner.Text()
					log.Printf("Added to prevlog")
				} else {
					log.Printf("Droped")
				}
			}
		}
	}
	if gotLog {
		if checkLogDate(prevlog.Date) == dateOK {
			err = callback(prevlog)
			if err != nil {
				return fmt.Errorf("callback func got %s", err.Error())
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %v", err)
	}

	return nil
}

func handlerKeep(l Log) error {
	for _, keep := range aConf.Keep {
		if keep.File == l.File && keep.Line == l.Line {
			fmt.Println(l.Log)
			break
		}
	}
	return nil
}

func handlerIgnores(l Log) error {
	for _, Ignore := range aConf.Ignores {
		if Ignore.File == l.File && Ignore.Line == l.Line {
			return nil
		}
	}
	fmt.Println(l.Log)
	return nil
}

type fileConf struct {
	LogDir     []string
	Keep       []Log
	Ignores    []Log
	DateFormat string
	StartDate  string
	StopDate   string
}

type Conf struct {
	LogDir    []string
	Keep      []Log
	Ignores   []Log
	StartDate time.Time
	StopDate  time.Time
}

var aConf Conf

func main() {
	argsLen := len(os.Args)
	if argsLen < 1 && argsLen > 3 {
		log.Fatalf("Usage: %s [g.toml] [g.log]", os.Args[0])
	}
	confDir := "g.toml"
	if argsLen > 1 {
		confDir = os.Args[1]
	}
	if filepath.Ext(confDir) != ".toml" {
		log.Fatalf("The file name extension of config file must be \".toml\"")
	}
	log.Printf("Try to load config from %s", confDir)
	m := multiconfig.NewWithPath(confDir)
	fconf := new(fileConf)
	m.MustLoad(fconf)

	//Convert fileconf to aConf
	if fconf.DateFormat != "" {
		var err error
		if fconf.StartDate != "" {
			aConf.StartDate, err = time.Parse(fconf.DateFormat, fconf.StartDate)
			if err != nil {
				log.Panicf("time.Parse StartDate %s: %s", fconf.StartDate, err.Error())
			}
		}
		if fconf.StopDate != "" {
			aConf.StopDate, err = time.Parse(fconf.DateFormat, fconf.StopDate)
			if err != nil {
				log.Panicf("time.Parse StopDate %s: %s", fconf.StopDate, err.Error())
			}
		}
	}
	aConf.LogDir = fconf.LogDir
	aConf.Keep = fconf.Keep
	aConf.Ignores = fconf.Ignores

	//Check Keep and Ignores
	if len(aConf.Keep) > 0 && len(aConf.Ignores) > 0 {
		log.Printf("Config Keep and Ignores cannot work together")
		return
	}
	var handler func(l Log) error
	if len(aConf.Keep) > 0 {
		handler = handlerKeep
	}
	if len(aConf.Ignores) > 0 {
		handler = handlerIgnores
	}

	//Check LogDir
	if argsLen == 3 {
		aConf.LogDir = append(aConf.LogDir, os.Args[2])
	}
	if len(aConf.LogDir) == 0 {
		aConf.LogDir = append(aConf.LogDir, "g.log")
	}

	log.Printf("Config: %+v", aConf)

	now := time.Now()
	year := fmt.Sprintf("%d", now.Year())
	zone, _ := now.Zone()

	for _, file := range aConf.LogDir {
		err := parseLog(year, zone, file, handler)
		if err != nil {
			log.Printf("Parse %s got error: %v", file, err)
		}
	}
}
