package main

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"

	"github.com/gocolly/colly"
	"github.com/gocolly/colly/extensions"
	"github.com/labstack/gommon/color"
)

var arguments = struct {
	Input       string
	Output      string
	Concurrency int
	RandomUA    bool
	Verbose     bool
	GetAlbums   bool
	GetVideos   bool
	StartID     int
	StopID      int
}{}

var client = http.Client{}

var checkPre = color.Yellow("[") + color.Green("✓") + color.Yellow("]")
var tildPre = color.Yellow("[") + color.Green("~") + color.Yellow("]")
var crossPre = color.Yellow("[") + color.Red("✗") + color.Yellow("]")

func init() {
	// Disable HTTP/2: Empty TLSNextProto map
	client.Transport = http.DefaultTransport
	client.Transport.(*http.Transport).TLSNextProto =
		make(map[string]func(authority string, c *tls.Conn) http.RoundTripper)
}

func downloadEPUB(link, name, index string) (err error) {
	// Replace slash
	name = strings.Replace(name, "/", "-", -1)

	// Check if file exist
	if _, err := os.Stat(arguments.Output + "/" + index + "-" + name + ".epub"); !os.IsNotExist(err) {
		fmt.Println(checkPre +
			color.Yellow("[") +
			color.Green(index) +
			color.Yellow("]") +
			color.Green(" Already downloaded: ") +
			color.Yellow(name))
		return err
	}

	// Create output dir
	os.MkdirAll(arguments.Output+"/", os.ModePerm)

	// Fetch the data from the URL
	resp, err := client.Get(link)
	if err != nil {
		fmt.Println(crossPre+color.Red(" Unable to download the file:"), color.Yellow(err))
		text := []byte(link + "\n")
		err = ioutil.WriteFile("./error.txt", text, 0644)
		if err != nil {
			return err
		}
		return nil
	}
	defer resp.Body.Close()

	// Create picture's file
	pictureFile, err := os.Create(arguments.Output + "/" + index + "-" + name + ".epub")
	if err != nil {
		log.Println(crossPre+color.Red(" Unable to create the file:"), color.Yellow(err))
		text := []byte(link + "\n")
		err = ioutil.WriteFile("./error.txt", text, 0644)
		if err != nil {
			return err
		}
		return err
	}
	defer pictureFile.Close()

	// Write the data to the file
	_, err = io.Copy(pictureFile, resp.Body)
	if err != nil {
		log.Println(crossPre+color.Red(" Unable to write to the file:"), color.Yellow(err))
		text := []byte(link + "\n")
		err = ioutil.WriteFile("./error.txt", text, 0644)
		if err != nil {
			return err
		}
		return err
	}

	fmt.Println(checkPre +
		color.Yellow("[") +
		color.Green(index) +
		color.Yellow("]") +
		color.Green(" Downloaded: ") +
		color.Yellow(name))

	return nil
}

func scrapeBookPage(url string, index int, worker *sync.WaitGroup) {
	defer worker.Done()
	var name, epubLink string
	var links []string

	// Create collector
	c := colly.NewCollector()

	// Randomize user agent on every request
	if arguments.RandomUA == true {
		extensions.RandomUserAgent(c)
	}

	// Get book's name
	c.OnHTML("div.header", func(e *colly.HTMLElement) {
		name = e.ChildText("h1")
	})

	// Get download link
	c.OnHTML("tbody", func(e *colly.HTMLElement) {
		links = append(links, e.ChildAttrs("tr.even", "about")...)
		for _, link := range links {
			if strings.Contains(link, "epub.images") {
				epubLink = "http:" + link
			}
		}
	})

	c.Visit(url)

	if len(name) < 1 || len(epubLink) < 1 {
		return
	}

	err := downloadEPUB(epubLink, name, strconv.Itoa(index))
	if err != nil {
		return
	}
}

func main() {
	var worker sync.WaitGroup
	var count int

	// Parse arguments and fill the arguments structure
	parseArgs(os.Args)

	// Set maxIdleConnsPerHost
	client.Transport.(*http.Transport).MaxIdleConnsPerHost = arguments.Concurrency

	for index := arguments.StartID; index <= arguments.StopID; index++ {
		worker.Add(1)
		count++
		url := "http://www.gutenberg.org/ebooks/" + strconv.Itoa(index)
		go scrapeBookPage(url, index, &worker)
		if count == arguments.Concurrency {
			worker.Wait()
			count = 0
		}
	}

	worker.Wait()
}
