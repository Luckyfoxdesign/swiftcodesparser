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
	CountryId    uint
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
	// ERR кажется ошибка в обработке кода т.к в самой структуре записи вида <a href="/australia/abocau2s/">ABOCAU2S
	// имеют нормальный вид
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
	var cfg common.Config = common.ReadConfig("./config.json", "runFactory")
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
	// we can use a non blocking buffered channel with a predefined capacity
	for i := 0; i < countriesToParse; i++ {
		time.Sleep(time.Second)
		getAllSwiftCodesByCountry(<-swiftInfoChanWithIdandName, &cfg, swiftInfoChanWithFirstData)
		break
	}
	for i := 0; i < countriesToParse; i++ {
		swiftInfoStruct := <-swiftInfoChanWithFirstData
		for i, v := range swiftInfoStruct.DetailsSlice {
			if v.SwiftCodeOrBIC != "" {
				// On this step the structure hasn't a valid swift code.
				// Field contains an html link element inside whom placed a swift code.
				// So we need to extract this code.
				// Example: <a href="/albania/usalaltrvl2/">USALALTRVL2

				// !!!I REALLY DON'T KNOW HOW IT WORKS. BUT IT WORK.
				// DON'T FORGET ABOUT THIS PLACE, LEARN.
				// Previously I've wrote extractSwiftCode as a separate func with
				// a pointer argument to the v variable.
				// I guess it works because the v in the loop as a copy in memory not a pointer
				// so when I access the child struct by the index directly from the parent struct
				// I can correctly change values for the child struct.

				//swiftInfoStruct.DetailsSlice[i].extractSwiftCode()
				swiftInfoStruct.DetailsSlice[i].SwiftCodeOrBIC = strings.ToLower(v.SwiftCodeOrBIC)
			}
			time.Sleep(time.Millisecond * 200)
			getSwiftCodeInfoFromPageAndWriteToExistingStruct(&cfg, i, &swiftInfoStruct)
		}
		sendStructToChannel(&swiftInfoStruct, swiftInfoChanWithAllData)
		break
	}
	for i := 0; i < countriesToParse; i++ {
		swiftInfoStruct := <-swiftInfoChanWithAllData
		for _, v := range swiftInfoStruct.DetailsSlice {
			insertSwiftInfoDetailsToDB(swiftInfoStruct.CountryId, v, db)
			// fmt.Println("BankOrInstitution: ", v.BankOrInstitution)
			// fmt.Println("City: ", v.City)
			// fmt.Println("Branch: ", v.Branch)
			// fmt.Println("SwiftCodeOrBIC: ", v.SwiftCodeOrBIC)
			// fmt.Println("END=============")
		}
		// cause we don't expect any error while parsing
		// we always send the status = 1 (without errors) as an argument

		// TODO??? Do I need functions that handle errors? I think know, cause
		// the website structure is pretty simple
		setCountryStatusToDB(swiftInfoStruct.CountryId, 1, db)
	}
}

// Function that reads the config.json with the ioutil.ReadFile() func
// and returns an unmarshaled json data in a Config struct.
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

// Function that sends a struct with the type SwiftInfo to a specific channel
// that specified in a second argument.
// First agrument is a pointer to the SwiftInfo struct.
func sendStructToChannel(swiftInfoStruct *SwiftInfo, ch chan SwiftInfo) {
	ch <- *swiftInfoStruct
}

// Function that requests a site data and parses a response in the html.
// On the first page we get a pages total count and run a loop that
// requests on each page in it.
// A result of this response we add to the existing sturcture that passes
// like an argument in the function and send structure to an another channel.
// On a first page we get the pages total count and iterate each of the page.
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

	if pagesNumber > 1 {
		for i := 2; i <= pagesNumber; i++ {
			findSwiftCodeInfoInPage(cfg.SiteURL+countryName+"/page/"+strconv.Itoa(i), proxyURL, &swiftInfoStruct, swiftInfoFirstDataChan, &emptyByteSlice)
		}
	}
	// we don't need to handle countries with a pages count equals zero
	// cause we expect that the country always have 1 or more pages
	// and these pages always have 1 or more swift codes
}

// The function that requests a site url via a proxy and returns a slice of bytes.
// If request has an error the function returns it or returns nil.
func getSiteHtmlCode(siteURL, proxyURL string) ([]byte, error) {
	src, err := greq.GetHTMLSource(siteURL, proxyURL)
	if err != nil {
		return src, err
	}
	return src, nil
}

