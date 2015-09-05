// Copyright (c) 2015 Julius Chrobak. You can use this source code
// under the terms of the MIT License found in the LICENSE file.

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strings"
)

type Member struct {
	Id        int    `json:"id"`
	FirstName string `json:"firstName"`
	LastName  string `json:"lastName"`
	Canton    string `json:"canton"`
	Party     string `json:"party"`
}

type CommitteeDetails struct {
	Id      int      `json:"id"`
	Members []Member `json:"members"`
}

type Committee struct {
	Id       int    `json:"id"`
	IsActive bool   `json:"isActive"`
	Name     string `json:"name"`
}

var committees []Committee
var details map[int]CommitteeDetails

func getjson(url string) ([]byte, error) {
	log.Printf("fetching %v\n", url)
	path := fmt.Sprintf("http://ws.parlament.ch/%v&format=json", url)

	client := &http.Client{}

	req, err := http.NewRequest("GET", path, nil)
	req.Header.Add("Accept", "application/json;q=0.9,*/*;q=0.8")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return ioutil.ReadAll(resp.Body)
	}

	return nil, nil
}

func fetchCommitteesPage(i int) ([]Committee, error) {
	url := fmt.Sprintf("committees?pageNumber=%v", i)
	blob, err := getjson(url)
	if err != nil {
		return nil, err
	}

	if blob == nil {
		return nil, nil
	}

	var committees []Committee
	if err := json.Unmarshal(blob, &committees); err != nil {
		return nil, err
	}

	return committees, nil
}

func fetchCommittees() ([]Committee, error) {
	result := make([]Committee, 0)
	for i := 1; ; i++ {
		coms, err := fetchCommitteesPage(i)
		if err != nil {
			return nil, err
		}

		if coms == nil {
			break
		}

		for _, c := range coms {
			if c.IsActive {
				result = append(result, c)
			}
		}
	}
	return result, nil
}

func fetchDetails(i int) (CommitteeDetails, error) {
	url := fmt.Sprintf("committees/%v?pageNumber=1", i)
	blob, err := getjson(url)
	if err != nil {
		return CommitteeDetails{}, err
	}

	var res CommitteeDetails
	if err := json.Unmarshal(blob, &res); err != nil {
		return CommitteeDetails{}, err
	}

	return res, nil
}

func match(val string, pat string) bool {
	return strings.Contains(strings.ToLower(val), strings.ToLower(pat))
}

func generate(w http.ResponseWriter, query string) {
	tmpl, err := ioutil.ReadFile("www/index.html")
	if err != nil {
		http.Error(w, "failed to read index.html", http.StatusInternalServerError)
		return
	}

	t, err := template.New("webpage").Parse(string(tmpl))
	if err != nil {
		log.Print(err)
		http.Error(w, "failed to parse index.html", http.StatusInternalServerError)
		return
	}

	type Result struct {
		Index         int
		Id            int
		CommitteeName string
		Members       int
		Match         int
		Url           string
	}
	data := make([]Result, 0)

	if query != "" {
		for _, c := range committees {
			d := details[c.Id]

			cnt := 0
			for _, m := range d.Members {
				if match(m.FirstName, query) || match(m.LastName, query) || match(m.Party, query) || match(m.Canton, query) {
					cnt += 1
				}
			}

			if cnt > 0 {
				data = append(data, Result{
					Index:         len(data) + 1,
					Id:            c.Id,
					CommitteeName: c.Name,
					Members:       len(d.Members),
					Match:         cnt,
					Url:           fmt.Sprintf("http://ws.parlament.ch/committees/%v", c.Id),
				})
			}
		}
	}

	err = t.Execute(w, data)
	if err != nil {
		log.Print(err)
		http.Error(w, "failed to execute template", http.StatusInternalServerError)
	}
}

func index(w http.ResponseWriter, req *http.Request) {
	if req.Method == "GET" {
		generate(w, "")
		return
	}

	if err := req.ParseForm(); err != nil {
		http.Error(w, "failed parsing the request form", http.StatusBadRequest)
	}

	if req.Method == "POST" {
		q, ok := req.PostForm["query"]
		if !ok || len(q) < 1 {
			http.Error(w, "no query value", http.StatusBadRequest)
			return
		}
		log.Printf("search for %v\n", q[0])
		generate(w, q[0])
		return
	}

	http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
}

func fetch() {
	var err error

	committees, err = fetchCommittees()
	if err != nil {
		log.Fatal(err)
	}

	details = make(map[int]CommitteeDetails)
	for _, c := range committees {
		d, err := fetchDetails(c.Id)
		if err != nil {
			log.Fatal(err)
		}

		details[c.Id] = d
	}
}

func data(w http.ResponseWriter, req *http.Request) {
	text := "<html><body><h1>Statistics</h1><p>Number of committees: %v</p></body></html>"
	io.WriteString(w, fmt.Sprintf(text, len(committees)))
}

func main() {
	var port = flag.Int("port", 8080, "http port to listen on")
	flag.Parse()

	p := fmt.Sprintf(":%v", *port)
	log.Printf(p)

	fetch()

	http.HandleFunc("/data/", data)
	http.HandleFunc("/", index)
    log.Printf("started")
	log.Fatal(http.ListenAndServe(p, nil))
}
