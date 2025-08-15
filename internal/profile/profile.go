package profile

import (
	"log"
	"net/http"
	_ "net/http/pprof"
)

func Setup() {
	go func() {
		log.Println(http.ListenAndServe("localhost:6060", nil))
	}()
}
