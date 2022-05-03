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
	cfg := readConfig()
	getAllCountries(&cfg)
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

	// runFactory()
}

func runFactory(db *sql.DB) {

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

// Description how this function works
func getAllCountries(cfg *Config, db *sql.DB) {
	// Slug for the page with all countries.
	// Site URL has a slash at the end of the URL.
	// browse-by-country/

	proxyURL := returnRandomProxyString(cfg)
	src, _ := greq.GetHTMLSource(cfg.SiteURL, proxyURL)

	parseHtmlAndInsertCountriesNamesToDB(&src, db)
}

// Function that parses html presented in slice of bytes
// and execute the function that inserts founded country name
// in to the database.
// Arguments are the html in slice of bytes and sql db pointer.
func parseHtmlAndInsertCountriesNamesToDB(src *[]byte, db *sql.DB) {
	for i, v := range *src {
		if i == 1 {
			swiftInfoStruct := SwiftInfo{}
			insertCountryNameToDB(db, swiftInfoStruct)
		}
	}
}

func insertCountryNameToDB(db *sql.DB, swiftInfoStruct SwiftInfo) error {
	return nil
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
