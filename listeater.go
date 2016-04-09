package listeater

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/publicsuffix"
	"io/ioutil"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"regexp"
	"sync"
)

var ErrInvalidConfig = errors.New("Invalid config, missing crawl ops.")
var ErrNoLoginCreds = errors.New("No login credentials provided, login needed.")
var ErrCannotLogin = errors.New("Could not login, check your credentials.")
var urlRegex = regexp.MustCompile("(http|https)://([\\w_-]+(?:(?:\\.[\\w_-]+)+))([\\w.,@?^=%&:/~+#-]*[\\w@?^=%&/~+#-])?")

type LoginDescriptor struct {
	Url           string `json:"url"`
	UserField     string `json:"user_field`
	PasswordField string `json:"psw_field"`
}

type CrawlDescriptor struct {
	ListUrl string `json:"url"`
	Element string `json:"element"`
}

type ListEaterConfig struct {
	Login *LoginDescriptor `json:"login"`
	Crawl *CrawlDescriptor `json:"crawl"`
}

//a single crawling result with relative error if the case
type CrawlResult struct {
	Element interface{}
	Error   error
	Done    bool
}

//credentials for login (if needed)
type LoginCredentials struct {
	user string
	pass string
}

//the interface that must be implemented to crawl a single element of the list
type ElementCrawler interface {
	Extract(r *http.Response, resChan chan CrawlResult)
}

//the listeater type, the main type of this package
type ListEater struct {
	LoginDesc *LoginDescriptor
	CrawlDesc *CrawlDescriptor
	Client    *http.Client
	Paginator PaginationHandler
}

//does the login and saves the cookie
func (le *ListEater) login(creds *LoginCredentials) error {
	r, err := le.Client.PostForm(le.LoginDesc.Url, url.Values{
		le.LoginDesc.UserField:     {creds.user},
		le.LoginDesc.PasswordField: {creds.pass},
	})
	if err != nil {
		return err
	}
	if r.StatusCode != 200 {
		fmt.Println(r.StatusCode)
		return ErrCannotLogin
	}
	fmt.Println("Successfully logged in")
	return nil
}

//the main listeater function, does the actual crawling
func (le *ListEater) Crawl(resChan chan CrawlResult, elementCrawler ElementCrawler, creds *LoginCredentials) error {
	defer func() { resChan <- CrawlResult{Done: true} }() //will signal the results reader(on resChan) that we finished crawling
	if le.CrawlDesc == nil {
		return ErrInvalidConfig
	}
	//create the cookie preserving http client
	options := cookiejar.Options{
		PublicSuffixList: publicsuffix.List,
	}
	jar, err := cookiejar.New(&options)
	if err != nil {
		return err
	}
	le.Client = &http.Client{Jar: jar}
	//if login is needed, do this
	if le.LoginDesc != nil {
		if creds == nil {
			return ErrNoLoginCreds
		}
		if err := le.login(creds); err != nil {
			return err
		}
	}
	//start the crawling fiesta
	nextReq, _ := http.NewRequest("GET", le.CrawlDesc.ListUrl, nil)
	hasNext := true
	for hasNext {
		r, crawlErr := le.Client.Do(nextReq)
		if crawlErr != nil {
			return errors.New("Error while crawling: " + crawlErr.Error())
		} else if r.StatusCode != 200 {
			fmt.Println(r.StatusCode)
			return errors.New("Error while crawling ")
		}
		// Read the content
		var bodyBytes []byte
		if r.Body != nil {
			bodyBytes, _ = ioutil.ReadAll(r.Body)
		}
		// Restore the io.ReadCloser to its original state
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		crawlErr = le.listPageCrawl(r, resChan, elementCrawler)
		// Restore the io.ReadCloser to its original state
		r.Body = ioutil.NopCloser(bytes.NewBuffer(bodyBytes))
		if crawlErr != nil {
			return errors.New("Error while crawling: " + crawlErr.Error())
		}
		nextReq, hasNext, crawlErr = le.Paginator.Paginate(r)
		if crawlErr != nil {
			fmt.Println("pagination error")
			return crawlErr
		}
	}

	return nil
}

//crawls a single page (of the paginated list), returns the next url or error
func (le *ListEater) listPageCrawl(resp *http.Response, resChan chan CrawlResult, elementCrawler ElementCrawler) error {
	log.Println("Processing ")
	wg := sync.WaitGroup{}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Println("Will be fatal, error reading resp goquery scroll")
		log.Println(err)
		return err
	}
	//scoped function to follow the single elemnt to be extracted
	asyncFollow := func(elUrl string) {
		defer wg.Done()
		r, err := le.Client.Get(elUrl)
		if err != nil || r.StatusCode != 200 {
			fmt.Println(err)
			res := CrawlResult{Element: nil, Error: err}
			resChan <- res
			return
		}
		elementCrawler.Extract(r, resChan)
	}
	//scrape the elements
	doc.Find(le.CrawlDesc.Element).Each(func(i int, s *goquery.Selection) {
		fUrl, exists := s.Attr("href")
		if !exists {
			log.Println("Warning no href in follow")
			return
		}
		fUrl = urlRegex.FindString(fUrl)
		if fUrl == "" {
			log.Println("Warning no url in href in follow")
			return
		}
		//log.Println("Following: " + fUrl)
		wg.Add(1)
		go asyncFollow(fUrl)
	})
	wg.Wait()

	return nil
}
