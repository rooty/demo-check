package main

import (
	"crypto/tls"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/joho/godotenv"

	_ "github.com/go-sql-driver/mysql"
)

type DemoURL struct {
	Id       int    `json:"id"`
	Platform string `json:"platform"`
	Translit string `json:"translit"`
	PlayURL  string `json:"url"`
}

// SafeCounter is safe to use concurrently.
type SafeCounter struct {
	mu        sync.Mutex
	numThread int
	count     int
}

var (
	db         *sql.DB
	DbHostName string
	DbPort     int = 3306
	counter    SafeCounter
	wg         sync.WaitGroup
	numThread  int
	printUsage bool

	//logger *log.Logger
	// %s:%s@tcp(%s)/%s login:password@tcp/host/dbname
	DB_DSN = ""
)

func init() {
	flag.IntVar(&numThread, "n", 20, "Num thread.")
	#flag.StringVar(&DbHostName, "d", "cas-db-stage.int.slotcatalog.com", "db host name")
	flag.BoolVar(&printUsage, "h", false, "Print usage")
	flag.Parse()
	if printUsage {
		flag.PrintDefaults()
		os.Exit(0)
	}
	err := godotenv.Load(".env")
	if err != nil {
		log.Println("don't load .env")
	}
}
func main() {
	DB_DSN, ok := os.LookupEnv("DB_DSN")
	if !ok {
		log.Fatal("DB_DSN not set")
		os.Exit(-1)
	}
	//proxyUrl, _ := url.Parse("http://127.0.0.1:5566")
	//http.DefaultTransport.(*http.Transport).Proxy = http.ProxyURL(proxyUrl)
	http.DefaultTransport.(*http.Transport).TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	http.DefaultTransport.(*http.Transport).IdleConnTimeout = 60 * time.Second

	var err error

	counter = SafeCounter{count: 0, numThread: 0}
	ts1 := time.Now()
	// db, err = sql.Open("mysql", dsn(dbname))
	db, err = sql.Open("mysql", DB_DSN)
	if err != nil {
		log.Fatalf("Error %s when opening DB\n", err)
		// return
	}
	defer db.Close()

	// See "Important settings" section.
	db.SetConnMaxLifetime(time.Minute * 3)
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(10)

	db.Exec("CREATE TABLE IF NOT EXISTS demo_url_check (" +
		"`url_id` int(11) DEFAULT 0," +
		"`platform` TINYINT UNSIGNED DEFAULT 0," +
		"`http_code` INT(11) UNSIGNED DEFAULT 0, " +
		"`detail` TEXT DEFAULT '', " +
		"`last_at` timestamp NULL DEFAULT current_timestamp()," +
		"PRIMARY KEY (`url_id`, `platform`))")
	if err != nil {
		log.Fatal(err.Error()) // proper error handling instead of panic in your app
	}
	_, err = db.Exec("TRUNCATE demo_url_check")
	if err != nil {
		log.Fatal(err.Error()) // proper error handling instead of panic in your app
	}

	rows, err := db.Query("SELECT id, 0 platform, translit, REPLACE(TRIM(REPLACE(Play_URL, '&amp;', '&')), '\t','') url  " +
		"FROM `itc_slots` WHERE NOT `play_URL` IS NULL and `play_URL` != '' " +
		"UNION ALL " +
		"SELECT id, 1 platform, translit, REPLACE(TRIM(REPLACE(play_url_mob, '&amp;', '&')), '\t','') url   " +
		"FROM `itc_slots` WHERE NOT `play_url_mob` IS NULL and `play_URL` != '' ")
	if err != nil {
		log.Fatal(err.Error())
	}
	defer rows.Close()

	lock := make(chan bool, numThread)
	//wg = new(sync.WaitGroup)

	for rows.Next() {
		var row DemoURL
		if err := rows.Scan(&row.Id, &row.Platform, &row.Translit, &row.PlayURL); err != nil {
			log.Println(err.Error())
		} else {
			wg.Add(1)
			//go worker(wg, row, lock, db)
			go worker(row, lock)
		}
		//time.Sleep(time.Second * 1)
	}
	wg.Wait()
	//tims := time.Now().Sub(ts1)
	tims := time.Since(ts1)
	fmt.Printf("Execute time %s\n", tims)
}

func saveAccess(urlrow DemoURL, status int, detail string) {
	if status == http.StatusOK {
		return
	}
	if status != 0 {
		detail = convertHTTPError(status)
	}

	//Prepare statement for inserting data
	stmtIns, err := db.Prepare(
		"INSERT INTO demo_url_check(url_id, platform, http_code, detail, last_at) VALUES( ?, ?,?,?,now() ) ")
	if err != nil {
		log.Println(err.Error()) // proper error handling instead of panic in your app
		return
	}
	defer stmtIns.Close() // Close the statement when we leave main() / the program terminates

	_, err = stmtIns.Exec(urlrow.Id, urlrow.Platform, status, detail)
	if err != nil {
		log.Println(err.Error()) // proper error handling instead of panic in your app
	}
}

