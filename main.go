package main

import (
	"bytes"
	"crypto/tls"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"github.com/Jeffail/gabs/v2"
	"github.com/fatih/color"
	"golang.org/x/net/html"
)

// UserAgent switch to obscure the one used by go
var (
	UserAgent        = "NCM Dump" // getLatestUserAgent()
	IPAddress        = flag.String("ip", "", "SW Orion server IP")
	userName         = flag.String("u", "guest", "SW Orion server username")
	password         = flag.String("p", "", "SW Orion server password")
	extractionMethod = flag.Bool("m", false, "Extract server configs  using export instead of edit")
	useTLS           = flag.Bool("tls", false, "Connect using TLS")
	exportContent    = flag.Bool("e", false, "Export server configurations to file")
	tcpPort          = flag.String("port", "", "tcp port for connection")
	URIHandler       string

	tr = &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
)

// stringCleaning - To avoid checking if config values from NCM are null we cast them as strings and clean up special characters later
func stringCleaning(userString string) string {
	value := userString
	value = strings.Replace(value, "[", "", -1)
	value = strings.Replace(value, "]", "", -1)
	value = strings.Replace(value, `"`, "", -1)
	return value
}

// OrionSession is an IPAddress + Cookie combo for authenticating to Orion
type OrionSession struct {
	IPAddress string
	Session   []*http.Cookie
}

// OrionNode is the output structure used by GetOrionNodes
type OrionNode struct {
	NodeID           string
	Name             string
	IPAddress        string
	ConfigCount      string
	Vendor           string
	MachineType      string
	City             string
	Country          string
	NodeConfigFileID string
}

//ConnectOrionServer -  Tests connection to remote Orion server
func ConnectOrionServer(IPAddress string, UserName string, Password string) http.Response {
	var portNumber string
	if *tcpPort != "" {
		portNumber = ":" + *tcpPort
	}
	BaseURI := URIHandler + IPAddress + portNumber + "/Orion/Login.aspx?ReturnUrl=%2fOrion%2fNCM%2fConfigurationManagement.aspx"
	LoginForm := (url.Values{
		"ctl00$BodyContent$Username": {UserName},
		"ctl00$BodyContent$Password": {Password},
		"__EVENTTARGET":              {""},
	})

	// Follow only first redirect retreiving the first response which contains the necessary headers
	httpClient := http.Client{Transport: tr, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}}
	var cookieString string
	httpResponse, err := httpClient.PostForm(BaseURI, LoginForm)
	if err != nil {
		color.Red("Connection Failed", err)
		os.Exit(1)
	}
	responseCookies := httpResponse.Cookies()
	countCookies := len(responseCookies)
	for i := 0; i < countCookies; i++ {
		cookieString += responseCookies[i].String() + " "
	}
	authCheck, _ := regexp.MatchString("ASPXAUTH", cookieString)
	if authCheck {
		log.Println("Login successful for", IPAddress)
	} else {
		log.Fatal("Login Failed for ", IPAddress)

	}
	// Clean up and return
	defer httpResponse.Body.Close()
	return *httpResponse
}

