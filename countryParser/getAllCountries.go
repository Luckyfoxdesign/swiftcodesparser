package countries

import (
	"database/sql"
	"fmt"
	"log"
	"strings"
	common "swiftcodesparser/main/structures"
	"time"

	greq "github.com/Luckyfoxdesign/greq"
	_ "github.com/go-sql-driver/mysql"
)

type LogMessage struct {
	place string
	msg   string
}

func GetAllCountriesAndIsertToDB() {
	var cfg common.Config = common.ReadConfig("../config.json", "GetAllCountriesAndIsertToDB")
	var connectionString string = fmt.Sprintf("%s:%s@tcp(%s)/%s", cfg.DB.User, cfg.DB.Password, cfg.DB.Host, cfg.DB.Name)

	db, err := sql.Open("mysql", connectionString)
	if err != nil {
		log.Fatal("Error when sql.Open() in GetAllCountriesAndIsertToDB(): ", err)
	}
	defer db.Close()

	err = db.Ping()
	if err != nil {
		log.Fatal("Error when db.Ping() in GetAllCountriesAndIsertToDB(): ", err)
	}

	db.SetConnMaxLifetime(time.Second * 2)
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(100)
	db.SetConnMaxIdleTime(time.Second * 2)

	getAllCountries(&cfg, db)
}

// Function that requests site url with proxy and execute
// functions that parses src and inserts a Country Name in to a database
func getAllCountries(cfg *common.Config, db *sql.DB) {
	// Slug for the page with all countries.
	// Site URL has the slash at the end of the URL.
	// browse-by-country/

	proxyURL := common.ReturnRandomProxyString(cfg)

	src, err := greq.GetHTMLSource(cfg.SiteURL+"browse-by-country/", proxyURL)
	if err != nil {
		log.Fatal("Error when greq.GetHTMLSource() in getAllCountries(): ", err)
	}

	parseHtmlAndInsertCountriesNamesToDb(&src, db)
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

						err := insertCountryNameToDB(db, countryName, countryNameURL, "countries")
						if err != nil {
							logMsg := LogMessage{}
							logMsg.place = "insertCountryNameToDB"
							logMsg.msg = "Error with insert country to countries name in " + logMsg.place + " with error: " + err.Error()

							insertLogMessage(&logMsg, db)
							log.Fatal("Error with insert country name to countries in insertCountryNameToDB: ", err)
						}

						err = insertCountryNameToDB(db, countryName, countryNameURL, "progress_temp")
						if err != nil {
							logMsg := LogMessage{}
							logMsg.place = "insertCountryNameToDB"
							logMsg.msg = "Error with insert country name to progress_temp in " + logMsg.place + " with error: " + err.Error()

							insertLogMessage(&logMsg, db)
							log.Fatal("Error with insert country to progress_temp name in insertCountryNameToDB: ", err)
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
func insertCountryNameToDB(db *sql.DB, countryName, countryNameURL, dbName string) error {
	var dbQuery string = fmt.Sprintf("INSERT INTO %s (name, url) VALUES(?, ?)", dbName)
	stmtIns, err := db.Prepare(dbQuery)
	if err != nil {
		return err
		// log.Fatal("Error with db.Prepare in the insertCountryNameToDB with error: ", err)
	}
	_, err = stmtIns.Exec(countryName, countryNameURL)
	if err != nil {
		return err
		// log.Fatal("Error with stmtIns.Exec in the insertCountryNameToDB with error: ", err)
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
