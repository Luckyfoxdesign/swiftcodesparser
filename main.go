package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"strconv"
	"strings"
	"time"

	greq "github.com/Luckyfoxdesign/greq"
	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Proxies []Proxy `json:"proxies"`
	SiteURL string
	DB      Database `json:"database"`
}

type Database struct {
	User     string `json:"dbUser"`
	Password string `json:"dbPassword"`
	Host     string `json:"dbHost"`
	Name     string `json:"dbName"`
}

type Proxy struct {
	User     string `json:"proxyUser"`
	Password string `json:"proxyPassword"`
	Host     string `json:"proxyHost"`
	Port     string `json:"proxyPort"`
}

type SwiftInfo struct {
	CountryName  string
	CountryId    int64
	DetailsSlice []SwiftInfoDetails
	Pages        int
}

type SwiftInfoDetails struct {
	BankOrInstitution,
	City,
	Branch,
	SwiftCodeOrBIC,
	Address,
	Postcode string
}

func main() {

	runFactory()
}

func runFactory() {
	var (
		swiftInfoChanWithIdandName chan SwiftInfo = make(chan SwiftInfo, 211)
		swiftInfoChanWithFirstData chan SwiftInfo = make(chan SwiftInfo, 211)
	)

	cfg := readConfig()

	//Open database connection
	var connectionString string = fmt.Sprintf("%s:%s@tcp(%s)/%s", cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Name)
	fmt.Println(connectionString)

	db, err := sql.Open("mysql", connectionString)
	if err != nil {
		log.Fatal("Error when sql.Open() in main(): ", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal("Error when db.Ping() in main(): ", err)
	}

	db.SetConnMaxLifetime(time.Second * 2)
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(100)
	db.SetConnMaxIdleTime(time.Second * 2)

	go getAllCountries(&cfg, db, swiftInfoChanWithIdandName)

	// Because site does have only 211 countries in total,
	// we can use non blocking buffered channel with predefined capacity
	for i := 0; i < 211; i++ {
		time.Sleep(time.Second)
		getAllSwiftCodesByCountry(<-swiftInfoChanWithIdandName, &cfg, swiftInfoChanWithFirstData)
		break
	}
	for i := 0; i < 211; i++ {
		// fmt.Printf("%+v\n", <-swiftInfoChanWithFirstData)

		sct := <-swiftInfoChanWithFirstData
		for i, v := range sct.DetailsSlice {
			extractSwiftCode(&v)
		}

		// IMPORTANT
		// on this step structure hasn't the valid swift code
		// field contains the html link element inside whom
		// placed swift code.
		break
	}
}

// Function that reads the config.json with ioutil.ReadFile()
// and returns unmarshaled json data in Config struct.
func readConfig() Config {
	var config Config

	content, err := ioutil.ReadFile("./config.json")
	if err != nil {
		log.Fatal("Error when ioutil.ReadFile() in readConfig(): ", err)
	}

	err = json.Unmarshal(content, &config)
	if err != nil {
		log.Fatal("Error during json.Unmarshal() in readConfig(): ", err)
	}
	return config
}

// Function that requests site url with proxy and execute
// functions that parses src, inserts a SwiftInfo struct in to a database
// and sends it in to the channel.
func getAllCountries(cfg *Config, db *sql.DB, swiftInfoChanWithIdandName chan SwiftInfo) {
	// Slug for the page with all countries.
	// Site URL has a slash at the end of the URL.
	// browse-by-country/

	proxyURL := returnRandomProxyString(cfg)

	src, err := greq.GetHTMLSource(cfg.SiteURL+"browse-by-country/", proxyURL)
	if err != nil {
		log.Fatal("Error when greq.GetHTMLSource() in getAllCountries(): ", err)
	}

	parseHtmlInsertCountriesNamesToDBSendStructToChan(&src, db, swiftInfoChanWithIdandName)
}

// Function that parses html presented in slice of bytes
// and execute the function that inserts founded country name
// in to the database.
// Arguments are the html in slice of bytes and sql db pointer.
func parseHtmlInsertCountriesNamesToDBSendStructToChan(src *[]byte, db *sql.DB, swiftInfoChanWithIdandName chan SwiftInfo) {
	var (
		w1, w2, w3, w4, w5 byte = 'i', 'o', 'n', 'v', '"'
		quoteCounter       uint8
		quoteStartIndex    int
		countryName        string
	)
	for i, v := range *src {
		if v == w1 && (*src)[i+1] == w2 && (*src)[i+2] == w3 && (*src)[i+4] == w4 {
			for k := i; ; k++ {
				if (*src)[k] == w5 {
					if quoteCounter > 0 {
						swiftInfoStruct := SwiftInfo{}
						countryName = strings.ToLower(string((*src)[quoteStartIndex+1 : k]))
						swiftInfoStruct.CountryName = countryName
						quoteCounter = 0
						swiftInfoStruct.CountryId = 1
						// err := insertCountryNameToDB(db, &swiftInfoStruct)
						// if err != nil {
						// 	log.Fatal("Error with insert country name in insertCountryNameToDB: ", err)
						// }

						sendStructToChannel(&swiftInfoStruct, swiftInfoChanWithIdandName)
						break
					}
					quoteCounter++
					quoteStartIndex = k
				}
			}
			break
		}
	}
}

// Function that extracts swift code from the <a> link element.
func extractSwiftCode(SwiftInfoDetailsStruct *SwiftInfoDetails) {
	var row []byte = []byte(SwiftInfoDetailsStruct.SwiftCodeOrBIC)
	for i := 10; i > 0; i-- {
		if row[i] == '>' {
			SwiftInfoDetailsStruct.SwiftCodeOrBIC = string(row[i+1:])
			break
		}
	}
}

// Function that inserts all data from SwiftInfo struct in to a specified database
func insertCountryNameToDB(db *sql.DB, swiftInfoStruct *SwiftInfo) error {
	return nil
}

// Function that sends struct with type SwiftInfo to a specific channel
// that specified in second argument.
// First agrument is a pointer to a SwiftInfo struct.
func sendStructToChannel(swiftInfoStruct *SwiftInfo, ch chan SwiftInfo) {
	ch <- *swiftInfoStruct
}

// Function returns random proxy string from the parameters
// that listed in the array in the config file.
// Function argument is a pointer to the config file constant.
func returnRandomProxyString(c *Config) string {
	var proxyIndex int

	rand.Seed(time.Now().UnixNano())
	proxyIndex = rand.Intn(len(c.Proxies))
	proxy := c.Proxies[proxyIndex]

	return returnProxyStringURL(&proxy)
}

// Function returns a formatted string.
// That string is using to connect to a proxy.
// String is constructing from User, Password, Host, Port parameters.
// Parameters are a part of a Proxy struct that is the argument for this function.
// Argument is a pointer to the Proxy struct variable.
func returnProxyStringURL(p *Proxy) string {
	return fmt.Sprintf("http://%s:%s@%s:%s", p.User, p.Password, p.Host, p.Port)
}

func getAllSwiftCodesByCountry(swiftInfoStruct SwiftInfo, cfg *Config, swiftInfoFirstDataChan chan SwiftInfo) {
	var (
		proxyURL       string = returnRandomProxyString(cfg)
		countryName    string = strings.ReplaceAll(swiftInfoStruct.CountryName, " ", "-")
		pagesNumber    int
		emptyByteSlice []byte
	)

	fmt.Println(cfg.SiteURL + countryName)

	src, _ := getSiteHtmlCode(cfg.SiteURL+countryName, proxyURL)
	getSwiftCodeInfoFromPage(cfg.SiteURL+countryName, proxyURL, &swiftInfoStruct, swiftInfoFirstDataChan, &src)

	pagesNumber = findPagesCount(&src)

	if pagesNumber > 0 {
		swiftInfoStruct.Pages = pagesNumber
		for i := 2; i <= pagesNumber; i++ {
			getSwiftCodeInfoFromPage(cfg.SiteURL+countryName+"/page/"+strconv.Itoa(i), proxyURL, &swiftInfoStruct, swiftInfoFirstDataChan, &emptyByteSlice)
		}
	}
}

func getSiteHtmlCode(siteURL, proxyURL string) ([]byte, error) {
	src, err := greq.GetHTMLSource(siteURL, proxyURL)
	if err != nil {
		log.Fatal("Error when greq.GetHTMLSource in getAllSwiftCodesByCountry(): ", err)
	}
	return src, nil
}

// Function that parses html code and search for the
// Bank or Institution, City, Branch, Swift code.
// When information will found function writes it to a SwiftInfo struct
// and sends in to a specific channel.
func getSwiftCodeInfoFromPage(siteURL, proxyURL string, swiftCodeStruct *SwiftInfo, swiftCodeChan chan SwiftInfo, src *[]byte) {
	if len(*src) == 0 {
		*src, _ = getSiteHtmlCode(siteURL, proxyURL)
	}
	var (
		firstTableIndex   int = bytes.Index(*src, []byte("<tb"))
		lastTableIndex    int = bytes.Index(*src, []byte("</tb")) // Do we really need 4th loop? !THINK
		elementData       string
		elementStartIndex int
		elementCounter    uint8
		elementsInfo      map[uint8]string = make(map[uint8]string, 5)
		details           SwiftInfoDetails
	)

	for i := firstTableIndex; i < lastTableIndex; i++ {
		// I don't know how to rewrite this complex condition and make it more easier.
		if (*src)[i] == '"' && (*src)[i+1] == '>' && (*src)[i-1] != '/' && (*src)[i-6] != 'p' && (*src)[i+5] != 'n' && (*src)[i+6] != 's' {
			elementStartIndex = i + 2
			for k := i; ; k++ {
				if (*src)[k] == '<' && (*src)[k+1] == '/' {
					elementData = string((*src)[elementStartIndex:k])

					// <ins class= it's a google ad element that inserts by js
					// we don't need this element
					if !strings.Contains(elementData, "<ins class") {
						elementsInfo[elementCounter] = elementData
					}
					break
				}
			}
			// Row with code under the comment helps with the understanding that the code/algorytm is working correctly.
			// Shows a correct/incorrect elementsData order
			// fmt.Println("ec", elementCounter, elementsInfo[elementCounter])

			elementCounter++
			if elementCounter == 5 {
				details.BankOrInstitution = elementsInfo[1]
				details.City = elementsInfo[2]
				details.Branch = elementsInfo[3]
				details.SwiftCodeOrBIC = elementsInfo[4]
				swiftCodeStruct.DetailsSlice = append(swiftCodeStruct.DetailsSlice, details)

				elementCounter = 0
			}
		}
	}
	sendStructToChannel(swiftCodeStruct, swiftCodeChan)
}

// Function that searchs for the >Last word and checking if the symbol / is before the searching word.
// Example: <a href="/china/page/54/">Last »</a>
// Don't forget that might be three elements, two of them related to the Last button
// and one element placed in the swift code description block.
// Example: <li>Last 3
func findPagesCount(src *[]byte) int {
	var (
		firstIndexForWord                int = bytes.Index(*src, []byte(">Last"))
		lastQuoteIndex, numberOfPagesInt int = firstIndexForWord - 2, 0
	)

	if (*src)[firstIndexForWord-2] == '/' {
		for i := 3; i != 6; i++ {
			if (*src)[firstIndexForWord-i] == '/' {
				numberOfPagesString := string((*src)[firstIndexForWord-i : lastQuoteIndex])
				numberOfPagesInt, _ = strconv.Atoi(numberOfPagesString)
				break
			}
		}
	}
	return numberOfPagesInt
}
