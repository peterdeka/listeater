package listeater

import (
	"errors"
	"github.com/PuerkitoBio/goquery"
	"log"
	"net/http"
	//"net/url"
)

var ErrInvalidSelector = errors.New("Invalid selector for pagination.")
var ErrNoHrefInPagination = errors.New("Pagination found but no href inside.")

//the interface that must be implemented to paginate from a page.
//Returns request for next page, hasNext bool, and an error
type PaginationHandler interface {
	Paginate(r *http.Response) (*http.Request, bool, error)
}

//a simple pagination handler that extracts the href from the specified element
type HrefPaginationHandler struct {
	Selector string `json:"selector"`
}

func (hph HrefPaginationHandler) Paginate(r *http.Response) (*http.Request, bool, error) {
	if hph.Selector == "" {
		return nil, false, ErrInvalidSelector
	}
	//extract the pagination link
	p, err := goquery.NewDocumentFromResponse(r)
	if err != nil {
		return nil, false, err
	}
	np := p.Find(hph.Selector).First()
	if np == nil || len(np.Nodes) < 1 {
		log.Println("no more pages") //this is good exit
		return nil, false, nil
	}
	nextUrl, exist := np.Attr("href")
	if !exist {
		log.Println("WARNING: no href in xeturl")
		return nil, false, ErrNoHrefInPagination
	}
	log.Println("Next page: " + nextUrl)
	req, _ := http.NewRequest("GET", nextUrl, nil)
	return req, true, nil
}