// GetOrionNodes - retrieves the list of connected nodes
func GetOrionNodes(Session []*http.Cookie, IPAddress string) []OrionNode {
	DeviceListURI := URIHandler + IPAddress + "/Orion/NCM/Services/ConfigManagement.asmx/GetNodesPaged?start=0&limit=1000&sort=LastTransferDate&dir=DESC"
	// Generate Auth Cookie

	// Create http client
	httpClient := http.Client{Transport: tr}
	// Create request skeleton
	httpRequestBody := []byte(`{"groupingQueryString":"","showSelectedOnly":"False","colToSearch":"Nodes.Caption","searchTerm":"","clientOffset":240}`)
	httpRequest, _ := http.NewRequest("POST", DeviceListURI, bytes.NewReader(httpRequestBody))
	httpRequest.Header.Add("X-Requested-With", "XMLHttpRequest")
	httpRequest.Header.Add("Content-Type", "application/json")
	httpRequest.Header.Add("User-Agent", UserAgent)

	cookieCount := len(Session)
	for i := 0; i < cookieCount; i++ {
		httpRequest.AddCookie(Session[i])
	}
	httpResponse, err := httpClient.Do(httpRequest)
	httpResponseBody, err := ioutil.ReadAll(httpResponse.Body)
	if err != nil {
		log.Fatal(err)
	}
	ParsedJSON, _ := gabs.ParseJSON(httpResponseBody)
	nodeDataRoot := ParsedJSON.Search("d")
	nodeDataTable := nodeDataRoot.Search("DataTable")
	nodeDataColumns := nodeDataTable.Search("Columns").Children()
	nodeColumnsCount := len(nodeDataColumns)

	nodeDataRows := nodeDataTable.Search("Rows").Children()
	nodeReportedCount := nodeDataRoot.Search("TotalRows").Data()
	nodeDataCount := len(nodeDataRows)

	// stdio messages
	fmt.Println(color.RedString("Discovered"), nodeReportedCount, "managed devices")
	var NodeContainer []OrionNode
	// Begin per node loop
	for i := 0; i < nodeDataCount; i++ {
		var CurrentNode OrionNode
		currentRow := nodeDataRows[i].Children()
		for c := 0; c < nodeColumnsCount; c++ {
			rowValue := currentRow[c]
			// Strings are use forced via gabs to avoid dealing with random null conditions
			switch c {
			case 0:
				CurrentNode.NodeID = rowValue.String()
			case 2:
				CurrentNode.Name = rowValue.String()
			case 3:
				CurrentNode.IPAddress = rowValue.String()
			case 17:
				CurrentNode.MachineType = rowValue.String()
			case 18:
				CurrentNode.Vendor = rowValue.String()
			case 23:
				CurrentNode.City = rowValue.String()
			case 26:
				CurrentNode.Country = rowValue.String()

			}
		}
		fmt.Println("")
		fmt.Println(CurrentNode)
		NodeContainer = append(NodeContainer, CurrentNode)

	}
	return NodeContainer
}

// GetNodeConfigCount - Retrieves configuration count from
func GetNodeConfigCount(Session []*http.Cookie, IPAddress string, NodeID string) string {
	ConfigCountURI := URIHandler + IPAddress + "/Orion/NCM/Services/ConfigManagement.asmx/GetConfigsTotalRows"

	// Create byte stream for request body
	httpRequestString := `{"nodeId":"` + NodeID + `"}`
	httpRequestBody := []byte(httpRequestString)

	// Create http client
	httpClient := http.Client{Transport: tr}

	// Prepare POST request
	httpRequest, _ := http.NewRequest("POST", ConfigCountURI, bytes.NewReader(httpRequestBody))
	httpRequest.Header.Add("X-Requested-With", "XMLHttpRequest")
	httpRequest.Header.Add("Content-Type", "application/json")
	httpRequest.Header.Add("User-Agent", UserAgent)
	cookieCount := len(Session)
	for i := 0; i < cookieCount; i++ {
		httpRequest.AddCookie(Session[i])
	}
	// Issue web request and capture response
	httpResponse, _ := httpClient.Do(httpRequest)
	httpResponseBody, _ := ioutil.ReadAll(httpResponse.Body)
	ParsedJSON, _ := gabs.ParseJSON(httpResponseBody)
	nodeDataRoot := ParsedJSON.Search("d")
	dataType := reflect.TypeOf(nodeDataRoot.Data()).Kind()
	var configCount string
	if dataType == reflect.Float64 {
		configCount = strconv.Itoa(int(nodeDataRoot.Data().(float64)))

	} else {
		configCount = nodeDataRoot.Data().(string)
	}
	return configCount
}

// GetNodeConfigFileID - Retrieves a list of config file identifiers
func GetNodeConfigFileID(Session []*http.Cookie, IPAddress string, NodeID string) string {
	ConfigFileIDURI := URIHandler + IPAddress + "/Orion/NCM/Services/ConfigManagement.asmx/GetConfigsPaged?sort=Name&dir=ASC"

	// Create byte stream for request body
	httpRequestString := `{"nodeId":"` + NodeID + `","start":"1","showAllConfigs":"FALSE","clientOffset":"240"}`
	httpRequestBody := []byte(httpRequestString)

	// Create http client
	httpClient := http.Client{Transport: tr}

	// Prepare POST request
	httpRequest, _ := http.NewRequest("POST", ConfigFileIDURI, bytes.NewReader(httpRequestBody))
	httpRequest.Header.Add("X-Requested-With", "XMLHttpRequest")
	httpRequest.Header.Add("Content-Type", "application/json")
	httpRequest.Header.Add("User-Agent", UserAgent)
	cookieCount := len(Session)
	for i := 0; i < cookieCount; i++ {
		httpRequest.AddCookie(Session[i])
	}
	// Issue web request and capture response
	httpResponse, _ := httpClient.Do(httpRequest)
	httpResponseBody, _ := ioutil.ReadAll(httpResponse.Body)
	ParsedJSON, _ := gabs.ParseJSON(httpResponseBody)

	nodeDataRoot := ParsedJSON.Search("d").Search("DataTable").Search("Rows")

	configFileIDs := nodeDataRoot.Children()
	configID := configFileIDs[0].Children()
	configIDString := configID[0].Data().(string)
	return configIDString
}

