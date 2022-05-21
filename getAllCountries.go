package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"math/rand"
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

type LogMessage struct {
	place, msg string
}

func main() {
	getAllCountriesAndIsertToDB()
}

func getAllCountriesAndIsertToDB() {
	var cfg Config = readConfig()
	var connectionString string = fmt.Sprintf("%s:%s@tcp(%s)/%s", cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Name)

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

	getAllCountries(&cfg, db)
}

// Function that requests site url with proxy and execute
// functions that parses src and inserts a Country Name in to a database
func getAllCountries(cfg *Config, db *sql.DB) {
	// Slug for the page with all countries.
	// Site URL has the slash at the end of the URL.
	// browse-by-country/

	proxyURL := returnRandomProxyString(cfg)

	src, err := greq.GetHTMLSource(cfg.SiteURL+"browse-by-country/", proxyURL)
	if err != nil {
		log.Fatal("Error when greq.GetHTMLSource() in getAllCountries(): ", err)
	}

	parseHtmlAndInsertCountriesNamesToDb(&src, db)
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

// Function that parses html presented in slice of bytes
// and execute the function that inserts founded country name in to the database.
// Arguments are the html in slice of bytes and sql db pointer.
func parseHtmlAndInsertCountriesNamesToDb(src *[]byte, db *sql.DB) {
	var (
		quoteCounter                uint8
		quoteStartIndex             int
		countryName, countryNameURL string
	)
	for i, v := range *src {
		if v == 'i' && (*src)[i+1] == 'o' && (*src)[i+2] == 'n' && (*src)[i+4] == 'v' {
			for k := i; ; k++ {
				if (*src)[k] == '"' {
					if quoteCounter > 0 {
						countryName = strings.ToLower(string((*src)[quoteStartIndex+1 : k]))
						countryNameURL = strings.ReplaceAll(countryName, " ", "-")
						quoteCounter = 0

						err := insertCountryNameToDB(db, countryName, countryNameURL)
						if err != nil {
							logMsg := LogMessage{}
							logMsg.place = "insertCountryNameToDB"
							logMsg.msg = "Error with insert country name in " + logMsg.place + " with error: " + err.Error()

							insertLogMessage(&logMsg, db)
							log.Fatal("Error with insert country name in insertCountryNameToDB: ", err)
						}
						break
					}
					quoteCounter++
					quoteStartIndex = k
				}
			}
		}
	}
}

// Function that inserts all data from SwiftInfo struct in to a specified database
func insertCountryNameToDB(db *sql.DB, countryName, countryNameURL string) error {
	stmtIns, err := db.Prepare("INSERT INTO countries (name, url) VALUES(?, ?)")
	if err != nil {
		log.Fatal("Error with db.Prepare in the insertCountryNameToDB with error: ", err)
	}
	_, err = stmtIns.Exec(countryName, countryNameURL)
	if err != nil {
		log.Fatal("Error with stmtIns.Exec in the insertCountryNameToDB with error: ", err)
	}
	defer stmtIns.Close()

	return nil
}

func insertLogMessage(msg *LogMessage, db *sql.DB) {
	stmtIns, err := db.Prepare("INSERT INTO logs (place, message) VALUES(?, ?)")
	if err != nil {
		log.Fatal("Error with db.Prepare in the insertLogMessage with error: ", err)
	}
	_, err = stmtIns.Exec(msg.place, msg.msg)
	if err != nil {
		log.Fatal("Error with stmtIns.Exec in the insertLogMessage with error: ", err)
	}
	defer stmtIns.Close()
}
