package main

import (
	"bufio"
	"crypto/tls"
	"flag"
	"fmt"
	"net"
	"net/url"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

func main() {

	fmt.Println("*******************************************************")
	fmt.Println("*** Simple Go Trickle Testing Tool                  ***")
	fmt.Println("*** Resilience and Performance Testing.             ***")
	fmt.Println("*** Use at your own risk.                           ***")
	fmt.Println("*** WUNDERWUZZI, September 2018, MIT License        ***")
	fmt.Println("*******************************************************")

	//Parse the command lines flags
	var testMode int
	flag.IntVar(&testMode, "TestMode", 0, "Operational mode. Default is to trickle the body of the request.")

	var workers int
	flag.IntVar(&workers, "Workers", 1, "Number of concurrent workers")

	var verb string
	flag.StringVar(&verb, "Verb", "GET", "HTTP verb to use (for HTTP tests)")

	var destination string
	flag.StringVar(&destination, "Destination", "http:://localhost:80/", "Destination URL. Port is required! Also make sure initial Path is set /")

	var protocol string
	flag.StringVar(&protocol, "Protocol", "HTTP/1.1", "HTTP Protocol")

	var headers string
	flag.StringVar(&headers, "Headers", "Host: http://localhost\nContent-Type: text/html", "Headers seperated by \n. Content-Length will automtically be added for POST/PUT")

	var body string
	flag.StringVar(&body, "Body", "MENACE - resilience and performance testing", "Payload sent to the server")

	var bodyRepeat int
	flag.IntVar(&bodyRepeat, "RepeatBody", 1, "Number of times the body will be repeated.")

	var trickleWait int
	flag.IntVar(&trickleWait, "TrickleWaitTime", 1, "Trickle wait time in seconds")

	var countdown int
	flag.IntVar(&countdown, "Countdown", 60, "Maximum of time the test is running in seconds.")

	flag.Parse()

	if testMode == 1 {
		flag.PrintDefaults()
		os.Exit(1)
	}

	destinationURL, err := url.Parse(destination)

	if err != nil {
		fmt.Println(err)
		panic("Error")
	}

	config := Configuration{}
	config.Mode = HTTPBody //figure out how to do enum/const parsing via flags
	config.Verb = strings.Trim(verb, "")
	config.Protocol = strings.Trim(protocol, "")
	config.Headers = make(map[string]int)

	tempHeaders := strings.Split(headers, "\n")
	for i := 0; i < len(tempHeaders); i++ {
		if strings.HasPrefix(tempHeaders[i], "Host: ") {
			tempHeaders[i] = "Host: " + destinationURL.Hostname()
		}
		config.Headers[tempHeaders[i]] = config.Headers[tempHeaders[i]] + 1
	}

	config.Destination = destinationURL
	config.TrickleWaitTime = trickleWait
	config.BodyTemplate = []byte(body)
	config.BodyTemplateRepeat = bodyRepeat
	config.Countdown = countdown

	if verb == "POST" || verb == "PUT" {
		contentLengthHeader := "Content-Length: " + strconv.Itoa(len(config.BodyTemplate)*config.BodyTemplateRepeat)
		config.Headers[contentLengthHeader] = 1
	}

	print(config)

	fmt.Println("Press ENTER to launch or CTRL+C to cancel")
	fmt.Scanln()

	runtime.GOMAXPROCS(4)

	var waitGroupSync sync.WaitGroup
	waitGroupSync.Add(workers)

	//emergency break - countdown
	startTime := time.Now()

	endTime := startTime.Add(time.Duration(config.Countdown) * time.Second)
	fmt.Println("End Time ", endTime)
	go monitor(endTime)

	//workers
	fmt.Println("Workers", workers)
	for i := 0; i < workers; i++ {
		go connect(i, &waitGroupSync, config)
	}

	waitGroupSync.Wait()

	//sync threads here
	fmt.Println("Complete.")
}

func monitor(endTime time.Time) {

	ticker := time.NewTicker(1000 * time.Millisecond)

	for t := range ticker.C {

		if t.Sub(endTime) > 0 {
			fmt.Println("\nCountdown expired. Shutting down... Done")
			os.Exit(0)
		}
	}
}

//TestMode ...
type TestMode int

//Enumeration for Test Modes
const (
	TCPConnections TestMode = iota + 1
	TCPBytes
	HTTPHeader
	HTTPBody
)

// // Implement the flag.Value interface
// func (mode TestMode) String() string {
//  return fmt.Sprintf("%v", mode)
// }

// // Implement the flag.Value interface
// func (mode TestMode) Set(value string) error {
//  switch(value)
//  TCPConnections
//  return nil
// }

// Configuration Settings
type Configuration struct {
	Mode               TestMode
	Verb               string
	Destination        *url.URL
	Protocol           string
	Headers            map[string]int //map of header and repeat count
	TrickleWaitTime    int
	BodyTemplate       []byte
	BodyTemplateRepeat int
	Countdown          int
}

func (config Configuration) validate() bool {

	result := false

	result = true
	return result
}

func print(config Configuration) {

	fmt.Println("Test Configuration:")
	fmt.Println("Test Mode", config.Mode)
	fmt.Println(config.Verb, config.Destination.RequestURI(), config.Protocol)
	fmt.Println("Headers", config.Headers)
	fmt.Println(string(config.BodyTemplate[:]))
	fmt.Println("Repeat:", config.BodyTemplateRepeat)
	fmt.Println("Trickle Wait Time:", config.TrickleWaitTime)

}

func getconnection(ID int, retryCount int, config Configuration) net.Conn {

	coninfo := net.JoinHostPort(config.Destination.Hostname(), config.Destination.Port())
	if config.Destination.Scheme == "https" {
		fmt.Println("Establishing TLS Connection....")
		conf := &tls.Config{
			InsecureSkipVerify: false,
			MinVersion:         tls.VersionTLS10,
		}

		conn, err := tls.Dial("tcp", coninfo, conf)
		if err != nil {
			if ID == 0 {
				fmt.Println("SSL Error : " + err.Error())
				fmt.Println(ID, "Error connecting to", config.Destination.Hostname(), "Retry", retryCount, "of 10. Waiting 10 seconds for retry...")
				time.Sleep(10 * time.Second)
			}
		}

		return conn
	}

	fmt.Println("Establishing HTTP Connection....")
	conn, err := net.Dial("tcp", coninfo)
	if err != nil {
		if ID == 0 {
			fmt.Println(ID, "Error connecting to", config.Destination.Hostname(), "Retry", retryCount, "of 10. Waiting 10 seconds for retry...")
			time.Sleep(10 * time.Second)
		}
	}
	return conn

}

func connect(ID int, waitGroupSync *sync.WaitGroup, config Configuration) {

	defer waitGroupSync.Done()

	if ID == 0 {
		fmt.Println("Debug Thread - Verbose Information")
		fmt.Println("Connecting...")
	}

	for retryCount := 0; retryCount < 10; retryCount++ {

		conn := getconnection(ID, retryCount, config)

		line := config.Verb + " " + config.Destination.Path + " " + config.Protocol + "\n"
		for v := range config.Headers {
			line = line + v + "\n"
		}

		line = line + "\n\n"
		//line = line + config.BodyTemplate //todo copy/make buffer larger config.BodyTemplateRepeat

		if ID == 0 {
			fmt.Println("Buffer to send:\n", line)
		}

		for i := 0; i < len(line); i++ {

			///use a fraction of 4 - to speed things up for testing
			time.Sleep(time.Duration(config.TrickleWaitTime) * time.Second / 10)
			b := line[i]

			_, err := conn.Write([]byte{b})
			if err != nil {
				fmt.Println(ID, "Error writing bytes to",
					config.Destination.Hostname(),
					"Retry", retryCount,
					"of 10. Waiting 10 seconds for retry...")

				time.Sleep(10 * time.Second)
				continue
			}

			if ID == 0 {
				fmt.Print(string(b))
			}
		}

		//Body
		if config.Verb == "POST" || config.Verb == "PUT" {
			for i := 0; i < len(config.BodyTemplate); i++ {
				///use a fraction of 4 - to speed things up for testing
				time.Sleep(time.Duration(config.TrickleWaitTime) * time.Second / 10)
				b := config.BodyTemplate[i]

				_, err := conn.Write([]byte{b})
				if err != nil {
					fmt.Println(ID, "Error writing bytes to",
						config.Destination.Hostname(),
						"Retry", retryCount,
						"of 10. Waiting 10 seconds for retry...")

					time.Sleep(10 * time.Second)
					continue
				}

				if ID == 0 {
					fmt.Print(string(b))
				}
			}
		}

		//awaiting result - this is for now optional
		//slow read is not yet implemented
		scanner := bufio.NewScanner(conn)
		for scanner.Scan() {
			if ID == 0 {
				fmt.Println(scanner.Text())
			}
		}

		if err := scanner.Err(); err != nil {

			fmt.Println(ID, "Error reading response.", err, config.Destination.Hostname(), "No retry for reading response.")
		}

		if ID == 0 {
			fmt.Println("Done.")
		}
	}

}
