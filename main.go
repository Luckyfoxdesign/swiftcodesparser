package main

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
	"strings"
	common "swiftcodesparser/main/structures"
	"time"

	greq "github.com/Luckyfoxdesign/greq"
	_ "github.com/go-sql-driver/mysql"
)

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
	// countries.GetAllCountriesAndIsertToDB()
}

func runFactory() {
	var cfg common.Config = readConfig()
	const countriesToParse = 1
	var (
		swiftInfoChanWithIdandName chan SwiftInfo = make(chan SwiftInfo, countriesToParse)
		swiftInfoChanWithFirstData chan SwiftInfo = make(chan SwiftInfo, countriesToParse)
		swiftInfoChanWithAllData   chan SwiftInfo = make(chan SwiftInfo, countriesToParse)
		connectionString           string         = fmt.Sprintf("%s:%s@tcp(%s)/%s", cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Name)
	)

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

	go getAllCountriesFromDBAndSendThemToChan(&cfg, db, swiftInfoChanWithIdandName, countriesToParse)

	// Because we run our app with a cron
	// we can use non blocking buffered channel with predefined capacity
	for i := 0; i < countriesToParse; i++ {
		time.Sleep(time.Second)
		getAllSwiftCodesByCountry(<-swiftInfoChanWithIdandName, &cfg, swiftInfoChanWithFirstData)
		break
	}
	for i := 0; i < countriesToParse; i++ {
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
			// TODO!!!
			// need to write swiftInfoDetails to the swift_codes table
			// need to write status to the progress_temp table
		}
		break
	}
}

// Function that reads the config.json with ioutil.ReadFile()
// and returns unmarshaled json data in Config struct.
func readConfig() common.Config {
	var config common.Config

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

func returnAllCountriesFromDB(*sql.DB) string {
	return "array of strings, don't forget to replace return type"
}

// Function that sends struct with type SwiftInfo to a specific channel
// that specified in second argument.
// First agrument is a pointer to a SwiftInfo struct.
func sendStructToChannel(swiftInfoStruct *SwiftInfo, ch chan SwiftInfo) {
	ch <- *swiftInfoStruct
}

// Function that requests site data and parses response in the html.
// On the first page we get pages total count and run loop that
// requests on each page in a loop.
// Result of this response we add to the existing sturcture that passes
// like an argument in the function and send structure to another channel.
// On the first page we get pages total count and iterate each of page.
func getAllSwiftCodesByCountry(swiftInfoStruct SwiftInfo, cfg *common.Config, swiftInfoFirstDataChan chan SwiftInfo) {
	var (
		proxyURL       string = common.ReturnRandomProxyString(cfg)
		countryName    string = strings.ReplaceAll(swiftInfoStruct.CountryName, " ", "-")
		pagesNumber    int
		emptyByteSlice []byte
	)

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
func getSwiftCodeInfoFromPageAndWriteToExistingStruct(cfg *common.Config, swiftCodeDetailsStructIndex int, swiftCodeInfoStruct *SwiftInfo) {
	url := cfg.SiteURL + swiftCodeInfoStruct.CountryName + "/" + swiftCodeInfoStruct.DetailsSlice[swiftCodeDetailsStructIndex].SwiftCodeOrBIC
	src, err := getSiteHtmlCode(url, common.ReturnRandomProxyString(cfg))
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

// The function that requests a country id and name from the progress_temp table.
// Writes them to a SwiftInfo struct and send that struct to a chan where will be next parse steps.
func getAllCountriesFromDBAndSendThemToChan(cfg *common.Config, db *sql.DB, swiftInfoChanWithIdandName chan SwiftInfo, limitToParse int) {
	var (
		baseStruct SwiftInfo = SwiftInfo{}
		dbQuery    string    = fmt.Sprintf("SELECT id, name FROM progress_temp WHERE status=0 LIMIT %d", limitToParse)
	)
	err := db.QueryRow(dbQuery).Scan(&baseStruct.CountryId, &baseStruct.CountryName)

	if err != nil {
		log.Fatal("Error when db.QueryRow() in the getAllCountriesFromDBAndSendThemToChan() with the err: ", err)
	}

	sendStructToChannel(&baseStruct, swiftInfoChanWithIdandName)
}
