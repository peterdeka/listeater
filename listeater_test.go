package listeater

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/require"
	"html/template"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strconv"
	"testing"
)

var oneElementTpl = `<html>
<body>
    <div id="title">{{.Title}}</div>
    <div id="desc">
        {{.Desc}}
    </div>
    {{range .Feats}}
    <div class="feat">{{.}}</div>
    {{end}}
</body>
</html>
`
var listTpl = `<html>
<body>
    <div> some testing things </div>
    <div id="thelist">
    {{range .items}}
        <li class="listitem"><a href="{{$.host}}/{{$.elemsurl}}/{{.Title}}">{{.Title}}</a></li>
    {{end}}
    </div>
    <div class="paginator" id="thepaginator">
    {{if .hasNext}}
    	<a href="{{.host}}/{{.listurl}}?p={{.nextIdx}}">NEXT</a>
    {{end}}
    </div> 
</body>
</html>
`

type testElement struct {
	Title string
	Desc  string
	Feats []string
}

var elements []testElement

const listUrl = "thelist"
const elemsUrl = "elements"
const userField = "username"
const pwField = "pass"
const user = "auser"
const pw = "somepassword"
const elPerPage = 8

func TestMain(m *testing.M) {
	setup()
	retCode := m.Run()
	os.Exit(retCode)
}

//tests setup
func setup() {
	//generate some random elements
	sz := rand.Intn(30) + 8
	elements = make([]testElement, sz)
	for i := 0; i < sz; i++ {
		fs_n := rand.Intn(5) + 1
		fs := []string{}
		for ii := 0; ii < fs_n; ii++ {
			fs = append(fs, randS(7))
		}
		elements[i] = testElement{Title: randS(10), Desc: randS(15), Feats: fs}
	}
}

func TestLoginCrawl(t *testing.T) {
	require := require.New(t)
	hostUrl := ""
	//build a mock backend service that will reply
	r := mux.NewRouter()
	r.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		r.ParseForm()
		if r.Form[userField][0] != user || r.Form[pwField][0] != pw {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		cookie := http.Cookie{Name: userField, Value: user}
		http.SetCookie(w, &cookie)
		w.Write([]byte("OK"))
	}).Methods("POST")
	r.HandleFunc("/"+listUrl, func(w http.ResponseWriter, r *http.Request) {
		if !isAuthed(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		pageIdx := r.URL.Query().Get("p")
		if pageIdx == "" {
			pageIdx = "0"
		}
		pIdx, err := strconv.Atoi(pageIdx)
		require.Nil(err)
		sliceIdx := pIdx * elPerPage
		if sliceIdx > len(elements)-1 {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		els := elements[sliceIdx:min(sliceIdx+elPerPage, len(elements))]
		t, _ := template.New("list").Parse(listTpl)

		if sliceIdx+elPerPage < len(elements) {
			pIdx = pIdx + 1
		}
		hasNext := sliceIdx+elPerPage < len(elements)
		err = t.Execute(w, map[string]interface{}{
			"items":    els,
			"listurl":  listUrl,
			"elemsurl": elemsUrl,
			"hasNext":  hasNext,
			"nextIdx":  pIdx,
			"host":     hostUrl,
		})
		require.Nil(err)
	}).Methods("GET")
	r.HandleFunc("/"+elemsUrl+"/{id}", func(w http.ResponseWriter, r *http.Request) {
		if !isAuthed(r) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		id := mux.Vars(r)["id"]
		//find the element
		for i := 0; i < len(elements); i++ {
			if elements[i].Title == id {
				t, err := template.New("element").Parse(oneElementTpl)
				err = t.Execute(w, elements[i])
				require.Nil(err)
				return
			}
		}
		w.WriteHeader(http.StatusNotFound)
	}).Methods("GET")
	backend := httptest.NewServer(r)
	defer backend.Close()
	backUrl, _ := url.Parse(backend.URL)
	hostUrl = backUrl.String()
	le := ListEater{
		LoginDesc: &LoginDescriptor{
			Url:           hostUrl + "/login",
			UserField:     userField,
			PasswordField: pwField,
		},
		CrawlDesc: &CrawlDescriptor{
			ListUrl: hostUrl + "/" + listUrl,
			Element: "li.listitem a",
		},
		Paginator: &HrefPaginationHandler{
			Selector: "#thepaginator a",
		},
	}
	//crawl
	result := []testElement{}
	resChan := make(chan CrawlResult)
	go func() {
		for {
			y := <-resChan
			if y.Error != nil {
				fmt.Println("Error")
			} else {
				result = append(result, y.Element.(testElement))
			}
		}
	}()
	if err := le.Crawl(resChan, testElCrawler{}, &LoginCredentials{user: user, pass: pw}); err != nil {
		fmt.Println(err)
	}
	require.Equal(len(elements), len(result))
	fmt.Println("DONE")
}

type testElCrawler struct {
}

func (tec testElCrawler) Extract(r *http.Response, resChan chan CrawlResult) {
	d, err := goquery.NewDocumentFromResponse(r)
	if err != nil {
		res := CrawlResult{Element: nil, Error: err}
		resChan <- res
		return
	}
	t := d.Find("#title").First().Text()
	desc := d.Find("#desc").First().Text()
	feats := []string{}
	d.Find(".feat").Each(func(i int, s *goquery.Selection) {
		feats = append(feats, s.Text())
	})
	resChan <- CrawlResult{Element: testElement{Title: t, Desc: desc, Feats: feats}, Error: nil}
}

//util funcs
const letterBytes = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func randS(n int) string {
	b := make([]byte, n)
	for i := range b {
		b[i] = letterBytes[rand.Intn(len(letterBytes))]
	}
	return string(b)
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func isAuthed(r *http.Request) bool {
	cookie, err := r.Cookie(userField)
	return err == nil && cookie.Value == user
}
