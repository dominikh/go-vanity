package main

import (
	"honnef.co/go/unix_socket"

	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
)

type Package struct {
	VCS           string
	Name          string
	URL           string
	Source        string
	Documentation string
}

var host = os.Getenv("GO_HOST")
var socketPath = os.Getenv("GO_SOCKET")

func loadPackages() (map[string]Package, error) {
	var packages map[string]Package
	data, err := ioutil.ReadFile("packages.json")
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &packages)
	return packages, err
}

func serveIndex(w http.ResponseWriter, r *http.Request) {
	packages, err := loadPackages()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 500)
	}
	var lines []string
	for name, _ := range packages {
		lines = append(lines, fmt.Sprintf(`<li><a href="/go/%s">%s</a></li>`, name, name))
	}
	sort.StringSlice(lines).Sort()

	html := fmt.Sprintf(`<html><body><ul>%s</ul></body></html>`, strings.Join(lines, ""))
	w.Write([]byte(html))
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Println("Path:", r.URL.Path)

	if r.URL.Path == "/" {
		serveIndex(w, r)
		return
	}

	// Load the package list on every request because traffic is low
	// and this allows the most easy updating of the list
	packages, err := loadPackages()
	if err != nil {
		log.Println(err)
		http.Error(w, err.Error(), 500)
	}
	parts := strings.Split(r.URL.Path[1:], "/")
	var pkg Package
	var ok bool
	for i := len(parts); i > 0; i-- {
		name := strings.Join(parts[:i], "/")
		pkg, ok = packages[name]
		if ok {
			break
		}
	}

	if !ok {
		http.Error(w, "No such package", http.StatusNotFound)
		return
	}

	html := fmt.Sprintf(`<html><head><meta name="go-import" content="%s/%s %s %s"></head><body>Install: go get -u %s/%s<br><a href="%s">Documentation</a><br><a href="%s">Source</a></body></html>`,
		host, pkg.Name, pkg.VCS, pkg.URL,
		host, pkg.Name,
		pkg.Documentation, pkg.Source)

	w.Write([]byte(html))
}

func main() {
	if host == "" {
		fmt.Fprintln(os.Stderr, "Please specify a valid host.")
		os.Exit(1)
	}

	http.HandleFunc("/", handler)
	if socketPath == "" {
		fmt.Fprintln(os.Stderr, "Please specify a socket to listen on.")
		os.Exit(1)
	}

	l, err := socket.Listen(socketPath, 0666)
	if err != nil {
		panic(err)
	}

	err = http.Serve(l, nil)
	if err != nil {
		panic(err)
	}
}
