package listeater

import (
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"golang.org/x/net/publicsuffix"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"sync"
)

var ErrInvalidConfig = errors.New("Invalid config, missing crawl ops.")
var ErrNoLoginCreds = errors.New("No login credentials provided, login needed.")
var ErrCannotLogin = errors.New("Could not login, check your credentials.")

type LoginDescriptor struct {
	Url           string `json:"url"`
	UserField     string `json:"user_field`
	PasswordField string `json:"psw_field"`
}

type CrawlDescriptor struct {
	ListUrl        string `json:"url"`
	PaginationLink string `json:"pagination_link"`
	Element        string `json:"element"`
}

type ListEaterConfig struct {
	Login *LoginDescriptor `json:"login"`
	Crawl *CrawlDescriptor `json:"crawl"`
}

//a single crawling result with relative error if the case
type CrawlResult struct {
	Element interface{}
	Error   error
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

//the listeater tyoe, the main type of this package
type ListEater struct {
	LoginDesc *LoginDescriptor
	CrawlDesc *CrawlDescriptor
	Client    *http.Client
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
	nextUrl := le.CrawlDesc.ListUrl
	for nextUrl != "" {
		r, crawlErr := le.Client.Get(nextUrl)
		if crawlErr != nil {
			return errors.New("Error while crawling: " + crawlErr.Error())
		} else if r.StatusCode != 200 {
			fmt.Println(r.StatusCode)
			return errors.New("Error while crawling ")
		}
		nextUrl, crawlErr = le.listPageCrawl(nextUrl, resChan, elementCrawler)
		fmt.Println(nextUrl)
		if crawlErr != nil {
			return errors.New("Error while crawling: " + crawlErr.Error())
		}
	}
	return nil
}

//crawls a single page (of the paginated list), returns the next url or error
func (le *ListEater) listPageCrawl(url string, resChan chan CrawlResult, elementCrawler ElementCrawler) (nextUrl string, e error) {
	log.Println("Processing " + url)
	wg := sync.WaitGroup{}
	resp, err := le.Client.Get(url)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Println("Will be fatal, error reading resp goquery scroll")
		log.Println(err)
		return "", err
	}
	//scoped function to follow the single eelemnt to be extracted
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
		} else {
			//log.Println("Following: " + fUrl)
			wg.Add(1)
			go asyncFollow(fUrl)
		}
	})
	wg.Wait()
	np := doc.Find(le.CrawlDesc.PaginationLink).First()
	if np == nil || len(np.Nodes) < 1 {
		log.Println("no more pages") //this is good exit
		return "", nil
	}
	exist := false
	nextUrl, exist = np.Attr("href")
	if !exist {
		log.Println("WARNING: no href in xeturl")
		return "", nil
	}
	log.Println("Next page: " + nextUrl)
	return nextUrl, nil
}
