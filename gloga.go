package main

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"time"
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
}

func parse_logfile(year, zone string, logfile string, callback func(Log) error) error {
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
					err = callback(prevlog)
					if err != nil {
						return fmt.Errorf("callback func got %s", err.Error())
					}
				}

				prevlog = Log{
					Stat:     string(line[1]),
					Date:     date,
					ThreadId: threadid,
					File:     string(line[4]),
					Line:     linenum,
					Msg:      string(line[6]),
				}
				gotLog = true
			} else {
				log.Printf("Got unsupport format line %s", scanner.Text())
				if gotLog {
					prevlog.Msg += scanner.Text()
					log.Printf("Added to prevlog")
				} else {
					log.Printf("Droped")
				}
			}
		}
	}
	if gotLog {
		err = callback(prevlog)
		if err != nil {
			return fmt.Errorf("callback func got %s", err.Error())
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scanner error: %v", err)
	}

	return nil
}

func handler(l Log) error {
	log.Println(l)
	return nil
}

func main() {
	now := time.Now()
	year := fmt.Sprintf("%d", now.Year())
	zone, _ := now.Zone()

	err := parse_logfile(year, zone, "g.log", handler)
	if err != nil {
		log.Println(err)
	}
}