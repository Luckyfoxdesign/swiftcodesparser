package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
	"time"

	greq "github.com/Luckyfoxdesign/greq"
	_ "github.com/go-sql-driver/mysql"
)

type Config struct {
	Proxies []Proxy
	SiteURL string
	DB      Database
}

type Database struct {
	User,
	Password,
	Host,
	Name string
}

type Proxy struct {
	User,
	Password,
	Host,
	Port string
}

type SwiftInfo struct {
	CountryName,
	CountryId string
	DetailsSlice []SwiftInfoDetails
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

	// runFactory()
}

func runFactory(db *sql.DB) {
	var (
		swiftInfoChanWithIdandName chan SwiftInfo = make(chan SwiftInfo, 211)
	)

	/*
		// Open database connection
		var connectionString string = fmt.Sprintf("%s:%s@tcp(%s)/%s", config.DbUser, config.DbPassword, config.DbHost, config.DbName)

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
	*/

	cfg := readConfig()
	getAllCountries(&cfg, db, swiftInfoChanWithIdandName)
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
// functions that parses src, inserts in to a database
// and sends struct to the channel.
func getAllCountries(cfg *Config, db *sql.DB, swiftInfoChanWithIdandName chan SwiftInfo) {
	// Slug for the page with all countries.
	// Site URL has a slash at the end of the URL.
	// browse-by-country/

	proxyURL := returnRandomProxyString(cfg)
	src, _ := greq.GetHTMLSource(cfg.SiteURL+"browse-by-country/", proxyURL)

	parseHtmlInsertCountriesNamesToDBSendStructToChan(&src, db, swiftInfoChanWithIdandName)
}

// Function that parses html presented in slice of bytes
// and execute the function that inserts founded country name
// in to the database.
// Arguments are the html in slice of bytes and sql db pointer.
func parseHtmlInsertCountriesNamesToDBSendStructToChan(src *[]byte, db *sql.DB, swiftInfoChanWithIdandName chan SwiftInfo) {
	for i, v := range *src {
		if i == 1 {
			swiftInfoStruct := SwiftInfo{}
			err := insertCountryNameToDB(db, &swiftInfoStruct)
			if err != nil {
				log.Fatal("Error with insert country name in insertCountryNameToDB: ", err)
			}

			sendStructToChannel(&swiftInfoStruct, swiftInfoChanWithIdandName)
		}
	}
}

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
