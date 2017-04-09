package httpbuf_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/chiffa-org/httpbuf"
	"log"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
)

type Request struct {
	Key  int    `json:"key"`
	Data string `json:"data"`
}

type Shard struct{ Name string }

func NewShard(name string) *Shard {
	return &Shard{Name: name}
}

func (s *Shard) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	var request Request
	err := json.NewDecoder(r.Body).Decode(&request)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
	}
	fmt.Printf("%s data is handled by %s shard\n", request.Data, s.Name)
	fmt.Fprint(w, "OK")
}

type Balancer struct{ Shards []*httputil.ReverseProxy }

func NewBalancer(shardURLs ...*url.URL) *Balancer {
	balancer := &Balancer{Shards: make([]*httputil.ReverseProxy, len(shardURLs))}
	for i, shardURL := range shardURLs {
		balancer.Shards[i] = httputil.NewSingleHostReverseProxy(shardURL)
	}
	return balancer
}

func (s *Balancer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	buf := new(bytes.Buffer)
	err := httpbuf.New(buf).ReadRequest(r)
	if err != nil {
		http.Error(w, "server error", http.StatusInternalServerError)
	}
	var request Request
	err = json.Unmarshal(buf.Bytes(), &request)
	if err != nil {
		http.Error(w, "bad request", http.StatusBadRequest)
	}
	shard := request.Key % len(s.Shards)
	s.Shards[shard].ServeHTTP(w, r)
}

func ExampleBalancer() {
	evenShard := httptest.NewServer(NewShard("even"))
	defer evenShard.Close()
	evenShardURL, _ := url.Parse(evenShard.URL)
	oddShard := httptest.NewServer(NewShard("odd"))
	defer oddShard.Close()
	oddShardURL, _ := url.Parse(oddShard.URL)

	balancer := httptest.NewServer(NewBalancer(evenShardURL, oddShardURL))
	defer balancer.Close()

	for _, request := range []Request{{Key: 7, Data: "god"}, {Key: 666, Data: "evil"}} {
		requestJSON, _ := json.Marshal(request)
		buf := new(bytes.Buffer)
		r, err := httpbuf.New(buf).ReadDo(http.Post(balancer.URL, "application/json", bytes.NewReader(requestJSON)))
		if err != nil {
			log.Fatal(err)
		}
		fmt.Printf("%d - %s\n", r.StatusCode, buf.String())
	}
	// Output:
	// god data is handled by odd shard
	// 200 - OK
	// evil data is handled by even shard
	// 200 - OK
}
