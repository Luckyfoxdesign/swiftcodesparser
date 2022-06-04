package common

import (
	"fmt"
	"math/rand"
	"time"
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

// Function returns random proxy string from the parameters
// that listed in the array in the config file.
// Function argument is a pointer to the config file constant.
func ReturnRandomProxyString(c *Config) string {
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