func worker(urlrow DemoURL, lock chan bool) {
	defer wg.Done() // This decreases counter by 1

	client := &http.Client{}
	req, err := http.NewRequest("GET", urlrow.PlayURL, nil)
	if err != nil {
		log.Println(urlrow.Id, urlrow.Translit, err)
		return
	}
	req.Header.Add("Accept-Charset", "UTF-8;q=1 ISO-8859;q=0")
	req.Header.Set("User-Agent", "Mozilla/5.0 (X11; Linux i686 on x86_64)"+
		" AppleWebKit/537.36 (KHTML, like Gecko)"+
		"Chrome/49.0.2623.63 Safari/537.36")
	lock <- true
	counter.IncThread()
	//res, err1 := http.Get(urlrow.Play_URL)
	res, err := client.Do(req)
	counter.DecThread()
	<-lock
	counter.IncCounter()

	if err != nil {
		// какая-то сетевая ощибка, мы ничего не получили
		//hasNetError(err)
		//detail := err.Error()
		detail := GetNetError2String(err)
		//go saveAccess(urlrow, 0, detail, dblink)
		go saveAccess(urlrow, 0, detail)
		return
	} else {
		if res.StatusCode == http.StatusBadRequest {
			// log.Printf("Status Code:%d Id:%d Thread: %d\n", res.StatusCode, urlrow.Id, counter.GetThread())
			fmt.Print("*")
		} else if res.StatusCode == http.StatusTooManyRequests {
			fmt.Print("E")
		} else if res.StatusCode == http.StatusOK {
			// log.Printf("Status Code:%d Id:%d Thread: %d\n", res.StatusCode, urlrow.Id, counter.GetThread())
			fmt.Print(".")
		} else {
			go saveAccess(urlrow, res.StatusCode, "")
		}
	}
	defer res.Body.Close()
}

func GetNetError2String(errext error) string {
	var ret string
	switch err := errext.(type) {
	case *url.Error:
		if err.Timeout() {
			ret = fmt.Sprintf("timeout: %s", err.Err)
		} else if err1, ok := err.Err.(*net.OpError); ok {
			ret = fmt.Sprintf("net error: %s", err1)
		} else {
			ret = fmt.Sprintf("original error: %T", err.Err.Error())
		}
	default:
		ret = fmt.Sprintf("unknown error: %v", err)
	}
	return ret
}
func hasNetError(errext error) {
	switch err := errext.(type) {
	case *url.Error:
		if err.Timeout() {
			log.Printf("timeout: %s", err.Err)
		} else if err1, ok := err.Err.(*net.OpError); ok {
			log.Printf("net error: %s", err1)
		} else {
			log.Printf("original error: %T", err.Err.Error())
		}
	default:
		log.Printf("unknown error: %v", err)
	}
}

func hasTimeOut(err error) bool {
	switch err := err.(type) {
	case *url.Error:
		if err, ok := err.Err.(net.Error); ok && err.Timeout() {
			return true
		}
	case net.Error:
		if err.Timeout() {
			return true
		}
	case *net.OpError:
		if err.Timeout() {
			return true
		}
		log.Println(err.Error())
	}
	errTxt := "use of closed network connection"
	//if err != nil && strings.Contains(err.Error(), errTxt) {
	if strings.Contains(err.Error(), errTxt) {
		return true
	}
	return false
}

func convertHTTPError(httpErrorCode int) string {
	switch httpErrorCode {
	case 0:
		return "Zero"
	case http.StatusBadRequest:
		return "Bad Request"
	case http.StatusNotFound:
		return "Not Found"
	case http.StatusUnauthorized:
		return "Unauthorized response"
	case http.StatusForbidden:
		return "Unauthorized response"
	case http.StatusTooManyRequests:
		return "Too Many Requests"
	case http.StatusUnavailableForLegalReasons:
		return "Content unavailable for legal reasons"
	case 520:
		return "Origin Error (CloudFlare)"
	case http.StatusGatewayTimeout:
		return "Gateway Timeout"
	case http.StatusServiceUnavailable:
		return "Service Unavailable"
	case http.StatusBadGateway:
		return "Bad Gateway"
	case http.StatusInternalServerError:
		return "Internal Server Error"
	default:
		return fmt.Sprintf("%d", httpErrorCode)
	}
}

// Inc increments the counter for the given key.
func (c *SafeCounter) IncCounter() {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.count++
	c.mu.Unlock()
}
func (c *SafeCounter) IncThread() {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.numThread++
	c.mu.Unlock()
}
func (c *SafeCounter) DecThread() {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	c.numThread--
	c.mu.Unlock()
}

// Value returns the current value of the counter for the given key.
func (c *SafeCounter) GetCounter() int {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mu.Unlock()
	return c.count
}
func (c *SafeCounter) GetThread() int {
	c.mu.Lock()
	// Lock so only one goroutine at a time can access the map c.v.
	defer c.mu.Unlock()
	return c.numThread
}