// GetNodeConfigFileBody retrieves the configuration file for the provided ID
func GetNodeConfigFileBody(Session []*http.Cookie, IPAddress string, ConfigFileID string) string {
	// Create http client
	httpClient := http.Client{Transport: tr}

	// Retrieve config using EXPORT
	var httpRequestURI string
	if *extractionMethod {
		httpRequestURI = URIHandler + IPAddress + "/Orion/NCM/Resources/NCMConfigDetails/ConfigExporter.ashx?configID={" + ConfigFileID + "}"
	} else {
		// Retrieve config using EDIT
		httpRequestURI = URIHandler + IPAddress + "/Orion/NCM/Resources/Configs/EditConfig.aspx?ConfigID=" + ConfigFileID
	}
	httpRequest, _ := http.NewRequest("GET", httpRequestURI, nil)
	httpRequest.Header.Add("User-Agent", UserAgent)
	cookieCount := len(Session)
	for i := 0; i < cookieCount; i++ {
		httpRequest.AddCookie(Session[i])
	}
	// Issue web request and capture response
	httpResponse, _ := httpClient.Do(httpRequest)
	httpResponseBody, _ := ioutil.ReadAll(httpResponse.Body)

	configFileData := html.NewTokenizer(bytes.NewReader(httpResponseBody))
	var attributeString string
	for configFileData.Err() == nil {
		tagName, hasAttribute := configFileData.TagName()
		tagNameString := string(tagName)
		if tagNameString == `textarea` && hasAttribute == true {
			configFileData.Next()
			attributeString = string(configFileData.Raw())
			fmt.Println(attributeString)
		}

		configFileData.Next()
	}

	defer httpResponse.Body.Close()
	return attributeString
}

func main() {
	flag.Parse()
	IPAddress := *IPAddress
	UserName := *userName
	Password := *password

	if IPAddress == "" {
		color.Red("Please enter a valid IP Address")
		os.Exit(1)
	}

	if *useTLS {
		URIHandler = "https://"
	} else {
		URIHandler = "http://"
	}

	// acceptData should contain the type http.Response with valid Cookies
	acceptData := ConnectOrionServer(IPAddress, UserName, Password)
	Session := acceptData.Cookies()

	// GetOrionNodes should return a map of all nodes discovered
	OrionNodesDirectory := GetOrionNodes(Session, IPAddress)
	OrionNodesCount := (len(OrionNodesDirectory))
	for i := 0; i < OrionNodesCount; i++ {

		NodeID := stringCleaning(OrionNodesDirectory[i].NodeID)
		OrionNodesDirectory[i].ConfigCount = GetNodeConfigCount(Session, IPAddress, NodeID)

		if OrionNodesDirectory[i].ConfigCount != "0" {
			fmt.Println("")
			color.HiBlue("Retrieving config file from " + OrionNodesDirectory[i].Name)

			OrionNodesDirectory[i].NodeConfigFileID = GetNodeConfigFileID(Session, IPAddress, NodeID)
			if OrionNodesDirectory[i].NodeConfigFileID != "{}" {

				if *exportContent {
					fileContents := GetNodeConfigFileBody(Session, IPAddress, OrionNodesDirectory[i].NodeConfigFileID)
					color.HiBlue("Writing contents to", string(OrionNodesDirectory[i].Name))
					ioutil.WriteFile(stringCleaning(OrionNodesDirectory[i].Name)+".ncm", []byte(fileContents), os.FileMode(7))

				} else {
					fmt.Println(GetNodeConfigFileBody(Session, IPAddress, OrionNodesDirectory[i].NodeConfigFileID))
				}

			}
		} else {
			color.Red("No config files found on " + OrionNodesDirectory[i].Name)
		}
	}

}
