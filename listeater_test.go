package listeater

import (
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
    <div id="title">{{.title}}</div>
    <div id="desc">
        {{.desc}}
    </div>
    {{range .feats}}
    <div class="feat">{{.}}</div>
    {{end}}
</body>
</html>
`
var listTpl = `<html>
<body>
    <div> some testing things </div>
    <div id="thelist">
    {{range items}}
        <li class="listitem"><a href="/{{elemsurl}}/{{.title}}.html">{{.title}}</a></li>
    {{end}}
    </div>
    <div class="paginator" id="thepaginator">
    {{if hasNext}}
    	<a href="/{{listurl}}?p={{nextidx}}">NEXT</a>
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
const elPerPage = 8

func TestMain(m *testing.M) {
	setup()
	retCode := m.Run()
	os.Exit(retCode)
}

//tests setup
func setup() {
	//generate some random elements
	sz := rand.Intn(30)
	elements := []testElement{}
	for i := 0; i < sz; i++ {
		fs_n := rand.Intn(5)
		fs := []string{}
		for ii := 0; ii < fs_n; ii++ {
			fs = append(fs, randS(7))
		}
		elements = append(elements, testElement{Title: randS(10), Desc: randS(15), Feats: fs})
	}
}

func TestNoLoginCrawl(t *testing.T) {
	require := require.New(t)
	//build a mock backend service that will reply
	r := mux.NewRouter()
	r.HandleFunc("/"+listUrl, func(w http.ResponseWriter, r *http.Request) {
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
		t, err := template.New("list").Parse(listTpl)
		if sliceIdx+elPerPage < len(elements) {
			pIdx = pIdx + 1
		}
		err = t.Execute(w, map[string]interface{}{
			"items":    els,
			"listurl":  listUrl,
			"elemsurl": elemsUrl,
			"hasNext":  sliceIdx+elPerPage < len(elements),
			"nextIdx":  pIdx,
		})
		require.Nil(err)
	}).Methods("GET")
	r.HandleFunc("/"+elemsUrl+"/{id}", func(w http.ResponseWriter, r *http.Request) {
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
	le := ListEater{CrawlDesc: &CrawlDescriptor{
		ListUrl:        "",
		PaginationLink: "#thepaginator a",
		Element:        "li.listitem a",
	}}
	//GOT to finish
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