// The function that requests a swift code page with a full information for a requested swift code.
// It searching for a postcode and a connection.
// When we find them we write them to an existing SwiftInfo struct.
func getSwiftCodeInfoFromPageAndWriteToExistingStruct(cfg *common.Config, swiftCodeDetailsStructIndex int, swiftCodeInfoStruct *SwiftInfo) {
	url := cfg.SiteURL + swiftCodeInfoStruct.CountryName + "/" + swiftCodeInfoStruct.DetailsSlice[swiftCodeDetailsStructIndex].SwiftCodeOrBIC
	src, err := getSiteHtmlCode(url, common.ReturnRandomProxyString(cfg))
	if err != nil {
		log.Fatal("Error when getSiteHtmlCode() in the getSwiftCodeInfoFromPageAndWriteToExistingStructAndSendToChan() with the err: ", err)
	}

	var (
		postCodeTitleStartIndex        int = bytes.Index(src, []byte(">Add"))
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

// The function that parses an html code and search for a
// Bank or Institution, a City, a Branch, a Swift code.
// When the information will be found the function writes it to a SwiftInfo struct
// and sends it into a specific channel.
func findSwiftCodeInfoInPage(siteURL, proxyURL string, swiftCodeStruct *SwiftInfo, swiftCodeChan chan SwiftInfo, src *[]byte) {
	var (
		firstTableIndex int = bytes.Index(*src, []byte("<tb"))
		lastTableIndex  int = bytes.Index(*src, []byte("</tb")) // Do we really need 4th loop? !THINK
		elementData     string
		elementCounter  uint8
		elementsInfo    map[uint8]string = make(map[uint8]string, 5)
		source          []byte
		err             error
		details         SwiftInfoDetails
	)

	// 	<tr>
	//      <td colspan="5"><ins class="adsbygoogle adsbygoogle--feed" style="display:block" data-ad-format="fluid" data-ad-layout-key="-gw-3+1f-3d+2z" data-ad-client="ca-pub-3108645613548918" data-ad-slot="9247727100"></ins>
	// <script>
	//      (adsbygoogle = window.adsbygoogle || []).push({});
	// </script></td>
	//    </tr>
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
		if (source)[i] == 't' && (source)[i+1] == 'a' && (source)[i+2] == 'b' && (source)[i-9] == 'd' {
			for k := i; ; k++ {
				if (source)[k] == '<' && (source)[k+1] == '/' {
					elementData = string((source)[i:k])

					// <ins class= it's a google ad element that inserts by js
					// we don't need this element
					if !strings.Contains(elementData, "<ins") {
						elementsInfo[elementCounter] = elementData
					}
					// TODO!!! Need check the condition above, 'couse main loop condition was rewritten
					// I don't sure that I need condition for <ins element...
					break
				}
			}

			elementCounter++
			if elementCounter == 5 {
				for i, v := range elementsInfo {
					for k := len(v) - 1; k > 0; k-- {
						if v[k] == '>' {
							elementsInfo[i] = string(v[k+1:])
							break
						}
					}
				}
				// For debugging
				// fmt.Println("id: ", elementsInfo[0])
				// fmt.Println("BankOrInstitution: ", elementsInfo[1])
				// fmt.Println("City: ", elementsInfo[2])
				// fmt.Println("Branch: ", elementsInfo[3])
				// fmt.Println("SwiftCodeOrBIC: ", elementsInfo[4])
				// fmt.Println("END=============")
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

// The function that searchs for the >Last word and checking, if the symbol / is before the searching word.
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

// The function that requests a country id and a name from the progress_temp table.
// Writes them to a SwiftInfo struct and send that struct to a channel where will be next parse steps.
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

// The function that inserts a new swift details tuple into the swift_codes table
func insertSwiftInfoDetailsToDB(countryId uint, swiftInfoDetailsStruct SwiftInfoDetails, db *sql.DB) {
	stmtIns, err := db.Prepare("INSERT INTO swift_codes (country_id, swift_bic, bank_institution, branch_name, address, city_name, postcode, connection) VALUES(?, ?, ?, ?, ?, ?, ?, ?)")
	if err != nil {
		log.Fatal("Error with db.Prepare in the insertSwiftInfoDetailsToDB with error: ", err)
	}
	_, err = stmtIns.Exec(countryId, swiftInfoDetailsStruct.SwiftCodeOrBIC, swiftInfoDetailsStruct.BankOrInstitution, swiftInfoDetailsStruct.Branch, swiftInfoDetailsStruct.Address, swiftInfoDetailsStruct.City, swiftInfoDetailsStruct.Postcode, swiftInfoDetailsStruct.Connection)
	if err != nil {
		log.Fatal("Error with stmtIns.Exec in the insertSwiftInfoDetailsToDB with error: ", err)
	}
	defer stmtIns.Close()
}

// The function that sets a new status value for a country in the progress_temp table
func setCountryStatusToDB(countryId, status uint, db *sql.DB) {
	// TODO??? need to handle parsing errors and to pass a specific status via argument
	stmtIns, err := db.Prepare("UPDATE progress_temp SET status=? WHERE id=?")
	if err != nil {
		log.Fatal("Error with db.Prepare in the setCountryStatusToDB with error: ", err)
	}
	_, err = stmtIns.Exec(status, countryId)
	if err != nil {
		log.Fatal("Error with stmtIns.Exec in the setCountryStatusToDB with error: ", err)
	}
	defer stmtIns.Close()
}
