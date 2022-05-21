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
}

type SwiftInfoDetails struct {
	BankOrInstitution,
	City,
	Branch,
	SwiftCodeOrBIC,
	Address,
	Connection,
	Postcode string
}

// Function that extracts swift code from the <a> link element.
func (SwiftInfoDetailsStruct *SwiftInfoDetails) extractSwiftCode() {
	var row []byte = []byte(SwiftInfoDetailsStruct.SwiftCodeOrBIC)
	for i := len(row) - 1; i > 0; i-- {
		if row[i] == '>' {
			SwiftInfoDetailsStruct.SwiftCodeOrBIC = strings.ToLower(string(row[i+1:]))
			break
		}
	}
}

func main() {
	runFactory()
}

func runFactory() {
	const countriesToParse = 211
	var (
		swiftInfoChanWithIdandName chan SwiftInfo = make(chan SwiftInfo, countriesToParse)
		swiftInfoChanWithFirstData chan SwiftInfo = make(chan SwiftInfo, countriesToParse)
		swiftInfoChanWithAllData   chan SwiftInfo = make(chan SwiftInfo, countriesToParse)
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

	// Befour we run the function below, we need to check
	go getAllCountries(&cfg, db, swiftInfoChanWithIdandName)

	// Because we run our app with a cron
	// we can use non blocking buffered channel with predefined capacity
	for i := 0; i < countriesToParse; i++ {
		time.Sleep(time.Second)
		getAllSwiftCodesByCountry(<-swiftInfoChanWithIdandName, &cfg, swiftInfoChanWithFirstData)
		break
	}
	for i := 0; i < countriesToParse; i++ {
		// fmt.Printf("%+v\n", <-swiftInfoChanWithFirstData)
		sct := <-swiftInfoChanWithFirstData
		// If I need to control the scraping process I need to know:
		// - how many pages were already parsed
		// - how many pages in total

		// I think I need to parse one page at a time???
		// so I need a loop with counter instead of the loop with a the range
		// or we don't need the loop at all.
		for i, v := range sct.DetailsSlice {
			if v.SwiftCodeOrBIC != "" {
				// On this step structure hasn't the valid swift code.
				// Field contains an html link element inside whom placed swift code.
				// So we need to extract this code.
				// Example: <a href="/albania/usalaltrvl2/">USALALTRVL2

				// !!!I REALLY DON'T KNOW HOW IT WORKS. BUT IT WORK.
				// DON'T FORGET ABOUT THIS PLACE, LEARN.
				// Previously I've wrote extractSwiftCode as a separate func with
				// a pointer argument to the v variable.
				// I guess it work because the v in the loop as a copy in memory not a pointer
				// so when I access child struct by the index directly from the parent struct
				// I can correctly change values for the child struct.
				sct.DetailsSlice[i].extractSwiftCode()
			}
			time.Sleep(time.Millisecond * 200)
			getSwiftCodeInfoFromPageAndWriteToExistingStruct(&cfg, i, &sct)
		}
		sendStructToChannel(&sct, swiftInfoChanWithAllData)
		break
	}
	for i := 0; i < countriesToParse; i++ {
		sct := <-swiftInfoChanWithAllData
		for i, v := range sct.DetailsSlice {
			fmt.Println(i, v)
			// write swiftInfoDetails to database
		}
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

	// TODO We need to write country id to help table

	parseHtmlAndInsertCountriesNamesToDbAndSendStructToChan(&src, db, swiftInfoChanWithIdandName)
}

// Function that parses html presented in slice of bytes
// and execute the function that inserts founded country name
// in to the database.
// Arguments are the html in slice of bytes and sql db pointer.
func parseHtmlAndInsertCountriesNamesToDbAndSendStructToChan(src *[]byte, db *sql.DB, swiftInfoChanWithIdandName chan SwiftInfo) {
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

// Function that requests site data and parses response in the html.
// On the first page we get pages total count and run loop that
// requests on each page in a loop.
// Result of this response we add to the existing sturcture that passes
// like an argument in the function and send structure to another channel.
// On the first page we get pages total count and iterate each of page.
func getAllSwiftCodesByCountry(swiftInfoStruct SwiftInfo, cfg *Config, swiftInfoFirstDataChan chan SwiftInfo) {
	var (
		proxyURL       string = returnRandomProxyString(cfg)
		countryName    string = strings.ReplaceAll(swiftInfoStruct.CountryName, " ", "-")
		pagesNumber    int
		emptyByteSlice []byte
	)

	// !!!DON'T FORGET TO REMOVE STRING BELOW
	fmt.Println(cfg.SiteURL + countryName)

	src, err := getSiteHtmlCode(cfg.SiteURL+countryName, proxyURL)
	if err != nil {
		log.Fatal("Error when getSiteHtmlCode() in the getAllSwiftCodesByCountry() with the err: ", err)
	}
	findSwiftCodeInfoInPage(cfg.SiteURL+countryName, proxyURL, &swiftInfoStruct, swiftInfoFirstDataChan, &src)

	pagesNumber = findPagesCount(&src)

	if pagesNumber > 0 {
		for i := 2; i <= pagesNumber; i++ {
			findSwiftCodeInfoInPage(cfg.SiteURL+countryName+"/page/"+strconv.Itoa(i), proxyURL, &swiftInfoStruct, swiftInfoFirstDataChan, &emptyByteSlice)
		}
	}
}

// Function that requests site url via a proxy and returns slice of bytes.
// If request has an error function returns it or returns nil.
func getSiteHtmlCode(siteURL, proxyURL string) ([]byte, error) {
	src, err := greq.GetHTMLSource(siteURL, proxyURL)
	if err != nil {
		return src, err
	}
	return src, nil
}

// Function that requests a swift code page with full information for requested swift code.
// It searching for a postcode and a connection.
// When we find them we write them to the existing SwiftInfo struct.
func getSwiftCodeInfoFromPageAndWriteToExistingStruct(cfg *Config, swiftCodeDetailsStructIndex int, swiftCodeInfoStruct *SwiftInfo) {
	url := cfg.SiteURL + swiftCodeInfoStruct.CountryName + "/" + swiftCodeInfoStruct.DetailsSlice[swiftCodeDetailsStructIndex].SwiftCodeOrBIC
	fmt.Println(url)
	src, err := getSiteHtmlCode(url, returnRandomProxyString(cfg))
	if err != nil {
		log.Fatal("Error when getSiteHtmlCode() in the getSwiftCodeInfoFromPageAndWriteToExistingStructAndSendToChan() with the err: ", err)
	}

	var (
		postCodeTitleStartIndex        int = bytes.Index(src, []byte("Addr"))
		tbodyEndIndex                  int = bytes.Index(src, []byte("</tb"))
		valueStartIndex, valueEndIndex int
		valuesSlice                    []string
	)
	for i := postCodeTitleStartIndex; i < tbodyEndIndex; i++ {
		if src[i] == 'd' && src[i+1] == '>' && src[i-2] == '<' {
			valueStartIndex = i + 2
			for k := i + 1; ; k++ {
				if src[k] == '<' {
					valueEndIndex = k
					value := string(src[valueStartIndex:valueEndIndex])
					valuesSlice = append(valuesSlice, value)
					break
				}
			}
		}
	}

	*&swiftCodeInfoStruct.DetailsSlice[swiftCodeDetailsStructIndex].Address = valuesSlice[0]
	*&swiftCodeInfoStruct.DetailsSlice[swiftCodeDetailsStructIndex].Postcode = valuesSlice[2]
	if valuesSlice[4] == "Active" {
		*&swiftCodeInfoStruct.DetailsSlice[swiftCodeDetailsStructIndex].Connection = "1"
	} else {
		*&swiftCodeInfoStruct.DetailsSlice[swiftCodeDetailsStructIndex].Connection = "0"
	}
}

// Function that parses html code and search for the
// Bank or Institution, City, Branch, Swift code.
// When information will found function writes it to a SwiftInfo struct
// and sends in to a specific channel.
func findSwiftCodeInfoInPage(siteURL, proxyURL string, swiftCodeStruct *SwiftInfo, swiftCodeChan chan SwiftInfo, src *[]byte) {
	var (
		firstTableIndex   int = bytes.Index(*src, []byte("<tb"))
		lastTableIndex    int = bytes.Index(*src, []byte("</tb")) // Do we really need 4th loop? !THINK
		elementData       string
		elementStartIndex int
		elementCounter    uint8
		elementsInfo      map[uint8]string = make(map[uint8]string, 5)
		source            []byte
		err               error
		details           SwiftInfoDetails
	)

	if len(*src) == 0 {
		source, err = getSiteHtmlCode(siteURL, proxyURL)
		if err != nil {
			log.Fatal("Error when getSiteHtmlCode() in the getSwiftCodeInfoFromPage() with the err: ", err)
		}
	} else {
		source = *src
	}

	for i := firstTableIndex; i < lastTableIndex; i++ {
		// I don't know how to rewrite this complex condition and make it more easier.
		if (source)[i] == '"' && (source)[i+1] == '>' && (source)[i-1] != '/' && (source)[i-6] != 'p' && (source)[i+5] != 'n' && (source)[i+6] != 's' {
			elementStartIndex = i + 2
			for k := i; ; k++ {
				if (source)[k] == '<' && (source)[k+1] == '/' {
					elementData = string((source)[elementStartIndex:k])

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
